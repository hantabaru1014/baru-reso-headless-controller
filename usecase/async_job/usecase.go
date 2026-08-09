// Package async_job は時間のかかるホスト/セッション操作 (起動・停止・再起動) を
// 永続的な job キューに乗せ、worker から非同期で実行する仕組みを提供する.
//
// RPC handler は Enqueue で job を投入して即座に job_id を返す. worker
// (worker.AsyncJobExecutor) が PENDING な job を claim し、Dispatch を経由して
// 既存の Usecase メソッドを呼び出す. 完了時には notification.Bus 経由で
// JobCompletedEvent を投入元 user に push して、フロントエンド側で toast 表示や
// クエリ invalidate を行う.
package async_job

import (
	"context"
	"encoding/json"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// PermissionChecker は「他ユーザーの job 履歴も見てよいか」だけを判定する narrow interface.
// handler.go の HostOperator 等と同様、usecase パッケージ全体への依存を避けるためのもの.
type PermissionChecker interface {
	RequireSystemPermission(ctx context.Context, permKey string) error
}

// ListFilter は job 履歴一覧の絞り込み条件.
// port.AsyncJobListFilter を埋め込まずフィールドを並べているのは、あちらの
// CreatedBy が「認可の結果」を表す出力用フィールドで、呼び出し側に指定させては
// いけないため (埋め込むと RPC handler から任意のユーザーを指定できてしまう).
type ListFilter struct {
	Status  *entity.AsyncJobStatus
	JobType *entity.AsyncJobType
	// IncludeAllUsers は全ユーザーの job を返す指定. system 権限を要求する.
	IncludeAllUsers bool
	PageIndex       int32
	PageSize        int32
}

type ListResult = port.AsyncJobListResult

type Usecase struct {
	repo port.AsyncJobRepository
	perm PermissionChecker
}

func NewUsecase(repo port.AsyncJobRepository, perm PermissionChecker) *Usecase {
	return &Usecase{repo: repo, perm: perm}
}

// List は job 履歴を返す. 既定では caller 自身が投入した job のみに絞り、
// 全ユーザー分の閲覧には system:group.manage を要求する — 他ユーザーの job の
// last_error には他グループのホスト設定等が現れうるため.
func (u *Usecase) List(ctx context.Context, filter ListFilter) (*ListResult, error) {
	repoFilter := port.AsyncJobListFilter{
		Status:    filter.Status,
		JobType:   filter.JobType,
		PageIndex: filter.PageIndex,
		PageSize:  filter.PageSize,
	}

	if filter.IncludeAllUsers {
		// CreatedBy は nil のまま = 全ユーザーが対象.
		if err := u.perm.RequireSystemPermission(ctx, entity.PermKey_SystemGroupManage); err != nil {
			return nil, err
		}
	} else {
		// CurrentUserID は ctx から claims を取り出すだけの純関数なので、
		// narrow interface にはせず usecase パッケージのものを直接使う.
		callerID, err := usecase.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}

		repoFilter.CreatedBy = &callerID
	}

	result, err := u.repo.List(ctx, repoFilter)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return result, nil
}

// JobDetail は 1 件の job とそのエラー詳細 (builder のフルログ等).
type JobDetail struct {
	Job *entity.AsyncJob
	// ErrorDetail は詳細ログが無ければ nil.
	ErrorDetail *string
}

// Get は job 1 件をエラー詳細付きで返す. 認可は List と同じ規則:
// 自分が投入した job は誰でも、他ユーザーの job は system:group.manage が要る.
func (u *Usecase) Get(ctx context.Context, id string) (*JobDetail, error) {
	job, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	callerID, err := usecase.CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	if job.CreatedBy == nil || *job.CreatedBy != callerID {
		if err := u.perm.RequireSystemPermission(ctx, entity.PermKey_SystemGroupManage); err != nil {
			return nil, err
		}
	}

	// 詳細ログを持たない job のほうが多数なので、無いことは正常系.
	log, err := u.repo.GetLog(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, errors.Wrap(err, 0)
	}

	detail := &JobDetail{Job: job}
	if log != "" {
		detail.ErrorDetail = &log
	}

	return detail, nil
}

// EnqueueStartHost はホスト起動 job を登録する. createdBy は完了通知の宛先 user.
// host_id は起動時 (Start repo 呼び出し時) に発番されるため payload には含まれない.
func (u *Usecase) EnqueueStartHost(ctx context.Context, req *hdlctrlv1.StartHeadlessHostRequest, createdBy *string) (string, error) {
	payload, err := marshalPayload(req)
	if err != nil {
		return "", err
	}

	return u.enqueue(ctx, entity.AsyncJobType_START_HOST, payload, nil, nil, createdBy)
}

// EnqueueBuildImage は 1 バージョンのローカルビルド job を投入する.
// thenStart が非 nil の場合、build 成功後に自動で START_HOST job が enqueue される (chain).
func (u *Usecase) EnqueueBuildImage(
	ctx context.Context,
	manifestID string,
	branch entity.ResoniteVersionBranch,
	thenStart *hdlctrlv1.StartHeadlessHostRequest,
	createdBy *string,
) (string, error) {
	payload, err := marshalPayload(&hdlctrlv1.BuildResoniteImageRequest{
		ManifestId:    manifestID,
		Branch:        string(branch),
		ThenStartHost: thenStart,
	})
	if err != nil {
		return "", err
	}

	return u.enqueue(ctx, entity.AsyncJobType_BUILD_IMAGE, payload, nil, nil, createdBy)
}

func (u *Usecase) EnqueueShutdownHost(ctx context.Context, req *hdlctrlv1.ShutdownHeadlessHostRequest, createdBy *string) (string, error) {
	payload, err := marshalPayload(req)
	if err != nil {
		return "", err
	}

	hostID := req.GetHostId()

	return u.enqueue(ctx, entity.AsyncJobType_SHUTDOWN_HOST, payload, &hostID, nil, createdBy)
}

func (u *Usecase) EnqueueRestartHost(ctx context.Context, req *hdlctrlv1.RestartHeadlessHostRequest, createdBy *string) (string, error) {
	payload, err := marshalPayload(req)
	if err != nil {
		return "", err
	}

	hostID := req.GetHostId()

	return u.enqueue(ctx, entity.AsyncJobType_RESTART_HOST, payload, &hostID, nil, createdBy)
}

func (u *Usecase) EnqueueStartSession(ctx context.Context, req *hdlctrlv1.StartWorldRequest, createdBy *string) (string, error) {
	payload, err := marshalPayload(req)
	if err != nil {
		return "", err
	}

	hostID := req.GetHostId()

	return u.enqueue(ctx, entity.AsyncJobType_START_SESSION, payload, &hostID, nil, createdBy)
}

func (u *Usecase) EnqueueStopSession(ctx context.Context, req *hdlctrlv1.StopSessionRequest, createdBy *string) (string, error) {
	payload, err := marshalPayload(req)
	if err != nil {
		return "", err
	}

	sessionID := req.GetSessionId()

	return u.enqueue(ctx, entity.AsyncJobType_STOP_SESSION, payload, nil, &sessionID, createdBy)
}

func (u *Usecase) enqueue(
	ctx context.Context,
	jobType entity.AsyncJobType,
	payload json.RawMessage,
	hostID, sessionID, createdBy *string,
) (string, error) {
	job, err := u.repo.Create(ctx, port.AsyncJobCreateParams{
		JobType:   jobType,
		Payload:   payload,
		HostID:    hostID,
		SessionID: sessionID,
		CreatedBy: createdBy,
	})
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return job.ID, nil
}

func marshalPayload(msg proto.Message) (json.RawMessage, error) {
	b, err := protojson.Marshal(msg)
	if err != nil {
		return nil, errors.WrapPrefix(err, "marshal async job payload", 0)
	}

	return b, nil
}

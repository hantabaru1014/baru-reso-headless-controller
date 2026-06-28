package async_job

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/encoding/protojson"
)

// 以下の narrow interface は worker / handler が usecase パッケージ全体に依存
// しないためのもの. scheduled_op.SessionOperator と同じ作り.
type (
	HostOperator interface {
		HeadlessHostStart(ctx context.Context, params port.HeadlessHostStartParams, userID *string) (string, error)
		HeadlessHostShutdown(ctx context.Context, id string) error
		HeadlessHostRestart(ctx context.Context, id string, newTag *string, withWorldRestart bool, timeoutSeconds int) error
	}

	SessionOperator interface {
		StartSession(ctx context.Context, hostID string, groupID string, userID *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error)
		StopSession(ctx context.Context, sessionID string) error
	}

	AccountFetcher interface {
		GetHeadlessAccount(ctx context.Context, id string) (*entity.HeadlessAccount, error)
	}
)

// Dispatcher は jobType ごとの handler を保持する. Dispatch は payload を
// decode し、戻り値の result を JSON で返す (MarkSucceeded の result_payload 用).
type Dispatcher struct {
	host    HostOperator
	session SessionOperator
	account AccountFetcher
}

func NewDispatcher(host HostOperator, session SessionOperator, account AccountFetcher) *Dispatcher {
	return &Dispatcher{host: host, session: session, account: account}
}

// JobResult は MarkSucceeded.result_payload に詰める形. 完了通知の message 組み立てや
// フロントエンドの後処理 (新規 host_id / session_id への navigation) に使う.
type JobResult struct {
	HostID    string `json:"host_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// Dispatch は 1 件の job を実行する. 戻り値の result は MarkSucceeded に詰める.
// notification 用の message も返す (level=SUCCESS or ERROR は呼び出し側で決める).
func (d *Dispatcher) Dispatch(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	switch job.JobType {
	case entity.AsyncJobType_START_HOST:
		return d.startHost(ctx, job)
	case entity.AsyncJobType_SHUTDOWN_HOST:
		return d.shutdownHost(ctx, job)
	case entity.AsyncJobType_RESTART_HOST:
		return d.restartHost(ctx, job)
	case entity.AsyncJobType_START_SESSION:
		return d.startSession(ctx, job)
	case entity.AsyncJobType_STOP_SESSION:
		return d.stopSession(ctx, job)
	case entity.AsyncJobType_UNKNOWN:
		return JobResult{}, "", errors.Errorf("unknown job type")
	default:
		return JobResult{}, "", errors.Errorf("unsupported job type: %d", job.JobType)
	}
}

func (d *Dispatcher) startHost(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	req := &hdlctrlv1.StartHeadlessHostRequest{}
	if err := protojson.Unmarshal(job.Payload, req); err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "decode start_host payload", 0)
	}

	account, err := d.account.GetHeadlessAccount(ctx, req.GetHeadlessAccountId())
	if err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "get headless account", 0)
	}

	groupID := req.GetGroupId()
	if groupID == "" {
		// 通常 RPC handler 側で resolve 済みだが、保険として account の group_id にフォールバック
		groupID = account.GroupID
	}

	params := port.HeadlessHostStartParams{
		Name:              req.GetName(),
		HeadlessAccount:   *account,
		ContainerImageTag: req.GetImageTag(),
		StartupConfig:     req.GetStartupConfig(),
		GroupID:           groupID,
	}
	if req.AutoUpdatePolicy != nil && req.GetAutoUpdatePolicy() != hdlctrlv1.HeadlessHostAutoUpdatePolicy_HEADLESS_HOST_AUTO_UPDATE_POLICY_UNKNOWN {
		params.AutoUpdatePolicy = entity.HostAutoUpdatePolicy(req.GetAutoUpdatePolicy())
	}

	if req.Memo != nil {
		params.Memo = req.GetMemo()
	}

	hostID, err := d.host.HeadlessHostStart(ctx, params, job.CreatedBy)
	if err != nil {
		return JobResult{}, "", err
	}

	return JobResult{HostID: hostID}, fmt.Sprintf("ホスト %q を起動しました", req.GetName()), nil
}

func (d *Dispatcher) shutdownHost(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	req := &hdlctrlv1.ShutdownHeadlessHostRequest{}
	if err := protojson.Unmarshal(job.Payload, req); err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "decode shutdown_host payload", 0)
	}

	if err := d.host.HeadlessHostShutdown(ctx, req.GetHostId()); err != nil {
		return JobResult{}, "", err
	}

	return JobResult{HostID: req.GetHostId()}, fmt.Sprintf("ホスト %s を停止しました", req.GetHostId()), nil
}

const defaultRestartTimeoutSeconds = 10 * 60

func (d *Dispatcher) restartHost(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	req := &hdlctrlv1.RestartHeadlessHostRequest{}
	if err := protojson.Unmarshal(job.Payload, req); err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "decode restart_host payload", 0)
	}

	var newTag *string

	if req.GetWithUpdate() {
		s := "latestRelease"
		newTag = &s
	} else if req.GetWithImageTag() != "" {
		s := req.GetWithImageTag()
		newTag = &s
	}

	timeout := defaultRestartTimeoutSeconds
	if req.TimeoutSeconds != nil {
		timeout = int(req.GetTimeoutSeconds())
	}

	if err := d.host.HeadlessHostRestart(ctx, req.GetHostId(), newTag, req.GetWithWorldRestart(), timeout); err != nil {
		return JobResult{}, "", err
	}

	return JobResult{HostID: req.GetHostId()}, fmt.Sprintf("ホスト %s を再起動しました", req.GetHostId()), nil
}

func (d *Dispatcher) startSession(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	req := &hdlctrlv1.StartWorldRequest{}
	if err := protojson.Unmarshal(job.Payload, req); err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "decode start_session payload", 0)
	}

	var memo *string

	if m := req.GetMemo(); m != "" {
		memo = &m
	}

	s, err := d.session.StartSession(ctx, req.GetHostId(), req.GetGroupId(), job.CreatedBy, req.GetParameters(), memo)
	if err != nil {
		return JobResult{}, "", err
	}

	name := req.GetParameters().GetName()
	if name == "" {
		name = s.ID
	}

	return JobResult{SessionID: s.ID, HostID: req.GetHostId()}, fmt.Sprintf("セッション %q を開始しました", name), nil
}

func (d *Dispatcher) stopSession(ctx context.Context, job *entity.AsyncJob) (JobResult, string, error) {
	req := &hdlctrlv1.StopSessionRequest{}
	if err := protojson.Unmarshal(job.Payload, req); err != nil {
		return JobResult{}, "", errors.WrapPrefix(err, "decode stop_session payload", 0)
	}

	if err := d.session.StopSession(ctx, req.GetSessionId()); err != nil {
		return JobResult{}, "", err
	}

	return JobResult{SessionID: req.GetSessionId()}, fmt.Sprintf("セッション %s を停止しました", req.GetSessionId()), nil
}

// MarshalResult は JobResult を JSON RawMessage に詰め直すヘルパー.
func MarshalResult(r JobResult) (json.RawMessage, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

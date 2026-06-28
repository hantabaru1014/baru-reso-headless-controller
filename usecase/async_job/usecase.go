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
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Usecase struct {
	repo port.AsyncJobRepository
}

func NewUsecase(repo port.AsyncJobRepository) *Usecase {
	return &Usecase{repo: repo}
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

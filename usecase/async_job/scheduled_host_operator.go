package async_job

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

// ScheduledHostOperator は予約操作からのホスト操作を非同期 job として投入する.
// ctx の caller (予約の作成者) を job の created_by にするので、完了通知はその user に届き、
// job 実行時の権限検査もその user で行われる.
type ScheduledHostOperator struct {
	jobs     *Usecase
	hostRepo port.HeadlessHostRepository
}

var _ scheduled_op.HostOperator = (*ScheduledHostOperator)(nil)

func NewScheduledHostOperator(jobs *Usecase, hostRepo port.HeadlessHostRepository) *ScheduledHostOperator {
	return &ScheduledHostOperator{jobs: jobs, hostRepo: hostRepo}
}

func (o *ScheduledHostOperator) StartStoppedHost(ctx context.Context, hostID string, withWorldRestart bool) (bool, error) {
	createdBy, err := o.authorize(ctx, hostID)
	if err != nil {
		return false, err
	}

	status, err := o.hostRepo.GetStatus(ctx, hostID)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if !status.IsStopped() {
		return false, nil
	}

	// 停止中ホストの起動は再起動と同じ経路 (最新版で起動し、未ビルドなら BUILD_IMAGE を chain する) に乗せる.
	_, err = o.jobs.EnqueueRestartHost(ctx, &hdlctrlv1.RestartHeadlessHostRequest{
		HostId:           hostID,
		WithWorldRestart: withWorldRestart,
	}, &createdBy)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (o *ScheduledHostOperator) RestartHost(ctx context.Context, req *hdlctrlv1.RestartHeadlessHostRequest) error {
	createdBy, err := o.authorize(ctx, req.GetHostId())
	if err != nil {
		return err
	}

	_, err = o.jobs.EnqueueRestartHost(ctx, req, &createdBy)

	return err
}

func (o *ScheduledHostOperator) ShutdownHost(ctx context.Context, hostID string) error {
	createdBy, err := o.authorize(ctx, hostID)
	if err != nil {
		return err
	}

	_, err = o.jobs.EnqueueShutdownHost(ctx, &hdlctrlv1.ShutdownHeadlessHostRequest{HostId: hostID}, &createdBy)

	return err
}

// authorize は host:write を確かめ、job の created_by にする caller の user id を返す.
// job 実行時にも検査されるが、権限の無い予約は投入せずに予約自体を失敗させる.
func (o *ScheduledHostOperator) authorize(ctx context.Context, hostID string) (string, error) {
	userID, err := usecase.CurrentUserID(ctx)
	if err != nil {
		return "", err
	}

	groupID, err := o.hostRepo.GetGroupID(ctx, hostID)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if err := o.jobs.perm.RequirePermissionForGroup(ctx, groupID, entity.PermKey_HostWrite); err != nil {
		return "", err
	}

	return userID, nil
}

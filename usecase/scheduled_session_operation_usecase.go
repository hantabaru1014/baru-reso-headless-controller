package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"

	// 副作用 import: trigger / action registry を埋める.
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
)

type ScheduledSessionOperationUsecase struct {
	repo port.ScheduledSessionOperationRepository
}

func NewScheduledSessionOperationUsecase(repo port.ScheduledSessionOperationRepository) *ScheduledSessionOperationUsecase {
	return &ScheduledSessionOperationUsecase{repo: repo}
}

type CreateScheduledSessionOperationParams struct {
	Action    scheduled_op.Action
	Trigger   scheduled_op.Trigger
	HostID    *string
	SessionID *string
	CreatedBy *string
}

func (u *ScheduledSessionOperationUsecase) Create(ctx context.Context, params CreateScheduledSessionOperationParams) (*entity.ScheduledSessionOperation, error) {
	if params.Action == nil {
		return nil, errors.New("create scheduled operation: action is required")
	}

	if params.Trigger == nil {
		return nil, errors.New("create scheduled operation: trigger is required")
	}

	actionPayload, err := params.Action.Marshal()
	if err != nil {
		return nil, errors.WrapPrefix(err, "marshal action", 0)
	}

	triggerConfig, err := params.Trigger.Marshal()
	if err != nil {
		return nil, errors.WrapPrefix(err, "marshal trigger", 0)
	}

	// next_fire_at は trigger.Evaluate(now=very past) でも nextCheck が返るが、
	// 初回登録時は trigger 側に "登録時の next_fire_at" を聞くのが筋。今回は単純に
	// Evaluate を 0 時刻で呼び出して nextCheck を貰う (TimeTrigger なら ScheduledAt が返る).
	_, nextFire, err := params.Trigger.Evaluate(ctx, scheduled_op.TriggerEvalDeps{})
	if err != nil {
		return nil, errors.WrapPrefix(err, "trigger initial evaluate", 0)
	}

	if nextFire.IsZero() {
		// Trigger が即座に ready を返した場合 (ScheduledAt が現在 <= now のケースなど)。
		// 直近の worker tick で拾えるよう now をそのまま入れる.
		nextFire = time.Now()
	}

	created, err := u.repo.Create(ctx, port.ScheduledSessionOperationCreateParams{
		OperationType:    params.Action.Type(),
		OperationPayload: actionPayload,
		TriggerType:      params.Trigger.Type(),
		TriggerConfig:    triggerConfig,
		NextFireAt:       nextFire,
		HostID:           params.HostID,
		SessionID:        params.SessionID,
		CreatedBy:        params.CreatedBy,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return created, nil
}

func (u *ScheduledSessionOperationUsecase) Get(ctx context.Context, id string) (*entity.ScheduledSessionOperation, error) {
	return u.repo.Get(ctx, id)
}

type ListScheduledSessionOperationsFilter = port.ScheduledSessionOperationListFilter
type ListScheduledSessionOperationsResult = port.ScheduledSessionOperationListResult

func (u *ScheduledSessionOperationUsecase) List(ctx context.Context, filter ListScheduledSessionOperationsFilter) (*ListScheduledSessionOperationsResult, error) {
	return u.repo.List(ctx, filter)
}

var ErrScheduledOperationNotCancelable = errors.New("scheduled operation cannot be canceled in its current status")

func (u *ScheduledSessionOperationUsecase) Cancel(ctx context.Context, id string) error {
	ok, err := u.repo.Cancel(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if !ok {
		return ErrScheduledOperationNotCancelable
	}

	return nil
}

// DecodeAction / DecodeTrigger は registry の薄いプロキシ. RPC handler から呼ぶ.
func (u *ScheduledSessionOperationUsecase) DecodeAction(t entity.ScheduledOperationType, payload json.RawMessage) (scheduled_op.Action, error) {
	return scheduled_op.DecodeAction(t, payload)
}

func (u *ScheduledSessionOperationUsecase) DecodeTrigger(t entity.ScheduledTriggerType, cfg json.RawMessage) (scheduled_op.Trigger, error) {
	return scheduled_op.DecodeTrigger(t, cfg)
}

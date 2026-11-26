package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"

	// 副作用 import: trigger / action registry を埋める.
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
)

type ScheduledSessionOperationUsecase struct {
	repo        port.ScheduledSessionOperationRepository
	hostRepo    port.HeadlessHostRepository
	sessionRepo port.SessionRepository
	permUC      *PermissionUsecase
}

func NewScheduledSessionOperationUsecase(
	repo port.ScheduledSessionOperationRepository,
	hostRepo port.HeadlessHostRepository,
	sessionRepo port.SessionRepository,
	permUC *PermissionUsecase,
) *ScheduledSessionOperationUsecase {
	return &ScheduledSessionOperationUsecase{
		repo:        repo,
		hostRepo:    hostRepo,
		sessionRepo: sessionRepo,
		permUC:      permUC,
	}
}

// resolveTargetGroupID は HostID または SessionID から対象の group_id を引く.
// どちらも未指定なら error.
//
//nolint:funcorder // Create/List/Cancel の手前にヘルパーを置く方が読みやすい
func (u *ScheduledSessionOperationUsecase) resolveTargetGroupID(ctx context.Context, hostID, sessionID *string) (string, error) {
	if sessionID != nil && *sessionID != "" {
		s, err := u.sessionRepo.Get(ctx, *sessionID)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return s.GroupID, nil
	}

	if hostID != nil && *hostID != "" {
		gid, err := u.hostRepo.GetGroupID(ctx, *hostID)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return gid, nil
	}

	return "", errors.New("either host_id or session_id is required")
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

	// 対象 host/session の group に対して action ごとの permission (session:write / host:write) を要求する.
	groupID, err := u.resolveTargetGroupID(ctx, params.HostID, params.SessionID)
	if err != nil {
		return nil, err
	}

	permKey, err := scheduled_op.RequiredPermission(params.Action.Type())
	if err != nil {
		return nil, err
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, permKey); err != nil {
		return nil, err
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

type ListScheduledSessionOperationsFilter struct {
	SessionID *string
	HostID    *string
	Status    *entity.ScheduledOperationStatus
	// GroupID 指定時はそのグループの op のみ返す (session:read 必須).
	GroupID   *string
	PageIndex int32
	PageSize  int32
}

type ListScheduledSessionOperationsResult = port.ScheduledSessionOperationListResult

func (u *ScheduledSessionOperationUsecase) List(ctx context.Context, filter ListScheduledSessionOperationsFilter) (*ListScheduledSessionOperationsResult, error) {
	repoFilter := port.ScheduledSessionOperationListFilter{
		SessionID: filter.SessionID,
		HostID:    filter.HostID,
		Status:    filter.Status,
		PageIndex: filter.PageIndex,
		PageSize:  filter.PageSize,
	}

	switch {
	case filter.GroupID != nil && *filter.GroupID != "":
		// group_id 指定: そのグループに session:read があるかチェック.
		if err := u.permUC.RequirePermissionForGroup(ctx, *filter.GroupID, entity.PermKey_SessionRead); err != nil {
			return nil, err
		}

		repoFilter.GroupIDs = []string{*filter.GroupID}

	case filter.HostID != nil || filter.SessionID != nil:
		// host_id / session_id 指定: 対象 host/session の group_id で認可.
		groupID, err := u.resolveTargetGroupID(ctx, filter.HostID, filter.SessionID)
		if err != nil {
			// 対象 host/session が DB に存在しないなら、その filter で hit する op も
			// 存在しない. 0 件を返す方が 404 より UX が素直.
			if errors.Is(err, domain.ErrNotFound) {
				return &port.ScheduledSessionOperationListResult{}, nil
			}

			return nil, err
		}

		if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_SessionRead); err != nil {
			return nil, err
		}

	default:
		// フィルタ未指定: caller が session:read を持つ全グループの op を返す.
		userID, err := CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}

		groupIDs, listAll, err := u.permUC.ResolveListGroupFilter(ctx, userID, entity.PermKey_SessionRead)
		if err != nil {
			return nil, err
		}

		if !listAll {
			// GroupIDs が nil だと全グループ対象になるため、権限を持つグループが無いならここで 0 件を返す.
			if len(groupIDs) == 0 {
				return &port.ScheduledSessionOperationListResult{}, nil
			}

			repoFilter.GroupIDs = groupIDs
		}
	}

	return u.repo.List(ctx, repoFilter)
}

var ErrScheduledOperationNotCancelable = errors.New("scheduled operation cannot be canceled in its current status")

func (u *ScheduledSessionOperationUsecase) Cancel(ctx context.Context, id string) error {
	// 対象 op を引いて group_id を導出し、登録時と同じ permission を要求.
	op, err := u.repo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	groupID, err := u.resolveTargetGroupID(ctx, op.HostID, op.SessionID)
	if err != nil {
		return err
	}

	permKey, err := scheduled_op.RequiredPermission(op.OperationType)
	if err != nil {
		return err
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, permKey); err != nil {
		return err
	}

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

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

	// 対象 host/session の group に対して session:write を要求する.
	groupID, err := u.resolveTargetGroupID(ctx, params.HostID, params.SessionID)
	if err != nil {
		return nil, err
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_SessionWrite); err != nil {
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

type ListScheduledSessionOperationsFilter = port.ScheduledSessionOperationListFilter
type ListScheduledSessionOperationsResult = port.ScheduledSessionOperationListResult

func (u *ScheduledSessionOperationUsecase) List(ctx context.Context, filter ListScheduledSessionOperationsFilter) (*ListScheduledSessionOperationsResult, error) {
	// group_id 指定: そのグループに session:read があるかチェック → 対象 op を group で post-filter.
	if filter.GroupID != nil && *filter.GroupID != "" {
		if err := u.permUC.RequirePermissionForGroup(ctx, *filter.GroupID, entity.PermKey_SessionRead); err != nil {
			return nil, err
		}

		return u.listFilteredByGroups(ctx, filter, map[string]bool{*filter.GroupID: true})
	}

	// host_id / session_id 指定: 対象 host/session の group_id で認可.
	if filter.HostID != nil || filter.SessionID != nil {
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

		return u.repo.List(ctx, filter)
	}

	// フィルタ未指定: caller が session:read を持つ全グループの op を返す.
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	groupIDs, listAll, err := u.permUC.ResolveListGroupFilter(ctx, userID, entity.PermKey_SessionRead)
	if err != nil {
		return nil, err
	}

	if listAll {
		return u.repo.List(ctx, filter)
	}

	accessible := make(map[string]bool, len(groupIDs))
	for _, g := range groupIDs {
		accessible[g] = true
	}

	return u.listFilteredByGroups(ctx, filter, accessible)
}

// listFilteredByGroups は repo.List の結果を accessible グループの op のみに in-app で絞る.
// scheduled_session_operations は group_id 列を持たないため、各 op の target host/session から
// 都度 group_id を引いて判定する. データ量は通常少ない (worker tick 単位の予約) ので問題ない想定.
//nolint:funcorder // List の直下にヘルパーを置く方が読みやすい
func (u *ScheduledSessionOperationUsecase) listFilteredByGroups(ctx context.Context, filter ListScheduledSessionOperationsFilter, accessible map[string]bool) (*ListScheduledSessionOperationsResult, error) {
	// page を取り払って全件取得 → 絞り込み → in-app paging.
	rawFilter := filter
	rawFilter.PageIndex = 0
	rawFilter.PageSize = 0 // repo 側で 0 は「上限なし」扱いになる前提.

	result, err := u.repo.List(ctx, rawFilter)
	if err != nil {
		return nil, err
	}

	filtered := make(entity.ScheduledSessionOperationList, 0, len(result.Items))

	for _, op := range result.Items {
		gid, err := u.resolveTargetGroupID(ctx, op.HostID, op.SessionID)
		if err != nil {
			// 対象 host/session が消えている op はスキップ.
			continue
		}

		if accessible[gid] {
			filtered = append(filtered, op)
		}
	}

	total := int32(len(filtered)) //nolint:gosec // 件数は in-memory 上限内, overflow しない

	// in-app paging.
	start := filter.PageIndex * filter.PageSize
	if start < 0 || start >= total {
		return &port.ScheduledSessionOperationListResult{Items: nil, TotalCount: total}, nil
	}

	end := start + filter.PageSize
	if filter.PageSize == 0 || end > total {
		end = total
	}

	return &port.ScheduledSessionOperationListResult{
		Items:      filtered[start:end],
		TotalCount: total,
	}, nil
}

var ErrScheduledOperationNotCancelable = errors.New("scheduled operation cannot be canceled in its current status")

func (u *ScheduledSessionOperationUsecase) Cancel(ctx context.Context, id string) error {
	// 対象 op を引いて group_id を導出し session:write を要求.
	op, err := u.repo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	groupID, err := u.resolveTargetGroupID(ctx, op.HostID, op.SessionID)
	if err != nil {
		return err
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_SessionWrite); err != nil {
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

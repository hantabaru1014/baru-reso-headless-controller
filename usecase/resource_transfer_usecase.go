package usecase

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalidTransferTarget は移管の起点リソースの指定が不正.
	ErrInvalidTransferTarget = errors.New("invalid transfer target")
	// ErrInvalidTransferDestination は移管先にできないグループ (移管元と同じ / system グループ).
	ErrInvalidTransferDestination = errors.New("invalid transfer destination group")
)

// ResourceTransferTarget は移管の起点にするリソース. いずれか 1 つだけを指定する.
type ResourceTransferTarget struct {
	HostID    string
	SessionID string
	// アカウント起点の場合の (group_id, resonite_id).
	AccountGroupID string
	AccountID      string
}

type TransferredResource struct {
	ID   string
	Name string
}

// ResourceTransferResult は移管対象 (dry run の場合は移管予定) のリソース一式.
type ResourceTransferResult struct {
	SourceGroupID      string
	DestinationGroupID string
	// Account は移管元のアカウント登録. 既に削除されている場合は nil.
	Account *entity.HeadlessAccount
	// AccountMerged は移管先に同一アカウントが既に登録されており、移管先の登録へ
	// マージされる (移管元の登録は削除される) ことを表す.
	AccountMerged bool
	Hosts         []TransferredResource
	Sessions      []TransferredResource
}

// ResourceTransferUsecase はリソースのグループ間移管を扱う.
//
// ホストはアカウントに、セッションはホストに依存し、3 者は同一グループに属する必要がある
// (同一グループ制約). 1 つだけ移すと残りが使えなくなるため、どのリソースを起点にしても
// 「アカウント + そのアカウントを使う全ホスト + それらのホスト上の全セッション」を
// 1 単位としてまとめて移す.
type ResourceTransferUsecase struct {
	queries   *db.Queries
	pool      *pgxpool.Pool
	permUC    *PermissionUsecase
	groupRepo port.GroupRepository
}

func NewResourceTransferUsecase(queries *db.Queries, pool *pgxpool.Pool, permUC *PermissionUsecase, groupRepo port.GroupRepository) *ResourceTransferUsecase {
	return &ResourceTransferUsecase{
		queries:   queries,
		pool:      pool,
		permUC:    permUC,
		groupRepo: groupRepo,
	}
}

// transferSource は起点リソースから解決した移管単位.
type transferSource struct {
	groupID string
	// accountID は移管単位を束ねるアカウント (resonite_id).
	// ホストが削除済みのセッション単体を移す場合は空で、orphanSession が入る.
	accountID     string
	orphanSession *db.Session
}

// Transfer は target を起点とする移管単位を destinationGroupID へ移す.
// dryRun が true の場合は権限・移管先の検証まで行い、移管対象だけを返す.
//
// 権限要件 (RPC interceptor は通過のみで、ここが唯一の認可ポイント):
//   - 起点リソースを閲覧できること (できなければ NotFound)
//   - 移管元・移管先の両グループに対し、移管対象に含まれるリソース種別ごとの write 権限
//   - ホストを移す場合は移管先グループの account:use (StartHeadlessHost と同じ要件)
func (u *ResourceTransferUsecase) Transfer(ctx context.Context, target ResourceTransferTarget, destinationGroupID string, dryRun bool) (*ResourceTransferResult, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	if destinationGroupID == "" {
		return nil, errors.Wrap(fmt.Errorf("%w: destination_group_id is required", ErrInvalidTransferDestination), 0)
	}

	src, err := u.resolveSource(ctx, userID, target)
	if err != nil {
		return nil, err
	}

	if src.groupID == destinationGroupID {
		return nil, errors.Wrap(fmt.Errorf("%w: same as source group", ErrInvalidTransferDestination), 0)
	}

	if dryRun {
		return u.plan(ctx, u.queries, userID, src, destinationGroupID)
	}

	var result *ResourceTransferResult

	err = db.RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		qtx := u.queries.WithTx(tx)

		// 移管対象の列挙と認可を tx 内でやり直し、確認後に増えたリソースの取りこぼしを防ぐ.
		planned, err := u.plan(ctx, qtx, userID, src, destinationGroupID)
		if err != nil {
			return err
		}

		result = planned

		return u.apply(ctx, qtx, src, planned)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// resolveSource は起点リソースから移管元グループと移管単位を解決する.
// 起点リソースを閲覧できない caller には、存在しない ID と区別がつかないよう NotFound を返す.
func (u *ResourceTransferUsecase) resolveSource(ctx context.Context, userID string, target ResourceTransferTarget) (*transferSource, error) {
	switch {
	case target.HostID != "":
		host, err := u.queries.GetHost(ctx, target.HostID)
		if err != nil {
			return nil, wrapTransferLookupErr(err, "headless host")
		}

		if err := u.requireReadable(ctx, userID, host.GroupID, entity.PermKey_HostRead, "headless host"); err != nil {
			return nil, err
		}

		return &transferSource{groupID: host.GroupID, accountID: host.AccountID}, nil
	case target.SessionID != "":
		session, err := u.queries.GetSession(ctx, target.SessionID)
		if err != nil {
			return nil, wrapTransferLookupErr(err, "session")
		}

		if err := u.requireReadable(ctx, userID, session.GroupID, entity.PermKey_SessionRead, "session"); err != nil {
			return nil, err
		}

		host, err := u.queries.GetHost(ctx, session.HostID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// ホスト削除後も残っている終了済みセッションは、それ単体で移す.
				return &transferSource{groupID: session.GroupID, orphanSession: &session}, nil
			}

			return nil, errors.Wrap(err, 0)
		}

		return &transferSource{groupID: host.GroupID, accountID: host.AccountID}, nil
	case target.AccountID != "":
		if target.AccountGroupID == "" {
			return nil, errors.Wrap(fmt.Errorf("%w: account group_id is required", ErrInvalidTransferTarget), 0)
		}

		account, err := u.queries.GetHeadlessAccount(ctx, db.GetHeadlessAccountParams{
			GroupID:    target.AccountGroupID,
			ResoniteID: target.AccountID,
		})
		if err != nil {
			return nil, wrapTransferLookupErr(err, "headless account")
		}

		if err := u.requireReadable(ctx, userID, account.GroupID, entity.PermKey_AccountRead, "headless account"); err != nil {
			return nil, err
		}

		return &transferSource{groupID: account.GroupID, accountID: account.ResoniteID}, nil
	default:
		return nil, errors.Wrap(fmt.Errorf("%w: one of host_id / session_id / account is required", ErrInvalidTransferTarget), 0)
	}
}

// plan は移管対象を列挙し、移管元・移管先に対する権限と移管先グループの妥当性を検証する.
func (u *ResourceTransferUsecase) plan(ctx context.Context, q *db.Queries, userID string, src *transferSource, destinationGroupID string) (*ResourceTransferResult, error) {
	result := &ResourceTransferResult{
		SourceGroupID:      src.groupID,
		DestinationGroupID: destinationGroupID,
		Hosts:              []TransferredResource{},
		Sessions:           []TransferredResource{},
	}

	if src.orphanSession != nil {
		result.Sessions = append(result.Sessions, TransferredResource{ID: src.orphanSession.ID, Name: src.orphanSession.Name})
	} else {
		if err := u.collectAccountUnit(ctx, q, src, result); err != nil {
			return nil, err
		}
	}

	sourceKeys := []string{}
	destinationKeys := []string{}

	if result.Account != nil {
		sourceKeys = append(sourceKeys, entity.PermKey_AccountWrite)
		destinationKeys = append(destinationKeys, entity.PermKey_AccountWrite)
	}

	if len(result.Hosts) > 0 {
		sourceKeys = append(sourceKeys, entity.PermKey_HostWrite)
		// 移したホストは移管先グループのアカウント登録を使うことになる.
		destinationKeys = append(destinationKeys, entity.PermKey_HostWrite, entity.PermKey_AccountUse)
	}

	if len(result.Sessions) > 0 {
		sourceKeys = append(sourceKeys, entity.PermKey_SessionWrite)
		destinationKeys = append(destinationKeys, entity.PermKey_SessionWrite)
	}

	if err := u.requireAll(ctx, userID, src.groupID, sourceKeys); err != nil {
		return nil, err
	}

	// 移管先の存在確認より先に権限を見る. 非所属グループの存在有無を
	// NotFound / PermissionDenied の違いで漏らさないため.
	if err := u.requireAll(ctx, userID, destinationGroupID, destinationKeys); err != nil {
		return nil, err
	}

	destination, err := u.groupRepo.Get(ctx, destinationGroupID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if destination.Type == entity.GroupType_System {
		return nil, errors.Wrap(fmt.Errorf("%w: system group cannot own resources", ErrInvalidTransferDestination), 0)
	}

	return result, nil
}

// collectAccountUnit は src のアカウントに紐づく移管単位 (アカウント登録 / ホスト / セッション) を result に詰める.
func (u *ResourceTransferUsecase) collectAccountUnit(ctx context.Context, q *db.Queries, src *transferSource, result *ResourceTransferResult) error {
	account, err := q.GetHeadlessAccount(ctx, db.GetHeadlessAccountParams{
		GroupID:    src.groupID,
		ResoniteID: src.accountID,
	})

	switch {
	case err == nil:
		result.Account = headlessAccountToEntity(account)

		merged, err := accountExists(ctx, q, result.DestinationGroupID, src.accountID)
		if err != nil {
			return err
		}

		result.AccountMerged = merged
	case errors.Is(err, pgx.ErrNoRows):
		// アカウント登録だけ先に削除されたホストも移せるようにする.
	default:
		return errors.Wrap(err, 0)
	}

	hosts, err := q.ListHostsByAccount(ctx, db.ListHostsByAccountParams{
		GroupID:   src.groupID,
		AccountID: src.accountID,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for _, h := range hosts {
		result.Hosts = append(result.Hosts, TransferredResource{ID: h.ID, Name: h.Name})
	}

	sessions, err := q.ListSessionsByHostAccount(ctx, db.ListSessionsByHostAccountParams{
		GroupID:   src.groupID,
		AccountID: src.accountID,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for _, s := range sessions {
		result.Sessions = append(result.Sessions, TransferredResource{ID: s.ID, Name: s.Name})
	}

	return nil
}

// apply は plan 済みの移管を実行する. tx-scoped な Queries で呼ぶこと.
func (u *ResourceTransferUsecase) apply(ctx context.Context, qtx *db.Queries, src *transferSource, result *ResourceTransferResult) error {
	if src.orphanSession != nil {
		if err := qtx.UpdateSessionGroup(ctx, db.UpdateSessionGroupParams{
			ID:      src.orphanSession.ID,
			GroupID: result.DestinationGroupID,
		}); err != nil {
			return errors.Wrap(err, 0)
		}

		return nil
	}

	// セッションは「移管元グループのホスト」を辿って特定するため、ホストより先に移す.
	if err := qtx.UpdateSessionsGroupByHostAccount(ctx, db.UpdateSessionsGroupByHostAccountParams{
		NewGroupID: result.DestinationGroupID,
		GroupID:    src.groupID,
		AccountID:  src.accountID,
	}); err != nil {
		return errors.Wrap(err, 0)
	}

	if err := qtx.UpdateHostsGroupByAccount(ctx, db.UpdateHostsGroupByAccountParams{
		NewGroupID: result.DestinationGroupID,
		GroupID:    src.groupID,
		AccountID:  src.accountID,
	}); err != nil {
		return errors.Wrap(err, 0)
	}

	if result.Account == nil {
		return nil
	}

	if result.AccountMerged {
		// アカウントは (group_id, resonite_id) で一意なので移管先の登録を残し、移管元の登録を消す.
		// 移したホストは group_id が変わったことで移管先の登録を参照するようになる.
		if err := qtx.DeleteHeadlessAccount(ctx, db.DeleteHeadlessAccountParams{
			GroupID:    src.groupID,
			ResoniteID: src.accountID,
		}); err != nil {
			return errors.Wrap(err, 0)
		}

		return nil
	}

	if err := qtx.UpdateHeadlessAccountGroup(ctx, db.UpdateHeadlessAccountGroupParams{
		NewGroupID: result.DestinationGroupID,
		GroupID:    src.groupID,
		ResoniteID: src.accountID,
	}); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func accountExists(ctx context.Context, q *db.Queries, groupID, resoniteID string) (bool, error) {
	_, err := q.GetHeadlessAccount(ctx, db.GetHeadlessAccountParams{
		GroupID:    groupID,
		ResoniteID: resoniteID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, errors.Wrap(err, 0)
	}

	return true, nil
}

func (u *ResourceTransferUsecase) requireReadable(ctx context.Context, userID, groupID, readKey, notFoundPrefix string) error {
	ok, err := u.permUC.HasPermission(ctx, userID, groupID, readKey)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if !ok {
		return errors.WrapPrefix(domain.ErrNotFound, notFoundPrefix, 0)
	}

	return nil
}

func (u *ResourceTransferUsecase) requireAll(ctx context.Context, userID, groupID string, permKeys []string) error {
	for _, k := range permKeys {
		ok, err := u.permUC.HasPermission(ctx, userID, groupID, k)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if !ok {
			return errors.Wrap(fmt.Errorf("%w: %s on group %s", domain.ErrPermissionDenied, k, groupID), 0)
		}
	}

	return nil
}

func wrapTransferLookupErr(err error, prefix string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.WrapPrefix(domain.ErrNotFound, prefix, 0)
	}

	return errors.Wrap(err, 0)
}

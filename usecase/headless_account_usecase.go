package usecase

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type HeadlessAccountUsecase struct {
	queries        *db.Queries
	skyfrostClient skyfrost.Client
	permUC         *PermissionUsecase
}

func NewHeadlessAccountUsecase(queries *db.Queries, skyfrostClient skyfrost.Client, permUC *PermissionUsecase) *HeadlessAccountUsecase {
	return &HeadlessAccountUsecase{
		queries:        queries,
		skyfrostClient: skyfrostClient,
		permUC:         permUC,
	}
}

func (u *HeadlessAccountUsecase) CreateHeadlessAccount(ctx context.Context, credential, password, groupID string, createdBy *string) error {
	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_AccountWrite); err != nil {
		return err
	}

	userSession, err := u.skyfrostClient.UserLogin(ctx, credential, password)
	if err != nil {
		return errors.Errorf("failed to login: %w", err)
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, userSession.UserId)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	createdByText := pgtype.Text{}
	if createdBy != nil {
		createdByText = pgtype.Text{String: *createdBy, Valid: true}
	}

	return u.queries.CreateHeadlessAccount(ctx, db.CreateHeadlessAccountParams{
		ResoniteID:      userSession.UserId,
		Credential:      credential,
		Password:        password,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
		LastIconUrl:     pgtype.Text{String: userInfo.IconUrl, Valid: true},
		GroupID:         groupID,
		CreatedBy:       createdByText,
	})
}

func (u *HeadlessAccountUsecase) UpdateHeadlessAccountCredentials(ctx context.Context, resoniteID, credential, password string) error {
	if err := u.requireAccountWrite(ctx, resoniteID); err != nil {
		return err
	}

	userSession, err := u.skyfrostClient.UserLogin(ctx, credential, password)
	if err != nil {
		return errors.Errorf("failed to login: %w", err)
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, userSession.UserId)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if resoniteID != userInfo.ID {
		return errors.New("does not match resonite ID")
	}

	return u.queries.UpdateHeadlessAccountCredentials(ctx, db.UpdateHeadlessAccountCredentialsParams{
		ResoniteID: resoniteID,
		Credential: credential,
		Password:   password,
	})
}

func headlessAccountToEntity(v db.HeadlessAccount) *entity.HeadlessAccount {
	e := &entity.HeadlessAccount{
		ResoniteID: v.ResoniteID,
		Credential: v.Credential,
		Password:   v.Password,
		GroupID:    v.GroupID,
	}
	if v.LastDisplayName.Valid {
		e.LastDisplayName = &v.LastDisplayName.String
	}

	if v.LastIconUrl.Valid {
		e.LastIconUrl = &v.LastIconUrl.String
	}

	if v.CreatedBy.Valid {
		s := v.CreatedBy.String
		e.CreatedBy = &s
	}

	return e
}

func (u *HeadlessAccountUsecase) ListHeadlessAccounts(ctx context.Context) ([]*entity.HeadlessAccount, error) {
	list, err := u.queries.ListHeadlessAccounts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := make([]*entity.HeadlessAccount, 0, len(list))
	for _, v := range list {
		res = append(res, headlessAccountToEntity(v))
	}

	return res, nil
}

type ListHeadlessAccountsPagedResult struct {
	Accounts   []*entity.HeadlessAccount
	TotalCount int32
}

// ListHeadlessAccountsPagedOptions is the page-and-filter spec.
// GroupIDs follows the same semantics as port.HostListPageOptions.GroupIDs:
//   - nil:          全グループ対象 (上位レイヤで認可済み / system:group.list 等)
//   - 空 slice:     マッチゼロ件 (= 所属グループが無いユーザーの自動絞り込み結果)
//   - 非空 slice:   指定 group_id 群でのみ絞り込み
type ListHeadlessAccountsPagedOptions struct {
	PageIndex int32
	PageSize  int32
	GroupIDs  []string
}

func (u *HeadlessAccountUsecase) ListHeadlessAccountsPaged(ctx context.Context, opts ListHeadlessAccountsPagedOptions) (*ListHeadlessAccountsPagedResult, error) {
	rows, err := u.queries.ListHeadlessAccountsPaged(ctx, db.ListHeadlessAccountsPagedParams{
		PageOffset: opts.PageIndex * opts.PageSize,
		PageSize:   opts.PageSize,
		GroupIds:   opts.GroupIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := &ListHeadlessAccountsPagedResult{
		Accounts: make([]*entity.HeadlessAccount, 0, len(rows)),
	}
	if len(rows) > 0 {
		result.TotalCount = int32(rows[0].TotalCount) //nolint:gosec // G115: total_count はテーブル件数で int32 範囲を超えない
	}

	for _, row := range rows {
		result.Accounts = append(result.Accounts, headlessAccountToEntity(row.HeadlessAccount))
	}

	return result, nil
}

func (u *HeadlessAccountUsecase) GetHeadlessAccount(ctx context.Context, resoniteID string) (*entity.HeadlessAccount, error) {
	v, err := u.queries.GetHeadlessAccount(ctx, resoniteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}

		return nil, errors.Wrap(err, 0)
	}

	return headlessAccountToEntity(v), nil
}

func (u *HeadlessAccountUsecase) DeleteHeadlessAccount(ctx context.Context, resoniteID string) error {
	if err := u.requireAccountWrite(ctx, resoniteID); err != nil {
		return err
	}

	return u.queries.DeleteHeadlessAccount(ctx, resoniteID)
}

func (u *HeadlessAccountUsecase) RefetchHeadlessAccountInfo(ctx context.Context, resoniteID string) error {
	if err := u.requireAccountWrite(ctx, resoniteID); err != nil {
		return err
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, resoniteID)
	if err != nil {
		return errors.Errorf("failed to fetch user info: %w", err)
	}

	return u.queries.UpdateAccountInfo(ctx, db.UpdateAccountInfoParams{
		ResoniteID:      resoniteID,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
		LastIconUrl:     pgtype.Text{String: userInfo.IconUrl, Valid: true},
	})
}

// UpdateHeadlessAccountIcon updates the headless account's profile icon
// It processes the image, uploads it to Resonite cloud, and updates the profile.
func (u *HeadlessAccountUsecase) UpdateHeadlessAccountIcon(ctx context.Context, resoniteID string, iconData []byte) (string, error) {
	if err := u.requireAccountWrite(ctx, resoniteID); err != nil {
		return "", err
	}

	// Get account credentials from DB
	account, err := u.queries.GetHeadlessAccount(ctx, resoniteID)
	if err != nil {
		return "", errors.Errorf("failed to get headless account: %w", err)
	}

	// Process the image (crop to square, resize to 256x256, convert to PNG)
	processedData, err := skyfrost.ProcessIconImage(iconData)
	if err != nil {
		return "", errors.Errorf("failed to process icon image: %w", err)
	}

	// Upload the image to Resonite cloud as a texture record
	_, iconUrl, err := u.skyfrostClient.UploadTextureRecord(ctx, account.Credential, account.Password, "Profile Icon", "Inventory", processedData)
	if err != nil {
		return "", errors.Errorf("failed to upload icon: %w", err)
	}

	// Update the user profile with new icon URL
	profile := &skyfrost.UserProfile{
		IconUrl: iconUrl,
	}
	if err := u.skyfrostClient.UpdateUserProfile(ctx, account.Credential, account.Password, profile); err != nil {
		return "", errors.Errorf("failed to update profile: %w", err)
	}

	// Update the DB with new icon URL
	if err := u.queries.UpdateAccountIconUrl(ctx, db.UpdateAccountIconUrlParams{
		ResoniteID:  resoniteID,
		LastIconUrl: pgtype.Text{String: iconUrl, Valid: true},
	}); err != nil {
		return "", errors.Errorf("failed to update account info in DB: %w", err)
	}

	return iconUrl, nil
}

// requireAccountWrite は resoniteID の account.group_id を引いて
// account:write を要求する. account が存在しなければ domain.ErrNotFound を返す.
func (u *HeadlessAccountUsecase) requireAccountWrite(ctx context.Context, resoniteID string) error {
	account, err := u.GetHeadlessAccount(ctx, resoniteID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return u.permUC.RequirePermissionForGroup(ctx, account.GroupID, entity.PermKey_AccountWrite)
}

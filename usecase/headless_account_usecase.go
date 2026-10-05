package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUniqueViolation は PostgreSQL の unique_violation (SQLSTATE).
const pgUniqueViolation = "23505"

var (
	// ErrHeadlessAccountAlreadyExists は同一グループに同じ Resonite アカウントを重複登録しようとした.
	ErrHeadlessAccountAlreadyExists = errors.New("headless account is already registered in this group")
	// ErrHeadlessAccountGroupAmbiguous は group_id 未指定のアカウント指定が複数グループの登録に該当した.
	ErrHeadlessAccountGroupAmbiguous = errors.New("headless account is registered in multiple groups; group_id is required")
	// ErrInvalidAccountRegistration は Resonite アカウント新規登録の入力が不正.
	ErrInvalidAccountRegistration = errors.New("invalid account registration")
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

// CreateHeadlessAccount は既存の Resonite アカウントを groupID のヘッドレスアカウントとして追加する.
// iconData を指定した場合はアイコンを設定してから追加する.
func (u *HeadlessAccountUsecase) CreateHeadlessAccount(ctx context.Context, credential, password, groupID string, iconData []byte, createdBy *string) error {
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

	iconUrl := userInfo.IconUrl
	if len(iconData) > 0 {
		iconUrl, err = u.uploadIcon(ctx, credential, password, iconData)
		if err != nil {
			return err
		}
	}

	createdByText := pgtype.Text{}
	if createdBy != nil {
		createdByText = pgtype.Text{String: *createdBy, Valid: true}
	}

	err = u.queries.CreateHeadlessAccount(ctx, db.CreateHeadlessAccountParams{
		ResoniteID:      userSession.UserId,
		Credential:      credential,
		Password:        password,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
		LastIconUrl:     pgtype.Text{String: iconUrl, Valid: true},
		GroupID:         groupID,
		CreatedBy:       createdByText,
	})
	if err != nil {
		// アカウントは (group_id, resonite_id) で一意. 別グループへの登録は重複にならない.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return errors.Wrap(ErrHeadlessAccountAlreadyExists, 0)
		}

		return errors.Wrap(err, 0)
	}

	return nil
}

// RegisterHeadlessAccount は Resonite アカウントを新規登録し、登録された Resonite ID を返す.
// dateOfBirth は YYYY-MM-DD. 未認証のアカウントはログインできないので、ヘッドレスアカウントへの
// 追加はメール認証後に CreateHeadlessAccount で行う. groupID は追加予定のグループで、権限チェックに使う.
func (u *HeadlessAccountUsecase) RegisterHeadlessAccount(ctx context.Context, username, email, password, dateOfBirth, groupID string) (string, error) {
	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_AccountWrite); err != nil {
		return "", err
	}

	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	dob, err := time.Parse(time.DateOnly, dateOfBirth)
	if err != nil {
		return "", errors.Errorf("%w: invalid date of birth", ErrInvalidAccountRegistration)
	}

	// Resonite に拒否される入力は送る前に弾く.
	if err := skyfrost.ValidateRegistration(username, email, password, dob, time.Now()); err != nil {
		return "", errors.Errorf("%w: %w", ErrInvalidAccountRegistration, err)
	}

	resoniteID, err := u.skyfrostClient.RegisterUser(ctx, username, email, password, dob)
	if err != nil {
		var apiErr *skyfrost.APIError
		if errors.As(err, &apiErr) && apiErr.IsClientError() {
			return "", errors.Errorf("%w: %w", ErrInvalidAccountRegistration, err)
		}

		if errors.Is(err, skyfrost.ErrRegistrationNotConfirmed) {
			return "", errors.Errorf("%w. if the account is created, add it as an existing account", err)
		}

		return "", errors.Errorf("failed to register resonite account: %w", err)
	}

	return resoniteID, nil
}

func (u *HeadlessAccountUsecase) UpdateHeadlessAccountCredentials(ctx context.Context, groupID, resoniteID, credential, password string) error {
	account, err := u.requireAccountWrite(ctx, groupID, resoniteID)
	if err != nil {
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
		GroupID:    account.GroupID,
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

// GetHeadlessAccount は groupID に登録された resoniteID のアカウントを引く.
// アカウントは (group_id, resonite_id) で一意で、同一 resoniteID を複数グループに登録できる.
//
// NOTE: このメソッドは意図的に権限チェックを行わない. RPC permission interceptor
// 自身が group_id 解決のために呼ぶほか、ホスト起動 / async job など「account:use
// のみ」の文脈からも呼ばれるため、ここで account:read を要求すると権限セマンティクス
// が壊れる. 戻り値には平文の認証情報が含まれるので、新規の呼び出し元は必ず呼び出し側
// で認可を済ませること (RPC handler なら interceptor 登録、usecase なら Require*).
func (u *HeadlessAccountUsecase) GetHeadlessAccount(ctx context.Context, groupID, resoniteID string) (*entity.HeadlessAccount, error) {
	v, err := u.queries.GetHeadlessAccount(ctx, db.GetHeadlessAccountParams{
		GroupID:    groupID,
		ResoniteID: resoniteID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}

		return nil, errors.Wrap(err, 0)
	}

	return headlessAccountToEntity(v), nil
}

// ResolveHeadlessAccount はリクエストで指定された (groupID, resoniteID) からアカウントを引く.
// groupID が空の場合は、resoniteID の登録先グループが 1 つに定まるときに限りそれを返す
// (複数グループに登録されていれば ErrHeadlessAccountGroupAmbiguous).
//
// GetHeadlessAccount と同じく権限チェックは行わない.
func (u *HeadlessAccountUsecase) ResolveHeadlessAccount(ctx context.Context, groupID, resoniteID string) (*entity.HeadlessAccount, error) {
	if groupID != "" {
		return u.GetHeadlessAccount(ctx, groupID, resoniteID)
	}

	list, err := u.queries.ListHeadlessAccountsByResoniteID(ctx, resoniteID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	switch len(list) {
	case 0:
		return nil, domain.ErrNotFound
	case 1:
		return headlessAccountToEntity(list[0]), nil
	default:
		return nil, errors.Wrap(ErrHeadlessAccountGroupAmbiguous, 0)
	}
}

func (u *HeadlessAccountUsecase) DeleteHeadlessAccount(ctx context.Context, groupID, resoniteID string) error {
	account, err := u.requireAccountWrite(ctx, groupID, resoniteID)
	if err != nil {
		return err
	}

	return u.queries.DeleteHeadlessAccount(ctx, db.DeleteHeadlessAccountParams{
		GroupID:    account.GroupID,
		ResoniteID: resoniteID,
	})
}

func (u *HeadlessAccountUsecase) RefetchHeadlessAccountInfo(ctx context.Context, groupID, resoniteID string) error {
	account, err := u.requireAccountWrite(ctx, groupID, resoniteID)
	if err != nil {
		return err
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, resoniteID)
	if err != nil {
		return errors.Errorf("failed to fetch user info: %w", err)
	}

	return u.queries.UpdateAccountInfo(ctx, db.UpdateAccountInfoParams{
		GroupID:         account.GroupID,
		ResoniteID:      resoniteID,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
		LastIconUrl:     pgtype.Text{String: userInfo.IconUrl, Valid: true},
	})
}

// UpdateHeadlessAccountIcon updates the headless account's profile icon
// It processes the image, uploads it to Resonite cloud, and updates the profile.
func (u *HeadlessAccountUsecase) UpdateHeadlessAccountIcon(ctx context.Context, groupID, resoniteID string, iconData []byte) (string, error) {
	account, err := u.requireAccountWrite(ctx, groupID, resoniteID)
	if err != nil {
		return "", err
	}

	iconUrl, err := u.uploadIcon(ctx, account.Credential, account.Password, iconData)
	if err != nil {
		return "", err
	}

	// Update the DB with new icon URL
	if err := u.queries.UpdateAccountIconUrl(ctx, db.UpdateAccountIconUrlParams{
		GroupID:     account.GroupID,
		ResoniteID:  resoniteID,
		LastIconUrl: pgtype.Text{String: iconUrl, Valid: true},
	}); err != nil {
		return "", errors.Errorf("failed to update account info in DB: %w", err)
	}

	return iconUrl, nil
}

// requireAccountWrite は (groupID, resoniteID) の account を解決し、その group_id に
// 対して account:write を要求する. account が存在しなければ domain.ErrNotFound を返す.
func (u *HeadlessAccountUsecase) requireAccountWrite(ctx context.Context, groupID, resoniteID string) (*entity.HeadlessAccount, error) {
	account, err := u.ResolveHeadlessAccount(ctx, groupID, resoniteID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, account.GroupID, entity.PermKey_AccountWrite); err != nil {
		return nil, err
	}

	return account, nil
}

// uploadIcon はアイコン画像を Resonite にアップロードしてプロフィールに設定し、アイコンの URL を返す.
func (u *HeadlessAccountUsecase) uploadIcon(ctx context.Context, credential, password string, iconData []byte) (string, error) {
	// Process the image (crop to square, resize to 256x256, convert to PNG)
	processedData, err := skyfrost.ProcessIconImage(iconData)
	if err != nil {
		return "", errors.Errorf("failed to process icon image: %w", err)
	}

	// Upload the image to Resonite cloud as a texture record
	_, iconUrl, err := u.skyfrostClient.UploadTextureRecord(ctx, credential, password, "Profile Icon", "Inventory", processedData)
	if err != nil {
		return "", errors.Errorf("failed to upload icon: %w", err)
	}

	// Update the user profile with new icon URL
	profile := &skyfrost.UserProfile{
		IconUrl: iconUrl,
	}
	if err := u.skyfrostClient.UpdateUserProfile(ctx, credential, password, profile); err != nil {
		return "", errors.Errorf("failed to update profile: %w", err)
	}

	return iconUrl, nil
}

package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	registrationTokenLength = 32
	registrationTokenTTL    = 24 * time.Hour
	minPasswordLength       = 8
)

type UserUsecase struct {
	queries        *db.Queries
	pool           *pgxpool.Pool
	skyfrostClient skyfrost.Client
	guc            *GroupUsecase
	permUC         *PermissionUsecase
}

func NewUserUsecase(queries *db.Queries, pool *pgxpool.Pool, skyfrostClient skyfrost.Client, guc *GroupUsecase, permUC *PermissionUsecase) *UserUsecase {
	return &UserUsecase{
		queries:        queries,
		pool:           pool,
		skyfrostClient: skyfrostClient,
		guc:            guc,
		permUC:         permUC,
	}
}

// CreateUser はユーザーアカウントを新規作成し personal グループ + メンバーシップを自動投入する.
// personalRoleID が空文字なら seed-admin を付与する.
// 権限要件: system:user.create (CLI / 管理 RPC).
func (u *UserUsecase) CreateUser(ctx context.Context, id, password, resoniteId, personalRoleID string) error {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemUserCreate); err != nil {
		return err
	}

	if id == domain.SystemUserID {
		return errors.New("user id 'system' is reserved")
	}

	// personal role の検証は user 行を書き込む前に済ませる. CreateUser は
	// 非トランザクションで user → personal group の順に書くため、後段で弾くと
	// personal group を持たない user 行だけが残り CLI で修復不能になる.
	if personalRoleID != "" {
		if err := u.validatePersonalRole(ctx, personalRoleID); err != nil {
			return err
		}
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	iconUrl := pgtype.Text{Valid: false}
	if userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, resoniteId); err == nil && userInfo.IconUrl != "" {
		iconUrl = pgtype.Text{String: userInfo.IconUrl, Valid: true}
	}

	if err := u.queries.CreateUser(ctx, db.CreateUserParams{
		ID:         id,
		Password:   passwordHash,
		ResoniteID: pgtype.Text{String: resoniteId, Valid: true},
		IconUrl:    iconUrl,
	}); err != nil {
		return errors.Wrap(err, 0)
	}

	if u.guc != nil {
		if _, err := u.guc.EnsurePersonalGroupForUser(ctx, id, personalRoleID); err != nil {
			return errors.WrapPrefix(err, "ensure personal group", 0)
		}
	}

	return nil
}

func (u *UserUsecase) GetUserWithPassword(ctx context.Context, id, password string) (*db.User, error) {
	// system ユーザーはパスワード経由でログイン不可. password=空文字で投入されている
	// ため bcrypt.CompareHashAndPassword は失敗するが、念のため明示的に弾く.
	if id == domain.SystemUserID {
		return nil, errors.New("invalid credentials")
	}

	user, err := u.queries.GetUser(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	err = auth.ComparePasswordAndHash(password, user.Password)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return &user, nil
}

// DeleteUser は指定 user_id のユーザーを削除する.
// 権限要件: system:user.delete.
func (u *UserUsecase) DeleteUser(ctx context.Context, id string) error {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemUserDelete); err != nil {
		return err
	}

	if id == domain.SystemUserID {
		return errors.New("system user cannot be deleted")
	}

	// 削除前に存在チェック (NotFound を区別したいため). DELETE は冪等だが
	// API 観点では存在しない user_id への delete は 404 を返したい.
	if _, err := u.queries.GetUser(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return errors.Wrap(err, 0)
	}

	if err := u.queries.DeleteUser(ctx, id); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

// CreateRegistrationToken creates a registration token for the given resonite ID.
// The token is valid for 24 hours.
// personalRoleID が空文字なら seed-admin が付与される (RegisterWithToken 実行時).
// 権限要件: system:user.create (CLI / 管理 RPC).
func (u *UserUsecase) CreateRegistrationToken(ctx context.Context, resoniteId, personalRoleID string) (string, error) {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemUserCreate); err != nil {
		return "", err
	}

	token, _, err := u.createRegistrationToken(ctx, resoniteId, personalRoleID)

	return token, err
}

// RegistrationTokenWithInfo は CreateRegistrationTokenWithInfo の戻り値.
// RPC ハンドラで招待リンク表示用に Resonite ユーザー情報も含めて返す.
type RegistrationTokenWithInfo struct {
	Token            string
	ExpiresAt        time.Time
	ResoniteUserName string
	IconUrl          string
}

// CreateRegistrationTokenWithInfo は Resonite ID の有効性を skyfrost で検証してから
// 登録トークンを発行し、Resonite ユーザー情報と合わせて返す.
// Resonite ID が不正な場合はトークンを発行せずエラーを返す.
// personalRoleID が空文字なら seed-admin が付与される (RegisterWithToken 実行時).
// 権限要件: system:user.create.
func (u *UserUsecase) CreateRegistrationTokenWithInfo(ctx context.Context, resoniteId, personalRoleID string) (*RegistrationTokenWithInfo, error) {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemUserCreate); err != nil {
		return nil, err
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, resoniteId)
	if err != nil {
		return nil, errors.WrapPrefix(err, "invalid resonite id", 0)
	}

	token, expiresAt, err := u.createRegistrationToken(ctx, resoniteId, personalRoleID)
	if err != nil {
		return nil, err
	}

	return &RegistrationTokenWithInfo{
		Token:            token,
		ExpiresAt:        expiresAt,
		ResoniteUserName: userInfo.UserName,
		IconUrl:          userInfo.IconUrl,
	}, nil
}

//nolint:funcorder // CreateRegistrationToken / WithInfo のヘルパー. 直下に置く方が読みやすい
func (u *UserUsecase) createRegistrationToken(ctx context.Context, resoniteId, personalRoleID string) (string, time.Time, error) {
	// 発行時に personal role を検証する. 不正な role id / scope 違反 /
	// グループ内ロールを招待に載せられないようにする (登録時の FK エラーや
	// 無効な membership を防ぐ).
	if personalRoleID != "" {
		if err := u.validatePersonalRole(ctx, personalRoleID); err != nil {
			return "", time.Time{}, err
		}
	}

	token, err := generateSecureToken(registrationTokenLength)
	if err != nil {
		return "", time.Time{}, errors.Wrap(err, 0)
	}

	expiresAt := time.Now().Add(registrationTokenTTL)

	personalRole := pgtype.Text{Valid: false}
	if personalRoleID != "" {
		personalRole = pgtype.Text{String: personalRoleID, Valid: true}
	}

	err = u.queries.CreateRegistrationToken(ctx, db.CreateRegistrationTokenParams{
		Token:          hashRegistrationToken(token),
		ResoniteID:     resoniteId,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
		PersonalRoleID: personalRole,
	})
	if err != nil {
		return "", time.Time{}, errors.Wrap(err, 0)
	}

	return token, expiresAt, nil
}

// ListUsers は全ユーザーを id 昇順で返す. (認証済みなら誰でも呼べる).
func (u *UserUsecase) ListUsers(ctx context.Context) ([]db.User, error) {
	users, err := u.queries.ListUsers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return users, nil
}

// GetUser は id でユーザーを引く. 存在しない場合は domain.ErrNotFound を返す.
func (u *UserUsecase) GetUser(ctx context.Context, id string) (*db.User, error) {
	user, err := u.queries.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}

		return nil, errors.Wrap(err, 0)
	}

	return &user, nil
}

// ValidateRegistrationToken checks if the token is valid and returns the associated resonite ID and user info.
func (u *UserUsecase) ValidateRegistrationToken(ctx context.Context, token string) (*skyfrost.UserInfo, error) {
	regToken, err := u.queries.GetValidRegistrationToken(ctx, hashRegistrationToken(token))
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, regToken.ResoniteID)
	if err != nil {
		// ユーザー情報が取得できなくてもResonite IDだけで返す
		//nolint:nilerr // intentional: return partial result with only ResoniteID
		return &skyfrost.UserInfo{
			ID: regToken.ResoniteID,
		}, nil
	}

	return userInfo, nil
}

// RegisterWithToken registers a new user using a registration token.
// CreateUser + personal group 作成 + MarkRegistrationTokenUsed は単一トランザクション
// で実行する. 途中で失敗すれば全てロールバックされ、orphan user / 未使用 token /
// 個人グループ無しユーザー といった不整合状態を残さない.
// personal グループに付与するロールは registration_tokens.personal_role_id から取得する
// (改竄不能、admin が発行時に指定). NULL なら seed-admin.
func (u *UserUsecase) RegisterWithToken(ctx context.Context, token, userId, password string) (*db.User, error) {
	tokenHash := hashRegistrationToken(token)

	// tx 外: token 検証 (read-only). validation 失敗時は副作用ゼロで早期 return.
	// single-use の最終保証は tx 内の MarkRegistrationTokenUsed (条件付き UPDATE) が持つ.
	regToken, err := u.queries.GetValidRegistrationToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	iconUrl := pgtype.Text{Valid: false}
	if userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, regToken.ResoniteID); err == nil && userInfo.IconUrl != "" {
		iconUrl = pgtype.Text{String: userInfo.IconUrl, Valid: true}
	}

	var created db.User

	err = db.RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		qtx := u.queries.WithTx(tx)

		if err := qtx.CreateUser(ctx, db.CreateUserParams{
			ID:         userId,
			Password:   passwordHash,
			ResoniteID: pgtype.Text{String: regToken.ResoniteID, Valid: true},
			IconUrl:    iconUrl,
		}); err != nil {
			return errors.Wrap(err, 0)
		}

		// personal group + member は EnsurePersonalGroupForUser と同じ logic を inline する.
		// GroupUsecase は tx を受け取らない設計なので、tx-safe にするためにここでは
		// qtx 直接呼びにする.
		personalGroupID := userId + "-personal"
		if _, err := qtx.CreateGroup(ctx, db.CreateGroupParams{
			ID:   personalGroupID,
			Name: personalGroupID,
			Type: string(entity.GroupType_Personal),
		}); err != nil {
			return errors.WrapPrefix(err, "create personal group", 0)
		}

		roleID := entity.SeedRoleID_Admin
		if regToken.PersonalRoleID.Valid && regToken.PersonalRoleID.String != "" {
			roleID = regToken.PersonalRoleID.String
		}

		if _, err := qtx.AddGroupMember(ctx, db.AddGroupMemberParams{
			GroupID: personalGroupID,
			UserID:  userId,
			RoleID:  roleID,
			AddedBy: pgtype.Text{Valid: false},
		}); err != nil {
			return errors.WrapPrefix(err, "register personal member", 0)
		}

		// used_at IS NULL 条件付き UPDATE で single-use をアトミックに保証する.
		// 並行登録では先勝ちし、後続は 0 rows でここで abort する.
		rows, err := qtx.MarkRegistrationTokenUsed(ctx, tokenHash)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if rows != 1 {
			return errors.New("registration token is already used")
		}

		u, err := qtx.GetUser(ctx, userId)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		created = u

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashRegistrationToken は登録トークンの保存用ダイジェスト (SHA-256 hex) を返す.
// DB には平文トークンを置かず、URL で渡ってきた平文をハッシュして照合する
// (DB read-only 漏洩時にトークンをそのまま使われないようにするため).
func hashRegistrationToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

func (u *UserUsecase) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := u.queries.GetUser(ctx, userID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if err := auth.ComparePasswordAndHash(currentPassword, user.Password); err != nil {
		return errors.New("現在のパスワードが正しくありません")
	}

	if len(newPassword) < minPasswordLength {
		return errors.New(fmt.Sprintf("パスワードは%d文字以上である必要があります", minPasswordLength))
	}

	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return u.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:       userID,
		Password: passwordHash,
	})
}

// validatePersonalRole は招待 / ユーザー作成時に指定される personal role が
// 割り当て可能かを検証する. GroupUsecase が未配線 (guc == nil) の場合は検証を
// スキップせず fail-closed でエラーにする — 検証の無効化を「たまたま nil だった」
// で握り潰すと権限昇格ロールを載せた招待が通ってしまうため.
func (u *UserUsecase) validatePersonalRole(ctx context.Context, personalRoleID string) error {
	if u.guc == nil {
		return errors.New("cannot validate personal role: group usecase is not configured")
	}

	return u.guc.ValidatePersonalRoleAssignable(ctx, personalRoleID)
}

package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserUsecase struct {
	queries        *db.Queries
	skyfrostClient skyfrost.Client
}

func NewUserUsecase(queries *db.Queries, skyfrostClient skyfrost.Client) *UserUsecase {
	return &UserUsecase{
		queries:        queries,
		skyfrostClient: skyfrostClient,
	}
}

func (u *UserUsecase) CreateUser(ctx context.Context, id, password, resoniteId string) error {
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	iconUrl := pgtype.Text{Valid: false}
	if userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, resoniteId); err == nil && userInfo.IconUrl != "" {
		iconUrl = pgtype.Text{String: userInfo.IconUrl, Valid: true}
	}

	return u.queries.CreateUser(ctx, db.CreateUserParams{
		ID:         id,
		Password:   passwordHash,
		ResoniteID: pgtype.Text{String: resoniteId, Valid: true},
		IconUrl:    iconUrl,
	})
}

func (u *UserUsecase) GetUserWithPassword(ctx context.Context, id, password string) (*db.User, error) {
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

func (u *UserUsecase) DeleteUser(ctx context.Context, id string) error {
	return u.queries.DeleteUser(ctx, id)
}

// CreateRegistrationToken creates a registration token for the given resonite ID.
// The token is valid for 24 hours.
func (u *UserUsecase) CreateRegistrationToken(ctx context.Context, resoniteId string) (string, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	err = u.queries.CreateRegistrationToken(ctx, db.CreateRegistrationTokenParams{
		Token:      token,
		ResoniteID: resoniteId,
		ExpiresAt:  pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return token, nil
}

// ValidateRegistrationToken checks if the token is valid and returns the associated resonite ID.
func (u *UserUsecase) ValidateRegistrationToken(ctx context.Context, token string) (string, error) {
	regToken, err := u.queries.GetValidRegistrationToken(ctx, token)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return regToken.ResoniteID, nil
}

// RegisterWithToken registers a new user using a registration token.
func (u *UserUsecase) RegisterWithToken(ctx context.Context, token, userId, password string) (*db.User, error) {
	// Validate the token
	regToken, err := u.queries.GetValidRegistrationToken(ctx, token)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// Create the user
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	iconUrl := pgtype.Text{Valid: false}
	if userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, regToken.ResoniteID); err == nil && userInfo.IconUrl != "" {
		iconUrl = pgtype.Text{String: userInfo.IconUrl, Valid: true}
	}

	err = u.queries.CreateUser(ctx, db.CreateUserParams{
		ID:         userId,
		Password:   passwordHash,
		ResoniteID: pgtype.Text{String: regToken.ResoniteID, Valid: true},
		IconUrl:    iconUrl,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// Mark the token as used
	err = u.queries.MarkRegistrationTokenUsed(ctx, token)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// Get and return the created user
	user, err := u.queries.GetUser(ctx, userId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return &user, nil
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (u *UserUsecase) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := u.queries.GetUser(ctx, userID)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	if err := auth.ComparePasswordAndHash(currentPassword, user.Password); err != nil {
		return errors.New("現在のパスワードが正しくありません")
	}
	if len(newPassword) < 8 {
		return errors.New("パスワードは8文字以上である必要があります")
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

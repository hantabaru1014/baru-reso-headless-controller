package usecase

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5/pgtype"
)

type HeadlessAccountUsecase struct {
	queries        *db.Queries
	skyfrostClient skyfrost.Client
}

func NewHeadlessAccountUsecase(queries *db.Queries, skyfrostClient skyfrost.Client) *HeadlessAccountUsecase {
	return &HeadlessAccountUsecase{
		queries:        queries,
		skyfrostClient: skyfrostClient,
	}
}

func (u *HeadlessAccountUsecase) CreateHeadlessAccount(ctx context.Context, credential, password string) error {
	userSession, err := u.skyfrostClient.UserLogin(ctx, credential, password)
	if err != nil {
		return errors.Errorf("failed to login: %w", err)
	}
	userInfo, err := u.skyfrostClient.FetchUserInfo(ctx, userSession.UserId)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return u.queries.CreateHeadlessAccount(ctx, db.CreateHeadlessAccountParams{
		ResoniteID:      userSession.UserId,
		Credential:      credential,
		Password:        password,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
		LastIconUrl:     pgtype.Text{String: userInfo.IconUrl, Valid: true},
	})
}

func (u *HeadlessAccountUsecase) UpdateHeadlessAccountCredentials(ctx context.Context, resoniteID, credential, password string) error {
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

func (u *HeadlessAccountUsecase) ListHeadlessAccounts(ctx context.Context) ([]*entity.HeadlessAccount, error) {
	list, err := u.queries.ListHeadlessAccounts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := make([]*entity.HeadlessAccount, 0, len(list))
	for _, v := range list {
		e := entity.HeadlessAccount{
			ResoniteID: v.ResoniteID,
			Credential: v.Credential,
			Password:   v.Password,
		}
		if v.LastDisplayName.Valid {
			e.LastDisplayName = &v.LastDisplayName.String
		}
		if v.LastIconUrl.Valid {
			e.LastIconUrl = &v.LastIconUrl.String
		}
		res = append(res, &e)
	}

	return res, nil
}

func (u *HeadlessAccountUsecase) GetHeadlessAccount(ctx context.Context, resoniteID string) (*entity.HeadlessAccount, error) {
	v, err := u.queries.GetHeadlessAccount(ctx, resoniteID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	e := entity.HeadlessAccount{
		ResoniteID: v.ResoniteID,
		Credential: v.Credential,
		Password:   v.Password,
	}
	if v.LastDisplayName.Valid {
		e.LastDisplayName = &v.LastDisplayName.String
	}
	if v.LastIconUrl.Valid {
		e.LastIconUrl = &v.LastIconUrl.String
	}

	return &e, nil
}

func (u *HeadlessAccountUsecase) DeleteHeadlessAccount(ctx context.Context, resoniteID string) error {
	return u.queries.DeleteHeadlessAccount(ctx, resoniteID)
}

func (u *HeadlessAccountUsecase) RefetchHeadlessAccountInfo(ctx context.Context, resoniteID string) error {
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
// It processes the image, uploads it to Resonite cloud, and updates the profile
func (u *HeadlessAccountUsecase) UpdateHeadlessAccountIcon(ctx context.Context, resoniteID string, iconData []byte) (string, error) {
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

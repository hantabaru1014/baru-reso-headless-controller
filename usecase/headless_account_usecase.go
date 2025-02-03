package usecase

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5/pgtype"
)

type HeadlessAccountUsecase struct {
	queries *db.Queries
}

func NewHeadlessAccountUsecase(queries *db.Queries) *HeadlessAccountUsecase {
	return &HeadlessAccountUsecase{
		queries: queries,
	}
}

func (u *HeadlessAccountUsecase) CreateHeadlessAccount(ctx context.Context, resoniteID, credential, password string) error {
	userInfo, err := skyfrost.FetchUserInfo(ctx, resoniteID)
	if err != nil {
		return err
	}
	return u.queries.CreateHeadlessAccount(ctx, db.CreateHeadlessAccountParams{
		ResoniteID:      resoniteID,
		Credential:      credential,
		Password:        password,
		LastDisplayName: pgtype.Text{String: userInfo.UserName, Valid: true},
	})
}

func (u *HeadlessAccountUsecase) ListHeadlessAccounts(ctx context.Context) ([]*entity.HeadlessAccount, error) {
	list, err := u.queries.ListHeadlessAccounts(ctx)
	if err != nil {
		return nil, err
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
		return nil, err
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

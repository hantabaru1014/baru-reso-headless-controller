package usecase

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserUsecase struct {
	queries *db.Queries
}

func NewUserUsecase(queries *db.Queries) *UserUsecase {
	return &UserUsecase{
		queries: queries,
	}
}

func (u *UserUsecase) CreateUser(ctx context.Context, id, password, resoniteId string) error {
	return u.queries.CreateUser(ctx, db.CreateUserParams{
		ID:         id,
		Password:   password,
		ResoniteID: pgtype.Text{String: resoniteId},
		IconUrl:    pgtype.Text{String: ""},
	})
}

func (u *UserUsecase) GetUserWithPassword(ctx context.Context, id, password string) (*db.User, error) {
	user, err := u.queries.GetUserWithPassword(ctx, db.GetUserWithPasswordParams{
		ID:       id,
		Password: password,
	})
	if err != nil {
		return nil, err
	}

	return &user, nil
}

package usecase

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
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
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return u.queries.CreateUser(ctx, db.CreateUserParams{
		ID:         id,
		Password:   passwordHash,
		ResoniteID: pgtype.Text{String: resoniteId, Valid: true},
		IconUrl:    pgtype.Text{Valid: false},
	})
}

func (u *UserUsecase) GetUserWithPassword(ctx context.Context, id, password string) (*db.User, error) {
	user, err := u.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	err = auth.ComparePasswordAndHash(password, user.Password)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *UserUsecase) DeleteUser(ctx context.Context, id string) error {
	return u.queries.DeleteUser(ctx, id)
}

package usecase

import (
	"context"

	"github.com/go-errors/errors"
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
		return errors.Wrap(err, 0)
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

func (u *UserUsecase) UpdatePassword(ctx context.Context, id, currentPassword, newPassword string) error {
	// 現在のパスワードを検証
	_, err := u.GetUserWithPassword(ctx, id, currentPassword)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	// 新しいパスワードをハッシュ化して更新
	newPasswordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return u.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:       id,
		Password: newPasswordHash,
	})
}

package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/jackc/pgx/v5"
)

// UserExistenceChecker は worker.UserExistenceChecker の実装で、
// *db.Queries.GetUser を薄くラップして存在確認だけ返す.
//
// 用途: worker (async_job / scheduled_op) が job claim 後、handler を呼ぶ前に
// created_by の user が依然として DB に存在することを確認するため.
type UserExistenceChecker struct {
	q *db.Queries
}

func NewUserExistenceChecker(q *db.Queries) *UserExistenceChecker {
	return &UserExistenceChecker{q: q}
}

// UserExistsByID は users テーブルに userID が存在するか判定する.
// 存在: (true, nil) / 不在: (false, nil) / DB エラー: (false, err).
func (c *UserExistenceChecker) UserExistsByID(ctx context.Context, userID string) (bool, error) {
	if _, err := c.q.GetUser(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, errors.Wrap(err, 0)
	}

	return true, nil
}

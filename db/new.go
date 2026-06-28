package db

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewConnPool は config からプールを作って返す. wire provider として使う.
func NewConnPool(cfg *config.DatabaseConfig) *pgxpool.Pool {
	ctx := context.Background()

	dbpool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		panic(err)
	}
	// TODO: conn.closeをちゃんと呼ぶ
	// defer conn.Close(ctx)

	return dbpool
}

// NewQueriesFromPool は既存プールから Queries を作る. wire provider 用.
func NewQueriesFromPool(pool *pgxpool.Pool) *Queries {
	return New(pool)
}

func NewQueries(cfg *config.DatabaseConfig) *Queries {
	return New(NewConnPool(cfg))
}

// RunInTx は fn を pgx の transaction でラップして実行する. fn が error を返すと
// rollback、nil なら commit. fn 内では Queries.WithTx(tx) で transaction-scoped な
// Queries を生成して使う.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)

		return err
	}

	return tx.Commit(ctx)
}

package db

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewQueries(cfg *config.DatabaseConfig) *Queries {
	ctx := context.Background()

	dbpool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		panic(err)
	}
	// TODO: conn.closeをちゃんと呼ぶ
	// defer conn.Close(ctx)

	return New(dbpool)
}

package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewQueries() *Queries {
	ctx := context.Background()

	dbpool, err := pgxpool.New(ctx, os.Getenv("DB_URL"))
	if err != nil {
		panic(err)
	}
	// TODO: conn.closeをちゃんと呼ぶ
	// defer conn.Close(ctx)

	return New(dbpool)
}

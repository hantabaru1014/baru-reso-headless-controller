package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5"
)

func NewQueries() *Queries {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, os.Getenv("DB_CONNECTION_STRING"))
	if err != nil {
		panic(err)
	}
	// TODO: conn.closeをちゃんと呼ぶ
	// defer conn.Close(ctx)

	return New(conn)
}

package main

import (
	"context"
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
)

func main() {
	ctx := context.Background()
	q := db.NewQueries()
	dc := hostconnector.NewDockerHostConnector()
	repo := adapter.NewHeadlessHostRepository(q, dc)
	host, err := repo.Find(ctx, os.Args[1])
	if err != nil {
		panic(err)
	}
	println("configJson:", host.StartupConfig)
}

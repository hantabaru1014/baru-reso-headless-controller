//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

func InitializeServer() *Server {
	wire.Build(
		// db
		db.NewQueries,

		// repository
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,

		// usecase
		usecase.NewHeadlessHostUsecase,
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,

		// controller
		rpc.NewUserService,
		rpc.NewControllerService,

		NewServer,
	)
	return &Server{}
}

func InitializeCli() *Cli {
	wire.Build(
		// db
		db.NewQueries,

		// usecase
		usecase.NewUserUsecase,

		NewCli,
	)
	return &Cli{}
}

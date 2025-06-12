//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
)

func InitializeServer() *Server {
	wire.Build(
		// db
		db.NewQueries,

		// host connector
		hostconnector.NewDockerHostConnector,

		// repository
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,
		wire.Bind(new(port.SessionRepository), new(*adapter.SessionRepository)),
		adapter.NewSessionRepository,

		// worker
		worker.NewImageChecker,

		// usecase
		usecase.NewHeadlessHostUsecase,
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,

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

		// host connector
		hostconnector.NewDockerHostConnector,

		// repository
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,
		wire.Bind(new(port.SessionRepository), new(*adapter.SessionRepository)),
		adapter.NewSessionRepository,

		// usecase
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,
		usecase.NewHeadlessHostUsecase,

		NewCli,
	)
	return &Cli{}
}

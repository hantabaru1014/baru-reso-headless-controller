//go:build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
)

// Config providers
func ProvideDatabaseConfig(cfg *config.EnvConfig) *config.DatabaseConfig {
	return &cfg.Database
}

func ProvideDockerConfig(cfg *config.EnvConfig) *config.DockerConfig {
	return &cfg.Docker
}

func ProvideGRPCConfig(cfg *config.EnvConfig) *config.GRPCConfig {
	return &cfg.GRPC
}

func ProvideWorkerConfig(cfg *config.EnvConfig) *config.WorkerConfig {
	return &cfg.Worker
}

func ProvideServerConfig(cfg *config.EnvConfig) *config.ServerConfig {
	return &cfg.Server
}

var ConfigSet = wire.NewSet(
	ProvideDatabaseConfig,
	ProvideDockerConfig,
	ProvideGRPCConfig,
	ProvideWorkerConfig,
	ProvideServerConfig,
)

func InitializeServer(cfg *config.EnvConfig) *Server {
	wire.Build(
		// config providers
		ConfigSet,

		// db
		db.NewQueries,

		// host connector
		hostconnector.NewDockerHostConnector,
		wire.Bind(new(hostconnector.HostConnector), new(*hostconnector.DockerHostConnector)),

		// skyfrost client
		skyfrost.NewDefaultClient,
		wire.Bind(new(skyfrost.Client), new(*skyfrost.DefaultClient)),

		// repository
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,
		wire.Bind(new(port.SessionRepository), new(*adapter.SessionRepository)),
		adapter.NewSessionRepository,

		// worker
		worker.NewImageChecker,
		worker.NewEventWatcher,

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

func InitializeCli(cfg *config.EnvConfig) *Cli {
	wire.Build(
		// config providers
		ConfigSet,

		// db
		db.NewQueries,

		// host connector
		hostconnector.NewDockerHostConnector,
		wire.Bind(new(hostconnector.HostConnector), new(*hostconnector.DockerHostConnector)),

		// skyfrost client
		skyfrost.NewDefaultClient,
		wire.Bind(new(skyfrost.Client), new(*skyfrost.DefaultClient)),

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

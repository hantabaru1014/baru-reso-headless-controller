//go:build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/resonitelink"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/blobstore"
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

func ProvideRustFSConfig(cfg *config.EnvConfig) *config.RustFSConfig {
	return &cfg.RustFS
}

func ProvideResoniteLinkConfig(cfg *config.EnvConfig) *config.ResoniteLinkConfig {
	return &cfg.ResoniteLink
}

// ProvideWorkerRunners groups the concrete background workers that the
// server supervises into a slice the worker.Manager consumes. Add new
// workers here when introducing one.
func ProvideWorkerRunners(
	imageChecker *worker.ImageChecker,
	dockerEventWatcher *worker.DockerEventWatcher,
	hostEventWatcher *worker.HostEventWatcher,
) []worker.Runner {
	return []worker.Runner{
		imageChecker,
		dockerEventWatcher,
		hostEventWatcher,
	}
}

// ProvideHostEventHandlers gathers consumers for the per-host event
// streams. Order matters: the logging handler runs last so other handlers'
// DB writes are visible by the time we log the event.
func ProvideHostEventHandlers(
	sessionStateSyncHandler *worker.SessionStateSyncHandler,
	loggingHandler *worker.LoggingHostEventHandler,
) []worker.HostEventHandler {
	return []worker.HostEventHandler{sessionStateSyncHandler, loggingHandler}
}

var ConfigSet = wire.NewSet(
	ProvideDatabaseConfig,
	ProvideDockerConfig,
	ProvideGRPCConfig,
	ProvideWorkerConfig,
	ProvideServerConfig,
	ProvideRustFSConfig,
	ProvideResoniteLinkConfig,
)

func InitializeServer(cfg *config.EnvConfig) (*Server, error) {
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

		// blob store
		blobstore.NewMinioClient,
		wire.Bind(new(blobstore.Client), new(*blobstore.MinioClient)),

		// repository
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,
		wire.Bind(new(port.SessionRepository), new(*adapter.SessionRepository)),
		adapter.NewSessionRepository,

		// worker
		worker.NewImageChecker,
		worker.NewDockerEventWatcher,
		worker.NewHostEventWatcher,
		worker.NewSQLHostEventStore,
		wire.Bind(new(worker.HostEventStore), new(*worker.SQLHostEventStore)),
		worker.NewLoggingHostEventHandler,
		worker.NewSessionStateSyncHandler,
		ProvideHostEventHandlers,
		ProvideWorkerRunners,
		worker.NewManager,

		// usecase
		usecase.NewHeadlessHostUsecase,
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,
		usecase.NewBlobUsecase,

		// controller
		rpc.NewUserService,
		rpc.NewControllerService,

		// resonite link bridge
		resonitelink.NewBridge,

		NewServer,
	)
	return nil, nil
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

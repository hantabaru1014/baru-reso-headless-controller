//go:build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/resonitelink"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/sessionstate"
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

// ProvideWorkerManager groups the concrete background workers AND
// performs two post-construction links that wire itself cannot express:
//   - The orchestrator needs a SessionStopper (SessionUsecase), but
//     SessionUsecase needs a HostDrainer (the orchestrator). Wire can
//     pick only one direction at construction time; we close the cycle
//     by setting the stopper here, after both ends exist.
//   - The orchestrator subscribes to ImageChecker so registry polling
//     happens in exactly one place.
func ProvideWorkerManager(
	imageChecker *worker.ImageChecker,
	dockerEventWatcher *worker.DockerEventWatcher,
	hostEventWatcher *worker.HostEventWatcher,
	upgradeOrchestrator *worker.HostUpgradeOrchestrator,
	scheduledOpExecutor *worker.ScheduledOperationExecutor,
	sessionStopper port.SessionStopper,
) *worker.Manager {
	upgradeOrchestrator.SetSessionStopper(sessionStopper)
	imageChecker.Subscribe(upgradeOrchestrator.OnNewImage)

	return worker.NewManager([]worker.Runner{
		imageChecker,
		dockerEventWatcher,
		hostEventWatcher,
		upgradeOrchestrator,
		scheduledOpExecutor,
	})
}

// ProvideScheduledOperationExecutor は scheduled session operation worker を
// 構築する. SessionUsecase をそのまま SessionOperator として渡し、interface
// 経由で worker パッケージから usecase パッケージへの依存を切る.
func ProvideScheduledOperationExecutor(
	repo port.ScheduledSessionOperationRepository,
	suc *usecase.SessionUsecase,
	srepo port.SessionRepository,
	stateCache port.SessionStateCache,
) *worker.ScheduledOperationExecutor {
	return worker.NewScheduledOperationExecutor(repo, suc, srepo, stateCache, worker.ScheduledOperationExecutorOptions{})
}

// ProvideHostEventHandlers gathers consumers for the per-host event
// streams. Order matters: the logging handler runs last so other handlers'
// DB writes are visible by the time we log the event.
func ProvideHostEventHandlers(
	sessionStateSyncHandler *worker.SessionStateSyncHandler,
	sessionLifecycleHandler *worker.SessionLifecycleHandler,
	upgradeOrchestrator *worker.HostUpgradeOrchestrator,
	loggingHandler *worker.LoggingHostEventHandler,
) []worker.HostEventHandler {
	return []worker.HostEventHandler{sessionStateSyncHandler, sessionLifecycleHandler, upgradeOrchestrator, loggingHandler}
}

// ProvideHeadlessAccountFetcher exposes HeadlessAccountUsecase under the
// orchestrator's narrow interface so the worker package does not have to
// depend on the entire usecase package.
func ProvideHeadlessAccountFetcher(u *usecase.HeadlessAccountUsecase) worker.HeadlessAccountFetcher {
	return u
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
		wire.Bind(new(port.ScheduledSessionOperationRepository), new(*adapter.ScheduledSessionOperationRepository)),
		adapter.NewScheduledSessionOperationRepository,

		// in-memory session-state cache (volatile snapshot owned by container)
		sessionstate.NewMemoryCache,
		wire.Bind(new(port.SessionStateCache), new(*sessionstate.MemoryCache)),

		// worker
		worker.NewImageChecker,
		worker.NewDockerEventWatcher,
		worker.NewHostEventWatcher,
		worker.NewSQLHostEventStore,
		wire.Bind(new(worker.HostEventStore), new(*worker.SQLHostEventStore)),
		worker.NewLoggingHostEventHandler,
		worker.NewSessionStateSyncHandler,
		worker.NewSessionLifecycleHandler,
		worker.NewHostUpgradeOrchestrator,
		wire.Bind(new(port.HostDrainer), new(*worker.HostUpgradeOrchestrator)),
		ProvideScheduledOperationExecutor,
		ProvideHeadlessAccountFetcher,
		ProvideHostEventHandlers,
		ProvideWorkerManager,

		// usecase
		usecase.NewHeadlessHostUsecase,
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,
		usecase.NewBlobUsecase,
		usecase.NewScheduledSessionOperationUsecase,
		wire.Bind(new(port.SessionStopper), new(*usecase.SessionUsecase)),

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
		wire.Bind(new(port.ScheduledSessionOperationRepository), new(*adapter.ScheduledSessionOperationRepository)),
		adapter.NewScheduledSessionOperationRepository,

		// CLI has no upgrade orchestrator running, so SessionUsecase
		// gets a no-op drainer.
		wire.Struct(new(port.NoopHostDrainer)),
		wire.Bind(new(port.HostDrainer), new(port.NoopHostDrainer)),

		// in-memory session-state cache (cli は通常 session を起動しないが、
		// SessionUsecase の constructor 依存を満たすために bind だけする)
		sessionstate.NewMemoryCache,
		wire.Bind(new(port.SessionStateCache), new(*sessionstate.MemoryCache)),

		// usecase
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,
		usecase.NewHeadlessHostUsecase,
		usecase.NewScheduledSessionOperationUsecase,

		NewCli,
	)
	return &Cli{}
}

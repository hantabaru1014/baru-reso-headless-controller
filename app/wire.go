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
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/image_builder"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
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

func ProvideResoniteBuildConfig(cfg *config.EnvConfig) *config.ResoniteBuildConfig {
	return &cfg.ResoniteBuild
}

// ProvideWorkerManager groups the concrete background workers AND
// performs two post-construction links that wire itself cannot express:
//   - The orchestrator needs a SessionStopper (SessionUsecase), but
//     SessionUsecase needs a HostDrainer (the orchestrator). Wire can
//     pick only one direction at construction time; we close the cycle
//     by setting the stopper here, after both ends exist.
//   - The orchestrator subscribes to ResoniteVersionUsecase's build-success
//     stream so a freshly-built image can trigger draining of RUNNING
//     auto-update hosts.
func ProvideWorkerManager(
	contentPoller *worker.ContentPoller,
	dockerEventWatcher *worker.DockerEventWatcher,
	hostEventWatcher *worker.HostEventWatcher,
	upgradeOrchestrator *worker.HostUpgradeOrchestrator,
	scheduledOpExecutor *worker.ScheduledOperationExecutor,
	asyncJobExecutor *worker.AsyncJobExecutor,
	sessionStopper port.SessionStopper,
	rvuc *usecase.ResoniteVersionUsecase,
) *worker.Manager {
	upgradeOrchestrator.SetSessionStopper(sessionStopper)
	rvuc.Subscribe(upgradeOrchestrator.OnNewImage)

	return worker.NewManager([]worker.Runner{
		contentPoller,
		dockerEventWatcher,
		hostEventWatcher,
		upgradeOrchestrator,
		scheduledOpExecutor,
		asyncJobExecutor,
	})
}

// ProvideContentPoller は worker.ContentPoller に必要な narrow interface を
// ResoniteVersionUsecase / async_job.Usecase から供給する.
func ProvideContentPoller(
	rvuc *usecase.ResoniteVersionUsecase,
	ajuc *async_job.Usecase,
	cfg *config.WorkerConfig,
) *worker.ContentPoller {
	return worker.NewContentPoller(rvuc, ajuc, cfg)
}

// ProvideImageBuildOperator は async_job.Dispatcher に ImageBuildOperator を
// 供給する. ResoniteVersionUsecase.RunBuild を裏で呼ぶ.
func ProvideImageBuildOperator(rvuc *usecase.ResoniteVersionUsecase) async_job.ImageBuildOperator {
	return rvuc
}

// ProvideScheduledOperationExecutor は scheduled session operation worker を
// 構築する. SessionUsecase をそのまま SessionOperator として渡し、interface
// 経由で worker パッケージから usecase パッケージへの依存を切る.
func ProvideScheduledOperationExecutor(
	repo port.ScheduledSessionOperationRepository,
	suc *usecase.SessionUsecase,
	srepo port.SessionRepository,
	stateCache port.SessionStateCache,
	userChecker worker.UserExistenceChecker,
) *worker.ScheduledOperationExecutor {
	return worker.NewScheduledOperationExecutor(repo, suc, srepo, stateCache, userChecker, worker.ScheduledOperationExecutorOptions{})
}

// ProvideAsyncJobDispatcher はホスト/セッションの非同期 job を実行する dispatcher を
// 構築する. HeadlessHostUsecase / SessionUsecase / HeadlessAccountUsecase をそれぞれ
// narrow operator として渡し、worker パッケージから usecase パッケージへの直接依存を切る.
func ProvideAsyncJobDispatcher(
	hhuc *usecase.HeadlessHostUsecase,
	suc *usecase.SessionUsecase,
	hauc *usecase.HeadlessAccountUsecase,
	imgOp async_job.ImageBuildOperator,
	ajuc *async_job.Usecase,
) *async_job.Dispatcher {
	return async_job.NewDispatcher(hhuc, suc, hauc, imgOp, ajuc)
}

// ProvideAsyncJobExecutor は AsyncJobExecutor worker を構築する.
func ProvideAsyncJobExecutor(
	repo port.AsyncJobRepository,
	dispatcher *async_job.Dispatcher,
	bus notification.Bus,
	userChecker worker.UserExistenceChecker,
) *worker.AsyncJobExecutor {
	return worker.NewAsyncJobExecutor(repo, dispatcher, bus, userChecker, worker.AsyncJobExecutorOptions{})
}

// ProvideHostEventHandlers gathers consumers for the per-host event
// streams. Order matters:
//   - DB-mutating handlers (state sync, lifecycle, upgrade orchestrator) run
//     first so the DB reflects the new state.
//   - NotificationDispatcher runs after those so frontend clients that
//     re-fetch on receipt of the notification get the post-mutation rows.
//   - LoggingHostEventHandler runs last so log lines reflect what all the
//     other handlers actually saw.
func ProvideHostEventHandlers(
	sessionStateSyncHandler *worker.SessionStateSyncHandler,
	sessionLifecycleHandler *worker.SessionLifecycleHandler,
	upgradeOrchestrator *worker.HostUpgradeOrchestrator,
	notificationDispatcher *worker.NotificationDispatcher,
	loggingHandler *worker.LoggingHostEventHandler,
) []worker.HostEventHandler {
	return []worker.HostEventHandler{sessionStateSyncHandler, sessionLifecycleHandler, upgradeOrchestrator, notificationDispatcher, loggingHandler}
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
	ProvideResoniteBuildConfig,
)

func InitializeServer(cfg *config.EnvConfig) (*Server, error) {
	wire.Build(
		// config providers
		ConfigSet,

		// db
		db.NewConnPool,
		db.NewQueriesFromPool,

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
		wire.Bind(new(port.AsyncJobRepository), new(*adapter.AsyncJobRepository)),
		adapter.NewAsyncJobRepository,
		wire.Bind(new(port.ResoniteVersionRepository), new(*adapter.ResoniteVersionRepository)),
		adapter.NewResoniteVersionRepository,
		wire.Bind(new(worker.UserExistenceChecker), new(*adapter.UserExistenceChecker)),
		adapter.NewUserExistenceChecker,
		wire.Bind(new(port.GroupRepository), new(*adapter.GroupRepository)),
		adapter.NewGroupRepository,
		wire.Bind(new(port.RoleRepository), new(*adapter.RoleRepository)),
		adapter.NewRoleRepository,
		wire.Bind(new(port.GroupMemberRepository), new(*adapter.GroupMemberRepository)),
		adapter.NewGroupMemberRepository,
		wire.Bind(new(port.MessageRepository), new(*adapter.MessageRepository)),
		adapter.NewMessageRepository,

		// in-memory session-state cache (volatile snapshot owned by container)
		sessionstate.NewMemoryCache,
		wire.Bind(new(port.SessionStateCache), new(*sessionstate.MemoryCache)),

		// in-memory notification bus (volatile pub/sub for frontend push)
		notification.NewBus,
		wire.Bind(new(notification.Bus), new(*notification.MemoryBus)),

		// image builder
		image_builder.NewBuilder,

		// worker
		ProvideContentPoller,
		worker.NewDockerEventWatcher,
		worker.NewHostEventWatcher,
		worker.NewSQLHostEventStore,
		wire.Bind(new(worker.HostEventStore), new(*worker.SQLHostEventStore)),
		worker.NewLoggingHostEventHandler,
		worker.NewSessionStateSyncHandler,
		worker.NewSessionLifecycleHandler,
		worker.NewHostUpgradeOrchestrator,
		worker.NewNotificationDispatcher,
		wire.Bind(new(port.HostDrainer), new(*worker.HostUpgradeOrchestrator)),
		ProvideScheduledOperationExecutor,
		ProvideAsyncJobDispatcher,
		ProvideAsyncJobExecutor,
		ProvideImageBuildOperator,
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
		usecase.NewPermissionUsecase,
		usecase.NewGroupUsecase,
		usecase.NewRoleUsecase,
		usecase.NewMessageUsecase,
		usecase.NewResoniteVersionUsecase,
		async_job.NewUsecase,
		wire.Bind(new(port.SessionStopper), new(*usecase.SessionUsecase)),

		// controller
		rpc.NewUserService,
		rpc.NewControllerService,
		rpc.NewNotificationService,
		rpc.NewGroupService,
		rpc.NewRoleService,
		rpc.NewMessageService,

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
		db.NewConnPool,
		db.NewQueriesFromPool,

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
		wire.Bind(new(port.GroupRepository), new(*adapter.GroupRepository)),
		adapter.NewGroupRepository,
		wire.Bind(new(port.RoleRepository), new(*adapter.RoleRepository)),
		adapter.NewRoleRepository,
		wire.Bind(new(port.GroupMemberRepository), new(*adapter.GroupMemberRepository)),
		adapter.NewGroupMemberRepository,

		// CLI has no upgrade orchestrator running, so SessionUsecase
		// gets a no-op drainer.
		wire.Struct(new(port.NoopHostDrainer)),
		wire.Bind(new(port.HostDrainer), new(port.NoopHostDrainer)),

		// in-memory session-state cache (cli は通常 session を起動しないが、
		// SessionUsecase の constructor 依存を満たすために bind だけする)
		sessionstate.NewMemoryCache,
		wire.Bind(new(port.SessionStateCache), new(*sessionstate.MemoryCache)),

		// resonite version + image builder (CLI 用にも wire する。CLI 経由の起動でも
		// 使う可能性があるため。)
		wire.Bind(new(port.ResoniteVersionRepository), new(*adapter.ResoniteVersionRepository)),
		adapter.NewResoniteVersionRepository,
		image_builder.NewBuilder,
		usecase.NewResoniteVersionUsecase,

		// usecase
		usecase.NewUserUsecase,
		usecase.NewHeadlessAccountUsecase,
		usecase.NewSessionUsecase,
		usecase.NewHeadlessHostUsecase,
		usecase.NewScheduledSessionOperationUsecase,
		usecase.NewPermissionUsecase,
		usecase.NewGroupUsecase,

		NewCli,
	)
	return &Cli{}
}

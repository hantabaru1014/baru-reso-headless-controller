package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	hostconnectormock "github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector/mock"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/sessionstate"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	blobstoremock "github.com/hantabaru1014/baru-reso-headless-controller/lib/blobstore/mock"
	skyfrostmock "github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost/mock"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type controllerServiceTestSetup struct {
	service           *ControllerService
	ctrl              *gomock.Controller
	mockHostConnector *hostconnectormock.MockHostConnector
	mockSkyfrost      *skyfrostmock.MockClient
	mockRpcClient     *hostconnectormock.MockHeadlessControlServiceClient
	mockBlobstore     *blobstoremock.MockClient
	queries           *db.Queries
	pool              *pgxpool.Pool
}

func (s *controllerServiceTestSetup) Cleanup() {
	s.ctrl.Finish()
}

func setupControllerServiceTest(t *testing.T) *controllerServiceTestSetup {
	t.Helper()

	// Initialize auth with test secret
	auth.Init("test-jwt-secret-for-testing")

	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	// permission interceptor を素通りさせるため、デフォルト test user を system-admin
	// として bootstrap. 個別 user で動かしたいテストは別途 SetupSystemAdminUser を
	// 呼び直すこと.
	testutil.SetupDefaultSystemAdminUser(t, queries)

	// Setup mocks for external dependencies only
	ctrl := gomock.NewController(t)
	mockHostConnector := hostconnectormock.NewMockHostConnector(ctrl)
	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)
	mockRpcClient := hostconnectormock.NewMockHeadlessControlServiceClient(ctrl)
	mockBlobstore := blobstoremock.NewMockClient(ctrl)

	// Load env
	cfg, err := config.LoadEnvConfig()
	require.NoError(t, err)
	err = cfg.Validate()
	require.NoError(t, err)

	// Setup repositories with real implementations
	srepo := adapter.NewSessionRepository(queries)
	hhrepo := adapter.NewHeadlessHostRepository(queries, mockHostConnector, &cfg.GRPC)
	stateCache := sessionstate.NewMemoryCache()
	groupRepo := adapter.NewGroupRepository(queries)
	roleRepo := adapter.NewRoleRepository(queries)
	memberRepo := adapter.NewGroupMemberRepository(queries)

	// permUC を usecase 群より先に作る必要がある (PermissionUsecase 引数に渡すため).
	permUC := usecase.NewPermissionUsecase(groupRepo, memberRepo, roleRepo)

	// Setup usecases with real repositories
	hauc := usecase.NewHeadlessAccountUsecase(queries, mockSkyfrost, permUC)
	suc := usecase.NewSessionUsecase(srepo, hhrepo, port.NoopHostDrainer{}, stateCache, &cfg.Server, &cfg.ResoniteLink, permUC)
	hhuc := usecase.NewHeadlessHostUsecase(hhrepo, srepo, suc, hauc, permUC)
	buc := usecase.NewBlobUsecase(srepo, hhrepo, mockBlobstore)
	sorepo := adapter.NewScheduledSessionOperationRepository(queries)
	souc := usecase.NewScheduledSessionOperationUsecase(sorepo, hhrepo, srepo, permUC)
	ajrepo := adapter.NewAsyncJobRepository(queries)
	ajuc := async_job.NewUsecase(ajrepo)

	// Setup service with real repositories
	service := NewControllerService(hhrepo, srepo, hhuc, hauc, suc, buc, souc, ajuc, permUC, groupRepo, roleRepo, mockSkyfrost, notification.NewBus())

	return &controllerServiceTestSetup{
		service:           service,
		ctrl:              ctrl,
		mockHostConnector: mockHostConnector,
		mockSkyfrost:      mockSkyfrost,
		mockRpcClient:     mockRpcClient,
		mockBlobstore:     mockBlobstore,
		queries:           queries,
		pool:              pool,
	}
}

// authAsMinPerm は「permKeys ちょうどしか持たないユーザー」を作り、そのユーザーで
// 認証された connect.Request を返す helper. groupID には対象リソースが所属する
// グループを渡す. 「最小権限ちょうど」での RPC 成功 / 「1 perm 不足」での
// PermissionDenied を簡潔に書くためのもの.
//
// userID はテスト関数内で一意な値を渡す (subtest 間で同じ ID を使い回すと
// CreateTestUser が衝突するため、各 subtest で新規 setup を使う前提).
func authAsMinPerm[T any](t *testing.T, queries *db.Queries, msg *T, userID, groupID string, permKeys []string) *connect.Request[T] {
	t.Helper()

	testutil.SetupUserWithExactPermissions(t, queries, userID, groupID, permKeys)

	return testutil.CreateAuthenticatedRequest(t, msg, userID, "U-resonite-"+userID, "")
}

func setupAuthenticatedClient(t *testing.T, service *ControllerService) hdlctrlv1connect.ControllerServiceClient {
	t.Helper()

	server := testutil.SetupAuthenticatedHTTPServer(t, service)
	t.Cleanup(server.Close)

	return hdlctrlv1connect.NewControllerServiceClient(
		server.Client(),
		server.URL,
	)
}

func TestControllerService_Authentication(t *testing.T) {
	t.Run("失敗: 認証トークンなしでRPCメソッドを呼び出し", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Try to call a method without authentication
		req := connect.NewRequest(&hdlctrlv1.ListHeadlessAccountsRequest{})

		_, err := client.ListHeadlessAccounts(t.Context(), req)
		require.Error(t, err)

		// Verify it's an authentication error
		connectErr := new(connect.Error)
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "token required")
		assert.NotEmpty(t, connectErr.Meta().Get("WWW-Authenticate"))
	})

	t.Run("失敗: 無効なトークンでRPCメソッドを呼び出し", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Try to call with invalid token
		req := connect.NewRequest(&hdlctrlv1.ListHeadlessAccountsRequest{})
		req.Header().Set("Authorization", "Bearer invalid_token_here")

		_, err := client.ListHeadlessAccounts(t.Context(), req)
		require.Error(t, err)

		// Verify it's an authentication error
		connectErr := new(connect.Error)
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "token")
		assert.NotEmpty(t, connectErr.Meta().Get("WWW-Authenticate"))
	})

	t.Run("成功: 有効なトークンでRPCメソッドを呼び出し", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test1", "user1@example.test", "password1")

		// Create authenticated request
		req := testutil.CreateAuthenticatedRequest(
			t,
			&hdlctrlv1.ListHeadlessAccountsRequest{},
			"test@example.test",
			"U-test123",
			"https://example.test/icon.png",
		)

		res, err := client.ListHeadlessAccounts(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify success response header
		assert.NotEmpty(t, res.Header().Get("WWW-Authenticate"))
	})

	t.Run("失敗: 別のメソッドでも認証が必要", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Test another method to ensure all methods require auth
		req := connect.NewRequest(&hdlctrlv1.ListHeadlessHostRequest{})

		_, err := client.ListHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := new(connect.Error)
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: セッション操作でも認証が必要", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Test session-related method
		req := connect.NewRequest(&hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
		})

		_, err := client.SearchSessions(t.Context(), req)
		require.Error(t, err)

		connectErr := new(connect.Error)
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})
}

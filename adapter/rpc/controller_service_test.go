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

	// Setup usecases with real repositories
	hauc := usecase.NewHeadlessAccountUsecase(queries, mockSkyfrost)
	suc := usecase.NewSessionUsecase(srepo, hhrepo, port.NoopHostDrainer{}, stateCache, &cfg.Server, &cfg.ResoniteLink)
	hhuc := usecase.NewHeadlessHostUsecase(hhrepo, srepo, suc, hauc)
	buc := usecase.NewBlobUsecase(srepo, hhrepo, mockBlobstore)
	sorepo := adapter.NewScheduledSessionOperationRepository(queries)
	souc := usecase.NewScheduledSessionOperationUsecase(sorepo)

	// Setup service with real repositories
	service := NewControllerService(hhrepo, srepo, hhuc, hauc, suc, buc, souc, mockSkyfrost, notification.NewBus())

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

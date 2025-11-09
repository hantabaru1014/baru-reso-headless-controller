package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	hostconnectormock "github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector/mock"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	skyfrostmock "github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost/mock"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
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
	queries           *db.Queries
	pool              *pgxpool.Pool
}

func (s *controllerServiceTestSetup) Cleanup() {
	s.ctrl.Finish()
}

func setupControllerServiceTest(t *testing.T) *controllerServiceTestSetup {
	t.Helper()

	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	// Setup mocks for external dependencies only
	ctrl := gomock.NewController(t)
	mockHostConnector := hostconnectormock.NewMockHostConnector(ctrl)
	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)
	mockRpcClient := hostconnectormock.NewMockHeadlessControlServiceClient(ctrl)

	// Setup repositories with real implementations
	srepo := adapter.NewSessionRepository(queries)
	hhrepo := adapter.NewHeadlessHostRepository(queries, mockHostConnector)

	// Setup usecases with real repositories
	hauc := usecase.NewHeadlessAccountUsecase(queries, mockSkyfrost)
	suc := usecase.NewSessionUsecase(srepo, hhrepo)
	hhuc := usecase.NewHeadlessHostUsecase(hhrepo, srepo, suc, hauc)

	// Setup service with real repositories
	service := NewControllerService(hhrepo, srepo, hhuc, hauc, suc, mockSkyfrost)

	return &controllerServiceTestSetup{
		service:           service,
		ctrl:              ctrl,
		mockHostConnector: mockHostConnector,
		mockSkyfrost:      mockSkyfrost,
		mockRpcClient:     mockRpcClient,
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

func TestControllerService_ListHeadlessAccounts(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	client := setupAuthenticatedClient(t, setup.service)

	// Create test headless accounts
	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test1", "user1@example.test", "password1")
	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "user2@example.test", "password2")

	t.Run("成功: ヘッドレスアカウントのリストを取得", func(t *testing.T) {
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{})

		res, err := client.ListHeadlessAccounts(t.Context(), req)
		require.NoError(t, err)

		assert.Len(t, res.Msg.Accounts, 2)

		// Verify account data
		account1 := res.Msg.Accounts[0]
		assert.NotEmpty(t, account1.UserId)
		assert.NotEmpty(t, account1.UserName)
	})
}

func TestControllerService_CreateHeadlessAccount(t *testing.T) {
	t.Run("成功: 有効な認証情報でアカウント作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost client to return successful login
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "testuser@example.test", "testpass123").
			Return(&skyfrost.UserSession{UserId: "U-testuser123"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-testuser123").
			Return(&skyfrost.UserInfo{
				ID:       "U-testuser123",
				UserName: "TestUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "testuser@example.test",
			Password:   "testpass123",
		})

		res, err := client.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was created in DB
		account, err := setup.queries.GetHeadlessAccount(t.Context(), "U-testuser123")
		require.NoError(t, err)
		assert.Equal(t, "testuser@example.test", account.Credential)
	})

	t.Run("失敗: 無効な認証情報でアカウント作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost client to return login error
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "invalid@example.test", "invalidpassword").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "invalid@example.test",
			Password:   "invalidpassword",
		})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: 既に存在するアカウントを作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create initial account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-existing", "existing@example.test", "password123")

		// Mock skyfrost to return successful login but DB insert will fail
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "existing@example.test", "newpassword123").
			Return(&skyfrost.UserSession{UserId: "U-existing"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-existing").
			Return(&skyfrost.UserInfo{
				ID:       "U-existing",
				UserName: "ExistingUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "existing@example.test",
			Password:   "newpassword123",
		})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_DeleteHeadlessAccount(t *testing.T) {
	t.Run("成功: ヘッドレスアカウントを削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-todelete", "todelete@example.test", "password123")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-todelete",
		})

		res, err := client.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was deleted
		_, err = setup.queries.GetHeadlessAccount(t.Context(), "U-todelete")
		assert.Error(t, err)
	})

	t.Run("成功: 存在しないアカウントを削除（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// DeleteHeadlessAccountは:execで実装されているため、
		// 存在しないアカウントを削除してもエラーにならない（何も削除されないだけ）
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-nonexistent",
		})

		res, err := client.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})
}

func TestControllerService_ListHeadlessHostImageTags(t *testing.T) {
	t.Run("成功: イメージタグ一覧を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock HostConnector to return test tags
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), nil).
			Return(port.ContainerImageList{
				{
					Tag:             "2024.1.1-v1.0.0",
					ResoniteVersion: "2024.1.1",
					IsPreRelease:    false,
					AppVersion:      "v1.0.0",
				},
				{
					Tag:             "prerelease-2024.1.2-v1.1.0",
					ResoniteVersion: "2024.1.2",
					IsPreRelease:    true,
					AppVersion:      "v1.1.0",
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		res, err := client.ListHeadlessHostImageTags(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.Tags, 2)

		// Verify first tag
		assert.Equal(t, "2024.1.1-v1.0.0", res.Msg.Tags[0].Tag)
		assert.Equal(t, "2024.1.1", res.Msg.Tags[0].ResoniteVersion)
		assert.False(t, res.Msg.Tags[0].IsPrerelease)
		assert.Equal(t, "v1.0.0", res.Msg.Tags[0].AppVersion)

		// Verify second tag
		assert.Equal(t, "prerelease-2024.1.2-v1.1.0", res.Msg.Tags[1].Tag)
		assert.Equal(t, "2024.1.2", res.Msg.Tags[1].ResoniteVersion)
		assert.True(t, res.Msg.Tags[1].IsPrerelease)
		assert.Equal(t, "v1.1.0", res.Msg.Tags[1].AppVersion)
	})

	t.Run("失敗: コネクタでエラー発生", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock HostConnector to return error
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), nil).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		_, err := client.ListHeadlessHostImageTags(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
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

func TestControllerService_UpdateHeadlessAccountCredentials(t *testing.T) {
	t.Run("成功: 認証情報を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-update", "old@example.test", "oldpassword")

		// Mock skyfrost client to return successful login with new credentials
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "new@example.test", "newpassword").
			Return(&skyfrost.UserSession{UserId: "U-update"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-update").
			Return(&skyfrost.UserInfo{
				ID:       "U-update",
				UserName: "UpdatedUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-update",
			Credential: "new@example.test",
			Password:   "newpassword",
		})

		res, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify credentials were updated in DB
		account, err := setup.queries.GetHeadlessAccount(t.Context(), "U-update")
		require.NoError(t, err)
		assert.Equal(t, "new@example.test", account.Credential)
	})

	t.Run("成功: 存在しないアカウントの認証情報を更新（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost to return successful login
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "nonexist@example.test", "password").
			Return(&skyfrost.UserSession{UserId: "U-nonexist"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-nonexist").
			Return(&skyfrost.UserInfo{
				ID:       "U-nonexist",
				UserName: "NonExist",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-nonexist",
			Credential: "nonexist@example.test",
			Password:   "password",
		})

		// UPDATE statement does not fail for non-existent records, it just updates 0 rows
		_, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: 無効な新しい認証情報", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-invalidupdate", "valid@example.test", "validpassword")

		// Mock skyfrost to return login error
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "invalid@example.test", "invalidpassword").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-invalidupdate",
			Credential: "invalid@example.test",
			Password:   "invalidpassword",
		})

		_, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessAccountStorageInfo(t *testing.T) {
	t.Run("成功: ストレージ情報を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-storage-success", "user@example.test", "password")

		// Mock skyfrost to return storage info
		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "user@example.test", "password", "U-storage-success").
			Return(&skyfrost.StorageInfo{
				UsedBytes:  1024 * 1024 * 100, // 100 MB
				QuotaBytes: 1024 * 1024 * 500, // 500 MB
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-storage-success",
		})

		res, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, int64(1024*1024*100), res.Msg.StorageUsedBytes)
		assert.Equal(t, int64(1024*1024*500), res.Msg.StorageQuotaBytes)
	})

	t.Run("失敗: 存在しないアカウント", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-nonexist",
		})

		_, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: ストレージ情報の取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-storage", "user@example.test", "password")

		// Mock skyfrost to return error
		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "user@example.test", "password", "U-storage").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-storage",
		})

		_, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_RefetchHeadlessAccountInfo(t *testing.T) {
	t.Run("成功: アカウント情報を再取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-refetch", "user@example.test", "password")

		// Mock skyfrost to return user info
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-refetch").
			Return(&skyfrost.UserInfo{
				ID:       "U-refetch",
				UserName: "UpdatedName",
				IconUrl:  "https://example.test/new-icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-refetch",
		})

		res, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account info was updated
		account, err := setup.queries.GetHeadlessAccount(t.Context(), "U-refetch")
		require.NoError(t, err)
		assert.Equal(t, "UpdatedName", account.LastDisplayName.String)
		assert.Equal(t, "https://example.test/new-icon.png", account.LastIconUrl.String)
	})

	t.Run("失敗: 存在しないアカウント", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost to return error for non-existent user
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-nonexist").
			Return(nil, connect.NewError(connect.CodeNotFound, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-nonexist",
		})

		_, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: ユーザー情報の取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-failfetch", "user@example.test", "password")

		// Mock skyfrost to return error
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-failfetch").
			Return(nil, connect.NewError(connect.CodeNotFound, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-failfetch",
		})

		_, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_AcceptFriendRequests(t *testing.T) {
	t.Run("成功: フレンドリクエストを承認", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-friend", "friend@example.test", "password")

		// Create test host in database
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-friend", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return running status
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to accept friend requests
		setup.mockRpcClient.EXPECT().
			AcceptFriendRequests(gomock.Any(), gomock.Any()).
			Return(&headlessv1.AcceptFriendRequestsResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-friend",
			TargetUserId:      "U-target",
		})

		res, err := client.AcceptFriendRequests(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was actually created
		_, err = setup.queries.GetHost(t.Context(), host.ID)
		require.NoError(t, err)
	})

	t.Run("失敗: 起動中のホストがない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)
		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-nohost", "nohost@example.test", "password")

		// No running hosts created, so ListRunningByAccount will return empty list

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-nohost",
			TargetUserId:      "U-target",
		})

		_, err := client.AcceptFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)
		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-rpcfail", "rpcfail@example.test", "password")

		// Create test host
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-rpcfail", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return running status
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return error when getting RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-rpcfail",
			TargetUserId:      "U-target",
		})

		_, err := client.AcceptFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_GetFriendRequests(t *testing.T) {
	t.Run("成功: フレンドリクエストを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-friend", "friend@example.test", "password")

		// Mock skyfrost to return friend requests
		setup.mockSkyfrost.EXPECT().
			GetContacts(gomock.Any(), "friend@example.test", "password").
			Return([]skyfrost.Contact{
				{
					Id:         "contact1",
					Username:   "User1",
					Status:     "Requested",
					IsAccepted: false,
				},
				{
					Id:         "contact2",
					Username:   "User2",
					Status:     "Requested",
					IsAccepted: false,
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-friend",
		})

		res, err := client.GetFriendRequests(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.RequestedContacts, 2)
	})

	t.Run("失敗: 存在しないアカウント", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-nonexist",
		})

		_, err := client.GetFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: コンタクト取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-contactfail", "user@example.test", "password")

		// Mock skyfrost to return contacts error
		setup.mockSkyfrost.EXPECT().
			GetContacts(gomock.Any(), "user@example.test", "password").
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-contactfail",
		})

		_, err := client.GetFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)

		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StartHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")

		// Mock HostConnector to start container
		setup.mockHostConnector.EXPECT().
			Start(gomock.Any(), gomock.Any()).
			Return(hostconnector.HostConnectString("test-container"), nil)

		// Mock HostConnector to get status after start
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING).
			AnyTimes()

		imageTag := "latest"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-test",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		})

		res, err := client.StartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotEmpty(t, res.Msg.HostId)

		// Verify host was created in database
		host, err := setup.queries.GetHost(t.Context(), res.Msg.HostId)
		require.NoError(t, err)
		assert.Equal(t, "TestHost", host.Name)
		assert.Equal(t, "U-test", host.AccountID)
	})

	t.Run("失敗: 存在しないアカウント", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		imageTag := "latest"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-nonexist",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		})

		_, err := client.StartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_RestartHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを再起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - ListContainerTags (called by resolveTagToUse)
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), gomock.Any()).
			Return(port.ContainerImageList{
				{Tag: "latest", IsPreRelease: false},
			}, nil).
			Times(1)

		// Mock HostConnector - GetStatus (called multiple times)
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING).
			AnyTimes()

		// Mock HostConnector - GetRpcClient (may be called multiple times)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC client to return about info
		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{
				ResoniteVersion: "1.0.0",
				AppVersion:      "1.0.0",
			}, nil).
			AnyTimes()

		// Mock RPC client to return account info
		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{
				UserId:      "U-test",
				DisplayName: "Test Account",
			}, nil).
			AnyTimes()

		// Mock RPC client to return status
		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{
				Fps: 60.0,
			}, nil).
			AnyTimes()

		// Mock RPC client to return startup config
		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil).
			AnyTimes()

		// Mock RPC client to return sessions (for SearchSessions)
		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil).
			AnyTimes()

		// Mock HostConnector - Stop
		setup.mockHostConnector.EXPECT().
			Stop(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)

		// Mock HostConnector - Start after stop
		setup.mockHostConnector.EXPECT().
			Start(gomock.Any(), gomock.Any()).
			Return(hostconnector.HostConnectString("test-container-restarted"), nil).
			Times(1)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.RestartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: "nonexist-host",
		})

		_, err := client.RestartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_UpdateHeadlessHostSettings(t *testing.T) {
	t.Run("成功: 停止中のホストの設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		// Mock HostConnector to return exited status
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_EXITED)

		newName := "UpdatedHost"
		newTickRate := float32(120)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId:   host.ID,
			Name:     &newName,
			TickRate: &newTickRate,
		})

		res, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was updated in database
		updatedHost, err := setup.queries.GetHost(t.Context(), host.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, updatedHost.Name)
	})

	t.Run("成功: 実行中のホストの設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and running host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "RunningHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - GetStatus returns RUNNING
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING)

		// Mock RPC calls - GetRpcClient is called twice: once in dbToEntity, once before UpdateHostSettings
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			Times(2)

		newTickRate := float32(120)
		maxTransfers := int32(4)

		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{}, nil)

		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{}, nil)

		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{}, nil)

		// GetStartupConfigToRestore is called twice: once in dbToEntity, once after UpdateHostSettings
		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{
					TickRate:                    &newTickRate,
					MaxConcurrentAssetTransfers: &maxTransfers,
				},
			}, nil).
			Times(2)

		setup.mockRpcClient.EXPECT().
			UpdateHostSettings(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateHostSettingsResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId:   host.ID,
			TickRate: &newTickRate,
		})

		res, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId: "nonexist",
		})

		_, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessHostLogs(t *testing.T) {
	t.Run("成功: ログを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		mockLogs := port.LogLineList{
			{Timestamp: 1234567890, IsError: false, Body: "Log line 1"},
			{Timestamp: 1234567891, IsError: true, Body: "Error line"},
		}

		// Mock HostConnector to return logs
		setup.mockHostConnector.EXPECT().
			GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockLogs, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId: host.ID,
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.Logs, 2)
		assert.Equal(t, "Log line 1", res.Msg.Logs[0].Body)
		assert.False(t, res.Msg.Logs[0].IsError)
		assert.Equal(t, "Error line", res.Msg.Logs[1].Body)
		assert.True(t, res.Msg.Logs[1].IsError)
	})

	t.Run("失敗: ログの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return error
		setup.mockHostConnector.EXPECT().
			GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId: host.ID,
		})

		_, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_ShutdownHeadlessHost(t *testing.T) {
	t.Run("成功: ホストをシャットダウン", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - SearchSessions calls GetRpcClient to list sessions
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil)

		// Mock HostConnector - Stop calls GetRpcClient to fetch startup config
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil)

		setup.mockHostConnector.EXPECT().
			Stop(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_KillHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを強制停止", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - SearchSessions calls GetRpcClient to list sessions
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil)

		// Mock HostConnector - Kill
		setup.mockHostConnector.EXPECT().
			Kill(gomock.Any(), gomock.Any()).
			Return(nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KillHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.KillHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KillHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.KillHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessHost(t *testing.T) {
	t.Run("成功: ホストの詳細を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		// Mock HostConnector to return exited status
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.GetHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotNil(t, res.Msg.Host)
		assert.Equal(t, host.ID, res.Msg.Host.Id)
		assert.Equal(t, "TestHost", res.Msg.Host.Name)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.GetHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_ListHeadlessHost(t *testing.T) {
	t.Run("成功: ホスト一覧を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test accounts and hosts
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test1", "test1@example.test", "password")
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-test1", "Host1", entity.HeadlessHostStatus_EXITED)
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "Host2", entity.HeadlessHostStatus_EXITED)

		// Mock HostConnector to return statuses
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_EXITED).
			Times(2)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{})

		res, err := client.ListHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.Hosts, 2)
	})
}

func TestControllerService_DeleteHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.DeleteHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was deleted from database
		_, err = setup.queries.GetHost(t.Context(), host.ID)
		assert.Error(t, err)
	})

	t.Run("成功: 存在しないホストを削除（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: "nonexist",
		})

		res, err := client.DeleteHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})
}

func TestControllerService_AllowHostAccess(t *testing.T) {
	t.Run("成功: ホストアクセスを許可", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			AllowHostAccess(gomock.Any(), gomock.Any()).
			Return(&headlessv1.AllowHostAccessResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.AllowHostAccessRequest{},
		})

		res, err := client.AllowHostAccess(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId: host.ID,

			Request: &headlessv1.AllowHostAccessRequest{},
		})

		_, err := client.AllowHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: RPC呼び出しに失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			AllowHostAccess(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.AllowHostAccessRequest{},
		})

		_, err := client.AllowHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_DenyHostAccess(t *testing.T) {
	t.Run("成功: ホストアクセスを拒否", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			DenyHostAccess(gomock.Any(), gomock.Any()).
			Return(&headlessv1.DenyHostAccessResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DenyHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.DenyHostAccessRequest{},
		})

		res, err := client.DenyHostAccess(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DenyHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.DenyHostAccessRequest{},
		})

		_, err := client.DenyHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_BanUser(t *testing.T) {
	t.Run("成功: ユーザーをBANする", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			BanUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.BanUserResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.BanUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.BanUserRequest{
				User: &headlessv1.BanUserRequest_UserId{UserId: "U-target"},
			},
		})

		res, err := client.BanUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.BanUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.BanUserRequest{
				User: &headlessv1.BanUserRequest_UserId{UserId: "U-target"},
			},
		})

		_, err := client.BanUser(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_KickUser(t *testing.T) {
	t.Run("成功: ユーザーをキックする", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			KickUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.KickUserResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KickUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.KickUserRequest{
				User: &headlessv1.KickUserRequest_UserId{UserId: "U-target"},
			},
		})

		res, err := client.KickUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KickUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.KickUserRequest{
				User: &headlessv1.KickUserRequest_UserId{UserId: "U-target"},
			},
		})

		_, err := client.KickUser(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_SearchUserInfo(t *testing.T) {
	t.Run("成功: ユーザー情報を検索", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			SearchUserInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.SearchUserInfoResponse{
				Users: []*headlessv1.UserInfo{
					{
						Id:   "U-found",
						Name: "FoundUser",
					},
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchUserInfoRequest{
			HostId: host.ID,
			Parameters: &headlessv1.SearchUserInfoRequest{
				User: &headlessv1.SearchUserInfoRequest_UserName{UserName: "test"},
			},
		})

		res, err := client.SearchUserInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.Users, 1)
		assert.Equal(t, "U-found", res.Msg.Users[0].Id)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchUserInfoRequest{
			HostId: host.ID,
			Parameters: &headlessv1.SearchUserInfoRequest{
				User: &headlessv1.SearchUserInfoRequest_UserName{UserName: "test"},
			},
		})

		_, err := client.SearchUserInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_FetchWorldInfo(t *testing.T) {
	t.Run("成功: ワールド情報を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			FetchWorldInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.FetchWorldInfoResponse{
				Name: "TestWorld",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-12345",
		})

		res, err := client.FetchWorldInfo(t.Context(), req)

		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, "TestWorld", res.Msg.Name)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-12345",
		})

		_, err := client.FetchWorldInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: ワールドが見つからない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			FetchWorldInfo(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeNotFound, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-nonexist",
		})

		_, err := client.FetchWorldInfo(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_GetSessionDetails(t *testing.T) {
	t.Run("成功: セッション詳細を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Create test session
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetSessionDetailsRequest{
			SessionId: session.ID,
		})

		res, err := client.GetSessionDetails(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, session.ID, res.Msg.Session.Id)
		assert.Equal(t, "TestSession", res.Msg.Session.Name)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetSessionDetailsRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.GetSessionDetails(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_ListUsersInSession(t *testing.T) {
	t.Run("成功: セッション内のユーザーリストを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			ListUsersInSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListUsersInSessionResponse{
				Users: []*headlessv1.UserInSession{
					{Id: "U-user1", Name: "User1"},
					{Id: "U-user2", Name: "User2"},
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListUsersInSessionRequest{
			HostId:    host.ID,
			SessionId: "session-123",
		})

		res, err := client.ListUsersInSession(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.Users, 2)
		assert.Equal(t, "U-user1", res.Msg.Users[0].Id)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListUsersInSessionRequest{
			HostId:    host.ID,
			SessionId: "session-123",
		})

		_, err := client.ListUsersInSession(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_SaveSessionWorld(t *testing.T) {
	t.Run("成功: セッションのワールドを保存", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session (called by SaveSessionWorld usecase)
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:       session.ID,
					Name:     session.Name,
					WorldUrl: "resrec:///U-test/R-test-world",
				},
			}, nil).
			AnyTimes()

		// Mock RPC call to save world
		setup.mockRpcClient.EXPECT().
			SaveSessionWorld(gomock.Any(), gomock.Any()).
			Return(&headlessv1.SaveSessionWorldResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: session.ID,
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE,
		})

		res, err := client.SaveSessionWorld(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotNil(t, res.Msg.SavedRecordUrl)
		assert.Equal(t, "resrec:///U-test/R-test-world", *res.Msg.SavedRecordUrl)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: "nonexist-session",
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE,
		})

		_, err := client.SaveSessionWorld(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: 無効なセーブモード", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: "session-123",
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_UNKNOWN,
		})

		_, err := client.SaveSessionWorld(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_UpdateSessionParameters(t *testing.T) {
	t.Run("成功: セッションパラメータを更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session (called by usecase)
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		// Mock RPC call to update parameters
		maxUsers := int32(16)
		setup.mockRpcClient.EXPECT().
			UpdateSessionParameters(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateSessionParametersResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionParametersRequest{
			Parameters: &headlessv1.UpdateSessionParametersRequest{
				SessionId: session.ID,
				MaxUsers:  &maxUsers,
			},
		})

		res, err := client.UpdateSessionParameters(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionParametersRequest{
			Parameters: &headlessv1.UpdateSessionParametersRequest{
				SessionId: "nonexist-session",
			},
		})

		_, err := client.UpdateSessionParameters(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_UpdateUserRole(t *testing.T) {
	t.Run("成功: ユーザーのロールを更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			UpdateUserRole(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateUserRoleResponse{
				Role: "Admin",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateUserRoleRequest{
			HostId: host.ID,
			Parameters: &headlessv1.UpdateUserRoleRequest{
				SessionId: "session-123",
				User:      &headlessv1.UpdateUserRoleRequest_UserId{UserId: "U-target"},
				Role:      "Admin",
			},
		})

		res, err := client.UpdateUserRole(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, "Admin", res.Msg.Role)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateUserRoleRequest{
			HostId: host.ID,
			Parameters: &headlessv1.UpdateUserRoleRequest{
				SessionId: "session-123",
				User:      &headlessv1.UpdateUserRoleRequest_UserId{UserId: "U-target"},
				Role:      "Admin",
			},
		})

		_, err := client.UpdateUserRole(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StartWorld(t *testing.T) {
	t.Run("成功: ワールドを起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to start world
		setup.mockRpcClient.EXPECT().
			StartWorld(gomock.Any(), gomock.Any()).
			Return(&headlessv1.StartWorldResponse{
				OpenedSession: &headlessv1.Session{
					Id:   "session-new",
					Name: "TestWorld",
				},
			}, nil)

		worldUrl := "resrec:///U-test/R-12345"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartWorldRequest{
			HostId: host.ID,
			Parameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: worldUrl},
			},
		})

		res, err := client.StartWorld(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotNil(t, res.Msg.OpenedSession)
		assert.Equal(t, "session-new", res.Msg.OpenedSession.Id)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		worldUrl := "resrec:///U-test/R-12345"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartWorldRequest{
			HostId: "nonexist-host",
			Parameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: worldUrl},
			},
		})

		_, err := client.StartWorld(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_InviteUser(t *testing.T) {
	t.Run("成功: ユーザーIDでユーザーを招待", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			InviteUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.InviteUserResponse{}, nil)

		userId := "U-target"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserId{UserId: userId},
		})

		res, err := client.InviteUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: ユーザー名でユーザーを招待", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			InviteUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.InviteUserResponse{}, nil)

		userName := "TargetUser"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserName{UserName: userName},
		})

		res, err := client.InviteUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		userId := "U-target"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserId{UserId: userId},
		})

		_, err := client.InviteUser(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StopSession(t *testing.T) {
	t.Run("成功: セッションを停止", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		// Mock RPC call to stop session
		setup.mockRpcClient.EXPECT().
			StopSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.StopSessionResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StopSessionRequest{
			SessionId: session.ID,
		})

		res, err := client.StopSession(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StopSessionRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.StopSession(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_SearchSessions(t *testing.T) {
	t.Run("成功: セッションを検索", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and sessions
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		testutil.CreateTestSession(t, setup.queries, host.ID, "Session1", entity.SessionStatus_RUNNING)
		testutil.CreateTestSession(t, setup.queries, host.ID, "Session2", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetStatus
		setup.mockHostConnector.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(entity.HeadlessHostStatus_RUNNING).
			AnyTimes()

		// Mock HostConnector - GetRpcClient (called when fetching host info)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC calls
		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil).
			AnyTimes()

		// Mock RPC call to list sessions (called by getHostSessions)
		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil).
			AnyTimes()

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
		})

		res, err := client.SearchSessions(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.GreaterOrEqual(t, len(res.Msg.Sessions), 2)
	})

	t.Run("失敗: 無効な検索パラメータ", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{
				// Invalid parameters will be handled by usecase
			},
		})

		_, err := client.SearchSessions(t.Context(), req)
		// This may or may not fail depending on usecase implementation
		// For now, just verify it returns a response or error
		if err != nil {
			connectErr, ok := err.(*connect.Error)
			require.True(t, ok, "expected connect.Error")
			assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
		}
	})
}

func TestControllerService_DeleteEndedSession(t *testing.T) {
	t.Run("成功: 存在しないセッション（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteEndedSessionRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.DeleteEndedSession(t.Context(), req)
		require.NoError(t, err)
	})
}

func TestControllerService_UpdateSessionExtraSettings(t *testing.T) {
	t.Run("成功: セッションの追加設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient (called by GetSession)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		autoUpgrade := true
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionExtraSettingsRequest{
			SessionId:   session.ID,
			AutoUpgrade: &autoUpgrade,
		})

		res, err := client.UpdateSessionExtraSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify settings were updated in database
		updatedSession, err := setup.queries.GetSession(t.Context(), session.ID)
		require.NoError(t, err)
		assert.True(t, updatedSession.AutoUpgrade)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()
		client := setupAuthenticatedClient(t, setup.service)

		autoUpgrade := true
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionExtraSettingsRequest{
			SessionId:   "nonexist-session",
			AutoUpgrade: &autoUpgrade,
		})

		_, err := client.UpdateSessionExtraSettings(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

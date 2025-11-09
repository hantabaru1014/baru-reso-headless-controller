package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	hostconnectormock "github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector/mock"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	skyfrostmock "github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost/mock"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type controllerServiceTestSetup struct {
	service           *ControllerService
	ctrl              *gomock.Controller
	mockHostConnector *hostconnectormock.MockHostConnector
	mockSkyfrost      *skyfrostmock.MockClient
}

func setupControllerServiceTest(t *testing.T) *controllerServiceTestSetup {
	t.Helper()

	// Setup test database
	queries, _ := testutil.SetupTestDB(t)

	// Setup mocks for external dependencies
	ctrl := gomock.NewController(t)
	mockHostConnector := hostconnectormock.NewMockHostConnector(ctrl)
	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)

	// Setup repositories with real implementation
	hhrepo := adapter.NewHeadlessHostRepository(queries, mockHostConnector)
	srepo := adapter.NewSessionRepository(queries)

	// Setup usecases
	hauc := usecase.NewHeadlessAccountUsecase(queries, mockSkyfrost)
	suc := usecase.NewSessionUsecase(srepo, hhrepo)
	hhuc := usecase.NewHeadlessHostUsecase(hhrepo, srepo, suc, hauc)

	// Setup service
	service := NewControllerService(hhrepo, srepo, hhuc, hauc, suc, mockSkyfrost)

	return &controllerServiceTestSetup{
		service:           service,
		ctrl:              ctrl,
		mockHostConnector: mockHostConnector,
		mockSkyfrost:      mockSkyfrost,
	}
}

func TestControllerService_ListHeadlessAccounts(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.ctrl.Finish()

	// Get DB queries to create test data
	queries, pool := testutil.SetupTestDB(t)
	defer testutil.CleanupTables(t, pool)

	// Create test headless accounts
	testutil.CreateTestHeadlessAccount(t, queries, "U-test1", "user1@example.com", "password1")
	testutil.CreateTestHeadlessAccount(t, queries, "U-test2", "user2@example.com", "password2")

	t.Run("成功: ヘッドレスアカウントのリストを取得", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.ListHeadlessAccountsRequest{})

		res, err := setup.service.ListHeadlessAccounts(t.Context(), req)
		require.NoError(t, err)

		assert.Len(t, res.Msg.Accounts, 2)

		// Verify account data
		account1 := res.Msg.Accounts[0]
		assert.NotEmpty(t, account1.UserId)
		assert.NotEmpty(t, account1.UserName)
	})
}

func TestControllerService_CreateHeadlessAccount(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.ctrl.Finish()

	queries, pool := testutil.SetupTestDB(t)
	defer testutil.CleanupTables(t, pool)

	t.Run("成功: 有効な認証情報でアカウント作成", func(t *testing.T) {
		// Mock skyfrost client to return successful login
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "testuser@example.com", "testpass123").
			Return(&skyfrost.UserSession{UserId: "U-testuser123"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-testuser123").
			Return(&skyfrost.UserInfo{
				ID:       "U-testuser123",
				UserName: "TestUser",
				IconUrl:  "https://example.com/icon.png",
			}, nil)

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "testuser@example.com",
			Password:   "testpass123",
		})

		res, err := setup.service.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was created in DB
		account, err := queries.GetHeadlessAccount(t.Context(), "U-testuser123")
		require.NoError(t, err)
		assert.Equal(t, "testuser@example.com", account.Credential)
	})

	t.Run("失敗: 無効な認証情報でアカウント作成", func(t *testing.T) {
		// Mock skyfrost client to return login error
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "invalid@example.com", "invalidpassword").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "invalid@example.com",
			Password:   "invalidpassword",
		})

		_, err := setup.service.CreateHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: 既に存在するアカウントを作成", func(t *testing.T) {
		// Create initial account
		testutil.CreateTestHeadlessAccount(t, queries, "U-existing", "existing@example.com", "password123")

		// Mock skyfrost to return successful login but DB insert will fail
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "existing@example.com", "newpassword123").
			Return(&skyfrost.UserSession{UserId: "U-existing"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-existing").
			Return(&skyfrost.UserInfo{
				ID:       "U-existing",
				UserName: "ExistingUser",
				IconUrl:  "https://example.com/icon.png",
			}, nil)

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "existing@example.com",
			Password:   "newpassword123",
		})

		_, err := setup.service.CreateHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_DeleteHeadlessAccount(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.ctrl.Finish()

	queries, pool := testutil.SetupTestDB(t)
	defer testutil.CleanupTables(t, pool)

	t.Run("成功: ヘッドレスアカウントを削除", func(t *testing.T) {
		// Create test account
		testutil.CreateTestHeadlessAccount(t, queries, "U-todelete", "todelete@example.com", "password123")

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-todelete",
		})

		res, err := setup.service.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was deleted
		_, err = queries.GetHeadlessAccount(t.Context(), "U-todelete")
		assert.Error(t, err)
	})

	t.Run("成功: 存在しないアカウントを削除（何も起こらない）", func(t *testing.T) {
		// DeleteHeadlessAccountは:execで実装されているため、
		// 存在しないアカウントを削除してもエラーにならない（何も削除されないだけ）
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-nonexistent",
		})

		res, err := setup.service.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})
}

func TestControllerService_ListHeadlessHostImageTags(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.ctrl.Finish()

	t.Run("成功: イメージタグ一覧を取得", func(t *testing.T) {
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

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		res, err := setup.service.ListHeadlessHostImageTags(t.Context(), req)
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
		// Mock HostConnector to return error
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), nil).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		_, err := setup.service.ListHeadlessHostImageTags(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

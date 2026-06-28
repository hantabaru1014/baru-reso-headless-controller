package rpc

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	skyfrostmock "github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost/mock"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUserService_GetTokenByPassword(t *testing.T) {
	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	// 先行テストが users テーブルに残骸を残している場合に備え、テスト開始時にも cleanup する.
	testutil.CleanupTables(t, pool)
	defer testutil.CleanupTables(t, pool)

	// Create test user
	testUserID := "test@example.test"
	testPassword := "testpassword123"
	testutil.CreateTestUser(t, queries, testUserID, testPassword)

	// Setup service
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)
	permUC := newPermissionUsecaseForTest(queries)
	guc := newGroupUsecaseForTest(queries, permUC)
	uu := usecase.NewUserUsecase(queries, pool, mockSkyfrost, guc, permUC)
	service := NewUserService(uu, permUC)

	t.Run("成功: 正しいIDとパスワードでトークンを取得", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
			Id:       testUserID,
			Password: testPassword,
		})

		res, err := service.GetTokenByPassword(t.Context(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Msg.GetToken())
		assert.NotEmpty(t, res.Msg.GetRefreshToken())
	})

	t.Run("失敗: 間違ったパスワード", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
			Id:       testUserID,
			Password: "wrongpassword",
		})

		_, err := service.GetTokenByPassword(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: 存在しないユーザー", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
			Id:       "nonexistent@example.test",
			Password: testPassword,
		})

		_, err := service.GetTokenByPassword(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestUserService_RefreshToken(t *testing.T) {
	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	// 先行テストが users テーブルに残骸を残している場合に備え、テスト開始時にも cleanup する.
	testutil.CleanupTables(t, pool)
	defer testutil.CleanupTables(t, pool)

	// Create test user
	testUserID := "test@example.test"
	testPassword := "testpassword123"
	testutil.CreateTestUser(t, queries, testUserID, testPassword)

	// Setup service
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)
	permUC := newPermissionUsecaseForTest(queries)
	guc := newGroupUsecaseForTest(queries, permUC)
	uu := usecase.NewUserUsecase(queries, pool, mockSkyfrost, guc, permUC)
	service := NewUserService(uu, permUC)

	t.Run("成功: 有効なトークンでリフレッシュ", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Get initial token
			initialReq := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
				Id:       testUserID,
				Password: testPassword,
			})

			initialRes, err := service.GetTokenByPassword(t.Context(), initialReq)
			require.NoError(t, err)

			// Wait for 1 second to ensure different IssuedAt timestamp
			// synctest makes time.Sleep run instantly with fake time
			time.Sleep(1 * time.Second)

			// Create refresh request with the token
			refreshReq := connect.NewRequest(&hdlctrlv1.RefreshTokenRequest{})
			refreshReq.Header().Set("Authorization", "Bearer "+initialRes.Msg.GetRefreshToken())

			res, err := service.RefreshToken(t.Context(), refreshReq)
			require.NoError(t, err)
			assert.NotEmpty(t, res.Msg.GetToken())
			assert.NotEmpty(t, res.Msg.GetRefreshToken())

			// Verify the new token is different from the old one
			assert.NotEqual(t, initialRes.Msg.GetToken(), res.Msg.GetToken())

			// Verify response headers
			assert.NotEmpty(t, res.Header().Get("WWW-Authenticate"))
		})
	})

	t.Run("失敗: トークンなし", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.RefreshTokenRequest{})

		_, err := service.RefreshToken(t.Context(), req)
		assert.Error(t, err)
	})

	t.Run("失敗: 無効なトークン", func(t *testing.T) {
		req := connect.NewRequest(&hdlctrlv1.RefreshTokenRequest{})
		req.Header().Set("Authorization", "Bearer invalid_token")

		_, err := service.RefreshToken(t.Context(), req)
		assert.Error(t, err)
	})

	t.Run("失敗: 有効期限切れのリフレッシュトークン", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Get initial token
			initialReq := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
				Id:       testUserID,
				Password: testPassword,
			})

			initialRes, err := service.GetTokenByPassword(t.Context(), initialReq)
			require.NoError(t, err)

			// Wait for more than 3 days to expire the refresh token
			// synctest makes time.Sleep run instantly with fake time
			time.Sleep(3*24*time.Hour + 1*time.Minute)

			// Try to refresh with expired token
			refreshReq := connect.NewRequest(&hdlctrlv1.RefreshTokenRequest{})
			refreshReq.Header().Set("Authorization", "Bearer "+initialRes.Msg.GetRefreshToken())

			_, err = service.RefreshToken(t.Context(), refreshReq)
			require.Error(t, err)

			// Verify the error message contains "expired"
			assert.Contains(t, err.Error(), "expired")
		})
	})
}

// ===== 管理用 RPC (ListUsers / GetUser / CreateRegistrationToken / DeleteUser) =====

// userServiceTestSetup は管理 RPC の HTTP server 経由テストで使う共有 setup.
type userServiceTestSetup struct {
	service      *UserService
	queries      *db.Queries
	pool         *pgxpool.Pool
	mockSkyfrost *skyfrostmock.MockClient
	ctrl         *gomock.Controller
}

func (s *userServiceTestSetup) Cleanup() {
	s.ctrl.Finish()
}

func newPermissionUsecaseForTest(queries *db.Queries) *usecase.PermissionUsecase {
	return usecase.NewPermissionUsecase(
		adapter.NewGroupRepository(queries),
		adapter.NewGroupMemberRepository(queries),
		adapter.NewRoleRepository(queries),
	)
}

// newGroupUsecaseForTest は UserUsecase が要求する GroupUsecase の最小依存を返す.
// テストはユーザー登録/削除しか触らないため personal グループ生成パス用に必要.
func newGroupUsecaseForTest(queries *db.Queries, permUC *usecase.PermissionUsecase) *usecase.GroupUsecase {
	return usecase.NewGroupUsecase(
		adapter.NewGroupRepository(queries),
		adapter.NewGroupMemberRepository(queries),
		adapter.NewRoleRepository(queries),
		permUC,
	)
}

func setupUserServiceTest(t *testing.T) *userServiceTestSetup {
	t.Helper()

	auth.Init("test-jwt-secret-for-testing")

	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	// デフォルト test user を system-admin にして permission interceptor を素通りさせる.
	testutil.SetupDefaultSystemAdminUser(t, queries)

	ctrl := gomock.NewController(t)
	mockSkyfrost := skyfrostmock.NewMockClient(ctrl)
	permUC := newPermissionUsecaseForTest(queries)
	guc := newGroupUsecaseForTest(queries, permUC)
	uu := usecase.NewUserUsecase(queries, pool, mockSkyfrost, guc, permUC)
	service := NewUserService(uu, permUC)

	return &userServiceTestSetup{
		service:      service,
		queries:      queries,
		pool:         pool,
		mockSkyfrost: mockSkyfrost,
		ctrl:         ctrl,
	}
}

func setupUserServiceClient(t *testing.T, service *UserService) hdlctrlv1connect.UserServiceClient {
	t.Helper()

	server := testutil.SetupAuthenticatedHTTPServer(t, service)
	t.Cleanup(server.Close)

	return hdlctrlv1connect.NewUserServiceClient(server.Client(), server.URL)
}

// createNormalUser は personal group なしのテスト用ユーザーを 1 件作って返す.
// system 権限を持たないユーザーを「呼び出し元」にしたい場面で使う.
func createNormalUser(t *testing.T, queries *db.Queries, userID string) {
	t.Helper()

	_ = testutil.CreateTestUser(t, queries, userID, "dummy-password")
	// personal group は ListUsers 等では不要だが、ResolveGroupIDForUser に
	// 引っかかる経路があれば必要になる. 念のため作成しておく.
	personalGroupID := userID + "-personal"

	_, err := queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   personalGroupID,
		Name: personalGroupID,
		Type: string(entity.GroupType_Personal),
	})
	require.NoError(t, err)

	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: personalGroupID,
		UserID:  userID,
		RoleID:  entity.SeedRoleID_Admin,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err)
}

func TestUserService_ListUsers(t *testing.T) {
	t.Run("成功: system-admin が全ユーザーを取得", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		// 別ユーザーも投入.
		createNormalUser(t, setup.queries, "alice@example.test")
		createNormalUser(t, setup.queries, "bob@example.test")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListUsersRequest{})
		res, err := client.ListUsers(t.Context(), req)
		require.NoError(t, err)

		ids := make([]string, 0, len(res.Msg.GetUsers()))
		for _, u := range res.Msg.GetUsers() {
			ids = append(ids, u.GetId())
			// Resonite ID/icon はテストユーザーで投入済み
			assert.NotEmpty(t, u.GetResoniteId(), "resonite_id should be populated")
		}

		assert.Contains(t, ids, "test@example.test")
		assert.Contains(t, ids, "alice@example.test")
		assert.Contains(t, ids, "bob@example.test")
	})

	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		_, err := client.ListUsers(t.Context(), connect.NewRequest(&hdlctrlv1.ListUsersRequest{}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("成功: system 権限を持たないユーザーでも一覧取得可能 (auth-only)", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		// system 権限を持たないユーザーを作る
		createNormalUser(t, setup.queries, "alice@example.test")

		req := testutil.CreateAuthenticatedRequest(
			t,
			&hdlctrlv1.ListUsersRequest{},
			"alice@example.test", "U-alice", "",
		)

		res, err := client.ListUsers(t.Context(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Msg.GetUsers())
	})
}

func TestUserService_GetUser(t *testing.T) {
	t.Run("成功: 認証済みなら自分以外のユーザーも取得可能", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		// system 権限を持たない alice を呼び出し元にする
		createNormalUser(t, setup.queries, "alice@example.test")
		createNormalUser(t, setup.queries, "bob@example.test")

		req := testutil.CreateAuthenticatedRequest(
			t,
			&hdlctrlv1.GetUserRequest{UserId: "bob@example.test"},
			"alice@example.test", "U-alice", "",
		)
		res, err := client.GetUser(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, "bob@example.test", res.Msg.GetUser().GetId())
		assert.NotEmpty(t, res.Msg.GetUser().GetResoniteId())
	})

	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		_, err := client.GetUser(t.Context(), connect.NewRequest(&hdlctrlv1.GetUserRequest{
			UserId: "test@example.test",
		}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: 存在しないユーザー → NotFound", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetUserRequest{
			UserId: "ghost@example.test",
		})

		_, err := client.GetUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: user_id 空 → InvalidArgument", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetUserRequest{
			UserId: "",
		})

		_, err := client.GetUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestUserService_CreateRegistrationToken(t *testing.T) {
	t.Run("成功: system-admin が登録トークンを発行 (Resonite情報も返る)", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		resoniteID := "U-newuser"
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), resoniteID).
			Return(&skyfrost.UserInfo{
				ID:       resoniteID,
				UserName: "newuser_display",
				IconUrl:  "https://example.test/newuser.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateRegistrationTokenRequest{
			ResoniteId: resoniteID,
		})
		res, err := client.CreateRegistrationToken(t.Context(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Msg.GetToken())
		assert.Equal(t, "newuser_display", res.Msg.GetResoniteUserName())
		assert.Equal(t, "https://example.test/newuser.png", res.Msg.GetIconUrl())

		// expires_at は 24h 後 (±5 分の許容).
		expectedExpires := time.Now().Add(24 * time.Hour)
		gotExpires := res.Msg.GetExpiresAt().AsTime()
		diff := gotExpires.Sub(expectedExpires)
		assert.Less(t, diff.Abs(), 5*time.Minute, "expires_at out of expected window")

		// 発行したトークンが DB に保存されており、ValidateRegistrationToken も通る.
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), resoniteID).
			Return(&skyfrost.UserInfo{ID: resoniteID, UserName: "newuser_display"}, nil)
		validateRes, err := client.ValidateRegistrationToken(t.Context(), connect.NewRequest(&hdlctrlv1.ValidateRegistrationTokenRequest{
			Token: res.Msg.GetToken(),
		}))
		require.NoError(t, err)
		assert.True(t, validateRes.Msg.GetValid())
		assert.Equal(t, resoniteID, validateRes.Msg.GetResoniteId())
	})

	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		_, err := client.CreateRegistrationToken(t.Context(), connect.NewRequest(&hdlctrlv1.CreateRegistrationTokenRequest{
			ResoniteId: "U-foo",
		}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: system:user.create 権限なし → PermissionDenied", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		createNormalUser(t, setup.queries, "alice@example.test")

		req := testutil.CreateAuthenticatedRequest(
			t,
			&hdlctrlv1.CreateRegistrationTokenRequest{ResoniteId: "U-foo"},
			"alice@example.test", "U-alice", "",
		)

		_, err := client.CreateRegistrationToken(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_SystemUserCreate)
	})

	t.Run("失敗: resonite_id 空 → InvalidArgument", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateRegistrationTokenRequest{
			ResoniteId: "",
		})

		_, err := client.CreateRegistrationToken(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: Resonite ID 不正 (skyfrostエラー) → InvalidArgument", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-invalid").
			Return(nil, errors.New("user not found in resonite"))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateRegistrationTokenRequest{
			ResoniteId: "U-invalid",
		})

		_, err := client.CreateRegistrationToken(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestUserService_DeleteUser(t *testing.T) {
	t.Run("成功: system-admin が他ユーザーを削除", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		createNormalUser(t, setup.queries, "victim@example.test")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteUserRequest{
			UserId: "victim@example.test",
		})

		_, err := client.DeleteUser(t.Context(), req)
		require.NoError(t, err)

		// 確認: 実際に削除されている.
		_, err = setup.queries.GetUser(t.Context(), "victim@example.test")
		require.Error(t, err)
	})

	t.Run("失敗: 自分自身は削除できない → FailedPrecondition", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteUserRequest{
			UserId: "test@example.test", // = default authenticated user
		})

		_, err := client.DeleteUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeFailedPrecondition, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "yourself")

		// 自分自身は削除されていない.
		_, err = setup.queries.GetUser(t.Context(), "test@example.test")
		require.NoError(t, err)
	})

	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		_, err := client.DeleteUser(t.Context(), connect.NewRequest(&hdlctrlv1.DeleteUserRequest{
			UserId: "anyone",
		}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: system:user.delete 権限なし → PermissionDenied", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		createNormalUser(t, setup.queries, "alice@example.test")
		createNormalUser(t, setup.queries, "bob@example.test")

		req := testutil.CreateAuthenticatedRequest(
			t,
			&hdlctrlv1.DeleteUserRequest{UserId: "bob@example.test"},
			"alice@example.test", "U-alice", "",
		)

		_, err := client.DeleteUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_SystemUserDelete)

		// bob は残っている.
		_, err = setup.queries.GetUser(t.Context(), "bob@example.test")
		require.NoError(t, err)
	})

	t.Run("失敗: 存在しないユーザー → NotFound", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteUserRequest{
			UserId: "ghost@example.test",
		})

		_, err := client.DeleteUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: user_id 空 → InvalidArgument", func(t *testing.T) {
		setup := setupUserServiceTest(t)
		defer setup.Cleanup()

		client := setupUserServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteUserRequest{
			UserId: "",
		})

		_, err := client.DeleteUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

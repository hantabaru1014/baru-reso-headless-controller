package rpc

import (
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_GetTokenByPassword(t *testing.T) {
	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	defer testutil.CleanupTables(t, pool)

	// Create test user
	testUserID := "test@example.test"
	testPassword := "testpassword123"
	testutil.CreateTestUser(t, queries, testUserID, testPassword)

	// Setup service
	uu := usecase.NewUserUsecase(queries)
	service := NewUserService(uu)

	t.Run("成功: 正しいIDとパスワードでトークンを取得", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
			Id:       testUserID,
			Password: testPassword,
		})

		res, err := service.GetTokenByPassword(t.Context(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Msg.Token)
		assert.NotEmpty(t, res.Msg.RefreshToken)
	})

	t.Run("失敗: 間違ったパスワード", func(t *testing.T) {
		req := testutil.CreateUnauthenticatedRequest(&hdlctrlv1.GetTokenByPasswordRequest{
			Id:       testUserID,
			Password: "wrongpassword",
		})

		_, err := service.GetTokenByPassword(t.Context(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
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

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestUserService_RefreshToken(t *testing.T) {
	// Setup test database
	queries, pool := testutil.SetupTestDB(t)
	defer testutil.CleanupTables(t, pool)

	// Create test user
	testUserID := "test@example.test"
	testPassword := "testpassword123"
	testutil.CreateTestUser(t, queries, testUserID, testPassword)

	// Setup service
	uu := usecase.NewUserUsecase(queries)
	service := NewUserService(uu)

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
			refreshReq.Header().Set("Authorization", "Bearer "+initialRes.Msg.RefreshToken)

			res, err := service.RefreshToken(t.Context(), refreshReq)
			require.NoError(t, err)
			assert.NotEmpty(t, res.Msg.Token)
			assert.NotEmpty(t, res.Msg.RefreshToken)

			// Verify the new token is different from the old one
			assert.NotEqual(t, initialRes.Msg.Token, res.Msg.Token)

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
			refreshReq.Header().Set("Authorization", "Bearer "+initialRes.Msg.RefreshToken)

			_, err = service.RefreshToken(t.Context(), refreshReq)
			require.Error(t, err)

			// Verify the error message contains "expired"
			assert.Contains(t, err.Error(), "expired")
		})
	})
}

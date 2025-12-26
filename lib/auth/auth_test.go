package auth

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set JWT_SECRET for testing if not already set
	if secretKey == "" {
		secretKey = "test-secret-key-for-testing"
	}
}

func TestGenerateToken(t *testing.T) {
	t.Run("成功: トークンを生成", func(t *testing.T) {
		claims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
			IconUrl:    "https://example.com/icon.png",
		}

		token, err := GenerateToken(claims, 30*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("成功: 異なるTTLで生成", func(t *testing.T) {
		claims := AuthClaims{
			UserID: "test-user",
		}

		token1, err := GenerateToken(claims, 1*time.Hour)
		require.NoError(t, err)

		token2, err := GenerateToken(claims, 24*time.Hour)
		require.NoError(t, err)

		// 異なるトークンが生成される（IssuedAtが異なる可能性があるため）
		assert.NotEmpty(t, token1)
		assert.NotEmpty(t, token2)
	})
}

func TestGenerateTokensWithDefaultTTL(t *testing.T) {
	t.Run("成功: トークンとリフレッシュトークンを生成", func(t *testing.T) {
		claims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
			IconUrl:    "https://example.com/icon.png",
		}

		token, refreshToken, err := GenerateTokensWithDefaultTTL(claims)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.NotEmpty(t, refreshToken)
		assert.NotEqual(t, token, refreshToken)
	})

	t.Run("トークンは30分で期限切れになる", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			token, _, err := GenerateTokensWithDefaultTTL(claims)
			require.NoError(t, err)

			// 29分後はまだ有効
			time.Sleep(29 * time.Minute)
			_, err = ParseToken(token)
			require.NoError(t, err)

			// 31分後は期限切れ
			time.Sleep(2 * time.Minute)
			_, err = ParseToken(token)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expired")
		})
	})

	t.Run("リフレッシュトークンは3日で期限切れになる", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			_, refreshToken, err := GenerateTokensWithDefaultTTL(claims)
			require.NoError(t, err)

			// 2日後はまだ有効
			time.Sleep(2 * 24 * time.Hour)
			_, err = ParseToken(refreshToken)
			require.NoError(t, err)

			// 3日+1分後は期限切れ
			time.Sleep(24*time.Hour + 1*time.Minute)
			_, err = ParseToken(refreshToken)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expired")
		})
	})
}

func TestParseToken(t *testing.T) {
	t.Run("成功: 有効なトークンをパース", func(t *testing.T) {
		originalClaims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
			IconUrl:    "https://example.com/icon.png",
		}

		token, err := GenerateToken(originalClaims, 30*time.Minute)
		require.NoError(t, err)

		parsedClaims, err := ParseToken(token)
		require.NoError(t, err)
		assert.Equal(t, originalClaims.UserID, parsedClaims.UserID)
		assert.Equal(t, originalClaims.ResoniteID, parsedClaims.ResoniteID)
		assert.Equal(t, originalClaims.IconUrl, parsedClaims.IconUrl)
	})

	t.Run("失敗: 無効なトークン", func(t *testing.T) {
		_, err := ParseToken("invalid-token")
		require.Error(t, err)
	})

	t.Run("失敗: 空のトークン", func(t *testing.T) {
		_, err := ParseToken("")
		require.Error(t, err)
	})

	t.Run("失敗: 有効期限切れのトークン", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			token, err := GenerateToken(claims, 1*time.Minute)
			require.NoError(t, err)

			// 1分以上待機して有効期限切れにする
			time.Sleep(2 * time.Minute)

			_, err = ParseToken(token)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expired")
		})
	})
}

func TestValidateToken(t *testing.T) {
	t.Run("成功: 有効なトークンで認証", func(t *testing.T) {
		claims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
			IconUrl:    "https://example.com/icon.png",
		}

		token, _, err := GenerateTokensWithDefaultTTL(claims)
		require.NoError(t, err)

		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer "+token)

		validatedClaims, err := ValidateToken(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, claims.UserID, validatedClaims.UserID)
	})

	t.Run("失敗: トークンなし", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})

		_, err := ValidateToken(context.Background(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: Bearer のみ", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer ")

		_, err := ValidateToken(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("失敗: 無効なトークン形式", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer invalid-token")

		_, err := ValidateToken(context.Background(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: 期限切れトークンで認証失敗", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			token, _, err := GenerateTokensWithDefaultTTL(claims)
			require.NoError(t, err)

			// トークンが有効な間は成功
			req := connect.NewRequest(&struct{}{})
			req.Header().Set("authorization", "Bearer "+token)
			_, err = ValidateToken(context.Background(), req)
			require.NoError(t, err)

			// 31分後は期限切れ
			time.Sleep(31 * time.Minute)
			req2 := connect.NewRequest(&struct{}{})
			req2.Header().Set("authorization", "Bearer "+token)
			_, err = ValidateToken(context.Background(), req2)
			require.Error(t, err)

			connectErr, ok := err.(*connect.Error)
			require.True(t, ok)
			assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
		})
	})
}

func TestNewAuthInterceptor(t *testing.T) {
	interceptor := NewAuthInterceptor()

	t.Run("成功: 有効なトークンでリクエストが通過", func(t *testing.T) {
		claims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
		}

		token, _, err := GenerateTokensWithDefaultTTL(claims)
		require.NoError(t, err)

		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer "+token)

		var capturedCtx context.Context
		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			capturedCtx = ctx
			return connect.NewResponse(&struct{}{}), nil
		}

		handler := interceptor(next)
		_, err = handler(context.Background(), req)
		require.NoError(t, err)

		// コンテキストにclaimsがセットされていることを確認
		ctxClaims, err := GetAuthClaimsFromContext(capturedCtx)
		require.NoError(t, err)
		assert.Equal(t, claims.UserID, ctxClaims.UserID)
	})

	t.Run("失敗: トークンなしでリクエストが拒否", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})

		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			t.Fatal("should not reach here")
			return nil, nil
		}

		handler := interceptor(next)
		_, err := handler(context.Background(), req)
		require.Error(t, err)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: 無効なトークンでリクエストが拒否", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer invalid-token")

		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			t.Fatal("should not reach here")
			return nil, nil
		}

		handler := interceptor(next)
		_, err := handler(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("失敗: 期限切れトークンでリクエストが拒否", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			token, _, err := GenerateTokensWithDefaultTTL(claims)
			require.NoError(t, err)

			// 31分後は期限切れ
			time.Sleep(31 * time.Minute)

			req := connect.NewRequest(&struct{}{})
			req.Header().Set("authorization", "Bearer "+token)

			next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				t.Fatal("should not reach here")
				return nil, nil
			}

			handler := interceptor(next)
			_, err = handler(context.Background(), req)
			require.Error(t, err)

			connectErr, ok := err.(*connect.Error)
			require.True(t, ok)
			assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
		})
	})
}

func TestNewOptionalAuthInterceptor(t *testing.T) {
	interceptor := NewOptionalAuthInterceptor()

	t.Run("成功: 有効なトークンでclaimsがセットされる", func(t *testing.T) {
		claims := AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
		}

		token, _, err := GenerateTokensWithDefaultTTL(claims)
		require.NoError(t, err)

		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer "+token)

		var capturedCtx context.Context
		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			capturedCtx = ctx
			return connect.NewResponse(&struct{}{}), nil
		}

		handler := interceptor(next)
		_, err = handler(context.Background(), req)
		require.NoError(t, err)

		// コンテキストにclaimsがセットされていることを確認
		ctxClaims, err := GetAuthClaimsFromContext(capturedCtx)
		require.NoError(t, err)
		assert.Equal(t, claims.UserID, ctxClaims.UserID)
	})

	t.Run("成功: トークンなしでもリクエストが通過（claimsなし）", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})

		var capturedCtx context.Context
		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			capturedCtx = ctx
			return connect.NewResponse(&struct{}{}), nil
		}

		handler := interceptor(next)
		_, err := handler(context.Background(), req)
		require.NoError(t, err)

		// コンテキストにclaimsがセットされていないことを確認
		_, err = GetAuthClaimsFromContext(capturedCtx)
		require.Error(t, err)
	})

	t.Run("成功: 無効なトークンでもリクエストが通過（claimsなし）", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer invalid-token")

		var capturedCtx context.Context
		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			capturedCtx = ctx
			return connect.NewResponse(&struct{}{}), nil
		}

		handler := interceptor(next)
		_, err := handler(context.Background(), req)
		require.NoError(t, err)

		// 無効なトークンなのでclaimsはセットされない
		_, err = GetAuthClaimsFromContext(capturedCtx)
		require.Error(t, err)
	})

	t.Run("成功: Bearer のみでもリクエストが通過", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set("authorization", "Bearer ")

		nextCalled := false
		next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return connect.NewResponse(&struct{}{}), nil
		}

		handler := interceptor(next)
		_, err := handler(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, nextCalled)
	})

	t.Run("成功: 期限切れトークンでもリクエストは通過（claimsなし）", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			claims := AuthClaims{
				UserID: "test-user",
			}

			token, _, err := GenerateTokensWithDefaultTTL(claims)
			require.NoError(t, err)

			// 31分後は期限切れ
			time.Sleep(31 * time.Minute)

			req := connect.NewRequest(&struct{}{})
			req.Header().Set("authorization", "Bearer "+token)

			var capturedCtx context.Context
			next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				capturedCtx = ctx
				return connect.NewResponse(&struct{}{}), nil
			}

			handler := interceptor(next)
			_, err = handler(context.Background(), req)
			require.NoError(t, err)

			// 期限切れトークンなのでclaimsはセットされない
			_, err = GetAuthClaimsFromContext(capturedCtx)
			require.Error(t, err)
		})
	})
}

func TestHashPassword(t *testing.T) {
	t.Run("成功: パスワードをハッシュ化", func(t *testing.T) {
		password := "testpassword123"

		hash, err := HashPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash)
	})

	t.Run("成功: 同じパスワードでも異なるハッシュが生成される", func(t *testing.T) {
		password := "testpassword123"

		hash1, err := HashPassword(password)
		require.NoError(t, err)

		hash2, err := HashPassword(password)
		require.NoError(t, err)

		// bcryptはsaltを含むため、同じパスワードでも異なるハッシュが生成される
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestComparePasswordAndHash(t *testing.T) {
	t.Run("成功: 正しいパスワードで検証成功", func(t *testing.T) {
		password := "testpassword123"

		hash, err := HashPassword(password)
		require.NoError(t, err)

		err = ComparePasswordAndHash(password, hash)
		require.NoError(t, err)
	})

	t.Run("失敗: 間違ったパスワードで検証失敗", func(t *testing.T) {
		password := "testpassword123"
		wrongPassword := "wrongpassword"

		hash, err := HashPassword(password)
		require.NoError(t, err)

		err = ComparePasswordAndHash(wrongPassword, hash)
		require.Error(t, err)
	})

	t.Run("失敗: 空のパスワードで検証失敗", func(t *testing.T) {
		password := "testpassword123"

		hash, err := HashPassword(password)
		require.NoError(t, err)

		err = ComparePasswordAndHash("", hash)
		require.Error(t, err)
	})
}

func TestGetAuthClaimsFromContext(t *testing.T) {
	t.Run("成功: コンテキストからclaimsを取得", func(t *testing.T) {
		claims := &AuthClaims{
			UserID:     "test-user",
			ResoniteID: "U-test123",
			IconUrl:    "https://example.com/icon.png",
		}

		ctx := context.WithValue(context.Background(), AuthClaimsKey, claims)

		retrievedClaims, err := GetAuthClaimsFromContext(ctx)
		require.NoError(t, err)
		assert.Equal(t, claims.UserID, retrievedClaims.UserID)
		assert.Equal(t, claims.ResoniteID, retrievedClaims.ResoniteID)
		assert.Equal(t, claims.IconUrl, retrievedClaims.IconUrl)
	})

	t.Run("失敗: claimsがないコンテキスト", func(t *testing.T) {
		ctx := context.Background()

		_, err := GetAuthClaimsFromContext(ctx)
		require.Error(t, err)
	})

	t.Run("失敗: 不正な型のclaims", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), AuthClaimsKey, "invalid-claims")

		_, err := GetAuthClaimsFromContext(ctx)
		require.Error(t, err)
	})
}

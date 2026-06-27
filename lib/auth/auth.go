package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/go-errors/errors"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var secretKey string

const (
	minSecretKeyLength = 10
	defaultTokenTTL    = 30 * time.Minute
	defaultRefreshTTL  = 3 * 24 * time.Hour

	// ResoniteLinkAudience は ResoniteLink 用 WebSocket 接続トークンの audience.
	// 通常のアクセストークン (AuthClaims) との誤用を防ぐ.
	ResoniteLinkAudience = "resonite-link-ws"
)

func Init(jwtSecret string) {
	secretKey = jwtSecret
}

type AuthClaims struct {
	UserID     string `json:"user_id"`
	ResoniteID string `json:"resonite_id"`
	IconUrl    string `json:"icon_url"`
	jwt.RegisteredClaims
}

func GenerateToken(claims AuthClaims, tokenTTL time.Duration) (string, error) {
	now := time.Now()
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(tokenTTL))
	claims.IssuedAt = jwt.NewNumericDate(now)

	return signJWT(claims)
}

// signJWT signs claims with the configured HS256 secret.
// All public token-issuing helpers funnel through here.
func signJWT(claims jwt.Claims) (string, error) {
	if len(secretKey) < minSecretKeyLength {
		return "", errors.New("invalid jwt secret key")
	}

	ss, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return ss, nil
}

// parseJWT verifies and decodes a token into the provided claims pointer.
// All public token-parsing helpers funnel through here.
func parseJWT(tokenString string, claims jwt.Claims, opts ...jwt.ParserOption) error {
	if len(secretKey) < minSecretKeyLength {
		return errors.New("invalid jwt secret key")
	}

	allOpts := append([]jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
	}, opts...)

	if _, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return []byte(secretKey), nil
	}, allOpts...); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

// GenerateTokensWithDefaultTTL generates a token and a refreshToken with default TTLs.
// The token will expire in 30 minutes and the refreshToken will expire in 3 days.
func GenerateTokensWithDefaultTTL(claims AuthClaims) (string, string, error) {
	token, err := GenerateToken(claims, defaultTokenTTL)
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	refreshToken, err := GenerateToken(claims, defaultRefreshTTL)
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	return token, refreshToken, nil
}

func ParseToken(tokenString string) (*AuthClaims, error) {
	claims := &AuthClaims{}
	if err := parseJWT(tokenString, claims); err != nil {
		return nil, err
	}

	return claims, nil
}

type AuthClaimsContextKey string

var AuthClaimsKey = AuthClaimsContextKey("claims")

// validateAuthHeader は Authorization ヘッダから Bearer トークンを抽出して
// claims をパースする. Unary/streaming 両方の interceptor から共有する.
func validateAuthHeader(h http.Header) (*AuthClaims, error) {
	token := h.Get("Authorization")
	if len(token) <= len("Bearer ") {
		err := connect.NewError(connect.CodeUnauthenticated, errors.New("token required"))
		err.Meta().Add("WWW-Authenticate", "Bearer realm=\"token_required\"")

		return nil, err
	}

	claims, err := ParseToken(token[len("Bearer "):])
	if err != nil {
		connectErr := connect.NewError(connect.CodeUnauthenticated, err)
		connectErr.Meta().Add("WWW-Authenticate", "Bearer error=\"invalid_token\"")

		return nil, connectErr
	}

	return claims, nil
}

// ValidateToken は Unary handler の AnyRequest からトークンを検証する.
// 既存の RPC handler から直接呼ばれるため公開 API として残す.
func ValidateToken(_ context.Context, req connect.AnyRequest) (*AuthClaims, error) {
	return validateAuthHeader(req.Header())
}

func SetSuccessResponseHeader(res connect.AnyResponse) {
	res.Header().Set("WWW-Authenticate", "Bearer realm=\"\"")
}

// authInterceptor は unary + server/client/bidi streaming の両方に対して
// Bearer トークン認証を行う connect.Interceptor.
type authInterceptor struct{}

func (authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		claims, err := validateAuthHeader(req.Header())
		if err != nil {
			return nil, err
		}

		res, err := next(context.WithValue(ctx, AuthClaimsKey, claims), req)
		if err != nil {
			return nil, err
		}

		SetSuccessResponseHeader(res)

		return res, nil
	}
}

func (authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		claims, err := validateAuthHeader(conn.RequestHeader())
		if err != nil {
			return err
		}

		return next(context.WithValue(ctx, AuthClaimsKey, claims), conn)
	}
}

// NewAuthInterceptor は Bearer トークンによる認証を unary / streaming の
// 両方に適用する interceptor を返す.
func NewAuthInterceptor() connect.Interceptor {
	return authInterceptor{}
}

// NewOptionalAuthInterceptor は認証情報があればコンテキストにセットするが、
// なくてもエラーにしないインターセプター.
func NewOptionalAuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token := req.Header().Get("Authorization")
			if len(token) > len("Bearer ") {
				token = token[len("Bearer "):]

				claims, err := ParseToken(token)
				if err == nil {
					ctx = context.WithValue(ctx, AuthClaimsKey, claims)
				}
			}

			return next(ctx, req)
		}
	}
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return string(hash), nil
}

func ComparePasswordAndHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GetAuthClaimsFromContext はコンテキストからAuthClaimsを取得します。
// 認証されていない場合はエラーを返します。
func GetAuthClaimsFromContext(ctx context.Context) (*AuthClaims, error) {
	claims, ok := ctx.Value(AuthClaimsKey).(*AuthClaims)
	if !ok {
		return nil, errors.New("認証されていません")
	}

	return claims, nil
}

// ResoniteLinkClaims は ResoniteLink WebSocket 接続用の短期トークン用クレーム.
type ResoniteLinkClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// GenerateResoniteLinkToken は ResoniteLink 接続用の短期 JWT を発行する.
// audience に ResoniteLinkAudience を固定し、ParseResoniteLinkToken で検証することで
// アクセストークン (AuthClaims) からの取り違えを防ぐ.
func GenerateResoniteLinkToken(userID, sessionID string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)
	claims := ResoniteLinkClaims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{ResoniteLinkAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	ss, err := signJWT(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	return ss, expiresAt, nil
}

func ParseResoniteLinkToken(tokenString string) (*ResoniteLinkClaims, error) {
	claims := &ResoniteLinkClaims{}
	if err := parseJWT(tokenString, claims, jwt.WithAudience(ResoniteLinkAudience)); err != nil {
		return nil, err
	}

	return claims, nil
}

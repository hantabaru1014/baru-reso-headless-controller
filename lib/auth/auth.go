package auth

import (
	"context"
	"errors"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	secretKey = os.Getenv("JWT_SECRET")
)

type AuthClaims struct {
	UserID     string `json:"user_id"`
	ResoniteID string `json:"resonite_id"`
	IconUrl    string `json:"icon_url"`
	jwt.RegisteredClaims
}

func GenerateToken(claims AuthClaims, tokenTTL time.Duration) (string, error) {
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(tokenTTL))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(secretKey))

	return ss, err
}

// GenerateTokensWithDefaultTTL generates a token and a refreshToken with default TTLs.
// The token will expire in 30 minutes and the refreshToken will expire in 3 days.
func GenerateTokensWithDefaultTTL(claims AuthClaims) (string, string, error) {
	token, err := GenerateToken(claims, 30*time.Minute)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := GenerateToken(claims, 3*24*time.Hour)
	if err != nil {
		return "", "", err
	}

	return token, refreshToken, nil
}

func ParseToken(tokenString string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&AuthClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AuthClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

type AuthClaimsContextKey string

var AuthClaimsKey = AuthClaimsContextKey("claims")

func ValidateToken(ctx context.Context, req connect.AnyRequest) (*AuthClaims, error) {
	token := req.Header().Get("authorization")
	if len(token) <= len("Bearer ") {
		err := connect.NewError(connect.CodeUnauthenticated, errors.New("token required"))
		err.Meta().Add("WWW-Authenticate", "Bearer realm=\"token_required\"")
		return nil, err
	}
	token = token[len("Bearer "):]

	claims, err := ParseToken(token)
	if err != nil {
		connectErr := connect.NewError(connect.CodeUnauthenticated, err)
		connectErr.Meta().Add("WWW-Authenticate", "Bearer error=\"invalid_token\"")
		return nil, connectErr
	}

	return claims, nil
}

func SetSuccessResponseHeader(res connect.AnyResponse) {
	res.Header().Set("WWW-Authenticate", "Bearer realm=\"\"")
}

func NewAuthInterceptor() connect.UnaryInterceptorFunc {
	i := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			claims, err := ValidateToken(ctx, req)
			if err != nil {
				return nil, err
			}
			ctx = context.WithValue(ctx, AuthClaimsKey, claims)

			res, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			SetSuccessResponseHeader(res)
			return res, nil
		})
	}
	return connect.UnaryInterceptorFunc(i)
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func ComparePasswordAndHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

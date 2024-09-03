package auth

import (
	"context"
	"errors"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
)

var (
	secretKey = os.Getenv("JWT_SECRET")
	tokenTTL  = 30 * time.Minute
)

type AuthClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(claims AuthClaims, overrideRegistered bool) (string, error) {
	if overrideRegistered {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(tokenTTL))
		claims.IssuedAt = jwt.NewNumericDate(time.Now())
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(secretKey))

	return ss, err
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

func NewAuthInterceptor() connect.UnaryInterceptorFunc {
	i := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
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
			ctx = context.WithValue(ctx, AuthClaimsKey, claims)

			res, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			res.Header().Set("WWW-Authenticate", "Bearer realm=\"\"")
			return res, nil
		})
	}
	return connect.UnaryInterceptorFunc(i)
}

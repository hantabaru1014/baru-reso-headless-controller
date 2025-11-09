package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
)

// CreateAuthenticatedContext creates a context with authentication claims for testing
func CreateAuthenticatedContext(userID, resoniteID, iconURL string) context.Context {
	claims := &auth.AuthClaims{
		UserID:     userID,
		ResoniteID: resoniteID,
		IconUrl:    iconURL,
	}
	return context.WithValue(context.Background(), auth.AuthClaimsKey, claims)
}

// CreateAuthenticatedRequest creates a Connect request with authentication token for testing
func CreateAuthenticatedRequest[T any](t *testing.T, msg *T, userID, resoniteID, iconURL string) *connect.Request[T] {
	t.Helper()

	token, _, err := auth.GenerateTokensWithDefaultTTL(auth.AuthClaims{
		UserID:     userID,
		ResoniteID: resoniteID,
		IconUrl:    iconURL,
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)

	return req
}

// CreateDefaultAuthenticatedRequest creates a Connect request with default authentication credentials for testing.
// Use this when the specific user identity doesn't matter for the test.
func CreateDefaultAuthenticatedRequest[T any](t *testing.T, msg *T) *connect.Request[T] {
	t.Helper()
	return CreateAuthenticatedRequest(t, msg, "test@example.test", "U-test123", "https://example.test/icon.png")
}

// CreateUnauthenticatedRequest creates a Connect request without authentication for testing
func CreateUnauthenticatedRequest[T any](msg *T) *connect.Request[T] {
	return connect.NewRequest(msg)
}

// GetAuthClaimsFromRequest extracts auth claims from a Connect request for testing purposes
func GetAuthClaimsFromRequest[T any](t *testing.T, req *connect.Request[T]) *auth.AuthClaims {
	t.Helper()

	authHeader := req.Header().Get("Authorization")
	if authHeader == "" || len(authHeader) <= len("Bearer ") {
		t.Fatal("no authorization header found")
	}

	token := authHeader[len("Bearer "):]
	claims, err := auth.ParseToken(token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	return claims
}

// CreateHTTPRequest creates a standard HTTP request for testing
func CreateHTTPRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

// ServiceHandler is an interface for services that can create HTTP handlers
type ServiceHandler interface {
	NewHandler() (string, http.Handler)
}

// SetupAuthenticatedHTTPServer creates an HTTP server with authentication interceptor for testing.
// It takes any service that implements ServiceHandler interface (has NewHandler() method).
// Returns the test server and server URL. The caller is responsible for closing the server.
func SetupAuthenticatedHTTPServer(t *testing.T, service ServiceHandler) *httptest.Server {
	t.Helper()

	path, handler := service.NewHandler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()

	return server
}

package skyfrost

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

// APIError は Resonite API が 2xx 以外を返したことを表す.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%d %s - %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}

// IsClientError は 4xx (Resonite にリクエストを拒否された) かどうか. リトライしても結果は変わらない.
func (e *APIError) IsClientError() bool {
	return e.StatusCode >= http.StatusBadRequest && e.StatusCode < http.StatusInternalServerError
}

func HashIDToToken(id, salt string) string {
	hash := sha256.Sum256([]byte(id + salt))

	return hex.EncodeToString(hash[:])
}

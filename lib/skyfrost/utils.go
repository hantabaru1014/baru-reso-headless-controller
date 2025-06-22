package skyfrost

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashIDToToken(id, salt string) string {
	hash := sha256.Sum256([]byte(id + salt))
	return hex.EncodeToString(hash[:])
}

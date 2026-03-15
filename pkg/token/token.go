package token

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateToken() string {		// TODO: should use TLS when sent to client && rate limiting to prevent brute force
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign returns the hex HMAC-SHA256 of body keyed by secret.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Verify checks an HMAC signature in constant time.
func Verify(secret string, body []byte, sig string) bool {
	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(sig))
}

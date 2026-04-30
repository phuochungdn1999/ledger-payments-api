// Package idempotency provides HMAC-SHA256 signing for outbound webhooks.
//
// We sign the raw JSON body with the subscriber's secret. Receivers verify
// in constant time by recomputing the HMAC over the raw bytes.
package idempotency

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func Sign(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret []byte, body []byte, header string) bool {
	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(header))
}

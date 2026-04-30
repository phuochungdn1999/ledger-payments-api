package idempotency

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	secret := []byte("whsec_test")
	body := []byte(`{"event":"transfer.settled","amount":100}`)
	sig := Sign(secret, body)
	require.True(t, Verify(secret, body, sig))
	require.False(t, Verify(secret, body, "sha256=deadbeef"))
}

func TestVerify_FailsOnDifferentSecret(t *testing.T) {
	body := []byte("hello")
	sig := Sign([]byte("a"), body)
	require.False(t, Verify([]byte("b"), body, sig))
}

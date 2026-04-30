package ledger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestHash_StableForSameInput(t *testing.T) {
	a := Transfer{From: "a", To: "b", Amount: 100, Currency: "USD"}
	b := Transfer{From: "a", To: "b", Amount: 100, Currency: "USD", Description: "ignored"}
	require.Equal(t, requestHash(a), requestHash(b),
		"description must not be part of the request hash")
}

func TestRequestHash_DiffersOnAmount(t *testing.T) {
	a := Transfer{From: "a", To: "b", Amount: 100, Currency: "USD"}
	b := Transfer{From: "a", To: "b", Amount: 101, Currency: "USD"}
	require.NotEqual(t, requestHash(a), requestHash(b))
}

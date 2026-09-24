package jobadapter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repeatBytes(n int, b byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// TestDecodeL2ExecutionRequest_ExtendedFields checks the envelope fields and the
// structured forced transactions the extended (dev-native) path needs.
func TestDecodeL2ExecutionRequest_ExtendedFields(t *testing.T) {
	data, err := os.ReadFile(referenceL2ExecutionRequest)
	require.NoError(t, err)

	req, err := DecodeL2ExecutionRequest(data)
	require.NoError(t, err)

	assert.Equal(t, repeatBytes(20, 0x11), req.L2MessageServiceAddress[:])
	assert.Equal(t, repeatBytes(20, 0x00), req.Coinbase[:])
	assert.Equal(t, repeatBytes(32, 0x0a), req.ParentFtxRollingHash[:])
	assert.Equal(t, uint64(15), req.ParentFtxNumber)

	require.Len(t, req.Payloads, 2)

	require.Len(t, req.Payloads[0].ForcedTransactions, 1)
	ftx := req.Payloads[0].ForcedTransactions[0]
	assert.Equal(t, uint64(16), ftx.Number)
	assert.Equal(t, uint64(1000599), ftx.Deadline)
	assert.Equal(t, []byte{0x02, 0xf8, 0x6b}, ftx.SignedTxRlp)
	assert.Equal(t, uint8(0), ftx.Acceptance) // INCLUDED

	require.Len(t, req.Payloads[1].ForcedTransactions, 2)
	filtered := req.Payloads[1].ForcedTransactions[1]
	assert.Equal(t, uint64(18), filtered.Number)
	assert.Empty(t, filtered.SignedTxRlp)
	assert.Equal(t, uint8(4), filtered.Acceptance) // FILTERED_ADDRESS_TO
}

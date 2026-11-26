package ssz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncodeRollupInput_PublicInputsSSZ checks that the PublicInputsSSZ of a
// proof is encoded as is, in place of its PublicInputs.
func TestEncodeRollupInput_PublicInputsSSZ(t *testing.T) {
	pi := L2ExecutionPublicInputs{EndBlockNumber: 14, BlockCount: 5, L2MessagingBlocksOffsets: []uint16{1}}
	raw, err := encodeL2ExecutionPublicInputs(pi)
	require.NoError(t, err)

	proof := L2ExecutionProof{StartBlockNumber: 10, Proof: []byte{0x01, 0x02}, ProgramVk: [32]byte{0x03}}
	withStruct, withRaw := proof, proof
	withStruct.PublicInputs = pi
	withRaw.PublicInputsSSZ = raw

	want, err := EncodeRollupInput(RollupInput{L2ExecutionProofs: []L2ExecutionProof{withStruct}})
	require.NoError(t, err)
	got, err := EncodeRollupInput(RollupInput{L2ExecutionProofs: []L2ExecutionProof{withRaw}})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

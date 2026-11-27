package backend

import (
	"bytes"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidityPLONKProofSize is the size of a BN254 PLONK proof with no BSB22
// commitment in the layout the gnark Solidity verifier reads: 64-byte G1
// points and 32-byte claimed values.
const solidityPLONKProofSize = 768

// solidityPLONKProofPoints are the offsets of the G1 points in that layout:
// L, R, O, H0, H1, H2, Z, and the two opening proofs.
var solidityPLONKProofPoints = []int{0, 64, 128, 192, 256, 320, 544, 640, 704}

func TestDevmodeProof(t *testing.T) {
	hash := bytes.Repeat([]byte{0xab}, guestOutputHashSize)
	pub := executedPublicInputs(hash)

	t.Run("aggregation is a gnark PLONK proof", func(t *testing.T) {
		got, err := devmodeProof(ProofTypeRollupAggregation, pub)
		require.NoError(t, err)
		require.Len(t, got, solidityPLONKProofSize, "the proof should have the Solidity layout")
		for _, off := range solidityPLONKProofPoints {
			var p bn254.G1Affine
			_, err := p.SetBytes(got[off : off+64])
			assert.NoError(t, err, "the bytes at %d should be a G1 point", off)
		}
	})

	for _, typ := range []ProofType{ProofTypeL2Execution, ProofTypeRollup} {
		t.Run(string(typ)+" is a wiop devmode image", func(t *testing.T) {
			got, err := devmodeProof(typ, pub)
			require.NoError(t, err)
			assert.Equal(t, devmodeImage(t, hash), got)
		})
	}
}

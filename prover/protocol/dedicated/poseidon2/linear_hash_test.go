package poseidon2_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	linhash "github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/require"
)

const blockSize = 8

func TestLinearHash(t *testing.T) {

	var tohash [blockSize]ifaces.Column
	var expectedhash [blockSize]ifaces.Column

	testcases := []struct {
		colSize int
		numhash int
	}{
		{colSize: 16, numhash: 4},  // Full Chunks
		{colSize: 20, numhash: 16}, // Partial last chunk
		{colSize: 31, numhash: 8},  // Partial last chunk
	}

	for _, tc := range testcases {
		// Each column to be hashed is split into chunks of 8 elements
		numhash := tc.numhash
		colSize := tc.colSize
		colChunks := (colSize + blockSize - 1) / blockSize
		totalChunks := colChunks * numhash
		numRowToHash := utils.NextPowerOfTwo(totalChunks)
		numRowExpectedHash := utils.NextPowerOfTwo(numhash)

		define := func(b *wizard.Builder) {
			for i := 0; i < blockSize; i++ {
				tohash[i] = b.RegisterCommit(ifaces.ColIDf("TOHASH_%v", i), numRowToHash)
				expectedhash[i] = b.RegisterCommit(ifaces.ColIDf("HASHED_%v", i), numRowExpectedHash)
			}
			linhash.CheckLinearHash(b.CompiledIOP, "test", colChunks, tohash, numhash, expectedhash)
		}

		prove := func(run *wizard.ProverRuntime) {
			var ex, th [blockSize][]field.Element

			for i := 0; i < blockSize; i++ {
				ex[i] = make([]field.Element, 0, numhash)
				th[i] = make([]field.Element, totalChunks)
			}

			for i := 0; i < numhash; i++ {
				// generate a segment at random, hash it
				// and append the segment to th and the result
				// to ex
				segment := vector.Rand(colSize)
				hasher := poseidon2_koalabear.NewMDHasher()
				hasher.WriteElements(segment...)
				y := hasher.SumElement()

				for j := 0; j < blockSize; j++ {
					ex[j] = append(ex[j], y[j])
				}
				start := i * colChunks
				th = linhash.PrepareToHashWitness(th, segment, start)
			}

			var exSV, thSV [blockSize]smartvectors.SmartVector
			for i := 0; i < blockSize; i++ {
				thSV[i] = smartvectors.RightZeroPadded(th[i], numRowToHash)
				exSV[i] = smartvectors.RightZeroPadded(ex[i], numRowExpectedHash)

				// And assign them
				run.AssignColumn(tohash[i].GetColID(), thSV[i])
				run.AssignColumn(expectedhash[i].GetColID(), exSV[i])

			}

		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		require.NoError(t, wizard.Verify(comp, proof))
	}

}

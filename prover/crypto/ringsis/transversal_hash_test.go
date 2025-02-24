package ringsis

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/require"
)

// randomConstRow generates a random constant smart-vector
func randomConstRow(rng *rand.Rand, size int) smartvectors.SmartVector {
	return smartvectors.NewConstant(field.PseudoRand(rng), size)
}

// randomRegularRow generates a random regular smart-vector
func randomRegularRow(rng *rand.Rand, size int) smartvectors.SmartVector {
	return smartvectors.PseudoRand(rng, size)
}

// generate a smartvector row-matrix by using randomly constant or regular smart-vectors
func fullyRandomTestVector(rng *rand.Rand, numRow, numCols int) []smartvectors.SmartVector {
	list := make([]smartvectors.SmartVector, numRow)
	for i := range list {
		coin := rng.IntN(2)
		switch {
		case coin == 0:
			list[i] = randomConstRow(rng, numCols)
		case coin == 1:
			list[i] = randomRegularRow(rng, numCols)
		}
	}
	return list
}

func constantRandomTestVector(rng *rand.Rand, numRow, numCols int) []smartvectors.SmartVector {
	list := make([]smartvectors.SmartVector, numRow)
	for i := range list {
		list[i] = randomConstRow(rng, numCols)
	}
	return list
}

func regularRandomTestVector(rng *rand.Rand, numRow, numCols int) []smartvectors.SmartVector {
	list := make([]smartvectors.SmartVector, numRow)
	for i := range list {
		list[i] = randomConstRow(rng, numCols)
	}
	return list
}

func TestSmartVectorTransversalSisHash(t *testing.T) {
	var (
		numReps   = 64
		nbCols    = 16
		rng       = rand.New(utils.NewRandSource(77442)) // nolint
		params    = Params{LogTwoBound: 16, LogTwoDegree: 6}
		testCases = [][]smartvectors.SmartVector{
			constantRandomTestVector(rng, 4, nbCols),
			regularRandomTestVector(rng, 4, nbCols),
		}
	)

	for i := 0; i < numReps; i++ {
		testCases = append(testCases, fullyRandomTestVector(rng, 4, nbCols))
	}

	for i := 0; i < numReps; i++ {
		testCases = append(testCases, fullyRandomTestVector(rng, 8, nbCols))
	}

	for i, c := range testCases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {
			assert := require.New(t)
			var (
				nbRows = len(c)
				nbCols = c[0].Len()
				key    = GenerateKey(params, nbRows)
				result = key.TransversalHash(c)
			)

			offset := key.modulusDegree()

			for col := 0; col < nbCols; col++ {
				column := make([]field.Element, nbRows)
				for r := 0; r < nbRows; r++ {
					column[r] = c[r].Get(col)
				}

				colHash := key.Hash(column)
				for j := 0; j < len(colHash); j++ {
					assert.True(colHash[j].Equal(&result[offset*col+j]), "transversal hash does not match col hash")
				}
			}
		})
	}
}

func BenchmarkTransversalHash(b *testing.B) {

	var (
		numRow          = 1024
		numCols         = 1024
		rng             = rand.New(utils.NewRandSource(77442)) // nolint
		params          = Params{LogTwoBound: 16, LogTwoDegree: 6}
		numInputPerPoly = params.OutputSize() / (field.Bytes * 8 / params.LogTwoBound)
		key             = GenerateKey(params, numRow)
		numTestCases    = 1 << numInputPerPoly
		numPoly         = numRow / numInputPerPoly
	)

	for tc := 0; tc < numTestCases; tc++ {

		b.Run(fmt.Sprintf("testcase-%b", tc), func(b *testing.B) {

			inputs := make([]smartvectors.SmartVector, 0, numPoly*numInputPerPoly)

			for p := 0; p < numPoly; p++ {
				for i := 0; i < numInputPerPoly; i++ {
					if (tc>>i)&1 == 0 {
						inputs = append(inputs, randomConstRow(rng, numCols))
					} else {
						inputs = append(inputs, randomRegularRow(rng, numCols))
					}
				}
			}

			b.ResetTimer()

			for c := 0; c < b.N; c++ {
				_ = key.TransversalHash(inputs)
			}

		})

	}
}

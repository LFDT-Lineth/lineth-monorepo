package ringsis_64_16_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis/ringsis_64_16"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	wfft "github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

func BenchmarkTransversalHash(b *testing.B) {

	var (
		numRow          = 1024
		numCols         = 1024
		rng             = rand.New(rand.NewSource(786868)) // nolint
		domain          = fft.NewDomain(64, fft.WithShift(wfft.GetOmega(64*2)))
		twiddles        = ringsis_64_16.PrecomputeTwiddlesCoset(domain.Generator, domain.FrMultiplicativeGen)
		params          = ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}
		numInputPerPoly = params.OutputSize() / (field.Bytes * 8 / params.LogTwoBound)
		key             = ringsis.GenerateKey(params, numRow)
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
				_ = ringsis_64_16.TransversalHash(key.Ag(), inputs, twiddles, domain)
			}

		})

	}
}

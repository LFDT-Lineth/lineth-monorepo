package mimc_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	linhash "github.com/consensys/linea-monorepo/prover/protocol/dedicated/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestLinearHash(t *testing.T) {

	var tohash, expectedhash ifaces.Column
	testcases := []struct {
		period  int
		numhash int
	}{
		{period: 4, numhash: 16},
		{period: 3, numhash: 16},
		{period: 4, numhash: 14},
		{period: 5, numhash: 17},
	}

	for _, tc := range testcases {

		period := tc.period
		numhash := tc.numhash
		numRowLarge := utils.NextPowerOfTwo(period * numhash)
		numRowSmall := utils.NextPowerOfTwo(numhash)

		define := func(b *wizard.Builder) {
			tohash = b.RegisterCommit("TOHASH", numRowLarge)
			expectedhash = b.RegisterCommit("HASHED", numRowSmall)
			linhash.CheckLinearHash(b.CompiledIOP, "test", tohash, period, numhash, expectedhash)
		}

		prove := func(run *wizard.ProverRuntime) {
			th := make([]field.Element, 0, numhash*period)
			ex := make([]field.Element, 0, numhash)

			for i := 0; i < numhash; i++ {
				// generate a segment at random, hash it
				// and append the segment to th and the result
				// to ex
				segment := vector.Rand(period)
				hasher := mimc.NewMiMC()

				for _, x := range segment {
					xbytes := x.Bytes()
					hasher.Write(xbytes[:])
				}
				var y field.Element
				y.SetBytes(hasher.Sum(nil))

				th = append(th, segment...)
				ex = append(ex, y)
			}

			// And assign them
			thSV := smartvectors.RightZeroPadded(th, numRowLarge)
			exSV := smartvectors.RightZeroPadded(ex, numRowSmall)

			run.AssignColumn(tohash.GetColID(), thSV)
			run.AssignColumn(expectedhash.GetColID(), exSV)
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		require.NoError(t, wizard.Verify(comp, proof))
	}

}

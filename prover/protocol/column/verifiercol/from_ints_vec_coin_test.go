package verifiercol_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

func TestFromIntVec(t *testing.T) {

	// mute the logs
	logrus.SetLevel(logrus.FatalLevel)

	testcases := []struct {
		CoinSize, Split int
		Shift           int
	}{
		{CoinSize: 16, Split: 8, Shift: 1},
		{CoinSize: 16, Split: 8, Shift: 0},
		{CoinSize: 16, Split: 8, Shift: 1},
		{CoinSize: 16, Split: 16, Shift: 1},
		{CoinSize: 16, Split: 16, Shift: 0},
		{CoinSize: 16, Split: 16, Shift: 1},
		{CoinSize: 16, Split: 32, Shift: 1},
		{CoinSize: 16, Split: 32, Shift: 0},
		{CoinSize: 16, Split: 32, Shift: 1},
	}

	// A test vector that is only here for technical reasons. If it is not
	// here, we will hit a sanity-check complaining that we are generating
	// coins before the prover sends any message.
	testP1 := smartvectors.ForTest(0, 1, 2, 3)

	for _, tc := range testcases {

		t.Run(
			fmt.Sprintf("testcase-%++v", tc),
			func(subT *testing.T) {

				var (
					P1      ifaces.ColID   = "P1"
					LOOKUP1 ifaces.QueryID = "LOOKUP"
					COIN    coin.Name      = "COIN"
					QUERY   ifaces.QueryID = "QUERY"
				)

				defineInclu := func(build *wizard.Builder) {

					// P1 and P2 are added on the size to add
					P1 := build.RegisterCommit(P1, testP1.Len()) // overshadows P1
					build.Range(LOOKUP1, P1, testP1.Len())

					coin := build.RegisterRandomCoin(COIN, coin.IntegerVec, tc.CoinSize, 4)
					coinCol := verifiercol.NewFromIntVecCoin(build.CompiledIOP, coin)

					// We apply a shift to force the application of the
					// naturalization compiler over our column
					if tc.Shift > 0 {
						coinCol = column.Shift(coinCol, tc.Shift)
					}

					build.Range(QUERY, coinCol, 4)
				}

				proveInclu := func(run *wizard.ProverRuntime) {
					// I should not need to do anything for the FromIntVec coin
					run.AssignColumn(P1, testP1)
				}

				// Compile with the full suite
				compiled := wizard.Compile(defineInclu,
					compiler.Arcane(
						compiler.WithStitcherMinSize(16),
						compiler.WithTargetColSize(16),
					),
					dummy.Compile,
				)

				proof := wizard.Prove(compiled, proveInclu)

				if err := wizard.Verify(compiled, proof); err != nil {
					subT.Logf("verifier failed : %v", err)
					subT.FailNow()
				}

			},
		)

	}
}

func TestFromIntVecWithPadding(t *testing.T) {

	// mute the logs
	logrus.SetLevel(logrus.FatalLevel)

	testcases := []struct {
		CoinSize, Split int
		Shift, Repeat   int
		PaddingVal      field.Element
	}{
		{CoinSize: 12, Split: 8, Shift: 1, Repeat: 0},
		{CoinSize: 12, Split: 8, Shift: 0, Repeat: 2},
		{CoinSize: 12, Split: 8, Shift: 1, Repeat: 2},
		{CoinSize: 12, Split: 16, Shift: 1, Repeat: 0},
		{CoinSize: 12, Split: 16, Shift: 0, Repeat: 2},
		{CoinSize: 12, Split: 16, Shift: 1, Repeat: 2},
		{CoinSize: 12, Split: 32, Shift: 1, Repeat: 0},
		{CoinSize: 12, Split: 32, Shift: 0, Repeat: 2},
		{CoinSize: 12, Split: 32, Shift: 1, Repeat: 2},
	}

	// A test vector that is only here for technical reasons. If it is not
	// here, we will hit a sanity-check complaining that we are generating
	// coins before the prover sends any message.
	testP1 := smartvectors.ForTest(0, 1, 2, 3)

	for _, tc := range testcases {

		t.Run(
			fmt.Sprintf("testcase-%++v", tc),
			func(subT *testing.T) {

				var (
					P1      ifaces.ColID   = "P1"
					LOOKUP1 ifaces.QueryID = "LOOKUP"
					COIN    coin.Name      = "COIN"
					QUERY   ifaces.QueryID = "QUERY"
				)

				defineInclu := func(build *wizard.Builder) {

					// P1 and P2 are added on the size to add
					P1 := build.RegisterCommit(P1, testP1.Len()) // overshadows P1
					build.Range(LOOKUP1, P1, testP1.Len())

					coin := build.RegisterRandomCoin(COIN, coin.IntegerVec, tc.CoinSize, 4)
					coinCol := verifiercol.NewFromIntVecCoin(build.CompiledIOP, coin, verifiercol.RightPadZeroToNextPowerOfTwo)

					// We apply a shift to force the application of the
					// naturalization compiler over our column
					if tc.Shift > 0 {
						coinCol = column.Shift(coinCol, tc.Shift)
					}

					build.Range(QUERY, coinCol, 4)
				}

				proveInclu := func(run *wizard.ProverRuntime) {
					// I should not need to do anything for the FromIntVec coin
					run.AssignColumn(P1, testP1)
				}

				// Compile with the full suite
				compiled := wizard.Compile(defineInclu,
					compiler.Arcane(
						compiler.WithTargetColSize(16),
						compiler.WithStitcherMinSize(16),
					),
					dummy.Compile,
				)

				proof := wizard.Prove(compiled, proveInclu)

				if err := wizard.Verify(compiled, proof); err != nil {
					subT.Logf("verifier failed : %v", err)
					subT.FailNow()
				}

			},
		)

	}

}

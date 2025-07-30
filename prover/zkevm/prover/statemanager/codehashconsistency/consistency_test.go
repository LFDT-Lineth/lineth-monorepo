package codehashconsistency

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mimccodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

func TestConsistency(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		runTestcase(t, "testdata/mimc-codehash.csv", "testdata/state-summary.csv")
	})
	t.Run("empty-rom", func(t *testing.T) {
		runTestcase(t, "testdata/mimc-codehash-empty.csv", "testdata/state-summary.csv")
	})
	t.Run("empty-state-summary", func(t *testing.T) {
		runTestcase(t, "testdata/mimc-codehash.csv", "testdata/state-summary-empty.csv")
	})
}

func runTestcase(t *testing.T, mimcCodeHashCsvPath, stateSummaryCsvPath string) {

	var (
		stateSummary     *statesummary.Module
		mimcCodeHash     *mimccodehash.Module
		consistency      Module
		sizeStateSummary = 128
		sizeMimcCodeHash = 256
	)

	define := func(b *wizard.Builder) {

		stateSummary = &statesummary.Module{
			IsActive:  b.InsertCommit(0, "SS_IS_ACTIVE", sizeStateSummary),
			IsStorage: b.InsertCommit(0, "SS_IS_STORAGE", sizeStateSummary),
			Account: statesummary.AccountPeek{
				Initial: statesummary.Account{
					KeccakCodeHash: common.NewHiLoColumns(b.CompiledIOP, sizeStateSummary, "SS_INITIAL_KECCAK"),
					MiMCCodeHash:   b.InsertCommit(0, "SS_INITIAL_MIMC", sizeStateSummary),
					Exists:         b.InsertCommit(0, "SS_INITIAL_EXISTS", sizeStateSummary),
				},
				Final: statesummary.Account{
					KeccakCodeHash: common.NewHiLoColumns(b.CompiledIOP, sizeStateSummary, "SS_FINAL_KECCAK"),
					MiMCCodeHash:   b.InsertCommit(0, "SS_FINAL_MIMC", sizeStateSummary),
					Exists:         b.InsertCommit(0, "SS_FINAL_EXISTS", sizeStateSummary),
				},
			},
		}

		mimcCodeHash = &mimccodehash.Module{
			IsActive:         b.InsertCommit(0, "MCH_IS_ACTIVE", sizeMimcCodeHash),
			IsHashEnd:        b.InsertCommit(0, "MCH_IS_HASH_END", sizeMimcCodeHash),
			NewState:         b.InsertCommit(0, "MCH_NEW_STATE", sizeMimcCodeHash),
			CodeHashHi:       b.InsertCommit(0, "MCH_KECCAK_HI", sizeMimcCodeHash),
			CodeHashLo:       b.InsertCommit(0, "MCH_KECCAK_LO", sizeMimcCodeHash),
			IsForConsistency: b.InsertCommit(0, "MCH_IS_FOR_CONSISTENCY", sizeMimcCodeHash),
		}

		consistency = NewModule(b.CompiledIOP, "CONSISTENCY", stateSummary, mimcCodeHash)
	}

	prover := func(run *wizard.ProverRuntime) {

		mchCt := csvtraces.MustOpenCsvFile(mimcCodeHashCsvPath)
		ssCt := csvtraces.MustOpenCsvFile(stateSummaryCsvPath)

		run.AssignColumn(mimcCodeHash.IsActive.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("IS_ACTIVE"), sizeMimcCodeHash))
		run.AssignColumn(mimcCodeHash.IsHashEnd.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("IS_HASH_END"), sizeMimcCodeHash))
		run.AssignColumn(mimcCodeHash.NewState.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("NEW_STATE"), sizeMimcCodeHash))
		run.AssignColumn(mimcCodeHash.CodeHashHi.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("KECCAK_HI"), sizeMimcCodeHash))
		run.AssignColumn(mimcCodeHash.CodeHashLo.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("KECCAK_LO"), sizeMimcCodeHash))
		run.AssignColumn(mimcCodeHash.IsForConsistency.GetColID(), smartvectors.RightZeroPadded(mchCt.Get("IS_FOR_CONSISTENCY"), sizeMimcCodeHash))

		run.AssignColumn(stateSummary.IsActive.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("IS_ACTIVE"), sizeStateSummary))
		run.AssignColumn(stateSummary.IsStorage.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("IS_STORAGE"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Initial.MiMCCodeHash.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("INITIAL_MIMC"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Final.MiMCCodeHash.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("FINAL_MIMC"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Initial.KeccakCodeHash.Hi.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("INITIAL_KECCAK_HI"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Final.KeccakCodeHash.Hi.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("FINAL_KECCAK_HI"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Initial.KeccakCodeHash.Lo.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("INITIAL_KECCAK_LO"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Final.KeccakCodeHash.Lo.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("FINAL_KECCAK_LO"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Initial.Exists.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("INITIAL_EXISTS"), sizeStateSummary))
		run.AssignColumn(stateSummary.Account.Final.Exists.GetColID(), smartvectors.RightZeroPadded(ssCt.Get("FINAL_EXISTS"), sizeStateSummary))

		consistency.Assign(run)
	}

	comp := wizard.Compile(define, dummy.CompileAtProverLvl())
	_ = wizard.Prove(comp, prover)

}

// TestCaseGeneration generates testdata csv and does not test anything per se.
// To re-generate the testcases, you need to unskip it.
func TestCaseGeneration(t *testing.T) {

	t.Skip()

	var (
		rng                = rand.New(utils.NewRandSource(67569)) // nolint
		shared             = [][3]field.Element{}
		numSharedRow       = 1
		numStateSummaryRow = 128
		numCodeHashRow     = 128
	)

	randRow := func() [3]field.Element {
		row := [3]field.Element{}
		row[0] = field.PseudoRand(rng)
		row[1] = field.PseudoRandTruncated(rng, 16)
		row[2] = field.PseudoRandTruncated(rng, 16)
		return row
	}

	for i := 0; i < numSharedRow; i++ {
		shared = append(shared, randRow())
	}

	var (
		ssIsActive      = make([]field.Element, 0)
		ssIsStorage     = make([]field.Element, 0)
		ssInitMimc      = make([]field.Element, 0)
		ssInitKeccakHi  = make([]field.Element, 0)
		ssInitKeccakLo  = make([]field.Element, 0)
		ssFinalMimc     = make([]field.Element, 0)
		ssFinalKeccakHi = make([]field.Element, 0)
		ssFinalKeccakLo = make([]field.Element, 0)
	)

	for i := 0; i < numStateSummaryRow; i++ {

		c := rng.IntN(4)

		if i == 0 {
			c = 0
		}

		switch {

		default:

			ssIsActive = append(ssIsActive, field.One())
			ssIsStorage = append(ssIsStorage, field.One())
			ssInitMimc = append(ssInitMimc, ssInitMimc[i-1])
			ssInitKeccakHi = append(ssInitKeccakHi, ssInitKeccakHi[i-1])
			ssInitKeccakLo = append(ssInitKeccakLo, ssInitKeccakLo[i-1])
			ssFinalMimc = append(ssFinalMimc, ssFinalMimc[i-1])
			ssFinalKeccakHi = append(ssFinalKeccakHi, ssFinalKeccakHi[i-1])
			ssFinalKeccakLo = append(ssFinalKeccakLo, ssFinalKeccakLo[i-1])

		case c == 0 || c == 1:

			var newInit, newFinal [3]field.Element

			if c == 0 {
				newInit = randRow()
				newFinal = randRow()
			}

			if c == 1 {
				fmt.Printf("row %v of state summary is shared\n", i)
				newInit = randChoose(rng, shared)
				newFinal = randChoose(rng, shared)
			}

			ssIsActive = append(ssIsActive, field.One())
			ssIsStorage = append(ssIsStorage, field.Zero())
			ssInitMimc = append(ssInitMimc, newInit[0])
			ssInitKeccakHi = append(ssInitKeccakHi, newInit[1])
			ssInitKeccakLo = append(ssInitKeccakLo, newInit[2])
			ssFinalMimc = append(ssFinalMimc, newFinal[0])
			ssFinalKeccakHi = append(ssFinalKeccakHi, newFinal[1])
			ssFinalKeccakLo = append(ssFinalKeccakLo, newFinal[2])

		}
	}

	csvtraces.WriteExplicit(
		files.MustOverwrite("./testdata/state-summary.csv"),
		[]string{"IS_ACTIVE", "IS_STORAGE", "INITIAL_MIMC", "INITIAL_KECCAK_HI", "INITIAL_KECCAK_LO", "FINAL_MIMC", "FINAL_KECCAK_HI", "FINAL_KECCAK_LO"},
		[][]field.Element{ssIsActive, ssIsStorage, ssInitMimc, ssInitKeccakHi, ssInitKeccakLo, ssFinalMimc, ssFinalKeccakHi, ssFinalKeccakLo},
		false,
	)

	/*
		For the codehash module, the rows are generated in reverse order and
		then reverted.
	*/

	var (
		romIsActive  = make([]field.Element, 0)
		romIsHashEnd = make([]field.Element, 0)
		romNewState  = make([]field.Element, 0)
		romKeccakHi  = make([]field.Element, 0)
		romKeccakLo  = make([]field.Element, 0)
	)

	for i := 0; i < numCodeHashRow; i++ {

		c0 := rng.Int64N(4)

		switch {

		case c0 == 0 || c0 == 1 || i == 0:

			row := randRow()

			if c0 == 0 {
				fmt.Printf("row %v of rom is shared\n", i)
				row = randChoose(rng, shared)
			}

			romIsActive = append(romIsActive, field.One())
			romIsHashEnd = append(romIsHashEnd, field.One())
			romNewState = append(romNewState, row[0])
			romKeccakHi = append(romKeccakHi, row[1])
			romKeccakLo = append(romKeccakLo, row[2])

		default:

			romIsActive = append(romIsActive, field.One())
			romIsHashEnd = append(romIsHashEnd, field.Zero())
			romNewState = append(romNewState, field.PseudoRand(rng))
			romKeccakHi = append(romKeccakHi, romKeccakHi[i-1])
			romKeccakLo = append(romKeccakLo, romKeccakLo[i-1])

		}
	}

	slices.Reverse(romIsActive)
	slices.Reverse(romIsHashEnd)
	slices.Reverse(romNewState)
	slices.Reverse(romKeccakHi)
	slices.Reverse(romKeccakLo)

	csvtraces.WriteExplicit(
		files.MustOverwrite("./testdata/mimc-codehash.csv"),
		[]string{"IS_ACTIVE", "IS_HASH_END", "NEW_STATE", "KECCAK_HI", "KECCAK_LO"},
		[][]field.Element{romIsActive, romIsHashEnd, romNewState, romKeccakHi, romKeccakLo},
		false,
	)
}

func randChoose[T any](rand *rand.Rand, slice []T) T {
	return slice[rand.IntN(len(slice))]
}

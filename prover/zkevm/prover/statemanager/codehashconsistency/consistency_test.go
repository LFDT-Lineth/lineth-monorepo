package codehashconsistency

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"
	"unsafe"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/lineacodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// TestConsistency validates the constraint logic of the codehashconsistency module.
// Note: The CSV test data was originally generated using MiMC hashes, but this test
// still passes because it verifies *consistency* between modules, not cryptographic
// correctness. The constraints check that matching Keccak hashes have identical
// Poseidon2 code hashes on both sides - the actual hash values are treated as opaque
// data.
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

func runTestcase(t *testing.T, poseidonCodeHashCsvPath, stateSummaryCsvPath string) {

	var (
		stateSummary        *statesummary.Module
		lineaCodeHashModule *lineacodehash.Module
		consistency         Module
		sizeStateSummary    = 128
		sizeCodeHashModule  = 256

		mchCt = csvtraces.MustOpenCsvFile(poseidonCodeHashCsvPath)
		ssCt  = csvtraces.MustOpenCsvFile(stateSummaryCsvPath)
	)

	define := func(b *wizard.Builder) {

		stateSummary = &statesummary.Module{
			IsActive:  b.InsertCommit(0, "SS_IS_ACTIVE", sizeStateSummary, true),
			IsStorage: b.InsertCommit(0, "SS_IS_STORAGE", sizeStateSummary, true),
			Account: statesummary.AccountPeek{
				Initial: statesummary.Account{
					KeccakCodeHash: common.NewHiLoColumns(b.CompiledIOP, sizeStateSummary, "SS_INITIAL_KECCAK"),
					Exists:         b.InsertCommit(0, "SS_INITIAL_EXISTS", sizeStateSummary, true),
				},
				Final: statesummary.Account{
					KeccakCodeHash: common.NewHiLoColumns(b.CompiledIOP, sizeStateSummary, "SS_FINAL_KECCAK"),
					Exists:         b.InsertCommit(0, "SS_FINAL_EXISTS", sizeStateSummary, true),
				},
			},
		}

		lineaCodeHashModule = &lineacodehash.Module{
			IsActive:         b.InsertCommit(0, "MCH_IS_ACTIVE", sizeCodeHashModule, true),
			IsHashEnd:        b.InsertCommit(0, "MCH_IS_HASH_END", sizeCodeHashModule, true),
			IsForConsistency: b.InsertCommit(0, "MCH_IS_FOR_CONSISTENCY", sizeCodeHashModule, true),
		}

		for i := range poseidon2.BlockSize {
			stateSummary.Account.Initial.LineaCodeHash[i] = b.InsertCommit(0, ifaces.ColIDf("SS_INITIAL_POSEIDON2_%v", i), sizeStateSummary, true)
			stateSummary.Account.Final.LineaCodeHash[i] = b.InsertCommit(0, ifaces.ColIDf("SS_FINAL_POSEIDON2_%v", i), sizeStateSummary, true)
			lineaCodeHashModule.NewState[i] = b.InsertCommit(0, ifaces.ColIDf("MCH_NEW_STATE_%v", i), sizeCodeHashModule, true)
		}

		// CodeHash is [common.NbLimbU256] columns (Keccak hash)
		for i := range common.NbLimbU256 {
			lineaCodeHashModule.CodeHash[i] = b.InsertCommit(0, ifaces.ColIDf("MCH_KECCAK_%v", i), sizeCodeHashModule, true)
		}

		consistency = NewModule(b.CompiledIOP, "CONSISTENCY", stateSummary, lineaCodeHashModule)
	}

	prover := func(run *wizard.ProverRuntime) {

		mchCt.Assign(run,
			lineaCodeHashModule.IsActive,
			lineaCodeHashModule.IsHashEnd,
			lineaCodeHashModule.IsForConsistency,
		)

		for i := range poseidon2.BlockSize {
			mchCt.Assign(run, lineaCodeHashModule.NewState[i])
			ssCt.Assign(run,
				stateSummary.Account.Initial.LineaCodeHash[i],
				stateSummary.Account.Final.LineaCodeHash[i],
			)
		}

		mchCt.AssignCols(run, lineaCodeHashModule.CodeHash[:]...)
		ssCt.AssignCols(run, stateSummary.Account.Initial.KeccakCodeHash.Hi[:]...)
		ssCt.AssignCols(run, stateSummary.Account.Final.KeccakCodeHash.Hi[:]...)
		ssCt.AssignCols(run, stateSummary.Account.Initial.KeccakCodeHash.Lo[:]...)
		ssCt.AssignCols(run, stateSummary.Account.Final.KeccakCodeHash.Lo[:]...)
		ssCt.Assign(run,
			stateSummary.IsActive,
			stateSummary.IsStorage,
			stateSummary.Account.Initial.Exists,
			stateSummary.Account.Final.Exists,
		)

		consistency.Assign(run)
	}

	comp := wizard.Compile(define, dummy.CompileAtProverLvl())
	_ = wizard.Prove(comp, prover)

}

var isForConsistencyMocked = []uint64{1, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 0, 0,
	0, 0, 0, 1, 0, 1, 1, 0, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0,
	1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0, 0, 1, 1, 0, 1, 1, 1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1,
	1, 0, 0, 0, 0, 0, 1, 1, 1, 0, 1, 0, 0, 0, 0, 0, 1, 1, 1, 0, 1, 1, 1}

// TestCaseGeneration generates testdata csv and does not test anything per se.
// To re-generate the testcases, you need to unskip it.
func TestCaseGeneration(t *testing.T) {

	t.Skip()

	isForConsistencyMockedElement := make([]field.Element, 0, len(isForConsistencyMocked))
	for _, num := range isForConsistencyMocked {
		isForConsistencyMockedElement = append(isForConsistencyMockedElement, field.NewElement(num))
	}

	var (
		rng                = rand.New(utils.NewRandSource(67569)) // nolint
		shared             = [][3][]field.Element{}
		numSharedRow       = 1
		numStateSummaryRow = 128
		numCodeHashRow     = 128
	)

	randRow := func() [3][]field.Element {
		row := [3][]field.Element{}
		row0 := GenerateRandomLimbs(common.NbLimbU256, rng)
		row1 := GenerateRandomLimbs(common.NbLimbU128, rng)
		row2 := GenerateRandomLimbs(common.NbLimbU128, rng)

		row[0] = make([]field.Element, common.NbLimbU256)
		copy(row[0], row0)
		row[1] = make([]field.Element, common.NbLimbU128)
		copy(row[1], row1)
		row[2] = make([]field.Element, common.NbLimbU128)
		copy(row[2], row2)
		return row
	}

	for i := 0; i < numSharedRow; i++ {
		shared = append(shared, randRow())
	}

	var (
		ssIsActive       = make([]field.Element, 0)
		ssIsStorage      = make([]field.Element, 0)
		ssInitPoseidon2  = make([][]field.Element, 0)
		ssInitKeccakHi   = make([][]field.Element, 0)
		ssInitKeccakLo   = make([][]field.Element, 0)
		ssFinalPoseidon2 = make([][]field.Element, 0)
		ssFinalKeccakHi  = make([][]field.Element, 0)
		ssFinalKeccakLo  = make([][]field.Element, 0)
		ssInitialExists  = make([]field.Element, 0)
		ssFinalExists    = make([]field.Element, 0)
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
			ssInitPoseidon2 = append(ssInitPoseidon2, ssInitPoseidon2[i-1])
			ssInitKeccakHi = append(ssInitKeccakHi, ssInitKeccakHi[i-1])
			ssInitKeccakLo = append(ssInitKeccakLo, ssInitKeccakLo[i-1])
			ssFinalPoseidon2 = append(ssFinalPoseidon2, ssFinalPoseidon2[i-1])
			ssFinalKeccakHi = append(ssFinalKeccakHi, ssFinalKeccakHi[i-1])
			ssFinalKeccakLo = append(ssFinalKeccakLo, ssFinalKeccakLo[i-1])

		case c == 0 || c == 1:

			var newInit, newFinal [3][]field.Element

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

			ssInitPoseidon2 = append(ssInitPoseidon2, newInit[0])
			ssInitKeccakHi = append(ssInitKeccakHi, newInit[1])
			ssInitKeccakLo = append(ssInitKeccakLo, newInit[2])
			ssFinalPoseidon2 = append(ssFinalPoseidon2, newFinal[0])
			ssFinalKeccakHi = append(ssFinalKeccakHi, newFinal[1])
			ssFinalKeccakLo = append(ssFinalKeccakLo, newFinal[2])

		}

		ssInitialExists = append(ssInitialExists, field.One())
		ssFinalExists = append(ssInitialExists, field.One())
	}

	colNamesStateSummary := []string{"IS_ACTIVE", "IS_STORAGE"}
	for i := range common.NbLimbU256 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("INITIAL_POSEIDON2_%d", i))
	}

	for i := range common.NbLimbU256 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("FINAL_POSEIDON2_%d", i))
	}

	for i := range common.NbLimbU128 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("INITIAL_KECCAK_HI_%d", i))
	}

	for i := range common.NbLimbU128 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("INITIAL_KECCAK_LO_%d", i))
	}

	for i := range common.NbLimbU128 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("FINAL_KECCAK_HI_%d", i))
	}

	for i := range common.NbLimbU128 {
		colNamesStateSummary = append(colNamesStateSummary, fmt.Sprintf("FINAL_KECCAK_LO_%d", i))
	}

	colNamesStateSummary = append(colNamesStateSummary, "INITIAL_EXISTS", "FINAL_EXISTS")

	var colValuesStateSummary = [][]field.Element{ssIsActive, ssIsStorage}
	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssInitPoseidon2)...)
	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssFinalPoseidon2)...)

	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssInitKeccakHi)...)
	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssInitKeccakLo)...)
	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssFinalKeccakHi)...)
	colValuesStateSummary = append(colValuesStateSummary, transposeLimbs(ssFinalKeccakLo)...)

	colValuesStateSummary = append(colValuesStateSummary, ssInitialExists)
	colValuesStateSummary = append(colValuesStateSummary, ssFinalExists)

	csvtraces.WriteExplicitFromKoala(
		files.MustOverwrite("./testdata/state-summary.csv"),
		colNamesStateSummary,
		colValuesStateSummary,
		false,
	)

	/*
		For the codehash module, the rows are generated in reverse order and
		then reverted.
	*/

	var (
		romIsActive  = make([]field.Element, 0)
		romIsHashEnd = make([]field.Element, 0)
		romNewState  = make([][]field.Element, 0)
		romKeccak    = make([][]field.Element, 0)
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
			romKeccak = append(romKeccak, append(row[1], row[2]...))
		default:

			romIsActive = append(romIsActive, field.One())
			romIsHashEnd = append(romIsHashEnd, field.Zero())
			romNewState = append(romNewState, GenerateRandomLimbs(common.NbLimbU256, rng))
			romKeccak = append(romKeccak, romKeccak[i-1])

		}
	}

	slices.Reverse(romIsActive)
	slices.Reverse(romIsHashEnd)
	slices.Reverse(romNewState)
	slices.Reverse(romKeccak)

	colNamesPoseidonCodehash := []string{"IS_FOR_CONSISTENCY", "IS_ACTIVE", "IS_HASH_END"}
	for i := range common.NbLimbU256 {
		colNamesPoseidonCodehash = append(colNamesPoseidonCodehash, fmt.Sprintf("NEW_STATE_%d", i))
	}

	for i := range common.NbLimbU256 {
		colNamesPoseidonCodehash = append(colNamesPoseidonCodehash, fmt.Sprintf("KECCAK_%d", i))
	}

	var colValuesPoseidonCodehash = [][]field.Element{isForConsistencyMockedElement, romIsActive, romIsHashEnd}
	colValuesPoseidonCodehash = append(colValuesPoseidonCodehash, transposeLimbs(romNewState)...)
	colValuesPoseidonCodehash = append(colValuesPoseidonCodehash, transposeLimbs(romKeccak)...)

	csvtraces.WriteExplicitFromKoala(
		files.MustOverwrite("./testdata/mimc-codehash.csv"),
		colNamesPoseidonCodehash,
		colValuesPoseidonCodehash,
		false,
	)
}

func randChoose[T any](rand *rand.Rand, slice []T) T {
	return slice[rand.IntN(len(slice))]
}

// GenerateRandomLimbs generates a slice of random limbs of the specified limbs number.
func GenerateRandomLimbs(limbs int, rng *rand.Rand) []field.Element {
	var (
		bigInt    = &big.Int{}
		res       = make([]field.Element, limbs)
		bareU64   = [4]uint64{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
		bareBytes = *(*[32]byte)(unsafe.Pointer(&bareU64))
	)

	bigInt.SetBytes(bareBytes[:limbs*common.LimbBytes]).Mod(bigInt, field.Modulus())

	limbBytes := common.SplitBytes(bareBytes[:])
	for i := range limbs {
		res[i].SetBytes(limbBytes[i])
	}

	return res
}

func transposeLimbs(inputMatrix [][]field.Element) [][]field.Element {
	if len(inputMatrix) == 0 || len(inputMatrix[0]) == 0 {
		return [][]field.Element{}
	}

	rows := len(inputMatrix)
	cols := len(inputMatrix[0])

	outputMatrix := make([][]field.Element, cols)
	for i := range outputMatrix {
		outputMatrix[i] = make([]field.Element, rows)
	}

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			outputMatrix[j][i] = inputMatrix[i][j]
		}
	}
	return outputMatrix
}

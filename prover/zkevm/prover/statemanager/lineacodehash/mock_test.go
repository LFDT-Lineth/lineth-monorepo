package lineacodehash

import (
	"fmt"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/require"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

// runCodeHashModule builds and proves the module over the given romlex fixture
// and returns the assignments of IsHashEnd and IsForConsistency.
func runCodeHashModule(t *testing.T, romLexPath string) (isHashEnd, isForConsistency []field.Element) {
	t.Helper()

	romFile, errRom := os.Open("testdata/rom_input.csv")
	if errRom != nil {
		t.Fatal(errRom)
	}
	romLexFile, errRomLex := os.Open(romLexPath)
	if errRomLex != nil {
		t.Fatal(errRomLex)
	}
	defer romFile.Close()
	defer romLexFile.Close()

	ctRom, errRom := csvtraces.NewCsvTrace(romFile)
	ctRomLex, errRomLex := csvtraces.NewCsvTrace(romLexFile)
	if errRom != nil {
		t.Fatal(errRom)
	}
	if errRomLex != nil {
		t.Fatal(errRomLex)
	}

	var (
		romInput    *RomInput
		romLexInput = &RomLexInput{}
		mod         Module
	)

	cmp := wizard.Compile(func(build *wizard.Builder) {

		// Define romInput
		romInput = &RomInput{
			NBytes:  ctRom.GetCommit(build, "NBYTES"),
			Counter: ctRom.GetCommit(build, "COUNTER"),
		}

		for i := range common.NbLimbU128 {
			romInput.Acc[i] = ctRom.GetCommit(build, fmt.Sprintf("ACC_%d", i))
		}

		for i := range common.NbLimbU32 {
			romInput.CFI[i] = ctRom.GetCommit(build, fmt.Sprintf("CFI_%d", i))
			romInput.CodeSize[i] = ctRom.GetCommit(build, fmt.Sprintf("CODESIZE_%d", i))
		}

		// Define romLexInput
		for i := range common.NbLimbU256 {
			romLexInput.CodeHash[i] = ctRomLex.GetCommit(build, fmt.Sprintf("CODEHASH_%d", i))
		}

		for i := range common.NbLimbU32 {
			romLexInput.CFIRomLex[i] = ctRomLex.GetCommit(build, fmt.Sprintf("CFI_ROMLEX_%d", i))
		}

		mod = NewModule(
			build.CompiledIOP,
			Inputs{
				Name: "POSEIDON2_CODE_HASH_TEST",
				Size: 1 << 13,
			},
		)

		// Check the consistency of different input connection via projection and lookup queries.
		mod.ConnectToRom(build.CompiledIOP, romInput, romLexInput)
	}, dummy.Compile)

	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		ctRom.AssignCols(run, romInput.CFI[:]...).
			AssignCols(run, romInput.Acc[:]...).
			AssignCols(run, romInput.NBytes, romInput.Counter).
			AssignCols(run, romInput.CodeSize[:]...)

		romInput.completeAssign(run)
		ctRomLex.AssignCols(run, romLexInput.CFIRomLex[:]...).
			AssignCols(run, romLexInput.CodeHash[:]...)

		mod.Assign(run)

		isHashEnd = mod.IsHashEnd.GetColAssignment(run).IntoRegVecSaveAlloc()
		isForConsistency = mod.IsForConsistency.GetColAssignment(run).IntoRegVecSaveAlloc()

		romInput := mod.InputModules.RomInput

		ctRom.CheckAssignmentCols(run, romInput.CFI[:]...).
			CheckAssignmentCols(run, romInput.Acc[:]...).
			CheckAssignmentCols(run, romInput.NBytes, romInput.Counter).
			CheckAssignmentCols(run, romInput.CodeSize[:]...)
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	return isHashEnd, isForConsistency
}

func countOnes(col []field.Element) int {
	n := 0
	for i := range col {
		if col[i].IsOne() {
			n++
		}
	}
	return n
}

func TestMiMCCodeHash(t *testing.T) {
	runCodeHashModule(t, "testdata/romlex_input.csv")
}

// TestCodeHashEmptyKeccakLimbs checks that IsForConsistency is decided by the
// whole codehash rather than by individual limbs. The fixture holds three
// codehashes: two matching emptyKeccak on a single limb (0 and 11) and one
// matching on all of them, so only the latter is filtered out.
func TestCodeHashEmptyKeccakLimbs(t *testing.T) {
	isHashEnd, isForConsistency := runCodeHashModule(t, "testdata/romlex_input_empty_keccak_limbs.csv")

	require.Equal(t, 3, countOnes(isHashEnd), "expected one hash-end per CFI segment")
	require.Equal(t, 2, countOnes(isForConsistency), "only the all-limbs match must be filtered out")
}

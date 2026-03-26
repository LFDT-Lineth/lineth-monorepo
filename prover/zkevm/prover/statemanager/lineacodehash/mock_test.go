package lineacodehash

import (
	"fmt"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func TestMiMCCodeHash(t *testing.T) {
	romFile, errRom := os.Open("testdata/rom_input.csv")
	if errRom != nil {
		t.Fatal(errRom)
	}
	romLexFile, errRomLex := os.Open("testdata/romlex_input.csv")
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

		romInput := mod.InputModules.RomInput

		ctRom.CheckAssignmentCols(run, romInput.CFI[:]...).
			CheckAssignmentCols(run, romInput.Acc[:]...).
			CheckAssignmentCols(run, romInput.NBytes, romInput.Counter).
			CheckAssignmentCols(run, romInput.CodeSize[:]...)
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}

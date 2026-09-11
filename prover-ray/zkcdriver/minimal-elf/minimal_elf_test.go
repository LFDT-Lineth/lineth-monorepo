package minimalelf

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

var (
	zkcCfg = vm.DEFAULT_TRACE_CONFIG
)

// TestRisc5InstructionCoverageGuest traces minimalelf.AllInOneElfProgram
// through the real main.zkc interpreter and checks the resulting trace against
// every compiled constraint.
//
// See [minimalelf] package for the definition of AllInOneElfProgram.
func TestRisc5InstructionCoverageGuest(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}

	inputsMap, err := predecoding.PrepareInputs(AllInOneElfProgram, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	// trace program with given input
	outputs, tr, errs := binf.Trace(inputsMap, zkcCfg)
	if len(errs) > 0 {
		t.Fatalf("could not trace the binary file: %v", errors.Join(errs...))
	}
	// check the traces work
	if errsSchema := binf.Check(zkcCfg, tr); len(errsSchema) > 0 {
		errs := make([]error, len(errsSchema))
		for i, e := range errsSchema {
			errs[i] = errors.New(e.Message())
		}
		t.Fatalf("constraint check failed: %v", errors.Join(errs...))
	}

	got, ok := outputs["guest_output"]
	if !ok {
		t.Fatalf("expected a %q output, got outputs: %v", "guest_output", outputs)
	}
	want := []byte{'A', 'B', 'C'}
	if string(got) != string(want) {
		t.Fatalf("guest_output = %q, want %q", got, want)
	}
}

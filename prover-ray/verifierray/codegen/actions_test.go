package codegen

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// unknownVerifierAction is a verifier action the codegen has no Zig sub-verifier for.
type unknownVerifierAction struct{}

func (unknownVerifierAction) Check(*wiop.Runtime) error { return nil }

// TestBuildCompiledSystemRejectsUnhandledAction checks that BuildCompiledSystem, which every
// asset path goes through (including `prover verifier-assets` without --image), refuses a
// system whose verifier action it would otherwise drop from the generated verifier.
func TestBuildCompiledSystemRejectsUnhandledAction(t *testing.T) {
	sys, err := devmode.NewSystem()
	if err != nil {
		t.Fatalf("devmode.NewSystem: %v", err)
	}
	if _, err := BuildCompiledSystem(sys); err != nil {
		t.Fatalf("BuildCompiledSystem on the unmodified devmode system: %v", err)
	}

	last := sys.Rounds[len(sys.Rounds)-1]
	last.RegisterVerifierAction(unknownVerifierAction{})

	_, err = BuildCompiledSystem(sys)
	var unhandled *UnhandledVerifierActionError
	if !errors.As(err, &unhandled) {
		t.Fatalf("BuildCompiledSystem error = %v, want an UnhandledVerifierActionError", err)
	}
	if unhandled.Round != last.ID {
		t.Fatalf("UnhandledVerifierActionError.Round = %d, want %d", unhandled.Round, last.ID)
	}
}

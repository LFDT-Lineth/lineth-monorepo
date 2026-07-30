package codegen

import (
	"errors"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
)

// TestAssertAllVerifierActionsHandledAcceptsKnown confirms the fail-closed guard
// passes a protocol whose only verifier actions are ones the codegen emits
// (global.Verifier from a vanishing constraint, plus the pcs opening action).
func TestAssertAllVerifierActionsHandledAcceptsKnown(t *testing.T) {
	sys, _ := newCommittedVanishing(t)
	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		t.Fatalf("AssertAllVerifierActionsHandled rejected a fully-handled protocol: %v", err)
	}
}

// TestAssertAllVerifierActionsHandledRejectsGrandProduct is the soundness gate:
// a permutation lowered through grandproduct registers CheckResultIsOne /
// FinalProductCheck — boundary identities the verifier-ray codegen does NOT
// emit. Without the guard those checks would be silently dropped and a
// non-permutation witness could be accepted. The guard must reject at
// generation time.
func TestAssertAllVerifierActionsHandledRejectsGrandProduct(t *testing.T) {
	sys := wiop.NewSystemf("perm-guard")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	// Lower the permutation to a grand product; this registers the
	// CheckResultIsOne / FinalProductCheck verifier actions.
	grandproduct.Compile(sys)
	global.Compile(sys)

	err := AssertAllVerifierActionsHandled(sys)
	if err == nil {
		t.Fatalf("AssertAllVerifierActionsHandled accepted a grandproduct protocol whose " +
			"boundary check the codegen does not emit — that is a silent soundness drop")
	}
	var unhandled *UnhandledVerifierActionError
	if !errors.As(err, &unhandled) {
		t.Fatalf("expected UnhandledVerifierActionError, got %T: %v", err, err)
	}
	if !strings.Contains(unhandled.Type, "grandproduct") {
		t.Fatalf("expected the unhandled action to be a grandproduct action, got %q", unhandled.Type)
	}
}

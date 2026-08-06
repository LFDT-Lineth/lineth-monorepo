package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// UnhandledVerifierActionError reports a registered wiop verifier action that
// the verifier-ray codegen does not translate into any Zig sub-verifier. It
// carries the Go type of the offending action so the fix is obvious.
type UnhandledVerifierActionError struct {
	// Type is the fully-qualified Go type name of the unhandled action.
	Type string
	// Round is the wiop round the action was registered on.
	Round int
}

func (e *UnhandledVerifierActionError) Error() string {
	return fmt.Sprintf(
		"codegen: round %d registers verifier action %s, which the verifier-ray codegen "+
			"does not emit — the check it enforces would be SILENTLY DROPPED. Add a Zig "+
			"sub-verifier + codegen for it, or extend AssertAllVerifierActionsHandled's "+
			"allowlist only if it is genuinely lowered to already-emitted constraints.",
		e.Round, e.Type)
}

// AssertAllVerifierActionsHandled fails CLOSED: it walks every verifier action
// registered on `sys` and returns an UnhandledVerifierActionError for any action
// whose enforced check the verifier-ray codegen does not translate into a Zig
// sub-verifier.
//
// This exists because each Build*System pass filters for ONLY its own action
// type and silently skips the rest (vanishing.go / logderivativesum.go / pcs.go
// all `continue` past unrecognized actions). Without this guard a protocol that
// registers, e.g., grandproduct.FinalProductCheck (∏Z == Result) or
// grandproduct.CheckResultIsOne (permutation Result == 1) or
// messagebus.CheckHandleSumInShard would compile to a Zig verifier that never
// enforces that boundary identity — a SOUNDNESS hole where a false grand-product
// / non-permutation statement is accepted. Callers assembling a CompiledSystem
// MUST invoke this before emitting; it turns "silently unenforced" into a loud
// generation-time error.
//
// The allowlist is the set of actions the Zig verifier DOES enforce:
//   - global.Verifier                          → vanishing (+ PCS claim link)
//   - logderivativesum.VerifierAction          → logderivativesum boundary sum
//   - lookuptologderivsum.ResultIsZeroVerifierAction → logderivativesum result-is-zero
//   - pcs.OpeningVerifierAction                 → BuildPcsSystem (performs no
//     boundary check the Zig side must re-emit — the whole PCS opening is
//     reconstructed by BuildPcsSystem from the committed batches and
//     LagrangeEvals — so it is handled implicitly)
//
// Any other action type — including new ones added to prover-ray later — trips
// the error, forcing an explicit decision rather than a silent drop.
func AssertAllVerifierActionsHandled(sys *wiop.System) error {
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if verifierActionIsHandled(action) {
				continue
			}
			return &UnhandledVerifierActionError{
				Type:  fmt.Sprintf("%T", action),
				Round: round.ID,
			}
		}
	}
	return nil
}

func verifierActionIsHandled(action wiop.VerifierAction) bool {
	switch action.(type) {
	case *global.Verifier:
		return true
	case *logderivativesum.VerifierAction:
		return true
	case *lookuptologderivsum.ResultIsZeroVerifierAction:
		return true
	case *pcs.OpeningVerifierAction:
		return true
	}
	return false
}

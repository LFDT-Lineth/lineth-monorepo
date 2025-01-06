package wizard

import "github.com/consensys/gnark/frontend"

// ProverAction represents an action to be performed by the prover.
// They have to be registered in the [CompiledIOP] via the
// [CompiledIOP.RegisterProverAction]
type ProverAction interface {
	// Run executes the ProverAction over a [ProverRuntime]
	Run(*ProverRuntime)
}

// VerifierAction represents an action to be performed by the verifier of the
// protocol. Usually, this is used to represent verifier checks. They can be
// registered via [CompiledIOP.RegisterVerifierAction].
type VerifierAction interface {
	// Skip indicates that the verifier action can be skipped
	Skip()
	// IsSkipped returns whether the current VerifierAction is skipped
	IsSkipped() bool
	// Run executes the VerifierAction over a [VerifierRuntime] it returns an
	// error.
	Run(*VerifierRuntime) error
	// RunGnark is as Run but in a gnark circuit. Instead, of the returning an
	// error the function enforces the passing of the verifier's checks.
	RunGnark(frontend.API, *WizardVerifierCircuit)
}

// genVerifierAction represents a verifier action represented by closures
type genVerifierAction struct {
	skipped  bool
	run      func(*VerifierRuntime) error
	runGnark func(frontend.API, *WizardVerifierCircuit)
}

func (gva *genVerifierAction) Run(run *VerifierRuntime) error {
	return gva.run(run)
}

func (gva *genVerifierAction) RunGnark(api frontend.API, run *WizardVerifierCircuit) {
	gva.runGnark(api, run)
}

func (gva *genVerifierAction) Skip() {
	gva.skipped = true
}

func (gva *genVerifierAction) IsSkipped() bool {
	return gva.skipped
}

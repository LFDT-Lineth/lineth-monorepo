package wizard

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
)

// ProverAction represents an action to be performed by the prover.
// They have to be registered in the [CompiledIOP] via the
// [CompiledIOP.RegisterProverAction]
type ProverAction interface {
	// Run executes the ProverAction over a [ProverRuntime]
	Run(*ProverRuntime)
}

// mainProverStepWrapper adapts a  MainProverStep to the ProverAction interface.
type mainProverStepWrapper struct {
	step func(*ProverRuntime)
}

// Run implements the ProverAction interface for MainProverStep.
func (w mainProverStepWrapper) Run(run *ProverRuntime) {
	w.step(run)
}

// VerifierAction represents an action to be performed by the verifier of the
// protocol. Usually, this is used to represent verifier checks. They can be
// registered via [CompiledIOP.RegisterVerifierAction].
type VerifierAction interface {
	// Run executes the VerifierAction over a [VerifierRuntime] it returns an
	// error.
	Run(Runtime) error
	// RunGnark is as Run but in a gnark circuit. Instead, of the returning an
	// error the function enforces the passing of the verifier's checks.
	RunGnark(frontend.API, GnarkRuntime)
}

// PrintingProverAction is a ProverAction printing a column content. And an
// additional message.
type PrintingProverAction struct {
	Column          ifaces.Column
	Message         string
	NameReplacement string
}

// PrintingVerifierAction is as PrintingProverAction but for the verifier.
type PrintingVerifierAction struct {
	Column          ifaces.Column
	Message         string
	NameReplacement string
}

// Run implements the ProverAction interface for PrintingProverAction.
func (p PrintingProverAction) Run(run *ProverRuntime) {

	name := p.Column.GetColID()
	if len(p.NameReplacement) > 0 {
		name = ifaces.ColID(p.NameReplacement)
	}

	c := p.Column.GetColAssignment(run)
	fmt.Printf("name=%v message=%v value=%v\n", name, p.Message, c.Pretty())
}

// Run implements the VerifierAction interface for PrintingVerifierAction.
func (p PrintingVerifierAction) Run(run Runtime) error {

	name := p.Column.GetColID()
	if len(p.NameReplacement) > 0 {
		name = ifaces.ColID(p.NameReplacement)
	}

	c := p.Column.GetColAssignment(run)
	fmt.Printf("name=%v message=%v value=%v\n", name, p.Message, c.Pretty())
	return nil
}

// RunGnark implements the VerifierAction interface for PrintingVerifierAction.
// func (p PrintingVerifierAction) RunGnark(api frontend.API, run GnarkRuntime) {

// 	name := p.Column.GetColID()
// 	if len(p.NameReplacement) > 0 {
// 		name = ifaces.ColID(p.NameReplacement)
// 	}

// 	c := p.Column.GetColAssignmentGnark(run)
// 	names := fmt.Sprintf("name=%v message=%v value=\n", name, p.Message)
// 	api.Println(append([]koalagnark.Var{names}, c...)...)
// }

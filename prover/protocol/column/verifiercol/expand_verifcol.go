package verifiercol

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = ExpandedVerifCol{}

type ExpandedVerifCol struct {
	Verifiercol VerifierCol
	Expansion   int
}

// Round returns the round ID of the column and implements the [ifaces.Column]
// interface.
func (ex ExpandedVerifCol) Round() int {
	return ex.Verifiercol.Round()
}

// GetColID returns the column ID
func (ex ExpandedVerifCol) GetColID() ifaces.ColID {
	return ifaces.ColIDf("Expanded_%v_%v", ex.Verifiercol.GetColID(), ex.Expansion)
}

// MustExists implements the [ifaces.Column] interface and always returns true.
func (ex ExpandedVerifCol) MustExists() {
	ex.Verifiercol.MustExists()
}

// Size returns the size of the colum and implements the [ifaces.Column]
// interface.
func (ex ExpandedVerifCol) Size() int {
	return ex.Verifiercol.Size() * ex.Expansion
}

// GetColAssignment returns the assignment of the current column
func (ex ExpandedVerifCol) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	assi := ex.Verifiercol.GetColAssignment(run)
	values := make([][]field.Element, ex.Expansion)
	for j := range values {
		values[j] = smartvectors.IntoRegVec(assi)
	}
	res := vector.Interleave(values...)
	return smartvectors.NewRegular(res)
}

// GetColAssignment returns a gnark assignment of the current column
func (ex ExpandedVerifCol) GetColAssignmentGnark(run ifaces.GnarkRuntime) []frontend.Variable {
	assi := ex.Verifiercol.GetColAssignmentGnark(run)
	res := make([]frontend.Variable, ex.Size())
	for i := 0; i < len(assi); i++ {
		for j := 0; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = assi[i]
		}
	}
	return res
}

// GetColAssignmentAt returns a particular position of the column
func (ex ExpandedVerifCol) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	return ex.Verifiercol.GetColAssignmentAt(run, pos/ex.Expansion)
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (ex ExpandedVerifCol) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) frontend.Variable {
	return ex.Verifiercol.GetColAssignmentGnarkAt(run, pos/ex.Expansion)
}

// IsComposite implements the [ifaces.Column] interface
func (ex ExpandedVerifCol) IsComposite() bool {
	return ex.Verifiercol.IsComposite()
}

// String implements the [symbolic.Metadata] interface
func (ex ExpandedVerifCol) String() string {
	return string(ex.GetColID())
}

// Split implements the [VerifierCol] interface
func (ex ExpandedVerifCol) Split(_ *wizard.CompiledIOP, from, to int) ifaces.Column {
	return ex.Verifiercol
}

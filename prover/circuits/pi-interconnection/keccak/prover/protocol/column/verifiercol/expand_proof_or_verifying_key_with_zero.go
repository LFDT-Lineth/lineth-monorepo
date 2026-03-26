package verifiercol

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = ExpandedProofOrVerifyingKeyColWithZero{}

type ExpandedProofOrVerifyingKeyColWithZero struct {
	Col       ifaces.Column
	Expansion int
}

// Round returns the round ID of the column and implements the [ifaces.Column]
// interface.
func (ex ExpandedProofOrVerifyingKeyColWithZero) Round() int {
	return ex.Col.Round()
}

// GetColID returns the column ID
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColID() ifaces.ColID {
	return ifaces.ColIDf("EXPANDED_%v_%v_WITH_ZERO", ex.Col.GetColID(), ex.Expansion)
}

// MustExists implements the [ifaces.Column] interface and always returns true.
func (ex ExpandedProofOrVerifyingKeyColWithZero) MustExists() {
	ex.Col.MustExists()
}

// Size returns the size of the colum and implements the [ifaces.Column]
// interface.
func (ex ExpandedProofOrVerifyingKeyColWithZero) Size() int {
	return ex.Col.Size() * ex.Expansion
}

// GetColAssignment returns the assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	assi := ex.Col.GetColAssignment(run)
	values := make([][]field.Element, ex.Expansion)
	values[0] = smartvectors.IntoRegVec(assi)
	// zeros denote slice of zeros of length of the verifier column
	zeros := make([]field.Element, ex.Col.Size())
	// we start at 1 because the first slice is the actual verifier column
	// the other slices are zeros
	for j := 1; j < ex.Expansion; j++ {
		values[j] = zeros
	}
	res := vector.Interleave(values...)
	return smartvectors.NewRegular(res)
}

// GetColAssignment returns a gnark assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnark(run ifaces.GnarkRuntime) []frontend.Variable {
	assi := ex.Col.GetColAssignmentGnark(run)
	res := make([]frontend.Variable, ex.Size())
	for i := 0; i < len(assi); i++ {
		res[i*ex.Expansion] = assi[i]
		for j := 1; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = frontend.Variable(0)
		}
	}
	return res
}

// GetColAssignmentAt returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentAt(run, pos/ex.Expansion)
	}
	return field.Zero()
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) frontend.Variable {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentGnarkAt(run, pos/ex.Expansion)
	}
	return frontend.Variable(0)
}

// IsComposite implements the [ifaces.Column] interface
func (ex ExpandedProofOrVerifyingKeyColWithZero) IsComposite() bool {
	return ex.Col.IsComposite()
}

// String implements the [symbolic.Metadata] interface
func (ex ExpandedProofOrVerifyingKeyColWithZero) String() string {
	return string(ex.GetColID())
}

// Split implements the [VerifierCol] interface,
// it is no-op for proofcols
func (ex ExpandedProofOrVerifyingKeyColWithZero) Split(_ *wizard.CompiledIOP, from, to int) ifaces.Column {
	return ex.Col
}

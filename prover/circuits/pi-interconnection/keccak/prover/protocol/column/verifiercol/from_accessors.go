package verifiercol

import (
	"fmt"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = FromAccessors{}

// FromAccessors is a [VerifierCol] implementation that is built by stacking the
// values of [ifaces.Accessor] on top of each others. It is the most type of
// column and subsumes almost all the other types.
type FromAccessors struct {
	// Accessors stores the list of accessors building the column.
	Accessors []ifaces.Accessor
	Padding   field.Element
	Size_     int
	// Round_ caches the round value of the column.
	Round_ int
}

// NewFromAccessors instantiates a [FromAccessors] column from a list of
// [ifaces.Accessor]. The column's size must be a power of 2.
//
// You should not pass accessors of type [expressionAsAccessor] as their
// evaluation within a gnark circuit requires using the frontend.API which we
// can't access in the context currently.
func NewFromAccessors(accessors []ifaces.Accessor, padding field.Element, size int) ifaces.Column {
	if !utils.IsPowerOfTwo(size) {
		utils.Panic("the column must be a power of two (size=%v)", size)
	}
	round := 0
	for i := range accessors {
		round = max(round, accessors[i].Round())
	}
	return FromAccessors{Accessors: accessors, Round_: round, Padding: padding, Size_: size}
}

// Round returns the round ID of the column and implements the [ifaces.Column]
// interface.
func (f FromAccessors) Round() int {
	return f.Round_
}

// GetColID returns the column ID
func (f FromAccessors) GetColID() ifaces.ColID {
	var sb strings.Builder
	sb.Grow(64 + len(f.Accessors)*16) // Reserve some capacity

	sb.WriteString("FROM_ACCESSORS_")
	for i, acc := range f.Accessors {
		if i > 0 {
			sb.WriteByte('_')
		}
		sb.WriteString(acc.Name())
	}
	sb.WriteString("_PADDING=")
	sb.WriteString(f.Padding.String())
	sb.WriteString("_SIZE=")
	sb.WriteString(fmt.Sprintf("%v", f.Size_))

	return ifaces.ColID(sb.String())
}

// MustExists implements the [ifaces.Column] interface and always returns true.
func (f FromAccessors) MustExists() {}

// Size returns the size of the colum and implements the [ifaces.Column]
// interface.
func (f FromAccessors) Size() int {
	return f.Size_
}

// GetColAssignment returns the assignment of the current column
func (f FromAccessors) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	res := make([]field.Element, len(f.Accessors))
	for i := range res {
		res[i] = f.Accessors[i].GetVal(run)
	}
	return smartvectors.RightPadded(res, f.Padding, f.Size_)
}

// GetColAssignment returns a gnark assignment of the current column
func (f FromAccessors) GetColAssignmentGnark(run ifaces.GnarkRuntime) []frontend.Variable {

	res := make([]frontend.Variable, f.Size_)
	for i := range f.Accessors {
		res[i] = f.Accessors[i].GetFrontendVariable(nil, run)
	}

	for i := len(f.Accessors); i < f.Size_; i++ {
		res[i] = f.Padding
	}

	return res
}

// GetColAssignmentAt returns a particular position of the column
func (f FromAccessors) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {

	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		return f.Padding
	}

	return f.Accessors[pos].GetVal(run)
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (f FromAccessors) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) frontend.Variable {
	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		return f.Padding
	}

	return f.Accessors[pos].GetFrontendVariable(nil, run)
}

// IsComposite implements the [ifaces.Column] interface
func (f FromAccessors) IsComposite() bool {
	return false
}

// String implements the [symbolic.Metadata] interface
func (f FromAccessors) String() string {
	return string(f.GetColID())
}

// Split implements the [VerifierCol] interface
func (f FromAccessors) Split(_ *wizard.CompiledIOP, from, to int) ifaces.Column {

	if from >= len(f.Accessors) {
		// The reason we don't want to remove the size from the name here is that
		// these columns tend to only exist as compilation artefacts.
		return NewConstantCol(f.Padding, to-from, "")
	}

	var subAccessors = f.Accessors[from:]

	if to < len(f.Accessors) {
		subAccessors = f.Accessors[from:to]
	}

	// We don't call the accessor to ensure that the segment has the same round
	// definition as the original column.
	return FromAccessors{
		Accessors: subAccessors,
		Round_:    f.Round_,
		Padding:   f.Padding,
		Size_:     to - from,
	}
}

func (f FromAccessors) GetFromAccessorsFields() (accs []ifaces.Accessor, padding field.Element) {
	return f.Accessors, f.Padding
}

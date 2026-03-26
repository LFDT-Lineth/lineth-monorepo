package verifiercol

import (
	"fmt"
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = FromAccessors{}

// FromAccessors is a [VerifierCol] implementation that is built by stacking the
// values of [ifaces.Accessor] on top of each others. It is the most type of
// column and subsumes almost all the other types.
type FromAccessors struct {
	// Accessors stores the list of accessors building the column.
	Accessors []ifaces.Accessor
	Padding   fext.Element
	Size_     int
	// Round_ caches the round value of the column.
	Round_ int
}

func (f FromAccessors) IsBase() bool {
	isExt := false
	for i := range f.Accessors {
		if !f.Accessors[i].IsBase() {
			isExt = true
			break
		}
	}
	return !isExt
}

func (f FromAccessors) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {

	if pos < 0 || pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		baseElem, errBase := fext.GetBase(&f.Padding)
		if errBase != nil {
			return field.Zero(), fmt.Errorf("From accessors, the padding is not base")
		}
		return baseElem, nil
	}

	return f.Accessors[pos].GetValBase(run)
}

func (f FromAccessors) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {

	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		return f.Padding
	}

	return f.Accessors[pos].GetValExt(run)
}

func (f FromAccessors) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	if !f.IsBase() {
		panic("From accessors, the column is not base")
	} else {
		panic("call GetColAssignmentGnarkExt instead")
		// res := make([]koalagnark.Var, f.Size_)
		// for i := range f.Accessors {
		// 	res[i] = f.Accessors[i].GetFrontendVariable(nil, run)
		// }

		// for i := len(f.Accessors); i < f.Size_; i++ {
		// 	// res[i] = f.Padding
		// 	res[i] = circuit.NewVar(f.Padding.B0.A0.String()) // @thomas fixme (should be ext)
		// }
		// return res, nil
	}

}

func (f FromAccessors) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	res := make([]koalagnark.Ext, f.Size_)
	for i := range f.Accessors {
		res[i] = f.Accessors[i].GetFrontendVariableExt(nil, run)
	}

	padding := koalagnark.NewExt(f.Padding)
	for i := len(f.Accessors); i < f.Size_; i++ {
		res[i] = padding
	}

	return res
}

func (f FromAccessors) GetColAssignmentGnarkExtAsPtr(run ifaces.GnarkRuntime) []*koalagnark.Ext {
	res := make([]*koalagnark.Ext, f.Size_)
	for i := range f.Accessors {
		v := f.Accessors[i].GetFrontendVariableExt(nil, run)
		res[i] = &v
	}

	padding := koalagnark.NewExt(f.Padding)
	for i := len(f.Accessors); i < f.Size_; i++ {
		res[i] = &padding
	}

	return res
}

func (f FromAccessors) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	if !f.IsBase() {
		utils.Panic("From accessors, the column is not base")
	}
	// we know that we are dealing with all base elements
	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		// return f.Padding, nil
		return koalagnark.NewElementFromKoala(f.Padding.B0.A0), nil

	}

	return f.Accessors[pos].GetFrontendVariable(nil, run), nil
}

func (f FromAccessors) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		return koalagnark.NewExt(f.Padding)
	}

	return f.Accessors[pos].GetFrontendVariableExt(nil, run)
}

// NewFromAccessors instantiates a [FromAccessors] column from a list of
// [ifaces.Accessor]. The column's size must be a power of 2.
//
// You should not pass accessors of type [expressionAsAccessor] as their
// evaluation within a gnark circuit requires using the frontend.API which we
// can't access in the context currently.
func NewFromAccessors(accessors []ifaces.Accessor, padding fext.Element, size int) ifaces.Column {
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
	accessorNames := make([]string, len(f.Accessors))
	for i := range f.Accessors {
		accessorNames[i] = f.Accessors[i].Name()
	}
	return ifaces.ColIDf("FROM_ACCESSORS_%v_PADDING=%v_SIZE=%v", strings.Join(accessorNames, "_"), f.Padding.String(), f.Size_)
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
	res := make([]fext.Element, len(f.Accessors))
	for i := range res {
		res[i] = f.Accessors[i].GetValExt(run)
	}
	return smartvectors.RightPaddedExt(res, f.Padding, f.Size_)
}

// GetColAssignment returns a gnark assignment of the current column
func (f FromAccessors) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {

	res := make([]koalagnark.Element, f.Size_)
	for i := range f.Accessors {
		res[i] = f.Accessors[i].GetFrontendVariable(nil, run)
	}

	for i := len(f.Accessors); i < f.Size_; i++ {
		// res[i] = f.Padding
		res[i] = koalagnark.NewElementFromKoala(f.Padding.B0.A0) // TODO @thomas fixme
	}

	return res
}

// GetColAssignmentAt returns a particular position of the column
func (f FromAccessors) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	panic("deprecated, use GetColAssignmentAtBase or GetColAssignmentAtExt")
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (f FromAccessors) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	if pos >= f.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", f.Size_, pos)
	}

	if pos >= len(f.Accessors) {
		// return f.Padding
		return koalagnark.NewElementFromKoala(f.Padding.B0.A0) // TODO @thomas fixme
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
		return NewConstantColExt(f.Padding, to-from, "")
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

func (f FromAccessors) GetFromAccessorsFields() (accs []ifaces.Accessor, padding fext.Element) {
	return f.Accessors, f.Padding
}

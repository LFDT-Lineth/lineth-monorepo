package verifiercol

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Represents a constant column
type ConstCol struct {
	F     fext.GenericFieldElem
	Size_ int
	Name  string
}

// NewConstantCol creates a new ConstCol column
func NewConstantCol(elem field.Element, size int, name string) ifaces.Column {
	return ConstCol{
		F:     fext.NewGenFieldFromBase(elem),
		Size_: size,
		Name:  name,
	}
}

func NewConstantColExt(elem fext.Element, size int, name string) ifaces.Column {
	return ConstCol{
		F:     fext.NewGenFieldFromExt(elem),
		Size_: size,
		Name:  name,
	}
}

// Returns the round of definition of the column (always zero)
// Even though this is more of a convention than a meaningful
// return value.
func (cc ConstCol) Round() int {
	return 0
}

// Returns a generic name from the column. Defined from the coin's.
func (cc ConstCol) GetColID() ifaces.ColID {

	val := cc.F.String()

	if len(cc.Name) > 0 {
		return ifaces.ColIDf("CONSTCOL_%v_%v", val, cc.Name)
	}

	return ifaces.ColIDf("CONSTCOL_%v_%v", val, cc.Size_)
}

// Always return true
func (cc ConstCol) MustExists() {}

// Returns the size of the column
func (cc ConstCol) Size() int {
	return cc.Size_
}

// Returns a constant smart-vector
func (cc ConstCol) GetColAssignment(_ ifaces.Runtime) ifaces.ColAssignment {
	if cc.F.IsBase {
		return smartvectors.NewConstant(cc.F.Base, cc.Size_)
	}
	return smartvectors.NewConstantExt(cc.F.Ext, cc.Size_)
}

func (cc ConstCol) GetColAssignmentAtBase(_ ifaces.Runtime, n int) (field.Element, error) {

	if n < 0 || n >= cc.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", cc.Size_, n)
	}

	f, err := cc.F.GetBase()
	if err != nil {
		return field.Element{}, fmt.Errorf("GetColAssignmentAtBase failed: %w", err)
	}
	return f, nil
}

func (cc ConstCol) GetColAssignmentAtExt(_ ifaces.Runtime, n int) fext.Element {
	if n < 0 || n >= cc.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", cc.Size_, n)
	}
	return cc.F.GetExt()
}

// Returns the column as a list of gnark constants
func (cc ConstCol) GetColAssignmentGnark(_ ifaces.GnarkRuntime) []koalagnark.Element {
	res := make([]koalagnark.Element, cc.Size_)
	x, err := cc.F.GetBase()
	if err != nil {
		utils.Panic("GetColAssignmentGnark failed: %v", err.Error())
	}

	for i := range res {
		res[i] = koalagnark.NewElement(x)
	}
	return res
}

func (cc ConstCol) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {

	res := make([]koalagnark.Element, cc.Size_)
	x, err := cc.F.GetBase()
	if err != nil {
		return nil, fmt.Errorf("GetColAssignmentGnarkBase failed: %w", err)
	}

	for i := range res {
		res[i] = koalagnark.NewElementFromKoala(x)
	}

	return res, nil
}

func (cc ConstCol) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	res := make([]koalagnark.Ext, cc.Size_)
	f := cc.F.GetExt()
	for i := range res {
		temp := koalagnark.NewExt(f)
		res[i] = temp
	}
	return res
}

// Returns a particular position of the coin value
func (cc ConstCol) GetColAssignmentAt(_ ifaces.Runtime, pos int) field.Element {

	if pos < 0 || pos >= cc.Size_ {
		utils.Panic("out of bound: size=%v pos=%v", cc.Size_, pos)
	}

	x, err := cc.F.GetBase()
	if err != nil {
		utils.Panic("GetColAssignmentGnark failed: %v", err.Error())
	}

	return x
}

// Returns a particular position of the coin value
func (cc ConstCol) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	f := cc.GetColAssignmentAt(nil, pos)
	return koalagnark.NewElementFromKoala(f)
}

// Returns a particular position of the coin value
func (cc ConstCol) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	// this does the boundary check
	f, err := cc.GetColAssignmentAtBase(nil, pos)
	if err != nil {
		return koalagnark.Element{}, fmt.Errorf("GetColAssignmentGnarkAtBase failed: %w", err)
	}
	return koalagnark.NewElement(f), nil
}

// Returns a particular position of the coin value
func (cc ConstCol) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	// this does the boundary check
	f := cc.GetColAssignmentAtExt(nil, pos)
	temp := koalagnark.NewExt(f)
	return temp
}

// Since the column is directly defined from the
// values of a random coin it does not count as a
// composite column.
func (cc ConstCol) IsComposite() bool {
	return false
}

// Returns the name of the column.
func (cc ConstCol) String() string {
	return string(cc.GetColID())
}

// Splits the column and return a handle of it
func (cc ConstCol) Split(comp *wizard.CompiledIOP, from, to int) ifaces.Column {

	if to < from || to-from > cc.Size() {
		utils.Panic("Can't split %++v into [%v, %v]", cc, from, to)
	}

	// Copy the underlying cc, and assigns the new from and to
	return ConstCol{
		F:     cc.F,
		Size_: to - from,
		Name:  cc.Name,
	}
}

func (cc ConstCol) IsBase() bool {
	return cc.F.IsBase
}

func (cc ConstCol) IsZero() bool {
	return cc.F.IsZero()
}

func (cc ConstCol) IsOne() bool {
	return cc.F.IsOne()
}

// Returns the string representation of the underlying field element
func (cc ConstCol) StringField() string {
	return cc.F.String()
}

// UnimplementedForInterfaceOnlyForConstCol is an unimplemented method that is only used to satisfy the interface and should never be called. It can be used to detect if a column is a ConstCol without creating a dependency cycle between the caller and verifiercol package.
func (cc ConstCol) UnimplementedForInterfaceOnlyForConstCol() {
	panic("unimplemented method called, this is only to satisfy the interface and should never be called")
}

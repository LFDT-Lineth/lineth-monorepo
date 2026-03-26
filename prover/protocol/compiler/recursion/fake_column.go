package recursion

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
)

var _ ifaces.Column = (*FakeColumn)(nil)

// FakeColumn is a dummy implementation of the [ifaces.Column] interface
// it only implements the following methods:
//
//   - [ifaces.Column.GetColID]
//   - [ifaces.Column.String]
type FakeColumn struct {
	ID ifaces.ColID
}

func (fc *FakeColumn) GetColID() ifaces.ColID {
	return fc.ID
}

func (fc *FakeColumn) String() string {
	return string(fc.ID)
}

func (fc *FakeColumn) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	panic("unimplemented")
}

func (fc *FakeColumn) Round() int {
	panic("unimplemented")
}

func (fc *FakeColumn) Size() int {
	panic("unimplemented")
}

func (fc *FakeColumn) MustExists() {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	panic("unimplemented")
}

func (fc *FakeColumn) IsComposite() bool {
	panic("unimplemented")
}

func (fc *FakeColumn) IsBase() bool {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	panic("unimplemented")
}

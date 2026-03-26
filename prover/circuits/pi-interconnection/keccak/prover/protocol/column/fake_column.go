package column

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
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

func (fc *FakeColumn) GetColAssignmentGnark(run ifaces.GnarkRuntime) []frontend.Variable {
	panic("unimplemented")
}

func (fc *FakeColumn) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) frontend.Variable {
	panic("unimplemented")
}

// The fake column is interpreted as composite column so that
func (fc *FakeColumn) IsComposite() bool {
	return true
}

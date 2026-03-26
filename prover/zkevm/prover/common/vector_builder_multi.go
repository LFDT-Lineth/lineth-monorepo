package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// MultiVectorBuilder is a convenience structure to assign groups of columns
// by appending rows on top of rows.
type MultiVectorBuilder struct {
	T []*VectorBuilder
}

// New returns a new MultiVectorBuilder
func NewMultiVectorBuilder(columns []ifaces.Column) *MultiVectorBuilder {
	res := make([]*VectorBuilder, len(columns))
	for i := range columns {
		res[i] = NewVectorBuilder(columns[i])
	}
	return &MultiVectorBuilder{T: res}
}

// PushRow appends a row to the MultiVectorBuilder
func (v *MultiVectorBuilder) PushRow(row []field.Element) {
	if len(row) != len(v.T) {
		utils.Panic("the row size %v does not match the number of columns %v", len(row), len(v.T))
	}
	for i := range row {
		v.T[i].PushField(row[i])
	}
}

// PushZeroes appends a row of zeroes to the MultiVectorBuilder
func (v *MultiVectorBuilder) PushZeroes() {
	for i := range v.T {
		v.T[i].PushZero()
	}
}

// PushSeqOfZeroes appends many rows of zeroes
func (v *MultiVectorBuilder) PushSeqOfZeroes(count int) {
	for i := 0; i < count; i++ {
		v.PushZeroes()
	}
}

// PadAssignZero assigns the columns and pad them on the right with zeroes
func (v *MultiVectorBuilder) PadAssignZero(run *wizard.ProverRuntime, padding ...[]field.Element) {
	for i := range v.T {
		var pds []field.Element
		if len(padding) > 0 {
			pds = []field.Element{padding[0][i]}
		}
		v.T[i].PadAndAssign(run, pds...)
	}
}

// Height returns the number of rows
func (v *MultiVectorBuilder) Height() int {
	return v.T[0].Height()
}

// PushRepeat repeatedly push the provided row
func (v *MultiVectorBuilder) PushRepeat(row []field.Element, count int) {
	for i := 0; i < count; i++ {
		v.PushRow(row)
	}
}

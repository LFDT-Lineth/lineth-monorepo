package common

import (
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// CreateMultiColumn creates multiple columns with names formatted as rootName_0, rootName_1, ..., rootName_{count-1}
func CreateMultiColumn(comp *wizard.CompiledIOP, rootName string, size int, count int, withPragmas pragmas.Pragma) []ifaces.Column {
	cols := make([]ifaces.Column, count)
	createCol := CreateColFn(comp, rootName, size, withPragmas)
	for i := 0; i < count; i++ {
		cols[i] = createCol("%d", i)
	}
	return cols

}

// GetMultiColumnAssignment retrieves the assignments for multiple columns and returns them as a slice of slices of field.Elements.
func GetMultiColumnAssignment(run *wizard.ProverRuntime, cols []ifaces.Column) [][]field.Element {
	assignments := make([][]field.Element, len(cols))
	for i := range cols {
		assignments[i] = cols[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}
	return assignments
}

// AssignMultiColumn assigns multiple columns with the provided values, right zero-padded to the specified size.
func AssignMultiColumn(run *wizard.ProverRuntime, cols []ifaces.Column, values [][]field.Element, size int) {

	for i := range cols {
		run.AssignColumn(cols[i].GetColID(), smartvectors.RightZeroPadded(values[i], size))
	}
}

// CreateColFn commits to the columns of round zero, from the same module.
func CreateColFn(comp *wizard.CompiledIOP, rootName string, size int, withPragmas pragmas.Pragma) func(name string, args ...interface{}) ifaces.Column {

	return func(name string, args ...interface{}) ifaces.Column {
		s := []string{rootName, name}
		v := strings.Join(s, "_")
		col := comp.InsertCommit(0, ifaces.ColIDf(v, args...), size, true)

		switch withPragmas {
		case pragmas.None:
		case pragmas.LeftPadded:
			pragmas.MarkLeftPadded(col)
		case pragmas.RightPadded:
			pragmas.MarkRightPadded(col)
		case pragmas.FullColumnPragma:
			pragmas.MarkFullColumn(col)
		}

		return col
	}

}

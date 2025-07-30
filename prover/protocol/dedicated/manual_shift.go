package dedicated

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// ManuallyShifted represents a column that is cyclically shifted as in
// [column.Shifted]. The difference with the later is that ManuallyShifted
// are [column.Natural] that are explicitly committed to. This means that
// they are less efficient to use and 99% of the case [column.Shift] should
// be prefered. But sometime, they are necessary: when building a distributed
// wizard it is not possible to support cases where we have lookups over
// shifted columns (this is for practical reason, this would require a lot
// of complicated)
//
// The struct inherits directly from [column.Natural] and overwrites
// [GetColAssignment] to add auto-evaluation logic. This way the column can
// more or less be used in the same way. It will need to need to be assigned
// if the value is already available.
type ManuallyShifted struct {
	Natural column.Natural
	// Root points to the original "unshifted version" of the column.
	Root ifaces.Column
	// Offset is the shift to apply to Root. A negative value is a "forward"
	// shift i -> [i+n] % size. And a positive value is a backward shift.
	Offset int
}

// ManuallyShift returns a cyclically version of root by offset. See
// [ManuallyShifted] for more details.
func ManuallyShift(comp *wizard.CompiledIOP, root ifaces.Column, offset int, name string) *ManuallyShifted {

	if len(name) == 0 {
		name = fmt.Sprintf("ManualShift/%v", len(comp.Columns.AllKeys()))
	}

	var (
		size = root.Size()
		res  = ManuallyShifted{
			Natural: comp.InsertCommit(root.Round(), ifaces.ColID(name)+"_COL", size).(column.Natural),
			Root:    root,
			Offset:  offset,
		}
	)

	comp.InsertGlobal(
		root.Round(),
		ifaces.QueryID(name)+"_CONSTRAINT",
		symbolic.Sub(
			res.Natural,
			column.Shift(root, offset),
		),
	)

	return &res
}

// Assign assigns [ManuallyShifted.Natural]
func (m ManuallyShifted) Assign(run *wizard.ProverRuntime) {

	var (
		val  = m.Root.GetColAssignment(run)
		size = val.Len()
		res  smartvectors.SmartVector
	)

	if m.Offset == 0 {
		res = val
	}

	valVec := val.IntoRegVecSaveAlloc()

	if m.Offset < 0 {
		// shiftedWal is obtained by prepending a zero to col and removing the last
		// element. All of this in a separate vector to not have side-effects on the
		// assignment to col.
		shiftedVal := append(make([]field.Element, -m.Offset), valVec[:size+m.Offset]...)
		res = smartvectors.NewRegular(shiftedVal)
	}

	if m.Offset > 0 {
		shiftedVal := append(valVec[m.Offset:], make([]field.Element, m.Offset)...)
		res = smartvectors.NewRegular(shiftedVal)
	}

	run.AssignColumn(m.Natural.ID, res)
}

// GetColAssignment overrides the main [Natural.GetColAssignment] by
// running [Assign] if needed.
func (m ManuallyShifted) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	if pr, isPR := run.(*wizard.ProverRuntime); isPR && !pr.Columns.Exists(m.Natural.ID) {
		m.Assign(pr)
	}
	return m.Natural.GetColAssignment(run)
}

// GetColAssignmentAt overrides the main [Natural.GetColAssignmentAt] by
// running [Assign] if needed.
func (m ManuallyShifted) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	if pr, isPR := run.(*wizard.ProverRuntime); isPR && !pr.Columns.Exists(m.Natural.ID) {
		m.Assign(pr)
	}
	return m.Natural.GetColAssignmentAt(run, pos)
}

package dedicated

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// StackedColumn is a dedicated wizard computing a column by stacking other
// columns on top of each others.
type StackedColumn struct {
	// Column is the built column
	Column column.Natural
	// Source is the list of columns to stack. All of them should have the
	// same number of rows. If after stacking all the rows, the total number of
	// rows is not a power of two, then
	// we pad with zeros up to the next power of two.
	Source []ifaces.Column
}

// StackColumn defines and constrains a [StackedColumn] wizard element.
func StackColumn(comp *wizard.CompiledIOP, srcs []ifaces.Column) StackedColumn {

	var (
		srcs_length = srcs[0].Size()
		// s is the identity permutation to be computed
		s = make([]smartvectors.SmartVector, 0, len(srcs))
		// count is the total number of elements in the stacked column
		count = 0
		name  = fmt.Sprintf("STACKED_COLUMN_%v_%v", len(comp.Columns.AllKeys()), comp.SelfRecursionCount)
		round = 0
		// Variables needed if the number of rows of the
		// stacked column is not a power of two.
		count_padded int
		srcs_padded  []ifaces.Column
		s_padded     []smartvectors.SmartVector
	)

	// Sanity check: all source columns should have the same size
	for i := 1; i < len(srcs); i++ {
		if srcs[i].Size() != srcs_length {
			utils.Panic("All source columns should have the same size, but got %v and %v", srcs_length, srcs[i].Size())
		}
	}

	for i := range srcs {
		round = max(round, srcs[i].Round())
		p := make([]field.Element, srcs_length)
		for j := range p {
			p[j].SetInt64(int64(count))
			count++
		}
		s = append(s, smartvectors.NewRegular(p))
	}

	if !utils.IsPowerOfTwo(count) {
		count_padded = utils.NextPowerOfTwo(count)
		padding_col := verifiercol.NewConstantCol(field.Zero(), srcs_length, "")
		srcs_padded = make([]ifaces.Column, 0, len(srcs)+(count_padded-count)/srcs_length)
		srcs_padded = append(srcs_padded, srcs...)
		for i := 0; i < (count_padded-count)/srcs_length; i++ {
			srcs_padded = append(srcs_padded, padding_col)
		}

		// Next we compute the padded identity permutation
		s_padded = make([]smartvectors.SmartVector, 0, len(s)+(count_padded-count)/srcs_length)
		s_padded = append(s_padded, s...)
		padding_count := 0
		for i := 0; i < (count_padded-count)/srcs_length; i++ {
			p := make([]field.Element, srcs_length)
			for j := range p {
				p[j].SetInt64(int64(count + padding_count))
				padding_count++
			}
			s_padded = append(s_padded, smartvectors.NewRegular(p))
		}
	} else {
		count_padded = count
		srcs_padded = srcs
		s_padded = s
	}

	col := comp.InsertCommit(round, ifaces.ColID(name), count_padded)

	comp.InsertFixedPermutation(
		round,
		ifaces.QueryID(name)+"_CHECK",
		s_padded,
		[]ifaces.Column{col},
		srcs_padded,
	)

	return StackedColumn{
		Column: col.(column.Natural),
		Source: srcs,
	}
}

// Assigns assigns the stack column
func (s StackedColumn) Run(run *wizard.ProverRuntime) {

	res := make([]field.Element, 0, s.Column.Size())
	for i := range s.Source {
		a := s.Source[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		res = append(res, a...)
	}

	run.AssignColumn(s.Column.ID, smartvectors.RightZeroPadded(res, s.Column.Size()))
}

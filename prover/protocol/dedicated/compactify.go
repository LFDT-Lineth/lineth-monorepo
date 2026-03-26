package dedicated

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// Compactification is a sub-wizard whose goal is to select and compactify the
// values of a table indicated by a selector column. A new table is created
// containing the compactified values of the original table. The structure
// also implements the [wizard.ProverAction] interface.
type Compactification struct {
	// ToCompactifyColumns is the list of columns to compactify
	ToCompactifyColumns []ifaces.Column
	// ToCompactifySelector is the selector column of the table to compactify.
	// The selector is trusted to be correctly constrained.
	ToCompactifySelector ifaces.Column
	// CompactifiedColumns is the list of compactified columns
	CompactifiedColumns []ifaces.Column
	// CompactifiedSelector is the selector column of the compactified table
	CompactifiedSelector ifaces.Column
}

// Compactify creates and constrains a [Compactification] from an input table
// and a selector column.
func Compactify(comp *wizard.CompiledIOP, table []ifaces.Column, selector ifaces.Column, name string) *Compactification {

	round := column.MaxRound(table...)
	round = max(round, selector.Round())

	res := &Compactification{
		ToCompactifyColumns:  table,
		ToCompactifySelector: selector,
		CompactifiedSelector: comp.InsertCommit(
			round,
			ifaces.ColIDf("%v_SELECTOR", name),
			selector.Size(),
			selector.IsBase(),
		),
	}

	for i := range table {

		newTable := comp.InsertCommit(
			round,
			ifaces.ColIDf("%v_%v", name, i),
			table[i].Size(),
			table[i].IsBase(),
		)

		res.CompactifiedColumns = append(res.CompactifiedColumns, newTable)
	}

	// -------
	// 	This constrains the selector to be an activation column
	// -------

	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("%v_SELECTOR_IS_BOOLEAN", name),
		symbolic.Sub(
			res.CompactifiedSelector,
			symbolic.Mul(res.CompactifiedSelector, res.CompactifiedSelector),
		),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("%v_SELECTOR_IS_CONTINUOUS_RANGE", name),
		symbolic.Sub(
			res.CompactifiedSelector,
			symbolic.Mul(column.Shift(res.CompactifiedSelector, -1), res.CompactifiedSelector),
		),
	)

	// -------
	// 	This constrains the correctness of the compactification
	// -------

	comp.InsertProjection(
		ifaces.QueryIDf("%v_COMPACTIFICATION", name),
		query.ProjectionInput{
			ColumnA: res.ToCompactifyColumns,
			ColumnB: res.CompactifiedColumns,
			FilterA: res.ToCompactifySelector,
			FilterB: res.CompactifiedSelector,
		},
	)

	return res
}

// Run implements the [wizard.ProverAction] interface.
func (ctx *Compactification) Run(run *wizard.ProverRuntime) {

	var (
		size              = ctx.ToCompactifySelector.Size()
		newSelector       = []field.Element{}
		newTable          = make([][]field.Element, len(ctx.CompactifiedColumns))
		toCompactifySel   = ctx.ToCompactifySelector.GetColAssignment(run)
		toCompactifyTable = make([]smartvectors.SmartVector, len(ctx.ToCompactifyColumns))
	)

	for k := range toCompactifyTable {
		toCompactifyTable[k] = ctx.ToCompactifyColumns[k].GetColAssignment(run)
	}

	for i := 0; i < size; i++ {

		if isTaken := toCompactifySel.GetPtr(i).IsOne(); !isTaken {
			continue
		}

		newSelector = append(newSelector, field.One())
		for k := range newTable {
			newTable[k] = append(newTable[k], toCompactifyTable[k].Get(i))
		}
	}

	run.AssignColumn(ctx.CompactifiedSelector.GetColID(), smartvectors.RightZeroPadded(newSelector, size))
	for k := range newTable {
		run.AssignColumn(ctx.CompactifiedColumns[k].GetColID(), smartvectors.RightZeroPadded(newTable[k], size))
	}
}

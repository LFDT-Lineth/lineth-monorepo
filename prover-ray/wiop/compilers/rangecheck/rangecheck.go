// Package rangecheck implements the RangeCheck compiler pass for the wiop
// protocol framework.
//
// It reduces every [wiop.RangeCheck] query to an [wiop.TableRelation] inclusion:
// the checked column must be a subset of a precomputed column that enumerates
// [0, B). A single precomputed range column is shared across all RangeChecks
// with the same bound B, keeping the number of precomputed columns minimal.
package rangecheck

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// Compile reduces all [wiop.RangeCheck] queries in sys to [wiop.TableRelation]
// inclusion constraints against precomputed range tables.
//
// For each unique bound B, one new module of size NextPowerOfTwo(B) is created
// with a precomputed column containing [0, 1, …, B-1, 0, …, 0]. Each
// RangeCheck is then replaced by an Inclusion asserting that the checked
// column is contained in the range column. Already-reduced queries are skipped.
func Compile(sys *wiop.System) {
	rangeColumns := make(map[int]*wiop.Column)
	compCtx := sys.Context.Childf("rangecheck")

	for mIdx, m := range sys.Modules {
		for rcIdx, rc := range m.RangeChecks {
			if rc.IsReduced() {
				continue
			}

			b := rc.B
			rangeCol, ok := rangeColumns[b]
			if !ok {
				rangeCol = createRangeColumn(sys, compCtx, b)
				rangeColumns[b] = rangeCol
			}

			included := []wiop.Table{wiop.NewTable(rc.Handle.View())}
			including := []wiop.Table{wiop.NewTable(rangeCol.View())}
			sys.NewInclusion(
				compCtx.Childf("inc-m%d-rc%d", mIdx, rcIdx),
				included,
				including,
			)
			rc.MarkAsReduced()
		}
	}
}

// createRangeColumn creates a new module of size NextPowerOfTwo(b) and a
// precomputed column containing [0, 1, …, b-1, 0, …, 0].
func createRangeColumn(sys *wiop.System, ctx *wiop.ContextFrame, b int) *wiop.Column {
	size := utils.NextPowerOfTwo(b)
	mod := sys.NewSizedModule(
		ctx.Childf("range-mod-b%d", b),
		size,
		wiop.PaddingDirectionNone,
	)

	elems := make([]field.Element, size)
	for i := range b {
		elems[i].SetUint64(uint64(i))
	}

	cv := &wiop.ConcreteVector{
		Plain: []field.Vec{field.VecFromBase(elems)},
	}
	return mod.NewPrecomputedColumn(
		ctx.Childf("range-col-b%d", b),
		wiop.VisibilityOracle,
		cv,
	)
}

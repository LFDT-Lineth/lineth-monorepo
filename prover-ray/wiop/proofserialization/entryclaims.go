package proofserialization

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// deriveEntryClaims rebuilds the verifier's `entry_claims`: the claimed
// evaluations of every opened column at zeta, laid out in the canonical entry
// order the Zig verifier reconstructs at runtime.
//
// The values are not new data. They are cells the proof already carries — the
// `LagrangeEval` evaluation claims — which the round messages also transport for
// the transcript. What this reproduces is the ORDER, and that is the risk:
//
// The same ordering exists in two other places, `verifier-ray/codegen`'s
// `pcsEntryOrder` and the Zig verifier's `pcs.reconstruct`, and this is a third
// copy that must agree with both. Nothing in prover-ray can check it: the two
// authorities live in a module that depends on this one, so they cannot be
// imported here. A disagreement is silent — claims land against the wrong
// entries, the DEEP reconstruction combines the wrong values, and FRI fails with
// no indication that the ordering was the cause.
//
// The cross-check therefore has to live in verifier-ray, which can import both:
// compare this against `codegen.ExtractPcsOpening` for the same proof. Until that
// exists, treat this function as unverified.
//
// The five rules mirrored here, all from verifier-ray/codegen/pcs.go:
//
//  1. column declaration index: batch-major over [pcscompiler.CommittedBatches],
//     then the round's own column order;
//  2. shift normalisation: raw offset for a dynamic module, offset mod size for
//     a static one (`pcsShiftFor`);
//  3. shift slots: each column's distinct opened shifts in first-seen order over
//     sys.LagrangeEvals (`pcsShiftSlots`);
//  4. per-column size: the module's fixed size, or the proof's runtime size for
//     a dynamic module;
//  5. entry order: size descending, batch ascending, base rows before extension
//     rows, declaration order within each bucket (`pcsEntryOrder`).
func deriveEntryClaims(sys *wiop.System, proof wiop.Proof) ([][]Ext, error) {
	batches := pcscompiler.CommittedBatches(sys)
	if len(batches) == 0 {
		return nil, nil // nothing committed, so nothing opened
	}

	cols := committedColumns(batches)
	if len(cols) == 0 {
		return nil, nil
	}

	sizeLog2, err := columnSizeLog2(sys, proof, cols)
	if err != nil {
		return nil, err
	}

	// Rule 3: each column's distinct shifts, in first-seen order.
	slots := make(map[wiop.ObjectID][]int, len(cols))
	seen := make(map[wiop.ObjectID]map[int]bool, len(cols))
	for _, le := range sys.LagrangeEvals {
		for _, cv := range le.Polynomials {
			id := cv.Column.Context.ID
			shift := normalizedShift(cv)
			if seen[id] == nil {
				seen[id] = map[int]bool{}
			}
			if seen[id][shift] {
				continue
			}
			seen[id][shift] = true
			slots[id] = append(slots[id], shift)
		}
	}

	declOf := make(map[wiop.ObjectID]int, len(cols))
	for i, c := range cols {
		declOf[c.column.Context.ID] = i
	}

	// The claimed values, indexed [declaration][slot].
	claims := make([][]Ext, len(cols))
	for i, c := range cols {
		claims[i] = make([]Ext, len(slots[c.column.Context.ID]))
	}
	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			id := cv.Column.Context.ID
			decl, ok := declOf[id]
			if !ok {
				return nil, fmt.Errorf("proofserialization: opened column %q is not in any committed batch",
					cv.Column.Context.Path())
			}
			slot := indexOf(slots[id], normalizedShift(cv))
			if slot < 0 {
				return nil, fmt.Errorf("proofserialization: shift %d missing from the schedule of column %q",
					normalizedShift(cv), cv.Column.Context.Path())
			}
			cell := le.EvaluationClaims[k]
			v, ok := proof.Cells[cell.Context.ID]
			if !ok {
				return nil, fmt.Errorf("proofserialization: evaluation claim %q is absent from the proof",
					cell.Context.Path())
			}
			claims[decl][slot] = ExtFrom(v.Ext)
		}
	}

	// Rule 5: reorder declarations into canonical entry order.
	order := entryOrder(cols, sizeLog2)
	out := make([][]Ext, len(order))
	for e, decl := range order {
		out[e] = claims[decl]
	}
	return out, nil
}

// committedColumn is one committed column with the batch it belongs to.
type committedColumn struct {
	column *wiop.Column
	batch  int
}

// committedColumns lists every committed column in declaration order: batch
// major, then the round's own column order (rule 1).
func committedColumns(batches []pcscompiler.BatchRef) []committedColumn {
	var out []committedColumn
	for b, ref := range batches {
		for _, col := range ref.Round.Columns {
			out = append(out, committedColumn{column: col, batch: b})
		}
	}
	return out
}

// normalizedShift applies rule 2: a dynamic module keeps the raw offset, since
// its size is not known until proving time; a static one reduces modulo the
// module size, with Go's negative-remainder correction.
func normalizedShift(cv *wiop.ColumnView) int {
	if cv.Column.Module.IsDynamic() {
		return cv.ShiftingOffset
	}
	size := cv.Column.Module.Size()
	return ((cv.ShiftingOffset % size) + size) % size
}

// columnSizeLog2 resolves each column's size for this proof (rule 4): the
// module's fixed size, or the runtime size the proof reports for a dynamic one.
func columnSizeLog2(sys *wiop.System, proof wiop.Proof, cols []committedColumn) ([]int, error) {
	moduleIdx := make(map[*wiop.Module]int, len(sys.Modules))
	for i, m := range sys.Modules {
		moduleIdx[m] = i
	}

	out := make([]int, len(cols))
	for i, c := range cols {
		m := c.column.Module
		size := m.Size()
		if m.IsDynamic() {
			idx, ok := moduleIdx[m]
			if !ok {
				return nil, fmt.Errorf("proofserialization: module %q of column %q is not in sys.Modules",
					m.Context.Path(), c.column.Context.Path())
			}
			size, ok = proof.DynamicSizes[idx]
			if !ok {
				return nil, fmt.Errorf("proofserialization: dynamic module %q has no size in the proof",
					m.Context.Path())
			}
		}
		if size <= 0 || size&(size-1) != 0 {
			return nil, fmt.Errorf("proofserialization: column %q has non-power-of-two size %d",
				c.column.Context.Path(), size)
		}
		out[i] = utils.Log2Ceil(size)
	}
	return out, nil
}

// entryOrder applies rule 5: size descending, batch ascending, base rows before
// extension rows, declaration order within each bucket. Returns declaration
// indices in entry order.
func entryOrder(cols []committedColumn, sizeLog2 []int) []int {
	maxSize := 0
	numBatches := 0
	for i, c := range cols {
		if sizeLog2[i] > maxSize {
			maxSize = sizeLog2[i]
		}
		if c.batch+1 > numBatches {
			numBatches = c.batch + 1
		}
	}

	order := make([]int, 0, len(cols))
	for size := maxSize; size >= 0; size-- {
		for batch := range numBatches {
			for _, wantExt := range [2]bool{false, true} {
				for i, c := range cols {
					if c.batch != batch || c.column.IsExtension != wantExt || sizeLog2[i] != size {
						continue
					}
					order = append(order, i)
				}
			}
		}
	}
	return order
}

func indexOf(xs []int, want int) int {
	for i, x := range xs {
		if x == want {
			return i
		}
	}
	return -1
}

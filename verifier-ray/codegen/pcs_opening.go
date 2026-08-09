package codegen

import (
	"fmt"
	"math/bits"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ExtractPcsOpening reads one proof's claimed evaluations for every opened
// (column, shift) from rt, and returns them jagged `[entry][shift]` in the
// canonical entry order for THIS proof's dynamic-module sizes — the same
// order the Zig verifier's runtime reconstruction produces at those sizes
// (see pcsEntryOrder), so the result lines up entry-for-entry with the
// verifier's `entry_claims`.
//
// pcs must be the PcsSystem BuildPcsSystem extracted from this same sys. rt
// supplies both the claimed values and, for dynamic modules, this proof's
// actual runtime size.
func ExtractPcsOpening(sys *wiop.System, pcs PcsSystem, rt *wiop.Runtime) ([][]field.Ext, error) {
	colDeclByID := pcsColumnDeclIndex(sys)
	if len(colDeclByID) != len(pcs.Columns) {
		return nil, fmt.Errorf("codegen: ExtractPcsOpening: sys has %d committed columns, pcs has %d; built from a different sys?",
			len(colDeclByID), len(pcs.Columns))
	}

	moduleSizes := DynamicModuleSizes(sys, rt)

	// This proof's resolved size_log2 per column, validated against the baked
	// envelope and, for dynamic columns, the minimum safe size the raw shift
	// schedule tolerates (see pcsDynamicMinSizeLog2) — the same
	// DynamicModuleSizeBelowMinimum bound the Zig verifier enforces on every
	// future proof.
	colSizeLog2 := make([]int, len(pcs.Columns))
	for c, col := range pcs.Columns {
		if !col.IsDynamic {
			colSizeLog2[c] = col.SizeLog2
			continue
		}
		if col.DynamicIndex >= len(moduleSizes) {
			return nil, fmt.Errorf("codegen: ExtractPcsOpening: dynamic column %d references module_sizes index %d, but sys has only %d dynamic modules",
				c, col.DynamicIndex, len(moduleSizes))
		}
		size := moduleSizes[col.DynamicIndex]
		sizeLog2 := bits.Len(uint(size)) - 1
		if size <= 0 || 1<<sizeLog2 != size {
			return nil, fmt.Errorf("codegen: ExtractPcsOpening: dynamic column %d proved at non-power-of-two size %d", c, size)
		}
		if sizeLog2 > pcs.MaxSizeLog2 {
			return nil, fmt.Errorf("codegen: ExtractPcsOpening: dynamic column %d proved at size 2^%d, above the verifier envelope's max 2^%d",
				c, sizeLog2, pcs.MaxSizeLog2)
		}
		if sizeLog2 < col.DynamicMinSizeLog2 {
			return nil, fmt.Errorf(
				"codegen: ExtractPcsOpening: dynamic column %d proved at size 2^%d, below its minimum safe size 2^%d — "+
					"its raw shift schedule aliases at this size", c, sizeLog2, col.DynamicMinSizeLog2)
		}
		colSizeLog2[c] = sizeLog2
	}

	// Claimed values per (declaration index, shift slot), read directly from
	// rt. Slot indices come from pcs.Columns[c].Shifts, the same schedule
	// BuildPcsSystem fixed from sys alone, so every (column, shift) opening
	// encountered here already has a reserved slot; a repeated opening writes
	// the same value to the same slot again.
	claims := make([][]field.Ext, len(pcs.Columns))
	for c, col := range pcs.Columns {
		claims[c] = make([]field.Ext, len(col.Shifts))
	}
	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			decl, ok := colDeclByID[cv.Column.Context.ID]
			if !ok {
				return nil, fmt.Errorf("codegen: ExtractPcsOpening: opened column %q not found in pcs.Columns", cv.Column.Context.Path())
			}
			_, shift := pcsShiftFor(cv)
			slot := slices.Index(pcs.Columns[decl].Shifts, shift)
			if slot < 0 {
				return nil, fmt.Errorf("codegen: ExtractPcsOpening: shift %d not found in baked schedule for column %q", shift, cv.Column.Context.Path())
			}
			claims[decl][slot] = rt.GetCellValue(le.EvaluationClaims[k]).AsExt()
		}
	}

	order := pcsEntryOrder(pcs, colSizeLog2)
	entryClaims := make([][]field.Ext, len(order))
	for e, c := range order {
		entryClaims[e] = claims[c]
	}
	return entryClaims, nil
}

// pcsEntryOrder returns, for one proof's resolved per-column size_log2s, the
// declaration index of every entry in canonical order: size DESC, batch ASC,
// base rows then ext rows, declaration order within each (batch, size, is_ext)
// bucket. This is a direct mirror of the Zig verifier's runtime reconstruction
// (src/query/pcs.zig's `reconstruct`): filtering columns — already in
// declaration order — by bucket reproduces the same relative order as its
// col_position counters without computing them, since `reconstruct` itself
// never consults col_position for entry ordering, only for entry_row_idx.
func pcsEntryOrder(pcs PcsSystem, colSizeLog2 []int) []int {
	order := make([]int, 0, len(pcs.Columns))
	for size := pcs.MaxSizeLog2; size >= 0; size-- {
		for batch := 0; batch < pcs.NumBatches; batch++ {
			for _, wantExt := range [2]bool{false, true} {
				for c, col := range pcs.Columns {
					if col.BatchIdx != batch || col.IsExt != wantExt || colSizeLog2[c] != size {
						continue
					}
					order = append(order, c)
				}
			}
		}
	}
	return order
}

package fri

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// ShapeOpeningProof returns an [OpeningProof] with exactly the geometry an
// honest prover emits for batches of the given shapes, opened with the given
// shifts: every slice length, every present or absent auxiliary level and every
// row width. Its values are meaningless.
//
// It lets a verifier circuit be allocated from the protocol description alone,
// before any proof exists (see [AllocateGnarkOpeningProof]). The geometry is a
// function of the parameters, shapes and shifts only: input trees are never
// merged (one per opened batch), and query positions, zeta and fold challenges
// select values, never shapes. To keep it in lockstep with the real prover, it
// is not derived by hand: the native prover runs on all-zero tables of the
// requested shapes, at a fixed out-of-domain zeta and fixed challenges.
//
// The receiver is not modified. The cost is one commit and open of zero data at
// the given sizes; it is meant to run once, at circuit setup.
func (pcs *PCS) ShapeOpeningProof(shapes []Shape, shifts []BatchShifts) (OpeningProof, error) {
	if len(shapes) != len(shifts) {
		return OpeningProof{}, fmt.Errorf("fri: ShapeOpeningProof: got %d shapes, %d shifts", len(shapes), len(shifts))
	}
	scratch := *pcs
	scratch.openings = nil
	scratch.zeta = field.Ext{}

	// An element with a non-zero extension coordinate is outside every
	// base-field evaluation domain, so the opening is well-formed at any size.
	zeta := field.UintsToExt(3, 1, 0, 0, 0, 0)
	for i, shape := range shapes {
		committed := scratch.Commit(zeroTable(shape))
		if err := scratch.AddOpening(committed, zeta, shifts[i], zeroClaims(shifts[i])); err != nil {
			return OpeningProof{}, fmt.Errorf("fri: ShapeOpeningProof: batch %d: %w", i, err)
		}
	}
	state, err := scratch.NewProverState()
	if err != nil {
		return OpeningProof{}, fmt.Errorf("fri: ShapeOpeningProof: %w", err)
	}
	alpha := field.UintsToExt(5, 0, 1, 0, 0, 0)
	for state.HasNext() {
		state.Fold(alpha)
	}
	return scratch.Open(state, make([]int, scratch.Params.NumQueries)), nil
}

// zeroTable returns an all-zero table of the given shape: each present size
// holds BaseWidth base rows and ExtWidth extension rows of 2^sizeLog2 entries.
func zeroTable(shape Shape) MultiSizeTable {
	table := make(MultiSizeTable, len(shape))
	for sizeLog2, sized := range shape {
		n := 1 << sizeLog2
		table[sizeLog2].Groups = sized.Groups
		if sized.BaseWidth > 0 {
			table[sizeLog2].Base = make([][]field.Element, sized.BaseWidth)
			for k := range table[sizeLog2].Base {
				table[sizeLog2].Base[k] = make([]field.Element, n)
			}
		}
		if sized.ExtWidth > 0 {
			table[sizeLog2].Ext = make([][]field.Ext, sized.ExtWidth)
			for k := range table[sizeLog2].Ext {
				table[sizeLog2].Ext[k] = make([]field.Ext, n)
			}
		}
	}
	return table
}

// zeroClaims returns the claimed values of an all-zero table: one zero per
// opened (row, shift), laid out like shifts.
func zeroClaims(shifts BatchShifts) BatchClaimedValues {
	zeros := func(rows [][]int) [][]field.Ext {
		if rows == nil {
			return nil
		}
		res := make([][]field.Ext, len(rows))
		for k, row := range rows {
			res[k] = make([]field.Ext, len(row))
		}
		return res
	}
	claims := make(BatchClaimedValues, len(shifts))
	for sizeLog2, sized := range shifts {
		claims[sizeLog2] = SizedClaimedValues{Base: zeros(sized.Base), Ext: zeros(sized.Ext)}
	}
	return claims
}

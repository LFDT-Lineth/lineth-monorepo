package pcs

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ObliviousOpening is the structure of the PCS opening of a System compiled
// [WithSizeOblivious], with nothing in it depending on the modules' sizes: it
// is the same for every proof of the System and for every System with the
// same modules, columns and openings, whatever their sizes. It is for
// verifiers written in another proof system that take the sizes as data.
//
// A module of log size ℓ is one group of each batch it has columns in: its
// row pairs are hashed on their own, at height ℓ + LogBlowup − 1 of the
// batch's tree, and it joins FRI at round MaxSizeLog2 − ℓ.
type ObliviousOpening struct {
	// Batches are the committed batches, in FRI order, and Groups[b] the
	// groups of batch b, by increasing module index.
	Batches []BatchRef
	Groups  [][]ObliviousGroup
	// NumColumns is the number of committed columns: the alpha_DEEP power of
	// the running codeword at every level (see fri's deepPowers).
	NumColumns int
	// Duplicates are pairs of claim cells of the same opening (column and
	// shift), which must hold the same value.
	Duplicates [][2]*wiop.Cell
	// EvalPoints are the evaluation points of the LagrangeEvals, which must
	// all evaluate to the same value: ζ.
	EvalPoints []wiop.Expression
	// Manifests are the column manifest cells, which must all hold
	// [ManifestPresent].
	Manifests []*wiop.Cell
	// PrecomputedRoot is the root of the precomputed batch, if any.
	PrecomputedRoot field.Octuplet
	// NumQueries is the number of FRI queries; LogBlowup the log of the RS
	// blowup; MaxSizeLog2 the log of the largest committable size: FRI runs
	// MaxSizeLog2 rounds over a domain of 2^(MaxSizeLog2+LogBlowup) points,
	// which the query positions are drawn from.
	NumQueries, LogBlowup, MaxSizeLog2 int
}

// ObliviousGroup is the columns of one module in one batch: Base then Ext,
// each in commit order. Their alpha_DEEP powers are Power, Power + 1, ….
type ObliviousGroup struct {
	Module    *wiop.Module
	Power     int
	Base, Ext []ObliviousColumn
}

// ObliviousColumn is one committed column: the shifts it is opened at, as the
// LagrangeEvals give them (not reduced modulo the size), and their claim
// cells. Distinct shifts stay distinct modulo any size from the module's
// [wiop.Module.MinRuntimeSize] on.
type ObliviousColumn struct {
	Column *wiop.Column
	Shifts []int
	Claims []*wiop.Cell
}

// Oblivious returns the size-independent structure of the opening of sys, the
// System whose verifier action a is. It fails unless sys is compiled
// [WithSizeOblivious], without Merkle cap, with the KoalaBear Merkle hash.
func (a *OpeningVerifierAction) Oblivious(sys *wiop.System) (*ObliviousOpening, error) {
	c := a.c
	switch {
	case !c.moduleGroups || !c.fixedFRIDomain:
		return nil, fmt.Errorf("pcs: the system is not compiled size-oblivious")
	case c.elisionEnabled:
		return nil, fmt.Errorf("pcs: column elision makes the opening's layout the prover's")
	case c.merkleCapHeight != 0:
		return nil, fmt.Errorf("pcs: a Merkle cap is not supported")
	case c.merkleHash != fri.MerkleHashKoalaBear:
		return nil, fmt.Errorf("pcs: the Merkle hash %v is not supported", c.merkleHash)
	}
	batches := CommittedBatches(sys)
	res := &ObliviousOpening{
		Batches:         batches,
		Groups:          make([][]ObliviousGroup, len(batches)),
		PrecomputedRoot: c.precomputedRoot,
		NumQueries:      c.friNumQueries,
		LogBlowup:       int(c.logRSBlowup),
		MaxSizeLog2:     int(c.maxSizeLog2),
	}
	type loc struct {
		batch, group, pos int
		ext               bool
	}
	locs := map[wiop.ObjectID]loc{}
	for b, batch := range batches {
		if !batch.IsPrecomp {
			res.Manifests = append(res.Manifests, c.manifestCells[batch.Round.ID]...)
		}
		// commit order: declaration order, stably sorted by module
		cols := slices.Clone(batch.Round.Columns)
		slices.SortStableFunc(cols, func(x, y *wiop.Column) int {
			return cmp.Compare(moduleIndex(x), moduleIndex(y))
		})
		for _, col := range cols {
			gs := &res.Groups[b]
			if n := len(*gs); n == 0 || (*gs)[n-1].Module != col.Module {
				*gs = append(*gs, ObliviousGroup{Module: col.Module})
			}
			g := &(*gs)[len(*gs)-1]
			l := loc{batch: b, group: len(*gs) - 1, ext: col.IsExtension}
			if col.IsExtension {
				l.pos = len(g.Ext)
				g.Ext = append(g.Ext, ObliviousColumn{Column: col})
			} else {
				l.pos = len(g.Base)
				g.Base = append(g.Base, ObliviousColumn{Column: col})
			}
			locs[col.Context.ID] = l
		}
		for i := range res.Groups[b] {
			g := &res.Groups[b][i]
			g.Power = res.NumColumns
			res.NumColumns += len(g.Base) + len(g.Ext)
		}
	}

	// As RecoverBatchClaims: one claim per (column, shift), the repeated ones
	// checked equal to it. Shifts are compared unreduced: see ObliviousColumn.
	for _, eval := range sys.LagrangeEvals {
		res.EvalPoints = append(res.EvalPoints, eval.EvaluationPoint)
		for k, view := range eval.Polynomials {
			l, ok := locs[view.Column.Context.ID]
			if !ok {
				return nil, fmt.Errorf("pcs: column %q is in a round that owns no committed batch",
					view.Column.Context.Path())
			}
			g := &res.Groups[l.batch][l.group]
			var col *ObliviousColumn
			if l.ext {
				col = &g.Ext[l.pos]
			} else {
				col = &g.Base[l.pos]
			}
			cell := eval.EvaluationClaims[k]
			if i := slices.Index(col.Shifts, view.ShiftingOffset); i >= 0 {
				res.Duplicates = append(res.Duplicates, [2]*wiop.Cell{col.Claims[i], cell})
				continue
			}
			col.Shifts = append(col.Shifts, view.ShiftingOffset)
			col.Claims = append(col.Claims, cell)
		}
	}
	if len(res.EvalPoints) == 0 {
		return nil, fmt.Errorf("pcs: no LagrangeEval queries to open")
	}
	for b := range res.Groups {
		for _, g := range res.Groups[b] {
			for _, col := range slices.Concat(g.Base, g.Ext) {
				if len(col.Shifts) == 0 {
					return nil, fmt.Errorf("pcs: column %q is committed but never opened", col.Column.Context.Path())
				}
			}
		}
	}
	return res, nil
}

// IsSizeOblivious reports whether sys was compiled [WithSizeOblivious].
func IsSizeOblivious(sys *wiop.System) bool {
	state := configOf(sys)
	return state.consumed && state.moduleGroups && state.fixedFRIDomain
}

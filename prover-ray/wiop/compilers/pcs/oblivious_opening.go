package pcs

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ObliviousOpening is the structure of a size-oblivious opening
// ([WithSizeOblivious]) that holds whatever the modules' sizes: the
// committed groups (one per module and batch), their columns, opening shifts,
// claims and alpha_DEEP powers, and the FRI schedule, fixed by the System.
// Only where each group enters the trees and FRI depends on its module's
// size ℓ (log2 of the padded size):
//
//   - its pair of a query is at height ℓ + RateLog − 1 of its batch's tree
//     (see [fri.MultiSizeTable.Merkleize]), hashed with its own leaf header;
//   - its DEEP terms join FRI at round MaxSizeLog2 − ℓ, with alphaDeep the
//     square of that round's fold challenge, at the claim points ζ·ω_n^s
//     (n = 2^ℓ, s a shift);
//   - the modules of one size share that round's level, in which every
//     column keeps its power (see fri.deepPowers): a group's columns have the
//     consecutive powers from Power, and every level the running power
//     RunningPower.
//
// It is what a size-oblivious verifier written in another proof system (the
// recursion machine's) lays out as data.
type ObliviousOpening struct {
	// Batches are the committed batches, in FRI order.
	Batches []BatchRef
	// Groups[b] are batch b's groups, in module order.
	Groups [][]OpeningGroup
	// RunningPower is the power of every level's running codeword: the
	// number of committed columns.
	RunningPower int
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
	// NumQueries is the number of FRI queries, drawn below 2^LogCodewordSize.
	NumQueries int

	// Params are the FRI parameters (the fixed domain's), NumRounds the
	// number of folds, LogCodewordSize the log size of the top codeword
	// domain, RateLog the log of the inverse rate, LogFinalPolySize the log
	// number of the final polynomial's coefficients, and Generators[j] the
	// generator of round j's domain (j ≤ NumRounds), as in fri.VerifierPlan.
	Params                                                fri.Params
	NumRounds, LogCodewordSize, RateLog, LogFinalPolySize int
	Generators                                            []field.Element
	// MaxSizeLog2 is the largest committable size, MinSizeLog2 the smallest
	// one FRI folds: every committed module's ℓ is in [MinSizeLog2,
	// MaxSizeLog2], and joins FRI at round MaxSizeLog2 − ℓ < NumRounds.
	MaxSizeLog2, MinSizeLog2 int
}

// OpeningGroup is the group of one module in one batch: its base and
// extension columns, in commit order, the first one's alpha_DEEP power.
// ModuleIndex is the module's index in wiop.System.Modules.
type OpeningGroup struct {
	Module      *wiop.Module
	ModuleIndex int
	Power       int
	Base, Ext   []OpenedColumn
}

// OpenedColumn is a committed column, its opening shifts (as declared, not
// reduced modulo the module's size: |s| < n/2 for every size n the module
// may take) and the claim cell of each.
type OpenedColumn struct {
	Column *wiop.Column
	Shifts []int
	Claims []*wiop.Cell
}

// Oblivious returns the size-oblivious structure of the opening of sys, the
// System whose verifier action a is. It fails unless sys is compiled
// [WithSizeOblivious], and on a sized module that is not folded (below
// 2^MinSizeLog2) or too small for its shifts: their rules would differ from
// the dynamic modules'.
func (a *OpeningVerifierAction) Oblivious(sys *wiop.System) (*ObliviousOpening, error) {
	if sys != a.sys {
		return nil, fmt.Errorf("pcs: Oblivious: called with a System other than the action's")
	}
	c := a.c
	if !c.moduleGroups || !c.fixedFRIDomain {
		return nil, fmt.Errorf("pcs: the opening is not size-oblivious (WithSizeOblivious)")
	}
	batches := CommittedBatches(sys)
	res := &ObliviousOpening{
		Batches:         batches,
		Groups:          make([][]OpeningGroup, len(batches)),
		PrecomputedRoot: c.precomputedRoot,
		NumQueries:      c.friNumQueries,
		MaxSizeLog2:     int(c.maxSizeLog2),
		MinSizeLog2:     c.minDynamicSizeLog2,
	}
	for _, b := range batches {
		if !b.IsPrecomp {
			res.Manifests = append(res.Manifests, c.manifestCells[b.Round.ID]...)
		}
	}

	type columnRef struct {
		batch, group int
		isExt        bool
		idx          int
	}
	refs := map[wiop.ObjectID]columnRef{}
	for i, b := range batches {
		order, _ := committedLayout(b.Round, staticSizeOrOne, allPresentManifest(len(b.Round.Columns)), true)
		var groups []OpeningGroup
		for _, k := range order {
			col := b.Round.Columns[k]
			if n := len(groups); n == 0 || groups[n-1].Module != col.Module {
				if err := checkObliviousModule(col.Module, res.MinSizeLog2); err != nil {
					return nil, err
				}
				groups = append(groups, OpeningGroup{Module: col.Module, ModuleIndex: moduleIndex(col)})
			}
			g := &groups[len(groups)-1]
			ref := columnRef{batch: i, group: len(groups) - 1, isExt: col.IsExtension}
			if col.IsExtension {
				ref.idx = len(g.Ext)
				g.Ext = append(g.Ext, OpenedColumn{Column: col})
			} else {
				ref.idx = len(g.Base)
				g.Base = append(g.Base, OpenedColumn{Column: col})
			}
			refs[col.Context.ID] = ref
		}
		// fri.deepPowers: batch order, then the groups by ordinal (the module
		// index, so module order), base rows then extension rows
		for k := range groups {
			groups[k].Power = res.RunningPower
			res.RunningPower += len(groups[k].Base) + len(groups[k].Ext)
		}
		res.Groups[i] = groups
	}

	// As RecoverBatchClaims: one claim per (column, shift), the repeated
	// ones checked equal to it. The shifts of a module are distinct modulo
	// any of its sizes, so the declared shift identifies the opening.
	type claimID struct {
		col   wiop.ObjectID
		shift int
	}
	seen := map[claimID]*wiop.Cell{}
	for _, eval := range sys.LagrangeEvals {
		res.EvalPoints = append(res.EvalPoints, eval.EvaluationPoint)
		for k, view := range eval.Polynomials {
			ref, ok := refs[view.Column.Context.ID]
			if !ok {
				return nil, fmt.Errorf("pcs: column %q is not committed", view.Column.Context.Path())
			}
			cell := eval.EvaluationClaims[k]
			id := claimID{view.Column.Context.ID, view.ShiftingOffset}
			if prev, dup := seen[id]; dup {
				res.Duplicates = append(res.Duplicates, [2]*wiop.Cell{prev, cell})
				continue
			}
			seen[id] = cell
			g := &res.Groups[ref.batch][ref.group]
			cols := g.Base
			if ref.isExt {
				cols = g.Ext
			}
			col := &cols[ref.idx]
			col.Shifts = append(col.Shifts, view.ShiftingOffset)
			col.Claims = append(col.Claims, cell)
		}
	}
	if len(res.EvalPoints) == 0 {
		return nil, fmt.Errorf("pcs: no LagrangeEval queries to open")
	}

	// The schedule, fixed by the domain: the plan of any sizes has it.
	shapes, shifts := res.layout(func(m *wiop.Module) int {
		if m.IsDynamic() {
			return max(m.MinRuntimeSize(), 1<<res.MinSizeLog2)
		}
		return m.Size()
	})
	plan, err := c.newPCS().VerifierPlan(shapes, shifts)
	if err != nil {
		return nil, err
	}
	res.Params = plan.Params
	res.NumRounds, res.LogCodewordSize, res.RateLog, res.LogFinalPolySize =
		plan.NumRounds, plan.LogCodewordSize, plan.RateLog, plan.LogFinalPolySize
	res.Generators = plan.Generators
	if int(plan.Params.LogPlainTextSize) != res.MaxSizeLog2 || res.NumRounds+res.LogFinalPolySize != res.MaxSizeLog2 {
		return nil, fmt.Errorf("pcs: a fixed domain of 2^%d rows and %d rounds, want 2^%d",
			plan.Params.LogPlainTextSize, res.NumRounds, res.MaxSizeLog2)
	}
	return res, nil
}

// staticSizeOrOne is the size of a sized module, 1 for a dynamic one: a
// committed layout's column order does not depend on the sizes with module
// groups.
func staticSizeOrOne(m *wiop.Module) int {
	if m.IsDynamic() {
		return 1
	}
	return m.Size()
}

// checkObliviousModule rejects a sized module whose rules would differ from
// a dynamic one's: below the smallest folded size, or too small for its
// shifts (see wiop.Module.MinRuntimeSize). A dynamic module is checked by
// the prover and the verifier.
func checkObliviousModule(m *wiop.Module, minSizeLog2 int) error {
	if m.IsDynamic() {
		return nil
	}
	n := utils.NextPowerOfTwo(m.Size())
	if n < 1<<minSizeLog2 || n < m.MinRuntimeSize() {
		return fmt.Errorf("pcs: sized module %q of size %d, below the 2^%d folded or the %d its shifts need",
			m.Context.Path(), n, minSizeLog2, m.MinRuntimeSize())
	}
	return nil
}

// layout returns the shapes and shifts of the batches for the module sizes
// sizeOf gives: the fri layout the prover commits and opens at those sizes.
func (o *ObliviousOpening) layout(sizeOf func(*wiop.Module) int) ([]fri.Shape, []fri.BatchShifts) {
	shapes := make([]fri.Shape, len(o.Batches))
	shifts := make([]fri.BatchShifts, len(o.Batches))
	for b, groups := range o.Groups {
		for _, g := range groups {
			n := utils.NextPowerOfTwo(sizeOf(g.Module))
			s := utils.Log2Ceil(n)
			for len(shapes[b]) <= s {
				shapes[b] = append(shapes[b], fri.SizedShape{})
				shifts[b] = append(shifts[b], fri.SizedShifts{})
			}
			sized := &shapes[b][s]
			sized.BaseWidth += len(g.Base)
			sized.ExtWidth += len(g.Ext)
			sized.Groups = append(sized.Groups, fri.GroupShape{
				BaseWidth: len(g.Base), ExtWidth: len(g.Ext), Ordinal: g.ModuleIndex,
			})
			reduce := func(cols []OpenedColumn) [][]int {
				res := make([][]int, 0, len(cols))
				for _, col := range cols {
					var r []int
					for _, sh := range col.Shifts {
						r = append(r, ((sh%n)+n)%n)
					}
					res = append(res, r)
				}
				return res
			}
			shifts[b][s].Base = append(shifts[b][s].Base, reduce(g.Base)...)
			shifts[b][s].Ext = append(shifts[b][s].Ext, reduce(g.Ext)...)
		}
		// as initializeBatchShift: a row list per size, empty or not
		for s := range shifts[b] {
			sized := &shifts[b][s]
			sized.Base = append(make([][]int, 0, len(sized.Base)), sized.Base...)
			sized.Ext = append(make([][]int, 0, len(sized.Ext)), sized.Ext...)
		}
	}
	return shapes, shifts
}

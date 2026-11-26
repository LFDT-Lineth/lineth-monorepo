package pcs

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// StaticOpening is the structure of a System's PCS opening when it does not
// depend on the proof: every module statically sized, and no column elision.
// It is what [compiled.verify] derives before reading the opening proof, for
// verifiers written in another proof system (the recursion machine's), as
// [OpeningVerifierAction.CheckGnark] uses it in gnark.
type StaticOpening struct {
	// Batches are the committed batches, in FRI order.
	Batches []BatchRef
	// Plan is the FRI verifier's plan for the batches' shapes and shifts.
	Plan fri.VerifierPlan
	// Claims holds the claim cells of the openings, laid out as the
	// fri.BatchClaimedValues of the batches (one per shift of Plan's
	// entries).
	Claims []StaticBatchClaims
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
	// QueryRange is the bound of the query positions: they are
	// RandomManyIntegers(NumQueries, QueryRange).
	QueryRange int
	// NumQueries is the number of FRI queries.
	NumQueries int
}

// StaticBatchClaims mirrors fri.BatchClaimedValues with the claim cells.
type StaticBatchClaims []StaticSizedClaims

// StaticSizedClaims mirrors fri.SizedClaimedValues with the claim cells.
type StaticSizedClaims struct {
	Base, Ext [][]*wiop.Cell
}

// Static returns the static structure of the opening of sys, the System
// whose verifier action a is. It fails with column elision (the layout is
// then the prover's choice) and with a dynamic module.
//
// It is computed once per action and shared by every caller (the circuit's
// allocation and definition, the recursion machine): the result must not be
// modified.
func (a *OpeningVerifierAction) Static(sys *wiop.System) (*StaticOpening, error) {
	if sys != a.sys {
		return nil, fmt.Errorf("pcs: Static: called with a System other than the action's")
	}
	a.staticOnce.Do(func() { a.static, a.staticErr = a.computeStatic(sys) })
	return a.static, a.staticErr
}

func (a *OpeningVerifierAction) computeStatic(sys *wiop.System) (*StaticOpening, error) {
	c := a.c
	if c.elisionEnabled {
		return nil, fmt.Errorf("pcs: column elision makes the opening's layout the prover's")
	}
	for _, mod := range sys.Modules {
		if mod.IsDynamic() {
			return nil, fmt.Errorf("pcs: module %q is dynamic", mod.Context.Path())
		}
	}
	batches := CommittedBatches(sys)
	manifests := allPresentManifests(batches)
	res := &StaticOpening{
		Batches:         batches,
		Claims:          make([]StaticBatchClaims, len(batches)),
		PrecomputedRoot: c.precomputedRoot,
		QueryRange:      c.effectiveNWith(staticSizeOf, batches, manifests),
		NumQueries:      c.friNumQueries,
	}
	for _, b := range batches {
		if !b.IsPrecomp {
			res.Manifests = append(res.Manifests, c.manifestCells[b.Round.ID]...)
		}
	}

	var (
		layouts = make([]map[wiop.ObjectID]ColumnLocation, len(batches))
		shapes  = make([]fri.Shape, len(batches))
		shifts  = make([]fri.BatchShifts, len(batches))
		batchOf = make(map[*wiop.Round]int, len(batches))
		seen    = make(map[claimKey]*wiop.Cell)
	)
	for i, b := range batches {
		layouts[i], shapes[i] = getLayoutWith(b.Round, staticSizeOf, manifests[i], c.moduleGroups)
		shifts[i] = initializeBatchShift(shapes[i])
		res.Claims[i] = make(StaticBatchClaims, len(shapes[i]))
		for s, sized := range shapes[i] {
			res.Claims[i][s] = StaticSizedClaims{
				Base: make([][]*wiop.Cell, sized.BaseWidth),
				Ext:  make([][]*wiop.Cell, sized.ExtWidth),
			}
		}
		batchOf[b.Round] = i
	}
	// As RecoverBatchClaims: one claim per (column, normalized shift), the
	// repeated ones checked equal to it.
	for _, eval := range sys.LagrangeEvals {
		res.EvalPoints = append(res.EvalPoints, eval.EvaluationPoint)
		for k, view := range eval.Polynomials {
			batchIdx, ok := batchOf[view.Column.Round()]
			if !ok {
				return nil, fmt.Errorf("pcs: column %q is in a round that owns no committed batch",
					view.Column.Context.Path())
			}
			loc := layouts[batchIdx][view.Column.Context.ID]
			size := 1 << loc.SizeID
			shift := ((view.ShiftingOffset % size) + size) % size
			cell := eval.EvaluationClaims[k]
			key := claimKey{batchIdx, loc.SizeID, loc.IsExt, loc.Position, shift}
			if prev, dup := seen[key]; dup {
				res.Duplicates = append(res.Duplicates, [2]*wiop.Cell{prev, cell})
				continue
			}
			seen[key] = cell
			sizedShift, sizedClaim := &shifts[batchIdx][loc.SizeID], &res.Claims[batchIdx][loc.SizeID]
			if loc.IsExt {
				sizedShift.Ext[loc.Position] = append(sizedShift.Ext[loc.Position], shift)
				sizedClaim.Ext[loc.Position] = append(sizedClaim.Ext[loc.Position], cell)
			} else {
				sizedShift.Base[loc.Position] = append(sizedShift.Base[loc.Position], shift)
				sizedClaim.Base[loc.Position] = append(sizedClaim.Base[loc.Position], cell)
			}
		}
	}
	if len(res.EvalPoints) == 0 {
		return nil, fmt.Errorf("pcs: no LagrangeEval queries to open")
	}

	plan, err := c.newPCS().VerifierPlan(shapes, shifts)
	if err != nil {
		return nil, err
	}
	res.Plan = plan
	return res, nil
}

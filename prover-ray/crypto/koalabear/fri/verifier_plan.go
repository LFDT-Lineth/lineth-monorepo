package fri

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// VerifierPlan is the structure of [PCS.Verify] for a statically known
// opening (the batches' shapes and shifts): everything it derives from them,
// before reading the proof. It is for verifiers written in another proof
// system, the recursion machine's for instance, which must lay out the same
// checks as data. It is computed by the functions Verify itself runs
// (canonicalLayout, restrictTo, inputCapShapeInfo, ...), so the two cannot
// drift apart.
type VerifierPlan struct {
	// Params is the schedule restricted to the largest opened size, as
	// Verify restricts it.
	Params Params
	// NumRounds is the number of FRI folds, LogCodewordSize the log size of
	// the top codeword domain (query positions are below 2^LogCodewordSize),
	// RateLog the log of the inverse rate and LogFinalPolySize the log
	// number of the final polynomial's coefficients.
	NumRounds, LogCodewordSize, RateLog, LogFinalPolySize int
	// Generators[j] generates the codeword domain of round j, for
	// j ≤ NumRounds: query position s of round j is the point
	// Generators[j]^bitrev(s).
	Generators []field.Element
	// Levels are the DEEP levels, in decreasing size (the canonical layout).
	Levels []PlanLevel
	// Trees are the input trees, in the order of an [InputQuery]'s openings.
	Trees []PlanTree

	// What Verify derives from the shapes and shifts, kept for
	// [VerifierPlan.VerifyGnark] and [VerifierPlan.GnarkOpeningProofShape] so that
	// neither derives it again.
	pcs               *PCS // restricted to the largest opened size
	shapes            []Shape
	shifts            []BatchShifts
	layout            layout
	orders            [][]int
	inputIndexByBatch []int          // tree of each batch, −1 for none
	capInfos          []inputCapInfo // per tree
}

// PlanLevel is one DEEP level: the opened columns of one size, joining FRI
// at round Round with alphaDeep FoldAlphas[Round]² (FoldAlphas[Round−1] at
// the boundary round NumRounds). Its value at a query point x is
//
//	a^RunningPower·running(x) + Σ_e a^(e.Power)·Σ_k (f_e(x) − y_(e,k))/(x − ζ·e.Rotations[k])
//
// with a the alphaDeep and y_(e,k) the claim of entry e at its k-th shift.
type PlanLevel struct {
	SizeLog2, Round, RunningPower int
	Entries                       []PlanEntry
}

// PlanEntry is one opened column of a level: row Row of the base or the
// extension rows of its size in batch Batch.
type PlanEntry struct {
	Batch, Row int
	IsExt      bool
	Power      int
	// Shifts are the column's opening shifts and Rotations their claim
	// points' factors ω_n^shift (n = 2^SizeLog2): the k-th claim is at
	// ζ·Rotations[k].
	Shifts    []int
	Rotations []field.Element
}

// PlanTree is the input tree of one batch: Height row levels (the bottom
// one, of the batch's largest size, and the auxiliary ones above it, see
// [MultiSizeTable.Merkleize]), a Merkle cap at depth CapDepth, and the
// batch's shape. Its leaf of a query at position s is s >> (LogCodewordSize −
// Height).
type PlanTree struct {
	Batch, Height, CapDepth int
	Grouped                 bool
	Shape                   Shape
}

// VerifierPlan returns the plan of [PCS.Verify] for openings of the given
// shapes and shifts, or the error Verify would return on them before reading
// the proof.
func (pcs *PCS) VerifierPlan(shapes []Shape, shifts []BatchShifts) (VerifierPlan, error) {
	layout, err := canonicalLayout(shapes, shifts)
	if err != nil {
		return VerifierPlan{}, err
	}
	restricted, err := pcs.restrictTo(layout.maxSizeLog2())
	if err != nil {
		return VerifierPlan{}, err
	}
	if err := restricted.checkFoldedSchedule(layout); err != nil {
		return VerifierPlan{}, fmt.Errorf("fri: VerifierPlan: %w", err)
	}
	p := restricted.Params
	plan := VerifierPlan{
		pcs:              &PCS{Params: p, Encoders: restricted.Encoders},
		shapes:           shapes,
		shifts:           shifts,
		layout:           layout,
		orders:           batchOrders(layout),
		Params:           p,
		NumRounds:        int(p.numRounds()),
		LogCodewordSize:  int(p.LogCodewordSize),
		RateLog:          int(p.LogCodewordSize) - int(p.LogPlainTextSize),
		LogFinalPolySize: int(p.logFinalPolySize),
	}
	for _, d := range p.domainsLight {
		plan.Generators = append(plan.Generators, d.generator)
	}
	for _, bundle := range layout {
		round, err := restricted.roundForSize(bundle.SizeLog2)
		if err != nil {
			return VerifierPlan{}, err
		}
		if round > p.numRounds() {
			return VerifierPlan{}, fmt.Errorf("fri: VerifierPlan: size %d joins at round %d, after the last %d",
				bundle.SizeLog2, round, p.numRounds())
		}
		level := PlanLevel{SizeLog2: int(bundle.SizeLog2), Round: int(round), RunningPower: bundle.RunningPower}
		for _, e := range bundle.Entries {
			entry := PlanEntry{
				Batch: e.BatchIdx, Row: e.RowIdx, IsExt: e.IsExt, Power: e.AlphaPower,
				Shifts: cloneInts(e.Shifts),
			}
			for _, s := range e.Shifts {
				rot, err := restricted.shiftedPoint(e.SizeLog2, s, field.OneExt())
				if err != nil {
					return VerifierPlan{}, err
				}
				entry.Rotations = append(entry.Rotations, rot.B0.A0)
			}
			level.Entries = append(level.Entries, entry)
		}
		plan.Levels = append(plan.Levels, level)
	}
	_, treeOf := inputOpeningRoots(layout, plan.orders, make([]field.Octuplet, len(shapes)))
	plan.inputIndexByBatch = treeOf
	plan.Trees = make([]PlanTree, 0, len(shapes))
	for b, t := range treeOf {
		if t < 0 {
			continue
		}
		info, err := inputCapShapeInfo(p, shapes[b])
		if err != nil {
			return VerifierPlan{}, fmt.Errorf("fri: VerifierPlan: batch %d: %w", b, err)
		}
		for len(plan.Trees) <= t {
			plan.Trees = append(plan.Trees, PlanTree{})
			plan.capInfos = append(plan.capInfos, inputCapInfo{})
		}
		plan.capInfos[t] = info
		plan.Trees[t] = PlanTree{Batch: b, Height: info.height, CapDepth: info.depth, Grouped: info.grouped, Shape: shapes[b]}
	}
	return plan, nil
}

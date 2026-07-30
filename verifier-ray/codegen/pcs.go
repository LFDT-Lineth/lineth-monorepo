package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// PcsSystem is the compile-time FRI/PCS description the Zig verifier consumes:
// the FRI params, the per-batch shapes and shift schedules (from which Zig's
// buildLayout reconstructs the canonical layout), the claim maps that re-slice
// the PCS-authenticated entry_claims into the vanishing witness/quotient claims,
// and the flat all_coins index of the shared opening point zeta.
//
// It is extracted from an ALREADY-compiled (global.Compile + pcs.Compile) and
// proven (sys, rt) protocol, so it can never drift from the prover's committed
// column ordering: batch order, per-batch layout, and the LagrangeEval openings
// all come from the prover-ray PCS compiler's own exported helpers.
type PcsSystem struct {
	SourceName string

	// FRI params (already restricted to the largest opened size, mirroring the
	// fixture: LogPlaintextSize == the max opened size, LogCodewordSize ==
	// that + FRILogInverseRate; the Zig verifier's restrictTo is then a no-op).
	LogCodewordSize  int
	LogPlaintextSize int
	LogFinalPolySize int
	NumQueries       int
	NumBatches       int

	// Per-batch declared shapes and shift schedules, batch-index aligned.
	Shapes []fri.Shape
	Shifts []fri.BatchShifts

	// Claim maps: WitnessMap[k] / QuotientMap[k] is the (entry, shift) the
	// vanishing System's k-th witness / quotient claim is re-sliced from. Their
	// lengths equal the vanishing System's TotalWitnessClaims / TotalQuotientClaims.
	WitnessMap  []PcsClaimRef
	QuotientMap []PcsClaimRef

	// ZetaCoinIndex is the flat all_coins index of the shared LagrangeEval eval
	// coin (zeta), which is also every vanishing module's eval coin.
	ZetaCoinIndex int
}

// PcsClaimRef is the verifier-ray verify.ClaimRef: the flat entry index plus the
// shift slot within that entry a single opened (column, shift) maps to.
type PcsClaimRef struct {
	Entry int
	Shift int
}

// pcsEntryKey identifies one opened column in the flat canonical entry order.
type pcsEntryKey struct {
	batchIdx int
	sizeLog2 int
	isExt    bool
	position int
}

// pcsLocWithBatch is a column location plus the batch index it belongs to.
type pcsLocWithBatch struct {
	batchIdx int
	loc      pcscompiler.ColumnLocation
}

// BuildPcsSystem extracts the FRI/PCS System from a compiled, proven protocol.
//
// Requires pcs.Compile to have run after global.Compile: the LagrangeEval
// openings (witness views + quotient shares) that the claim maps re-slice are
// registered by global.Compile, and pcs.Compile commits the batches and produces
// the opening proof. It reads only the committed batches, the LagrangeEvals, the
// coin routing, and the per-batch layout — nothing scenario-specific — so it
// works for any real protocol, not just the fixtures.
//
// `rt` must be the runtime that proved `sys` (it supplies the per-round layout
// sizes and the claimed evaluation values). `routing` is the shared coin layout
// from BuildCoinRouting; it locates the zeta coin in the flat all_coins array.
func BuildPcsSystem(sys *wiop.System, rt *wiop.Runtime, routing CoinRouting) (PcsSystem, error) {
	batches := pcscompiler.CommittedBatches(sys)
	if len(batches) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no committed batches; did pcs.Compile run?")
	}
	if len(sys.LagrangeEvals) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no LagrangeEval openings; did global.Compile run before pcs.Compile?")
	}

	// Per-batch layout + shapes, and a global column->location map (with batch).
	colLoc := map[wiop.ObjectID]pcsLocWithBatch{}
	shapes := make([]fri.Shape, len(batches))
	for i, b := range batches {
		locs, shape := pcscompiler.GetLayout(b.Round, rt)
		shapes[i] = shape
		for id, l := range locs {
			colLoc[id] = pcsLocWithBatch{batchIdx: i, loc: l}
		}
	}

	// Shifts: for each opened (column, shift), record the normalized shift into
	// the batch's per-size schedule, keyed to the column's row. shiftOrder keeps
	// the per-key ordering that fixes each opened value's shift slot.
	shifts := make([]fri.BatchShifts, len(batches))
	for i := range batches {
		shifts[i] = initPcsShifts(shapes[i])
	}
	shiftOrder := map[pcsEntryKey][]int{}
	seen := map[pcsEntryKey]map[int]bool{}

	recordClaim := func(cv *wiop.ColumnView) error {
		lb, ok := colLoc[cv.Column.Context.ID]
		if !ok {
			return fmt.Errorf("codegen: BuildPcsSystem: opened column %q not in any committed batch",
				cv.Column.Context.Path())
		}
		size := 1 << lb.loc.SizeID
		shift := ((cv.ShiftingOffset % size) + size) % size
		key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
		if seen[key] == nil {
			seen[key] = map[int]bool{}
		}
		if seen[key][shift] {
			return nil // deduplicate repeated (column, shift) openings
		}
		seen[key][shift] = true
		shiftOrder[key] = append(shiftOrder[key], shift)
		ss := &shifts[key.batchIdx][key.sizeLog2]
		if key.isExt {
			ss.Ext[key.position] = append(ss.Ext[key.position], shift)
		} else {
			ss.Base[key.position] = append(ss.Base[key.position], shift)
		}
		return nil
	}

	for _, le := range sys.LagrangeEvals {
		for _, cv := range le.Polynomials {
			if err := recordClaim(cv); err != nil {
				return PcsSystem{}, err
			}
		}
	}

	// Flat canonical entry order: size DESC, batch ASC, base-then-ext, position
	// ASC — matching Zig's buildLayout enumeration. Assign each key its entry g.
	maxSize := 0
	for _, sh := range shapes {
		if len(sh)-1 > maxSize {
			maxSize = len(sh) - 1
		}
	}
	entryIndex := map[pcsEntryKey]int{}
	g := 0
	for sizeLog2 := maxSize; sizeLog2 >= 0; sizeLog2-- {
		for bi := range batches {
			if sizeLog2 >= len(shapes[bi]) {
				continue
			}
			sh := shapes[bi][sizeLog2]
			for pos := 0; pos < sh.BaseWidth; pos++ {
				entryIndex[pcsEntryKey{batchIdx: bi, sizeLog2: sizeLog2, isExt: false, position: pos}] = g
				g++
			}
			for pos := 0; pos < sh.ExtWidth; pos++ {
				entryIndex[pcsEntryKey{batchIdx: bi, sizeLog2: sizeLog2, isExt: true, position: pos}] = g
				g++
			}
		}
	}

	witnessMap, quotientMap, err := buildPcsClaimMaps(sys, colLoc, entryIndex, shiftOrder)
	if err != nil {
		return PcsSystem{}, err
	}

	zetaIdx, err := pcsZetaCoinIndex(sys, routing)
	if err != nil {
		return PcsSystem{}, err
	}

	return PcsSystem{
		SourceName:       sys.Context.Path(),
		LogCodewordSize:  maxSize + pcscompiler.FRILogInverseRate,
		LogPlaintextSize: maxSize,
		LogFinalPolySize: 0,
		NumQueries:       pcscompiler.FRINumQueries(),
		NumBatches:       len(batches),
		Shapes:           shapes,
		Shifts:           shifts,
		WitnessMap:       witnessMap,
		QuotientMap:      quotientMap,
		ZetaCoinIndex:    zetaIdx,
	}, nil
}

func initPcsShifts(shape fri.Shape) fri.BatchShifts {
	bs := make(fri.BatchShifts, len(shape))
	for i, ss := range shape {
		bs[i] = fri.SizedShifts{
			Base: make([][]int, ss.BaseWidth),
			Ext:  make([][]int, ss.ExtWidth),
		}
	}
	return bs
}

// buildPcsClaimMaps produces the witness/quotient claim maps in the SAME order
// BuildVanishingSystem enumerates its flat witness/quotient claims (per
// global.Verifier: WitnessClaims, then per bucket QuotientClaims). Each claim
// CELL is a LagrangeEval.EvaluationClaims entry whose paired Polynomials[k] gives
// the opened (column, shift); that maps to a flat entry index + shift slot. This
// is the concrete realization of the invariant that the vanishing claims ARE a
// re-slicing of entry_claims.
func buildPcsClaimMaps(
	sys *wiop.System,
	colLoc map[wiop.ObjectID]pcsLocWithBatch,
	entryIndex map[pcsEntryKey]int,
	shiftOrder map[pcsEntryKey][]int,
) (witnessMap, quotientMap []PcsClaimRef, err error) {
	// cell ObjectID -> the column view it opens, via the LagrangeEvals.
	cellView := map[wiop.ObjectID]*wiop.ColumnView{}
	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			cellView[le.EvaluationClaims[k].Context.ID] = cv
		}
	}

	refFor := func(cell *wiop.Cell) (PcsClaimRef, error) {
		cv, ok := cellView[cell.Context.ID]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf(
				"codegen: BuildPcsSystem: vanishing claim cell %q has no LagrangeEval opening — "+
					"the column it opens is not PCS-authenticated", cell.Context.Path())
		}
		lb := colLoc[cv.Column.Context.ID]
		size := 1 << lb.loc.SizeID
		shift := ((cv.ShiftingOffset % size) + size) % size
		key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
		entry, ok := entryIndex[key]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: no flat entry for %+v", key)
		}
		slot := -1
		for i, s := range shiftOrder[key] {
			if s == shift {
				slot = i
				break
			}
		}
		if slot < 0 {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: shift %d not found for entry %+v", shift, key)
		}
		return PcsClaimRef{Entry: entry, Shift: slot}, nil
	}

	for _, verifier := range pcsGlobalVerifiers(sys) {
		for _, cell := range verifier.WitnessClaims {
			ref, e := refFor(cell)
			if e != nil {
				return nil, nil, e
			}
			witnessMap = append(witnessMap, ref)
		}
		for _, bucket := range verifier.Buckets {
			for _, cell := range bucket.QuotientClaims {
				ref, e := refFor(cell)
				if e != nil {
					return nil, nil, e
				}
				quotientMap = append(quotientMap, ref)
			}
		}
	}
	return witnessMap, quotientMap, nil
}

// pcsGlobalVerifiers collects the compiled global.Verifier actions in
// round/registration order — the SAME order BuildVanishingSystem walks, so the
// claim maps align index-for-index with the vanishing System's flat claims.
func pcsGlobalVerifiers(sys *wiop.System) []*global.Verifier {
	var verifiers []*global.Verifier
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if v, ok := action.(*global.Verifier); ok {
				verifiers = append(verifiers, v)
			}
		}
	}
	return verifiers
}

// pcsZetaCoinIndex returns the flat all_coins index of the shared LagrangeEval
// eval coin (zeta). All LagrangeEvals share this coin (global.Compile's
// evalCoin), so reading it from the first is sufficient.
func pcsZetaCoinIndex(sys *wiop.System, routing CoinRouting) (int, error) {
	coin, ok := sys.LagrangeEvals[0].EvaluationPoint.(*wiop.CoinField)
	if !ok {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: LagrangeEval EvaluationPoint is not a CoinField")
	}
	round := coin.Context.ID.Slot()
	pos := coin.Context.ID.Position()
	if round >= len(routing.RoundCoinOffsets) {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: zeta coin round %d out of range", round)
	}
	idx := routing.RoundCoinOffsets[round] + pos
	if idx >= routing.TotalRoundCoins {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: zeta coin flat index %d >= total %d", idx, routing.TotalRoundCoins)
	}
	return idx, nil
}

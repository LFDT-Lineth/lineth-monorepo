// Package logderivativesum implements the LogDerivativeSum compiler pass for
// the wiop protocol framework.
//
// Each [wiop.Fraction] carries an optional Filter expression on top of the
// (Numerator, Denominator) pair, and the contribution of a fraction at row i is
//
//	Filter[i] · Numerator[i] / Denominator[i]
//
// (with a nil Filter treated as constant 1). This is the natural target for
// conditional lookups, where rows with Filter[i] = 0 should not contribute to
// the running sum even if Denominator[i] would not be invertible on those
// rows.
//
// The compiler reduces every [wiop.LogDerivativeSum] query into:
//
//   - one or more "running-sum" extension columns Z, each absorbing up to
//     packingArity fractions whose vector-valued sides live on the same module;
//   - a vanishing recurrence per Z column linking it to its source fractions;
//   - a local constraint pinning the row-0 boundary of each Z column;
//   - an opening of Z[n-1] (column endpoint) per Z column;
//   - a verifier action that checks the sum of endpoints matches the query's
//     claimed Result cell.
//
// The prover-side computation is filter-aware: rows with a zero filter are
// skipped without inverting the corresponding denominator. This is what
// allows the compiler to be used for conditional lookups where the
// denominator may be ill-defined on filtered-out rows.
//
// The constraint system itself does not enforce non-zero denominators;
// callers should ensure denominators are non-zero on every row (typically by
// binding them to a randomness coin) so that the recurrence uniquely pins
// down Z.
package logderivativesum

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/hugepage"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
)

// packingArity is the maximum number of fractions packed into a single Z
// column. The value matches the linea/logderivativesum compiler.
const packingArity = 3

// Compile reduces every [wiop.LogDerivativeSum] query in sys to a Z-column
// recurrence plus endpoint openings, and registers prover/verifier
// actions that tie the resulting artefacts back to the query's Result cell.
// Already-reduced queries are skipped.
func Compile(sys *wiop.System) {
	for _, ld := range sys.LogDerivativeSums {
		if ld.IsReduced() {
			continue
		}
		compileQuery(ld)
		ld.MarkAsReduced()
	}
}

// compileQuery reduces a single LogDerivativeSum query.
func compileQuery(ld *wiop.LogDerivativeSum) {
	resultRound := ld.Result.Round()
	compCtx := ld.Context().Childf("logderiv-compile")

	buckets := bucketByModule(ld.Fractions)

	var entries []ZEntry
	for bIdx, b := range buckets {
		groups := packFractions(b.fractions)
		for kIdx, packed := range groups {
			entries = append(entries,
				buildZ(b.module, packed, resultRound, compCtx, bIdx, kIdx))
		}
	}

	resultRound.RegisterAction(&proverAction{ld: ld, entries: entries})
	resultRound.RegisterVerifierAction(&VerifierAction{LogDerivativeSum: ld, Entries: entries})
}

// fractionBucket groups fractions whose vector-valued side lives on the same
// module. Z columns are committed to that module.
type fractionBucket struct {
	module    *wiop.Module
	fractions []wiop.Fraction
}

// bucketByModule groups fractions by the module that owns their vector-valued
// side. Order of first appearance is preserved so the compilation output is
// deterministic.
func bucketByModule(fractions []wiop.Fraction) []fractionBucket {
	indexByModule := make(map[*wiop.Module]int)
	var buckets []fractionBucket
	for _, f := range fractions {
		m := fractionModule(f)
		i, ok := indexByModule[m]
		if !ok {
			i = len(buckets)
			indexByModule[m] = i
			buckets = append(buckets, fractionBucket{module: m})
		}
		buckets[i].fractions = append(buckets[i].fractions, f)
	}
	return buckets
}

// packFractions splits a list of fractions into groups of at most packingArity.
func packFractions(fractions []wiop.Fraction) [][]wiop.Fraction {
	groups := make([][]wiop.Fraction, 0, utils.DivCeil(len(fractions), packingArity))
	for k := 0; k < len(fractions); k += packingArity {
		end := k + packingArity
		if end > len(fractions) {
			end = len(fractions)
		}
		groups = append(groups, fractions[k:end])
	}
	return groups
}

// fractionModule returns the module that owns the vector-valued side of f.
// The LogDerivativeSum constructor guarantees at least one of Numerator and
// Denominator is vector-valued, so the result is never nil.
func fractionModule(f wiop.Fraction) *wiop.Module {
	if m := f.Numerator.Module(); m != nil {
		return m
	}
	return f.Denominator.Module()
}

// ZEntry collects the per-Z artefacts shared by the prover and verifier
// actions: the Z column, the raw fractions (filter, num, den) used by the
// prover for filter-aware row skipping, and the column endpoint opening.
type ZEntry struct {
	zCol   *wiop.Column
	packed []wiop.Fraction // raw fractions used by the prover for filter-aware evaluation
	// ZFinal is the (round, slot) coordinate of the Z[n-1] opening in the proof transcript; carries no witness value.
	ZFinal *wiop.Cell
}

// buildZ allocates one Z column for a packed fraction group, registers the
// recurrence Vanishing, pins the row-0 boundary with a local constraint, and
// opens the column endpoint (a lazy [wiop.Cell] returned by
// [ColumnPosition.Open]). The module's size does not need to be known at
// compile time: the endpoint opening at Z[last] is addressed via
// [ColumnPosition]'s negative-row convention (Position = −1 ⇒ last row),
// and Vanishing's cancelled-positions logic gracefully handles a runtime
// size of 1 by automatically skipping row 0 (the only row of a one-row
// module), so the recurrence Vanishing is registered unconditionally for
// dynamic modules.
func buildZ(
	m *wiop.Module,
	packed []wiop.Fraction,
	round *wiop.Round,
	ctx *wiop.ContextFrame,
	bIdx, kIdx int,
) ZEntry {
	zNum, zDen := buildZExpressions(packed)

	zCol := m.NewExtensionColumn(
		ctx.Childf("z-b%d-k%d", bIdx, kIdx),
		round)

	// The recurrence zNum − (Z − Z<<−1)·zDen carries a −1 shift on Z, so
	// NewVanishing automatically cancels row 0; the row-0 boundary is pinned
	// separately by the local constraint below.
	//
	// For a *statically* one-row module the recurrence is vacuous and we
	// skip it as an optimisation. For a dynamic module we cannot know the
	// runtime size at compile time, so the Vanishing is registered
	// unconditionally; Vanishing.Check's cancelled-positions logic makes
	// the constraint vacuous if RuntimeSize ends up being 1.
	if m.IsDynamic() || m.Size() > 1 {
		zView := zCol.View()
		recurrence := wiop.Sub(
			zNum,
			wiop.Mul(
				// Shift(-1) is load-bearing for the dynamic n=1 corner case: it puts row 0 in NewVanishing's
				// CancelledPositions, so when RuntimeSize == 1 the only row is cancelled and Check is vacuous.
				wiop.Sub(zView, zView.Shift(-1)),
				zDen,
			),
		)
		m.NewVanishing(
			ctx.Childf("z-recurrence-b%d-k%d", bIdx, kIdx),
			recurrence,
		)
	}

	// Initial condition at row 0: zNum[0] − Z[0]·zDen[0] = 0. The recurrence
	// cancels row 0, so the boundary is pinned here as a sound local constraint
	// (lifted by localvanishing and discharged by global) rather than by the
	// verifier reading the oracle witness columns. The constraint lives on row
	// 0, which always exists, so it is well-defined for dynamic modules too;
	// it also covers the single-row (RuntimeSize == 1) case where the
	// recurrence is skipped or made vacuous.
	m.NewLocalConstraint(
		ctx.Childf("z-init-b%d-k%d", bIdx, kIdx),
		wiop.Sub(zNum, wiop.Mul(zCol.View(), zDen)),
		0,
	)

	// Endpoint opening Z[last] for the running-sum total. For static modules we
	// resolve the endpoint to a concrete row index up front so the downstream
	// localvanishing pass — which lowers each opening's binding Vanishing via a
	// [LagrangeSelector] — sees only non-negative positions. Dynamic modules
	// keep the negative-row form (Position = −1 resolved to RuntimeSize−1
	// per-Runtime by [ColumnPosition.resolvedRow]).
	endpointPos := -1
	if !m.IsDynamic() {
		endpointPos = m.Size() - 1
	}
	zFinal := zCol.At(endpointPos).Open(ctx.Childf("z-final-b%d-k%d", bIdx, kIdx))

	return ZEntry{
		zCol:   zCol,
		packed: packed,
		ZFinal: zFinal,
	}
}

// buildZExpressions packs up to packingArity filter-aware fractions into a
// single Numerator/Denominator pair using the cross-product identity
//
//	Σ_j (F_j · N_j) / D_j = (Σ_j F_j · N_j · ∏_{k≠j} D_k) / (∏_k D_k).
//
// A nil Filter is treated as the constant 1, in which case the j-th term
// reduces to N_j · ∏_{k≠j} D_k as in the non-filter compiler.
func buildZExpressions(packed []wiop.Fraction) (zNum, zDen wiop.Expression) {
	zDen = packed[0].Denominator
	for i := 1; i < len(packed); i++ {
		zDen = wiop.Mul(zDen, packed[i].Denominator)
	}

	for j := range packed {
		// effectiveNum_j = Filter_j · Num_j (with nil Filter → Num_j).
		term := packed[j].Numerator
		if packed[j].Filter != nil {
			term = wiop.Mul(packed[j].Filter, term)
		}
		for k := range packed {
			if k != j {
				term = wiop.Mul(term, packed[k].Denominator)
			}
		}
		if zNum == nil {
			zNum = term
		} else {
			zNum = wiop.Add(zNum, term)
		}
	}
	return zNum, zDen
}

// proverAction computes each Z column from its packed fractions, assigns the
// Z column and its endpoint openings, and writes the aggregated sum into
// ld.Result.
type proverAction struct {
	ld      *wiop.LogDerivativeSum
	entries []ZEntry
}

// Run implements [wiop.ProverAction].
//
// The Z columns are independent of one another, so they are computed
// concurrently, largest module first to balance the workers. Each entry only
// writes its own Z column and its own slot of finals; the total is summed
// afterwards in entry order.
//
// Each entry's inner loops get a share of the CPUs proportional to its row
// count (at least one), so the largest columns -- which bound the wall time --
// are themselves computed in parallel, while the shares add up to about
// GOMAXPROCS.
func (a *proverAction) Run(rt *wiop.Runtime) {
	sizes := make([]int, len(a.entries))
	order := make([]int, len(a.entries))
	total := 0
	for i, e := range a.entries {
		sizes[i] = e.zCol.Module.RuntimeSize(rt)
		order[i] = i
		total += sizes[i]
	}
	sort.SliceStable(order, func(i, j int) bool { return sizes[order[i]] > sizes[order[j]] })

	cpus := runtime.GOMAXPROCS(0)
	finals := make([]field.Ext, len(a.entries))
	parallel.ExecuteDynamic(len(order), func(k int) {
		i := order[k]
		e, n := a.entries[i], sizes[i]
		workers := 1
		if total > 0 {
			workers = max(1, cpus*n/total)
		}
		z := computeFilteredPrefixSum(rt, e.packed, n, workers)

		rt.AssignColumn(e.zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(z)})

		// zFinal is a lazy opening of Z[n-1]; it resolves from this column
		// assignment on first read (or at round advance), so no explicit
		// assignment is needed here.

		finals[i] = z[n-1]
	})

	var sum field.Ext
	for i := range finals {
		sum.Add(&sum, &finals[i])
	}

	if !rt.HasCellAssignment(a.ld.Result) {
		rt.AssignCell(a.ld.Result, field.ElemFromExt(sum))
	}
}

// computeFilteredPrefixSum returns the running-sum
//
//	Z[i] = Σ_{k≤i, j} F_j[k] · N_j[k] / D_j[k]
//
// over the rows of a packed fraction group, skipping rows where the
// fraction's filter is zero. A zero denominator at a filtered-out row is
// benign: its inverse is taken as zero and multiplied by the zero filter.
//
// Panics if a fraction's denominator is zero on a row where its filter is
// non-zero, since that input is malformed.
//
// The row terms Σ_j F_j·N_j/D_j are computed by one [vecprog] program over
// every fraction of the group: subexpressions shared by the fractions are
// computed once, the denominators' random linear combinations become linear
// forms, and each block of rows takes one batch inversion per denominator,
// without materialising the fractions' vectors. The running sum is then a
// two-pass chunked scan (each chunk sums locally, then adds the total of the
// chunks before it). workers bounds the goroutines used. Field arithmetic is
// exact, so the result does not depend on workers.
func computeFilteredPrefixSum(rt *wiop.Runtime, packed []wiop.Fraction, n int, workers int) []field.Ext {
	l := wiop.NewRowLowering(rt, n)
	b := l.B
	var one field.Ext
	one.SetOne()
	rows := make([]field.Ext, n)
	hugepage.Advise(rows)
	for j, p := range packed {
		den := l.Lower(p.Denominator)
		term := b.Op(vecprog.Div, l.Lower(p.Numerator), den)
		guard := b.Scalar(one, true)
		if p.Filter != nil {
			guard = l.Lower(p.Filter)
			term = b.Op(vecprog.Mul, term, guard)
		}
		b.Check(den, guard, func(i int) {
			panic(fmt.Sprintf(
				"wiop/compilers/logderivativesum: zero denominator at row %d for fraction %d "+
					"with non-zero filter; the filter must mask this row",
				i, j,
			))
		})
		b.AddTo(term, rows)
	}
	b.Compile(n, 1, 1).Run(nil, nil, workers)

	// First pass: each chunk's running sum from zero, and its total.
	chunk := max(prefixSumMinChunk, (n+workers-1)/workers)
	nbChunks := (n + chunk - 1) / chunk
	bounds := func(c int) (int, int) { return c * chunk, min((c+1)*chunk, n) }
	z := rows // the running sum overwrites the row terms in place
	chunkTotals := make([]field.Ext, nbChunks)
	parallel.Execute(nbChunks, func(start, end int) {
		for c := start; c < end; c++ {
			lo, hi := bounds(c)
			var running field.Ext
			for i := lo; i < hi; i++ {
				running.Add(&running, &rows[i])
				z[i] = running
			}
			chunkTotals[c] = running
		}
	}, workers)

	// Second pass: shift every chunk but the first by the total of the
	// chunks before it.
	if nbChunks < 2 {
		return z
	}
	offsets := make([]field.Ext, nbChunks)
	for c := 1; c < nbChunks; c++ {
		offsets[c].Add(&offsets[c-1], &chunkTotals[c-1])
	}
	parallel.Execute(nbChunks-1, func(start, end int) {
		for c := start + 1; c < end+1; c++ {
			lo, hi := bounds(c)
			for i := lo; i < hi; i++ {
				z[i].Add(&z[i], &offsets[c])
			}
		}
	}, workers)
	return z
}

// prefixSumMinChunk is the smallest number of rows a worker of
// [computeFilteredPrefixSum] handles: below it, splitting costs more than it
// saves.
const prefixSumMinChunk = 4096

// VerifierAction enforces the only boundary identity that is not already
// pinned in-circuit: the sum of all Z[n-1] endpoint openings equals the
// claimed Result cell value. The per-Z initial condition is enforced by the
// row-0 local constraint registered in buildZ, so this action reads only
// local openings (cells) — never the oracle witness columns.
//
// Exported (with exported fields) so out-of-package consumers — notably the
// verifier-ray codegen — can read the endpoint openings and the Result cell.
type VerifierAction struct {
	LogDerivativeSum *wiop.LogDerivativeSum
	Entries          []ZEntry
}

// Check implements [wiop.VerifierAction].
func (a *VerifierAction) Check(rt *wiop.Runtime) error {
	var sum field.Ext

	for _, e := range a.Entries {
		zFinal := rt.GetCellValue(e.ZFinal).AsExt()
		sum.Add(&sum, &zFinal)
	}

	claimed := rt.GetCellValue(a.LogDerivativeSum.Result).AsExt()
	var diff field.Ext
	diff.Sub(&sum, &claimed)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/logderivativesum: final-sum check failed for query %q",
			a.LogDerivativeSum.Context().Path(),
		)
	}
	return nil
}

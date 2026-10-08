package global

import (
	"fmt"
	"runtime"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// Compile adds the global-quotient compilation pass to sys. It groups each
// module's vanishing constraints by their quotient ratio, commits to one set of
// extension-field quotient-share columns per (module, ratio) pair, and
// registers prover and verifier actions that carry out the PLONK quotient
// argument at a fresh random evaluation point.
//
// Two new rounds are appended to the system:
//   - quotientRound: holds the merging coin (per module) and the quotient
//     share columns.
//   - evalRound: holds the evaluation coin (per module), the evaluation-claim
//     cells for both witness columns and quotient shares, and the verifier
//     check.
//
// This compiler supports dynamic-size modules. The quotient ratio is computed
// from the expression's DegreeFactor() which doesn't require knowing the module
// size at compile time. Size-dependent data (FFT domains, annihilator inverses,
// cancellation cosets) is computed at runtime using RuntimeSize.
func Compile(sys *wiop.System) {
	var hasWork bool
	for _, m := range sys.Modules {
		if len(m.Vanishings) > 0 {
			hasWork = true
			break
		}
	}
	if !hasWork {
		return
	}

	quotientRound := sys.NewRound()
	evalRound := sys.NewRound()
	compCtx := sys.Context.Childf("global-quotient")

	// The evaluation coin is shared across the entire compilation: every module
	// is opened at the same random point r. It lives in evalRound (after all
	// quotient share columns have been committed in quotientRound).
	evalCoin := evalRound.NewCoinField(compCtx.Childf("eval-coin"))

	for i, m := range sys.Modules {
		if len(m.Vanishings) == 0 {
			continue
		}
		mCtx := compCtx.Childf("m%d", i)
		compileModule(sys, m, mCtx, quotientRound, evalRound, evalCoin)
	}
}

// colViewKey is a map key for deduplicating column views (column + shift).
type colViewKey struct {
	id    wiop.ObjectID
	shift int
}

// rawBucket is an intermediate compilation description of one ratio bucket
// before runtime artefacts (domains, annihilator inverses, etc.) are built.
type rawBucket struct {
	ratio      int
	vanishings []*wiop.Vanishing
	shares     []*wiop.Column
}

// proverVanishingEntry bundles a Vanishing with the precomputed base-field
// evaluations of its cancellation polynomial on the large coset.
// cancellationCoset[j] = C(g · ω_{N}^j) where g is the multiplicative
// generator and N = n · ratio. Only populated for static modules.
type proverVanishingEntry struct {
	v                 *wiop.Vanishing
	cancellationCoset []field.Element // length N = n*ratio; nil if no cancellation
}

// proverBucket holds all compilation artefacts needed by the prover to compute
// the quotient shares for one ratio bucket.
//
// For static modules, size-dependent data (FFT domains, annihilator inverses,
// cancellation cosets) is precomputed at compile time. For dynamic modules,
// these fields are nil and the data is computed at runtime using RuntimeSize.
type proverBucket struct {
	ratio    int
	rootCols []*wiop.Column // deduplicated root columns from all expressions
	shares   []*wiop.Column // quotient share columns (length = ratio)

	// --- Static-module fields (nil for dynamic modules) ---
	entries      []proverVanishingEntry // precomputed cancellation cosets
	smallDomain  *fft.Domain            // FFT domain of size n
	cosetDomains []*fft.Domain          // size-n domains shifted to each small coset (see cosetShifts)
	annInv       []field.Element        // 1/(g^n · ω_ratio^k − 1) for k = 0..ratio-1

	// --- Dynamic-module fields (nil for static modules) ---
	vanishings []*wiop.Vanishing // raw vanishings for runtime computation

	// Pre-allocated scratch slices populated by Plan; nil until Plan is called.
	// When non-nil, Run uses these instead of allocating fresh memory.
	scratchAgg []field.Ext // aggregate[j], length N = n*ratio
}

// VerifierBucket holds everything the verifier needs for one ratio bucket.
type VerifierBucket struct {
	Ratio          int
	Vanishings     []*wiop.Vanishing
	QuotientClaims []*wiop.Cell // Q_k(r) claim cells, length = ratio
}

// ---------------------------------------------------------------------------
// Module-level compilation
// ---------------------------------------------------------------------------

func compileModule(
	sys *wiop.System,
	m *wiop.Module,
	ctx *wiop.ContextFrame,
	quotientRound, evalRound *wiop.Round,
	evalCoin *wiop.CoinField,
) {
	// Static modules must be sized before compilation.
	if !m.IsDynamic() && !m.IsSized() {
		panic(fmt.Sprintf("wiop/compilers: static module %q must be sized before calling Compile", m.Context.Path()))
	}

	// --- Step 1: bucket vanishing constraints by ratio ---
	// Ratio is computed from DegreeFactor() which doesn't require knowing the
	// module size, allowing compilation to proceed for dynamic-size modules.
	// Vanishings already consumed by an earlier pass (e.g. localvanishing, which
	// marks the scalar input it lifts as reduced and registers a fresh
	// multi-valued replacement) are skipped here.
	ratioToEntries := make(map[int][]*wiop.Vanishing)
	var ratioOrder []int
	for _, v := range m.Vanishings {
		if v.IsReduced() {
			continue
		}
		r := computeRatio(v)
		if _, exists := ratioToEntries[r]; !exists {
			ratioOrder = append(ratioOrder, r)
		}
		ratioToEntries[r] = append(ratioToEntries[r], v)
	}

	// --- Step 2: merging coin in quotientRound ---
	mergeCoin := quotientRound.NewCoinField(ctx.Childf("merge-coin"))

	// --- Step 3: declare quotient share columns (extension) per bucket ---
	rawBuckets := make([]rawBucket, 0, len(ratioOrder))
	for _, ratio := range ratioOrder {
		vs := ratioToEntries[ratio]
		shares := make([]*wiop.Column, ratio)
		for k := range ratio {
			shareCtx := ctx.Childf("q-r%d-s%d", ratio, k)
			shares[k] = m.NewExtensionColumn(shareCtx, quotientRound)
		}
		rawBuckets = append(rawBuckets, rawBucket{ratio: ratio, vanishings: vs, shares: shares})
	}

	// --- Step 4: eval coin ---
	// The eval coin is shared across the whole compilation (created in Compile);
	// every module is opened at the same random evaluation point.

	// --- Step 5: collect all unique column views across all vanishings ---
	viewKeyToIdx := make(map[colViewKey]int)
	var views []*wiop.ColumnView
	for _, bkt := range rawBuckets {
		for _, v := range bkt.vanishings {
			for _, cv := range collectColumnViews(v.Expression) {
				key := colViewKey{id: cv.Column.Context.ID, shift: cv.ShiftingOffset}
				if _, exists := viewKeyToIdx[key]; !exists {
					viewKeyToIdx[key] = len(views)
					views = append(views, cv)
				}
			}
		}
	}

	// --- Step 6: claim cells for witness LagrangeEval (all extension, eval at ext point) ---
	witnessClaims := make([]*wiop.Cell, len(views))
	for i := range views {
		witnessClaims[i] = evalRound.NewCell(ctx.Childf("w-claim%d", i), true)
	}
	var witnessLagrangeEval *wiop.LagrangeEval
	if len(views) > 0 {
		witnessLagrangeEval = sys.NewLagrangeEvalFrom(
			ctx.Childf("witness-eval"),
			views,
			evalCoin,
			witnessClaims,
		)
	}

	// --- Step 7: LagrangeEvals for quotient shares ---
	allLagrangeEvals := make([]*wiop.LagrangeEval, 0)
	if witnessLagrangeEval != nil {
		allLagrangeEvals = append(allLagrangeEvals, witnessLagrangeEval)
	}
	quotientBucketClaims := make([][]*wiop.Cell, len(rawBuckets))
	for i, bkt := range rawBuckets {
		shareViews := make([]*wiop.ColumnView, bkt.ratio)
		claimsForBucket := make([]*wiop.Cell, bkt.ratio)
		for k, shareCol := range bkt.shares {
			shareViews[k] = shareCol.View()
			claimsForBucket[k] = evalRound.NewCell(ctx.Childf("q-claim-r%d-s%d", bkt.ratio, k), true)
		}
		qLE := sys.NewLagrangeEvalFrom(
			ctx.Childf("q-eval%d", i),
			shareViews,
			evalCoin,
			claimsForBucket,
		)
		allLagrangeEvals = append(allLagrangeEvals, qLE)
		quotientBucketClaims[i] = claimsForBucket
	}

	// --- Step 8: build prover buckets ---
	// For static modules, precompute size-dependent data (FFT domains, annihilator
	// inverses, cancellation cosets). For dynamic modules, defer to runtime.
	proverBuckets := buildProverBuckets(rawBuckets, m)

	// --- Step 9: register prover actions ---
	quotientRound.RegisterAction(&QuotientProverAction{
		m:         m,
		mergeCoin: mergeCoin,
		buckets:   proverBuckets,
	})
	evalRound.RegisterAction(&EvalProverAction{
		lagrangeEvals: allLagrangeEvals,
	})

	// --- Step 10: register verifier action ---
	vBuckets := make([]VerifierBucket, len(rawBuckets))
	for i, bkt := range rawBuckets {
		vBuckets[i] = VerifierBucket{
			Ratio:          bkt.ratio,
			Vanishings:     bkt.vanishings,
			QuotientClaims: quotientBucketClaims[i],
		}
	}
	evalRound.RegisterVerifierAction(&Verifier{
		Module:        m,
		MergeCoin:     mergeCoin,
		EvalCoin:      evalCoin,
		WitnessViews:  views,
		WitnessClaims: witnessClaims,
		viewKeyToIdx:  viewKeyToIdx,
		Buckets:       vBuckets,
	})
}

// buildProverBuckets constructs the prover buckets from the raw bucket
// descriptions. For static modules, size-dependent data (FFT domains,
// annihilator inverses, cancellation cosets) is precomputed. For dynamic
// modules, these are left nil and computed at runtime using RuntimeSize.
func buildProverBuckets(rawBuckets []rawBucket, m *wiop.Module) []proverBucket {
	result := make([]proverBucket, len(rawBuckets))

	// For static modules, get n now; for dynamic, n=0 signals runtime computation.
	var n int
	if !m.IsDynamic() {
		n = m.Size()
	}

	for i, bkt := range rawBuckets {
		ratio := bkt.ratio

		// Collect deduplicated root columns from all expressions.
		rootColsSeen := make(map[wiop.ObjectID]*wiop.Column)
		for _, v := range bkt.vanishings {
			for _, col := range collectRootColumns(v.Expression) {
				rootColsSeen[col.Context.ID] = col
			}
		}
		rootCols := make([]*wiop.Column, 0, len(rootColsSeen))
		for _, col := range rootColsSeen {
			rootCols = append(rootCols, col)
		}

		pb := proverBucket{
			ratio:    ratio,
			rootCols: rootCols,
			shares:   bkt.shares,
		}

		if m.IsDynamic() {
			// Dynamic module: store vanishings for runtime computation.
			pb.vanishings = bkt.vanishings
		} else {
			// Static module: precompute size-dependent data.
			N := n * ratio

			pb.smallDomain = fft.NewDomain(uint64(n))
			pb.cosetDomains = newCosetDomains(n, ratio)

			// Precompute annihilator inverses: 1/(g^n · ω_ratio^k − 1) for k=0..ratio-1.
			pb.annInv = annihilatorInverses(n, ratio)

			// Precompute cancellation polynomial coset evaluations.
			pb.entries = make([]proverVanishingEntry, len(bkt.vanishings))
			for j, v := range bkt.vanishings {
				pb.entries[j] = proverVanishingEntry{
					v:                 v,
					cancellationCoset: computeCancellationCoset(v.CancelledPositions, n, N),
				}
			}
		}

		result[i] = pb
	}
	return result
}

// computeCancellationCoset returns the base-field evaluation of the
// cancellation polynomial C(X) = Π_{k ∈ cancelled} (X − ω_n^{norm(k)}) at
// all N = n·ratio coset points, in the coset-major layout of [cosetShifts].
// Returns nil when there are no cancelled positions.
func computeCancellationCoset(cancelled []int, n, N int) []field.Element {
	if len(cancelled) == 0 {
		return nil
	}

	// Compute the roots ω_n^{norm(k)} for each cancelled position.
	omega := field.RootOfUnityBy(n)
	roots := make([]field.Element, len(cancelled))
	for i, pos := range cancelled {
		k := pos
		if k < 0 {
			k = n + pos
		}
		field.ExpToInt(&roots[i], omega, k)
	}

	// Evaluate the product at every coset point. Points are independent, so
	// each worker starts from its own point and walks the layout.
	shifts := cosetShifts(n, N/n)
	cVals := make([]field.Element, N)
	parallel.Execute(N, func(start, end int) {
		pt := newCosetWalk(shifts, n, start)
		for t := start; t < end; t++ {
			var prod field.Element
			prod.SetOne()
			for _, root := range roots {
				var diff field.Element
				diff.Sub(&pt.x, &root)
				prod.Mul(&prod, &diff)
			}
			cVals[t] = prod
			pt.next()
		}
	})
	return cVals
}

// cosetWalk enumerates the points x_t of the coset-major layout of
// [cosetShifts] from a given t, one field multiplication per step.
type cosetWalk struct {
	shifts []field.Element
	omega  field.Element // ω_n
	n      int
	k, i   int // x = shifts[k]·ω_n^i
	x      field.Element
}

func newCosetWalk(shifts []field.Element, n, t int) *cosetWalk {
	w := &cosetWalk{shifts: shifts, omega: field.RootOfUnityBy(n), n: n, k: t / n, i: t % n}
	if w.k < len(shifts) {
		field.ExpToInt(&w.x, w.omega, w.i)
		w.x.Mul(&w.x, &shifts[w.k])
	}
	return w
}

func (w *cosetWalk) next() {
	w.i++
	if w.i < w.n {
		w.x.Mul(&w.x, &w.omega)
		return
	}
	w.i = 0
	w.k++
	if w.k < len(w.shifts) {
		w.x = w.shifts[w.k]
	}
}

// cancellationCosets memoises [computeCancellationCoset] per set of cancelled
// positions for one (n, N): the vanishings of a bucket mostly cancel the same
// few rows, and each coset is N elements long. The cosets are only read.
type cancellationCosets struct {
	n, N  int
	byKey map[string][]field.Element
}

func newCancellationCosets(n, N int) *cancellationCosets {
	return &cancellationCosets{n: n, N: N, byKey: make(map[string][]field.Element)}
}

func (c *cancellationCosets) get(cancelled []int) []field.Element {
	if len(cancelled) == 0 {
		return nil
	}
	key := fmt.Sprint(cancelled)
	if v, ok := c.byKey[key]; ok {
		return v
	}
	v := computeCancellationCoset(cancelled, c.n, c.N)
	c.byKey[key] = v
	return v
}

// computeLagrangeSelectorCoset returns the base-field evaluation of the
// Lagrange selector polynomial
//
//	L_p(X) = ω^p · (X^n − 1) / (n · (X − ω^p))
//
// at all N = n·ratio coset points, in the coset-major layout of
// [cosetShifts], where ω = RootOfUnityBy(n) and p is `position` normalised
// into [0, n). This is the polynomial that [wiop.LagrangeSelector] represents
// (1 at row p, 0 elsewhere on the domain); the same closed form is used
// pointwise by [wiop.LagrangeSelector.EvaluateOutOfDomain].
//
// The denominator never vanishes: a coset point is never a pure n-th root of
// unity, so X − ω^p ≠ 0.
func computeLagrangeSelectorCoset(position, n, N int) []field.Element {
	// Normalise the anchor into [0, n).
	p := ((position % n) + n) % n

	omega := field.RootOfUnityBy(n)
	var omegaP field.Element
	field.ExpToInt(&omegaP, omega, p)

	// Constant numerator coefficient ω^p / n.
	var nInv field.Element
	nInv.SetUint64(uint64(n))
	nInv.Inverse(&nInv)
	var numCoef field.Element
	numCoef.Mul(&omegaP, &nInv)

	// x^n − 1 is constant on each small coset: x^n = s_k^n.
	shifts := cosetShifts(n, N/n)
	num := make([]field.Element, len(shifts))
	var one field.Element
	one.SetOne()
	for k := range shifts {
		field.ExpToInt(&num[k], shifts[k], n)
		num[k].Sub(&num[k], &one)
		num[k].Mul(&num[k], &numCoef)
	}

	// Points are independent, so each worker starts from its own point and
	// inverts its own chunk of denominators.
	res := make([]field.Element, N)
	parallel.Execute(N, func(start, end int) {
		m := end - start
		denom := make([]field.Element, m) // x_t − ω^p
		pt := newCosetWalk(shifts, n, start)
		for t := range m {
			denom[t].Sub(&pt.x, &omegaP)
			pt.next()
		}

		invDenom := make([]field.Element, m)
		field.VecBatchInvBase(invDenom, denom)

		for t := range m {
			res[start+t].Mul(&num[(start+t)/n], &invDenom[t])
		}
	})
	return res
}

// collectLagrangeSelectorPositions records the Position of every
// [wiop.LagrangeSelector] leaf reachable from expr into out.
func collectLagrangeSelectorPositions(expr wiop.Expression, out map[int]struct{}) {
	switch e := expr.(type) {
	case *wiop.LagrangeSelector:
		out[e.Position] = struct{}{}
	case *wiop.ArithmeticOperation:
		for _, op := range e.Operands {
			collectLagrangeSelectorPositions(op, out)
		}
	}
}

// bktVanishings returns the Vanishing constraints of a bucket, regardless of
// whether it is a static bucket (size-dependent data precomputed into entries)
// or a dynamic bucket (raw vanishings deferred to runtime).
func bktVanishings(bkt *proverBucket) []*wiop.Vanishing {
	if bkt.entries != nil {
		vs := make([]*wiop.Vanishing, len(bkt.entries))
		for i, e := range bkt.entries {
			vs[i] = e.v
		}
		return vs
	}
	return bkt.vanishings
}

// ---------------------------------------------------------------------------
// Prover actions
// ---------------------------------------------------------------------------

// QuotientProverAction computes the quotient share columns for all ratio
// buckets of a single module. It runs in quotientRound.
type QuotientProverAction struct {
	m         *wiop.Module
	mergeCoin *wiop.CoinField
	buckets   []proverBucket
}

// Plan pre-allocates scratch buffers for each ratio bucket from the planning
// arena. For static modules, Run uses these slices instead of allocating fresh
// memory on every invocation. For dynamic modules, this is a no-op since the
// size isn't known until runtime.
func (a *QuotientProverAction) Plan(ctx *wiop.PlanningContext) {
	if a.m.IsDynamic() {
		return // Size not known at plan time for dynamic modules.
	}
	n := a.m.Size()
	for i := range a.buckets {
		bkt := &a.buckets[i]
		N := n * bkt.ratio
		bkt.scratchAgg = ctx.AllocExt(N)
	}
}

// Run executes the quotient polynomial computation and assigns quotient share columns.
// For static modules, uses precomputed domains and scratch buffers. For dynamic
// modules, computes size-dependent data at runtime using RuntimeSize.
func (a *QuotientProverAction) Run(rt *wiop.Runtime) {
	n := a.m.RuntimeSize(rt)

	if !a.m.IsDynamic() && n != a.m.Size() {
		panic(fmt.Sprintf(
			"wiop/compilers: global quotient prover action called with runtime size %d but module size is %d",
			n,
			a.m.Size(),
		))
	}
	coinExt := rt.GetCoinValue(a.mergeCoin).Ext

	for _, bkt := range a.buckets {
		ratio := bkt.ratio
		N := n * ratio

		// Get or compute FFT domains and annihilator inverses. The large coset
		// g·H_N is handled as the union of its ratio small cosets s_k·H_n (see
		// [cosetShifts]): every coset table below is laid out coset-major, point
		// k·n + i being s_k·ω_n^i.
		var (
			smallDomain  *fft.Domain
			cosetDomains []*fft.Domain
			annInv       []field.Element
		)
		if bkt.smallDomain != nil {
			// Static module: use precomputed values.
			smallDomain, cosetDomains, annInv = bkt.smallDomain, bkt.cosetDomains, bkt.annInv
		} else {
			// Dynamic module: compute at runtime. The domains depend only on
			// the size and the shift, so gnark's process-wide domain cache
			// serves them across buckets and proofs instead of rebuilding the
			// twiddles.
			smallDomain = fft.NewDomain(uint64(n), fft.WithCache())
			cosetDomains = newCosetDomains(n, ratio)
			annInv = annihilatorInverses(n, ratio)
		}

		// --- Evaluate all root columns on the cosets ---
		// cosetEvals[colID] holds a base-field column's evaluations,
		// cosetEvalsExt[colID] an extension-field column's. A column populates
		// exactly one of the two maps; expression evaluators dispatch on
		// Column.IsExtension.
		cosetEvals, cosetEvalsExt := evalColumnsOnCosets(rt, a.m, bkt.rootCols, smallDomain, cosetDomains)

		// --- Evaluate every distinct Lagrange selector on the cosets ---
		// Selectors are not committed columns, so they are computed analytically
		// rather than re-FFT'd. selectorCosets[position][t] = L_position(x_t).
		selectorPositions := make(map[int]struct{})
		for _, v := range bktVanishings(&bkt) {
			collectLagrangeSelectorPositions(v.Expression, selectorPositions)
		}
		selectorCosets := make(map[int][]field.Element, len(selectorPositions))
		for pos := range selectorPositions {
			selectorCosets[pos] = computeLagrangeSelectorCoset(pos, n, N)
		}

		// --- Compute the aggregate extension-field polynomial on the cosets ---
		// aggregate[t] = Σ_i coin^i · P_i(x_t) · C_i(x_t)
		//
		// Reuse scratchAgg if Plan was called; it may contain stale data from
		// the previous proof run, so clear it before use as an accumulator.
		aggregate := bkt.scratchAgg
		if len(aggregate) < N {
			aggregate = make([]field.Ext, N)
		} else {
			clear(aggregate[:N])
		}

		// Bind every vanishing expression once: leaves resolve to their coset
		// slices and runtime scalars up front, so the evaluation below involves
		// no map lookups and no runtime access. This is what makes the parallel
		// workers free of shared-state reads (and of the runtime mutex).
		var coinPow field.Ext
		coinPow.SetOne()
		var bound []boundEntry
		if bkt.entries != nil {
			// Static module: use precomputed cancellation cosets.
			for _, entry := range bkt.entries {
				bound = append(bound, boundEntry{
					expr:         bindExpr(rt, entry.v.Expression, cosetEvals, cosetEvalsExt, selectorCosets, n),
					cancellation: entry.cancellationCoset,
					coinPow:      coinPow,
				})
				// advance coinPow: coinPow *= coinExt
				coinPow.Mul(&coinPow, &coinExt)
			}
		} else {
			// Dynamic module: compute cancellation cosets at runtime, once per
			// distinct set of cancelled positions in the bucket.
			cosets := newCancellationCosets(n, N)
			for _, v := range bkt.vanishings {
				bound = append(bound, boundEntry{
					expr:         bindExpr(rt, v.Expression, cosetEvals, cosetEvalsExt, selectorCosets, n),
					cancellation: cosets.get(v.CancelledPositions),
					coinPow:      coinPow,
				})
				coinPow.Mul(&coinPow, &coinExt)
			}
		}

		// The bound entries are lowered into one program (shared subexpressions,
		// folded scalars and linear forms; see [compileQuotientProgram]) that
		// runs over blocks of coset points in parallel, then divides by the
		// annihilator (x^n − 1), whose inverse on coset k is annInv[k].
		compileQuotientProgram(bound, n, ratio).run(aggregate[:N], annInv)

		// --- Interpolate the quotient into its shares ---
		for k, share := range cosetsToShares(aggregate[:N], smallDomain, cosetDomains) {
			rt.AssignColumn(bkt.shares[k], &wiop.ConcreteVector{Plain: field.VecFromExt(share)})
		}
	}
}

// cosetShifts returns the shifts s_k = g·ω_N^k, k < ratio, of the small cosets
// s_k·H_n whose union is the large coset g·H_N, N = n·ratio, where g is the
// multiplicative generator. The quotient handles g·H_N one small coset at a
// time: a coset table is laid out coset-major, entry t = k·n + i holding the
// value at x_t = s_k·ω_n^i (the large-coset point of index i·ratio + k).
//
// The layout turns every large-coset FFT into ratio independent size-n coset
// FFTs, so nothing is zero-padded or bit-reversed at size N; a column shift
// by s rows stays inside each small coset (x_t·ω_n^s); and x_t^n = s_k^n only
// depends on the coset, so the annihilator is constant per coset.
func cosetShifts(n, ratio int) []field.Element {
	var g field.Element
	g.SetUint64(field.MultiplicativeGen)
	omegaN := field.RootOfUnityBy(n * ratio)
	shifts := make([]field.Element, ratio)
	shifts[0] = g
	for k := 1; k < ratio; k++ {
		shifts[k].Mul(&shifts[k-1], &omegaN)
	}
	return shifts
}

// newCosetDomains returns the size-n FFT domains shifted to each small coset
// of [cosetShifts].
func newCosetDomains(n, ratio int) []*fft.Domain {
	shifts := cosetShifts(n, ratio)
	domains := make([]*fft.Domain, ratio)
	for k := range shifts {
		domains[k] = fft.NewDomain(uint64(n), fft.WithShift(shifts[k]), fft.WithCache())
	}
	return domains
}

// annihilatorInverses returns 1/(x^n − 1) on each small coset of
// [cosetShifts], where x^n = s_k^n = g^n·ω_ratio^k is constant.
func annihilatorInverses(n, ratio int) []field.Element {
	annInv := make([]field.Element, ratio)
	field.VecBatchInvBase(annInv, polynomials.EvalXnMinusOneOnCoset(n, n*ratio))
	return annInv
}

// evalColumnsOnCosets evaluates every column of cols on the small cosets of
// cosetDomains, in the coset-major layout of [cosetShifts]. A column is
// interpolated once on H_n, its coefficients being shared by every coset,
// then evaluated by one size-n coset FFT per coset, written straight into the
// coset's contiguous slot. Both steps are spread over the CPUs: columns, then
// (column, coset) pairs, so a bucket with few tall columns keeps every CPU
// busy; the FFTs share out what is left of the CPUs, at least one each.
func evalColumnsOnCosets(
	rt *wiop.Runtime,
	m *wiop.Module,
	cols []*wiop.Column,
	smallDomain *fft.Domain,
	cosetDomains []*fft.Domain,
) (map[wiop.ObjectID][]field.Element, map[wiop.ObjectID][]field.Ext) {
	var (
		n         = int(smallDomain.Cardinality)
		ratio     = len(cosetDomains)
		cpus      = runtime.GOMAXPROCS(0)
		baseEvals = make([][]field.Element, len(cols))
		extEvals  = make([][]field.Ext, len(cols))
	)

	// Interpolation: the coefficients land, bit-reversed, in coset 0's slot.
	tasks := max(1, cpus/max(1, len(cols)))
	parallel.ExecuteDynamic(len(cols), func(c int) {
		col := cols[c]
		cv := rt.GetColumnAssignment(col)
		if col.IsExtension {
			vals := make([]field.Ext, n*ratio)
			writeColumnExt(cv, m.Padding, vals[:n])
			smallDomain.FFTInverseExt6(vals[:n], fft.DIF, fft.WithNbTasks(tasks))
			extEvals[c] = vals
			return
		}
		if !cv.Plain.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: global quotient does not support extension-field data in base column %q",
				col.Context.Path(),
			))
		}
		vals := make([]field.Element, n*ratio)
		writeColumnBase(cv, m.Padding, vals[:n])
		smallDomain.FFTInverse(vals[:n], fft.DIF, fft.WithNbTasks(tasks))
		baseEvals[c] = vals
	})

	// Evaluation: coset k > 0 copies the coefficients from coset 0's slot
	// before coset 0 is evaluated in place, in a second pass. FFT(DIT) takes
	// the bit-reversed coefficients and returns the natural order.
	evalOn := func(c, k, tasks int) {
		if v := extEvals[c]; v != nil {
			dst := v[k*n : (k+1)*n]
			if k > 0 {
				copy(dst, v[:n])
			}
			cosetDomains[k].FFTExt6(dst, fft.DIT, fft.OnCoset(), fft.WithNbTasks(tasks))
			return
		}
		v := baseEvals[c]
		dst := v[k*n : (k+1)*n]
		if k > 0 {
			copy(dst, v[:n])
		}
		cosetDomains[k].FFT(dst, fft.DIT, fft.OnCoset(), fft.WithNbTasks(tasks))
	}
	type item struct{ c, k int }
	items := make([]item, 0, len(cols)*(ratio-1))
	for c := range cols {
		for k := 1; k < ratio; k++ {
			items = append(items, item{c, k})
		}
	}
	tasks = max(1, cpus/max(1, len(items)))
	parallel.ExecuteDynamic(len(items), func(i int) { evalOn(items[i].c, items[i].k, tasks) })
	tasks = max(1, cpus/max(1, len(cols)))
	parallel.ExecuteDynamic(len(cols), func(c int) { evalOn(c, 0, tasks) })

	cosetEvals := make(map[wiop.ObjectID][]field.Element, len(cols))
	cosetEvalsExt := make(map[wiop.ObjectID][]field.Ext, len(cols))
	for i, col := range cols {
		if col.IsExtension {
			cosetEvalsExt[col.Context.ID] = extEvals[i]
		} else {
			cosetEvals[col.Context.ID] = baseEvals[i]
		}
	}
	return cosetEvals, cosetEvalsExt
}

// writeColumnBase writes cv down as its len(out) rows on H_n, padded as the
// module pads (see [wiop.ConcreteVector.ElementAtN]). The data must be
// base-field.
func writeColumnBase(cv *wiop.ConcreteVector, padding wiop.PaddingDirection, out []field.Element) {
	plain := cv.Plain.AsBase()
	switch padding {
	case wiop.PaddingDirectionLeft:
		gap := len(out) - len(plain)
		if gap < 0 {
			copy(out, plain[-gap:])
			return
		}
		field.VecFillBase(out[:gap], cv.Padding)
		copy(out[gap:], plain)
	case wiop.PaddingDirectionRight:
		k := copy(out, plain)
		field.VecFillBase(out[k:], cv.Padding)
	default:
		copy(out, plain[:len(out)])
	}
}

// writeColumnExt is [writeColumnBase] into the extension field, lifting
// base-field data and padding.
func writeColumnExt(cv *wiop.ConcreteVector, padding wiop.PaddingDirection, out []field.Ext) {
	n := len(out)
	start, plainLen := 0, cv.Plain.Len() // rows [start, start+plainLen) hold data
	switch padding {
	case wiop.PaddingDirectionLeft:
		start = n - plainLen
		field.VecFillExt(out[:max(start, 0)], field.Lift(cv.Padding))
	case wiop.PaddingDirectionRight:
		field.VecFillExt(out[min(plainLen, n):], field.Lift(cv.Padding))
	default:
		plainLen = n
	}
	skip := max(-start, 0) // a Left plain longer than n keeps its last n rows
	data := out[max(start, 0):min(start+plainLen, n)]
	if cv.Plain.IsBase() {
		for i, v := range cv.Plain.AsBase()[skip : skip+len(data)] {
			data[i] = field.Lift(v)
		}
		return
	}
	copy(data, cv.Plain.AsExt()[skip:skip+len(data)])
}

// cosetsToShares interpolates the quotient Q, given by its evaluations on the
// small cosets in the coset-major layout of [cosetShifts], into its ratio
// shares in Lagrange form on H_n, Q = Σ_m X^{mn}·Q_m (share m holds the
// coefficients α_{mn..mn+n-1} of Q).
//
// With s_k^n = g^n·ω_ratio^k, the size-n interpolation on coset k yields
// β_k[r] = Σ_m α_{mn+r}·(g^n·ω_ratio^k)^m, so α_{mn+r} = g^{-nm}·(1/ratio)·
// Σ_k ω_ratio^{-km}·β_k[r] is a size-ratio inverse DFT across the cosets. The
// interpolations leave r bit-reversed, which is the order each share's final
// FFT(DIT) expects. agg is overwritten.
func cosetsToShares(agg []field.Ext, smallDomain *fft.Domain, cosetDomains []*fft.Domain) [][]field.Ext {
	var (
		n     = int(smallDomain.Cardinality)
		ratio = len(cosetDomains)
		tasks = max(1, runtime.GOMAXPROCS(0)/ratio)
	)
	parallel.Execute(ratio, func(start, end int) {
		for k := start; k < end; k++ {
			cosetDomains[k].FFTInverseExt6(agg[k*n:(k+1)*n], fft.DIF, fft.OnCoset(), fft.WithNbTasks(tasks))
		}
	}, ratio)

	// coefs[m][k] = g^{-nm}·ω_ratio^{-km}/ratio, with ω_ratio = ω_N^n.
	var g, gInvN, omegaInv, ratioInv field.Element
	g.SetUint64(field.MultiplicativeGen)
	field.ExpToInt(&gInvN, g, n)
	gInvN.Inverse(&gInvN)
	field.ExpToInt(&omegaInv, field.RootOfUnityBy(n*ratio), n)
	omegaInv.Inverse(&omegaInv)
	ratioInv.SetUint64(uint64(ratio))
	ratioInv.Inverse(&ratioInv)
	coefs := make([][]field.Element, ratio)
	var rowScale field.Element // g^{-nm}/ratio
	rowScale.Set(&ratioInv)
	for m := range coefs {
		coefs[m] = make([]field.Element, ratio)
		var step field.Element // ω_ratio^{-m}
		field.ExpToInt(&step, omegaInv, m)
		coefs[m][0] = rowScale
		for k := 1; k < ratio; k++ {
			coefs[m][k].Mul(&coefs[m][k-1], &step)
		}
		rowScale.Mul(&rowScale, &gInvN)
	}

	shares := make([][]field.Ext, ratio)
	parallel.Execute(ratio, func(start, end int) {
		for m := start; m < end; m++ {
			shares[m] = make([]field.Ext, n)
		}
	}, ratio)
	parallel.Execute(n, func(start, end int) {
		for m, share := range shares {
			for p := start; p < end; p++ {
				var acc, term field.Ext
				for k := range ratio {
					term.MulByElement(&agg[k*n+p], &coefs[m][k])
					acc.Add(&acc, &term)
				}
				share[p] = acc
			}
		}
	})

	parallel.Execute(ratio, func(start, end int) {
		for m := start; m < end; m++ {
			smallDomain.FFTExt6(shares[m], fft.DIT, fft.WithNbTasks(tasks))
		}
	}, ratio)
	return shares
}

// EvalProverAction self-assigns all LagrangeEval queries for a module.
// It runs in evalRound.
type EvalProverAction struct {
	lagrangeEvals []*wiop.LagrangeEval
}

// Run self-assigns all LagrangeEval queries registered for this module.
func (a *EvalProverAction) Run(rt *wiop.Runtime) {
	for _, le := range a.lagrangeEvals {
		le.SelfAssign(rt)
	}
}

// ---------------------------------------------------------------------------
// Verifier action
// ---------------------------------------------------------------------------

// Verifier checks the PLONK quotient identity for one module.
// It runs in evalRound.
type Verifier struct {
	Module        *wiop.Module
	MergeCoin     *wiop.CoinField
	EvalCoin      *wiop.CoinField
	WitnessViews  []*wiop.ColumnView
	WitnessClaims []*wiop.Cell
	viewKeyToIdx  map[colViewKey]int
	Buckets       []VerifierBucket
}

// Check verifies the PLONK quotient identity for the module using the runtime's claimed values.
func (gv *Verifier) Check(rt *wiop.Runtime) error {
	n := gv.Module.RuntimeSize(rt)

	if !gv.Module.IsDynamic() && n != gv.Module.Size() {
		panic(fmt.Sprintf(
			"wiop/compilers: global quotient Check called with runtime size %d but module size is %d",
			n,
			gv.Module.Size(),
		))
	}
	r := rt.GetCoinValue(gv.EvalCoin)
	coinExt := rt.GetCoinValue(gv.MergeCoin).Ext

	// Build the map from column-view key → evaluation at r.
	viewEvals := make(map[colViewKey]field.Gen, len(gv.WitnessViews))
	for i, cv := range gv.WitnessViews {
		key := colViewKey{id: cv.Column.Context.ID, shift: cv.ShiftingOffset}
		viewEvals[key] = rt.GetCellValue(gv.WitnessClaims[i])
	}

	// Compute annihilator r^n − 1.
	annihilator := computeAnnihilator(r, n)

	for _, bkt := range gv.Buckets {
		// --- Recombine quotient shares: Q(r) = Σ_k r^{kn} · Q_k(r) ---
		qr := field.ElemZero()
		rPowKN := field.ElemOne() // r^{kn}, starts at r^0 = 1
		for k, claim := range bkt.QuotientClaims {
			_ = k
			qk := rt.GetCellValue(claim) // Q_k(r)
			qr = qr.Add(rPowKN.Mul(qk))
			// Advance: r^{(k+1)n} = r^{kn} · r^n
			rPowN := computeAnnihilator(r, n) // r^n − 1 + 1 = r^n ... just compute r^n
			rPowN = rPowN.Add(field.ElemOne())
			rPowKN = rPowKN.Mul(rPowN)
		}

		// --- Compute P_agg(r) = Σ_i coin^i · P_i(r) · C_i(r) ---
		pagg := field.ElemZero()
		var coinPow field.Ext
		coinPow.SetOne()
		for _, v := range bkt.Vanishings {
			pr := evalExprAtPoint(v.Expression, viewEvals, r, rt)
			cr := evalCancellationAtPoint(v.CancelledPositions, n, r)
			pTimesC := pr.Mul(cr)
			// coinPow · pTimesC  (coinPow is Ext, pTimesC may be base or ext)
			var term field.Ext
			if pTimesC.IsBase() {
				pBase := pTimesC.AsBase()
				term.MulByElement(&coinPow, &pBase)
			} else {
				pExt := pTimesC.AsExt()
				term.Mul(&coinPow, &pExt)
			}
			pagg = pagg.Add(field.ElemFromExt(term))
			coinPow.Mul(&coinPow, &coinExt)
		}

		// --- Check: P_agg(r) = annihilator · Q(r) ---
		lhs := pagg
		rhs := annihilator.Mul(qr)
		diff := lhs.Sub(rhs)
		if !diff.IsZero() {
			return fmt.Errorf(
				"wiop/compilers: global quotient check failed for module (n=%d, ratio=%d): P_agg(r) ≠ (r^n−1)·Q(r)",
				n, bkt.Ratio,
			)
		}
	}
	return nil
}

// computeAnnihilator computes r^n − 1.
func computeAnnihilator(r field.Gen, n int) field.Gen {
	return expFieldElem(r, n).Sub(field.ElemOne())
}

// expFieldElem computes base^exp using binary exponentiation.
func expFieldElem(base field.Gen, exp int) field.Gen {
	result := field.ElemOne()
	b := base
	for exp > 0 {
		if exp&1 == 1 {
			result = result.Mul(b)
		}
		b = b.Square()
		exp >>= 1
	}
	return result
}

// evalCancellationAtPoint evaluates C(r) = Π_{k ∈ cancelled} (r − ω_n^{norm(k)}).
func evalCancellationAtPoint(cancelled []int, n int, r field.Gen) field.Gen {
	if len(cancelled) == 0 {
		return field.ElemOne()
	}
	omega := field.RootOfUnityBy(n)
	result := field.ElemOne()
	for _, pos := range cancelled {
		k := pos
		if k < 0 {
			k = n + pos
		}
		var omegaK field.Element
		field.ExpToInt(&omegaK, omega, k)
		factor := r.Sub(field.ElemFromBase(omegaK))
		result = result.Mul(factor)
	}
	return result
}

// evalExprAtPoint evaluates a symbolic expression at the point r using the
// witness column evaluation map (from LagrangeEval claim cells). Coins and
// cells are looked up directly from the runtime.
func evalExprAtPoint(
	expr wiop.Expression,
	viewEvals map[colViewKey]field.Gen,
	r field.Gen,
	rt *wiop.Runtime,
) field.Gen {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		key := colViewKey{id: e.Column.Context.ID, shift: e.ShiftingOffset}
		v, ok := viewEvals[key]
		if !ok {
			panic(fmt.Sprintf(
				"wiop/compilers: ColumnView (%v, shift=%d) not in witness eval map",
				e.Column.Context.ID, e.ShiftingOffset,
			))
		}
		return v
	case *wiop.LagrangeSelector:
		return e.EvaluateOutOfDomain(rt, r)
	case *wiop.ArithmeticOperation:
		eval := func(i int) field.Gen {
			return evalExprAtPoint(e.Operands[i], viewEvals, r, rt)
		}
		a0 := eval(0)
		switch e.Operator {
		case wiop.ArithmeticOperatorAdd:
			return a0.Add(eval(1))
		case wiop.ArithmeticOperatorSub:
			return a0.Sub(eval(1))
		case wiop.ArithmeticOperatorMul:
			return a0.Mul(eval(1))
		case wiop.ArithmeticOperatorDiv:
			return a0.Div(eval(1))
		case wiop.ArithmeticOperatorDouble:
			return a0.Add(a0)
		case wiop.ArithmeticOperatorSquare:
			return a0.Square()
		case wiop.ArithmeticOperatorNegate:
			return a0.Neg()
		case wiop.ArithmeticOperatorInverse:
			return a0.Inverse()
		default:
			panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", e.Operator))
		}
	case *wiop.Constant:
		return field.ElemFromBase(e.Value)
	case *wiop.CoinField:
		return rt.GetCoinValue(e)
	case *wiop.Cell:
		return rt.GetCellValue(e)
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in evalExprAtPoint", expr))
	}
}

// boundKind discriminates the node types of a [boundExpr].
type boundKind uint8

const (
	boundOp         boundKind = iota // arithmetic node
	boundVecBase                     // base-field column or selector coset evaluations
	boundVecExt                      // extension-field column coset evaluations
	boundScalarBase                  // constant or base cell, invariant across coset points
	boundScalarExt                   // extension cell or coin, invariant across coset points
)

// boundExpr is a Vanishing expression specialised against one bucket's coset
// tables: every leaf holds a direct slice or a resolved scalar, so the
// [quotientProgram] lowered from it involves no map lookups and no runtime
// access. Binding happens once per (bucket, expression).
//
// isBase reports whether the subtree evaluates in the base field: extension
// cells, extension column views, and coins make a subtree extension. Base
// subtrees are evaluated in base-field arithmetic and only meet the extension
// field at their boundary.
type boundExpr struct {
	kind       boundKind
	isBase     bool
	operator   wiop.ArithmeticOperator
	operands   []boundExpr
	vecBase    []field.Element // boundVecBase: length-N coset evaluations
	vecExt     []field.Ext     // boundVecExt: length-N coset evaluations
	offset     int             // boundVec*: column shift within a small coset, in [0, n)
	scalarBase field.Element   // boundScalarBase
	scalarExt  field.Ext       // boundScalarExt
}

// bindExpr resolves every leaf of expr against the runtime and the bucket's
// coset tables. A ColumnView with shift s reads the point x_t·ω_n^s, which is
// in the same small coset as x_t (see [cosetShifts]): offset is s mod n, the
// shift within a coset of n points.
func bindExpr(
	rt *wiop.Runtime,
	expr wiop.Expression,
	cosetEvals map[wiop.ObjectID][]field.Element,
	cosetEvalsExt map[wiop.ObjectID][]field.Ext,
	selectorCosets map[int][]field.Element,
	n int,
) boundExpr {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		offset := ((e.ShiftingOffset % n) + n) % n
		if e.Column.IsExtension {
			return boundExpr{kind: boundVecExt, vecExt: cosetEvalsExt[e.Column.Context.ID], offset: offset}
		}
		return boundExpr{kind: boundVecBase, isBase: true, vecBase: cosetEvals[e.Column.Context.ID], offset: offset}
	case *wiop.LagrangeSelector:
		// Selectors are base-field and unshifted.
		return boundExpr{kind: boundVecBase, isBase: true, vecBase: selectorCosets[e.Position]}
	case *wiop.Constant:
		return boundExpr{kind: boundScalarBase, isBase: true, scalarBase: e.Value}
	case *wiop.Cell:
		v := rt.GetCellValue(e)
		if e.IsExtension() {
			return boundExpr{kind: boundScalarExt, scalarExt: v.AsExt()}
		}
		if !v.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: cell %q declared as base but holds an extension-field value",
				e.Context.Path(),
			))
		}
		return boundExpr{kind: boundScalarBase, isBase: true, scalarBase: v.AsBase()}
	case *wiop.CoinField:
		return boundExpr{kind: boundScalarExt, scalarExt: rt.GetCoinValue(e).AsExt()}
	case *wiop.ArithmeticOperation:
		operands := make([]boundExpr, len(e.Operands))
		isBase := true
		for i, op := range e.Operands {
			operands[i] = bindExpr(rt, op, cosetEvals, cosetEvalsExt, selectorCosets, n)
			isBase = isBase && operands[i].isBase
		}
		return boundExpr{kind: boundOp, isBase: isBase, operator: e.Operator, operands: operands}
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in bindExpr", expr))
	}
}

// boundEntry pairs one bound Vanishing expression with its cancellation coset
// and its merging-coin power, ready for accumulation.
type boundEntry struct {
	expr         boundExpr
	cancellation []field.Element // nil when the constraint has no cancelled positions
	coinPow      field.Ext       // coin^i for the i-th constraint of the bucket
}

// ---------------------------------------------------------------------------
// Expression tree traversal helpers
// ---------------------------------------------------------------------------

// collectColumnViews recursively collects all *ColumnView leaves in expr.
func collectColumnViews(expr wiop.Expression) []*wiop.ColumnView {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		return []*wiop.ColumnView{e}
	case *wiop.ArithmeticOperation:
		var result []*wiop.ColumnView
		for _, op := range e.Operands {
			result = append(result, collectColumnViews(op)...)
		}
		return result
	default:
		return nil
	}
}

// collectRootColumns recursively collects all unique root *Column objects
// referenced by expr (deduplication by ObjectID).
func collectRootColumns(expr wiop.Expression) []*wiop.Column {
	views := collectColumnViews(expr)
	seen := make(map[wiop.ObjectID]*wiop.Column, len(views))
	for _, cv := range views {
		id := cv.Column.Context.ID
		if _, ok := seen[id]; !ok {
			seen[id] = cv.Column
		}
	}
	result := make([]*wiop.Column, 0, len(seen))
	for _, col := range seen {
		result = append(result, col)
	}
	return result
}

// ---------------------------------------------------------------------------
// Ratio computation
// ---------------------------------------------------------------------------

// computeRatio returns the smallest power of two ratio such that the quotient
// polynomial fits within ratio shares. The ratio is computed from the
// expression's DegreeFactor() which doesn't require knowing the module size,
// allowing compilation to proceed for dynamic-size modules.
//
// For a vanishing constraint with expression degree d = degreeFactor * (n-1)
// and c cancelled positions, the numerator polynomial has degree at most
// d + c = degreeFactor * (n-1) + c. Dividing by the annihilator (x^n - 1)
// gives a quotient of degree at most:
//
//	quotientDeg = degreeFactor * (n-1) + c - n +1
//	            = (degreeFactor - 1) * n + (c - degreeFactor + 1)
//
// For this to fit in ratio shares of size n (i.e., degree < ratio * n), we need:
//
//	ratio * n > quotientDeg
//	ratio > (degreeFactor - 1) + (c - degreeFactor +1) / n
func computeRatio(v *wiop.Vanishing) int {
	factor := v.Expression.DegreeFactor()
	// usually n > c, n >factor, so if c-factor+1> 0 ratio= factor, otherwise ratio= factor-1.
	// We use
	// max(1, ratio) since ratio must be at least 1.
	if len(v.CancelledPositions)-factor+1 > 0 {
		return utils.NextPowerOfTwo(max(1, factor))
	}
	return utils.NextPowerOfTwo(max(1, factor-1))
}

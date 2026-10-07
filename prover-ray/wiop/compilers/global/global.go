package global

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gnarkutils "github.com/consensys/gnark-crypto/utils"
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
	entries     []proverVanishingEntry // precomputed cancellation cosets
	smallDomain *fft.Domain            // FFT domain of size n
	largeDomain *fft.Domain            // FFT domain of size n*ratio
	annInv      []field.Element        // 1/(g^n · ω_ratio^j − 1) for j = 0..ratio-1

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
			pb.largeDomain = fft.NewDomain(uint64(N))

			// Precompute annihilator inverses: 1/(g^n · ω_ratio^j − 1) for j=0..ratio-1.
			annVals := polynomials.EvalXnMinusOneOnCoset(n, N)
			pb.annInv = make([]field.Element, ratio)
			field.VecBatchInvBase(pb.annInv, annVals)

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
// all N = n·ratio coset points {g · ω_N^j : j = 0…N-1}. Returns nil when
// there are no cancelled positions.
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

	// Iterate over coset points and evaluate the product.
	omegaN := field.RootOfUnityBy(N)
	var g field.Element
	g.SetUint64(field.MultiplicativeGen)

	cVals := make([]field.Element, N)
	x := g // x = g · ω_N^0 = g
	for j := 0; j < N; j++ {
		var prod field.Element
		prod.SetOne()
		for _, root := range roots {
			var diff field.Element
			diff.Sub(&x, &root)
			prod.Mul(&prod, &diff)
		}
		cVals[j] = prod
		x.Mul(&x, &omegaN)
	}
	return cVals
}

// computeLagrangeSelectorCoset returns the base-field evaluation of the
// Lagrange selector polynomial
//
//	L_p(X) = ω^p · (X^n − 1) / (n · (X − ω^p))
//
// at all N = n·ratio coset points {g · ω_N^j : j = 0…N-1}, where ω =
// RootOfUnityBy(n), ω_N = RootOfUnityBy(N), g = MultiplicativeGen, and p is
// `position` normalised into [0, n). This is the polynomial that
// [wiop.LagrangeSelector] represents (1 at row p, 0 elsewhere on the domain);
// the same closed form is used pointwise by
// [wiop.LagrangeSelector.EvaluateOutOfDomain].
//
// The coset parametrisation mirrors [computeCancellationCoset] exactly so the
// result lines up with the FFT-coset evaluations of the witness columns. The
// denominator never vanishes: a coset point g·ω_N^j is never a pure n-th root
// of unity, so X − ω^p ≠ 0.
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

	omegaN := field.RootOfUnityBy(N)
	var g field.Element
	g.SetUint64(field.MultiplicativeGen)

	// x_j^n = (g·ω_N^j)^n = g^n · (ω_N^n)^j cycles with period ratio = N/n,
	// advanced incrementally to avoid a per-point exponentiation.
	var gPowN, omegaNPowN field.Element
	field.ExpToInt(&gPowN, g, n)           // g^n
	field.ExpToInt(&omegaNPowN, omegaN, n) // ω_N^n
	var one field.Element
	one.SetOne()

	denom := make([]field.Element, N) // x_j − ω^p
	num := make([]field.Element, N)   // x_j^n − 1
	x := g
	xPowN := gPowN
	for j := 0; j < N; j++ {
		denom[j].Sub(&x, &omegaP)
		num[j].Sub(&xPowN, &one)
		x.Mul(&x, &omegaN)
		xPowN.Mul(&xPowN, &omegaNPowN)
	}

	invDenom := make([]field.Element, N)
	field.VecBatchInvBase(invDenom, denom)

	res := make([]field.Element, N)
	for j := 0; j < N; j++ {
		res[j].Mul(&numCoef, &num[j])
		res[j].Mul(&res[j], &invDenom[j])
	}
	return res
}

// collectLagrangeSelectorPositions records the Position of every
// [wiop.LagrangeSelector] leaf reachable from expr into out.
func collectLagrangeSelectorPositions(expr wiop.Expression, out map[int]struct{}) {
	walkLeaves(expr, func(leaf wiop.Expression) {
		if ls, ok := leaf.(*wiop.LagrangeSelector); ok {
			out[ls.Position] = struct{}{}
		}
	})
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

		// Get or compute FFT domains and annihilator inverses.
		var smallDomain, largeDomain *fft.Domain
		var annInv []field.Element

		if bkt.smallDomain != nil {
			// Static module: use precomputed values.
			smallDomain = bkt.smallDomain
			largeDomain = bkt.largeDomain
			annInv = bkt.annInv
		} else {
			// Dynamic module: compute at runtime.
			smallDomain = fft.NewDomain(uint64(n))
			largeDomain = fft.NewDomain(uint64(N))
			annVals := polynomials.EvalXnMinusOneOnCoset(n, N)
			annInv = make([]field.Element, ratio)
			field.VecBatchInvBase(annInv, annVals)
		}

		// --- Evaluate all root columns on the large coset ---
		// cosetEvals[colID][j] = col evaluated at coset point j (base-field
		// columns); cosetEvalsExt[colID][j] for extension-field columns.
		// A column populates exactly one of the two maps; expression
		// evaluators dispatch on Column.IsExtension.
		cosetEvals := make(map[wiop.ObjectID][]field.Element, len(bkt.rootCols))
		cosetEvalsExt := make(map[wiop.ObjectID][]field.Ext, len(bkt.rootCols))
		for _, col := range bkt.rootCols {
			if col.IsExtension {
				cosetEvalsExt[col.Context.ID] = reevalOnLargeCosetExt(
					rt, col, a.m, n, N, smallDomain, largeDomain,
				)
			} else {
				cosetEvals[col.Context.ID] = reevalOnLargeCoset(
					rt, col, a.m, n, N, smallDomain, largeDomain,
				)
			}
		}

		// --- Evaluate every distinct Lagrange selector on the large coset ---
		// Selectors are not committed columns, so they are computed analytically
		// rather than re-FFT'd. selectorCosets[position][j] = L_position(coset_j).
		selectorPositions := make(map[int]struct{})
		for _, v := range bktVanishings(&bkt) {
			collectLagrangeSelectorPositions(v.Expression, selectorPositions)
		}
		selectorCosets := make(map[int][]field.Element, len(selectorPositions))
		for pos := range selectorPositions {
			selectorCosets[pos] = computeLagrangeSelectorCoset(pos, n, N)
		}

		// --- Compute the aggregate extension-field polynomial on the coset ---
		// aggregate[j] = Σ_i coin^i · P_i(coset_j) · C_i(coset_j)
		//
		// Reuse scratchAgg if Plan was called; it may contain stale data from
		// the previous proof run, so clear it before use as an accumulator.
		aggregate := bkt.scratchAgg
		if len(aggregate) < N {
			aggregate = make([]field.Ext, N)
		} else {
			clear(aggregate[:N])
		}

		// Compile the bucket's vanishings into one program: leaves resolve to
		// their coset slices and runtime scalars up front, and every distinct
		// subexpression becomes a single instruction however many vanishings
		// share it. The parallel workers then only read the program, which
		// keeps them free of shared-state reads (and of the runtime mutex).
		var coinPow field.Ext
		coinPow.SetOne()
		var roots []quotientRoot
		if bkt.entries != nil {
			// Static module: use precomputed cancellation cosets.
			roots = make([]quotientRoot, 0, len(bkt.entries))
			for _, entry := range bkt.entries {
				roots = append(roots, quotientRoot{
					expr:         entry.v.Expression,
					cancellation: entry.cancellationCoset,
					coinPow:      coinPow,
				})
				// advance coinPow: coinPow *= coinExt
				coinPow.Mul(&coinPow, &coinExt)
			}
		} else {
			// Dynamic module: compute cancellation cosets at runtime.
			roots = make([]quotientRoot, 0, len(bkt.vanishings))
			for _, v := range bkt.vanishings {
				roots = append(roots, quotientRoot{
					expr:         v.Expression,
					cancellation: computeCancellationCoset(v.CancelledPositions, n, N),
					coinPow:      coinPow,
				})
				coinPow.Mul(&coinPow, &coinExt)
			}
		}
		prog := buildQuotientProgram(rt, roots, cosetEvals, cosetEvalsExt, selectorCosets, ratio, N)

		// Coset points are independent, so the accumulation is chunked across
		// CPUs: each worker owns a disjoint aggregate[start:end] range and only
		// reads the program. Field addition is exact, so the order in which
		// the constraints accumulate does not affect the result.
		parallel.Execute(N, func(start, end int) {
			prog.run(aggregate, start, end)
			// --- Divide by annihilator (x^n − 1) at each coset point ---
			// annihilator at point j is annInv[j % ratio] (already inverted).
			for j := start; j < end; j++ {
				aggregate[j].MulByElement(&aggregate[j], &annInv[j%ratio])
			}
		}, prog.workers(N, runtime.GOMAXPROCS(0)))

		// --- IFFT on the large coset: coset evals → canonical coefficients ---
		// FFTInverseExt6 operates directly on the contiguous E6 layout.
		// In DIF mode it returns coefficients in BIT-REVERSED order across
		// the full size-N domain.
		largeDomain.FFTInverseExt6(aggregate[:N], fft.DIF, fft.OnCoset())

		// For ratio == 1 the FFT(DIT) in the loop below consumes bit-reversed
		// input directly, so we can slice without a prior bit-reverse. For
		// ratio > 1 the bit-reversal across N interleaves coefficients
		// across the ratio chunks (for ratio = 2, aggregate[0:n] would hold
		// the even-indexed coefficients of the size-N polynomial rather
		// than the contiguous low-degree slice we need to form Q_0). We
		// bit-reverse to natural order, then bit-reverse each chunk so the
		// subsequent FFT(DIT) still sees its expected bit-reversed input.
		// Net effect for ratio > 1: aggregate -> natural, then per-chunk
		// natural -> bit-reversed -> FFT(DIT) -> natural Lagrange.
		if ratio > 1 {
			gnarkutils.BitReverse(aggregate[:N])
		}

		// --- Split into ratio chunks and FFT each to standard Lagrange form ---
		for k := range ratio {
			chunk := make([]field.Ext, n)
			copy(chunk, aggregate[k*n:(k+1)*n])
			if ratio > 1 {
				gnarkutils.BitReverse(chunk)
			}
			extFFT(smallDomain, chunk)

			cv := &wiop.ConcreteVector{
				Plain: field.VecFromExt(chunk),
			}
			rt.AssignColumn(bkt.shares[k], cv)
		}
	}
}

// reevalOnLargeCoset evaluates the column col in Lagrange basis on the large
// coset {g · ω_N^j : j = 0…N-1} using the iFFT → zero-pad → FFT(coset) route.
func reevalOnLargeCoset(
	rt *wiop.Runtime,
	col *wiop.Column,
	m *wiop.Module,
	n, N int,
	smallDomain, largeDomain *fft.Domain,
) []field.Element {
	cv := rt.GetColumnAssignment(col)

	// Build the full n-length standard-domain evaluation.
	// Use ElementAtN with explicit size to support dynamic modules.
	vals := make([]field.Element, N) // zero-padded
	for i := range n {
		elem := cv.ElementAtN(m.Padding, n, i)
		if !elem.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: global quotient does not support extension-field columns in vanishing expressions; column %q",
				col.Context.Path(),
			))
		}
		vals[i] = elem.AsBase()
	}

	// iFFT on small domain (standard, no coset shift): Lagrange → canonical.
	// FFTInverse(DIF) leaves the output in bit-reversed-of-n order. When
	// n == N (ratio == 1) the trailing zero-pad is empty and bit-reversed-of-n
	// matches bit-reversed-of-N, so FFT(DIT) below consumes the result
	// directly. For n < N the bit-reversal index space changes between the
	// two FFTs, so we normalise to natural order in between (BitReverse on
	// vals[:n] then on vals[:N]) before re-introducing bit-reversal for the
	// large FFT's DIT input convention.
	smallDomain.FFTInverse(vals[:n], fft.DIF)
	if N != n {
		gnarkutils.BitReverse(vals[:n])
		// vals[n:N] is already zero, so vals[:N] is now natural-order
		// coefficients of the zero-padded polynomial. Re-bit-reverse to
		// feed FFT(DIT) which expects bit-reversed input.
		gnarkutils.BitReverse(vals[:N])
	}
	// FFT on large coset: canonical → coset Lagrange.
	largeDomain.FFT(vals, fft.DIT, fft.OnCoset())
	return vals
}

// reevalOnLargeCosetExt is the extension-field counterpart of
// [reevalOnLargeCoset]. It evaluates an extension-field column on the
// large coset, using the Ext6 FFT path so the prover can incorporate
// extension witness columns (e.g. the Z columns produced by the
// log-derivative compiler) into a quotient bucket.
//
// The bit-reversal accounting mirrors the base-field version: the small
// IFFT(DIF) returns bit-reversed-of-n coefficients in vals[:n] with the
// trailing zero-pad untouched; if n < N the index space switches between
// the two FFTs, so we BitReverse twice to normalise the polynomial layout
// before feeding it to the large FFT(DIT, OnCoset).
func reevalOnLargeCosetExt(
	rt *wiop.Runtime,
	col *wiop.Column,
	m *wiop.Module,
	n, N int,
	smallDomain, largeDomain *fft.Domain,
) []field.Ext {
	cv := rt.GetColumnAssignment(col)

	vals := make([]field.Ext, N) // zero-padded
	for i := range n {
		elem := cv.ElementAtN(m.Padding, n, i)
		if elem.IsBase() {
			vals[i] = field.Lift(elem.AsBase())
		} else {
			vals[i] = elem.AsExt()
		}
	}

	smallDomain.FFTInverseExt6(vals[:n], fft.DIF)
	if N != n {
		gnarkutils.BitReverse(vals[:n])
		gnarkutils.BitReverse(vals[:N])
	}
	largeDomain.FFTExt6(vals, fft.DIT, fft.OnCoset())
	return vals
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

	// planOnce guards plan, the evaluation order of the distinct expression
	// nodes of every bucket, built on the first Check.
	planOnce sync.Once
	plan     *verifierPlan
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

	// Every bucket is evaluated at the same point r, so each distinct node of
	// the module -- shared between vanishings or between buckets -- is
	// evaluated once.
	gv.planOnce.Do(func() { gv.plan = newVerifierPlan(gv.Buckets) })
	vals := gv.plan.evaluate(viewEvals, r, rt)

	for b, bkt := range gv.Buckets {
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
		for i, v := range bkt.Vanishings {
			pr := vals[gv.plan.roots[b][i]]
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

// verifierPlan lists the distinct expression nodes (by pointer) of a
// module's vanishings in post-order, so that operands precede the nodes
// reading them, and the node of each vanishing per bucket.
type verifierPlan struct {
	nodes []verifierNode
	roots [][]int // roots[bucket][vanishing] indexes nodes
}

// verifierNode is a leaf, resolved against the claims and the runtime, or an
// operator over earlier nodes.
type verifierNode struct {
	leaf     wiop.Expression // nil for an operator node
	operator wiop.ArithmeticOperator
	operands [2]int // indices into nodes; -1 when absent
}

func newVerifierPlan(buckets []VerifierBucket) *verifierPlan {
	p := &verifierPlan{roots: make([][]int, len(buckets))}
	index := make(map[wiop.Expression]int)
	var visit func(e wiop.Expression) int
	visit = func(e wiop.Expression) int {
		if i, ok := index[e]; ok {
			return i
		}
		n := verifierNode{operands: [2]int{-1, -1}}
		if op, ok := e.(*wiop.ArithmeticOperation); ok {
			n.operator = op.Operator
			for k, o := range op.Operands {
				n.operands[k] = visit(o)
			}
		} else {
			n.leaf = e
		}
		p.nodes = append(p.nodes, n)
		index[e] = len(p.nodes) - 1
		return len(p.nodes) - 1
	}
	for b, bkt := range buckets {
		p.roots[b] = make([]int, len(bkt.Vanishings))
		for i, v := range bkt.Vanishings {
			p.roots[b][i] = visit(v.Expression)
		}
	}
	return p
}

// evaluate returns the value of every node at the point r, using the witness
// column evaluation map (from LagrangeEval claim cells). Coins and cells are
// looked up directly from the runtime.
func (p *verifierPlan) evaluate(viewEvals map[colViewKey]field.Gen, r field.Gen, rt *wiop.Runtime) []field.Gen {
	vals := make([]field.Gen, len(p.nodes))
	for i, n := range p.nodes {
		if n.leaf != nil {
			vals[i] = evalLeafAtPoint(n.leaf, viewEvals, r, rt)
			continue
		}
		a0 := vals[n.operands[0]]
		switch n.operator {
		case wiop.ArithmeticOperatorAdd:
			vals[i] = a0.Add(vals[n.operands[1]])
		case wiop.ArithmeticOperatorSub:
			vals[i] = a0.Sub(vals[n.operands[1]])
		case wiop.ArithmeticOperatorMul:
			vals[i] = a0.Mul(vals[n.operands[1]])
		case wiop.ArithmeticOperatorDiv:
			vals[i] = a0.Div(vals[n.operands[1]])
		case wiop.ArithmeticOperatorDouble:
			vals[i] = a0.Add(a0)
		case wiop.ArithmeticOperatorSquare:
			vals[i] = a0.Square()
		case wiop.ArithmeticOperatorNegate:
			vals[i] = a0.Neg()
		case wiop.ArithmeticOperatorInverse:
			vals[i] = a0.Inverse()
		default:
			panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", n.operator))
		}
	}
	return vals
}

// evalLeafAtPoint evaluates a non-compound expression at the point r.
func evalLeafAtPoint(
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
	case *wiop.Constant:
		return field.ElemFromBase(e.Value)
	case *wiop.CoinField:
		return rt.GetCoinValue(e)
	case *wiop.Cell:
		return rt.GetCellValue(e)
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in global verifier", expr))
	}
}

// ---------------------------------------------------------------------------
// Expression tree traversal helpers
// ---------------------------------------------------------------------------

// walkLeaves calls visit on the leaves of expr in depth-first, left-to-right
// order, descending into each distinct compound node once. A leaf reachable
// only through an already-visited node was reported on the first visit, so
// the order of first occurrences is that of the full tree walk, while the cost
// is linear in the distinct nodes rather than in the tree size.
func walkLeaves(expr wiop.Expression, visit func(wiop.Expression)) {
	seen := make(map[*wiop.ArithmeticOperation]struct{})
	var walk func(wiop.Expression)
	walk = func(e wiop.Expression) {
		op, ok := e.(*wiop.ArithmeticOperation)
		if !ok {
			visit(e)
			return
		}
		if _, done := seen[op]; done {
			return
		}
		seen[op] = struct{}{}
		for _, o := range op.Operands {
			walk(o)
		}
	}
	walk(expr)
}

// collectColumnViews collects the *ColumnView leaves of expr in order of first
// occurrence. A view may appear more than once.
func collectColumnViews(expr wiop.Expression) []*wiop.ColumnView {
	var result []*wiop.ColumnView
	walkLeaves(expr, func(leaf wiop.Expression) {
		if cv, ok := leaf.(*wiop.ColumnView); ok {
			result = append(result, cv)
		}
	})
	return result
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

// ---------------------------------------------------------------------------
// Extension-field FFT helpers
// ---------------------------------------------------------------------------

// extFFT applies the forward standard-domain FFT to the extension-field slice
// v. The gnark-crypto FFTExt6 implementation handles the six E6 coordinates
// directly on the contiguous layout.
func extFFT(d *fft.Domain, v []field.Ext) {
	d.FFTExt6(v, fft.DIT)
}

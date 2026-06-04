package global

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gnarkutils "github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
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

	for i, m := range sys.Modules {
		if len(m.Vanishings) == 0 {
			continue
		}
		mCtx := compCtx.Childf("m%d", i)
		compileModule(sys, m, mCtx, quotientRound, evalRound)
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
			shares[k] = m.NewExtensionColumn(shareCtx, wiop.VisibilityOracle, quotientRound)
		}
		rawBuckets = append(rawBuckets, rawBucket{ratio: ratio, vanishings: vs, shares: shares})
	}

	// --- Step 4: eval coin in evalRound ---
	evalCoin := evalRound.NewCoinField(ctx.Childf("eval-coin"))

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
func (a *QuotientProverAction) Run(rt wiop.Runtime) {
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

		var coinPow field.Ext
		coinPow.SetOne()

		if bkt.entries != nil {
			// Static module: use precomputed cancellation cosets.
			for _, entry := range bkt.entries {
				accumulateOnCoset(
					rt, entry.v.Expression, cosetEvals, cosetEvalsExt,
					entry.cancellationCoset, &coinPow, aggregate, ratio, N,
				)
				// advance coinPow: coinPow *= coinExt
				coinPow.Mul(&coinPow, &coinExt)
			}
		} else {
			// Dynamic module: compute cancellation cosets at runtime.
			for _, v := range bkt.vanishings {
				cancellationCoset := computeCancellationCoset(v.CancelledPositions, n, N)
				accumulateOnCoset(
					rt, v.Expression, cosetEvals, cosetEvalsExt,
					cancellationCoset, &coinPow, aggregate, ratio, N,
				)
				coinPow.Mul(&coinPow, &coinExt)
			}
		}

		// --- Divide by annihilator (x^n − 1) at each coset point ---
		// annihilator at point j is annInv[j % ratio] (already inverted).
		for j := 0; j < N; j++ {
			aggregate[j].MulByElement(&aggregate[j], &annInv[j%ratio])
		}

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
	rt wiop.Runtime,
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
	rt wiop.Runtime,
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
func (a *EvalProverAction) Run(rt wiop.Runtime) {
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
func (gv *Verifier) Check(rt wiop.Runtime) error {
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
			pr := evalExprAtPoint(v.Expression, viewEvals, rt)
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

// evalExprAtPoint evaluates a symbolic expression at a pre-computed scalar
// point using the witness column evaluation map (from LagrangeEval claim cells).
// Coins and cells are looked up directly from the runtime.
func evalExprAtPoint(
	expr wiop.Expression,
	viewEvals map[colViewKey]field.Gen,
	rt wiop.Runtime,
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
	case *wiop.ArithmeticOperation:
		eval := func(i int) field.Gen {
			return evalExprAtPoint(e.Operands[i], viewEvals, rt)
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

// accumulateOnCoset adds the contribution of one Vanishing expression to the
// per-coset-point aggregate accumulator. It classifies the expression once
// via [isBaseExpr] and then runs a specialised inner loop that stays
// entirely in the base or extension field — avoiding the field.Gen
// dispatch overhead that would otherwise be paid on every operation across
// all N coset points.
//
//   - For a base-only expression: the j-loop multiplies pVal·cancellation in
//     base, then promotes once into Ext via [field.Ext.MulByElement].
//   - For an extension expression: the j-loop multiplies pVal·cancellation
//     in extension (the cancellation is base, so MulByElement is used), then
//     [field.Ext.Mul] accumulates into the aggregate.
//
// cancellationCoset may be nil, in which case the cancellation factor is
// implicitly 1 and skipped.
func accumulateOnCoset(
	rt wiop.Runtime,
	expr wiop.Expression,
	cosetEvals map[wiop.ObjectID][]field.Element,
	cosetEvalsExt map[wiop.ObjectID][]field.Ext,
	cancellationCoset []field.Element,
	coinPow *field.Ext,
	aggregate []field.Ext,
	ratio, N int,
) {
	if isBaseExpr(expr) {
		for j := 0; j < N; j++ {
			pVal := evalExprOnCoset(rt, expr, cosetEvals, j, ratio, N)
			var pTimesC field.Element
			if cancellationCoset != nil {
				pTimesC.Mul(&pVal, &cancellationCoset[j])
			} else {
				pTimesC = pVal
			}
			var term field.Ext
			term.MulByElement(coinPow, &pTimesC)
			aggregate[j].Add(&aggregate[j], &term)
		}
		return
	}
	for j := 0; j < N; j++ {
		pVal := evalExprOnCosetExt(rt, expr, cosetEvals, cosetEvalsExt, j, ratio, N)
		var pTimesC field.Ext
		if cancellationCoset != nil {
			pTimesC.MulByElement(&pVal, &cancellationCoset[j])
		} else {
			pTimesC = pVal
		}
		var term field.Ext
		term.Mul(coinPow, &pTimesC)
		aggregate[j].Add(&aggregate[j], &term)
	}
}

// isBaseExpr reports whether expr evaluates to a base-field element at every
// coset point. Extension-field cells, extension-field column views, and
// CoinField leaves (always extension) make the result extension;
// everything else is base. The check is purely structural, so it is
// computed once per Vanishing expression and reused across all N coset
// points.
func isBaseExpr(expr wiop.Expression) bool {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		return !e.Column.IsExtension
	case *wiop.Constant:
		return true
	case *wiop.Cell:
		return !e.IsExtension()
	case *wiop.CoinField:
		return false
	case *wiop.ArithmeticOperation:
		for _, op := range e.Operands {
			if !isBaseExpr(op) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// evalExprOnCoset evaluates a base-field vanishing expression at coset point j
// and returns a base-field element. The caller must guarantee that the
// expression is base — i.e. [isBaseExpr] returned true — otherwise the
// Cell case will panic on an extension-typed leaf and CoinField will panic
// unconditionally.
//
// cosetEvals maps each root column ID to its N-length coset evaluation array.
// For a ColumnView with shift k, the coset index is (j + k·ratio) mod N.
//
// For expressions containing extension-typed leaves, use [evalExprOnCosetExt]
// instead.
func evalExprOnCoset(
	rt wiop.Runtime,
	expr wiop.Expression,
	cosetEvals map[wiop.ObjectID][]field.Element,
	j, ratio, N int,
) field.Element {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		k := e.ShiftingOffset
		idx := ((j+k*ratio)%N + N) % N
		return cosetEvals[e.Column.Context.ID][idx]
	case *wiop.Cell:
		if e.IsExtension() {
			panic(fmt.Sprintf(
				"wiop/compilers: extension-field cell %q reached the base-field coset evaluator; "+
					"the caller must dispatch on isBaseExpr",
				e.Context.Path(),
			))
		}
		v := rt.GetCellValue(e)
		if !v.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: cell %q declared as base but holds an extension-field value",
				e.Context.Path(),
			))
		}
		return v.AsBase()
	case *wiop.Constant:
		return e.Value
	case *wiop.ArithmeticOperation:
		eval := func(i int) field.Element {
			return evalExprOnCoset(rt, e.Operands[i], cosetEvals, j, ratio, N)
		}
		a0 := eval(0)
		var res field.Element
		switch e.Operator {
		case wiop.ArithmeticOperatorAdd:
			a1 := eval(1)
			res.Add(&a0, &a1)
		case wiop.ArithmeticOperatorSub:
			a1 := eval(1)
			res.Sub(&a0, &a1)
		case wiop.ArithmeticOperatorMul:
			a1 := eval(1)
			res.Mul(&a0, &a1)
		case wiop.ArithmeticOperatorDiv:
			a1 := eval(1)
			var invA1 field.Element
			invA1.Inverse(&a1)
			res.Mul(&a0, &invA1)
		case wiop.ArithmeticOperatorDouble:
			res.Add(&a0, &a0)
		case wiop.ArithmeticOperatorSquare:
			res.Square(&a0)
		case wiop.ArithmeticOperatorNegate:
			res.Neg(&a0)
		case wiop.ArithmeticOperatorInverse:
			res.Inverse(&a0)
		default:
			panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", e.Operator))
		}
		return res
	case *wiop.CoinField:
		panic("wiop/compilers: CoinField reached the base-field coset evaluator; the caller must dispatch on isBaseExpr")
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in evalExprOnCoset", expr))
	}
}

// evalExprOnCosetExt is the extension-field counterpart of [evalExprOnCoset].
// It accepts any leaf type, lifting base-field values (column samples,
// constants, base cells) into the extension field, and is used when the
// expression contains at least one extension-typed leaf (extension cell,
// extension column view, or any coin).
//
// All arithmetic runs in the extension field — including for sub-expressions
// that happen to be pure base — so this path is slower per operation than
// [evalExprOnCoset]. Callers should dispatch on [isBaseExpr] to use the
// base-field fast path whenever possible.
//
// cosetEvalsExt holds the extension-field coset evaluations for any
// extension witness columns referenced by the expression. ColumnView leaves
// dispatch on their underlying column's IsExtension flag.
func evalExprOnCosetExt(
	rt wiop.Runtime,
	expr wiop.Expression,
	cosetEvals map[wiop.ObjectID][]field.Element,
	cosetEvalsExt map[wiop.ObjectID][]field.Ext,
	j, ratio, N int,
) field.Ext {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		k := e.ShiftingOffset
		idx := ((j+k*ratio)%N + N) % N
		if e.Column.IsExtension {
			return cosetEvalsExt[e.Column.Context.ID][idx]
		}
		return field.Lift(cosetEvals[e.Column.Context.ID][idx])
	case *wiop.Cell:
		return rt.GetCellValue(e).AsExt()
	case *wiop.CoinField:
		return rt.GetCoinValue(e).AsExt()
	case *wiop.Constant:
		return field.Lift(e.Value)
	case *wiop.ArithmeticOperation:
		eval := func(i int) field.Ext {
			return evalExprOnCosetExt(rt, e.Operands[i], cosetEvals, cosetEvalsExt, j, ratio, N)
		}
		a0 := eval(0)
		var res field.Ext
		switch e.Operator {
		case wiop.ArithmeticOperatorAdd:
			a1 := eval(1)
			res.Add(&a0, &a1)
		case wiop.ArithmeticOperatorSub:
			a1 := eval(1)
			res.Sub(&a0, &a1)
		case wiop.ArithmeticOperatorMul:
			a1 := eval(1)
			res.Mul(&a0, &a1)
		case wiop.ArithmeticOperatorDiv:
			a1 := eval(1)
			var inv field.Ext
			inv.Inverse(&a1)
			res.Mul(&a0, &inv)
		case wiop.ArithmeticOperatorDouble:
			res.Double(&a0)
		case wiop.ArithmeticOperatorSquare:
			res.Square(&a0)
		case wiop.ArithmeticOperatorNegate:
			res.Neg(&a0)
		case wiop.ArithmeticOperatorInverse:
			res.Inverse(&a0)
		default:
			panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", e.Operator))
		}
		return res
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in evalExprOnCosetExt", expr))
	}
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

// ---------------------------------------------------------------------------
// Extension-field FFT helpers
// ---------------------------------------------------------------------------

// extFFT applies the forward standard-domain FFT to the extension-field slice
// v. The gnark-crypto FFTExt6 implementation handles the six E6 coordinates
// directly on the contiguous layout.
func extFFT(d *fft.Domain, v []field.Ext) {
	d.FFTExt6(v, fft.DIT)
}

package global

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
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
// Panics if any module with vanishing constraints has not been sized yet.
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
// generator and N = n · ratio.
type proverVanishingEntry struct {
	v                 *wiop.Vanishing
	cancellationCoset []field.Element // length N = n*ratio; nil if no cancellation
}

// proverBucket holds all compilation artefacts needed by the prover to compute
// the quotient shares for one ratio bucket.
type proverBucket struct {
	ratio       int
	entries     []proverVanishingEntry
	rootCols    []*wiop.Column  // deduplicated root columns from all expressions
	shares      []*wiop.Column  // quotient share columns (length = ratio)
	smallDomain *fft.Domain     // FFT domain of size n
	largeDomain *fft.Domain     // FFT domain of size n*ratio
	annInv      []field.Element // 1/(g^n · ω_ratio^j − 1) for j = 0..ratio-1

	// Pre-allocated scratch slices populated by Plan; nil until Materialize is
	// called. When non-nil, Run uses these instead of allocating fresh memory.
	scratchAgg  []field.Ext     // aggregate[j], length N = n*ratio
	scratchVals []field.Element // coset re-evaluation buffer, length N
	scratchC0   []field.Element // coordinate slice 0 for applyBaseFFT4, length N
	scratchC1   []field.Element // coordinate slice 1 for applyBaseFFT4, length N
	scratchC2   []field.Element // coordinate slice 2 for applyBaseFFT4, length N
	scratchC3   []field.Element // coordinate slice 3 for applyBaseFFT4, length N
}

// verifierBucket holds everything the verifier needs for one ratio bucket.
type verifierBucket struct {
	ratio          int
	vanishings     []*wiop.Vanishing
	quotientClaims []*wiop.Cell // Q_k(r) claim cells, length = ratio
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
	n := m.Size()
	if n == 0 {
		panic(fmt.Sprintf("wiop/compilers: module %q must be sized before calling Compile", m.Context.Path()))
	}

	// --- Step 1: bucket vanishing constraints by ratio ---
	ratioToEntries := make(map[int][]*wiop.Vanishing)
	var ratioOrder []int
	for _, v := range m.Vanishings {
		r := computeRatio(v, n)
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

	// --- Step 8: build prover buckets (precompute coset data) ---
	proverBuckets := buildProverBuckets(rawBuckets, n)

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
	vBuckets := make([]verifierBucket, len(rawBuckets))
	for i, bkt := range rawBuckets {
		vBuckets[i] = verifierBucket{
			ratio:          bkt.ratio,
			vanishings:     bkt.vanishings,
			quotientClaims: quotientBucketClaims[i],
		}
	}
	evalRound.RegisterVerifierAction(&Verifier{
		n:             n,
		mergeCoin:     mergeCoin,
		evalCoin:      evalCoin,
		witnessViews:  views,
		witnessClaims: witnessClaims,
		viewKeyToIdx:  viewKeyToIdx,
		buckets:       vBuckets,
	})
}

// buildProverBuckets constructs the runtime prover buckets from the raw bucket
// descriptions, precomputing all data that depends only on the system
// structure (not on runtime witness assignments).
func buildProverBuckets(rawBuckets []rawBucket, n int) []proverBucket {
	result := make([]proverBucket, len(rawBuckets))
	for i, bkt := range rawBuckets {
		ratio := bkt.ratio
		N := n * ratio

		smallDomain := fft.NewDomain(uint64(n))
		largeDomain := fft.NewDomain(uint64(N))

		// Precompute annihilator inverses: 1/(g^n · ω_ratio^j − 1) for j=0..ratio-1.
		annVals := polynomials.EvalXnMinusOneOnCoset(n, N)
		annInv := make([]field.Element, ratio)
		field.VecBatchInvBase(annInv, annVals)

		// Precompute cancellation polynomial coset evaluations.
		entries := make([]proverVanishingEntry, len(bkt.vanishings))
		for j, v := range bkt.vanishings {
			entries[j] = proverVanishingEntry{
				v:                 v,
				cancellationCoset: precomputeCancellationCoset(v.CancelledPositions, n, N),
			}
		}

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

		result[i] = proverBucket{
			ratio:       ratio,
			entries:     entries,
			rootCols:    rootCols,
			shares:      bkt.shares,
			smallDomain: smallDomain,
			largeDomain: largeDomain,
			annInv:      annInv,
		}
	}
	return result
}

// precomputeCancellationCoset returns the base-field evaluation of the
// cancellation polynomial C(X) = Π_{k ∈ cancelled} (X − ω_n^{norm(k)}) at
// all N = n·ratio coset points {g · ω_N^j : j = 0…N-1}. Returns nil when
// there are no cancelled positions.
func precomputeCancellationCoset(cancelled []int, n, N int) []field.Element {
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
// arena. When called before the first proof, Run uses these slices instead of
// allocating fresh memory on every invocation.
func (a *QuotientProverAction) Plan(ctx *wiop.PlanningContext) {
	n := a.m.Size()
	for i := range a.buckets {
		bkt := &a.buckets[i]
		N := n * bkt.ratio
		bkt.scratchAgg = ctx.AllocExt(N)
		bkt.scratchVals = ctx.AllocField(N)
		bkt.scratchC0 = ctx.AllocField(N)
		bkt.scratchC1 = ctx.AllocField(N)
		bkt.scratchC2 = ctx.AllocField(N)
		bkt.scratchC3 = ctx.AllocField(N)
	}
}

// Run executes the quotient polynomial computation and assigns quotient share columns.
func (a *QuotientProverAction) Run(rt wiop.Runtime) {
	n := a.m.Size()
	coinExt := rt.GetCoinValue(a.mergeCoin).Ext

	for _, bkt := range a.buckets {
		ratio := bkt.ratio
		N := n * ratio

		// --- Evaluate all root columns on the large coset ---
		// cosetEvals[colID][j] = col evaluated at coset point j
		cosetEvals := make(map[wiop.ObjectID][]field.Element, len(bkt.rootCols))
		for _, col := range bkt.rootCols {
			cosetEvals[col.Context.ID] = reevalOnLargeCoset(
				rt, col, a.m, n, N, bkt.smallDomain, bkt.largeDomain,
			)
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
		for _, entry := range bkt.entries {
			for j := 0; j < N; j++ {
				pVal := evalExprOnCoset(entry.v.Expression, cosetEvals, j, ratio, N)
				var pTimesC field.Element
				if entry.cancellationCoset != nil {
					pTimesC.Mul(&pVal, &entry.cancellationCoset[j])
				} else {
					pTimesC = pVal
				}
				// aggregate[j] += coinPow * pTimesC
				var term field.Ext
				term.MulByElement(&coinPow, &pTimesC)
				aggregate[j].Add(&aggregate[j], &term)
			}
			// advance coinPow: coinPow *= coinExt
			coinPow.Mul(&coinPow, &coinExt)
		}

		// --- Divide by annihilator (x^n − 1) at each coset point ---
		// annihilator at point j is annInv[j % ratio] (already inverted).
		for j := 0; j < N; j++ {
			aggregate[j].MulByElement(&aggregate[j], &bkt.annInv[j%ratio])
		}

		// --- IFFT on the large coset: coset evals → canonical coefficients ---
		// Operates component-wise on the 4 base-field components of Ext.
		// Use pre-allocated coordinate scratch buffers when available.
		applyBaseFFT4(bkt.largeDomain, aggregate[:N], func(d *fft.Domain, c []field.Element) {
			d.FFTInverse(c, fft.DIF, fft.OnCoset())
		}, bkt.scratchC0, bkt.scratchC1, bkt.scratchC2, bkt.scratchC3)

		// --- Split into ratio chunks and FFT each to standard Lagrange form ---
		for k := range ratio {
			chunk := make([]field.Ext, n)
			copy(chunk, aggregate[k*n:(k+1)*n])
			extFFT(bkt.smallDomain, chunk)

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
	vals := make([]field.Element, N) // zero-padded
	for i := range n {
		elem := cv.ElementAt(m, i)
		if !elem.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: global quotient does not support extension-field columns in vanishing expressions; column %q",
				col.Context.Path(),
			))
		}
		vals[i] = elem.AsBase()
	}

	// iFFT on small domain (standard, no coset shift): Lagrange → canonical.
	smallDomain.FFTInverse(vals[:n], fft.DIF)
	// FFT on large coset: canonical → coset Lagrange.
	largeDomain.FFT(vals, fft.DIT, fft.OnCoset())
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
	n             int
	mergeCoin     *wiop.CoinField
	evalCoin      *wiop.CoinField
	witnessViews  []*wiop.ColumnView
	witnessClaims []*wiop.Cell
	viewKeyToIdx  map[colViewKey]int
	buckets       []verifierBucket
}

// Check verifies the PLONK quotient identity for the module using the runtime's claimed values.
func (gv *Verifier) Check(rt wiop.Runtime) error {
	n := gv.n
	r := rt.GetCoinValue(gv.evalCoin)
	coinExt := rt.GetCoinValue(gv.mergeCoin).Ext

	// Build the map from column-view key → evaluation at r.
	viewEvals := make(map[colViewKey]field.Gen, len(gv.witnessViews))
	for i, cv := range gv.witnessViews {
		key := colViewKey{id: cv.Column.Context.ID, shift: cv.ShiftingOffset}
		viewEvals[key] = rt.GetCellValue(gv.witnessClaims[i])
	}

	// Compute annihilator r^n − 1.
	annihilator := computeAnnihilator(r, n)

	for _, bkt := range gv.buckets {
		// --- Recombine quotient shares: Q(r) = Σ_k r^{kn} · Q_k(r) ---
		qr := field.ElemZero()
		rPowKN := field.ElemOne() // r^{kn}, starts at r^0 = 1
		for k, claim := range bkt.quotientClaims {
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
		for _, v := range bkt.vanishings {
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
				n, bkt.ratio,
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

// evalExprOnCoset evaluates a base-field vanishing expression at coset point j.
// cosetEvals maps each root column ID to its N-length coset evaluation array.
// For a ColumnView with shift k, the coset index is (j + k·ratio) mod N.
// Panics if the expression contains a CoinField or Cell (not supported for
// base-field coset evaluation).
func evalExprOnCoset(
	expr wiop.Expression,
	cosetEvals map[wiop.ObjectID][]field.Element,
	j, ratio, N int,
) field.Element {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		k := e.ShiftingOffset
		idx := ((j+k*ratio)%N + N) % N
		return cosetEvals[e.Column.Context.ID][idx]
	case *wiop.ArithmeticOperation:
		eval := func(i int) field.Element {
			return evalExprOnCoset(e.Operands[i], cosetEvals, j, ratio, N)
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
	case *wiop.Constant:
		return e.Value
	case *wiop.CoinField, *wiop.Cell:
		panic("wiop/compilers: CoinField and Cell in vanishing expression coset evaluation are not supported")
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in evalExprOnCoset", expr))
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

// computeRatio returns the smallest power of two ≥ 1 such that
// ratio · n ≥ deg(v.Expression) + len(v.CancelledPositions) + 1.
func computeRatio(v *wiop.Vanishing, n int) int {
	exprDeg := v.Expression.Degree()
	cancelDeg := len(v.CancelledPositions)
	effectiveDeg := exprDeg + cancelDeg
	quotientSize := effectiveDeg - n + 1
	ratio := utils.DivCeil(max(1, quotientSize), n)
	return utils.NextPowerOfTwo(ratio)
}

// ---------------------------------------------------------------------------
// Extension-field FFT helpers
// ---------------------------------------------------------------------------

// extFFT applies the forward standard-domain FFT to the extension-field slice v,
// operating component-wise. After the call, v contains standard Lagrange
// evaluations.
func extFFT(d *fft.Domain, v []field.Ext) {
	applyBaseFFT4(d, v, func(d *fft.Domain, c []field.Element) {
		d.FFT(c, fft.DIT)
	}, nil, nil, nil, nil)
}

// applyBaseFFT4 deinterleaves v into four base-field coordinate slices, applies
// fn to each, then reassembles the result back into v. If any of c0..c3 is
// non-nil and long enough, it is used as scratch instead of allocating.
func applyBaseFFT4(d *fft.Domain, v []field.Ext, fn func(*fft.Domain, []field.Element),
	c0, c1, c2, c3 []field.Element) {
	n := len(v)
	if len(c0) < n {
		c0 = make([]field.Element, n)
	}
	if len(c1) < n {
		c1 = make([]field.Element, n)
	}
	if len(c2) < n {
		c2 = make([]field.Element, n)
	}
	if len(c3) < n {
		c3 = make([]field.Element, n)
	}
	for i, e := range v {
		c0[i] = e.B0.A0
		c1[i] = e.B0.A1
		c2[i] = e.B1.A0
		c3[i] = e.B1.A1
	}
	fn(d, c0)
	fn(d, c1)
	fn(d, c2)
	fn(d, c3)
	for i := range v {
		v[i].B0.A0 = c0[i]
		v[i].B0.A1 = c1[i]
		v[i].B1.A0 = c2[i]
		v[i].B1.A1 = c3[i]
	}
}

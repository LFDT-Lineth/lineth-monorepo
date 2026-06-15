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

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
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

	var entries []zEntry
	for bIdx, b := range buckets {
		groups := packFractions(b.fractions)
		for kIdx, packed := range groups {
			entries = append(entries,
				buildZ(b.module, packed, resultRound, compCtx, bIdx, kIdx))
		}
	}

	resultRound.RegisterAction(&proverAction{ld: ld, entries: entries})
	resultRound.RegisterVerifierAction(&verifierAction{ld: ld, entries: entries})
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

// zEntry collects the per-Z artefacts shared by the prover and verifier
// actions: the Z column, the raw fractions (filter, num, den) used by the
// prover for filter-aware row skipping, and the column endpoint opening.
type zEntry struct {
	zCol   *wiop.Column
	packed []wiop.Fraction // raw fractions used by the prover for filter-aware evaluation
	zFinal *wiop.Cell      // lazily-assigned opening of Z[n-1]
}

// buildZ allocates one Z column for a packed fraction group, registers the
// recurrence Vanishing, pins the row-0 boundary with a local constraint, and
// opens the column endpoint.
func buildZ(
	m *wiop.Module,
	packed []wiop.Fraction,
	round *wiop.Round,
	ctx *wiop.ContextFrame,
	bIdx, kIdx int,
) zEntry {
	n := m.Size()
	if n <= 0 {
		panic(fmt.Sprintf(
			"wiop/compilers/logderivativesum: module %q must be sized before Compile",
			m.Context.Path(),
		))
	}

	zNum, zDen := buildZExpressions(packed)

	zCol := m.NewExtensionColumn(
		ctx.Childf("z-b%d-k%d", bIdx, kIdx),
		wiop.VisibilityOracle,
		round,
	)

	// The recurrence zNum − (Z − Z<<−1)·zDen carries a −1 shift on Z, so
	// NewVanishing automatically cancels row 0. For a single-row module the
	// recurrence is vacuous: skip it.
	if n > 1 {
		zView := zCol.View()
		recurrence := wiop.Sub(
			zNum,
			wiop.Mul(
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
	// verifier reading the oracle witness columns. This also covers the
	// single-row (n == 1) case where the recurrence is skipped.
	m.NewLocalConstraint(
		ctx.Childf("z-init-b%d-k%d", bIdx, kIdx),
		wiop.Sub(zNum, wiop.Mul(zCol.View(), zDen)),
		0,
	)

	zFinal := zCol.At(n - 1).Open(ctx.Childf("z-final-b%d-k%d", bIdx, kIdx))

	return zEntry{
		zCol:   zCol,
		packed: packed,
		zFinal: zFinal,
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
	entries []zEntry
}

// Run implements [wiop.ProverAction].
func (a *proverAction) Run(rt wiop.Runtime) {
	var total field.Ext

	for _, e := range a.entries {
		n := e.zCol.Module.RuntimeSize(rt)
		z := computeFilteredPrefixSum(rt, e.packed, n)

		rt.AssignColumn(e.zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(z)})

		// zFinal is a lazy opening of Z[n-1]; it resolves from this column
		// assignment on first read (or at round advance), so no explicit
		// assignment is needed here.

		total.Add(&total, &z[n-1])
	}

	if !rt.HasCellAssignment(a.ld.Result) {
		rt.AssignCell(a.ld.Result, field.ElemFromExt(total))
	}
}

// computeFilteredPrefixSum returns the running-sum
//
//	Z[i] = Σ_{k≤i, j} F_j[k] · N_j[k] / D_j[k]
//
// over the rows of a packed fraction group, skipping rows where the
// fraction's filter is zero. Each fraction's denominator is batch-inverted
// once; the inverse is consulted only at active rows so a zero denominator at
// a filtered-out row is benign.
//
// Panics if a fraction's denominator is zero on a row where its filter is
// non-zero, since that input is malformed.
func computeFilteredPrefixSum(rt wiop.Runtime, packed []wiop.Fraction, n int) []field.Ext {
	type evalFrac struct {
		filter []field.Ext // nil ⇒ filter is the constant 1 on every row
		num    []field.Ext
		den    []field.Ext
		invDen []field.Ext
	}
	fracs := make([]evalFrac, len(packed))
	for j, p := range packed {
		fracs[j].num = evaluateAsExtVec(rt, p.Numerator, n)
		fracs[j].den = evaluateAsExtVec(rt, p.Denominator, n)
		// BatchInvertExt silently leaves zero entries as zero; safe to call on
		// vectors that contain zeros at filtered-out rows.
		fracs[j].invDen = field.BatchInvertExt(fracs[j].den)
		if p.Filter != nil {
			fracs[j].filter = evaluateAsExtVec(rt, p.Filter, n)
		}
	}

	z := make([]field.Ext, n)
	var running, term field.Ext
	for i := 0; i < n; i++ {
		for j := range fracs {
			if fracs[j].filter != nil && fracs[j].filter[i].IsZero() {
				continue
			}
			if fracs[j].den[i].IsZero() {
				panic(fmt.Sprintf(
					"wiop/compilers/logderivativesum: zero denominator at row %d for fraction %d "+
						"with non-zero filter; the filter must mask this row",
					i, j,
				))
			}
			term.Mul(&fracs[j].num[i], &fracs[j].invDen[i])
			if fracs[j].filter != nil {
				term.Mul(&term, &fracs[j].filter[i])
			}
			running.Add(&running, &term)
		}
		z[i] = running
	}
	return z
}

// evaluateAsExtVec evaluates expr against the runtime and returns a length-n
// extension-field slice. Scalar expressions are broadcast to every position.
func evaluateAsExtVec(rt wiop.Runtime, expr wiop.Expression, n int) []field.Ext {
	out := make([]field.Ext, n)

	if !expr.IsMultiValued() {
		ext := genToExt(expr.EvaluateSingle(rt).Value)
		for i := range out {
			out[i] = ext
		}
		return out
	}

	cv := expr.EvaluateVector(rt)
	plain := cv.Plain
	if plain.IsBase() {
		base := plain.AsBase()
		copyLen := len(base)
		if copyLen > n {
			copyLen = n
		}
		for i := 0; i < copyLen; i++ {
			out[i] = field.Lift(base[i])
		}
		if copyLen < n {
			pad := field.Lift(cv.Padding)
			for i := copyLen; i < n; i++ {
				out[i] = pad
			}
		}
		return out
	}

	ext := plain.AsExt()
	copyLen := len(ext)
	if copyLen > n {
		copyLen = n
	}
	copy(out[:copyLen], ext[:copyLen])
	if copyLen < n {
		pad := field.Lift(cv.Padding)
		for i := copyLen; i < n; i++ {
			out[i] = pad
		}
	}
	return out
}

// genToExt projects a [field.Gen] onto its extension representation.
func genToExt(v field.Gen) field.Ext {
	if v.IsBase() {
		return field.Lift(v.AsBase())
	}
	return v.AsExt()
}

// verifierAction enforces the only boundary identity that is not already
// pinned in-circuit: the sum of all Z[n-1] endpoint openings equals the
// claimed Result cell value. The per-Z initial condition is enforced by the
// row-0 local constraint registered in buildZ, so this action reads only
// local openings (cells) — never the oracle witness columns.
type verifierAction struct {
	ld      *wiop.LogDerivativeSum
	entries []zEntry
}

// Check implements [wiop.VerifierAction].
func (a *verifierAction) Check(rt wiop.Runtime) error {
	var sum field.Ext

	for _, e := range a.entries {
		zFinal := genToExt(rt.GetCellValue(e.zFinal))
		sum.Add(&sum, &zFinal)
	}

	claimed := genToExt(rt.GetCellValue(a.ld.Result))
	var diff field.Ext
	diff.Sub(&sum, &claimed)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/logderivativesum: final-sum check failed for query %q",
			a.ld.Context().Path(),
		)
	}
	return nil
}

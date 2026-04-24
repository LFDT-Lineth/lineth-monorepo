package wiop

import (
	"fmt"
	"math/bits"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
)

// LagrangeEval is a [Query] that evaluates a batch of polynomials at a single
// point X and asserts that the results match a set of prover-supplied cells
// (the EvaluationClaims). The polynomials are represented in Lagrange basis
// over the roots-of-unity domain.
//
// For a column C of size n and evaluation point X, the evaluation is:
//
//	(X^n − 1) / n · Σ_{i<n} C[i] / (X − ω^i)
//
// where ω is the canonical n-th root of unity of the ambient field.
//
// LagrangeEval is both [GnarkCheckableQuery] and [AssignableQuery]:
//   - Check and CheckGnark assert that all polynomials and the evaluation point
//     are publicly visible, then verify that the claims match the evaluations.
//   - SelfAssign computes the evaluations from the runtime assignment and
//     stores them in the EvaluationClaims cells.
//
// An important invariant: the round of the EvaluationPoint must be strictly
// later than the round of every polynomial column. The default constructor
// [System.NewLagrangeEval] enforces this automatically.
type LagrangeEval struct {
	baseQuery
	// Polynomials is the ordered list of column views to evaluate.
	// All views must reference columns from the same module.
	Polynomials []*ColumnView
	// EvaluationPoint is the symbolic scalar at which the polynomials are
	// evaluated. It is typically a verifier coin or a public cell.
	EvaluationPoint FieldPromise
	// EvaluationClaims is the parallel list of cells holding the alleged
	// evaluations. EvaluationClaims[i] is the prover's claim for Polynomials[i].
	EvaluationClaims []*Cell
}

// Round implements [Query]. Returns the round of the EvaluationPoint, which
// by the construction invariant is the latest round among all referenced
// objects.
//
// Panics if the EvaluationPoint does not carry a round (i.e. it is not a
// *Cell or *CoinField).
func (le *LagrangeEval) Round() *Round {
	r := roundOf(le.EvaluationPoint)
	if r == nil {
		panic("wiop: LagrangeEval.Round: EvaluationPoint has no associated round; it must be a *Cell or *CoinField")
	}
	return r
}

// IsAlreadyAssigned implements [AssignableQuery]. Reports whether all
// EvaluationClaims cells already hold a runtime assignment.
func (le *LagrangeEval) IsAlreadyAssigned(rt Runtime) bool {
	for _, claim := range le.EvaluationClaims {
		if !rt.HasCellValue(claim) {
			return false
		}
	}
	return true
}

// SelfAssign implements [AssignableQuery]. Evaluates each polynomial at the
// EvaluationPoint and writes the results into the corresponding
// EvaluationClaims cells.
func (le *LagrangeEval) SelfAssign(rt Runtime) {
	evals := le.evalPolynomials(rt)
	for i, claim := range le.EvaluationClaims {
		rt.AssignCell(claim, evals[i])
	}
}

// Check implements [Query]. For each polynomial in Polynomials, evaluates it
// at EvaluationPoint using the barycentric Lagrange formula and asserts that
// the result equals the corresponding EvaluationClaims cell.
//
// Precondition: every polynomial column must be assigned in rt. The method
// returns a descriptive error for the first misassigned column or failing
// claim rather than panicking, so callers can surface the problem cleanly.
func (le *LagrangeEval) Check(rt Runtime) error {

	// Verify that all polynomial columns have been assigned in the runtime.
	for i, pv := range le.Polynomials {
		if !rt.HasColumnAssignment(pv.Column) {
			return fmt.Errorf(
				"wiop: LagrangeEval(%s): polynomial[%d] column %q is not assigned",
				le.context.Path(), i, pv.Column.Context.Path(),
			)
		}
	}

	for i, got := range le.evalPolynomials(rt) {
		claimCell := le.EvaluationClaims[i]
		if !rt.HasCellValue(claimCell) {
			return fmt.Errorf(
				"wiop: LagrangeEval(%s): claim cell[%d] %q is not assigned",
				le.context.Path(), i, claimCell.Context.Path(),
			)
		}
		claim := rt.GetCellValue(claimCell)
		diff := got.Sub(claim)
		if !diff.IsZero() {
			return fmt.Errorf(
				"wiop: LagrangeEval(%s): polynomial[%d] evaluation mismatch at claim cell %q",
				le.context.Path(), i, claimCell.Context.Path(),
			)
		}
	}

	return nil
}

// evalPolynomials evaluates each polynomial in le.Polynomials at the
// EvaluationPoint, applying the cyclic-shift adjustment for each [ColumnView].
// It is the shared kernel used by both [Check] and [SelfAssign].
func (le *LagrangeEval) evalPolynomials(rt Runtime) []field.Gen {
	evalPoint := le.EvaluationPoint.EvaluateSingle(rt)
	results := make([]field.Gen, len(le.Polynomials))
	for i, pv := range le.Polynomials {
		// Adjust the evaluation point for the column view's cyclic shift.
		// C'[j] = C[(j+k) mod n]  implies  C'(z) = C(ω^k · z),
		// so we evaluate the original column data at ω^k · z instead.
		z := evalPoint.Value
		if k := pv.ShiftingOffset; k != 0 {
			var (
				n      = pv.Column.Module.RuntimeSize(rt)
				omega  = field.RootOfUnityBy(n)
				omegaK field.Element
			)
			omegaK.ExpInt64(omega, int64(k))
			z = z.Mul(field.ElemFromBase(omegaK))
		}
		results[i] = evalLagrangePadded(rt.GetColumnAssignment(pv.Column), pv.Column.Module, rt, z)
	}
	return results
}

// CheckGnark implements [GnarkCheckableQuery]. Asserts inside a gnark circuit
// that each polynomial evaluates to the claimed value at the EvaluationPoint.
//
// Precondition: all polynomials and the EvaluationPoint must be publicly
// visible; panics otherwise.
func (le *LagrangeEval) CheckGnark(_ frontend.API, _ GnarkRuntime) {
	panic("wiop: LagrangeEval.CheckGnark not yet implemented")
}

// NewLagrangeEval constructs and registers a [LagrangeEval] query on sys.
// One fresh [Cell] is allocated automatically for each polynomial, placed in
// the round immediately following the latest polynomial column's round.
//
// The evaluation point x must carry a round (i.e. be a *Cell or *CoinField).
// The invariant that x's round is ≥ the claim cells' round is the caller's
// responsibility.
//
// Panics if ctx is nil, polys is empty, no round follows the latest column
// round, or x does not carry a round.
func (sys *System) NewLagrangeEval(ctx *ContextFrame, polys []*ColumnView, x FieldPromise) *LagrangeEval {
	if ctx == nil {
		panic("wiop: System.NewLagrangeEval requires a non-nil ContextFrame")
	}
	if len(polys) == 0 {
		panic("wiop: System.NewLagrangeEval requires at least one polynomial")
	}

	// Identify the latest round among the polynomial columns.
	var maxColRound *Round
	for _, pv := range polys {
		r := pv.Round()
		if maxColRound == nil || r.ID > maxColRound.ID {
			maxColRound = r
		}
	}

	// The EvaluationClaims live in the round immediately following the latest
	// column round. This round must already exist in the system; NewLagrangeEval
	// does not create new rounds.
	claimRound, ok := maxColRound.Next()
	if !ok {
		panic(fmt.Sprintf(
			"wiop: System.NewLagrangeEval: no round follows round %d (the latest polynomial column round); "+
				"call sys.NewRound() before registering this query",
			maxColRound.ID,
		))
	}

	// Allocate one claim cell per polynomial. The cell inherits the extension
	// status of its corresponding column view.
	claims := make([]*Cell, len(polys))
	for i, pv := range polys {
		cellCtx := ctx.Childf("claim[%d]", i)
		claims[i] = claimRound.NewCell(cellCtx, pv.IsExtension())
	}

	return sys.newLagrangeEval(ctx, polys, x, claims)
}

// NewLagrangeEvalFrom constructs and registers a [LagrangeEval] query on sys,
// using caller-supplied EvaluationClaims instead of freshly allocated ones.
// This is used when the claim cells already exist (e.g. when two queries share
// the same set of alleged evaluations).
//
// Panics if ctx is nil, polys is empty, or len(claims) != len(polys).
func (sys *System) NewLagrangeEvalFrom(ctx *ContextFrame, polys []*ColumnView, x FieldPromise,
	claims []*Cell) *LagrangeEval {
	if ctx == nil {
		panic("wiop: System.NewLagrangeEvalFrom requires a non-nil ContextFrame")
	}
	if len(polys) == 0 {
		panic("wiop: System.NewLagrangeEvalFrom requires at least one polynomial")
	}
	if len(claims) != len(polys) {
		panic(fmt.Sprintf(
			"wiop: System.NewLagrangeEvalFrom: claim count (%d) must equal polynomial count (%d)",
			len(claims), len(polys),
		))
	}
	return sys.newLagrangeEval(ctx, polys, x, claims)
}

// newLagrangeEval is the shared constructor used by [System.NewLagrangeEval]
// and [System.NewLagrangeEvalFrom]. It builds the [LagrangeEval], appends it
// to sys.LagrangeEvals, and returns it.
func (sys *System) newLagrangeEval(ctx *ContextFrame, polys []*ColumnView, x FieldPromise,
	claims []*Cell) *LagrangeEval {
	le := &LagrangeEval{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Polynomials:      polys,
		EvaluationPoint:  x,
		EvaluationClaims: claims,
	}
	sys.LagrangeEvals = append(sys.LagrangeEvals, le)
	return le
}

// evalLagrangePadded evaluates the polynomial encoded in cv (Lagrange basis
// over the n-point subgroup, with the module's padding semantics) at z.
//
// For PaddingDirectionNone it delegates to polynomials.EvalLagrange on the
// already-full-sized Plain[0]. For directional padding it accumulates the
// barycentric sum directly from Plain[0], treating padding and data rows
// separately with a single shared batch of denominator inverses. This avoids
// materialising the full n-length padded data vector.
func evalLagrangePadded(cv *ConcreteVector, m *Module, rt Runtime, z field.Gen) field.Gen {
	data := cv.Plain[0]
	if m.Padding == PaddingDirectionNone {
		return polynomials.EvalLagrange(data, z)
	}

	n := m.RuntimeSize(rt)
	plainLen := data.Len()
	gen := field.RootOfUnityBy(n)

	// dataStart is the index of the first data row in the padded domain.
	// Rows [0, dataStart) are padding for Left; rows [plainLen, n) for Right.
	var dataStart int
	if m.Padding == PaddingDirectionLeft {
		dataStart = n - plainLen
	}

	switch {
	case data.IsBase() && z.IsBase():
		return field.ElemFromBase(
			evalLagrangePaddedBaseBase(n, data.AsBase(), cv.Padding, z.AsBase(), gen, dataStart, plainLen),
		)
	case data.IsBase():
		return field.ElemFromExt(
			evalLagrangePaddedBaseExt(n, data.AsBase(), cv.Padding, z.AsExt(), gen, dataStart, plainLen),
		)
	case z.IsBase():
		return field.ElemFromExt(
			evalLagrangePaddedExtBase(n, data.AsExt(), cv.Padding, z.AsBase(), gen, dataStart, plainLen),
		)
	default:
		return field.ElemFromExt(
			evalLagrangePaddedExtExt(n, data.AsExt(), cv.Padding, z.AsExt(), gen, dataStart, plainLen),
		)
	}
}

// evalLagrangePaddedBaseBase: data in 𝔽_p (partial), padding in 𝔽_p, z in 𝔽_p → result in 𝔽_p.
func evalLagrangePaddedBaseBase(n int, data []field.Element, pad field.Element,
	z, gen field.Element, dataStart, plainLen int) field.Element {

	tb := bits.TrailingZeros(uint(n))
	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)
	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Element, n)
	denom := make([]field.Element, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		if j := i - dataStart; j >= 0 && j < plainLen {
			weightedP[i].Mul(&wi, &data[j])
		} else {
			weightedP[i].Mul(&wi, &pad)
		}
		denom[i].Sub(&z, &accOmega)
		accOmega.Mul(&accOmega, &gen)
	}
	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Element
	for i := 0; i < n; i++ {
		var term field.Element
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}
	var result field.Element
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// evalLagrangePaddedBaseExt: data in 𝔽_p, padding in 𝔽_p, z in 𝔽_{p^4} → result in 𝔽_{p^4}.
func evalLagrangePaddedBaseExt(n int, data []field.Element, pad field.Element,
	z field.Ext, gen field.Element, dataStart, plainLen int) field.Ext {

	tb := bits.TrailingZeros(uint(n))
	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)
	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	// weightedP stays in 𝔽_p: data, padding, and ω^i/n are all base-field.
	weightedP := make([]field.Element, n)
	denom := make([]field.Ext, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		if j := i - dataStart; j >= 0 && j < plainLen {
			weightedP[i].Mul(&wi, &data[j])
		} else {
			weightedP[i].Mul(&wi, &pad)
		}
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &accOmega) // z - ω^i
		accOmega.Mul(&accOmega, &gen)
	}
	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&invDenom[i], &weightedP[i])
		sum.Add(&sum, &term)
	}
	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// evalLagrangePaddedExtBase: data in 𝔽_{p^4}, padding in 𝔽_p, z in 𝔽_p → result in 𝔽_{p^4}.
func evalLagrangePaddedExtBase(n int, data []field.Ext, pad field.Element,
	z, gen field.Element, dataStart, plainLen int) field.Ext {

	tb := bits.TrailingZeros(uint(n))
	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)
	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Ext, n)
	denom := make([]field.Element, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		if j := i - dataStart; j >= 0 && j < plainLen {
			weightedP[i].MulByElement(&data[j], &wi)
		} else {
			// Padding value is base-field; lift to 𝔽_{p^4}.
			var padWeighted field.Element
			padWeighted.Mul(&wi, &pad)
			weightedP[i] = field.Lift(padWeighted)
		}
		denom[i].Sub(&z, &accOmega)
		accOmega.Mul(&accOmega, &gen)
	}
	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}
	var result field.Ext
	result.MulByElement(&sum, &zPowNMinusOne)
	return result
}

// evalLagrangePaddedExtExt: data in 𝔽_{p^4}, padding in 𝔽_p, z in 𝔽_{p^4} → result in 𝔽_{p^4}.
func evalLagrangePaddedExtExt(n int, data []field.Ext, pad field.Element,
	z field.Ext, gen field.Element, dataStart, plainLen int) field.Ext {

	tb := bits.TrailingZeros(uint(n))
	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)
	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Ext, n)
	denom := make([]field.Ext, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		if j := i - dataStart; j >= 0 && j < plainLen {
			weightedP[i].MulByElement(&data[j], &wi)
		} else {
			var padWeighted field.Element
			padWeighted.Mul(&wi, &pad)
			weightedP[i] = field.Lift(padWeighted)
		}
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &accOmega) // z - ω^i
		accOmega.Mul(&accOmega, &gen)
	}
	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}
	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

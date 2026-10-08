package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
)

// GrandProduct is a [Query] that reduces two lists of vector-valued
// expressions — Numerators and Denominators — to a single field-element
// result stored in a [Cell]:
//
//	Result = ( ∏_k ∏_row Numerator_k[row] ) / ( ∏_k ∏_row Denominator_k[row] )
//
// It is the target of the permutation argument: a [TableRelationQuery] of kind
// [KindPermutation] is reduced (by the grandproduct compiler) to a
// GrandProduct whose Result the verifier then constrains to be one. The
// product is taken over every row of every factor's module, padding rows
// included, so a permutation that holds over the padded domains yields
// Result == 1.
//
// Numerators and Denominators need not be paired or of equal length: because
// the value is a single ratio of two flat products, the grouping of factors is
// irrelevant to the semantics. The grandproduct compiler regroups them by
// module when it builds the running-product Z columns.
//
// GrandProduct implements [AssignableQuery] but not [GnarkCheckableQuery]: a
// compiler pass must reduce it before gnark verification.
//
// Use [System.NewGrandProduct] to construct and register an instance.
type GrandProduct struct {
	baseQuery
	// Numerators is the list of vector-valued factor expressions contributing
	// to the numerator product. May be empty (an empty product is one).
	Numerators []Expression
	// Denominators is the list of vector-valued factor expressions
	// contributing to the denominator product. May be empty.
	Denominators []Expression
	// Result is the cell holding the prover's claimed grand-product value.
	// Allocated automatically by the constructor.
	Result *Cell
}

// Round implements [Query]. Returns the round of the [Result] cell, which is
// the round immediately following the latest column/coin round across every
// numerator and denominator expression.
func (gp *GrandProduct) Round() *Round { return gp.Result.Round() }

// IsAlreadyAssigned implements [AssignableQuery]. Reports whether the Result
// cell already holds a runtime assignment.
func (gp *GrandProduct) IsAlreadyAssigned(rt *Runtime) bool {
	return rt.HasCellAssignment(gp.Result)
}

// SelfAssign implements [AssignableQuery]. Computes the grand product from the
// runtime column assignments and writes it into Result.
func (gp *GrandProduct) SelfAssign(rt *Runtime) {
	rt.AssignCell(gp.Result, gp.compute(rt))
}

// Check implements [Query]. Verifies that the Result cell holds the correct
// grand-product value. Returns an error if the claimed Result does not match.
func (gp *GrandProduct) Check(rt *Runtime) error {
	got := rt.GetCellValue(gp.Result)
	want := gp.compute(rt)
	diff := want.Sub(got)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop: GrandProduct.Check(%s): result mismatch",
			gp.context.Path(),
		)
	}
	return nil
}

// compute evaluates ( ∏ numerator entries ) / ( ∏ denominator entries ) over
// every row of every factor, padding rows included. It is the shared core of
// [SelfAssign] and [Check]. Panics if the denominator product is zero. The
// result is base-field when every factor is.
func (gp *GrandProduct) compute(rt *Runtime) field.Gen {
	num, numBase := productOverRows(rt, gp.Numerators)
	den, denBase := productOverRows(rt, gp.Denominators)
	if den.IsZero() {
		panic(fmt.Sprintf("wiop: GrandProduct.compute(%s): zero denominator", gp.context.Path()))
	}
	var res field.Ext
	res.Inverse(&den)
	res.Mul(&num, &res)
	if numBase && denBase {
		return field.ElemFromBase(res.B0.A0)
	}
	return field.ElemFromExt(res)
}

// productOverRows returns the product of every row of every factor, and
// whether every factor is base-field. The factors of each module are
// multiplied row by row by one [vecprog] program, then the rows are
// multiplied in parallel chunks; field multiplication is exact, so the order
// does not matter.
func productOverRows(rt *Runtime, factors []Expression) (field.Ext, bool) {
	var prod field.Ext
	prod.SetOne()
	allBase := true
	byRows := make(map[int][]Expression)
	var sizes []int
	for _, e := range factors {
		allBase = allBase && !e.IsExtension()
		n := factorRows(rt, e)
		if _, ok := byRows[n]; !ok {
			sizes = append(sizes, n)
		}
		byRows[n] = append(byRows[n], e)
	}
	for _, n := range sizes {
		l := NewRowLowering(rt, n)
		root := -1
		for _, e := range byRows[n] {
			id := l.Lower(e)
			if root < 0 {
				root = id
			} else {
				root = l.B.Op(vecprog.Mul, root, id)
			}
		}
		rows := make([]field.Ext, n)
		l.B.Store(root, rows)
		l.B.Compile(n, 1, 1).Run(nil, nil, 0)
		p := parallelProduct(rows)
		prod.Mul(&prod, &p)
	}
	return prod, allBase
}

// factorRows returns the number of rows of a vector-valued factor: its
// module's size, or the length of its evaluation when it has no module.
func factorRows(rt *Runtime, e Expression) int {
	if m := e.Module(); m != nil {
		return m.RuntimeSize(rt)
	}
	return e.EvaluateVector(rt).Plain.Len()
}

// parallelProduct returns the product of v, computed in parallel chunks.
func parallelProduct(v []field.Ext) field.Ext {
	const chunk = 1 << 12
	partials := make([]field.Ext, (len(v)+chunk-1)/chunk)
	parallel.Execute(len(partials), func(start, end int) {
		for c := start; c < end; c++ {
			var acc field.Ext
			acc.SetOne()
			for i := c * chunk; i < min((c+1)*chunk, len(v)); i++ {
				acc.Mul(&acc, &v[i])
			}
			partials[c] = acc
		}
	})
	var prod field.Ext
	prod.SetOne()
	for i := range partials {
		prod.Mul(&prod, &partials[i])
	}
	return prod
}

// NewGrandProduct constructs and registers a [GrandProduct] query on sys. A
// fresh [Cell] is allocated automatically for the result, placed in the round
// immediately following the latest column/coin round across all numerator and
// denominator expressions.
//
// Invariants enforced at construction:
//   - At least one factor is supplied (len(numerators)+len(denominators) ≥ 1).
//   - Every factor is vector-valued (IsMultiValued() == true).
//
// Panics if ctx is nil, any invariant is violated, or no round follows the
// latest factor round (call [System.NewRound] first in that case).
func (sys *System) NewGrandProduct(ctx *ContextFrame, numerators, denominators []Expression) *GrandProduct {
	if ctx == nil {
		panic("wiop: System.NewGrandProduct requires a non-nil ContextFrame")
	}
	if len(numerators)+len(denominators) == 0 {
		panic("wiop: System.NewGrandProduct requires at least one factor")
	}

	var maxRound *Round
	isExt := false
	for side, factors := range [2][]Expression{numerators, denominators} {
		for i, e := range factors {
			if e == nil {
				panic(fmt.Sprintf("wiop: System.NewGrandProduct: factor [side=%d,%d] is nil", side, i))
			}
			if !e.IsMultiValued() {
				panic(fmt.Sprintf(
					"wiop: System.NewGrandProduct: factor [side=%d,%d] is not vector-valued",
					side, i,
				))
			}
			if r := maxRoundInExpr(e); r != nil && (maxRound == nil || r.ID > maxRound.ID) {
				maxRound = r
			}
			if e.IsExtension() {
				isExt = true
			}
		}
	}

	if maxRound == nil {
		panic("wiop: System.NewGrandProduct: no column or coin found in any factor; " +
			"at least one factor must be round-bearing")
	}

	resultRound, ok := maxRound.Next()
	if !ok {
		panic(fmt.Sprintf(
			"wiop: System.NewGrandProduct: no round follows round %d (the latest factor round); "+
				"call sys.NewRound() before registering this query",
			maxRound.ID,
		))
	}

	result := resultRound.NewCell(ctx.Childf("result"), isExt)
	gp := &GrandProduct{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Numerators:   numerators,
		Denominators: denominators,
		Result:       result,
	}
	sys.GrandProducts = append(sys.GrandProducts, gp)
	return gp
}

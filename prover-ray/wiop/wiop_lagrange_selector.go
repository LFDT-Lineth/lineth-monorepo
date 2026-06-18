package wiop

import (
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// LagrangeSelector is a special type of vector promises representing a column
// of the form (0, 0, 1, 0, 0); e.g. zero everywhere and one in single position.
// Such a column is used to lower local constraints into global constraints. It
// has the property that its Lagrange representation can be evaluated in
// logarithmic time by the verifier. Therefore, it is generally not useful
// committing to the column.
//
// The structure implements [VectorPromise] and
type LagrangeSelector struct {
	// Module is the module in which the selector is meant to be used as part
	// of global constraints.
	module   *Module
	Position int
}

// Compile-time check that *LagrangeSelector satisfies the [VectorPromise]
// (and thereby [Expression]) interface.
var _ VectorPromise = (*LagrangeSelector)(nil)

// NewLagrangeSelector returns a new LagrangeSelector that is 1 at the given
// row position and 0 elsewhere.
//
// Panics if position is negative, or — when the module is statically sized — if
// position is outside [0, module.Size()). For dynamic modules the upper bound
// cannot be checked at construction time and is enforced lazily by
// [LagrangeSelector.EvaluateVector] against the runtime size.
func NewLagrangeSelector(module *Module, position int) *LagrangeSelector {
	if position < 0 {
		panic(fmt.Sprintf("wiop: NewLagrangeSelector: position must be non-negative, got %d", position))
	}
	if module.IsSized() && position >= module.Size() {
		panic(fmt.Sprintf(
			"wiop: NewLagrangeSelector: position %d out of range [0, %d) for module %q",
			position, module.Size(), module.Context.Path(),
		))
	}
	return &LagrangeSelector{module: module, Position: position}
}

// IsExtension implements [VectorPromise]. Always returns false: a Lagrange
// selector is always base-field.
func (ls *LagrangeSelector) IsExtension() bool { return false }

// IsMultiValued implements [VectorPromise]. Always returns true: a Lagrange
// selector is always vector-valued.
func (ls *LagrangeSelector) IsMultiValued() bool { return true }

// Degree implements [VectorPromise]. Always returns 0: a Lagrange selector is
// a degree-0 vector.
func (ls *LagrangeSelector) Degree() int {
	return ls.Size() - 1
}

// DegreeFactor implements [VectorPromise].
func (ls *LagrangeSelector) DegreeFactor() int { return 1 }

// Size implements [VectorPromise].
func (ls *LagrangeSelector) Size() int {
	if ls.module.IsDynamic() {
		panic(fmt.Sprintf("wiop: Size() called on dynamic-sized module: %s", ls.module.Context.Path()))
	}
	if !ls.module.IsSized() {
		panic(fmt.Sprintf("wiop: Size() called on unsized module: %s", ls.module.Context.Path()))
	}
	return ls.module.Size()
}

// IsSized implements [VectorPromise].
func (ls *LagrangeSelector) IsSized() bool { return ls.module.IsSized() }

// Module implements [VectorPromise].
func (ls *LagrangeSelector) Module() *Module { return ls.module }

// Visibility implements [VectorPromise].
func (ls *LagrangeSelector) Visibility() Visibility { return VisibilityPublic }

// EvaluateSingle is a placeholder implementation of [VectorPromise].
func (ls *LagrangeSelector) EvaluateSingle(_ Runtime) ConcreteField {
	panic("wiop: EvaluateSingle() cannot be called on a VectorPromise")
}

// EvaluateVector returns the evaluation of the Lagrange selector.
func (ls *LagrangeSelector) EvaluateVector(rt Runtime) ConcreteVector {

	size := ls.module.RuntimeSize(rt)

	// @alex: we could use a better memory representation for this as this
	// used size * 4 bytes to represent a sequence of 0s with a 1 somewhere.
	res := ConcreteVector{
		promise: ls,
		Plain:   field.VecFromBase(make([]field.Element, size)),
		Padding: field.Zero(),
	}

	res.Plain.AsBase()[ls.Position] = field.One()
	return res
}

// EvaluateOutOfDomain evaluates the low-degree extension of the Lagrange
// selector at a point x lying outside the evaluation domain.
//
// The selector is the column (0, …, 0, 1, 0, …, 0) holding a single 1 at row
// Position. Its Lagrange interpolant over the n-th roots-of-unity domain is the
// Lagrange basis polynomial
//
//	L_Position(X) = ω^Position · (X^n − 1) / (n · (X − ω^Position))
//
// where ω is the canonical n-th root of unity and n is the module size. The
// result lies in the base field iff x does.
//
// Panics if x equals ω^Position (i.e. the point is in the domain at the
// selector's own row): the denominator X−ω^Position vanishes there, so the
// out-of-domain contract is violated.
func (ls *LagrangeSelector) EvaluateOutOfDomain(rt Runtime, x field.Gen) field.Gen {

	n := ls.module.RuntimeSize(rt)

	// omegaPos = ω^Position, the domain point at which the selector is 1.
	var omegaPos field.Element
	omegaPos.ExpInt64(field.RootOfUnityBy(n), int64(ls.Position))

	// numerator = ω^Position · (x^n − 1). Since n is a power of two, x^n is
	// computed by squaring log2(n) times.
	xPowN := x
	for i := 0; i < bits.TrailingZeros(uint(n)); i++ {
		xPowN = xPowN.Square()
	}
	numerator := xPowN.Sub(field.ElemOne()).Mul(field.ElemFromBase(omegaPos))

	// denominator = n · (x − ω^Position). Guard against an in-domain point
	// explicitly: the field defines 1/0 = 0, so without this check Div would
	// silently return 0 instead of signalling the contract violation.
	var nElem field.Element
	nElem.SetUint64(uint64(n))
	xMinusOmega := x.Sub(field.ElemFromBase(omegaPos))
	if xMinusOmega.IsZero() {
		panic(fmt.Sprintf(
			"wiop: LagrangeSelector.EvaluateOutOfDomain called at ω^%d, which is inside the domain",
			ls.Position,
		))
	}
	denominator := xMinusOmega.Mul(field.ElemFromBase(nElem))

	return numerator.Div(denominator)
}

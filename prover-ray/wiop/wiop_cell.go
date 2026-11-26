package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// Cell is a scalar-valued symbolic object representing a single field element
// that the prover assigns at runtime. It is the scalar counterpart to [Column]:
// where a column is a vector bound to a [Module], a cell stands alone and
// always has size 1.
//
// Cell implements [FieldPromise] and thereby [Expression]. The vector-oriented
// methods (Size, IsSized, EvaluateVector) panic because they have no meaning
// for a scalar value.
//
// Cells are declared via [Round.NewCell], not constructed directly.
type Cell struct {
	// Context identifies this cell in the protocol hierarchy.
	Context *ContextFrame
	// Annotations holds arbitrary metadata attached to this cell.
	Annotations Annotations
	// isExtensionCached stores whether this cell is associated with an
	// extended domain. Set once at construction and never mutated.
	isExtensionCached bool
	// round is the owning Round. Set once at construction, never nil for a
	// well-formed Cell.
	round *Round
	// Assigner, when non-nil, makes this a lazily-assigned cell: instead of the
	// prover writing the value through [Runtime.AssignCell], the value is
	// computed on demand by calling Assigner with the runtime. The runtime
	// resolves it the first time the cell is read ([Runtime.GetCellValue]) or,
	// at the latest, when its round is folded into the transcript
	// ([Runtime.AdvanceRound]). The closure must only depend on data available
	// in the cell's own round (e.g. a column assignment of that round).
	//
	// An explicit assignment always wins: if the cell is already assigned when
	// the runtime would resolve it, Assigner is not called.
	Assigner func(*Runtime) field.Gen
}

// Round returns the round in which this cell is committed. It is always
// non-nil for a well-formed Cell.
func (c *Cell) Round() *Round { return c.round }

// Module implements [Expression]. Always returns nil: a cell is scalar and
// not bound to any module.
func (c *Cell) Module() *Module { return nil }

// IsExtension implements [Expression]. Reports whether this cell is declared
// as an extension-field value. The declaration is binding: the runtime stores
// every value of the cell with this field tag (see [Cell.canonical]), so the
// tag of a proof value, and with it the number of field elements the cell
// absorbs into the transcript, is a property of the System, not of the prover.
func (c *Cell) IsExtension() bool { return c.isExtensionCached }

// canonical returns v tagged with the cell's declared field. A base value
// assigned to an extension cell is lifted, which is lossless. An extension
// value assigned to a base cell is an error, even if its extension
// coordinates happen to be zero: the declaration is wrong, or the value is.
func (c *Cell) canonical(v field.Gen) (field.Gen, error) {
	if c.isExtensionCached {
		if v.IsBase() {
			return field.ElemFromExt(v.AsExt()), nil
		}
		return v, nil
	}
	if !v.IsBase() {
		return v, fmt.Errorf("wiop: cell %q is declared base-field but holds an extension-field value",
			c.Context.Path())
	}
	return v, nil
}

// hasDeclaredTag reports whether v carries the field tag the cell declares.
func (c *Cell) hasDeclaredTag(v field.Gen) bool { return v.IsBase() != c.isExtensionCached }

// IsMultiValued implements [Expression]. Always returns false: a cell is
// always scalar.
func (c *Cell) IsMultiValued() bool { return false }

// Degree implements [Expression]. Always returns 0: a cell is a single field
// element, not a polynomial.
func (c *Cell) Degree() int { return 0 }

// DegreeFactor implements [Expression]. Always returns 0: a cell is a scalar
// constant.
func (c *Cell) DegreeFactor() int { return 0 }

// Size implements [Expression]. Panics unconditionally: size has no meaning
// for a scalar FieldPromise. Check IsMultiValued() before calling Size.
func (c *Cell) Size() int {
	panic("wiop: Size() cannot be called on a FieldPromise")
}

// IsSized implements [Expression]. Panics unconditionally: IsSized has no
// meaning for a scalar FieldPromise. Check IsMultiValued() before calling
// IsSized.
func (c *Cell) IsSized() bool {
	panic("wiop: IsSized() cannot be called on a FieldPromise")
}

// EvaluateVector implements [Expression]. Panics unconditionally: a cell is
// scalar and produces no vector. Check IsMultiValued() before calling
// EvaluateVector.
func (c *Cell) EvaluateVector(_ *Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression].
func (c *Cell) EvaluateSingle(rt *Runtime) ConcreteField {
	return ConcreteField{Value: rt.GetCellValue(c), promise: c}
}

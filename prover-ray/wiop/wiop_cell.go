package wiop

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
}

// Round returns the round in which this cell is committed. It is always
// non-nil for a well-formed Cell.
func (c *Cell) Round() *Round { return c.round }

// Module implements [Expression]. Always returns nil: a cell is scalar and
// not bound to any module.
func (c *Cell) Module() *Module { return nil }

// IsExtension implements [Expression]. Reports whether this cell is associated
// with an extended domain.
func (c *Cell) IsExtension() bool { return c.isExtensionCached }

// IsMultiValued implements [Expression]. Always returns false: a cell is
// always scalar.
func (c *Cell) IsMultiValued() bool { return false }

// Degree implements [Expression]. Always returns 0: a cell is a single field
// element, not a polynomial.
func (c *Cell) Degree() int { return 0 }

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
func (c *Cell) EvaluateVector(_ Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression].
func (c *Cell) EvaluateSingle(rt Runtime) ConcreteField {
	return ConcreteField{Value: rt.GetCellValue(c), promise: c}
}

// Visibility implements [Expression]. Always returns [VisibilityPublic]: cells
// are prover-supplied scalar openings that the verifier can observe directly.
func (c *Cell) Visibility() Visibility { return VisibilityPublic }

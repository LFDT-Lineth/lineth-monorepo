package wiop

// CoinField represents a random coin challenge drawn by the verifier during a
// protocol round. The sampled value is always an extension-field element
// (IsExtension always returns true) to provide sufficient soundness — sampling
// over the base field alone would not yield enough security margin for any
// relevant use-case.
//
// CoinField implements [FieldPromise] and thereby [Expression]. The
// vector-oriented methods (Size, IsSized, EvaluateVector) panic because they
// have no meaning for a scalar value.
//
// Coins are declared via [Round.NewCoinField], not constructed directly.
type CoinField struct {
	// Context identifies this coin in the protocol hierarchy.
	Context *ContextFrame
	// Annotations holds arbitrary metadata attached to this coin.
	Annotations Annotations
	// round is the owning Round. Set once at construction, never nil for a
	// well-formed CoinField.
	round *Round
}

// Round returns the round in which this coin is drawn. It is always non-nil
// for a well-formed CoinField.
func (cf *CoinField) Round() *Round { return cf.round }

// Module implements [Expression]. Always returns nil: a coin is scalar and
// not bound to any module.
func (cf *CoinField) Module() *Module { return nil }

// IsExtension implements [Expression]. Always returns true: coins are always
// sampled over a finite-field extension.
func (cf *CoinField) IsExtension() bool { return true }

// IsMultiValued implements [Expression]. Always returns false: a coin is a
// single field element.
func (cf *CoinField) IsMultiValued() bool { return false }

// Degree implements [Expression]. Always returns 0: a coin is a constant with
// respect to any polynomial evaluation.
func (cf *CoinField) Degree() int { return 0 }

// Size implements [Expression]. Panics unconditionally: size has no meaning
// for a scalar FieldPromise. Check IsMultiValued() before calling Size.
func (cf *CoinField) Size() int {
	panic("wiop: Size() cannot be called on a FieldPromise")
}

// IsSized implements [Expression]. Panics unconditionally: IsSized has no
// meaning for a scalar FieldPromise. Check IsMultiValued() before calling
// IsSized.
func (cf *CoinField) IsSized() bool {
	panic("wiop: IsSized() cannot be called on a FieldPromise")
}

// EvaluateVector implements [Expression]. Panics unconditionally: a coin is
// scalar and produces no vector. Check IsMultiValued() before calling
// EvaluateVector.
func (cf *CoinField) EvaluateVector(_ Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression].
func (cf *CoinField) EvaluateSingle(rt Runtime) ConcreteField {
	return ConcreteField{Value: rt.GetCoinValue(cf), promise: cf}
}

// Visibility implements [Expression]. Always returns [VisibilityPublic]: coins
// are drawn by the verifier and are therefore always verifier-visible.
func (cf *CoinField) Visibility() Visibility { return VisibilityPublic }

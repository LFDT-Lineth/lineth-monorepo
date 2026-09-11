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
	// seeded marks this coin as drawn from the Fiat-Shamir state installed by
	// its round's [Round.PreSamplingHook] rather than from the running
	// transcript. See [CoinField.MarkSeeded].
	seeded bool
}

// Round returns the round in which this coin is drawn. It is always non-nil
// for a well-formed CoinField.
func (cf *CoinField) Round() *Round { return cf.round }

// MarkSeeded opts this coin into seeded sampling: [Runtime.AdvanceRound] draws
// it from the Fiat-Shamir state its round's [Round.PreSamplingHook] installs,
// then restores the pre-hook transcript before drawing the round's remaining
// coins. Marking per coin is what lets a round be shared — several compiler
// passes anchor their coin round at "one past the last witness round" and reuse
// whatever sits there, so a pass needing a seeded challenge lands on the same
// round as one that must stay bound to its own commitments.
//
// The marking chooses whether a coin is seeded, not which seed it gets: a round
// has one hook and therefore one seed, and every coin marked on that round is
// drawn from it, consecutively (staying distinct through the usual safeguard
// update).
//
// Call at compile time only. A round carrying a seeded coin must also carry a
// pre-sampling hook to install the seed; [Runtime.AdvanceRound] panics otherwise.
func (cf *CoinField) MarkSeeded() { cf.seeded = true }

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

// DegreeFactor implements [Expression]. Always returns 0: a coin is a scalar
// constant.
func (cf *CoinField) DegreeFactor() int { return 0 }

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
func (cf *CoinField) EvaluateVector(_ *Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression].
func (cf *CoinField) EvaluateSingle(rt *Runtime) ConcreteField {
	return ConcreteField{Value: rt.GetCoinValue(cf), promise: cf}
}

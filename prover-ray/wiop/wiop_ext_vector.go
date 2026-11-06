package wiop

// ExtVector is a vector of extension elements that the prover hands to the
// verifier in one round outside the Fiat-Shamir transcript. A prover action sets
// it with [Runtime.SetExtVector], [System.Prove] carries it in
// [Proof.ExtVectors], and verifier actions read it with [Runtime.GetExtVector].
//
// Nothing absorbs it into the transcript and nothing in [System.Verify] checks
// its content: a verifier action that reads one must bind it to what the
// transcript does absorb. Its length is not declared.
//
// Like cells, a declared ExtVector must be in the proof, and no other may be.
//
// ExtVectors are declared via [Round.NewExtVector], not constructed directly.
type ExtVector struct {
	// Context identifies this vector in the protocol hierarchy.
	Context *ContextFrame
	// Annotations holds arbitrary metadata attached to this vector.
	Annotations Annotations
	// round is the owning Round. Set once at construction, never nil for a
	// well-formed ExtVector.
	round *Round
}

// Round returns the round in which this vector is set. It is always non-nil for
// a well-formed ExtVector.
func (v *ExtVector) Round() *Round { return v.round }

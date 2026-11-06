package wiop_test

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// extVectorChecked is the first element of the vector that extVectorVerifier checks.
func extVectorChecked() (e field.Ext) {
	e.B0.A0.SetUint64(7)
	return
}

// extVectorProver sets a vector of two extension elements.
type extVectorProver struct{ vec *wiop.ExtVector }

func (a extVectorProver) Run(rt *wiop.Runtime) {
	rt.SetExtVector(a.vec, []field.Ext{extVectorChecked(), {}})
}

// extVectorVerifier checks the first element of the vector and records the coin of its round, which
// the verifier derives from the transcript.
type extVectorVerifier struct {
	vec  *wiop.ExtVector
	coin *wiop.CoinField
	seen *field.Gen
}

func (a extVectorVerifier) Check(rt *wiop.Runtime) error {
	*a.seen = rt.GetCoinValue(a.coin)
	v, ok := rt.GetExtVector(a.vec)
	if !ok || len(v) != 2 {
		return errors.New("the vector is missing")
	}
	if v[0] != extVectorChecked() {
		return errors.New("the first element of the vector is wrong")
	}
	return nil
}

// newExtVectorSystem returns a system of two rounds: a vector declared in the first, which a prover
// action sets, and a coin in the second, so that a vector that the transcript absorbed would change
// the coin. If withVerifier, a verifier action reads the vector in the second round.
func newExtVectorSystem(withVerifier bool) (sys *wiop.System, vec *wiop.ExtVector, seen *field.Gen) {
	seen = new(field.Gen)
	sys = wiop.NewSystemf("ext-vector")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	vec = r0.NewExtVector(sys.Context.Childf("vector"))
	r0.RegisterAction(extVectorProver{vec})
	if withVerifier {
		r1.RegisterVerifierAction(extVectorVerifier{vec, coin, seen})
	}
	return
}

// TestExtVector checks that a vector set by a prover action reaches the verifier action that reads
// it, outside the transcript: nothing in it, not even what the verifier action ignores, changes the
// coin the verifier derives. The verifier fails only for what its action checks.
func TestExtVector(t *testing.T) {
	sys, vec, seen := newExtVectorSystem(true)

	proof, pub := sys.Prove(func(*wiop.Runtime) {})
	require.Contains(t, proof.ExtVectors, vec.Context.ID)
	require.NoError(t, sys.Verify(proof, pub))
	honestCoin := *seen

	// An element that the verifier action does not check changes nothing it derives.
	var other field.Ext
	other.B2.A1.SetUint64(11)
	tampered := proof
	tampered.ExtVectors = map[wiop.ObjectID][]field.Ext{vec.Context.ID: {extVectorChecked(), other}}
	require.NoError(t, sys.Verify(tampered, pub))
	require.Equal(t, honestCoin, *seen)

	// One that it checks is rejected.
	tampered.ExtVectors = map[wiop.ObjectID][]field.Ext{vec.Context.ID: {other, {}}}
	require.Error(t, sys.Verify(tampered, pub))
}

// TestExtVector_Declared checks that, as for cells, the proof must carry the declared vectors and no
// others, whether or not a verifier action reads them.
func TestExtVector_Declared(t *testing.T) {
	sys, vec, _ := newExtVectorSystem(false)
	proof, pub := sys.Prove(func(*wiop.Runtime) {})
	require.NoError(t, sys.Verify(proof, pub))

	missing := proof
	missing.ExtVectors = nil
	require.ErrorContains(t, sys.Verify(missing, pub), "not found in proof")

	// An ID that is no vector of the system: a vector beyond the declared ones, and an object of
	// another kind.
	for _, extra := range []wiop.ObjectID{vec.Context.ID + 1, sys.Rounds[1].Coins[0].Context.ID} {
		extended := proof
		extended.ExtVectors = map[wiop.ObjectID][]field.Ext{vec.Context.ID: proof.ExtVectors[vec.Context.ID], extra: nil}
		require.ErrorContains(t, sys.Verify(extended, pub), "not found in system")
	}
}

// TestExtVector_SetAndGet checks the prover's side: a declared vector must be set, once, in its
// round, and a system that declares none has none in its proof.
func TestExtVector_SetAndGet(t *testing.T) {
	sys := wiop.NewSystemf("ext-vector-set")
	sys.NewRound()
	r1 := sys.NewRound()
	vec := r1.NewExtVector(sys.Context.Childf("vector"))

	require.Panics(t, func() { sys.Prove(func(*wiop.Runtime) {}) }, "a declared vector that no action set")

	rt := wiop.NewRuntime(sys)
	_, ok := rt.GetExtVector(vec)
	require.False(t, ok)
	require.Panics(t, func() { rt.SetExtVector(vec, nil) }, "set in another round")
	rt.AdvanceRound()
	rt.SetExtVector(vec, []field.Ext{{}})
	got, ok := rt.GetExtVector(vec)
	require.True(t, ok)
	require.Len(t, got, 1)
	require.Panics(t, func() { rt.SetExtVector(vec, nil) }, "set twice")

	none := wiop.NewSystemf("no-ext-vector")
	none.NewRound()
	proof, pub := none.Prove(func(*wiop.Runtime) {})
	require.Nil(t, proof.ExtVectors)
	require.NoError(t, none.Verify(proof, pub))
}

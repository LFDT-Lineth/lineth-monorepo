package fiatshamir_test

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

const (
	fsTestNumBase      = 5
	fsTestNumExt       = 3
	fsTestNumIntegers  = 11
	fsTestIntegerBound = 1 << 10
)

// fsReplayCircuit absorbs a fixed transcript and asserts that every sampled
// challenge matches the value the native transcript produced.
type fsReplayCircuit struct {
	Base     []circuit.Element
	Ext      []circuit.Ext
	Digest   poseidon2.KoalagnarkOctuplet
	Coin     circuit.Ext
	Integers []frontend.Variable
	Seeded   circuit.Ext
	Seed     poseidon2.KoalagnarkOctuplet
}

func (c *fsReplayCircuit) Define(api frontend.API) error {
	fs := fiatshamir.NewGnarkFiatShamir(api)
	kapi := fs.API()

	fs.Update(c.Base...)
	fs.UpdateExt(c.Ext...)
	digest := fs.RandomDigest()
	for i := range digest {
		kapi.AssertIsEqual(digest[i], c.Digest[i])
	}
	kapi.AssertIsEqualExt(fs.RandomFext(), c.Coin)

	ints := fs.RandomManyIntegers(fsTestNumIntegers, fsTestIntegerBound)
	for i := range ints {
		api.AssertIsEqual(ints[i], c.Integers[i])
	}

	// State round-trip: seeding the state and sampling must match the native
	// transcript, and a seeded transcript must not depend on prior absorptions.
	fs.SetState(c.Seed)
	kapi.AssertIsEqualExt(fs.RandomFext(), c.Seeded)
	return nil
}

// fsReplayCircuitTemplate returns an unassigned circuit with the witness shape.
// gnark writes into the struct during compilation, so every compilation needs
// a fresh template.
func fsReplayCircuitTemplate() *fsReplayCircuit {
	return &fsReplayCircuit{
		Base:     make([]circuit.Element, fsTestNumBase),
		Ext:      make([]circuit.Ext, fsTestNumExt),
		Integers: make([]frontend.Variable, fsTestNumIntegers),
	}
}

// fsReplayWitness builds the differential fixture: it runs the native
// transcript over a fixed input and returns a circuit assignment carrying both
// that input and every challenge the native transcript produced. The circuit
// absorbs the same input and asserts it derives the same challenges, so a
// solved circuit means the two transcripts agree step for step.
//
// The sequence of calls below must stay in lock-step with [fsReplayCircuit.Define].
// A Fiat-Shamir transcript is order-dependent by construction: absorbing the
// same values in a different order, or sampling a challenge the other side does
// not sample, changes every subsequent digest. Reordering one side alone turns
// this into a test that always fails; reordering both in the same way turns it
// into a test that passes while the circuit diverges from the real verifier.
func fsReplayWitness(t *testing.T) *fsReplayCircuit {
	t.Helper()
	// Fixed seed: the fixture must be reproducible, since a failure here is a
	// mismatch between the two implementations, not a property to fuzz.
	rng := rand.New(rand.NewPCG(7, 11))

	base := make([]field.Element, fsTestNumBase)
	ext := make([]field.Ext, fsTestNumExt)
	for i := range base {
		base[i] = field.PseudoRand(rng)
	}
	for i := range ext {
		ext[i] = field.PseudoRandExt(rng)
	}
	var seed field.Octuplet
	for i := range seed {
		seed[i] = field.PseudoRand(rng)
	}

	// Drive the native transcript. Each sampled value becomes an expected
	// output the circuit is constrained against.
	native := fiatshamir.NewFiatShamir()
	native.Update(base...)
	native.UpdateExt(ext...)
	digest := native.RandomDigest()
	coin := native.RandomFext()
	ints := native.RandomManyIntegers(fsTestNumIntegers, fsTestIntegerBound)
	// Seeding replaces the accumulated state outright, so `seeded` must depend
	// only on seed — covering the state round-trip the recursion relies on to
	// resume a transcript mid-protocol.
	native.SetState(seed)
	seeded := native.RandomFext()

	witness := &fsReplayCircuit{
		Base:     make([]circuit.Element, fsTestNumBase),
		Ext:      make([]circuit.Ext, fsTestNumExt),
		Digest:   poseidon2.NewKoalagnarkOctuplet(digest),
		Coin:     circuit.NewExt(coin),
		Integers: make([]frontend.Variable, fsTestNumIntegers),
		Seeded:   circuit.NewExt(seeded),
		Seed:     poseidon2.NewKoalagnarkOctuplet(seed),
	}
	for i := range base {
		witness.Base[i] = circuit.NewElementFromKoala(base[i])
	}
	for i := range ext {
		witness.Ext[i] = circuit.NewExt(ext[i])
	}
	for i := range ints {
		witness.Integers[i] = ints[i]
	}
	// Guard against a vacuous fixture: an all-zero integer sample would be
	// satisfied by a circuit whose RandomManyIntegers returns nothing useful,
	// so assert the native side actually produced varied output.
	require.NotZero(t, ints[0]+ints[1]+ints[2], "sampled integers should not all be zero")
	return witness
}

func TestGnarkFiatShamir_MatchesNative(t *testing.T) {
	// gnark also mutates the assignment while building a witness, so each
	// subtest builds its own.
	t.Run("native", func(t *testing.T) {
		ccs, err := frontend.CompileU32(field.Modulus(), scs.NewBuilder, fsReplayCircuitTemplate())
		require.NoError(t, err)
		w, err := frontend.NewWitness(fsReplayWitness(t), field.Modulus())
		require.NoError(t, err)
		require.NoError(t, ccs.IsSolved(w), "circuit transcript must derive the native challenges")
	})

	t.Run("emulated-bn254", func(t *testing.T) {
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, fsReplayCircuitTemplate())
		require.NoError(t, err)
		w, err := frontend.NewWitness(fsReplayWitness(t), ecc.BN254.ScalarField())
		require.NoError(t, err)
		require.NoError(t, ccs.IsSolved(w), "circuit transcript must derive the native challenges")
	})
}

func TestGnarkFiatShamir_RejectsWrongCoin(t *testing.T) {
	witness := fsReplayWitness(t)
	one := field.One()
	var tampered field.Ext
	tampered.B0.A0 = one
	witness.Coin = circuit.NewExt(tampered)

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, fsReplayCircuitTemplate())
	require.NoError(t, err)
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
	require.Error(t, ccs.IsSolved(w), "a forged coin must not satisfy the circuit")
}

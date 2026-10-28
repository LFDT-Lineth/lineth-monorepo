package fiatshamir

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/mamabear"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
)

func newTranscriptFactory[T any, PT generic.Element[T]](fieldName string) func() *FiatShamir[T, PT] {
	const width, rf, rp = poseidon2.CompressionWidth, 6, 21
	perm := poseidon2.NewPermutation(poseidon2.NewParameters[T, PT](width, rf, rp,
		poseidon2.DefaultSeed(fieldName, width, rf, rp)))
	compress := poseidon2.Compressor(perm)
	return func() *FiatShamir[T, PT] { return New[T, PT](compress) }
}

func elems[T any, PT generic.Element[T]](n int) []T {
	v := make([]T, n)
	for i := range v {
		PT(&v[i]).SetUint64(uint64(1000 + i))
	}
	return v
}

func testTranscript[T any, PT generic.Element[T]](t *testing.T, fieldName string) {
	newFS := newTranscriptFactory[T, PT](fieldName)
	vals := elems[T, PT](13)

	t.Run("deterministic", func(t *testing.T) {
		a, b := newFS(), newFS()
		a.Update(vals...)
		b.UpdateVec(vals[:4], vals[4:])
		require.Equal(t, a.RandomDigest(), b.RandomDigest())
		require.Equal(t, a.RandomDigest(), b.RandomDigest())
	})

	t.Run("transcript-binding", func(t *testing.T) {
		a, b := newFS(), newFS()
		a.Update(vals...)
		b.Update(vals[:12]...)
		require.NotEqual(t, a.RandomDigest(), b.RandomDigest())
	})

	t.Run("successive-coins-differ", func(t *testing.T) {
		fs := newFS()
		fs.Update(vals...)
		// Without the safeguard update, sampling twice in a row would repeat.
		require.NotEqual(t, fs.RandomDigest(), fs.RandomDigest())
	})

	t.Run("seed-keeps-state", func(t *testing.T) {
		a, b := newFS(), newFS()
		a.Update(vals...)
		b.Update(vals...)
		seed := a.State()
		x := a.RandomDigestFromSeed(seed, "alpha")
		require.Equal(t, b.State(), a.State())
		require.Equal(t, x, a.RandomDigestFromSeed(seed, "alpha"))
		require.NotEqual(t, x, a.RandomDigestFromSeed(seed, "beta"))
	})

	t.Run("state-roundtrip", func(t *testing.T) {
		a, b := newFS(), newFS()
		a.Update(vals...)
		b.SetState(a.State())
		require.Equal(t, a.RandomDigest(), b.RandomDigest())
	})

	t.Run("integers", func(t *testing.T) {
		fs := newFS()
		fs.Update(vals...)
		ints := fs.RandomManyIntegers(20, 1<<5)
		require.Len(t, ints, 20)
		for _, x := range ints {
			require.True(t, x >= 0 && x < 1<<5)
		}
		require.Panics(t, func() { fs.RandomManyIntegers(1, 3) })
	})
}

func TestTranscript(t *testing.T) {
	t.Run("koalabear", func(t *testing.T) { testTranscript[koalabear.Element](t, "koalabear") })
	t.Run("mamabear", func(t *testing.T) { testTranscript[mamabear.Element](t, "mamabear") })
}

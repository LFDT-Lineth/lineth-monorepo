package poseidon2

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	kbposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	kbvortex "github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark-crypto/field/mamabear"
	mbposeidon2 "github.com/consensys/gnark-crypto/field/mamabear/poseidon2"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
)

const (
	testFullRounds    = 6
	testPartialRounds = 21
)

// fieldCase bundles a field instance with gnark-crypto's reference permutation.
type fieldCase[T any, PT generic.Element[T]] struct {
	name      string
	reference func(width int) func([]T) error
	random    func() T
}

var koalabearCase = fieldCase[koalabear.Element, *koalabear.Element]{
	name: "koalabear",
	reference: func(width int) func([]koalabear.Element) error {
		return kbposeidon2.NewPermutation(width, testFullRounds, testPartialRounds).Permutation
	},
	random: func() koalabear.Element { var x koalabear.Element; x.MustSetRandom(); return x },
}

var mamabearCase = fieldCase[mamabear.Element, *mamabear.Element]{
	name: "mamabear",
	reference: func(width int) func([]mamabear.Element) error {
		return mbposeidon2.NewPermutation(width, testFullRounds, testPartialRounds).Permutation
	},
	random: func() mamabear.Element { var x mamabear.Element; x.MustSetRandom(); return x },
}

func newTestPermutation[T any, PT generic.Element[T]](fc fieldCase[T, PT], width int) *Permutation[T, PT] {
	return NewPermutation(NewParameters[T, PT](width, testFullRounds, testPartialRounds,
		DefaultSeed(fc.name, width, testFullRounds, testPartialRounds)))
}

func randomVec[T any, PT generic.Element[T]](fc fieldCase[T, PT], n int) []T {
	v := make([]T, n)
	for i := range v {
		v[i] = fc.random()
	}
	return v
}

func randomOctuplet[T any, PT generic.Element[T]](fc fieldCase[T, PT]) (o Octuplet[T]) {
	copy(o[:], randomVec(fc, BlockSize))
	return o
}

// testPermutationMatchesReference checks the generic permutation is
// bit-identical to gnark-crypto's field-specific one.
func testPermutationMatchesReference[T any, PT generic.Element[T]](t *testing.T, fc fieldCase[T, PT]) {
	for _, width := range []int{16, 24} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			perm := newTestPermutation(fc, width)
			ref := fc.reference(width)
			for range 20 {
				got := randomVec(fc, width)
				want := append([]T(nil), got...)
				require.NoError(t, perm.Permute(got))
				require.NoError(t, ref(want))
				require.Equal(t, want, got)
			}
			// The zero input exercises the round keys alone.
			got, want := make([]T, width), make([]T, width)
			require.NoError(t, perm.Permute(got))
			require.NoError(t, ref(want))
			require.Equal(t, want, got)

			require.ErrorIs(t, perm.Permute(make([]T, width+1)), ErrInvalidWidth)
		})
	}
}

func TestPermutationMatchesGnark(t *testing.T) {
	t.Run("koalabear", func(t *testing.T) { testPermutationMatchesReference(t, koalabearCase) })
	t.Run("mamabear", func(t *testing.T) { testPermutationMatchesReference(t, mamabearCase) })
}

// TestCompressMatchesKoalabearVortex checks that [Compress] instantiated over
// KoalaBear is the compression function used throughout the prover.
func TestCompressMatchesKoalabearVortex(t *testing.T) {
	perm := newTestPermutation(koalabearCase, CompressionWidth)
	for range 20 {
		a, b := randomOctuplet(koalabearCase), randomOctuplet(koalabearCase)
		require.Equal(t, kbvortex.CompressPoseidon2(a, b), Compress(perm, a, b))
	}
}

// testMDHasher checks the hasher's invariants over a given field.
func testMDHasher[T any, PT generic.Element[T]](t *testing.T, fc fieldCase[T, PT]) {
	perm := newTestPermutation(fc, CompressionWidth)
	compress := Compressor(perm)
	vals := randomVec(fc, 13)

	t.Run("matches-manual-chain", func(t *testing.T) {
		// 13 = 8 + 5: the second block is left-padded with 3 zeroes.
		var b0, b1 Octuplet[T]
		copy(b0[:], vals[:8])
		copy(b1[3:], vals[8:])
		want := Compress(perm, Compress(perm, Octuplet[T]{}, b0), b1)
		require.Equal(t, want, HashVec[T, PT](compress, vals...))
	})

	t.Run("incremental-writes", func(t *testing.T) {
		h := NewMDHasher[T, PT](compress)
		h.WriteElements(vals[:5]...)
		h.WriteElements(vals[5:9]...)
		h.WriteElements(vals[9:]...)
		require.Equal(t, HashVec[T, PT](compress, vals...), h.SumDigest())
	})

	t.Run("state-is-non-destructive", func(t *testing.T) {
		h := NewMDHasher[T, PT](compress)
		h.WriteElements(vals...)
		s := h.State()
		require.Equal(t, s, h.SumDigest())
	})

	t.Run("reset", func(t *testing.T) {
		h := NewMDHasher[T, PT](compress)
		h.WriteElements(vals...)
		_ = h.SumDigest()
		h.Reset()
		h.WriteElements(vals...)
		require.Equal(t, HashVec[T, PT](compress, vals...), h.SumDigest())
	})

	t.Run("set-state", func(t *testing.T) {
		h := NewMDHasher[T, PT](compress)
		h.WriteElements(vals[:8]...)
		mid := h.SumDigest()
		h.WriteElements(vals[8:]...)
		want := h.SumDigest()

		h2 := NewMDHasher[T, PT](compress)
		h2.SetStateOctuplet(mid)
		h2.WriteElements(vals[8:]...)
		require.Equal(t, want, h2.SumDigest())
	})

	t.Run("bytes-roundtrip", func(t *testing.T) {
		size := generic.ByteSize[T, PT]()
		var buf []byte
		for i := range vals {
			buf = append(buf, PT(&vals[i]).Marshal()...)
		}
		h := NewMDHasher[T, PT](compress)
		got := h.Sum(buf)
		require.Len(t, got, h.Size())
		require.Equal(t, OctupletToBytes[T, PT](HashVec[T, PT](compress, vals...)), got)

		_, err := NewMDHasher[T, PT](compress).Write(make([]byte, size+1))
		require.Error(t, err)
	})
}

func TestMDHasher(t *testing.T) {
	t.Run("koalabear", func(t *testing.T) { testMDHasher(t, koalabearCase) })
	t.Run("mamabear", func(t *testing.T) { testMDHasher(t, mamabearCase) })
}

// TestFieldsDiffer is a sanity check that the instances are actually distinct.
func TestFieldsDiffer(t *testing.T) {
	kb := newTestPermutation(koalabearCase, CompressionWidth)
	mb := newTestPermutation(mamabearCase, CompressionWidth)
	kbOut := Compress(kb, Octuplet[koalabear.Element]{}, Octuplet[koalabear.Element]{})
	mbOut := Compress(mb, Octuplet[mamabear.Element]{}, Octuplet[mamabear.Element]{})
	require.NotEqual(t, kbOut[0].Uint64(), mbOut[0].Uint64())
}

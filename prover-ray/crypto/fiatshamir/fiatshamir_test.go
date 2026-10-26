package fiatshamir

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/mamabear"
	"github.com/consensys/gnark-crypto/field/mamabear/extensions"
	"github.com/consensys/gnark-crypto/field/mamabear/poseidon2"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/stretchr/testify/require"
)

type (
	mamaDigest8 = [poseidon2.BlockLen]mamabear.Element
	mamaDigest6 = [6]mamabear.Element
)

// truncatedCompressor is a compression over 6-element digests built from the
// width 16 compression, for exercising [6] digests only: gnark-crypto has no
// width 12 Poseidon2 parameters for mamabear yet. Not a vetted construction.
type truncatedCompressor struct{ c poseidon2.Compressor }

func (t truncatedCompressor) CompressField(dst, left, right *mamaDigest6) {
	var l, r mamaDigest8
	copy(l[:], left[:])
	copy(r[:], right[:])
	t.c.CompressField(&l, &l, &r)
	copy(dst[:], l[:])
}

var (
	mamaCompressor8 = poseidon2.NewCompressor(poseidon2.NewDefaultPermutation())
	mamaCompressor6 hash.FieldCompressor[mamaDigest6] = truncatedCompressor{mamaCompressor8}
)

func newMamaTranscript8() *Transcript[mamabear.Element, extensions.E3, mamaDigest8] {
	return NewTranscript[mamabear.Element, extensions.E3](mamaCompressor8)
}

func newMamaTranscript6() *Transcript[mamabear.Element, extensions.E3, mamaDigest6] {
	return NewTranscript[mamabear.Element, extensions.E3](mamaCompressor6)
}

// TestMamabearExt3 checks the transcript over mamabear with its degree 3
// extension, against a direct use of the compression function.
func TestMamabearExt3(t *testing.T) {
	var a, b extensions.E3
	a.A0.SetUint64(1)
	a.A1.SetUint64(2)
	a.A2.SetUint64(3)
	b.A0.SetUint64(4)
	b.A1.SetUint64(5)
	b.A2.SetUint64(6)

	t.Run("digest8", func(t *testing.T) {
		fs := newMamaTranscript8()
		fs.UpdateExt(a, b)
		coin := fs.RandomFext()

		// [a, b] is 6 elements, left-padded into one block.
		var block, state mamaDigest8
		block[2], block[3], block[4] = a.A0, a.A1, a.A2
		block[5], block[6], block[7] = b.A0, b.A1, b.A2
		mamaCompressor8.CompressField(&state, &state, &block)
		require.Equal(t, extensions.E3{A0: state[0], A1: state[1], A2: state[2]}, coin)
	})

	t.Run("digest6", func(t *testing.T) {
		fs := newMamaTranscript6()
		fs.UpdateExt(a, b)
		coin := fs.RandomFext()

		// [a, b] is exactly one block.
		var state mamaDigest6
		block := mamaDigest6{a.A0, a.A1, a.A2, b.A0, b.A1, b.A2}
		mamaCompressor6.CompressField(&state, &state, &block)
		require.Equal(t, extensions.E3{A0: state[0], A1: state[1], A2: state[2]}, coin)

		ints := fs.RandomManyIntegers(13, 1<<8)
		require.Len(t, ints, 13)
	})
}

// e9 is a degree 9 extension, wider than mamaDigest8.
type e9 [9]mamabear.Element

func (e e9) AppendBase(s []mamabear.Element) []mamabear.Element { return append(s, e[:]...) }

// TestExtensionTooLarge checks that a transcript with an extension wider than
// the digest cannot be created.
func TestExtensionTooLarge(t *testing.T) {
	require.Panics(t, func() { NewTranscript[mamabear.Element, e9](mamaCompressor8) })
}

func BenchmarkMamabearCoin(b *testing.B) {
	var v [3]mamabear.Element
	for i := range v {
		v[i].SetUint64(uint64(i + 1))
	}
	b.Run("digest8", func(b *testing.B) {
		fs := newMamaTranscript8()
		b.ReportAllocs()
		for b.Loop() {
			fs.Update(v[:]...)
			_ = fs.RandomFext()
		}
	})
	b.Run("digest6", func(b *testing.B) {
		fs := newMamaTranscript6()
		b.ReportAllocs()
		for b.Loop() {
			fs.Update(v[:]...)
			_ = fs.RandomFext()
		}
	})
}

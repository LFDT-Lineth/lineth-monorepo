package fiatshamir

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	koalaext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	koalaposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark-crypto/field/mamabear"
	mamaext "github.com/consensys/gnark-crypto/field/mamabear/extensions"
	mamaposeidon2 "github.com/consensys/gnark-crypto/field/mamabear/poseidon2"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/stretchr/testify/require"
)

type (
	mamaDigest  = [mamaposeidon2.BlockLen]mamabear.Element
	koalaDigest = [koalaposeidon2.BlockLen]koalabear.Element
)

func newMamaTranscript6() *Transcript[mamabear.Element, mamaext.E3, mamaDigest] {
	c := mamaposeidon2.NewCompressor(mamaposeidon2.NewDefaultPermutation())
	return NewTranscript[mamabear.Element, mamaext.E3](c)
}

func newKoalaTranscript() *Transcript[koalabear.Element, koalaext.E6, koalaDigest] {
	c := koalaposeidon2.NewCompressor(koalaposeidon2.NewDefaultPermutation())
	return NewTranscript[koalabear.Element, koalaext.E6](c)
}

// TestMamabearExt3 checks the transcript over mamabear with its degree 3
// extension, against a direct use of the compression function.
func TestMamabearExt3(t *testing.T) {
	var a, b mamaext.E3
	a.A0.SetUint64(1)
	a.A1.SetUint64(2)
	a.A2.SetUint64(3)
	b.A0.SetUint64(4)
	b.A1.SetUint64(5)
	b.A2.SetUint64(6)

	t.Run("digest6", func(t *testing.T) {
		comp := mamaposeidon2.NewCompressor(mamaposeidon2.NewDefaultPermutation())
		fs := newMamaTranscript6()
		fs.UpdateExt(a, b)
		coin := fs.RandomFext()

		// [a, b] is exactly one block.
		var state mamaDigest
		block := mamaDigest{a.A0, a.A1, a.A2, b.A0, b.A1, b.A2}
		comp.CompressField(&state, &state, &block)
		require.Equal(t, mamaext.E3{A0: state[0], A1: state[1], A2: state[2]}, coin)

		ints := fs.RandomManyIntegers(13, 1<<8)
		require.Len(t, ints, 13)
	})
}

// benchTranscript runs the transcript benchmarks for one instantiation, with
// base elements v and extension elements ev.
func benchTranscript[F Element, E Extension[F], D hash.ArrayBlock[F], PE SetBaser[F, E]](
	b *testing.B, newFS func() *Transcript[F, E, D], v []F, ev []E,
) {
	b.Helper()
	b.Run("op=Coin", func(b *testing.B) {
		fs := newFS()
		b.ReportAllocs()
		for b.Loop() {
			fs.Update(v[:3]...)
			_ = fs.RandomFext[PE]()
		}
	})
	b.Run("op=Update1024", func(b *testing.B) {
		fs := newFS()
		b.ReportAllocs()
		for b.Loop() {
			fs.Update(v...)
			_ = fs.RandomDigest()
		}
	})
	b.Run("op=UpdateExt256", func(b *testing.B) {
		fs := newFS()
		b.ReportAllocs()
		for b.Loop() {
			fs.UpdateExt(ev...)
			_ = fs.RandomDigest()
		}
	})
	b.Run("op=RandomManyIntegers64", func(b *testing.B) {
		fs := newFS()
		b.ReportAllocs()
		for b.Loop() {
			_ = fs.RandomManyIntegers(64, 1<<16)
		}
	})
}

// BenchmarkTranscript compares the transcript over mamabear (degree 3
// extension, 6-element digest) with the one over koalabear (degree 6
// extension, 8-element digest). Compare with benchstat -col /field.
func BenchmarkTranscript(b *testing.B) {
	const n, nExt = 1024, 256
	b.Run("field=mamabear", func(b *testing.B) {
		v := make([]mamabear.Element, n)
		for i := range v {
			v[i].SetUint64(uint64(i + 1))
		}
		ev := make([]mamaext.E3, nExt)
		for i := range ev {
			ev[i].SetBase(v[3*i : 3*i+3])
		}
		benchTranscript[mamabear.Element, mamaext.E3, mamaDigest, *mamaext.E3](b, newMamaTranscript6, v, ev)
	})
	b.Run("field=koalabear", func(b *testing.B) {
		v := make([]koalabear.Element, n)
		for i := range v {
			v[i].SetUint64(uint64(i + 1))
		}
		ev := make([]koalaext.E6, nExt)
		for i := range ev {
			ev[i].SetBase(v[(6*i)%(n-6):][:6])
		}
		benchTranscript[koalabear.Element, koalaext.E6, koalaDigest, *koalaext.E6](b, newKoalaTranscript, v, ev)
	})
}

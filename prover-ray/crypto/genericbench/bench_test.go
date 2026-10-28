// Package genericbench compares the generic Poseidon2 / Fiat-Shamir code
// (crypto/poseidon2, crypto/fiatshamir) against non-generic baselines, for
// KoalaBear and MamaBear. It contains only tests and benchmarks.
//
// Implementations, as the impl=... benchmark key:
//
//   - baseline: non-generic code. For KoalaBear, gnark-crypto's permutation
//     and vortex.CompressPoseidon2, and the pre-generic MD hasher and
//     transcript (legacy_test.go). For MamaBear, which never had a
//     non-generic hasher, gnark-crypto's permutation underneath the generic
//     hasher and transcript.
//   - current (KoalaBear MD hasher and Fiat-Shamir only): what the prover
//     runs today, i.e. the generic hasher/transcript over gnark-crypto's
//     compression.
//   - generic: everything generic, permutation included.
//
// Sizes are selected by GENERIC_BENCH_SCALE: "small" (default, a few seconds
// locally) or "full" (meant for a dedicated machine). See
// scripts/bench-generic-poseidon.sh.
package genericbench

import (
	"fmt"
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	kbposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	kbvortex "github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark-crypto/field/mamabear"
	mbposeidon2 "github.com/consensys/gnark-crypto/field/mamabear/poseidon2"
	"github.com/stretchr/testify/require"

	gfs "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/fiatshamir"
	kbfs "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	kbp2 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	gp2 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
)

const (
	fullRounds    = 6
	partialRounds = 21
)

// scale holds the workload sizes.
type scale struct {
	// widths of the raw permutation benchmark
	widths []int
	// number of field elements hashed by the MD hasher
	msgLens []int
	// Fiat-Shamir workload: number of rounds, and elements absorbed per round
	fsRounds    []int
	fsUpdateLen []int
	// number of integers sampled per Fiat-Shamir round (as for FRI queries)
	fsQueries int
}

var scales = map[string]scale{
	"small": {
		widths:      []int{16},
		msgLens:     []int{64, 4096},
		fsRounds:    []int{16},
		fsUpdateLen: []int{64},
		fsQueries:   16,
	},
	"full": {
		widths:      []int{16, 24},
		msgLens:     []int{8, 64, 4096, 1 << 16, 1 << 20},
		fsRounds:    []int{16, 256, 4096},
		fsUpdateLen: []int{8, 1024, 1 << 14},
		fsQueries:   256,
	},
}

func currentScale(tb testing.TB) scale {
	name := os.Getenv("GENERIC_BENCH_SCALE")
	if name == "" {
		name = "small"
	}
	s, ok := scales[name]
	if !ok {
		tb.Fatalf("unknown GENERIC_BENCH_SCALE=%q (want small or full)", name)
	}
	return s
}

// ---------------------------------------------------------------------------
// Implementations

type permImpl[T any] struct {
	name    string
	newPerm func(width int) func([]T) error
}

type compressImpl[T any] struct {
	name     string
	compress gp2.CompressFunc[T]
}

// transcript is the subset of the transcript API exercised by the workload.
type transcript[T any] interface {
	Update(vec ...T)
	RandomDigest() [gp2.BlockSize]T
	RandomManyIntegers(num, upperBound int) []int
	State() [gp2.BlockSize]T
}

type transcriptImpl[T any] struct {
	name string
	new  func() transcript[T]
	// fromSeed samples a digest from a seed and a name without altering the
	// transcript (RandomFextFromSeed / RandomDigestFromSeed).
	fromSeed func(fs transcript[T], seed [gp2.BlockSize]T, name string)
}

type hasherImpl[T any] struct {
	name string
	hash func(msg []T) [gp2.BlockSize]T
}

// fieldSuite gathers every implementation for one field.
type fieldSuite[T any, PT generic.Element[T]] struct {
	name        string
	perms       []permImpl[T]
	compressors []compressImpl[T]
	hashers     []hasherImpl[T]
	transcripts []transcriptImpl[T]
}

func genericPerm[T any, PT generic.Element[T]](fieldName string, width int) *gp2.Permutation[T, PT] {
	return gp2.NewPermutation(gp2.NewParameters[T, PT](width, fullRounds, partialRounds,
		gp2.DefaultSeed(fieldName, width, fullRounds, partialRounds)))
}

// compressFromPermutation builds the 8+8 → 8 compression of [gp2.Compress]
// on top of an arbitrary width-16 permutation.
func compressFromPermutation[T any, PT generic.Element[T]](perm func([]T) error) gp2.CompressFunc[T] {
	return func(state, block gp2.Octuplet[T]) gp2.Octuplet[T] {
		var x [gp2.CompressionWidth]T
		copy(x[:gp2.BlockSize], state[:])
		copy(x[gp2.BlockSize:], block[:])
		if err := perm(x[:]); err != nil {
			panic(err)
		}
		res := block
		for i := range res {
			PT(&res[i]).Add(&res[i], &x[gp2.BlockSize+i])
		}
		return res
	}
}

func genericHasher[T any, PT generic.Element[T]](name string, compress gp2.CompressFunc[T]) hasherImpl[T] {
	return hasherImpl[T]{name: name, hash: func(msg []T) [gp2.BlockSize]T {
		return gp2.HashVec[T, PT](compress, msg...)
	}}
}

func genericTranscript[T any, PT generic.Element[T]](name string, compress gp2.CompressFunc[T]) transcriptImpl[T] {
	return transcriptImpl[T]{
		name: name,
		new:  func() transcript[T] { return gfs.New[T, PT](compress) },
		fromSeed: func(fs transcript[T], seed [gp2.BlockSize]T, n string) {
			fs.(*gfs.FiatShamir[T, PT]).RandomDigestFromSeed(seed, n)
		},
	}
}

func koalabearSuite() fieldSuite[koalabear.Element, *koalabear.Element] {
	type E = koalabear.Element
	gen := gp2.Compressor(genericPerm[E]("koalabear", gp2.CompressionWidth))
	return fieldSuite[E, *E]{
		name: "koalabear",
		perms: []permImpl[E]{
			{"baseline", func(w int) func([]E) error {
				return kbposeidon2.NewPermutation(w, fullRounds, partialRounds).Permutation
			}},
			{"generic", func(w int) func([]E) error { return genericPerm[E]("koalabear", w).Permute }},
		},
		compressors: []compressImpl[E]{
			{"baseline", kbvortex.CompressPoseidon2},
			{"generic", gen},
		},
		hashers: []hasherImpl[E]{
			{"baseline", func(msg []E) [8]E {
				h := newLegacyMDHasher()
				h.WriteElements(msg...)
				return h.SumDigest()
			}},
			{"current", func(msg []E) [8]E { return kbp2.HashVec(msg...) }},
			genericHasher[E]("generic", gen),
		},
		transcripts: []transcriptImpl[E]{
			{
				name: "baseline",
				new:  func() transcript[E] { return newLegacyFiatShamir() },
				fromSeed: func(fs transcript[E], seed [8]E, n string) {
					fs.(*legacyFiatShamir).RandomFextFromSeed(seed, n)
				},
			},
			{
				name: "current",
				new:  func() transcript[E] { return kbfs.NewFiatShamir() },
				fromSeed: func(fs transcript[E], seed [8]E, n string) {
					fs.(*kbfs.FiatShamir).RandomFextFromSeed(seed, n)
				},
			},
			genericTranscript[E]("generic", gen),
		},
	}
}

func mamabearSuite() fieldSuite[mamabear.Element, *mamabear.Element] {
	type E = mamabear.Element
	gnarkPerm := mbposeidon2.NewPermutation(gp2.CompressionWidth, fullRounds, partialRounds).Permutation
	gnark := compressFromPermutation[E](gnarkPerm)
	gen := gp2.Compressor(genericPerm[E]("mamabear", gp2.CompressionWidth))
	return fieldSuite[E, *E]{
		name: "mamabear",
		perms: []permImpl[E]{
			{"baseline", func(w int) func([]E) error {
				return mbposeidon2.NewPermutation(w, fullRounds, partialRounds).Permutation
			}},
			{"generic", func(w int) func([]E) error { return genericPerm[E]("mamabear", w).Permute }},
		},
		compressors: []compressImpl[E]{
			{"baseline", gnark},
			{"generic", gen},
		},
		hashers: []hasherImpl[E]{
			genericHasher[E]("baseline", gnark),
			genericHasher[E]("generic", gen),
		},
		transcripts: []transcriptImpl[E]{
			genericTranscript[E]("baseline", gnark),
			genericTranscript[E]("generic", gen),
		},
	}
}

// ---------------------------------------------------------------------------
// Workloads

func inputVec[T any, PT generic.Element[T]](n int) []T {
	v := make([]T, n)
	for i := range v {
		// deterministic, full-size values
		PT(&v[i]).SetUint64(uint64(i)*0x9E3779B97F4A7C15 + 1)
	}
	return v
}

// runTranscript plays a protocol-shaped workload: each round absorbs a
// message, samples a challenge, a batch of query indices and a named
// challenge from a seed. It returns everything sampled.
func runTranscript[T any](impl transcriptImpl[T], msg []T, rounds, queries int) (digests [][gp2.BlockSize]T, ints [][]int) {
	fs := impl.new()
	for r := range rounds {
		fs.Update(msg...)
		d := fs.RandomDigest()
		digests = append(digests, d)
		ints = append(ints, fs.RandomManyIntegers(queries, 1<<20))
		impl.fromSeed(fs, d, fmt.Sprintf("round-%d", r))
		digests = append(digests, fs.State())
	}
	return digests, ints
}

// ---------------------------------------------------------------------------
// Agreement: all implementations of a field must compute the same thing,
// otherwise the benchmark compares different work.

func testAgreement[T any, PT generic.Element[T]](t *testing.T, s fieldSuite[T, PT]) {
	for _, w := range []int{16, 24} {
		in := inputVec[T, PT](w)
		var ref []T
		for _, p := range s.perms {
			got := append([]T(nil), in...)
			require.NoError(t, p.newPerm(w)(got))
			if ref == nil {
				ref = got
			}
			require.Equal(t, ref, got, "permutation %s width %d", p.name, w)
		}
	}

	in := inputVec[T, PT](2 * gp2.BlockSize)
	a, b := gp2.Octuplet[T](in[:8]), gp2.Octuplet[T](in[8:])
	for _, c := range s.compressors {
		require.Equal(t, s.compressors[0].compress(a, b), c.compress(a, b), "compress %s", c.name)
	}

	for _, n := range []int{0, 1, 8, 13, 100} {
		msg := inputVec[T, PT](n)
		for _, h := range s.hashers {
			require.Equal(t, s.hashers[0].hash(msg), h.hash(msg), "hasher %s n=%d", h.name, n)
		}
	}

	msg := inputVec[T, PT](13)
	refD, refI := runTranscript(s.transcripts[0], msg, 5, 20)
	for _, tr := range s.transcripts {
		d, i := runTranscript(tr, msg, 5, 20)
		require.Equal(t, refD, d, "transcript %s digests", tr.name)
		require.Equal(t, refI, i, "transcript %s integers", tr.name)
	}
}

func TestAgreement(t *testing.T) {
	t.Run("koalabear", func(t *testing.T) { testAgreement(t, koalabearSuite()) })
	t.Run("mamabear", func(t *testing.T) { testAgreement(t, mamabearSuite()) })
}

// ---------------------------------------------------------------------------
// Benchmarks. Names are key=value so that results can be pivoted with
// `benchstat -col /impl`.

func benchPermutation[T any, PT generic.Element[T]](b *testing.B, s fieldSuite[T, PT]) {
	for _, w := range currentScale(b).widths {
		for _, p := range s.perms {
			b.Run(fmt.Sprintf("field=%s/width=%d/impl=%s", s.name, w, p.name), func(b *testing.B) {
				perm := p.newPerm(w)
				x := inputVec[T, PT](w)
				for b.Loop() {
					_ = perm(x)
				}
			})
		}
	}
}

func BenchmarkPermutation(b *testing.B) {
	benchPermutation(b, koalabearSuite())
	benchPermutation(b, mamabearSuite())
}

func benchCompress[T any, PT generic.Element[T]](b *testing.B, s fieldSuite[T, PT], parallel bool) {
	in := inputVec[T, PT](2 * gp2.BlockSize)
	for _, c := range s.compressors {
		b.Run(fmt.Sprintf("field=%s/impl=%s", s.name, c.name), func(b *testing.B) {
			if parallel {
				b.RunParallel(func(pb *testing.PB) {
					st, blk := gp2.Octuplet[T](in[:8]), gp2.Octuplet[T](in[8:])
					for pb.Next() {
						st = c.compress(st, blk)
					}
				})
				return
			}
			st, blk := gp2.Octuplet[T](in[:8]), gp2.Octuplet[T](in[8:])
			for b.Loop() {
				st = c.compress(st, blk)
			}
		})
	}
}

func BenchmarkCompress(b *testing.B) {
	benchCompress(b, koalabearSuite(), false)
	benchCompress(b, mamabearSuite(), false)
}

// BenchmarkCompressParallel measures throughput with GOMAXPROCS goroutines,
// to expose scaling differences on many-core machines.
func BenchmarkCompressParallel(b *testing.B) {
	benchCompress(b, koalabearSuite(), true)
	benchCompress(b, mamabearSuite(), true)
}

func benchMDHasher[T any, PT generic.Element[T]](b *testing.B, s fieldSuite[T, PT]) {
	for _, n := range currentScale(b).msgLens {
		msg := inputVec[T, PT](n)
		for _, h := range s.hashers {
			b.Run(fmt.Sprintf("field=%s/n=%d/impl=%s", s.name, n, h.name), func(b *testing.B) {
				b.SetBytes(int64(n * generic.ByteSize[T, PT]()))
				for b.Loop() {
					_ = h.hash(msg)
				}
			})
		}
	}
}

func BenchmarkMDHasher(b *testing.B) {
	benchMDHasher(b, koalabearSuite())
	benchMDHasher(b, mamabearSuite())
}

func benchFiatShamir[T any, PT generic.Element[T]](b *testing.B, s fieldSuite[T, PT]) {
	sc := currentScale(b)
	for _, rounds := range sc.fsRounds {
		for _, n := range sc.fsUpdateLen {
			msg := inputVec[T, PT](n)
			for _, tr := range s.transcripts {
				b.Run(fmt.Sprintf("field=%s/rounds=%d/update=%d/impl=%s", s.name, rounds, n, tr.name), func(b *testing.B) {
					for b.Loop() {
						_, _ = runTranscript(tr, msg, rounds, sc.fsQueries)
					}
				})
			}
		}
	}
}

func BenchmarkFiatShamir(b *testing.B) {
	benchFiatShamir(b, koalabearSuite())
	benchFiatShamir(b, mamabearSuite())
}

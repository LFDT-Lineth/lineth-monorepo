package fiatshamir

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// transcript runs a fixed sequence of operations covering every method of the
// transcript and returns all the sampled values, so that implementation
// changes can be checked against a golden output.
func transcript(fs *FiatShamir) (field.Octuplet, []field.Ext, []int) {
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	exts := make([]field.Ext, 0, 4)

	// Partial block, then a coin.
	fs.Update(field.PseudoRand(rng), field.PseudoRand(rng), field.PseudoRand(rng))
	exts = append(exts, fs.RandomFext())

	// Coin with no pending input.
	exts = append(exts, fs.RandomFext())

	// Extension elements spanning several blocks.
	ev := make([]field.Ext, 5)
	for i := range ev {
		ev[i] = field.PseudoRandExt(rng)
	}
	fs.UpdateExt(ev...)
	fs.UpdateGeneric(field.ElemFromBase(field.PseudoRand(rng)), field.ElemFromExt(field.PseudoRandExt(rng)))
	exts = append(exts, fs.RandomFext())

	// State round trip with pending input.
	fs.Update(field.PseudoRand(rng))
	s := fs.State()
	fs.SetState(s)

	bv := make([]field.Element, 17)
	for i := range bv {
		bv[i] = field.PseudoRand(rng)
	}
	fs.UpdateVec(bv, bv[:3])
	fs.UpdateExt(ev[:2]...)
	ints := fs.RandomManyIntegers(19, 1<<10)

	exts = append(exts, fs.RandomFext())

	return fs.RandomDigest(), exts, ints
}

// TestTranscriptGolden pins the transcript to the outputs of the original
// buffering implementation.
func TestTranscriptGolden(t *testing.T) {
	d, exts, ints := transcript(NewFiatShamir())
	require.Equal(t, goldenDigest, d)
	require.Equal(t, goldenExts, exts)
	require.Equal(t, goldenInts, ints)
}

// TestTranscriptPrint is used to (re)generate the golden values.
func TestTranscriptPrint(t *testing.T) {
	d, exts, ints := transcript(NewFiatShamir())
	t.Logf("digest: %#v", d)
	t.Logf("exts: %#v", exts)
	t.Logf("ints: %#v", ints)
}

func pseudoRandVec(n int) []field.Element {
	rng := rand.New(rand.NewChaCha8([32]byte{1}))
	v := make([]field.Element, n)
	for i := range v {
		v[i] = field.PseudoRand(rng)
	}
	return v
}

func pseudoRandExtVec(n int) []field.Ext {
	rng := rand.New(rand.NewChaCha8([32]byte{2}))
	v := make([]field.Ext, n)
	for i := range v {
		v[i] = field.PseudoRandExt(rng)
	}
	return v
}

// BenchmarkCoin is the common protocol step: a short prover message followed
// by an extension field coin.
func BenchmarkCoin(b *testing.B) {
	fs := NewFiatShamir()
	v := pseudoRandVec(3)
	b.ReportAllocs()
	for b.Loop() {
		fs.Update(v...)
		_ = fs.RandomFext()
	}
}

// BenchmarkUpdate1024 measures absorption throughput of base elements.
func BenchmarkUpdate1024(b *testing.B) {
	fs := NewFiatShamir()
	v := pseudoRandVec(1024)
	b.ReportAllocs()
	for b.Loop() {
		fs.Update(v...)
		_ = fs.RandomFext()
	}
}

// BenchmarkUpdateExt256 measures absorption throughput of extension elements.
func BenchmarkUpdateExt256(b *testing.B) {
	fs := NewFiatShamir()
	v := pseudoRandExtVec(256)
	b.ReportAllocs()
	for b.Loop() {
		fs.UpdateExt(v...)
		_ = fs.RandomFext()
	}
}

// BenchmarkRandomManyIntegers samples query positions.
func BenchmarkRandomManyIntegers(b *testing.B) {
	fs := NewFiatShamir()
	b.ReportAllocs()
	for b.Loop() {
		_ = fs.RandomManyIntegers(64, 1<<20)
	}
}

// BenchmarkStateRoundTrip snapshots and restores the state with pending input.
func BenchmarkStateRoundTrip(b *testing.B) {
	fs := NewFiatShamir()
	v := pseudoRandVec(3)
	b.ReportAllocs()
	for b.Loop() {
		fs.Update(v...)
		fs.SetState(fs.State())
	}
}

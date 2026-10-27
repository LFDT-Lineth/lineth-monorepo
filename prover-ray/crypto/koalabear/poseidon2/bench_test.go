package poseidon2

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

func benchVec(n int) []field.Element {
	rng := rand.New(rand.NewChaCha8([32]byte{3}))
	v := make([]field.Element, n)
	for i := range v {
		v[i] = field.PseudoRand(rng)
	}
	return v
}

// BenchmarkCompress is the Merkle tree node compression.
func BenchmarkCompress(b *testing.B) {
	rng := rand.New(rand.NewChaCha8([32]byte{4}))
	l, r := field.PseudoRandOctuplet(rng), field.PseudoRandOctuplet(rng)
	b.ReportAllocs()
	for b.Loop() {
		l = Compress(l, r)
	}
}

// BenchmarkLeaf hashes a short leaf with a reused hasher, as the FRI
// commitment scalar path does.
func BenchmarkLeaf(b *testing.B) {
	v := benchVec(27)
	h := NewMDHasher()
	b.ReportAllocs()
	for b.Loop() {
		h.Reset()
		h.WriteElements(v...)
		_ = h.SumDigest()
	}
}

// BenchmarkNewHasherLeaf hashes a short leaf with a fresh hasher, as the PCS
// opening and multiset hashing paths do.
func BenchmarkNewHasherLeaf(b *testing.B) {
	v := benchVec(27)
	b.ReportAllocs()
	for b.Loop() {
		h := NewMDHasher()
		h.WriteElements(v...)
		_ = h.SumDigest()
	}
}

// BenchmarkAbsorb1024 measures absorption throughput.
func BenchmarkAbsorb1024(b *testing.B) {
	v := benchVec(1024)
	h := NewMDHasher()
	b.ReportAllocs()
	for b.Loop() {
		h.Reset()
		h.WriteElements(v...)
		_ = h.SumDigest()
	}
}

package poseidon2

import (
	"fmt"
	"math/big"
	"testing"

	kbcircuit "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	bnposeidon2 "github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon2"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

type bnPackedCircuit struct {
	Inputs []kbcircuit.Element
	Output frontend.Variable
	Domain BNPackedDomain `gnark:"-"`
}

func (c *bnPackedCircuit) Define(api frontend.API) error {
	h := NewBNPackedGnarkHasher(api, c.Domain)
	h.Write(c.Inputs...)
	api.AssertIsEqual(h.Sum(), c.Output)
	return nil
}

func randKoala(n int) []field.Element {
	vals := make([]field.Element, n)
	for i := range vals {
		if _, err := vals[i].SetRandom(); err != nil {
			panic(err)
		}
	}
	return vals
}

func TestBNPackedHasherCircuit(t *testing.T) {
	domains := map[string]BNPackedDomain{"lengthInIV": BNPackedLengthInIV, "lengthSuffix": BNPackedLengthSuffix}
	for name, domain := range domains {
		for _, n := range []int{0, 1, 7, 8, 9, 16, 23} {
			if n == 0 && domain == BNPackedLengthInIV {
				continue // rejected, see TestBNPackedLengthInIVRejectsEmpty
			}
			t.Run(fmt.Sprintf("%s/n=%d", name, n), func(t *testing.T) {
				vals := randKoala(n)
				nh := NewBNPackedHasher(domain)
				nh.Write(vals...)
				digest := nh.Sum()

				circuit := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Domain: domain}
				witness := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Output: digest, Domain: domain}
				for i := range vals {
					witness.Inputs[i] = kbcircuit.NewElementFromKoala(vals[i])
				}
				require.NoError(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()))

				// wrong digest must fail
				var bad = digest
				bad.SetUint64(42)
				witness.Output = bad
				require.Error(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()))
			})
		}
	}
}

func TestBNPackedLengthInIVRejectsEmpty(t *testing.T) {
	require.Panics(t, func() { NewBNPackedHasher(BNPackedLengthInIV).Sum() })
}

// TestBNPackedLengthInIVSeparation checks that inputs of different lengths
// that zero padding would otherwise confuse get different digests, and that
// the two domain modes give different digests.
func TestBNPackedLengthInIVSeparation(t *testing.T) {
	var x, zero field.Element
	x.SetUint64(5)
	chunk := randKoala(8)
	cases := map[string][]field.Element{
		"[x]":         {x},
		"[0, x]":      {zero, x},
		"[x, 0]":      {x, zero},
		"chunk":       chunk,
		"chunk + [0]": append(append([]field.Element{}, chunk...), zero),
	}
	seen := map[fr.Element]string{}
	for name, in := range cases {
		for _, domain := range []BNPackedDomain{BNPackedLengthInIV, BNPackedLengthSuffix} {
			h := NewBNPackedHasher(domain)
			h.Write(in...)
			d := h.Sum()
			key := fmt.Sprintf("%s (domain %d)", name, domain)
			if other, ok := seen[d]; ok {
				t.Fatalf("collision between %s and %s", key, other)
			}
			seen[d] = key
		}
	}
}

func TestBNPackedHasherInjective(t *testing.T) {
	var x field.Element
	x.SetUint64(5)
	var zero field.Element

	h1 := NewBNPackedHasher(BNPackedLengthSuffix)
	h1.Write(x)
	h2 := NewBNPackedHasher(BNPackedLengthSuffix)
	h2.Write(zero, x)
	a, b := h1.Sum(), h2.Sum()
	require.False(t, a.Equal(&b))
}

// TestBNPackedVsEmulatedConstraints prints the PLONK constraint counts of the
// emulated KoalaBear Poseidon2 hasher vs the BN-packed one.
func TestBNPackedVsEmulatedConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles circuits")
	}
	for _, n := range []int{8, 64, 512} {
		emu := &KoalagnarkMDHasherCircuit{Inputs: make([]kbcircuit.Element, n)}
		csEmu, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, emu)
		require.NoError(t, err)

		packed := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n)}
		csPacked, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, packed)
		require.NoError(t, err)

		t.Logf("n=%4d  emulated=%9d  bn-packed=%9d  ratio=%.1fx",
			n, csEmu.GetNbConstraints(), csPacked.GetNbConstraints(),
			float64(csEmu.GetNbConstraints())/float64(csPacked.GetNbConstraints()))
	}
}

// TestBNPackedCanonicalSoundness replaces the canonical-split hint with
// malicious ones and checks that the circuit rejects them.
func TestBNPackedCanonicalSoundness(t *testing.T) {
	p := big.NewInt(2130706433)
	split := func(x *big.Int, q int64, out []*big.Int) {
		out[0].SetInt64(q)
		out[1].Rsh(x, 24)
		out[2].And(x, big.NewInt(1<<24-1))
	}
	cases := []struct {
		name string
		v    uint64
		hint solver.Hint
	}{
		{
			// v < p, prover claims q = 1 and x = v - p (wraps mod r)
			"q=1 on canonical input", 5,
			func(_ *big.Int, in, out []*big.Int) error {
				x := new(big.Int).Sub(in[0], p)
				x.Mod(x, ecc.BN254.ScalarField())
				split(x, 1, out)
				return nil
			},
		},
		{
			// v in [p, 2^31), prover keeps x = v (non-canonical) with q = 0
			"non-canonical x = v", 2130706433 + 5,
			func(_ *big.Int, in, out []*big.Int) error {
				split(new(big.Int).Set(in[0]), 0, out)
				return nil
			},
		},
		{
			// h = 127 with l != 0, i.e. x >= p
			"x = p exactly", 2130706433,
			func(_ *big.Int, in, out []*big.Int) error {
				split(new(big.Int).Set(in[0]), 0, out)
				return nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			circuit := &bnPackedCanonicalCircuit{}
			ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, circuit)
			require.NoError(t, err)

			w, err := frontend.NewWitness(&bnPackedCanonicalCircuit{
				In: tc.v,
			}, ecc.BN254.ScalarField())
			require.NoError(t, err)

			// honest hint must succeed
			require.NoError(t, ccs.IsSolved(w))
			// malicious hint must fail
			err = ccs.IsSolved(w, solver.OverrideHint(solver.GetHintID(koalaCanonicalSplitHint), tc.hint))
			require.Error(t, err)
		})
	}
}

// bnPackedCanonicalCircuit takes the raw limb as a native variable so that
// the test can feed non-canonical values in [p, 2^31): emulated.ValueOf would
// reduce them mod p at assignment time.
type bnPackedCanonicalCircuit struct {
	In frontend.Variable
}

func (c *bnPackedCanonicalCircuit) Define(api frontend.API) error {
	kb := kbcircuit.NewAPI(api)
	x := canonicalKoala(api, kb, rangecheck.New(api), kbcircuit.WrapFrontendVariable(c.In))
	api.AssertIsDifferent(x, -1)
	return nil
}

// bnPackedSumBytesReference is the original byte-based implementation, kept
// to pin that the element-based Sum produces the same digest.
func bnPackedSumBytesReference(xs []field.Element) fr.Element {
	md := bnposeidon2.NewMerkleDamgardHasher()
	for start := 0; start < len(xs); start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, len(xs))
		acc, tmp := new(big.Int), new(big.Int)
		for _, x := range xs[start:end] {
			acc.Lsh(acc, BNPackedBitsPerKoala)
			acc.Add(acc, tmp.SetUint64(x.Uint64()))
		}
		var e fr.Element
		e.SetBigInt(acc)
		b := e.Bytes()
		_, _ = md.Write(b[:])
	}
	var length fr.Element
	length.SetUint64(uint64(len(xs)))
	lb := length.Bytes()
	_, _ = md.Write(lb[:])
	var res fr.Element
	res.SetBytes(md.Sum(nil))
	return res
}

func TestBNPackedNativeMatchesByteReference(t *testing.T) {
	for _, n := range []int{0, 1, 2, 7, 8, 9, 15, 16, 17, 64, 100} {
		vals := randKoala(n)
		// include the extreme value p-1 to exercise slot straddling
		if n > 0 {
			vals[0].SetUint64(2130706432)
		}
		h := NewBNPackedHasher(BNPackedLengthSuffix)
		h.Write(vals...)
		got, want := h.Sum(), bnPackedSumBytesReference(vals)
		require.True(t, got.Equal(&want), "n=%d", n)
	}
}

func BenchmarkBNPackedNative(b *testing.B) {
	vals := randKoala(512)
	b.Run("elements", func(b *testing.B) {
		h := NewBNPackedHasher(BNPackedLengthSuffix)
		h.Write(vals...)
		b.ReportAllocs()
		for b.Loop() {
			_ = h.Sum()
		}
	})
	b.Run("bytes-reference", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = bnPackedSumBytesReference(vals)
		}
	})
}

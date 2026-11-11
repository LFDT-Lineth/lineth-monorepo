package poseidon2

import (
	"os"
	"testing"
	"time"

	kbcircuit "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

func bnTimingMedian(ds []time.Duration) time.Duration {
	for i := range ds {
		for j := i + 1; j < len(ds); j++ {
			if ds[j] < ds[i] {
				ds[i], ds[j] = ds[j], ds[i]
			}
		}
	}
	return ds[len(ds)/2]
}

func bnTimingNative(f func()) time.Duration {
	for i := 0; i < 3; i++ {
		f()
	}
	const reps = 200
	start := time.Now()
	for i := 0; i < reps; i++ {
		f()
	}
	return time.Since(start) / reps
}

// TestBNHashTimings measures native hashing time, circuit compile time and
// PLONK proving time (unsafe test SRS) for the emulated KoalaBear hasher and
// the BN-packed hashers. It takes about a minute, so it only runs when
// BN_HASH_TIMINGS=1. Results are reported in prover-ray/hashing_report.
func TestBNHashTimings(t *testing.T) {
	if os.Getenv("BN_HASH_TIMINGS") != "1" {
		t.Skip("set BN_HASH_TIMINGS=1 to run")
	}
	for _, n := range []int{8, 64, 512} {
		vals := randKoala(n)
		kb := make([]kbcircuit.Element, n)
		for i := range vals {
			kb[i] = kbcircuit.NewElementFromKoala(vals[i])
		}

		// native
		emuNat := bnTimingNative(func() { h := NewMDHasher(); h.WriteElements(vals...); _ = h.SumDigest() })
		ivNat := bnTimingNative(func() { h := NewBNPackedHasher(BNPackedLengthInIV); h.Write(vals...); _ = h.Sum() })
		spNat := bnTimingNative(func() { h := NewBNSpongeHasher(BNSpongeLengthInCapacity); h.Write(vals...); _ = h.Sum() })

		// witnesses
		mh := NewMDHasher()
		mh.WriteElements(vals...)
		md := mh.SumDigest()
		var emuW KoalagnarkMDHasherCircuit
		emuW.Inputs = kb
		for i := 0; i < 8; i++ {
			emuW.Output[i] = kbcircuit.NewElementFromKoala(md[i])
		}
		ivH := NewBNPackedHasher(BNPackedLengthInIV)
		ivH.Write(vals...)
		spH := NewBNSpongeHasher(BNSpongeLengthInCapacity)
		spH.Write(vals...)

		type cse struct {
			name    string
			circ, w frontend.Circuit
			native  time.Duration
		}
		cases := []cse{
			{"emulated", &KoalagnarkMDHasherCircuit{Inputs: make([]kbcircuit.Element, n)}, &emuW, emuNat},
			{"width2-IV", &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Domain: BNPackedLengthInIV},
				&bnPackedCircuit{Inputs: kb, Output: ivH.Sum(), Domain: BNPackedLengthInIV}, ivNat},
			{"width3-sponge", &bnSpongeCircuit{Inputs: make([]kbcircuit.Element, n)},
				&bnSpongeCircuit{Inputs: kb, Output: spH.Sum()}, spNat},
		}
		for _, c := range cases {
			t0 := time.Now()
			ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, c.circ)
			if err != nil {
				t.Fatal(err)
			}
			compile := time.Since(t0)
			srs, srsL, err := unsafekzg.NewSRS(ccs)
			if err != nil {
				t.Fatal(err)
			}
			pk, vk, err := plonk.Setup(ccs, srs, srsL)
			if err != nil {
				t.Fatal(err)
			}
			wit, err := frontend.NewWitness(c.w, ecc.BN254.ScalarField())
			if err != nil {
				t.Fatal(err)
			}
			pub, _ := wit.Public()
			var proves []time.Duration
			for i := 0; i < 5; i++ {
				t1 := time.Now()
				proof, err := plonk.Prove(ccs, pk, wit)
				if err != nil {
					t.Fatal(err)
				}
				proves = append(proves, time.Since(t1))
				if i == 0 {
					if err := plonk.Verify(proof, vk, pub); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Logf("n=%d %-14s constraints=%8d native=%10v compile=%10v prove(median of 5)=%10v",
				n, c.name, ccs.GetNbConstraints(), c.native, compile.Round(time.Millisecond), bnTimingMedian(proves).Round(time.Millisecond))
		}
	}
}

package fri

import (
	"math/big"
	"math/bits"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear"
)

// TestE2RotationIdentityMatchesDense validates the rotation identity that
// Level.EvalsAt relies on: for a size-N codeword domain with generator ω, a
// claim point ζ·ω_n^s (ω_n = ω^r), and a domain position pos with natural-order
// exponent e = bitReverse(pos),
//
//	1/(ω^e − ζ·ω^{rs}) == ω^{−rs} · DenomBaseInv[(e − rs) mod N]
//
// where DenomBaseInv[k] = 1/(ω^k − ζ). It compares the rotated/scaled value
// against the dense denominatorInverses table on random shifts, which is the
// pre-E2 reference implementation.
func TestE2RotationIdentityMatchesDense(t *testing.T) {
	zeta := benchChallengePoint(0x1234_5678)

	for _, sizeLog2 := range []int{1, 2, 4, 6} {
		for _, rate := range []int{2, 4, 8} {
			n := 1 << sizeLog2
			N := n * rate
			logN := bits.TrailingZeros(uint(N))

			gen, err := koalabear.Generator(uint64(N))
			if err != nil {
				t.Fatalf("Generator(%d): %v", N, err)
			}
			domain := domainLight{cardinality: uint64(N), generator: gen}

			// ω_n = ω_N^rate, the plaintext-domain generator.
			var omegaN field.Element
			omegaN.Exp(gen, big.NewInt(int64(rate)))

			// A representative set of shifts (including 0 and n-1).
			shifts := distinctShifts(n)

			// Dense reference table: domainPoints × claimPoints.
			domainPoints := make([]field.Ext, N)
			for pos := range domainPoints {
				domainPoints[pos] = domainPointExt(domain, pos)
			}
			claimPoints := make([]field.Ext, len(shifts))
			for j, s := range shifts {
				var rot field.Element
				rot.Exp(omegaN, big.NewInt(int64(s)))
				claimPoints[j].MulByElement(&zeta, &rot)
			}
			dense, err := denominatorInverses(domainPoints, claimPoints)
			if err != nil {
				t.Fatalf("denominatorInverses: %v", err)
			}

			denomBaseInv, err := denomBaseInverses(gen, N, zeta)
			if err != nil {
				t.Fatalf("denomBaseInverses: %v", err)
			}

			mask := N - 1
			for pos := 0; pos < N; pos++ {
				e := int(bits.Reverse64(uint64(pos)) >> (64 - logN))
				for j, s := range shifts {
					rot := (rate * s) & mask
					var scale field.Element
					scale.Exp(gen, big.NewInt(int64((N-rot)&mask)))

					var got field.Ext
					got.MulByElement(&denomBaseInv[(e-rot)&mask], &scale)

					want := dense[pos*len(shifts)+j]
					if !got.Equal(&want) {
						t.Fatalf("sizeLog2=%d rate=%d pos=%d shift=%d: identity mismatch\n got=%v\nwant=%v",
							sizeLog2, rate, pos, s, got, want)
					}
				}
			}
		}
	}
}

// distinctShifts returns a small deterministic set of distinct shifts in
// [0,n), always including 0 and n-1.
func distinctShifts(n int) []int {
	seen := map[int]struct{}{}
	var out []int
	add := func(s int) {
		if s < 0 || s >= n {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(0)
	add(n - 1)
	x := uint64(0x9e3779b1)
	for len(out) < 5 && len(out) < n {
		x = benchRand(x)
		add(int(x % uint64(n)))
	}
	return out
}

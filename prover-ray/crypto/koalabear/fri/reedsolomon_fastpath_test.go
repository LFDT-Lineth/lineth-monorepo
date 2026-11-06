package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
)

// TestEncodeRateTwoFastPath checks that the rate-two coset fast path produces
// exactly the same codeword as the generic zero-pad-and-FFT path (exercised by
// disabling the fast-path trigger on a copy of the encoder), for base and
// extension columns across sizes that cover the naive and COBRA bit-reversal
// regimes.
func TestEncodeRateTwoFastPath(t *testing.T) {
	prng := rand.New(utils.NewRandSource(1))

	for _, logN := range []int{0, 1, 2, 5, 10, 14} {
		t.Run(fmt.Sprintf("2^%d", logN), func(t *testing.T) {
			n := 1 << logN
			enc := NewEncoder(uint64(2*n), n)
			if enc.cosetSmallDomain == nil {
				t.Fatal("rate-two encoder must have a coset domain")
			}
			generic := enc
			generic.cosetSmallDomain = nil

			p := make([]field.Element, n)
			for i := range p {
				p[i].SetUint64(prng.Uint64())
			}
			got, want := enc.Encode(p), generic.Encode(p)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("base codeword mismatch at %d", i)
				}
			}

			pe := make([]field.Ext, n)
			for i := range pe {
				pe[i] = field.PseudoRandExt(prng)
			}
			gotExt, wantExt := enc.EncodeExt(pe), generic.EncodeExt(pe)
			for i := range wantExt {
				if gotExt[i] != wantExt[i] {
					t.Fatalf("ext codeword mismatch at %d", i)
				}
			}
		})
	}

	// Rate four must keep using the generic path.
	if enc4 := NewEncoder(4*1024, 1024); enc4.cosetSmallDomain != nil {
		t.Fatal("rate-four encoder must not have a coset domain")
	}
}

// TestEncodeGenericPath checks the generic (non-rate-two) encode against its
// definition: interpolate in natural order, zero-pad to the codeword size and
// evaluate, emitting the codeword in N-bit-reversed order.
func TestEncodeGenericPath(t *testing.T) {
	prng := rand.New(utils.NewRandSource(2))

	for _, rate := range []int{2, 4, 8} {
		for _, logN := range []int{0, 1, 5, 10, 13} {
			t.Run(fmt.Sprintf("rate=%d/2^%d", rate, logN), func(t *testing.T) {
				n := 1 << logN
				enc := NewEncoder(uint64(rate*n), n)
				enc.cosetSmallDomain = nil

				p := make([]field.Element, n)
				for i := range p {
					p[i].SetUint64(prng.Uint64())
				}
				want := make([]field.Element, rate*n)
				copy(want, p)
				enc.smallDomain.FFTInverse(want[:n], fft.DIF)
				gutils.BitReverse(want[:n])
				enc.Domain.FFT(want, fft.DIF)
				got := enc.Encode(p)
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("base codeword mismatch at %d", i)
					}
				}

				pe := make([]field.Ext, n)
				for i := range pe {
					pe[i] = field.PseudoRandExt(prng)
				}
				wantExt := make([]field.Ext, rate*n)
				copy(wantExt, pe)
				enc.smallDomain.FFTInverseExt6(wantExt[:n], fft.DIF)
				gutils.BitReverse(wantExt[:n])
				enc.Domain.FFTExt6(wantExt, fft.DIF)
				gotExt := enc.EncodeExt(pe)
				for i := range wantExt {
					if gotExt[i] != wantExt[i] {
						t.Fatalf("ext codeword mismatch at %d", i)
					}
				}
			})
		}
	}
}

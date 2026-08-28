package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
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

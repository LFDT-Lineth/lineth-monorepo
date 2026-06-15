package codegen

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fri"
)

func newTestParams(t *testing.T, n, d, numQueries int, opts ...fri.Option) fri.Params {
	t.Helper()
	p, err := fri.NewParams(n, d, numQueries,
		commitment.DefaultLeafHasher, commitment.DefaultNodeHasher, opts...)
	if err != nil {
		t.Fatalf("fri.NewParams: %v", err)
	}
	return p
}

// TestFRIParamsNumRounds verifies that NumRounds = log2(D).
func TestFRIParamsNumRounds(t *testing.T) {
	for _, tc := range []struct {
		n, d, want int
	}{
		{16, 4, 2},
		{32, 8, 3},
		{16, 2, 1},
		{8, 2, 1},
	} {
		got := newTestParams(t, tc.n, tc.d, 4)
		if got.NumRounds != tc.want {
			t.Errorf("N=%d D=%d: NumRounds = %d, want %d", tc.n, tc.d, got.NumRounds, tc.want)
		}
		if len(got.DomainsLight) != tc.want+1 {
			t.Errorf("N=%d D=%d: len(DomainsLight) = %d, want %d", tc.n, tc.d, len(got.DomainsLight), tc.want+1)
		}
	}
}

// TestFRIParamsGenInvAreInverse verifies that each domain generator is invertible
// in the Koalabear field for every fold round.
func TestFRIParamsGenInvAreInverse(t *testing.T) {
	p := newTestParams(t, 32, 8, 4)
	var one koalabear.Element
	one.SetOne()
	for j, domain := range p.DomainsLight {
		var inv, product koalabear.Element
		inv.Inverse(&domain.Generator)
		product.Mul(&domain.Generator, &inv)
		if product != one {
			t.Errorf("round %d: gen * inv != 1 (gen=%d inv=%d product=%v)", j, domain.Generator.Uint64(), inv.Uint64(), product)
		}
	}
}

// TestFRIParamsHalvingProperty verifies that DomainGens[j+1] = DomainGens[j]^2,
// i.e. squaring the round-j generator produces the round-(j+1) generator.
func TestFRIParamsHalvingProperty(t *testing.T) {
	p := newTestParams(t, 32, 8, 4)
	for j := 0; j < p.NumRounds; j++ {
		var squared koalabear.Element
		squared.Square(&p.DomainsLight[j].Generator)
		want := squared.Uint64()
		if got := p.DomainsLight[j+1].Generator.Uint64(); got != want {
			t.Errorf("round %d: DomainGens[%d] = %d, want %d (= DomainGens[%d]^2)", j+1, j+1, got, want, j)
		}
	}
}

// TestFRIParamsGrinding verifies that the grinding bit count is preserved.
func TestFRIParamsGrinding(t *testing.T) {
	const grinding = 8
	p := newTestParams(t, 16, 4, 4, fri.WithGrinding(grinding))
	if p.Grinding != 8 {
		t.Errorf("Grinding = %d, want 8", p.Grinding)
	}
}

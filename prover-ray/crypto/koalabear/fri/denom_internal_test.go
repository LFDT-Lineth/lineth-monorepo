package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// The chunked denominator inverses must equal one inversion per position,
// across chunk boundaries.
func TestDenomBaseInversesMatchesPointwise(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{1, 8, 1 << 12, 3<<12 + 1<<10} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			gen := field.RootOfUnityBy(1 << 14)
			zeta := field.VecPseudoRandExt(rng, 1)[0]
			got, err := denomBaseInverses(gen, n, zeta)
			require.NoError(t, err)
			pow := field.One()
			for e := range n {
				var want field.Ext
				x := field.Lift(pow)
				want.Sub(&x, &zeta)
				want.Inverse(&want)
				require.Equal(t, want, got[e], "position %d", e)
				pow.Mul(&pow, &gen)
			}
		})
	}
}

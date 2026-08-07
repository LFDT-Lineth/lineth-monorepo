package poseidon2

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// TestCompressChain16MatchesScalar checks every lane of the batch kernel and
// of the generic fallback against the scalar Compress chain, for the one- and
// two-block chains the Merkle tree uses, with all-zero and random values.
func TestCompressChain16MatchesScalar(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))

	randOctuplet := func(zero bool) field.Octuplet {
		var o field.Octuplet
		if !zero {
			for i := range o {
				o[i].SetUint64(rng.Uint64())
			}
		}
		return o
	}

	for _, nbSteps := range []int{1, 2} {
		for _, zero := range []bool{true, false} {
			var (
				states [BatchLanes]field.Octuplet
				blocks = make([][BatchLanes]field.Octuplet, nbSteps)
				state  = make([]field.Element, 8*BatchLanes)
				matrix = make([]field.Element, nbSteps*8*BatchLanes)
			)
			for lane := range BatchLanes {
				states[lane] = randOctuplet(zero)
				StageOctuplet(state, lane, &states[lane])
				for step := range nbSteps {
					blocks[step][lane] = randOctuplet(zero)
					StageOctuplet(matrix[step*8*BatchLanes:], lane, &blocks[step][lane])
				}
			}

			var got, gotGeneric [BatchLanes]field.Octuplet
			CompressChain16(state, matrix, got[:])
			compressChain16Generic(state, matrix, gotGeneric[:])

			for lane := range BatchLanes {
				want := states[lane]
				for step := range nbSteps {
					want = Compress(want, blocks[step][lane])
				}
				if got[lane] != want || gotGeneric[lane] != want {
					t.Fatalf("nbSteps=%d zero=%v lane=%d: batch result diverges from scalar Compress chain", nbSteps, zero, lane)
				}
			}
		}
	}
}

package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryMutationSoundness drives every per-query scenario from
// [wioptest.All] through [wioptest.SweepMutations]. For each scenario it builds
// the honest witness and then perturbs each committed input — every entry of
// every column and every opening cell — asserting the query's Check rejects at
// least one perturbation.
//
// A query whose Check survives every single-value perturbation does not depend
// on its inputs, a soundness gap this flags.
func TestQueryMutationSoundness(t *testing.T) {
	// Tweaks tried per position. AddOne is the minimal change, good for
	// exact-equality constraints; RandomValue is the general default; addLarge
	// deterministically escapes the slack of range and lookup relations, where
	// a small bump can stay in range.
	tweaks := []wioptest.Tweak{wioptest.AddOne, wioptest.RandomValue, addLarge}

	for _, build := range wioptest.All() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			// Completeness sanity: the honest witness must pass.
			rt := wiop.NewRuntime(sc.Sys)
			sc.RunHonest(&rt)
			require.NoError(t, sc.Query.Check(rt),
				"honest witness must pass Check")

			verify := func(rt wiop.Runtime) error { return sc.Query.Check(rt) }
			caught := false
			for _, tw := range tweaks {
				if wioptest.SweepMutations(sc.Sys, sc.RunHonest, verify, tw, 0).AnyCaught() {
					caught = true
					break
				}
			}
			assert.True(t, caught,
				"Check must reject at least one single-value perturbation of the honest witness")
		})
	}
}

// addLarge adds 2^30 to v, large enough to break range and lookup relations
// whose honest values sit far from their bounds, while staying below the
// koalabear modulus so it cannot wrap back into range.
func addLarge(v field.Gen) field.Gen {
	var e field.Element
	e.SetUint64(1 << 30)
	return v.Add(field.ElemFromBase(e))
}

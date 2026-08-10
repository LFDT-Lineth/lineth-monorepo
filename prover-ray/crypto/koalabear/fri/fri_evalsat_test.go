package fri

// Equality tests for the combine-then-divide Level.EvalsAt: the production
// implementation regroups the batched DEEP quotient (combine columns first,
// divide once per distinct claim point), which is exact field arithmetic and
// must therefore be bit-identical to the straightforward
// quotient-per-claim + Horner form retained below as the reference —
// mirroring how denominatorInverses backs the E2 rotation identity tests.

import (
	"math/bits"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

// evalsAtReference is the pre-combine-then-divide implementation of
// Level.EvalsAt: for every position, each column contributes its per-claim
// quotients and the running value is folded in via a Horner ladder over
// columns.
func (l Level) evalsAtReference(alphaDeep field.Ext, running []field.Ext) []field.Ext {
	evals := make([]field.Ext, len(running))
	copy(evals, running)

	mask := len(l.DenomBaseInv) - 1
	logSize := bits.TrailingZeros(uint(len(l.DenomBaseInv)))

	parallel.Execute(len(evals), func(start, end int) {
		for pos := start; pos < end; pos++ {
			e := bitReverseExponent(pos, logSize)
			for c := len(l.Columns) - 1; c >= 0; c-- {
				column := &l.Columns[c]
				var ev *field.Ext
				var lifted field.Ext
				if column.isBase() {
					lifted = field.Lift(column.EvalsBase[pos])
					ev = &lifted
				} else {
					ev = &column.EvalsExt[pos]
				}
				var columnSum field.Ext
				for k := range column.Claims {
					rot := &column.Rotations[k]
					inv := &l.DenomBaseInv[(e-rot.Rot)&mask]

					var numerator, term field.Ext
					numerator.Sub(ev, &column.Claims[k].Value)
					term.Mul(&numerator, inv)
					term.MulByElement(&term, &rot.Scale)
					columnSum.Add(&columnSum, &term)
				}

				evals[pos].Mul(&evals[pos], &alphaDeep)
				evals[pos].Add(&evals[pos], &columnSum)
			}
		}
	})
	return evals
}

// evalsAtTestLevels reconstructs the levels of a bench-style workload so both
// EvalsAt implementations can be compared on realistic column/claim shapes.
func evalsAtTestLevels(tb testing.TB, cfg benchConfig) ([]Level, []field.Ext) {
	tb.Helper()
	fx := newBenchFixture(tb, cfg)
	fx.pcs.Reset()
	if err := fx.pcs.AddOpening(fx.committed, fx.zeta, fx.shifts, fx.claimed); err != nil {
		tb.Fatalf("AddOpening: %v", err)
	}
	restricted, err := fx.pcs.restrictToOpenings()
	if err != nil {
		tb.Fatalf("restrictToOpenings: %v", err)
	}
	levels, err := restricted.reconstructLevels()
	if err != nil {
		tb.Fatalf("reconstructLevels: %v", err)
	}
	running := make([]field.Ext, 1<<restricted.Params.LogCodewordSize)
	rng := uint64(0x1234)
	for i := range running {
		running[i], rng = benchExt(rng)
	}
	return levels, running
}

// TestEvalsAtMatchesReference exercises shift schedules with per-row random
// claim points (many single-column groups), production-like shared points
// (one group), base-only and ext-only batches, and single-column levels —
// each against a random alphaDeep and the zero alphaDeep the D=1 prover path
// uses. Results must be exactly equal.
func TestEvalsAtMatchesReference(t *testing.T) {
	configs := []benchConfig{
		{minLog2: 4, maxLog2: 6, basePolys: 7, extPolys: 5, rate: 4, numQueries: 4, maxShifts: 3, seed: 1},
		{minLog2: 4, maxLog2: 6, basePolys: 7, extPolys: 5, rate: 4, numQueries: 4, sharedShifts: []int{0, 1, 2}, seed: 2},
		{minLog2: 4, maxLog2: 5, basePolys: 6, extPolys: 0, rate: 4, numQueries: 4, maxShifts: 2, seed: 3},
		{minLog2: 4, maxLog2: 5, basePolys: 0, extPolys: 6, rate: 4, numQueries: 4, maxShifts: 2, seed: 4},
		{minLog2: 4, maxLog2: 4, basePolys: 1, extPolys: 0, rate: 4, numQueries: 4, maxShifts: 1, seed: 5},
	}
	alphaRand, _ := benchExt(0xabcde)
	alphas := []field.Ext{alphaRand, {}}

	for _, cfg := range configs {
		t.Run(cfg.name(), func(t *testing.T) {
			levels, running := evalsAtTestLevels(t, cfg)
			for _, alphaDeep := range alphas {
				for li, level := range levels {
					run := running[:len(level.DenomBaseInv)]
					want := level.evalsAtReference(alphaDeep, run)
					got := level.EvalsAt(alphaDeep, run)
					for i := range want {
						if !want[i].Equal(&got[i]) {
							t.Fatalf("level %d: mismatch at position %d (alphaDeep zero: %v)",
								li, i, alphaDeep.IsZero())
						}
					}
				}
			}
		})
	}
}

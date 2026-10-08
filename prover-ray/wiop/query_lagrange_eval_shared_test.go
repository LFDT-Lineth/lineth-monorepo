package wiop_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestLagrangeEval_SharedWeights_MatchesReference checks the shared-weight
// evaluation at an extension point against polynomials.EvalLagrange on the
// fully materialised (padded, then rotated) vector, over every padding
// direction, base and extension data, several shifts, partial assignments with
// a non-zero padding value, and a domain spanning several row chunks. Check
// cannot catch a wrong evaluation here, since it shares the kernel with
// SelfAssign.
func TestLagrangeEval_SharedWeights_MatchesReference(t *testing.T) {
	var (
		rng    = rand.New(rand.NewPCG(1, 2))
		shifts = []int{0, 1, -1, 3}
	)
	for _, n := range []int{4, 1 << 14} {
		for _, pd := range []wiop.PaddingDirection{wiop.PaddingDirectionNone, wiop.PaddingDirectionLeft, wiop.PaddingDirectionRight} {
			t.Run(fmt.Sprintf("n=%d/pd=%v", n, pd), func(t *testing.T) {
				plainLen := n
				if pd != wiop.PaddingDirectionNone {
					plainLen = n - n/4 - 1
				}

				sys := wiop.NewSystemf("shared")
				r0 := sys.NewRound()
				r1 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, pd)
				base := mod.NewColumn(sys.Context.Childf("base"), r0)
				ext := mod.NewExtensionColumn(sys.Context.Childf("ext"), r0)
				coin := r1.NewCoinField(sys.Context.Childf("coin"))

				views := make([]*wiop.ColumnView, 0, 2*len(shifts))
				for _, k := range shifts {
					views = append(views, base.View().Shift(k), ext.View().Shift(k))
				}
				le := sys.NewLagrangeEval(sys.Context.Childf("le"), views, coin)

				var pad field.Element
				if pd != wiop.PaddingDirectionNone {
					pad.SetUint64(rng.Uint64())
				}
				baseCV := &wiop.ConcreteVector{Plain: field.VecFromBase(field.VecPseudoRandBase(rng, plainLen)), Padding: pad}
				extCV := &wiop.ConcreteVector{Plain: field.VecFromExt(field.VecPseudoRandExt(rng, plainLen)), Padding: pad}

				rt := wiop.NewRuntime(sys)
				rt.AssignColumn(base, baseCV)
				rt.AssignColumn(ext, extCV)
				rt.AdvanceRound()
				le.SelfAssign(rt)

				z := le.EvaluationPoint.EvaluateSingle(rt).Value
				require.False(t, z.IsBase(), "the shared path is only taken at an extension point")
				for i, pv := range views {
					cv := baseCV
					if pv.Column == ext {
						cv = extCV
					}
					want := polynomials.EvalLagrange(materialize(cv, n, pd, pv.ShiftingOffset), z)
					got := rt.GetCellValue(le.EvaluationClaims[i])
					require.Equal(t, want.AsExt(), got.AsExt(), "view %d (shift %d, ext=%v)", i, pv.ShiftingOffset, pv.Column == ext)
				}
			})
		}
	}
}

// materialize writes cv down as its full n-row padded vector, rotated by shift:
// out[j] = padded[(j+shift) mod n].
func materialize(cv *wiop.ConcreteVector, n int, pd wiop.PaddingDirection, shift int) field.Vec {
	dataStart := 0
	if pd == wiop.PaddingDirectionLeft {
		dataStart = n - cv.Plain.Len()
	}
	src := func(j int) int {
		j = ((j+shift)%n + n) % n
		if j < dataStart || j >= dataStart+cv.Plain.Len() {
			return -1
		}
		return j - dataStart
	}
	if cv.Plain.IsBase() {
		out := make([]field.Element, n)
		for j := range out {
			if s := src(j); s >= 0 {
				out[j] = cv.Plain.AsBase()[s]
			} else {
				out[j] = cv.Padding
			}
		}
		return field.VecFromBase(out)
	}
	out := make([]field.Ext, n)
	for j := range out {
		if s := src(j); s >= 0 {
			out[j] = cv.Plain.AsExt()[s]
		} else {
			out[j] = field.Lift(cv.Padding)
		}
	}
	return field.VecFromExt(out)
}

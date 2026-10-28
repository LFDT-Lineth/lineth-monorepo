package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestEvaluateRLCAsExt_ConstantEntries checks the constant fast path of
// EvaluateRLCAsExt against a naive per-row Horner fold, with constants in the
// first, middle and last positions and in every position but one.
func TestEvaluateRLCAsExt_ConstantEntries(t *testing.T) {
	const n = 8
	sys := wiop.NewSystemf("rlc-entries")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	c0 := mod.NewColumn(sys.Context.Childf("c0"), r0)
	c1 := mod.NewColumn(sys.Context.Childf("c1"), r0)

	rt := wiop.NewRuntime(sys)
	vals := map[*wiop.Column][]uint64{
		c0: {3, 1, 4, 1, 5, 9, 2, 6},
		c1: {2, 7, 1, 8, 2, 8, 1, 8},
	}
	for col, v := range vals {
		rt.AssignColumn(col, baseVec(v...))
	}
	k := func(v uint64) *wiop.Constant { return wiop.NewConstantVector(mod, field.NewElement(v)) }

	cases := map[string][]wiop.TableEntry{
		"const first":       {k(11), c0.View(), c1.View()},
		"const middle":      {c0.View(), k(12), c1.View()},
		"const last":        {c0.View(), c1.View(), k(13)},
		"consts around one": {k(14), c1.View(), k(15)},
		"single const":      {k(16)},
		"two trailing":      {c0.View(), k(17), k(18)},
		"run after gap":     {k(19), c0.View(), k(20), k(21), k(22)},
		"only consts":       {k(23), k(24), k(25)},
		"view then consts":  {c1.View(), k(26), k(27), k(28)},
	}
	alpha := field.RandomElementExt()
	for name, entries := range cases {
		got := wiop.EvaluateRLCAsExt(rt, alpha, entries, n)
		for row := range n {
			var want field.Ext
			for i := len(entries) - 1; i >= 0; i-- {
				var x field.Element
				if c, ok := wiop.EntryConst(entries[i]); ok {
					x = c
				} else {
					cv, _ := wiop.EntryView(entries[i])
					x.SetUint64(vals[cv.Column][row])
				}
				lifted := field.Lift(x)
				want.Mul(&want, &alpha)
				want.Add(&want, &lifted)
			}
			require.True(t, want.Equal(&got[row]), "%s: row %d", name, row)
		}
	}
}

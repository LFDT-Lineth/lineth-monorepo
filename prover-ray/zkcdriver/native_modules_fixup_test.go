package zkcdriver

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestNativeMulmodPaddingFixup checks that the fixup rewrites the all-zero
// Modulus rows to 1, padding rows included, and leaves every other row alone.
// In particular a Modulus whose limb 0 is zero but whose upper limb is not
// (2^16 * 3 here) is a valid modulus and must keep its value.
func TestNativeMulmodPaddingFixup(t *testing.T) {
	sys := wiop.NewSystemf("fixup")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	limb0 := mod.NewColumn(sys.Context.Childf("modulus-0"), r0)
	limb1 := mod.NewColumn(sys.Context.Childf("modulus-1"), r0)

	vec := func(vals ...uint64) *wiop.ConcreteVector {
		elems := make([]field.Element, len(vals))
		for i, v := range vals {
			elems[i].SetUint64(v)
		}
		return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	}

	// Three trace rows, then one padding row (the module is padded to 4).
	//            row:  0  1  2  (3 = padding)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(limb0, vec(0, 5, 0))
	rt.AssignColumn(limb1, vec(0, 0, 3))

	(&nativeMulmodPaddingFixup{modulus: []*wiop.Column{limb0, limb1}}).Run(rt)

	const nbRows = 4
	require.Equal(t, nbRows, mod.RuntimeSize(rt))
	got := func(col *wiop.Column) []uint64 {
		v := rt.GetColumnAssignment(col)
		out := make([]uint64, nbRows)
		for row := range out {
			e := v.ElementAtN(mod.Padding, nbRows, row).AsBase()
			out[row] = e.Uint64()
		}
		return out
	}
	require.Equal(t, []uint64{1, 5, 0, 1}, got(limb0), "limb 0: all-zero rows become 1")
	require.Equal(t, []uint64{0, 0, 3, 0}, got(limb1), "upper limbs are never rewritten")
}

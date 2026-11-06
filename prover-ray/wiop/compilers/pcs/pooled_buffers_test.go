package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// newPooledTestSystem is [newPCSTestSystem] over a column of n rows, large
// enough for the parallel Merkle levels.
func newPooledTestSystem(n int) (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("pcs-pooled")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys)
	return sys, col
}

// A proof following another one reuses its codewords and Merkle nodes, which
// the opening returned to the field pools holding stale values. It must be
// exactly the proof produced from freshly allocated buffers, and verify; and
// the earlier proof, whose buffers were reused, must be left unchanged, so it
// holds no reference into them.
func TestCompileReusesPooledBuffers(t *testing.T) {
	const n = 1 << 12
	sys, col := newPooledTestSystem(n)
	proofA, pubA := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, baseVec(n, 3)) })
	require.NoError(t, sys.Verify(proofA, pubA))
	proofB, pubB := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, baseVec(n, 5)) })
	require.NoError(t, sys.Verify(proofB, pubB), "a proof on pooled buffers must verify")

	field.DrainPools()
	fresh, freshCol := newPooledTestSystem(n)
	want, wantPub := fresh.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(freshCol, baseVec(n, 5)) })
	require.Equal(t, want, proofB, "a proof on pooled buffers must equal a fresh one")
	require.Equal(t, wantPub, pubB)

	field.DrainPools()
	freshA, freshACol := newPooledTestSystem(n)
	wantA, _ := freshA.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(freshACol, baseVec(n, 3)) })
	require.Equal(t, wantA, proofA, "a proof must not change when a later proof reuses its buffers")
	require.NoError(t, sys.Verify(proofA, pubA))
}

package wiop_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- baseQuery (via Vanishing) ----

func TestBaseQuery_IsReduced_MarkAsReduced(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("bqCol"), r0)
	v := mod.NewVanishing(sys.Context.Childf("bqV"), col.View())

	assert.False(t, v.IsReduced())
	v.MarkAsReduced()
	assert.True(t, v.IsReduced())
	v.MarkAsReduced() // idempotent
	assert.True(t, v.IsReduced())
}

func TestBaseQuery_Context(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("ctxCol"), r0)
	ctx := sys.Context.Childf("ctxV")
	v := mod.NewVanishing(ctx, col.View())
	assert.Equal(t, ctx, v.Context())
}

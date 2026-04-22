package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Visibility & PaddingDirection strings ----

func TestVisibility_String(t *testing.T) {
	cases := []struct {
		v    wiop.Visibility
		want string
	}{
		{wiop.VisibilityInternal, "Internal"},
		{wiop.VisibilityOracle, "Oracle"},
		{wiop.VisibilityPublic, "Public"},
		{wiop.Visibility(99), "Visibility(99)"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.v.String(), tc.want)
	}
}

func TestPaddingDirection_String(t *testing.T) {
	cases := []struct {
		pd   wiop.PaddingDirection
		want string
	}{
		{wiop.PaddingDirectionNone, "None"},
		{wiop.PaddingDirectionLeft, "Left"},
		{wiop.PaddingDirectionRight, "Right"},
		{wiop.PaddingDirection(99), "PaddingDirection(99)"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.pd.String(), tc.want)
	}
}

// ---- ObjectID ----

func TestObjectID_Encoding(t *testing.T) {
	cases := []struct {
		desc     string
		id       wiop.ObjectID
		wantKind wiop.ObjectKind
		wantSlot int
		wantPos  int
	}{
		{"column id=0,0", wiop.ObjectID(wiop.KindColumn) << 56, wiop.KindColumn, 0, 0},
		{"cell id=3,7", wiop.ObjectID(wiop.KindCell)<<56 | wiop.ObjectID(3)<<40 | wiop.ObjectID(7), wiop.KindCell, 3, 7},
		{"coin id=1,2", wiop.ObjectID(wiop.KindCoinField)<<56 | wiop.ObjectID(1)<<40 | wiop.ObjectID(2), wiop.KindCoinField, 1, 2},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantKind, tc.id.Kind())
			assert.Equal(t, tc.wantSlot, tc.id.Slot())
			assert.Equal(t, tc.wantPos, tc.id.Position())
		})
	}
}

func TestObjectKind_String(t *testing.T) {
	cases := []struct {
		k    wiop.ObjectKind
		want string
	}{
		{wiop.KindColumn, "Column"},
		{wiop.KindCell, "Cell"},
		{wiop.KindCoinField, "CoinField"},
		{wiop.ObjectKind(0xFF), "ObjectKind(0xff)"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.k.String())
	}
}

// ---- System construction ----

func TestNewSystemf(t *testing.T) {
	sys := wiop.NewSystemf("proto-%d", 1)
	require.NotNil(t, sys)
	assert.Equal(t, "proto-1", sys.Context.Path())
	require.NotNil(t, sys.PrecomputedRound)
	assert.Empty(t, sys.Rounds)
	assert.Empty(t, sys.Modules)
}

func TestSystem_NewRound(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	r0 := sys.NewRound()
	r1 := sys.NewRound()

	assert.Equal(t, 0, r0.ID)
	assert.Equal(t, 1, r1.ID)
	assert.Len(t, sys.Rounds, 2)
	assert.Equal(t, sys, r0.System())
}

func TestRound_PrevNext(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	r2 := sys.NewRound()

	// first round has no predecessor
	prev, ok := r0.Prev()
	assert.False(t, ok)
	assert.Nil(t, prev)

	// last round has no successor
	next, ok := r2.Next()
	assert.False(t, ok)
	assert.Nil(t, next)

	// middle round
	p, ok := r1.Prev()
	assert.True(t, ok)
	assert.Equal(t, r0, p)

	n, ok := r1.Next()
	assert.True(t, ok)
	assert.Equal(t, r2, n)
}

func TestPrecomputedRound_PrevNextPanic(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	assert.Panics(t, func() { sys.PrecomputedRound.Prev() })
	assert.Panics(t, func() { sys.PrecomputedRound.Next() })
}

// ---- Module construction ----

func TestSystem_NewModule(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	ctx := sys.Context.Childf("mod")
	m := sys.NewModule(ctx, wiop.PaddingDirectionRight)

	require.NotNil(t, m)
	assert.Equal(t, sys, m.System())
	assert.Equal(t, wiop.PaddingDirectionRight, m.Padding)
	assert.False(t, m.IsSized())
	assert.Equal(t, 0, m.Size())
	assert.Len(t, sys.Modules, 1)
}

func TestSystem_NewModule_NilCtxPanic(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	assert.Panics(t, func() { sys.NewModule(nil, wiop.PaddingDirectionNone) })
}

func TestSystem_NewSizedModule(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	ctx := sys.Context.Childf("mod")
	m := sys.NewSizedModule(ctx, 16, wiop.PaddingDirectionLeft)

	assert.True(t, m.IsSized())
	assert.Equal(t, 16, m.Size())
}

func TestModule_SetSize(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	ctx := sys.Context.Childf("mod")
	m := sys.NewModule(ctx, wiop.PaddingDirectionNone)

	m.SetSize(8)
	assert.Equal(t, 8, m.Size())

	// double-sizing panics
	assert.Panics(t, func() { m.SetSize(16) })
}

func TestModule_SetSize_InvalidPanic(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	ctx := sys.Context.Childf("mod")
	m := sys.NewModule(ctx, wiop.PaddingDirectionNone)
	assert.Panics(t, func() { m.SetSize(0) })
	assert.Panics(t, func() { m.SetSize(-1) })
}

// ---- System.LookupColumn/Cell/CoinField ----

func TestSystem_Lookup(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	r := sys.NewRound()
	modCtx := sys.Context.Childf("mod")
	mod := sys.NewSizedModule(modCtx, 4, wiop.PaddingDirectionNone)

	colCtx := modCtx.Childf("col")
	col := mod.NewColumn(colCtx, wiop.VisibilityOracle, r)

	cellCtx := sys.Context.Childf("cell")
	cell := r.NewCell(cellCtx, false)

	coinCtx := sys.Context.Childf("coin")
	coin := r.NewCoinField(coinCtx)

	// column lookup
	got := sys.LookupColumn(col.Context.ID)
	assert.Equal(t, col, got)

	// cell lookup
	gotCell := sys.LookupCell(cell.Context.ID)
	assert.Equal(t, cell, gotCell)

	// coin lookup
	gotCoin := sys.LookupCoinField(coin.Context.ID)
	assert.Equal(t, coin, gotCoin)
}

func TestSystem_Lookup_WrongKindPanic(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	r := sys.NewRound()
	modCtx := sys.Context.Childf("mod")
	mod := sys.NewSizedModule(modCtx, 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(modCtx.Childf("col"), wiop.VisibilityOracle, r)

	assert.Panics(t, func() { sys.LookupCell(col.Context.ID) })
	assert.Panics(t, func() { sys.LookupCoinField(col.Context.ID) })
}

func TestSystem_LookupColumn_WrongKindPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cellLookup"), false)
	assert.Panics(t, func() { sys.LookupColumn(cell.Context.ID) })
}

func TestModule_NewExtensionColumn_NilRoundPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewExtensionColumn(sys.Context.Childf("ext"), wiop.VisibilityOracle, nil) })
}

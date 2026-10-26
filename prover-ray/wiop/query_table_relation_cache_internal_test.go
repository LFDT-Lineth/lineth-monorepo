package wiop

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// constVec builds a PaddingDirectionNone ConcreteVector of length n whose every
// element is val.
func constVec(n int, val uint64) *ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(val)
	}
	return &ConcreteVector{Plain: field.VecFromBase(elems)}
}

// TestInclusionSetCache_SharedAcrossQueries pins the behaviour
// [inclusionSetCache] has to get right when [System.checkUnreducedQueries]
// hands the same cache to every table relation of a pass: a set may only be
// reused by a query whose target tables are the ones it was built from.
//
// Reuse is keyed on column identity, so the failure mode to guard against is a
// key that conflates two distinct targets — it would make a query pass or fail
// on another query's data. The three queries below share one cache and cover
// both directions: a second target must get its own set (or the passing query
// over it would fail), and a genuinely absent row must still be caught after a
// passing query has warmed the cache with the same target.
//
// It is white-box because the cache and checkWith are unexported; the public
// [TableRelationQuery.Check] always allocates a fresh cache and so never
// exercises sharing.
func TestInclusionSetCache_SharedAcrossQueries(t *testing.T) {
	sys := NewSystemf("incl-cache")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, PaddingDirectionNone)

	target1 := mod.NewColumn(sys.Context.Childf("target1"), r0)
	target2 := mod.NewColumn(sys.Context.Childf("target2"), r0)
	inTarget1 := mod.NewColumn(sys.Context.Childf("inTarget1"), r0)
	inTarget2 := mod.NewColumn(sys.Context.Childf("inTarget2"), r0)
	inNeither := mod.NewColumn(sys.Context.Childf("inNeither"), r0)

	newInc := func(name string, a, b *Column) *TableRelationQuery {
		return sys.NewInclusion(
			sys.Context.Childf("%s", name),
			[]Table{NewTable(a.View())},
			[]Table{NewTable(b.View())},
		)
	}
	incFirst := newInc("incFirst", inTarget1, target1)
	incOtherTarget := newInc("incOtherTarget", inTarget2, target2)
	incSameTarget := newInc("incSameTarget", inNeither, target1)

	rt := NewRuntime(sys)
	rt.AssignColumn(target1, constVec(4, 1))
	rt.AssignColumn(target2, constVec(4, 2))
	rt.AssignColumn(inTarget1, constVec(4, 1))
	rt.AssignColumn(inTarget2, constVec(4, 2))
	rt.AssignColumn(inNeither, constVec(4, 3))

	cache := newInclusionSetCache()

	require.NoError(t, incFirst.checkWith(rt, cache),
		"first query must pass on its own target")
	require.NoError(t, incOtherTarget.checkWith(rt, cache),
		"a query over a different target must not be served the first target's set")
	require.Error(t, incSameTarget.checkWith(rt, cache),
		"a row absent from the target must still be caught once the cache is warm")

	require.Len(t, cache.sets, 2,
		"the two distinct targets must yield exactly two cached sets, one reused by the third query")
}

// TestInclusionSetCache_ReusesSetForSharedTarget is the memoisation half of the
// contract: several queries over one target build that target's set once. This
// is the case that dominates lookup-heavy systems, where a single range-check
// column is the target of hundreds of queries.
func TestInclusionSetCache_ReusesSetForSharedTarget(t *testing.T) {
	sys := NewSystemf("incl-cache-reuse")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, PaddingDirectionNone)

	target := mod.NewColumn(sys.Context.Childf("target"), r0)
	rt := NewRuntime(sys)
	rt.AssignColumn(target, constVec(4, 1))

	cache := newInclusionSetCache()
	for i := range 3 {
		source := mod.NewColumn(sys.Context.Childf("source%d", i), r0)
		rt.AssignColumn(source, constVec(4, 1))
		inc := sys.NewInclusion(
			sys.Context.Childf("inc%d", i),
			[]Table{NewTable(source.View())},
			[]Table{NewTable(target.View())},
		)
		require.NoError(t, inc.checkWith(rt, cache))
	}

	require.Len(t, cache.sets, 1, "one target must be built once, however many queries read it")
}

// TestInclusionSetCache_ConstantsInKey checks that the cache key covers
// constant entries: two targets over the same column that differ only in a
// constant (or in constant-versus-column at one position) must not share a
// cached set, or one query would be checked against the other's rows.
func TestInclusionSetCache_ConstantsInKey(t *testing.T) {
	sys := NewSystemf("incl-cache-const")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, PaddingDirectionNone)

	target := mod.NewColumn(sys.Context.Childf("target"), r0)
	tag := mod.NewColumn(sys.Context.Childf("tag"), r0)
	in := mod.NewColumn(sys.Context.Childf("in"), r0)

	one, two := field.One(), field.NewElement(2)
	newInc := func(name string, a, b Table) *TableRelationQuery {
		return sys.NewInclusion(sys.Context.Childf("%s", name), []Table{a}, []Table{b})
	}
	incOne := newInc("incOne",
		NewTableEntries(mod, nil, in.View(), NewConstantVector(mod, one)),
		NewTableEntries(mod, nil, target.View(), NewConstantVector(mod, one)))
	incTwo := newInc("incTwo",
		NewTableEntries(mod, nil, in.View(), NewConstantVector(mod, one)),
		NewTableEntries(mod, nil, target.View(), NewConstantVector(mod, two)))
	incCol := newInc("incCol",
		NewTableEntries(mod, nil, in.View(), NewConstantVector(mod, one)),
		NewTable(target.View(), tag.View()))

	for _, tables := range [][]Table{incOne.B, incTwo.B, incCol.B} {
		_, ok := inclusionSetKey(tables)
		require.True(t, ok)
	}
	k1, _ := inclusionSetKey(incOne.B)
	logrus.Printf("k1 = %+v", k1)
	k2, _ := inclusionSetKey(incTwo.B)
	k3, _ := inclusionSetKey(incCol.B)
	require.NotEqual(t, k1, k2)
	require.NotEqual(t, k1, k3)

	rt := NewRuntime(sys)
	rt.AssignColumn(target, constVec(4, 3))
	logrus.Printf("target = %+v", target.At(0))
	rt.AssignColumn(tag, constVec(4, 1))
	rt.AssignColumn(in, constVec(4, 3))

	cache := newInclusionSetCache()
	require.NoError(t, incOne.checkWith(rt, cache), "(3, 1) is in the target with constant 1")
	require.Error(t, incTwo.checkWith(rt, cache),
		"the constant-2 target must not be served the constant-1 target's set")
	require.NoError(t, incCol.checkWith(rt, cache), "(3, 1) is in (target, tag)")
	require.Len(t, cache.sets, 3)
}

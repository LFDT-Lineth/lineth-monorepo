package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/stretchr/testify/require"
)

// testPITag is the base tag every public input in this file is registered under.
// The tag set is caller-defined, so a test declares its own; a test registering
// more than one public input suffixes it, since tags must be unique.
const testPITag wiop.PublicInputTag = "TestPublicInput"

// piVec builds a base-field ConcreteVector from the given values.
func piVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// piGen builds a base-field scalar value.
func piGen(v uint64) field.Gen {
	var e field.Element
	e.SetUint64(v)
	return field.ElemFromBase(e)
}

// TestPublicInput exercises the public-input flow: a public value is exposed by
// opening a column position into a cell (the opening also registers a local
// constraint binding cell == col[pos]), the cell is the sole registered public
// input, and the system is compiled so the binding constraint becomes a
// verifier check. The statement is an ordered []field.Gen aligned to
// registration order.
func TestPublicInput(t *testing.T) {
	const (
		n   = 4
		pos = 2 // col[2] == 30 below
	)

	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("pi-cells")
		r0 := sys.NewRound()
		m := sys.NewSizedModule(sys.Context.Childf("m"), n, wiop.PaddingDirectionNone)
		col := m.NewColumn(sys.Context.Childf("col"), r0)
		// Open col[pos] into a cell; Open registers the local constraint
		// cell == col[pos], which soundly binds the public input into the proof.
		cell := col.At(pos).Open(sys.Context.Childf("my-public-input"))
		sys.RegisterPublicInputs(testPITag, cell)
		localvanishing.Compile(sys)
		global.Compile(sys)
		return sys, col, cell
	}

	sys, col, cell := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, piVec(10, 20, 30, 40))
	})

	// The public value lives only in the statement (in registration order),
	// never in the proof.
	require.Len(t, pub, 1)
	require.Equal(t, piGen(30), pub[0], "opened cell must equal col[pos]")
	require.NotContains(t, proof.Cells, cell.Context.ID, "public-input cell must not be in the proof")

	// Honest statement verifies.
	require.NoError(t, sys.Verify(proof, pub))

	// A statement of the wrong length (missing / extra) is rejected.
	require.Error(t, sys.Verify(proof, wiop.PublicInput{}))
	require.Error(t, sys.Verify(proof, wiop.PublicInput{pub[0], piGen(0)}))

	// A wrong public value breaks the cell == col[pos] binding and is rejected.
	require.Error(t, sys.Verify(proof, wiop.PublicInput{piGen(99)}), "tampered public input must be rejected")

	// A proof that smuggles the public-input cell back in is rejected.
	sys2, col2, cell2 := build()
	proof2, pub2 := sys2.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col2, piVec(10, 20, 30, 40))
	})
	proof2.Cells[cell2.Context.ID] = pub2[0]
	require.Error(t, sys2.Verify(proof2, pub2), "public-input cell must not appear in the proof")
}

// TestPublicInputDynamicColumn is the same flow as [TestPublicInput] but the
// opened column lives on a dynamic-size module: its domain size is inferred from
// the assignment at prove time and round-trips through Proof.DynamicSizes, while
// the public input is still exposed as the opened cell.
func TestPublicInputDynamicColumn(t *testing.T) {
	const pos = 0 // col[0] == 30 below

	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("pi-dyn")
		r0 := sys.NewRound()
		m := sys.NewDynamicModule(sys.Context.Childf("m"), wiop.PaddingDirectionRight)
		col := m.NewColumn(sys.Context.Childf("col"), r0)
		cell := col.At(pos).Open(sys.Context.Childf("open"))
		sys.RegisterPublicInputs(testPITag, cell)
		localvanishing.Compile(sys)
		global.Compile(sys)
		return sys, col, cell
	}

	sys, col, cell := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		// Length 4 (a power of two): the dynamic module's size is inferred as 4.
		rt.AssignColumn(col, piVec(30, 20, 10, 40))
	})

	// The dynamic module's runtime size round-trips in the proof.
	require.Equal(t, 4, proof.DynamicSizes[col.Context.ID.Slot()], "dynamic module size must round-trip")

	// The public value lives only in the statement, never in the proof.
	require.Len(t, pub, 1)
	require.Equal(t, piGen(30), pub[0], "opened cell must equal col[0]")
	require.NotContains(t, proof.Cells, cell.Context.ID)

	// Honest statement verifies; a wrong public value breaks the cell == col[0]
	// binding and is rejected.
	require.NoError(t, sys.Verify(proof, pub))
	require.Error(t, sys.Verify(proof, wiop.PublicInput{piGen(99)}),
		"tampered public input must be rejected")
}

// TestPublicInputOrder checks that the flat PublicInput vector follows
// registration order, and that LookupPublicInput reports the matching position.
func TestPublicInputOrder(t *testing.T) {
	sys := wiop.NewSystemf("order")
	r0 := sys.NewRound()
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(sys.Context.Childf("col"), r0)
	cellA := col.At(0).Open(sys.Context.Childf("a"))
	cellB := col.At(1).Open(sys.Context.Childf("b"))
	// Registered in reverse column order: the statement follows registration
	// order, not declaration order. The suffixes make the two tags distinct; they
	// do not affect the positions, which come from the call order.
	sys.RegisterPublicInputs(testPITag, cellB, 0)
	sys.RegisterPublicInputs(testPITag, cellA, 1)
	localvanishing.Compile(sys)
	global.Compile(sys)

	_, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, piVec(10, 20, 30, 40))
	})

	require.Equal(t, wiop.PublicInput{piGen(20), piGen(10)}, pub)

	gotB, posB := sys.LookupPublicInput(cellB.Context.ID)
	require.Equal(t, cellB, gotB)
	require.Equal(t, 0, posB)

	gotA, posA := sys.LookupPublicInput(cellA.Context.ID)
	require.Equal(t, cellA, gotA)
	require.Equal(t, 1, posA)
}

// TestPublicInputTag checks that registration tags the cell under the reserved
// key and that the tag is readable back.
func TestPublicInputTag(t *testing.T) {
	sys := wiop.NewSystemf("pi-tag")
	r0 := sys.NewRound()
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(sys.Context.Childf("col"), r0)

	// An unregistered cell carries no tag.
	plain := col.At(3).Open(sys.Context.Childf("plain"))
	_, ok := wiop.PublicInputTagOf(plain)
	require.False(t, ok, "an unregistered cell must not carry a tag")

	cell := col.At(0).Open(sys.Context.Childf("open"))
	sys.RegisterPublicInputs(testPITag, cell)

	tag, ok := wiop.PublicInputTagOf(cell)
	require.True(t, ok)
	require.Equal(t, testPITag, tag)
	require.Equal(t, testPITag, cell.Annotations[wiop.PublicInputTagKey],
		"the tag must live under the reserved annotation key")
}

// TestLookupPublicInputByTag checks the tag-to-cell retrieval path: a suffixed
// role is found by (base tag, index), an unsuffixed one by its bare tag, the
// returned position is the cell's index in the flat vector, and an unknown tag
// reports a miss instead of a wrong cell.
func TestLookupPublicInputByTag(t *testing.T) {
	const (
		vecTag    wiop.PublicInputTag = "Vector"
		scalarTag wiop.PublicInputTag = "Scalar"
	)

	sys := wiop.NewSystemf("pi-by-tag")
	r0 := sys.NewRound()
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(sys.Context.Childf("col"), r0)

	// A three-cell role registered under suffixes, then a single-cell role
	// registered bare. The scalar goes last so its position differs from any
	// suffix, which would otherwise let an index/position mix-up pass.
	vec := make([]*wiop.Cell, 3)
	for i := range vec {
		vec[i] = col.At(i).Open(sys.Context.Childf("vec-%d", i))
		sys.RegisterPublicInputs(vecTag, vec[i], i)
	}
	scalar := col.At(3).Open(sys.Context.Childf("scalar"))
	sys.RegisterPublicInputs(scalarTag, scalar)

	for i, want := range vec {
		got, pos := sys.LookupPublicInputByTag(vecTag, i)
		require.Equal(t, want, got, "Vector_%d must resolve to the cell registered under it", i)
		require.Equal(t, i, pos)
	}

	gotScalar, posScalar := sys.LookupPublicInputByTag(scalarTag)
	require.Equal(t, scalar, gotScalar)
	require.Equal(t, 3, posScalar, "position is the index in the flat vector, not the suffix")

	// The suffix is spelled "_<n>", so the literal tag finds the same cell.
	gotLiteral, posLiteral := sys.LookupPublicInputByTag("Vector_1")
	require.Equal(t, vec[1], gotLiteral)
	require.Equal(t, 1, posLiteral)

	// Misses: an unused base tag, an out-of-range suffix, and a suffix on a role
	// that was registered bare.
	for _, miss := range []struct {
		name   string
		lookup func() (*wiop.Cell, int)
	}{
		{"unknown tag", func() (*wiop.Cell, int) { return sys.LookupPublicInputByTag("Nope") }},
		{"bare base tag of a suffixed role", func() (*wiop.Cell, int) { return sys.LookupPublicInputByTag(vecTag) }},
		{"out-of-range suffix", func() (*wiop.Cell, int) { return sys.LookupPublicInputByTag(vecTag, 3) }},
		{"suffix on an unsuffixed role", func() (*wiop.Cell, int) { return sys.LookupPublicInputByTag(scalarTag, 0) }},
	} {
		cell, pos := miss.lookup()
		require.Nil(t, cell, "%s must not resolve", miss.name)
		require.Equal(t, -1, pos, "%s must report -1", miss.name)
	}
}

// TestPublicInputTagUniqueness pins the tag namespace rules: a tag identifies
// exactly one public input, so a second cell claiming it is rejected — which is
// why a multi-cell role must be suffixed. It also pins the two sharp edges of
// building the tag by string concatenation.
func TestPublicInputTagUniqueness(t *testing.T) {
	newSys := func(name string) (*wiop.System, *wiop.Column) {
		sys := wiop.NewSystemf("pi-unique-%s", name)
		r0 := sys.NewRound()
		m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
		return sys, m.NewColumn(sys.Context.Childf("col"), r0)
	}

	// Two distinct cells cannot share a tag: this is what forces a 32-byte hash
	// to be registered as Hash_0 … Hash_7 rather than eight cells under "Hash".
	sysDup, colDup := newSys("dup")
	sysDup.RegisterPublicInputs("Hash", colDup.At(0).Open(sysDup.Context.Childf("a")))
	require.Panics(t, func() {
		sysDup.RegisterPublicInputs("Hash", colDup.At(1).Open(sysDup.Context.Childf("b")))
	}, "a second cell under the same tag must be rejected")
	require.Len(t, sysDup.PublicInputs, 1, "a rejected registration must not append")

	// Suffixing is plain concatenation, so a literal Foo_1 collides with Foo
	// suffixed by 1. Base tags must therefore not end in "_<int>".
	sysCollide, colCollide := newSys("collide")
	sysCollide.RegisterPublicInputs("Foo", colCollide.At(0).Open(sysCollide.Context.Childf("a")), 1)
	require.Panics(t, func() {
		sysCollide.RegisterPublicInputs("Foo_1", colCollide.At(1).Open(sysCollide.Context.Childf("b")))
	}, "a literal Foo_1 must collide with Foo suffixed by 1")

	// Only the first numeric suffix is used; the rest are silently dropped.
	sysExtra, colExtra := newSys("extra")
	cellExtra := colExtra.At(0).Open(sysExtra.Context.Childf("a"))
	sysExtra.RegisterPublicInputs("Bar", cellExtra, 7, 9, 11)
	tag, ok := wiop.PublicInputTagOf(cellExtra)
	require.True(t, ok)
	require.Equal(t, wiop.PublicInputTag("Bar_7"), tag, "extra numeric suffixes must be ignored")
	found, _ := sysExtra.LookupPublicInputByTag("Bar", 7)
	require.Equal(t, cellExtra, found, "lookup must apply the suffix the same way")
}

// TestRegisterPublicInputGuards checks that RegisterPublicInputs rejects bad
// registrations immediately (not deferred to Prove time): an untagged call, a
// re-registration of the same cell, a foreign cell, a nil cell, and a cell whose
// reserved key was written behind the API's back.
func TestRegisterPublicInputGuards(t *testing.T) {
	// newPICell builds a fresh single-cell system, so each guard below starts
	// from an unregistered, untagged cell.
	newPICell := func(name string) (*wiop.System, *wiop.Cell) {
		sys := wiop.NewSystemf("pi-guards-%s", name)
		r0 := sys.NewRound()
		m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
		col := m.NewColumn(sys.Context.Childf("col"), r0)
		return sys, col.At(0).Open(sys.Context.Childf("open"))
	}

	// The tag is mandatory: an empty one would leave the position anonymous.
	sysEmpty, cellEmpty := newPICell("empty-tag")
	require.Panics(t, func() {
		sysEmpty.RegisterPublicInputs("", cellEmpty)
	}, "empty tag must be rejected")
	require.Empty(t, sysEmpty.PublicInputs, "a rejected registration must not append")

	// Same cell in two separate calls, under the same tag and under a different
	// one: both are duplicate registrations.
	sysDup, cellDup := newPICell("dup")
	sysDup.RegisterPublicInputs(testPITag, cellDup)
	require.Panics(t, func() {
		sysDup.RegisterPublicInputs(testPITag, cellDup)
	}, "duplicate across calls must be rejected")
	require.Panics(t, func() {
		sysDup.RegisterPublicInputs("OtherTag", cellDup)
	}, "re-tagging an already-registered cell must be rejected")
	require.Len(t, sysDup.PublicInputs, 1)

	// Same cell twice back to back.
	sysTwice, cellTwice := newPICell("twice")
	require.Panics(t, func() {
		sysTwice.RegisterPublicInputs(testPITag, cellTwice)
		sysTwice.RegisterPublicInputs(testPITag, cellTwice)
	}, "re-registering the same cell must be rejected")

	// A cell belonging to another system.
	sysHost, _ := newPICell("host")
	_, foreign := newPICell("foreign")
	require.Panics(t, func() {
		sysHost.RegisterPublicInputs(testPITag, foreign)
	}, "foreign cell must be rejected")

	// Nil cells.
	require.Panics(t, func() {
		sysHost.RegisterPublicInputs(testPITag, nil)
	}, "nil cell must be rejected")

	// A cell pre-annotated on the reserved key, bypassing the API: registering it
	// would silently overwrite a role somebody else believes in.
	sysPre, cellPre := newPICell("pre-annotated")
	cellPre.Annotations[wiop.PublicInputTagKey] = wiop.PublicInputTag("SetByHand")
	require.Panics(t, func() {
		sysPre.RegisterPublicInputs(testPITag, cellPre)
	}, "pre-annotated cell must be rejected")

	// A foreign value under the reserved key is a bug, not a missing tag.
	sysJunk, cellJunk := newPICell("junk-annotation")
	cellJunk.Annotations[wiop.PublicInputTagKey] = 42
	require.Panics(t, func() {
		wiop.PublicInputTagOf(cellJunk)
	}, "a non-tag value under the reserved key must panic")
	require.Empty(t, sysJunk.PublicInputs)
}

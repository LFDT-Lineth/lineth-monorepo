package wioptest

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Tweak transforms an honest field value into a corrupted one. To make a
// soundness test meaningful it must return a value distinct from its input;
// a Tweak that returns its argument unchanged makes the test vacuous.
type Tweak func(field.Gen) field.Gen

// AddOne adds one to the targeted value — the smallest change that is
// non-trivial in any field. It is handy for exact-equality constraints but can
// leave a value inside a range or lookup table, where [RandomValue] is more
// reliable.
func AddOne(v field.Gen) field.Gen { return v.Add(field.ElemOne()) }

// RandomValue is the default [Tweak]: it replaces the value with a pseudo-random
// field element of the same kind (base or extension). Unlike [AddOne] it does
// not cluster near the honest value, so it reliably escapes range bounds and
// lookup tables.
//
// The RNG is seeded from the input value, so RandomValue is a pure function:
// the same honest value always maps to the same corrupted value. This keeps
// mutation runs reproducible and independent of evaluation order, without any
// shared RNG state. The astronomically unlikely event that the draw equals the
// input yields a no-op mutation, which the surrounding tester simply records as
// "not caught".
func RandomValue(v field.Gen) field.Gen {
	// A deterministic, input-seeded RNG is intentional: it keeps mutation runs
	// reproducible. This is test tooling, not a security context.
	rng := rand.New(rand.NewChaCha8(seedFromGen(v))) //nolint:gosec // G404: deterministic RNG by design
	if v.IsBase() {
		return field.PseudoRandElemBase(rng)
	}
	return field.PseudoRandElemExt(rng)
}

// Mutator is a soundness-testing pass over a [wiop.System]. Like a wiop
// compiler it is applied to a System and registers a [wiop.ProverAction], but
// instead of advancing the protocol that action corrupts a single honest
// assignment at runtime. A sound constraint system must then reject the mutated
// witness — if the verifier still accepts, either the targeted value is
// unconstrained or the system has a soundness gap worth flagging.
//
// Exactly one of Column or Cell must be non-nil. Tweak defaults to [RandomValue]
// when nil.
//
// For a column target, the corrupted position is one of:
//   - a Plain entry selected by Row. Row may be negative to index from the end,
//     so Row == -1 is the last assigned (non-padding) value — convenient for
//     dynamic columns whose length is not known at registration time;
//   - the padding fill value, when Padding is true (Row is then ignored). This
//     reaches the rows a padded column synthesises beyond its assigned data.
//
// Two entry points corrupt the value, differing only in timing:
//   - [Mutator.Compile] registers a prover action on the target's round, so the
//     tweak runs while [RunAndVerify] drives the protocol — before the round is
//     folded into the Fiat-Shamir transcript. Use this for pipeline tests.
//   - [Mutator.Apply] corrupts the value immediately. Use this when driving a
//     Runtime manually, e.g. between an honest assignment and a query's Check.
type Mutator struct {
	// Column is the targeted column, or nil when targeting a Cell.
	Column *wiop.Column
	// Row is the Plain entry within Column to corrupt; negative values index
	// from the end (-1 == last). Ignored for cell or padding targets.
	Row int
	// Padding, when true, corrupts the column's padding fill value rather than a
	// Plain entry. Valid only for a Column target.
	Padding bool
	// Cell is the targeted cell, or nil when targeting a Column.
	Cell *wiop.Cell
	// Tweak maps the honest value to its corrupted replacement. Defaults to
	// [RandomValue] when nil.
	Tweak Tweak
}

// Compile registers a prover action that runs [Mutator.Apply] on the target's
// round. It is the compiler-shaped entry point, so a Mutator can be appended to
// a pipeline of other Compile passes.
//
// Panics if sys is nil or unless exactly one of Column or Cell is set.
func (m Mutator) Compile(sys *wiop.System) {
	if sys == nil {
		panic("wioptest: Mutator.Compile requires a non-nil System")
	}
	m.validate()
	m.targetRound().RegisterAction(mutationAction{m})
}

// Apply corrupts the targeted value in rt using [wiop.Runtime.OverrideColumn] /
// [wiop.Runtime.OverrideCell]. The target must already be assigned.
//
// Panics unless exactly one of Column or Cell is set, or if a column Row is out
// of bounds.
func (m Mutator) Apply(rt wiop.Runtime) {
	m.validate()
	tweak := m.Tweak
	if tweak == nil {
		tweak = RandomValue
	}

	if m.Cell != nil {
		rt.OverrideCell(m.Cell, tweak(rt.GetCellValue(m.Cell)))
		return
	}

	cv := rt.GetColumnAssignment(m.Column)

	if m.Padding {
		mutated := tweak(field.ElemFromBase(cv.Padding))
		if !mutated.IsBase() {
			panic("wioptest: column padding must remain a base field element")
		}
		rt.OverrideColumn(m.Column, &wiop.ConcreteVector{
			Plain:   cv.Plain,
			Padding: mutated.AsBase(),
		})
		return
	}

	plain := cv.Plain
	row := m.resolveRow(plain.Len())
	mutated := tweak(genAt(plain, row))
	rt.OverrideColumn(m.Column, &wiop.ConcreteVector{
		Plain:   cloneVecWith(plain, row, mutated),
		Padding: cv.Padding,
	})
}

func (m Mutator) validate() {
	if (m.Column == nil) == (m.Cell == nil) {
		panic("wioptest: Mutator must target exactly one of Column or Cell")
	}
	if m.Padding && m.Column == nil {
		panic("wioptest: Mutator.Padding is only valid for a Column target")
	}
}

func (m Mutator) targetRound() *wiop.Round {
	if m.Column != nil {
		return m.Column.Round()
	}
	return m.Cell.Round()
}

// resolveRow turns m.Row (which may be negative) into an index in [0, n).
func (m Mutator) resolveRow(n int) int {
	row := m.Row
	if row < 0 {
		row += n
	}
	if row < 0 || row >= n {
		panic(fmt.Sprintf(
			"wioptest: column mutation row %d out of bounds for assigned length %d (column %q)",
			m.Row, n, m.Column.Context.Path(),
		))
	}
	return row
}

// String describes the targeted position for diagnostics, e.g.
// "column mod/col [row 3]", "column mod/col [padding]", or "cell le/claim".
func (m Mutator) String() string {
	switch {
	case m.Cell != nil:
		return fmt.Sprintf("cell %s", m.Cell.Context.Path())
	case m.Column != nil && m.Padding:
		return fmt.Sprintf("column %s [padding]", m.Column.Context.Path())
	case m.Column != nil:
		return fmt.Sprintf("column %s [row %d]", m.Column.Context.Path(), m.Row)
	default:
		return "<empty mutator>"
	}
}

// mutationAction runs a Mutator's tweak as a prover action.
type mutationAction struct{ m Mutator }

func (a mutationAction) Run(rt wiop.Runtime) { a.m.Apply(rt) }

// genAt reads entry i of v as a [field.Gen], preserving its field kind.
func genAt(v field.Vec, i int) field.Gen {
	if v.IsBase() {
		return field.ElemFromBase(v.AsBase()[i])
	}
	return field.ElemFromExt(v.AsExt()[i])
}

// cloneVecWith returns a copy of v with entry row replaced by val. The field
// kind of v is preserved: storing an extension value into a base vector is
// rejected to avoid silently widening the column.
func cloneVecWith(v field.Vec, row int, val field.Gen) field.Vec {
	if v.IsBase() {
		if !val.IsBase() {
			panic("wioptest: cannot store an extension value into a base column")
		}
		src := v.AsBase()
		cp := make([]field.Element, len(src))
		copy(cp, src)
		cp[row] = val.AsBase()
		return field.VecFromBase(cp)
	}
	src := v.AsExt()
	cp := make([]field.Ext, len(src))
	copy(cp, src)
	cp[row] = val.AsExt()
	return field.VecFromExt(cp)
}

// seedFromGen derives a deterministic ChaCha8 seed from the six base
// coordinates of v's canonical extension storage, so [RandomValue] is a pure
// function of its input.
func seedFromGen(v field.Gen) [32]byte {
	x := v.AsExt()
	coords := [6]uint32{
		x.B0.A0[0], x.B0.A1[0],
		x.B1.A0[0], x.B1.A1[0],
		x.B2.A0[0], x.B2.A1[0],
	}
	var seed [32]byte
	for i, c := range coords {
		binary.LittleEndian.PutUint32(seed[i*4:], c)
	}
	return seed
}

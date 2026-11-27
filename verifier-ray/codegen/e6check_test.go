package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
)

// e6PublicProgram compiles a recursion program exposing a scalar, an E6 input
// and an E6 product as public inputs, and returns it with an execution and the
// addresses of X and Y.
func e6PublicProgram(t *testing.T) (*recursion.Program, *recursion.Run, [2]int) {
	t.Helper()
	m := &recursion.Machine{}
	s, x := m.InputScalar(), m.InputE6()
	y := m.Mul(s, x)
	m.Public(s, recursion.KindScalar, "S", 0)
	m.Public(x, recursion.KindE6, "X", 0)
	m.Public(y, recursion.KindE6, "Y", 0)
	p, err := m.Compile(wioptest.TestingCompileOptions()...)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var xv recursion.Cell
	for i := range recursion.KindE6 {
		xv[i].SetUint64(uint64(i) + 11)
	}
	r, err := p.Execute([]recursion.Cell{recursion.ScalarCell(field.NewElement(4)), xv})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return p, r, [2]int{x, y}
}

// TestE6CheckHandled checks that a recursion program exposing E6 public inputs
// passes the fail-closed verifier-action guard.
func TestE6CheckHandled(t *testing.T) {
	p, _, _ := e6PublicProgram(t)
	if err := AssertAllVerifierActionsHandled(p.Sys); err != nil {
		t.Fatalf("AssertAllVerifierActionsHandled: %v", err)
	}
}

// TestBuildE6CheckSystem checks that every E6 public input yields one entry
// whose refs resolve, in the prover's transcript, to the public input and to
// six base lanes recomposing to it — the identity the Zig sub-verifier checks.
func TestBuildE6CheckSystem(t *testing.T) {
	p, r, addrs := e6PublicProgram(t)
	sys, err := BuildE6CheckSystem(p.Sys)
	if err != nil {
		t.Fatalf("BuildE6CheckSystem: %v", err)
	}
	if len(sys.Checks) != 2 {
		t.Fatalf("got %d checks, want 2 (X and Y; the scalar S has none)", len(sys.Checks))
	}

	proof, pub := p.Prove(r)
	piPos := make(map[wiop.ObjectID]int, len(p.Sys.PublicInputs))
	for i, c := range p.Sys.PublicInputs {
		piPos[c.Context.ID] = i
	}
	cellAt := func(ref ScalarCellRef) field.Gen {
		t.Helper()
		if ref.Round >= len(p.Sys.Rounds) || ref.Index >= len(p.Sys.Rounds[ref.Round].Cells) {
			t.Fatalf("ref %+v out of range", ref)
		}
		cell := p.Sys.Rounds[ref.Round].Cells[ref.Index]
		if i, ok := piPos[cell.Context.ID]; ok {
			return pub[i]
		}
		v, ok := proof.Cells[cell.Context.ID]
		if !ok {
			t.Fatalf("ref %+v: cell not in the proof", ref)
		}
		return v
	}

	for k, tag := range []wiop.PublicInputTag{"X", "Y"} {
		cell, pos := p.Sys.LookupPublicInputByTag(tag, 0)
		if pos < 0 {
			t.Fatalf("%s not registered", tag)
		}
		entry := sys.Checks[k]
		if entry.Cell != cellRef(cell) {
			t.Fatalf("%s: check %d names cell %+v, want the public input at %+v", tag, k, entry.Cell, cellRef(cell))
		}
		var lanes recursion.Cell
		for l, ref := range entry.Lanes {
			v := cellAt(ref)
			if !v.IsBase() {
				t.Fatalf("%s: lane %d is not base", tag, l)
			}
			lanes[l] = v.AsBase()
		}
		want, got := recursion.CellE6(lanes), cellAt(entry.Cell).AsExt()
		if !want.Equal(&got) {
			t.Fatalf("%s: lanes do not recompose to the public input", tag)
		}
		if want := recursion.CellE6(r.Value(addrs[k])); !want.Equal(&got) {
			t.Fatalf("%s: public input is not the program's value", tag)
		}
	}
}

// TestBuildE6CheckSystemRejectsMalformed checks that an action the Zig
// sub-verifier could not check like the Go one is rejected at generation time.
func TestBuildE6CheckSystemRejectsMalformed(t *testing.T) {
	sys := wiop.NewSystemf("malformed")
	round := sys.NewRound()
	last := round.EnsureNext()
	ext := round.NewCell(sys.Context.Childf("ext"), true)
	base := round.NewCell(sys.Context.Childf("base"), false)
	lastExt := last.NewCell(sys.Context.Childf("last-ext"), true)
	lastBase := last.NewCell(sys.Context.Childf("last-base"), false)
	six := func(c *wiop.Cell) []*wiop.Cell {
		return []*wiop.Cell{c, c, c, c, c, c}
	}
	lastSlot := len(sys.Rounds) - 1
	if _, err := buildE6CheckEntry(&recursion.E6Check{Cell: ext, Lanes: six(base)}, lastSlot); err != nil {
		t.Fatalf("well-formed check rejected: %v", err)
	}
	cases := map[string]*recursion.E6Check{
		"base cell":          {Cell: base, Lanes: six(base)},
		"five lanes":         {Cell: ext, Lanes: six(base)[:5]},
		"extension lane":     {Cell: ext, Lanes: six(ext)},
		"nil lane":           {Cell: ext, Lanes: []*wiop.Cell{base, base, nil, base, base, base}},
		"cell in last round": {Cell: lastExt, Lanes: six(base)},
		"lane in last round": {Cell: ext, Lanes: []*wiop.Cell{base, base, base, base, base, lastBase}},
	}
	for name, check := range cases {
		if _, err := buildE6CheckEntry(check, lastSlot); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// TestWriteE6CheckSystemZig checks the emitted Zig data.
func TestWriteE6CheckSystemZig(t *testing.T) {
	sys := E6CheckSystem{SourceName: "s", Checks: []E6CheckEntry{{
		Cell:  ScalarCellRef{Round: 0, Index: 9},
		Lanes: [recursion.KindE6]ScalarCellRef{{0, 1}, {0, 2}, {0, 3}, {0, 4}, {0, 5}, {0, 6}},
	}}}
	var buf bytes.Buffer
	if err := WriteE6CheckSystemZig(&buf, 3, sys); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`const e6check = @import("../query/e6check.zig");`,
		`.{ .cell = .{ .round = 0, .index = 9 }, .lanes = .{ .{ .round = 0, .index = 1 }, .{ .round = 0, .index = 2 }, .{ .round = 0, .index = 3 }, .{ .round = 0, .index = 4 }, .{ .round = 0, .index = 5 }, .{ .round = 0, .index = 6 } } },`,
		`const system_3_e6check = e6check.System{ .checks = &system_3_e6check_checks };`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q in:\n%s", want, buf.String())
		}
	}
}

package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
)

func publicInputSystem(name string) *wiop.System {
	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	cell := col.At(2).Open(sys.Context.Childf("public"))
	sys.RegisterPublicInputs("PublicInput", cell)
	localvanishing.Compile(sys)
	global.Compile(sys)
	return sys
}

func TestBuildPublicInputSystem(t *testing.T) {
	publicInput, err := BuildPublicInputSystem(publicInputSystem("public-input"))
	if err != nil {
		t.Fatalf("BuildPublicInputSystem() error = %v", err)
	}

	if got, want := len(publicInput.Refs), 1; got != want {
		t.Fatalf("public input count = %d, want %d", got, want)
	}
	if got, want := publicInput.RoundCellCounts[0], 1; got < want {
		t.Fatalf("round 0 cell count = %d, want >= %d", got, want)
	}

	assertCellExtBits(t, publicInputSystem("public-input"), publicInput)

	ref := publicInput.Refs[0]
	if ref.StatementIndex != 0 {
		t.Fatalf("statement index = %d, want 0", ref.StatementIndex)
	}
	if ref.Round != 0 {
		t.Fatalf("round = %d, want 0", ref.Round)
	}
	if ref.Index != 0 {
		t.Fatalf("index = %d, want 0", ref.Index)
	}
}

func TestWritePublicInputSystemZig(t *testing.T) {
	publicInput := PublicInputSystem{
		RoundCellCounts: []int{1, 0},
		CellExtBits:     []uint64{0x1},
		Refs: []PublicInputRef{
			{StatementIndex: 0, Round: 0, Index: 0},
		},
	}

	var buf bytes.Buffer
	if err := WritePublicInputSystemZig(&buf, publicInput); err != nil {
		t.Fatalf("WritePublicInputSystemZig() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`const protocol = @import("../protocol/root.zig");`,
		"pub const public_input = protocol.public_input.Spec{",
		".round_cell_counts = &[_]usize{ 1, 0 },",
		".cell_ext_bits = &[_]u64{ 0x1 },",
		".refs = &[_]protocol.public_input.CellRef{ .{ .statement_index = 0, .round = 0, .index = 0 } },",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated public-input spec missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestBuildPublicInputSystemCellExtBits checks the bit set on a system whose
// replayed rounds mix base and extension cells, spread over more than one
// 64-bit word and more than one round.
func TestBuildPublicInputSystemCellExtBits(t *testing.T) {
	sys := wiop.NewSystemf("cell-ext-bits")
	r0, r1 := sys.NewRound(), sys.NewRound()
	sys.NewRound()
	for i := range 70 {
		r0.NewCell(sys.Context.Childf("r0-%d", i), i%3 == 0)
	}
	for i := range 5 {
		r1.NewCell(sys.Context.Childf("r1-%d", i), i == 4)
	}
	publicInput, err := BuildPublicInputSystem(sys)
	if err != nil {
		t.Fatalf("BuildPublicInputSystem() error = %v", err)
	}
	if got, want := len(publicInput.CellExtBits), 2; got != want {
		t.Fatalf("75 cells need %d words, got %d", want, got)
	}
	assertCellExtBits(t, sys, publicInput)
}

// assertCellExtBits checks that every replayed cell's bit matches its
// declaration, and that no bit is set past the last cell.
func assertCellExtBits(t *testing.T, sys *wiop.System, publicInput PublicInputSystem) {
	t.Helper()
	flat := 0
	for _, round := range sys.Rounds[:len(sys.Rounds)-1] {
		for _, cell := range round.Cells {
			got := publicInput.CellExtBits[flat/64]>>(flat%64)&1 == 1
			if got != cell.IsExtension() {
				t.Fatalf("cell %q (flat index %d): bit = %v, declared extension = %v",
					cell.Context.Path(), flat, got, cell.IsExtension())
			}
			flat++
		}
	}
	if want := (flat + 63) / 64; len(publicInput.CellExtBits) != want {
		t.Fatalf("%d cells need %d words, got %d", flat, want, len(publicInput.CellExtBits))
	}
	if flat%64 != 0 && publicInput.CellExtBits[len(publicInput.CellExtBits)-1]>>(flat%64) != 0 {
		t.Fatalf("bits set past the last cell")
	}
}

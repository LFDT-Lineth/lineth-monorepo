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
		".refs = &[_]protocol.public_input.CellRef{ .{ .statement_index = 0, .round = 0, .index = 0 } },",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated public-input spec missing %q\n--- got ---\n%s", want, out)
		}
	}
}

package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
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

func TestBuildPublicInputSystemHonestRiscvRebindsOriginalRoundCells(t *testing.T) {
	honest, err := buildHonestRiscvProof()
	if err != nil {
		t.Fatalf("buildHonestRiscvProof() error = %v", err)
	}
	publicInputSystem, err := BuildPublicInputSystem(honest.Sys)
	if err != nil {
		t.Fatalf("BuildPublicInputSystem() error = %v", err)
	}
	projected, err := ps.Project(honest.Sys, honest.Proof, honest.Pub)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}

	bound, err := rebindProjectedRounds(publicInputSystem, projected.Proof.Rounds, projected.PublicInputs)
	if err != nil {
		t.Fatalf("rebindProjectedRounds() error = %v", err)
	}

	if len(bound) != len(honest.Sys.Rounds)-1 {
		t.Fatalf("bound round count = %d, want %d", len(bound), len(honest.Sys.Rounds)-1)
	}
	for roundIndex, round := range honest.Sys.Rounds[:len(honest.Sys.Rounds)-1] {
		if len(bound[roundIndex]) != len(round.Cells) {
			t.Fatalf("bound round %d cell count = %d, want %d", roundIndex, len(bound[roundIndex]), len(round.Cells))
		}
		for cellIndex, cell := range round.Cells {
			want, ok := originalRoundCellValue(honest.Sys, honest.Proof, honest.Pub, cell)
			if !ok {
				t.Fatalf("missing original value for round %d cell %d (%s)", roundIndex, cellIndex, cell.Context.Path())
			}
			if bound[roundIndex][cellIndex] != want {
				t.Fatalf("bound round %d cell %d (%s) = %#v, want %#v",
					roundIndex, cellIndex, cell.Context.Path(), bound[roundIndex][cellIndex], want)
			}
		}
	}
}

func rebindProjectedRounds(spec PublicInputSystem, rounds []ps.RoundMessage, publicInputs []ps.Scalar) ([][]ps.Scalar, error) {
	if len(rounds) != len(spec.RoundCellCounts) {
		return nil, fmt.Errorf("round count = %d, want %d", len(rounds), len(spec.RoundCellCounts))
	}
	if len(publicInputs) != len(spec.Refs) {
		return nil, fmt.Errorf("public input count = %d, want %d", len(publicInputs), len(spec.Refs))
	}

	bound := make([][]ps.Scalar, len(rounds))
	publicInputCursor := 0
	for roundIndex, round := range rounds {
		totalCells := spec.RoundCellCounts[roundIndex]
		bound[roundIndex] = make([]ps.Scalar, totalCells)

		proofCellIndex := 0
		for cellIndex := 0; cellIndex < totalCells; cellIndex++ {
			if publicInputCursor < len(spec.Refs) &&
				spec.Refs[publicInputCursor].Round == roundIndex &&
				spec.Refs[publicInputCursor].Index == cellIndex {
				ref := spec.Refs[publicInputCursor]
				bound[roundIndex][cellIndex] = publicInputs[ref.StatementIndex]
				publicInputCursor++
				continue
			}
			if proofCellIndex >= len(round.Cells) {
				return nil, fmt.Errorf("round %d ran out of proof cells at transcript cell %d", roundIndex, cellIndex)
			}
			bound[roundIndex][cellIndex] = round.Cells[proofCellIndex]
			proofCellIndex++
		}
		if proofCellIndex != len(round.Cells) {
			return nil, fmt.Errorf("round %d left %d unconsumed proof cells", roundIndex, len(round.Cells)-proofCellIndex)
		}
	}

	return bound, nil
}

func originalRoundCellValue(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput, cell *wiop.Cell) (ps.Scalar, bool) {
	for i, publicCell := range sys.PublicInputs {
		if publicCell.Context.ID == cell.Context.ID {
			return ps.ScalarFrom(pub[i]), true
		}
	}
	value, ok := proof.Cells[cell.Context.ID]
	if !ok {
		return ps.Scalar{}, false
	}
	return ps.ScalarFrom(value), true
}

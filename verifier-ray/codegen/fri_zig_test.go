package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

func testFRIParams(t *testing.T) fri.Params {
	t.Helper()
	return newTestParams(t, 32, 8, 4, fri.WithGrinding(0))
}

func testLayout(t *testing.T) Layout {
	t.Helper()
	l, err := BuildLayout(LayoutConfig{
		NumTrees:   1,
		SetupBegin: 0, SetupEnd: 0,
		TraceBegin: []int{0}, TraceEnd: []int{1},
		AirBegin: 1, AirEnd: 1,
		TreeSizes: []int{32},
		ColSlots: map[string]Slot{
			"col0": {TreeIdx: 0, PolyIdx: 0, Rail: field.KindBase},
			"col1": {TreeIdx: 0, PolyIdx: 1, Rail: field.KindExt},
		},
		AirChunkSlots: map[string]Slot{},
	})
	if err != nil {
		t.Fatalf("BuildLayout: %v", err)
	}
	return l
}

func testDQLayout(t *testing.T) DQLayout {
	t.Helper()
	dq, err := BuildDQLayout([]DQLevel{
		{
			Size:      32,
			Shifts:    []int{0, 1},
			ColGroups: [][]ColRef{{{"col0", "key0"}}, {{"col1", "key1"}}},
			AirChunks: []string{},
		},
	})
	if err != nil {
		t.Fatalf("BuildDQLayout: %v", err)
	}
	return dq
}

func TestWriteFRISpecZigEmitsParams(t *testing.T) {
	var out bytes.Buffer
	if err := WriteFRISpecZig(&out, testFRIParams(t), testLayout(t), testDQLayout(t)); err != nil {
		t.Fatalf("WriteFRISpecZig: %v", err)
	}
	zig := out.String()
	for _, want := range []string{
		"pub const params",
		".n = 32",
		".d = 8",
		".num_queries = 4",
		".num_rounds = 3",
		".grinding = 0",
		"domain_gens",
		"domain_gens_inv",
		"field.Element.init(",
	} {
		if !strings.Contains(zig, want) {
			t.Errorf("generated Zig missing %q", want)
		}
	}
}

func TestWriteFRISpecZigEmitsLayout(t *testing.T) {
	var out bytes.Buffer
	if err := WriteFRISpecZig(&out, testFRIParams(t), testLayout(t), testDQLayout(t)); err != nil {
		t.Fatalf("WriteFRISpecZig: %v", err)
	}
	zig := out.String()
	for _, want := range []string{
		"pub const layout",
		".num_trees = 1",
		".tree_size = ",
		"fri_spec.NamedSlot",
		`"col0"`,
		`"col1"`,
		".rail = .base",
		".rail = .ext",
	} {
		if !strings.Contains(zig, want) {
			t.Errorf("generated Zig missing %q", want)
		}
	}
}

func TestWriteFRISpecZigEmitsDQLayout(t *testing.T) {
	var out bytes.Buffer
	if err := WriteFRISpecZig(&out, testFRIParams(t), testLayout(t), testDQLayout(t)); err != nil {
		t.Fatalf("WriteFRISpecZig: %v", err)
	}
	zig := out.String()
	for _, want := range []string{
		"pub const dq_layout",
		"fri_spec.DQLevel",
		".size = 32",
		"fri_spec.ColRef",
		`"key0"`,
		`"key1"`,
	} {
		if !strings.Contains(zig, want) {
			t.Errorf("generated Zig missing %q", want)
		}
	}
}

func TestWriteFRISpecZigNoControlFlow(t *testing.T) {
	var out bytes.Buffer
	if err := WriteFRISpecZig(&out, testFRIParams(t), testLayout(t), testDQLayout(t)); err != nil {
		t.Fatalf("WriteFRISpecZig: %v", err)
	}
	zig := out.String()
	for _, banned := range []string{"if ", "for ", "while ", "fn "} {
		if strings.Contains(zig, banned) {
			t.Errorf("generated Zig contains control flow %q — output must be data only", banned)
		}
	}
}

// Ensure WithGrinding option is used in commitment package import check.
var _ = commitment.DefaultLeafHasher

package codegen

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

func newTestParams(t *testing.T, n, d, numQueries int, opts ...fri.Option) fri.Params {
	t.Helper()
	p, err := fri.NewParams(n, d, numQueries,
		commitment.DefaultLeafHasher, commitment.DefaultNodeHasher, opts...)
	if err != nil {
		t.Fatalf("fri.NewParams: %v", err)
	}
	return p
}

// TestFRIParamsNumRounds verifies that NumRounds = log2(D).
func TestFRIParamsNumRounds(t *testing.T) {
	for _, tc := range []struct {
		n, d, want int
	}{
		{16, 4, 2},
		{32, 8, 3},
		{16, 2, 1},
		{8, 2, 1},
	} {
		got := newTestParams(t, tc.n, tc.d, 4)
		if got.NumRounds != tc.want {
			t.Errorf("N=%d D=%d: NumRounds = %d, want %d", tc.n, tc.d, got.NumRounds, tc.want)
		}
		if len(got.DomainsLight) != tc.want+1 {
			t.Errorf("N=%d D=%d: len(DomainsLight) = %d, want %d", tc.n, tc.d, len(got.DomainsLight), tc.want+1)
		}
	}
}

// TestFRIParamsGenInvAreInverse verifies that each domain generator is invertible
// in the Koalabear field for every fold round.
func TestFRIParamsGenInvAreInverse(t *testing.T) {
	p := newTestParams(t, 32, 8, 4)
	var one koalabear.Element
	one.SetOne()
	for j, domain := range p.DomainsLight {
		var inv, product koalabear.Element
		inv.Inverse(&domain.Generator)
		product.Mul(&domain.Generator, &inv)
		if product != one {
			t.Errorf("round %d: gen * inv != 1 (gen=%d inv=%d product=%v)", j, domain.Generator.Uint64(), inv.Uint64(), product)
		}
	}
}

// TestFRIParamsHalvingProperty verifies that DomainGens[j+1] = DomainGens[j]^2,
// i.e. squaring the round-j generator produces the round-(j+1) generator.
func TestFRIParamsHalvingProperty(t *testing.T) {
	p := newTestParams(t, 32, 8, 4)
	for j := 0; j < p.NumRounds; j++ {
		var squared koalabear.Element
		squared.Square(&p.DomainsLight[j].Generator)
		want := squared.Uint64()
		if got := p.DomainsLight[j+1].Generator.Uint64(); got != want {
			t.Errorf("round %d: DomainGens[%d] = %d, want %d (= DomainGens[%d]^2)", j+1, j+1, got, want, j)
		}
	}
}

// TestFRIParamsGrinding verifies that the grinding bit count is preserved.
func TestFRIParamsGrinding(t *testing.T) {
	const grinding = 8
	p := newTestParams(t, 16, 4, 4, fri.WithGrinding(grinding))
	if p.Grinding != 8 {
		t.Errorf("Grinding = %d, want 8", p.Grinding)
	}
}

// TestBuildLayoutRejectsTreeSizeMismatch checks that BuildLayout rejects a
// TreeSizes slice that does not match NumTrees.
func TestBuildLayoutRejectsTreeSizeMismatch(t *testing.T) {
	_, err := BuildLayout(Layout{
		NumTrees:  3,
		TreeSizes: []int{128, 64}, // length 2 != NumTrees 3
	})
	if err == nil {
		t.Fatal("BuildLayout: expected error for TreeSizes length mismatch, got nil")
	}
}

// TestBuildLayoutRejectsOutOfRangeColSlot checks that BuildLayout rejects a
// column slot whose TreeIdx is >= NumTrees.
func TestBuildLayoutRejectsOutOfRangeColSlot(t *testing.T) {
	_, err := BuildLayout(Layout{
		NumTrees:  2,
		TreeSizes: []int{64, 32},
		ColSlots: map[string]Slot{
			"col": {TreeIdx: 2, PolyIdx: 0, Rail: field.KindBase}, // TreeIdx 2 >= NumTrees 2
		},
	})
	if err == nil {
		t.Fatal("BuildLayout: expected error for out-of-range ColSlot tree_idx, got nil")
	}
}

// TestBuildLayoutRejectsOutOfRangeAirChunkSlot checks that BuildLayout rejects
// an air-chunk slot whose TreeIdx is >= NumTrees.
func TestBuildLayoutRejectsOutOfRangeAirChunkSlot(t *testing.T) {
	_, err := BuildLayout(Layout{
		NumTrees:  2,
		TreeSizes: []int{64, 32},
		AirChunkSlots: map[string]Slot{
			"air": {TreeIdx: 5, PolyIdx: 0, Rail: field.KindExt},
		},
	})
	if err == nil {
		t.Fatal("BuildLayout: expected error for out-of-range AirChunkSlot tree_idx, got nil")
	}
}

// TestBuildLayoutAcceptsValidConfig checks that a well-formed config builds
// without error and round-trips its data.
func TestBuildLayoutAcceptsValidConfig(t *testing.T) {
	cfg := Layout{
		NumTrees:   2,
		SetupBegin: 0, SetupEnd: 1,
		TraceBegin: []int{1}, TraceEnd: []int{2},
		AirBegin: 2, AirEnd: 3,
		TreeSizes: []int{64, 32},
		ColSlots: map[string]Slot{
			"col0": {TreeIdx: 0, PolyIdx: 0, Rail: field.KindBase},
			"col1": {TreeIdx: 1, PolyIdx: 0, Rail: field.KindExt},
		},
		AirChunkSlots: map[string]Slot{
			"air0": {TreeIdx: 0, PolyIdx: 1, Rail: field.KindBase},
		},
	}
	got, err := BuildLayout(cfg)
	if err != nil {
		t.Fatalf("BuildLayout: unexpected error: %v", err)
	}
	if got.NumTrees != 2 {
		t.Errorf("NumTrees = %d, want 2", got.NumTrees)
	}
	if len(got.TreeSizes) != 2 || got.TreeSizes[0] != 64 || got.TreeSizes[1] != 32 {
		t.Errorf("TreeSizes = %v, want [64 32]", got.TreeSizes)
	}
	if slot, ok := got.ColSlots["col1"]; !ok || slot.Rail != field.KindExt {
		t.Errorf("ColSlots[col1] = %+v, want Rail=KindExt", slot)
	}
}

// TestBuildDQLayoutRejectsNonPowerOfTwoSize checks that BuildDQLayout rejects
// a level whose domain size is not a positive power of two.
func TestBuildDQLayoutRejectsNonPowerOfTwoSize(t *testing.T) {
	_, err := BuildDQLayout([]DQLevel{{Size: 6}})
	if err == nil {
		t.Fatal("BuildDQLayout: expected error for size=6, got nil")
	}
}

// TestBuildDQLayoutRejectsShiftsGroupsMismatch checks that BuildDQLayout
// rejects a level where Shifts and ColGroups have different lengths.
func TestBuildDQLayoutRejectsShiftsGroupsMismatch(t *testing.T) {
	_, err := BuildDQLayout([]DQLevel{{
		Size:      16,
		Shifts:    []int{0, 1},
		ColGroups: [][]ColRef{{}}, // length 1 != len(Shifts) 2
	}})
	if err == nil {
		t.Fatal("BuildDQLayout: expected error for Shifts/ColGroups length mismatch, got nil")
	}
}

// TestBuildDQLayoutAcceptsValidLevels checks a well-formed input round-trips.
func TestBuildDQLayoutAcceptsValidLevels(t *testing.T) {
	levels := []DQLevel{
		{
			Size:      16,
			Shifts:    []int{0, 1},
			ColGroups: [][]ColRef{{{"col0", "key0"}}, {{"col1", "key1"}}},
			AirChunks: []string{"air0"},
		},
		{
			Size:      8,
			Shifts:    []int{2},
			ColGroups: [][]ColRef{{{"col2", "key2"}}},
			AirChunks: nil,
		},
	}
	got, err := BuildDQLayout(levels)
	if err != nil {
		t.Fatalf("BuildDQLayout: unexpected error: %v", err)
	}
	if len(got.Levels) != 2 {
		t.Fatalf("len(Levels) = %d, want 2", len(got.Levels))
	}
	if got.Levels[0].Size != 16 || got.Levels[1].Size != 8 {
		t.Errorf("Level sizes = [%d, %d], want [16, 8]", got.Levels[0].Size, got.Levels[1].Size)
	}
	if got.Levels[0].ColGroups[1][0].Name != "col1" {
		t.Errorf("ColGroups[1][0].Name = %q, want %q", got.Levels[0].ColGroups[1][0].Name, "col1")
	}
}

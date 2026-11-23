package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

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

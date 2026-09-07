package predecoding

import (
	"fmt"
	"math"
	"testing"
)

// TestInstructionTypeFromOpcode
// checks the opcode -> instruction-type mapping.
func TestInstructionTypeFromOpcode(t *testing.T) {
	tests := []struct {
		name          string
		opcode        uint32
		type_expected uint32
	}{
		{"op", opcodeOP, rType},
		{"op32", opcodeOP32, rType},
		{"custom1", opcodeCUSTOM1, rType},
		{"load", opcodeLOAD, iType},
		{"opimm", opcodeOPIMM, iType},
		{"opimm32", opcodeOPIMM32, iType},
		{"jalr", opcodeJALR, iType},
		{"system", opcodeSYSTEM, iType},
		{"store", opcodeSTORE, sType},
		{"branch", opcodeBRANCH, bType},
		{"lui", opcodeLUI, uType},
		{"auipc", opcodeAUIPC, uType},
		{"jal", opcodeJAL, jType},
		{"miscmem", opcodeMISCMEM, miscMemType},
		{"unknown zero", 0b0000000, undefinedType},
		{"unknown all-ones", 0b1111111, undefinedType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if type_value := instructionTypeFromOpcode(tt.opcode); type_value != tt.type_expected {
				t.Fatalf("instructionTypeFromOpcode(%#09b) = %d, want %d", tt.opcode, type_value, tt.type_expected)
			}
		})
	}
}

// ------------------------------------------------------------
// IMM12 FIELD EXTRACTORS
// ------------------------------------------------------------
// TestImm12Funct6, TestImm12Funct7, TestImm12Uimm6, TestImm12Uimm5
// checks the imm12 field extractors.
// ------------------------------------------------------------

// extractBits returns bits [lo:hi] of v (inclusive), right-aligned to bit 0.
// It uses math.Pow plus division and modulo, giving a shift-free reference
// independent of the >>/& idiom the imm12 field extractors use. Exponents here
// are tiny (<= 12), well within float64's exact-integer range.
func extractBits(v uint32, lo, hi int) uint32 {
	width := hi - lo + 1
	return (v / uint32(math.Pow(2, float64(lo)))) % uint32(math.Pow(2, float64(width)))
}

// TestImm12Funct6 checks imm12Funct6 extracts bits [11:6] for every imm12 in the
// full 12-bit domain, cross-checked against extractBits.
func TestImm12Funct6(t *testing.T) {
	for imm12 := uint32(0); imm12 < 1<<12; imm12++ {
		want := extractBits(imm12, 6, 11)
		if got := imm12Funct6(imm12); got != want {
			t.Fatalf("imm12Funct6(%#05x) = %#x, want %#x", imm12, got, want)
		}
	}
}

// TestImm12Funct7 checks imm12Funct7 extracts bits [11:5] for every imm12 in the
// full 12-bit domain, cross-checked against extractBits.
func TestImm12Funct7(t *testing.T) {
	for imm12 := uint32(0); imm12 < 1<<12; imm12++ {
		want := extractBits(imm12, 5, 11)
		if got := imm12Funct7(imm12); got != want {
			t.Fatalf("imm12Funct7(%#05x) = %#x, want %#x", imm12, got, want)
		}
	}
}

// TestImm12Uimm6 checks imm12Uimm6 extracts bits [5:0] for every imm12 in the
// full 12-bit domain, cross-checked against extractBits.
func TestImm12Uimm6(t *testing.T) {
	for imm12 := uint32(0); imm12 < 1<<12; imm12++ {
		want := extractBits(imm12, 0, 5)
		if got := imm12Uimm6(imm12); got != want {
			t.Fatalf("imm12Uimm6(%#05x) = %#x, want %#x", imm12, got, want)
		}
	}
}

// TestImm12Uimm5 checks imm12Uimm5 extracts bits [4:0] for every imm12 in the
// full 12-bit domain, cross-checked against extractBits.
func TestImm12Uimm5(t *testing.T) {
	for imm12 := uint32(0); imm12 < 1<<12; imm12++ {
		want := extractBits(imm12, 0, 4)
		if got := imm12Uimm5(imm12); got != want {
			t.Fatalf("imm12Uimm5(%#05x) = %#x, want %#x", imm12, got, want)
		}
	}
}

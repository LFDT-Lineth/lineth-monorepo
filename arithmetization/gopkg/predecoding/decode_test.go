package predecoding

import (
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

// ------------------------------------------------------------
// decodeITypeSemantic: local op
// ------------------------------------------------------------

// iTypeArm is an (opcode, funct3) pair.
type iTypeArm struct {
	opcode uint32
	funct3 uint32
}

// iTypeInput is a full (opcode, funct3, imm12) decode input.
type iTypeInput struct {
	opcode uint32
	funct3 uint32
	imm12  uint32
}

// decodeITypeVectors is the static truth table mapping a decode input to the
// local op (first return value) decodeITypeSemantic must produce. It lists every
// valid arm and, for the imm12-sensitive arms, the imm12 edges that steer
// funct6/funct7 shift validation and SYSTEM funct12 discrimination (including
// the imm12 values that make an otherwise-valid arm reject).
var decodeITypeVectors = map[iTypeInput]uint32{
	// LOAD: op fixed by funct3, imm12 irrelevant (min + max sampled).
	{opcodeLOAD, 0b000, 0x008}: itypeRead8SgnWB,
	{opcodeLOAD, 0b000, 0xfff}: itypeRead8SgnWB,
	{opcodeLOAD, 0b001, 0x004}: itypeRead16SgnWB,
	{opcodeLOAD, 0b010, 0x000}: itypeRead32SgnWB,
	{opcodeLOAD, 0b011, 0x7ff}: itypeRead64WB,
	{opcodeLOAD, 0b100, 0x001}: itypeRead8ZextWB,
	{opcodeLOAD, 0b101, 0x002}: itypeRead16ZextWB,
	{opcodeLOAD, 0b110, 0x003}: itypeRead32ZextWB,

	// OPIMM non-shift: op fixed by funct3, imm12 irrelevant.
	{opcodeOPIMM, 0b000, 0x02a}: itypeOpAddiWB,
	{opcodeOPIMM, 0b000, 0xfff}: itypeOpAddiWB,
	{opcodeOPIMM, 0b010, 0x005}: itypeOpSltiWB,
	{opcodeOPIMM, 0b011, 0x007}: itypeOpSltiuWB,
	{opcodeOPIMM, 0b100, 0x0ff}: itypeOpXoriWB,
	{opcodeOPIMM, 0b110, 0x0f0}: itypeOpOriWB,
	{opcodeOPIMM, 0b111, 0xabc}: itypeOpAndiWB,

	// OPIMM shifts: op depends on funct6 (imm12[11:6]).
	{opcodeOPIMM, 0b001, 0x000}: itypeOpSlliWB, // funct6 0
	{opcodeOPIMM, 0b001, 0x03f}: itypeOpSlliWB, // funct6 0, shamt 63
	{opcodeOPIMM, 0b001, 0x040}: itypeInvalid,  // funct6 != 0
	{opcodeOPIMM, 0b101, 0x003}: itypeOpSrliWB, // funct6 000000
	{opcodeOPIMM, 0b101, 0x03f}: itypeOpSrliWB, // funct6 000000, shamt 63
	{opcodeOPIMM, 0b101, 0x405}: itypeOpSraiWB, // funct6 010000
	{opcodeOPIMM, 0b101, 0x43f}: itypeOpSraiWB, // funct6 010000, shamt 63
	{opcodeOPIMM, 0b101, 0x100}: itypeInvalid,  // funct6 neither 0 nor 010000

	// OPIMM32: word shifts depend on funct7 (imm12[11:5]).
	{opcodeOPIMM32, 0b000, 0x007}: itypeOpAddiwWB,
	{opcodeOPIMM32, 0b001, 0x000}: itypeOpSlliwWB, // funct7 0
	{opcodeOPIMM32, 0b001, 0x01f}: itypeOpSlliwWB, // funct7 0, shamt 31
	{opcodeOPIMM32, 0b001, 0x020}: itypeInvalid,   // funct7 != 0
	{opcodeOPIMM32, 0b101, 0x003}: itypeOpSrliwWB, // funct7 0000000
	{opcodeOPIMM32, 0b101, 0x01f}: itypeOpSrliwWB, // funct7 0000000, shamt 31
	{opcodeOPIMM32, 0b101, 0x405}: itypeOpSraiwWB, // funct7 0100000
	{opcodeOPIMM32, 0b101, 0x41f}: itypeOpSraiwWB, // funct7 0100000, shamt 31
	{opcodeOPIMM32, 0b101, 0x200}: itypeInvalid,   // funct7 neither 0 nor 0100000

	// JALR: op fixed, imm12 irrelevant.
	{opcodeJALR, 0b000, 0x123}: itypeJalr,
	{opcodeJALR, 0b000, 0xfff}: itypeJalr,

	// SYSTEM: op selected by funct12 (== imm12).
	{opcodeSYSTEM, 0b000, funct12Ecall}:  itypeEcall,
	{opcodeSYSTEM, 0b000, funct12Ebreak}: itypeEbreak,
	{opcodeSYSTEM, 0b000, 0x002}:         itypeInvalid, // unknown funct12
}

// validITypeArms is the static set of (opcode, funct3) pairs that can decode to
// a non-invalid op for at least one imm12. Every (opcode, funct3) NOT listed
// here must return itypeInvalid for all imm12 (asserted exhaustively by
// TestDecodeITypeSemanticInvalidArms).
var validITypeArms = map[iTypeArm]bool{
	{opcodeLOAD, 0b000}: true, {opcodeLOAD, 0b001}: true,
	{opcodeLOAD, 0b010}: true, {opcodeLOAD, 0b011}: true,
	{opcodeLOAD, 0b100}: true, {opcodeLOAD, 0b101}: true,
	{opcodeLOAD, 0b110}: true,

	{opcodeOPIMM, 0b000}: true, {opcodeOPIMM, 0b001}: true,
	{opcodeOPIMM, 0b010}: true, {opcodeOPIMM, 0b011}: true,
	{opcodeOPIMM, 0b100}: true, {opcodeOPIMM, 0b101}: true,
	{opcodeOPIMM, 0b110}: true, {opcodeOPIMM, 0b111}: true,

	{opcodeOPIMM32, 0b000}: true, {opcodeOPIMM32, 0b001}: true,
	{opcodeOPIMM32, 0b101}: true,

	{opcodeJALR, 0b000}:   true,
	{opcodeSYSTEM, 0b000}: true,
}

// TestDecodeITypeSemanticOp checks the local op against the decodeITypeVectors
// static truth table.
func TestDecodeITypeSemanticOp(t *testing.T) {
	for in, wantOp := range decodeITypeVectors {
		// Guard: every input in the table must belong to a valid arm.
		if !validITypeArms[iTypeArm{in.opcode, in.funct3}] {
			t.Fatalf("decodeITypeVectors has entry for non-valid arm opcode=%#x funct3=%#03b", in.opcode, in.funct3)
		}
		if gotOp, _ := decodeITypeSemantic(in.opcode, in.funct3, in.imm12); gotOp != wantOp {
			t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#03b, imm=%#05x) op = %d, want %d",
				in.opcode, in.funct3, in.imm12, gotOp, wantOp)
		}
	}
}

// TestDecodeITypeSemanticInvalidArms sweeps every opcode (0..127) and funct3
// (0..7) NOT in validITypeArms and asserts decodeITypeSemantic returns
// itypeInvalid with imm12 unchanged for all imm12 (0..4095).
func TestDecodeITypeSemanticInvalidArms(t *testing.T) {
	for opcode := uint32(0); opcode < 1<<7; opcode++ {
		for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
			if validITypeArms[iTypeArm{opcode, funct3}] {
				continue
			}
			for imm12 := uint32(0); imm12 < 1<<12; imm12++ {
				gotOp, gotImm := decodeITypeSemantic(opcode, funct3, imm12)
				if gotOp != itypeInvalid || gotImm != imm12 {
					t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#03b, imm=%#05x) = (%d, %#x), want (%d, %#x)",
						opcode, funct3, imm12, gotOp, gotImm, itypeInvalid, imm12)
				}
			}
		}
	}
}

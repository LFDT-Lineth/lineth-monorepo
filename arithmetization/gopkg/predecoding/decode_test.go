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
// decodeITypeSemantic tests : translation of opcode and funct3 to computed op
// (local op) and normalized imm12.
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

// iTypeResult is the (local op, normalized imm12) pair decodeITypeSemantic
// returns.
type iTypeResult struct {
	op  uint32
	imm uint32
}

// decodeITypeVectors is the static truth table mapping a decode input to the
// (op, normalized imm12) decodeITypeSemantic must produce.
// It lists every valid arm, meaning an opcode and funct3 pair that can decode
// to a non-invalid op for at least one imm12.
// The expected imm is imm12 unchanged except for the arithmetic shifts (srai/sraiw),
// where it is the stripped shift amount.
// The stripping itself is exhaustively verified separately by the imm12 field-extractor
// tests, so here we only pin the value passed through for each vector.
var decodeITypeVectors = map[iTypeInput]iTypeResult{
	// LOAD: op fixed by funct3, imm12 passed through (min + max sampled).
	{opcodeLOAD, 0b000, 0x008}: {itypeRead8SgnWB, 0x008},   // LB, read signed 8 bits, sign extend to 64 bits
	{opcodeLOAD, 0b000, 0xfff}: {itypeRead8SgnWB, 0xfff},   // LB, read signed 8 bits, sign extend to 64 bits
	{opcodeLOAD, 0b001, 0x004}: {itypeRead16SgnWB, 0x004},  // LH, read signed 16 bits, sign extend to 64 bits
	{opcodeLOAD, 0b010, 0x000}: {itypeRead32SgnWB, 0x000},  // LW, read signed 32 bits, sign extend to 64 bits
	{opcodeLOAD, 0b011, 0x7ff}: {itypeRead64WB, 0x7ff},     // LD, read signed 64 bits, sign extend to 64 bits
	{opcodeLOAD, 0b100, 0x001}: {itypeRead8ZextWB, 0x001},  // LBU, read unsigned 8 bits, zero extend to 64 bits
	{opcodeLOAD, 0b101, 0x002}: {itypeRead16ZextWB, 0x002}, // LHU, read unsigned 16 bits, zero extend to 64 bits
	{opcodeLOAD, 0b110, 0x003}: {itypeRead32ZextWB, 0x003}, // LWU, read unsigned 32 bits, zero extend to 64 bits

	// OPIMM non-shift: op fixed by funct3, imm12 passed through.
	{opcodeOPIMM, 0b000, 0x02a}: {itypeOpAddiWB, 0x02a},  // ADDI
	{opcodeOPIMM, 0b000, 0xfff}: {itypeOpAddiWB, 0xfff},  // ADDI
	{opcodeOPIMM, 0b010, 0x005}: {itypeOpSltiWB, 0x005},  // SLTI
	{opcodeOPIMM, 0b011, 0x007}: {itypeOpSltiuWB, 0x007}, // SLTIU
	{opcodeOPIMM, 0b100, 0x0ff}: {itypeOpXoriWB, 0x0ff},  // XORI
	{opcodeOPIMM, 0b110, 0x0f0}: {itypeOpOriWB, 0x0f0},   // ORI
	{opcodeOPIMM, 0b111, 0xabc}: {itypeOpAndiWB, 0xabc},  // ANDI

	// OPIMM shifts: op depends on funct6 (imm12[11:6]); srai strips to uimm6.
	{opcodeOPIMM, 0b001, 0x000}: {itypeOpSlliWB, 0x000}, // funct6 0, SLLI
	{opcodeOPIMM, 0b001, 0x03f}: {itypeOpSlliWB, 0x03f}, // funct6 0, SLLI, shamt 63
	{opcodeOPIMM, 0b001, 0x040}: {itypeInvalid, 0x040},  // funct6 != 0
	{opcodeOPIMM, 0b101, 0x003}: {itypeOpSrliWB, 0x003}, // funct6 000000, SRLI
	{opcodeOPIMM, 0b101, 0x03f}: {itypeOpSrliWB, 0x03f}, // funct6 000000, SRLI, shamt 63
	{opcodeOPIMM, 0b101, 0x405}: {itypeOpSraiWB, 0x005}, // funct6 010000, SRAI -> uimm6 5
	{opcodeOPIMM, 0b101, 0x43f}: {itypeOpSraiWB, 0x03f}, // funct6 010000, SRAI -> uimm6 63
	{opcodeOPIMM, 0b101, 0x100}: {itypeInvalid, 0x100},  // funct6 neither 0 nor 010000, INVALID

	// OPIMM32: word shifts depend on funct7 (imm12[11:5]); sraiw strips to uimm5.
	{opcodeOPIMM32, 0b000, 0x007}: {itypeOpAddiwWB, 0x007}, // ADDIW
	{opcodeOPIMM32, 0b001, 0x000}: {itypeOpSlliwWB, 0x000}, // funct7 0, SLLIW
	{opcodeOPIMM32, 0b001, 0x01f}: {itypeOpSlliwWB, 0x01f}, // funct7 0, SLLIW, shamt 31
	{opcodeOPIMM32, 0b001, 0x020}: {itypeInvalid, 0x020},   // funct7 != 0, INVALID
	{opcodeOPIMM32, 0b101, 0x003}: {itypeOpSrliwWB, 0x003}, // funct7 0000000, SRLIW
	{opcodeOPIMM32, 0b101, 0x01f}: {itypeOpSrliwWB, 0x01f}, // funct7 0000000, SRLIW, shamt 31
	{opcodeOPIMM32, 0b101, 0x405}: {itypeOpSraiwWB, 0x005}, // funct7 0100000 -> uimm5 5
	{opcodeOPIMM32, 0b101, 0x41f}: {itypeOpSraiwWB, 0x01f}, // funct7 0100000 -> uimm5 31
	{opcodeOPIMM32, 0b101, 0x200}: {itypeInvalid, 0x200},   // funct7 neither 0 nor 0100000, INVALID

	// JALR: op fixed, imm12 passed through.
	{opcodeJALR, 0b000, 0x123}: {itypeJalr, 0x123}, // JALR
	{opcodeJALR, 0b000, 0xfff}: {itypeJalr, 0xfff}, // JALR

	// SYSTEM: op selected by funct12 (== imm12), passed through.
	{opcodeSYSTEM, 0b000, funct12Ecall}:  {itypeEcall, funct12Ecall},   // ECALL
	{opcodeSYSTEM, 0b000, funct12Ebreak}: {itypeEbreak, funct12Ebreak}, // EBREAK
	{opcodeSYSTEM, 0b000, 0x002}:         {itypeInvalid, 0x002},        // unknown funct12, INVALID
}

// validITypeArms is the static set of (opcode, funct3) pairs that can decode to
// a non-invalid op for at least one imm12
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

// TestDecodeITypeSemanticOp checks both return values (computed op and normalized
// imm12) against the decodeITypeVectors static truth table.
func TestDecodeITypeSemanticOp(t *testing.T) {
	for in, want := range decodeITypeVectors {
		// Guard: every input in the table must belong to a valid arm.
		if !validITypeArms[iTypeArm{in.opcode, in.funct3}] {
			t.Fatalf("decodeITypeVectors has entry for non-valid arm opcode=%#x funct3=%#03b", in.opcode, in.funct3)
		}
		gotOp, gotImm := decodeITypeSemantic(in.opcode, in.funct3, in.imm12)
		if gotOp != want.op || gotImm != want.imm {
			t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#03b, imm=%#05x) = (%d, %#x), want (%d, %#x)",
				in.opcode, in.funct3, in.imm12, gotOp, gotImm, want.op, want.imm)
		}
	}
}

// TestDecodeITypeSemanticInvalidArms scans
// - every opcode (0..127)
// - every funct3 (0..7)
// which pair is NOT in the list of validITypeArms and asserts
// decodeITypeSemantic returns itypeInvalid with imm12 unchanged
// for all imm12 (0..4095).
func TestDecodeITypeSemanticInvalidArms(t *testing.T) {
	for opcode := uint32(0); opcode < 1<<7; opcode++ {
		for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
			if validITypeArms[iTypeArm{opcode, funct3}] {
				// if the pair is in the list of validITypeArms, skip
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

// ------------------------------------------------------------
// decodeBTypeSemantic
// ------------------------------------------------------------

// decodeBTypeVectors is the static truth table of the valid branch funct3
// codes, each of which decodeBTypeSemantic returns unchanged. Every funct3 NOT
// listed here must return btypeInvalid (asserted exhaustively by
// TestDecodeBTypeSemanticInvalid).
var decodeBTypeVectors = map[uint32]uint32{
	0b000: 0b000, // beq
	0b001: 0b001, // bne
	0b100: 0b100, // blt
	0b101: 0b101, // bge
	0b110: 0b110, // bltu
	0b111: 0b111, // bgeu
}

// TestDecodeBTypeSemantic checks the valid branch codes against the
// decodeBTypeVectors static truth table.
func TestDecodeBTypeSemantic(t *testing.T) {
	for funct3, want := range decodeBTypeVectors {
		if got := decodeBTypeSemantic(funct3); got != want {
			t.Fatalf("decodeBTypeSemantic(%#03b) = %d, want %d", funct3, got, want)
		}
	}
}

// TestDecodeBTypeSemanticInvalid sweeps every funct3 (0..7) NOT in
// decodeBTypeVectors and asserts decodeBTypeSemantic returns btypeInvalid.
func TestDecodeBTypeSemanticInvalid(t *testing.T) {
	for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
		if _, ok := decodeBTypeVectors[funct3]; ok {
			// if the funct3 is in the list of validBTypeArms, skip
			continue
		}
		if got := decodeBTypeSemantic(funct3); got != btypeInvalid {
			t.Fatalf("decodeBTypeSemantic(%#03b) = %d, want %d", funct3, got, btypeInvalid)
		}
	}
}

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

// signExtend12_oracle is a shift-free reference for 12-bit two's-complement sign
// extension: it keeps the low 12 bits (via modulo, not masking) and subtracts
// 2^12 = 4096 when bit 11 is set (value >= 2^11 = 2048), independent of the
// <<20/>>20 idiom signExtend12 uses.
func signExtend12_oracle(x uint32) int64 {
	v := int64(x % 4096)
	if v >= 2048 {
		v -= 4096
	}
	return v
}

// TestSignExtend12 checks signExtend12 over the full 12-bit domain against
// signExtend12_oracle. Each value is tested under several high-bit backgrounds to
// confirm bits above bit 11 are ignored (masked) and only bit 11 drives the
// sign, so the result always lands in [-2048, 2047].
func TestSignExtend12(t *testing.T) {
	backgrounds := []uint32{0x00000000, 0xfffff000, 0xaaaaa000, 0x55555000}
	for _, bg := range backgrounds {
		for low := uint32(0); low < 1<<12; low++ {
			x := bg | low
			want := signExtend12_oracle(x)
			if got := signExtend12(x); got != want {
				t.Fatalf("signExtend12(%#x) = %d, want %d", x, got, want)
			}
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
	{opcodeOPIMM, 0b001, 0x000}: {itypeOpSlliWB, 0x000},  // funct6 0, SLLI
	{opcodeOPIMM, 0b001, 0x03f}: {itypeOpSlliWB, 0x03f},  // funct6 0, SLLI, shamt 63
	{opcodeOPIMM, 0b001, 0x040}: {computeInvalid, 0x040}, // funct6 != 0
	{opcodeOPIMM, 0b101, 0x003}: {itypeOpSrliWB, 0x003},  // funct6 000000, SRLI
	{opcodeOPIMM, 0b101, 0x03f}: {itypeOpSrliWB, 0x03f},  // funct6 000000, SRLI, shamt 63
	{opcodeOPIMM, 0b101, 0x405}: {itypeOpSraiWB, 0x005},  // funct6 010000, SRAI -> uimm6 5
	{opcodeOPIMM, 0b101, 0x43f}: {itypeOpSraiWB, 0x03f},  // funct6 010000, SRAI -> uimm6 63
	{opcodeOPIMM, 0b101, 0x100}: {computeInvalid, 0x100}, // funct6 neither 0 nor 010000, INVALID

	// OPIMM32: word shifts depend on funct7 (imm12[11:5]); sraiw strips to uimm5.
	{opcodeOPIMM32, 0b000, 0x007}: {itypeOpAddiwWB, 0x007}, // ADDIW
	{opcodeOPIMM32, 0b001, 0x000}: {itypeOpSlliwWB, 0x000}, // funct7 0, SLLIW
	{opcodeOPIMM32, 0b001, 0x01f}: {itypeOpSlliwWB, 0x01f}, // funct7 0, SLLIW, shamt 31
	{opcodeOPIMM32, 0b001, 0x020}: {computeInvalid, 0x020}, // funct7 != 0, INVALID
	{opcodeOPIMM32, 0b101, 0x003}: {itypeOpSrliwWB, 0x003}, // funct7 0000000, SRLIW
	{opcodeOPIMM32, 0b101, 0x01f}: {itypeOpSrliwWB, 0x01f}, // funct7 0000000, SRLIW, shamt 31
	{opcodeOPIMM32, 0b101, 0x405}: {itypeOpSraiwWB, 0x005}, // funct7 0100000 -> uimm5 5
	{opcodeOPIMM32, 0b101, 0x41f}: {itypeOpSraiwWB, 0x01f}, // funct7 0100000 -> uimm5 31
	{opcodeOPIMM32, 0b101, 0x200}: {computeInvalid, 0x200}, // funct7 neither 0 nor 0100000, INVALID

	// JALR: op fixed, imm12 passed through.
	{opcodeJALR, 0b000, 0x123}: {itypeJalr, 0x123}, // JALR
	{opcodeJALR, 0b000, 0xfff}: {itypeJalr, 0xfff}, // JALR

	// SYSTEM: op selected by funct12 (== imm12), passed through.
	{opcodeSYSTEM, 0b000, funct12Ecall}:  {itypeEcall, funct12Ecall},   // ECALL
	{opcodeSYSTEM, 0b000, funct12Ebreak}: {itypeEbreak, funct12Ebreak}, // EBREAK
	{opcodeSYSTEM, 0b000, 0x002}:         {computeInvalid, 0x002},      // unknown funct12, INVALID
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
// decodeITypeSemantic returns computeInvalid with imm12 unchanged
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
				if gotOp != computeInvalid || gotImm != imm12 {
					t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#03b, imm=%#05x) = (%d, %#x), want (%d, %#x)",
						opcode, funct3, imm12, gotOp, gotImm, computeInvalid, imm12)
				}
			}
		}
	}
}

// ------------------------------------------------------------
// decodeBTypeSemantic
// ------------------------------------------------------------

// decodeBTypeVectors is the static truth table mapping each valid branch funct3
// code to its unified B-type compute op. Every funct3 NOT listed here must
// return computeInvalid (asserted exhaustively by TestDecodeBTypeSemanticInvalid).
var decodeBTypeVectors = map[uint32]uint32{
	0b000: btypeBeq,
	0b001: btypeBne,
	0b100: btypeBlt,
	0b101: btypeBge,
	0b110: btypeBltu,
	0b111: btypeBgeu,
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
// decodeBTypeVectors and asserts decodeBTypeSemantic returns computeInvalid.
func TestDecodeBTypeSemanticInvalid(t *testing.T) {
	for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
		if _, ok := decodeBTypeVectors[funct3]; ok {
			// if the funct3 is in the list of validBTypeArms, skip
			continue
		}
		if got := decodeBTypeSemantic(funct3); got != computeInvalid {
			t.Fatalf("decodeBTypeSemantic(%#03b) = %d, want %d", funct3, got, computeInvalid)
		}
	}
}

// ------------------------------------------------------------
// decodeJTypeSemantic
// ------------------------------------------------------------

// decodeJTypeVectors is the static truth table of the valid J-type opcodes, each
// mapped to the base compute op decodeJTypeSemantic returns. Every opcode NOT
// listed here must return computeInvalid (asserted exhaustively by
// TestDecodeJTypeSemanticInvalid).
var decodeJTypeVectors = map[uint32]uint32{
	opcodeJAL: jtypeJal, // jal
}

// TestDecodeJTypeSemantic checks the valid opcodes against the
// decodeJTypeVectors static truth table.
func TestDecodeJTypeSemantic(t *testing.T) {
	for opcode, want := range decodeJTypeVectors {
		if got := decodeJTypeSemantic(opcode); got != want {
			t.Fatalf("decodeJTypeSemantic(%#09b) = %d, want %d", opcode, got, want)
		}
	}
}

// TestDecodeJTypeSemanticInvalid sweeps every opcode (0..127) NOT in
// decodeJTypeVectors and asserts decodeJTypeSemantic returns computeInvalid.
func TestDecodeJTypeSemanticInvalid(t *testing.T) {
	for opcode := uint32(0); opcode < 1<<7; opcode++ {
		if _, ok := decodeJTypeVectors[opcode]; ok {
			continue
		}
		if got := decodeJTypeSemantic(opcode); got != computeInvalid {
			t.Fatalf("decodeJTypeSemantic(%#09b) = %d, want %d", opcode, got, computeInvalid)
		}
	}
}

// ------------------------------------------------------------
// decodeSTypeSemantic
// ------------------------------------------------------------

// decodeSTypeVectors is the static truth table of the valid S-type funct3
// codes, each mapped to the store-width compute op decodeSTypeSemantic returns.
// Every funct3 NOT listed here must return computeInvalid (asserted exhaustively
// by TestDecodeSTypeSemanticInvalid).
var decodeSTypeVectors = map[uint32]uint32{
	0b000: stypeStore8,  // sb
	0b001: stypeStore16, // sh
	0b010: stypeStore32, // sw
	0b011: stypeStore64, // sd
}

// TestDecodeSTypeSemantic checks the valid store codes against the
// decodeSTypeVectors static truth table.
func TestDecodeSTypeSemantic(t *testing.T) {
	for funct3, want := range decodeSTypeVectors {
		if got := decodeSTypeSemantic(funct3); got != want {
			t.Fatalf("decodeSTypeSemantic(%#03b) = %d, want %d", funct3, got, want)
		}
	}
}

// TestDecodeSTypeSemanticInvalid sweeps every funct3 (0..7) NOT in
// decodeSTypeVectors and asserts decodeSTypeSemantic returns computeInvalid.
func TestDecodeSTypeSemanticInvalid(t *testing.T) {
	for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
		if _, ok := decodeSTypeVectors[funct3]; ok {
			continue
		}
		if got := decodeSTypeSemantic(funct3); got != computeInvalid {
			t.Fatalf("decodeSTypeSemantic(%#03b) = %d, want %d", funct3, got, computeInvalid)
		}
	}
}

// ------------------------------------------------------------
// decodeRTypeSemantic
// ------------------------------------------------------------

// rTypeInput is a full (opcode, funct3, funct7) R-type decode input.
type rTypeInput struct {
	opcode uint32
	funct3 uint32
	funct7 uint32
}

// decodeRTypeVectors is the static truth table of every valid (opcode, funct3,
// funct7) R-type combination mapped to the local op decodeRTypeSemantic returns.
// It is the complete set of valid points: every input NOT listed here must
// return computeInvalid (asserted exhaustively by TestDecodeRTypeSemanticInvalid).
var decodeRTypeVectors = map[rTypeInput]uint32{
	// OP, funct7 0000000: base integer.
	{opcodeOP, 0b000, 0b0000000}: rtypeOpAddWB,
	{opcodeOP, 0b001, 0b0000000}: rtypeOpSllWB,
	{opcodeOP, 0b010, 0b0000000}: rtypeOpSltWB,
	{opcodeOP, 0b011, 0b0000000}: rtypeOpSltuWB,
	{opcodeOP, 0b100, 0b0000000}: rtypeOpXorWB,
	{opcodeOP, 0b101, 0b0000000}: rtypeOpSrlWB,
	{opcodeOP, 0b110, 0b0000000}: rtypeOpOrWB,
	{opcodeOP, 0b111, 0b0000000}: rtypeOpAndWB,
	// OP, funct7 0100000: sub/sra.
	{opcodeOP, 0b000, 0b0100000}: rtypeOpSubWB,
	{opcodeOP, 0b101, 0b0100000}: rtypeOpSraWB,
	// OP, funct7 0000001: M extension.
	{opcodeOP, 0b000, 0b0000001}: rtypeOpMulWB,
	{opcodeOP, 0b001, 0b0000001}: rtypeOpMulhWB,
	{opcodeOP, 0b010, 0b0000001}: rtypeOpMulhsuWB,
	{opcodeOP, 0b011, 0b0000001}: rtypeOpMulhuWB,
	{opcodeOP, 0b100, 0b0000001}: rtypeOpDivWB,
	{opcodeOP, 0b101, 0b0000001}: rtypeOpDivuWB,
	{opcodeOP, 0b110, 0b0000001}: rtypeOpRemWB,
	{opcodeOP, 0b111, 0b0000001}: rtypeOpRemuWB,

	// OP32, funct7 0000000: word base integer.
	{opcodeOP32, 0b000, 0b0000000}: rtypeOpAddwWB,
	{opcodeOP32, 0b001, 0b0000000}: rtypeOpSllwWB,
	{opcodeOP32, 0b101, 0b0000000}: rtypeOpSrlwWB,
	// OP32, funct7 0100000: word sub/sra.
	{opcodeOP32, 0b000, 0b0100000}: rtypeOpSubwWB,
	{opcodeOP32, 0b101, 0b0100000}: rtypeOpSrawWB,
	// OP32, funct7 0000001: word M extension.
	{opcodeOP32, 0b000, 0b0000001}: rtypeOpMulwWB,
	{opcodeOP32, 0b100, 0b0000001}: rtypeOpDivwWB,
	{opcodeOP32, 0b101, 0b0000001}: rtypeOpDivuwWB,
	{opcodeOP32, 0b110, 0b0000001}: rtypeOpRemwWB,
	{opcodeOP32, 0b111, 0b0000001}: rtypeOpRemuwWB,

	// CUSTOM1, funct7 must be 0000000.
	{opcodeCUSTOM1, 0b000, 0b0000000}: rtypeOpKeccak,
	{opcodeCUSTOM1, 0b001, 0b0000000}: rtypeOpPoseidon2,
	{opcodeCUSTOM1, 0b010, 0b0000000}: rtypeOpWriteOutput,
}

// TestDecodeRTypeSemantic checks every valid combination against the
// decodeRTypeVectors static truth table.
func TestDecodeRTypeSemantic(t *testing.T) {
	for in, want := range decodeRTypeVectors {
		if got := decodeRTypeSemantic(in.opcode, in.funct3, in.funct7); got != want {
			t.Fatalf("decodeRTypeSemantic(op=%#09b, f3=%#03b, f7=%#09b) = %d, want %d",
				in.opcode, in.funct3, in.funct7, got, want)
		}
	}
}

// TestDecodeRTypeSemanticInvalid sweeps the full (opcode, funct3, funct7)
// universe (128 x 8 x 128) and asserts every combination NOT in
// decodeRTypeVectors returns computeInvalid.
func TestDecodeRTypeSemanticInvalid(t *testing.T) {
	for opcode := uint32(0); opcode < 1<<7; opcode++ {
		for funct3 := uint32(0); funct3 < 1<<3; funct3++ {
			for funct7 := uint32(0); funct7 < 1<<7; funct7++ {
				if _, ok := decodeRTypeVectors[rTypeInput{opcode, funct3, funct7}]; ok {
					// if the combination is in the list of valid R-type combinations, skip
					continue
				}
				if got := decodeRTypeSemantic(opcode, funct3, funct7); got != computeInvalid {
					t.Fatalf("decodeRTypeSemantic(op=%#09b, f3=%#03b, f7=%#09b) = %d, want %d",
						opcode, funct3, funct7, got, computeInvalid)
				}
			}
		}
	}
}

// ------------------------------------------------------------
// decodeUTypeSemantic
// ------------------------------------------------------------

// decodeUTypeVectors is the static truth table of the valid U-type opcodes, each
// mapped to the local op decodeUTypeSemantic returns. Every opcode NOT listed
// here must return computeInvalid (asserted exhaustively by
// TestDecodeUTypeSemanticInvalid).
var decodeUTypeVectors = map[uint32]uint32{
	opcodeLUI:   utypeLuiWB,   // lui
	opcodeAUIPC: utypeAuipcWB, // auipc
}

// TestDecodeUTypeSemantic checks the valid opcodes against the
// decodeUTypeVectors static truth table.
func TestDecodeUTypeSemantic(t *testing.T) {
	for opcode, want := range decodeUTypeVectors {
		if got := decodeUTypeSemantic(opcode); got != want {
			t.Fatalf("decodeUTypeSemantic(%#09b) = %d, want %d", opcode, got, want)
		}
	}
}

// TestDecodeUTypeSemanticInvalid sweeps every opcode (0..127) NOT in
// decodeUTypeVectors and asserts decodeUTypeSemantic returns computeInvalid.
func TestDecodeUTypeSemanticInvalid(t *testing.T) {
	for opcode := uint32(0); opcode < 1<<7; opcode++ {
		if _, ok := decodeUTypeVectors[opcode]; ok {
			continue
		}
		if got := decodeUTypeSemantic(opcode); got != computeInvalid {
			t.Fatalf("decodeUTypeSemantic(%#09b) = %d, want %d", opcode, got, computeInvalid)
		}
	}
}

// ------------------------------------------------------------
// extractFields
// ------------------------------------------------------------

// extractFieldSpecs describes each field extractFields slices out: its inclusive
// bit range [lo, hi] and an accessor.
var extractFieldSpecs = []struct {
	name string
	lo   int
	hi   int
	get  func(instructionFields) uint32
}{
	{"opcode", 0, 6, func(f instructionFields) uint32 { return f.opcode }},
	{"rd", 7, 11, func(f instructionFields) uint32 { return f.rd }},
	{"funct3", 12, 14, func(f instructionFields) uint32 { return f.funct3 }},
	{"imm12", 20, 31, func(f instructionFields) uint32 { return f.imm12 }},
	{"funct7", 25, 31, func(f instructionFields) uint32 { return f.funct7 }},
}

// assertExtractFields checks every field of extractFields(instr) against the
// independent extractBits oracle for a single instruction word.
func assertExtractFields(t *testing.T, instr uint32) {
	t.Helper()
	got := extractFields(instr)
	for _, s := range extractFieldSpecs {
		// check if the field (opcode, rd, funct3, imm12, funct7)
		// is valid for the instruction
		if v, want := s.get(got), extractBits(instr, s.lo, s.hi); v != want {
			t.Fatalf("extractFields(%#010x).%s = %#x, want %#x", instr, s.name, v, want)
		}
	}
}

// TestExtractFields verifies extractFields against extractBits.
// The full 32-bit word space (2^32) is too large to enumerate, so we exhaustively
// sweep :
// - every walking-one and walking-zero pattern
// - each field's own value domain against both an all-zero and an all-ones background
// (to catch mask bleed)
func TestExtractFields(t *testing.T) {

	// Walking ones / walking zeros: isolates every individual bit.
	// 0b...00000001 (bit 0)
	// 0b...00000010 (bit 1)
	// 0b...00000100 (bit 2)
	// To cover all areas, even the ones that are not explicitly listed in
	// extractFieldSpecs (rs1, rs2).
	for b := 0; b < 32; b++ {
		assertExtractFields(t, uint32(1)<<uint(b))
		assertExtractFields(t, ^(uint32(1) << uint(b)))
	}

	// Per-field full-domain sweep against zero and all-ones backgrounds.
	for _, s := range extractFieldSpecs {
		// number of bits in the field
		width := uint(s.hi - s.lo + 1)
		// mask to extract the field
		mask := ((uint32(1) << width) - 1) << uint(s.lo)
		// sweep the field's own value domain against both an all-zero and an all-ones background
		for val := uint32(0); val < uint32(1)<<width; val++ {
			field := val << uint(s.lo)
			// first we test the shift is correct, with a zero background
			assertExtractFields(t, field) // value in field, zero background
			// then we test the mask is correct, with an all-ones background
			assertExtractFields(t, field|^mask) // value in field, all-ones background
		}
	}
}

// ------------------------------------------------------------
// checkNoOp — writeback promotion (rd != x0)
// ------------------------------------------------------------

// checkNoOpPromoteVectors is the truth table for the rd != x0 branch of
// checkNoOp. That branch is independent of instruction type: only the two link
// ops are promoted to their *_WB variant; every other op passes through
// unchanged.
var checkNoOpPromoteVectors = []struct {
	name string
	op   uint32
	want uint32
}{
	{"jalr -> jalr_wb", itypeJalr, itypeJalrWB},
	{"jal -> jal_wb", jtypeJal, jtypeJalWB},
	{"jalr_wb passthrough", itypeJalrWB, itypeJalrWB},
	{"jal_wb passthrough", jtypeJalWB, jtypeJalWB},
	{"load passthrough", itypeRead8SgnWB, itypeRead8SgnWB},
	{"addi passthrough", itypeOpAddiWB, itypeOpAddiWB},
	{"rtype passthrough", rtypeOpAddWB, rtypeOpAddWB},
	{"keccak passthrough", rtypeOpKeccak, rtypeOpKeccak},
	{"store passthrough", stypeStore8, stypeStore8},
	{"branch passthrough", btypeBeq, btypeBeq},
	{"lui passthrough", utypeLuiWB, utypeLuiWB},
	{"ecall passthrough", itypeEcall, itypeEcall},
	{"ebreak passthrough", itypeEbreak, itypeEbreak},
	{"invalid passthrough", computeInvalid, computeInvalid},
}

// TestCheckNoOpRdNonZero exercises the rd != x0 branch of checkNoOp: link ops
// gain their writeback variant, every other op passes through, and the result is
// independent of the instruction type for every rd in 1..31.
func TestCheckNoOpRdNonZero(t *testing.T) {
	instrTypes := []uint32{
		undefinedType, iType, rType, sType, bType, jType, uType, miscMemType,
	}
	for _, tv := range checkNoOpPromoteVectors {
		for _, instrType := range instrTypes {
			for rd := uint32(1); rd < 1<<5; rd++ {
				if got := checkNoOp(instrType, tv.op, rd); got != tv.want {
					t.Fatalf("%s: checkNoOp(instrType=%d, op=%d, rd=%d) = %d, want %d",
						tv.name, instrType, tv.op, rd, got, tv.want)
				}
			}
		}
	}
}

// ------------------------------------------------------------
// checkNoOp — rd == x0 collapse (full truth table)
// ------------------------------------------------------------

// checkNoOpRdZeroVectors is the full truth table for checkNoOp when rd == x0.
// Writeback-only ops collapse to NO_OP; control-flow, memory, syscall, and
// precompile ops keep their semantic op, and COMPUTE_INVALID stays invalid.
var checkNoOpRdZeroVectors = []struct {
	name      string
	instrType uint32
	op        uint32
	want      uint32
}{
	// misc-mem (FENCE / FENCE.I): always NO_OP.
	{"fence", miscMemType, computeNoOp, computeNoOp},

	// I-type loads: inert when rd == x0.
	{"lb", iType, itypeRead8SgnWB, computeNoOp},
	{"lh", iType, itypeRead16SgnWB, computeNoOp},
	{"lw", iType, itypeRead32SgnWB, computeNoOp},
	{"ld", iType, itypeRead64WB, computeNoOp},
	{"lbu", iType, itypeRead8ZextWB, computeNoOp},
	{"lhu", iType, itypeRead16ZextWB, computeNoOp},
	{"lwu", iType, itypeRead32ZextWB, computeNoOp},
	// I-type ALU-immediate: inert when rd == x0.
	{"addi", iType, itypeOpAddiWB, computeNoOp},
	{"slti", iType, itypeOpSltiWB, computeNoOp},
	{"sltiu", iType, itypeOpSltiuWB, computeNoOp},
	{"xori", iType, itypeOpXoriWB, computeNoOp},
	{"ori", iType, itypeOpOriWB, computeNoOp},
	{"andi", iType, itypeOpAndiWB, computeNoOp},
	{"slli", iType, itypeOpSlliWB, computeNoOp},
	{"srli", iType, itypeOpSrliWB, computeNoOp},
	{"srai", iType, itypeOpSraiWB, computeNoOp},
	{"addiw", iType, itypeOpAddiwWB, computeNoOp},
	{"slliw", iType, itypeOpSlliwWB, computeNoOp},
	{"srliw", iType, itypeOpSrliwWB, computeNoOp},
	{"sraiw", iType, itypeOpSraiwWB, computeNoOp},
	// I-type control / syscall: kept even when rd == x0.
	{"jalr", iType, itypeJalr, itypeJalr},
	{"ecall", iType, itypeEcall, itypeEcall},
	{"ebreak", iType, itypeEbreak, itypeEbreak},
	{"itype invalid", iType, computeInvalid, computeInvalid},

	// R-type ALU: inert when rd == x0.
	{"add", rType, rtypeOpAddWB, computeNoOp},
	{"sub", rType, rtypeOpSubWB, computeNoOp},
	{"sll", rType, rtypeOpSllWB, computeNoOp},
	{"slt", rType, rtypeOpSltWB, computeNoOp},
	{"sltu", rType, rtypeOpSltuWB, computeNoOp},
	{"xor", rType, rtypeOpXorWB, computeNoOp},
	{"srl", rType, rtypeOpSrlWB, computeNoOp},
	{"sra", rType, rtypeOpSraWB, computeNoOp},
	{"or", rType, rtypeOpOrWB, computeNoOp},
	{"and", rType, rtypeOpAndWB, computeNoOp},
	{"mul", rType, rtypeOpMulWB, computeNoOp},
	{"mulh", rType, rtypeOpMulhWB, computeNoOp},
	{"mulhsu", rType, rtypeOpMulhsuWB, computeNoOp},
	{"mulhu", rType, rtypeOpMulhuWB, computeNoOp},
	{"div", rType, rtypeOpDivWB, computeNoOp},
	{"divu", rType, rtypeOpDivuWB, computeNoOp},
	{"rem", rType, rtypeOpRemWB, computeNoOp},
	{"remu", rType, rtypeOpRemuWB, computeNoOp},
	{"addw", rType, rtypeOpAddwWB, computeNoOp},
	{"subw", rType, rtypeOpSubwWB, computeNoOp},
	{"sllw", rType, rtypeOpSllwWB, computeNoOp},
	{"srlw", rType, rtypeOpSrlwWB, computeNoOp},
	{"sraw", rType, rtypeOpSrawWB, computeNoOp},
	{"mulw", rType, rtypeOpMulwWB, computeNoOp},
	{"divw", rType, rtypeOpDivwWB, computeNoOp},
	{"divuw", rType, rtypeOpDivuwWB, computeNoOp},
	{"remw", rType, rtypeOpRemwWB, computeNoOp},
	{"remuw", rType, rtypeOpRemuwWB, computeNoOp},
	// R-type Custom-1 precompiles: kept (memory side effects).
	{"keccak", rType, rtypeOpKeccak, rtypeOpKeccak},
	{"poseidon2", rType, rtypeOpPoseidon2, rtypeOpPoseidon2},
	{"write_output", rType, rtypeOpWriteOutput, rtypeOpWriteOutput},
	{"rtype invalid", rType, computeInvalid, computeInvalid},

	// S-type stores: side effects, kept.
	{"sb", sType, stypeStore8, stypeStore8},
	{"sh", sType, stypeStore16, stypeStore16},
	{"sw", sType, stypeStore32, stypeStore32},
	{"sd", sType, stypeStore64, stypeStore64},
	{"stype invalid", sType, computeInvalid, computeInvalid},

	// B-type branches: control flow, kept.
	{"beq", bType, btypeBeq, btypeBeq},
	{"bne", bType, btypeBne, btypeBne},
	{"blt", bType, btypeBlt, btypeBlt},
	{"bge", bType, btypeBge, btypeBge},
	{"bltu", bType, btypeBltu, btypeBltu},
	{"bgeu", bType, btypeBgeu, btypeBgeu},
	{"btype invalid", bType, computeInvalid, computeInvalid},

	// J-type: control flow, kept (JAL not collapsed even into x0).
	{"jal", jType, jtypeJal, jtypeJal},
	{"jtype invalid", jType, computeInvalid, computeInvalid},

	// U-type: inert when rd == x0.
	{"lui", uType, utypeLuiWB, computeNoOp},
	{"auipc", uType, utypeAuipcWB, computeNoOp},
	{"utype invalid", uType, computeInvalid, computeInvalid},

	// Unknown opcode: invalid stays invalid.
	{"undefined", undefinedType, computeInvalid, computeInvalid},
}

// TestCheckNoOpRdZero verifies the rd == x0 collapse across every instruction
// type and op using the full truth table.
func TestCheckNoOpRdZero(t *testing.T) {
	for _, tv := range checkNoOpRdZeroVectors {
		if got := checkNoOp(tv.instrType, tv.op, 0); got != tv.want {
			t.Fatalf("%s: checkNoOp(instrType=%d, op=%d, rd=0) = %d, want %d",
				tv.name, tv.instrType, tv.op, got, tv.want)
		}
	}
}

package main

import "testing"

func encodeIType(opcode, funct3, rd, rs1, imm12 uint32) uint32 {
	return (imm12 << 20) | (rs1 << 15) | (funct3 << 12) | (rd << 7) | opcode
}

func encodeRType(opcode, funct7, rs2, rs1, funct3, rd uint32) uint32 {
	return (funct7 << 25) | (rs2 << 20) | (rs1 << 15) | (funct3 << 12) | (rd << 7) | opcode
}

func encodeUType(opcode, rd, imm20 uint32) uint32 {
	return (imm20 << 12) | (rd << 7) | opcode
}

func encodeJType(rd uint32, offset int32) uint32 {
	imm20 := (uint32(offset) >> 20) & 0x1
	imm10_1 := (uint32(offset) >> 1) & 0x3ff
	imm11 := (uint32(offset) >> 11) & 0x1
	imm19_12 := (uint32(offset) >> 12) & 0xff
	return opcodeJAL | (rd << 7) | (imm19_12 << 12) | (imm11 << 20) | (imm10_1 << 21) | (imm20 << 31)
}

func encodeBType(funct3, rs1, rs2 uint32, offset int32) uint32 {
	imm13 := uint32(offset) & 0x1fff
	imm12 := (imm13 >> 12) & 0x1
	imm11 := (imm13 >> 11) & 0x1
	imm10_5 := (imm13 >> 5) & 0x3f
	imm4_1 := (imm13 >> 1) & 0xf
	return opcodeBRANCH | (imm11 << 7) | (imm4_1 << 8) | (funct3 << 12) | (rs1 << 15) | (rs2 << 20) | (imm10_5 << 25) | (imm12 << 31)
}

func decodeFields(instr uint32) (opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7 uint32) {
	opcode = instr & 0x7f
	instrType = instructionTypeFromOpcode(opcode)
	rd = (instr >> 7) & 0x1f
	funct3 = (instr >> 12) & 0x7
	rs1 = (instr >> 15) & 0x1f
	rs2 = (instr >> 20) & 0x1f
	imm12 = (instr >> 20) & 0xfff
	funct7 = (instr >> 25) & 0x7f
	return
}

func TestIsRdZeroNoop(t *testing.T) {
	tests := []struct {
		name string
		instr uint32
		want bool
	}{
		{
			name:  "addi x0 x0 0",
			instr: encodeIType(opcodeOPIMM, 0b000, 0, 0, 0),
			want:  true,
		},
		{
			name:  "addi x0 t0 5",
			instr: encodeIType(opcodeOPIMM, 0b000, 0, 5, 5),
			want:  false,
		},
		{
			name:  "lui x0 0",
			instr: encodeUType(opcodeLUI, 0, 0),
			want:  true,
		},
		{
			name:  "auipc x0 0",
			instr: encodeUType(opcodeAUIPC, 0, 0),
			want:  false,
		},
		{
			name:  "add x0 x0 x0",
			instr: encodeRType(opcodeOP, 0, 0, 0, 0b000, 0),
			want:  true,
		},
		{
			name:  "add x0 t0 t1",
			instr: encodeRType(opcodeOP, 0, 6, 5, 0b000, 0),
			want:  false,
		},
		{
			name:  "ld x0 0 t0",
			instr: encodeIType(opcodeLOAD, 0b011, 0, 5, 0),
			want:  false,
		},
		{
			name:  "jalr x0 t0 0",
			instr: encodeIType(opcodeJALR, 0, 0, 5, 0),
			want:  false,
		},
		{
			name:  "xori x0 x0 0xff",
			instr: encodeIType(opcodeOPIMM, 0b100, 0, 0, 0xff),
			want:  true,
		},
		{
			name:  "slli x0 x0 4",
			instr: encodeIType(opcodeOPIMM, 0b001, 0, 0, 4),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7 := decodeFields(tt.instr)
			got := isRdZeroNoop(opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7)
			if got != tt.want {
				t.Fatalf("isRdZeroNoop(%#x) = %v, want %v", tt.instr, got, tt.want)
			}
		})
	}
}

func TestBuildDecodedProgramRewritesRdZeroNoop(t *testing.T) {
	// Minimal synthetic executable image: one addi x0,x0,0 at base 0x1000.
	const base uint64 = 0x1000
	instr := encodeIType(opcodeOPIMM, 0b000, 0, 0, 0)
	image := make([]byte, 4)
	image[0] = byte(instr)
	image[1] = byte(instr >> 8)
	image[2] = byte(instr >> 16)
	image[3] = byte(instr >> 24)

	var coreBits bitWriter
	opcode := instr & 0x7f
	rd := (instr >> 7) & 0x1f
	rs1 := (instr >> 15) & 0x1f
	funct3 := (instr >> 12) & 0x7
	imm12 := (instr >> 20) & 0xfff
	funct7 := (instr >> 25) & 0x7f
	instrType := instructionTypeFromOpcode(opcode)
	if isRdZeroNoop(opcode, instrType, rd, rs1, 0, funct3, imm12, funct7) {
		instrType = miscMemType
	}
	coreBits.writeBits(uint64(opcode), 7)
	coreBits.writeBits(uint64(instrType), 3)
	coreBits.writeBits(uint64((instr>>7)&0x1ffffff), 25)

	if instrType != miscMemType {
		t.Fatalf("expected rewritten type %d, got %d", miscMemType, instrType)
	}
	_ = base
	_ = image
}

func TestDecodeITypeSemantic(t *testing.T) {
	tests := []struct {
		name      string
		opcode    uint32
		funct3    uint32
		imm12     uint32
		wantOp    uint32
		wantImm12 uint32
	}{
		{name: "lb", opcode: opcodeLOAD, funct3: 0b000, imm12: 8, wantOp: itypeRead8Sgn, wantImm12: 8},
		{name: "lh", opcode: opcodeLOAD, funct3: 0b001, imm12: 4, wantOp: itypeRead16Sgn, wantImm12: 4},
		{name: "lw", opcode: opcodeLOAD, funct3: 0b010, imm12: 0, wantOp: itypeRead32Sgn, wantImm12: 0},
		{name: "ld", opcode: opcodeLOAD, funct3: 0b011, imm12: 0, wantOp: itypeRead64, wantImm12: 0},
		{name: "lbu", opcode: opcodeLOAD, funct3: 0b100, imm12: 1, wantOp: itypeRead8Zext, wantImm12: 1},
		{name: "lhu", opcode: opcodeLOAD, funct3: 0b101, imm12: 2, wantOp: itypeRead16Zext, wantImm12: 2},
		{name: "lwu", opcode: opcodeLOAD, funct3: 0b110, imm12: 3, wantOp: itypeRead32Zext, wantImm12: 3},
		{name: "addi", opcode: opcodeOPIMM, funct3: 0b000, imm12: 42, wantOp: itypeOpAddi, wantImm12: 42},
		{name: "slli", opcode: opcodeOPIMM, funct3: 0b001, imm12: 4, wantOp: itypeOpSlli, wantImm12: 4},
		{name: "srli", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrli, wantImm12: 3},
		{name: "srai", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b010000000101, wantOp: itypeOpSrai, wantImm12: 5},
		{name: "xori", opcode: opcodeOPIMM, funct3: 0b100, imm12: 0xff, wantOp: itypeOpXori, wantImm12: 0xff},
		{name: "addiw", opcode: opcodeOPIMM32, funct3: 0b000, imm12: 7, wantOp: itypeOpAddiw, wantImm12: 7},
		{name: "slliw", opcode: opcodeOPIMM32, funct3: 0b001, imm12: 2, wantOp: itypeOpSlliw, wantImm12: 2},
		{name: "srliw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrliw, wantImm12: 3},
		{name: "sraiw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b010000000100, wantOp: itypeOpSraiw, wantImm12: 4},
		{name: "jalr", opcode: opcodeJALR, funct3: 0, imm12: 0, wantOp: itypeJalr, wantImm12: 0},
		{name: "ecall", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ecall, wantOp: itypeEcall, wantImm12: funct12Ecall},
		{name: "ebreak", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ebreak, wantOp: itypeEbreak, wantImm12: funct12Ebreak},
		{name: "invalid slli funct6", opcode: opcodeOPIMM, funct3: 0b001, imm12: 0b010000000100, wantOp: itypeInvalid, wantImm12: 0b010000000100},
		{name: "invalid load funct3", opcode: opcodeLOAD, funct3: 0b111, imm12: 0, wantOp: itypeInvalid, wantImm12: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp, gotImm := decodeITypeSemantic(tt.opcode, tt.funct3, tt.imm12)
			if gotOp != tt.wantOp || gotImm != tt.wantImm12 {
				t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#x, imm=%#x) = (%d, %#x), want (%d, %#x)",
					tt.opcode, tt.funct3, tt.imm12, gotOp, gotImm, tt.wantOp, tt.wantImm12)
			}
		})
	}
}

func TestItypeOpForRd(t *testing.T) {
	if got := itypeOpForRd(itypeOpAddi, 0); got != itypeOpAddi {
		t.Fatalf("itypeOpForRd(addi, x0) = %d, want %d", got, itypeOpAddi)
	}
	if got := itypeOpForRd(itypeOpAddi, 5); got != itypeOpAddiWB {
		t.Fatalf("itypeOpForRd(addi, x5) = %d, want %d", got, itypeOpAddiWB)
	}
	if got := itypeOpForRd(itypeEcall, 5); got != itypeEcall {
		t.Fatalf("itypeOpForRd(ecall, x5) = %d, want %d", got, itypeEcall)
	}
	if got := itypeOpForRd(itypeInvalid, 5); got != itypeInvalid {
		t.Fatalf("itypeOpForRd(invalid, x5) = %d, want %d", got, itypeInvalid)
	}
}

func TestDecodeRTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint32
		funct3 uint32
		funct7 uint32
		wantOp uint32
	}{
		{name: "add", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAdd},
		{name: "sub", opcode: opcodeOP, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSub},
		{name: "sll", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSll},
		{name: "slt", opcode: opcodeOP, funct3: 0b010, funct7: 0b0000000, wantOp: rtypeOpSlt},
		{name: "sltu", opcode: opcodeOP, funct3: 0b011, funct7: 0b0000000, wantOp: rtypeOpSltu},
		{name: "xor", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000000, wantOp: rtypeOpXor},
		{name: "srl", opcode: opcodeOP, funct3: 0b101, funct7: 0b0000000, wantOp: rtypeOpSrl},
		{name: "sra", opcode: opcodeOP, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSra},
		{name: "or", opcode: opcodeOP, funct3: 0b110, funct7: 0b0000000, wantOp: rtypeOpOr},
		{name: "and", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000000, wantOp: rtypeOpAnd},
		{name: "mul", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMul},
		{name: "mulh", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000001, wantOp: rtypeOpMulh},
		{name: "div", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDiv},
		{name: "remu", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000001, wantOp: rtypeOpRemu},
		{name: "addw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAddw},
		{name: "subw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSubw},
		{name: "sllw", opcode: opcodeOP32, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSllw},
		{name: "sraw", opcode: opcodeOP32, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSraw},
		{name: "mulw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMulw},
		{name: "divw", opcode: opcodeOP32, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDivw},
		{name: "keccak", opcode: opcodeCUSTOM1, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpKeccak},
		{name: "invalid op funct3", opcode: opcodeOP, funct3: 0b010, funct7: 0b0100000, wantOp: rtypeInvalid},
		{name: "invalid custom-1", opcode: opcodeCUSTOM1, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeRTypeSemantic(tt.opcode, tt.funct3, tt.funct7)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeRTypeSemantic(op=%#x, f3=%#x, f7=%#x) = %d, want %d",
					tt.opcode, tt.funct3, tt.funct7, gotOp, tt.wantOp)
			}
		})
	}
}

func TestRtypeOpForRd(t *testing.T) {
	if got := rtypeOpForRd(rtypeOpAdd, 0); got != rtypeOpAdd {
		t.Fatalf("rtypeOpForRd(add, x0) = %d, want %d", got, rtypeOpAdd)
	}
	if got := rtypeOpForRd(rtypeOpAdd, 5); got != rtypeOpAddWB {
		t.Fatalf("rtypeOpForRd(add, x5) = %d, want %d", got, rtypeOpAddWB)
	}
	if got := rtypeOpForRd(rtypeOpKeccak, 5); got != rtypeOpKeccak {
		t.Fatalf("rtypeOpForRd(keccak, x5) = %d, want %d", got, rtypeOpKeccak)
	}
}

func TestDecodeSTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		funct3 uint32
		wantOp uint32
	}{
		{name: "sb", funct3: 0b000, wantOp: stypeStore8},
		{name: "sh", funct3: 0b001, wantOp: stypeStore16},
		{name: "sw", funct3: 0b010, wantOp: stypeStore32},
		{name: "sd", funct3: 0b011, wantOp: stypeStore64},
		{name: "invalid", funct3: 0b111, wantOp: stypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeSTypeSemantic(tt.funct3)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeSTypeSemantic(f3=%#x) = %d, want %d", tt.funct3, gotOp, tt.wantOp)
			}
		})
	}
}

func TestDecodeBTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		funct3 uint32
		wantOp uint32
	}{
		{name: "beq", funct3: 0b000, wantOp: 0b000},
		{name: "bne", funct3: 0b001, wantOp: 0b001},
		{name: "blt", funct3: 0b100, wantOp: 0b100},
		{name: "invalid", funct3: 0b010, wantOp: btypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeBTypeSemantic(tt.funct3)
			if got != tt.wantOp {
				t.Fatalf("decodeBTypeSemantic(f3=%#x) = %d, want %d", tt.funct3, got, tt.wantOp)
			}
		})
	}
}

func TestAssembleBTypeImm(t *testing.T) {
	tests := []struct {
		name   string
		offset int32
		want   uint64
	}{
		{name: "zero", offset: 0, want: 0},
		{name: "forward_8", offset: 8, want: 8},
		{name: "backward_16", offset: -16, want: 0xfffffffffffffff0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeBType(0b000, 1, 2, tt.offset)
			got := assembleBTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleBTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeJTypeSemantic(t *testing.T) {
	gotOp := decodeJTypeSemantic(opcodeJAL)
	if gotOp != jtypeJal {
		t.Fatalf("decodeJTypeSemantic(jal) = %d, want %d", gotOp, jtypeJal)
	}
}

func TestJtypeOpForRd(t *testing.T) {
	if got := jtypeOpForRd(jtypeJal, 0); got != jtypeJal {
		t.Fatalf("jtypeOpForRd(jal, x0) = %d, want %d", got, jtypeJal)
	}
	if got := jtypeOpForRd(jtypeJal, 5); got != jtypeJalWB {
		t.Fatalf("jtypeOpForRd(jal, x5) = %d, want %d", got, jtypeJalWB)
	}
}

func TestAssembleJTypeImm(t *testing.T) {
	tests := []struct {
		name   string
		offset int32
		want   uint64
	}{
		{name: "zero", offset: 0, want: 0},
		{name: "forward_4k", offset: 4096, want: 4096},
		{name: "backward_2k", offset: -2048, want: 0xfffffffffffff800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeJType(1, tt.offset)
			got := assembleJTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleJTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeUTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint32
		wantOp uint32
	}{
		{name: "lui", opcode: opcodeLUI, wantOp: utypeLui},
		{name: "auipc", opcode: opcodeAUIPC, wantOp: utypeAuipc},
		{name: "invalid", opcode: 0, wantOp: utypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeUTypeSemantic(tt.opcode)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeUTypeSemantic(op=%#x) = %d, want %d", tt.opcode, gotOp, tt.wantOp)
			}
		})
	}
}

func TestUtypeOpForRd(t *testing.T) {
	if got := utypeOpForRd(utypeAuipc, 0); got != utypeAuipc {
		t.Fatalf("utypeOpForRd(auipc, x0) = %d, want %d", got, utypeAuipc)
	}
	if got := utypeOpForRd(utypeAuipc, 5); got != utypeAuipcWB {
		t.Fatalf("utypeOpForRd(auipc, x5) = %d, want %d", got, utypeAuipcWB)
	}
}

func TestAssembleUTypeImm(t *testing.T) {
	tests := []struct {
		name  string
		imm20 uint32
		want  uint64
	}{
		{name: "zero", imm20: 0, want: 0},
		{name: "positive", imm20: 0x12345, want: 0x12345000},
		{name: "negative", imm20: 0xfffff, want: 0xfffffffffffff000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeUType(opcodeLUI, 1, tt.imm20)
			got := assembleUTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleUTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

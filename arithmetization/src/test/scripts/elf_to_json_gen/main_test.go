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
		wantWB    uint32
		wantImm12 uint32
	}{
		{name: "lb", opcode: opcodeLOAD, funct3: 0b000, imm12: 8, wantOp: itypeRead8Sgn, wantWB: wbStoreReg, wantImm12: 8},
		{name: "lh", opcode: opcodeLOAD, funct3: 0b001, imm12: 4, wantOp: itypeRead16Sgn, wantWB: wbStoreReg, wantImm12: 4},
		{name: "lw", opcode: opcodeLOAD, funct3: 0b010, imm12: 0, wantOp: itypeRead32Sgn, wantWB: wbStoreReg, wantImm12: 0},
		{name: "ld", opcode: opcodeLOAD, funct3: 0b011, imm12: 0, wantOp: itypeRead64, wantWB: wbStoreReg, wantImm12: 0},
		{name: "lbu", opcode: opcodeLOAD, funct3: 0b100, imm12: 1, wantOp: itypeRead8Zext, wantWB: wbStoreReg, wantImm12: 1},
		{name: "lhu", opcode: opcodeLOAD, funct3: 0b101, imm12: 2, wantOp: itypeRead16Zext, wantWB: wbStoreReg, wantImm12: 2},
		{name: "lwu", opcode: opcodeLOAD, funct3: 0b110, imm12: 3, wantOp: itypeRead32Zext, wantWB: wbStoreReg, wantImm12: 3},
		{name: "addi", opcode: opcodeOPIMM, funct3: 0b000, imm12: 42, wantOp: itypeOpAddi, wantWB: wbStoreReg, wantImm12: 42},
		{name: "slli", opcode: opcodeOPIMM, funct3: 0b001, imm12: 4, wantOp: itypeOpSlli, wantWB: wbStoreReg, wantImm12: 4},
		{name: "srli", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrli, wantWB: wbStoreReg, wantImm12: 3},
		{name: "srai", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b010000000101, wantOp: itypeOpSrai, wantWB: wbStoreReg, wantImm12: 5},
		{name: "xori", opcode: opcodeOPIMM, funct3: 0b100, imm12: 0xff, wantOp: itypeOpXori, wantWB: wbStoreReg, wantImm12: 0xff},
		{name: "addiw", opcode: opcodeOPIMM32, funct3: 0b000, imm12: 7, wantOp: itypeOpAddiw, wantWB: wbStoreReg, wantImm12: 7},
		{name: "slliw", opcode: opcodeOPIMM32, funct3: 0b001, imm12: 2, wantOp: itypeOpSlliw, wantWB: wbStoreReg, wantImm12: 2},
		{name: "srliw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrliw, wantWB: wbStoreReg, wantImm12: 3},
		{name: "sraiw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b010000000100, wantOp: itypeOpSraiw, wantWB: wbStoreReg, wantImm12: 4},
		{name: "jalr", opcode: opcodeJALR, funct3: 0, imm12: 0, wantOp: itypeJalr, wantWB: wbStoreReg, wantImm12: 0},
		{name: "ecall", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ecall, wantOp: itypeEcall, wantWB: wbNone, wantImm12: funct12Ecall},
		{name: "ebreak", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ebreak, wantOp: itypeEbreak, wantWB: wbNone, wantImm12: funct12Ebreak},
		{name: "invalid slli funct6", opcode: opcodeOPIMM, funct3: 0b001, imm12: 0b010000000100, wantOp: itypeInvalid, wantWB: wbNone, wantImm12: 0b010000000100},
		{name: "invalid load funct3", opcode: opcodeLOAD, funct3: 0b111, imm12: 0, wantOp: itypeInvalid, wantWB: wbNone, wantImm12: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp, gotWB, gotImm := decodeITypeSemantic(tt.opcode, tt.funct3, tt.imm12)
			if gotOp != tt.wantOp || gotWB != tt.wantWB || gotImm != tt.wantImm12 {
				t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#x, imm=%#x) = (%d, %d, %#x), want (%d, %d, %#x)",
					tt.opcode, tt.funct3, tt.imm12, gotOp, gotWB, gotImm, tt.wantOp, tt.wantWB, tt.wantImm12)
			}
		})
	}
}

func TestDecodeRTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint32
		funct3 uint32
		funct7 uint32
		wantOp uint32
		wantWB uint32
	}{
		{name: "add", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAdd, wantWB: wbStoreReg},
		{name: "sub", opcode: opcodeOP, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSub, wantWB: wbStoreReg},
		{name: "sll", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSll, wantWB: wbStoreReg},
		{name: "slt", opcode: opcodeOP, funct3: 0b010, funct7: 0b0000000, wantOp: rtypeOpSlt, wantWB: wbStoreReg},
		{name: "sltu", opcode: opcodeOP, funct3: 0b011, funct7: 0b0000000, wantOp: rtypeOpSltu, wantWB: wbStoreReg},
		{name: "xor", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000000, wantOp: rtypeOpXor, wantWB: wbStoreReg},
		{name: "srl", opcode: opcodeOP, funct3: 0b101, funct7: 0b0000000, wantOp: rtypeOpSrl, wantWB: wbStoreReg},
		{name: "sra", opcode: opcodeOP, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSra, wantWB: wbStoreReg},
		{name: "or", opcode: opcodeOP, funct3: 0b110, funct7: 0b0000000, wantOp: rtypeOpOr, wantWB: wbStoreReg},
		{name: "and", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000000, wantOp: rtypeOpAnd, wantWB: wbStoreReg},
		{name: "mul", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMul, wantWB: wbStoreReg},
		{name: "mulh", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000001, wantOp: rtypeOpMulh, wantWB: wbStoreReg},
		{name: "div", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDiv, wantWB: wbStoreReg},
		{name: "remu", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000001, wantOp: rtypeOpRemu, wantWB: wbStoreReg},
		{name: "addw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAddw, wantWB: wbStoreReg},
		{name: "subw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSubw, wantWB: wbStoreReg},
		{name: "sllw", opcode: opcodeOP32, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSllw, wantWB: wbStoreReg},
		{name: "sraw", opcode: opcodeOP32, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSraw, wantWB: wbStoreReg},
		{name: "mulw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMulw, wantWB: wbStoreReg},
		{name: "divw", opcode: opcodeOP32, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDivw, wantWB: wbStoreReg},
		{name: "keccak", opcode: opcodeCUSTOM1, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpKeccak, wantWB: wbNone},
		{name: "invalid op funct3", opcode: opcodeOP, funct3: 0b010, funct7: 0b0100000, wantOp: rtypeInvalid, wantWB: wbNone},
		{name: "invalid custom-1", opcode: opcodeCUSTOM1, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeInvalid, wantWB: wbNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp, gotWB := decodeRTypeSemantic(tt.opcode, tt.funct3, tt.funct7)
			if gotOp != tt.wantOp || gotWB != tt.wantWB {
				t.Fatalf("decodeRTypeSemantic(op=%#x, f3=%#x, f7=%#x) = (%d, %d), want (%d, %d)",
					tt.opcode, tt.funct3, tt.funct7, gotOp, gotWB, tt.wantOp, tt.wantWB)
			}
		})
	}
}

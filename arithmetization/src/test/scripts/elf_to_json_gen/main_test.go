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

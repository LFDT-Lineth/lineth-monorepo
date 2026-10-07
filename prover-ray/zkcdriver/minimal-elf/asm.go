package minimalelf

import (
	"encoding/binary"
	"fmt"
)

// reg is a RISC-V integer register, named by its ABI mnemonic below.
type reg uint32

const (
	zero reg = iota
	ra
	sp
	gp
	tp
	t0
	t1
	t2
	s0
	s1
	a0
	a1
	a2
	a3
	a4
	a5
	a6
	a7
	s2
	s3
	s4
	s5
	s6
	s7
	s8
	s9
	s10
	s11
	t3
	t4
	t5
	t6
)

// RV64IM major opcodes, plus the custom-1 opcode carrying the Lineth
// precompiles (R_KECCAK, R_POSEIDON2, R_WRITE_OUTPUT).
const (
	opLoad     = 0b0000011
	opOpImm    = 0b0010011
	opAuipc    = 0b0010111
	opOpImm32  = 0b0011011
	opStore    = 0b0100011
	opCustom1  = 0b0101011
	opOp       = 0b0110011
	opLui      = 0b0110111
	opOp32     = 0b0111011
	opBranch   = 0b1100011
	opJalr     = 0b1100111
	opJal      = 0b1101111
	opSystem   = 0b1110011
	funct7Sub  = 0b0100000 // SUB/SRA/SRAI selector
	funct7MulM = 0b0000001 // M-extension selector
)

// assembler builds a RISC-V instruction stream one instruction per method
// call. Branch and jump targets are labels, resolved by [assembler.bytes];
// a label may be referenced before it is defined.
type assembler struct {
	words  []uint32
	labels map[string]int // label -> instruction index
	fixups []fixup
}

// fixup patches the instruction at index at once the offset to label is known.
type fixup struct {
	at     int
	label  string
	encode func(off int32) uint32
}

func newAssembler() *assembler {
	return &assembler{labels: map[string]int{}}
}

func (a *assembler) emit(w uint32) { a.words = append(a.words, w) }

// label binds name to the address of the next emitted instruction.
func (a *assembler) label(name string) {
	if _, ok := a.labels[name]; ok {
		panic(fmt.Sprintf("minimalelf: duplicate label %q", name))
	}
	a.labels[name] = len(a.words)
}

// emitTo emits a placeholder for a pc-relative instruction to label.
func (a *assembler) emitTo(label string, encode func(off int32) uint32) {
	a.fixups = append(a.fixups, fixup{at: len(a.words), label: label, encode: encode})
	a.emit(0)
}

// bytes resolves every label reference and returns the little-endian
// instruction stream.
func (a *assembler) bytes() []byte {
	for _, f := range a.fixups {
		target, ok := a.labels[f.label]
		if !ok {
			panic(fmt.Sprintf("minimalelf: undefined label %q", f.label))
		}
		a.words[f.at] = f.encode(int32(4 * (target - f.at)))
	}
	buf := make([]byte, 4*len(a.words))
	for i, w := range a.words {
		binary.LittleEndian.PutUint32(buf[4*i:], w)
	}
	return buf
}

// I-type ALU, RV64 shifts take a 6-bit shamt and the *W shifts a 5-bit one.
func (a *assembler) addi(rd, rs1 reg, imm int32)  { a.emit(encIType(opOpImm, 0b000, rd, rs1, imm)) }
func (a *assembler) slti(rd, rs1 reg, imm int32)  { a.emit(encIType(opOpImm, 0b010, rd, rs1, imm)) }
func (a *assembler) sltiu(rd, rs1 reg, imm int32) { a.emit(encIType(opOpImm, 0b011, rd, rs1, imm)) }
func (a *assembler) xori(rd, rs1 reg, imm int32)  { a.emit(encIType(opOpImm, 0b100, rd, rs1, imm)) }
func (a *assembler) ori(rd, rs1 reg, imm int32)   { a.emit(encIType(opOpImm, 0b110, rd, rs1, imm)) }
func (a *assembler) andi(rd, rs1 reg, imm int32)  { a.emit(encIType(opOpImm, 0b111, rd, rs1, imm)) }
func (a *assembler) slli(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm, 0b001, 0, rd, rs1, sh, 63))
}
func (a *assembler) srli(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm, 0b101, 0, rd, rs1, sh, 63))
}
func (a *assembler) srai(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm, 0b101, funct7Sub, rd, rs1, sh, 63))
}
func (a *assembler) addiw(rd, rs1 reg, imm int32) { a.emit(encIType(opOpImm32, 0b000, rd, rs1, imm)) }
func (a *assembler) slliw(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm32, 0b001, 0, rd, rs1, sh, 31))
}
func (a *assembler) srliw(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm32, 0b101, 0, rd, rs1, sh, 31))
}
func (a *assembler) sraiw(rd, rs1 reg, sh uint32) {
	a.emit(encShift(opOpImm32, 0b101, funct7Sub, rd, rs1, sh, 31))
}

// R-type ALU.
func (a *assembler) add(rd, rs1, rs2 reg)  { a.emit(encRType(opOp, 0b000, 0, rd, rs1, rs2)) }
func (a *assembler) mul(rd, rs1, rs2 reg)  { a.emit(encRType(opOp, 0b000, funct7MulM, rd, rs1, rs2)) }
func (a *assembler) addw(rd, rs1, rs2 reg) { a.emit(encRType(opOp32, 0b000, 0, rd, rs1, rs2)) }
func (a *assembler) subw(rd, rs1, rs2 reg) { a.emit(encRType(opOp32, 0b000, funct7Sub, rd, rs1, rs2)) }
func (a *assembler) sllw(rd, rs1, rs2 reg) { a.emit(encRType(opOp32, 0b001, 0, rd, rs1, rs2)) }
func (a *assembler) srlw(rd, rs1, rs2 reg) { a.emit(encRType(opOp32, 0b101, 0, rd, rs1, rs2)) }
func (a *assembler) sraw(rd, rs1, rs2 reg) { a.emit(encRType(opOp32, 0b101, funct7Sub, rd, rs1, rs2)) }

// U-type. imm20 is the raw upper-immediate field.
func (a *assembler) lui(rd reg, imm20 uint32)   { a.emit(encUType(opLui, rd, imm20)) }
func (a *assembler) auipc(rd reg, imm20 uint32) { a.emit(encUType(opAuipc, rd, imm20)) }

// Loads (rd = mem[rs1+off]) and stores (mem[rs1+off] = rs2).
func (a *assembler) lb(rd, rs1 reg, off int32)  { a.emit(encIType(opLoad, 0b000, rd, rs1, off)) }
func (a *assembler) lh(rd, rs1 reg, off int32)  { a.emit(encIType(opLoad, 0b001, rd, rs1, off)) }
func (a *assembler) lw(rd, rs1 reg, off int32)  { a.emit(encIType(opLoad, 0b010, rd, rs1, off)) }
func (a *assembler) ld(rd, rs1 reg, off int32)  { a.emit(encIType(opLoad, 0b011, rd, rs1, off)) }
func (a *assembler) sb(rs2, rs1 reg, off int32) { a.emit(encSType(opStore, 0b000, rs1, rs2, off)) }
func (a *assembler) sh(rs2, rs1 reg, off int32) { a.emit(encSType(opStore, 0b001, rs1, rs2, off)) }
func (a *assembler) sw(rs2, rs1 reg, off int32) { a.emit(encSType(opStore, 0b010, rs1, rs2, off)) }
func (a *assembler) sd(rs2, rs1 reg, off int32) { a.emit(encSType(opStore, 0b011, rs1, rs2, off)) }

// Branches and jumps to labels.
func (a *assembler) branch(funct3 uint32, rs1, rs2 reg, label string) {
	a.emitTo(label, func(off int32) uint32 { return encBType(opBranch, funct3, rs1, rs2, off) })
}
func (a *assembler) beq(rs1, rs2 reg, label string)  { a.branch(0b000, rs1, rs2, label) }
func (a *assembler) bne(rs1, rs2 reg, label string)  { a.branch(0b001, rs1, rs2, label) }
func (a *assembler) blt(rs1, rs2 reg, label string)  { a.branch(0b100, rs1, rs2, label) }
func (a *assembler) bge(rs1, rs2 reg, label string)  { a.branch(0b101, rs1, rs2, label) }
func (a *assembler) bltu(rs1, rs2 reg, label string) { a.branch(0b110, rs1, rs2, label) }
func (a *assembler) bgeu(rs1, rs2 reg, label string) { a.branch(0b111, rs1, rs2, label) }
func (a *assembler) jal(rd reg, label string) {
	a.emitTo(label, func(off int32) uint32 { return encJType(opJal, rd, off) })
}
func (a *assembler) jalr(rd, rs1 reg, off int32) { a.emit(encIType(opJalr, 0b000, rd, rs1, off)) }
func (a *assembler) ecall()                      { a.emit(opSystem) }

// Lineth precompiles on the custom-1 opcode.

// keccak hashes length bytes at msg and writes the digest to out.
func (a *assembler) keccak(out, msg, length reg) {
	a.emit(encRType(opCustom1, 0b000, 0, out, msg, length))
}

// poseidon2 permutes the state at in and writes the result to out.
func (a *assembler) poseidon2(out, in reg) { a.emit(encRType(opCustom1, 0b001, 0, out, in, zero)) }

// writeOutput appends size bytes at addr to the guest_output channel.
func (a *assembler) writeOutput(addr, size reg) {
	a.emit(encRType(opCustom1, 0b010, 0, zero, addr, size))
}

// Instruction-format encoders. They panic on out-of-range immediates so that
// a mistyped program fails loudly instead of silently truncating.

func encRType(opcode, funct3, funct7 uint32, rd, rs1, rs2 reg) uint32 {
	return funct7<<25 | uint32(rs2)<<20 | uint32(rs1)<<15 | funct3<<12 | uint32(rd)<<7 | opcode
}

func encIType(opcode, funct3 uint32, rd, rs1 reg, imm int32) uint32 {
	checkSigned(imm, 12)
	return uint32(imm&0xFFF)<<20 | uint32(rs1)<<15 | funct3<<12 | uint32(rd)<<7 | opcode
}

// encShift encodes an immediate shift: funct7 sits above a shamt of up to
// maxShamt (63 for RV64 shifts, 31 for the *W variants).
func encShift(opcode, funct3, funct7 uint32, rd, rs1 reg, shamt, maxShamt uint32) uint32 {
	if shamt > maxShamt {
		panic(fmt.Sprintf("minimalelf: shift amount %d exceeds %d", shamt, maxShamt))
	}
	return funct7<<25 | shamt<<20 | uint32(rs1)<<15 | funct3<<12 | uint32(rd)<<7 | opcode
}

func encSType(opcode, funct3 uint32, rs1, rs2 reg, imm int32) uint32 {
	checkSigned(imm, 12)
	u := uint32(imm)
	return (u>>5&0x7F)<<25 | uint32(rs2)<<20 | uint32(rs1)<<15 | funct3<<12 | (u&0x1F)<<7 | opcode
}

func encBType(opcode, funct3 uint32, rs1, rs2 reg, imm int32) uint32 {
	checkSigned(imm, 13)
	checkEven(imm)
	u := uint32(imm)
	return (u>>12&1)<<31 | (u>>5&0x3F)<<25 | uint32(rs2)<<20 | uint32(rs1)<<15 |
		funct3<<12 | (u>>1&0xF)<<8 | (u>>11&1)<<7 | opcode
}

func encUType(opcode uint32, rd reg, imm20 uint32) uint32 {
	if imm20 > 0xFFFFF {
		panic(fmt.Sprintf("minimalelf: upper immediate %#x exceeds 20 bits", imm20))
	}
	return imm20<<12 | uint32(rd)<<7 | opcode
}

func encJType(opcode uint32, rd reg, imm int32) uint32 {
	checkSigned(imm, 21)
	checkEven(imm)
	u := uint32(imm)
	return (u>>20&1)<<31 | (u>>1&0x3FF)<<21 | (u>>11&1)<<20 | (u>>12&0xFF)<<12 | uint32(rd)<<7 | opcode
}

func checkSigned(imm int32, bits uint) {
	if lim := int32(1) << (bits - 1); imm < -lim || imm >= lim {
		panic(fmt.Sprintf("minimalelf: immediate %d does not fit %d signed bits", imm, bits))
	}
}

func checkEven(imm int32) {
	if imm&1 != 0 {
		panic(fmt.Sprintf("minimalelf: pc-relative offset %d is not even", imm))
	}
}

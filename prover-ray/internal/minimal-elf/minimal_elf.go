package minimalelf

import (
	"bytes"
	"encoding/binary"
)

const (
	DefaultEntryPoint  = 0x00800000
	DefaultSectionAddr = 0x00800000
)

// ValidSectionData is a valid RISC-V auipc x5, 0 instruction encoding.
var ValidSectionData = []byte{0x97, 0x02, 0x00, 0x00}

// ExitZeroSectionData is a tiny valid RISC-V program that halts through the
// Linux-style exit syscall path used by the RISC-V arithmetization:
//
//	addi a7, x0, 93
//	addi a0, x0, 0
//	ecall
var ExitZeroSectionData = []byte{
	0x93, 0x08, 0xd0, 0x05,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// MemoryRoundTripSectionData is a tiny valid RISC-V program that exercises a
// guest-issued store and load before halting through the same exit syscall
// path as ExitZeroSectionData. It stores 42 to a scratch word 32 bytes past
// its own entry point (computed via auipc, so the program is
// position-independent), loads it back, and exits with code 0 if the values
// match or code 1 otherwise:
//
//	auipc t0, 0
//	addi  t1, x0, 42
//	sw    t1, 32(t0)
//	lw    t2, 32(t0)
//	addi  a7, x0, 93
//	bne   t1, t2, fail
//	addi  a0, x0, 0
//	ecall
//
// fail:
//	addi  a0, x0, 1
//	ecall
var MemoryRoundTripSectionData = []byte{
	0x97, 0x02, 0x00, 0x00,
	0x13, 0x03, 0xa0, 0x02,
	0x23, 0xa0, 0x62, 0x02,
	0x83, 0xa3, 0x02, 0x02,
	0x93, 0x08, 0xd0, 0x05,
	0x63, 0x16, 0x73, 0x00,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// ArithmeticSectionData is a tiny valid RISC-V program that exercises
// LUI, JAL, JALR, an R-type base op (ADD) and an M-extension op (MUL) before
// halting through the same exit syscall path as ExitZeroSectionData. Main
// flow calls a subroutine via JAL, which computes t3 = (t0+t1)*3 using ADD
// and MUL, builds the same value independently via LUI+ADDI into t4, and
// returns via JALR; main then branches to a failure exit if the two computed
// values disagree. a7 (the syscall number) is set once, before the branch,
// so both the success and failure paths can ecall with only a0 differing:
//
//	lui  t0, 1            // t0 = 0x1000
//	addi t1, x0, 5         // t1 = 5
//	addi a7, x0, 93
//	jal  ra, add_mul       // ra = pc+4; jump to subroutine
//	bne  t3, t4, fail
//	addi a0, x0, 0
//	ecall
//
// fail:
//	addi a0, x0, 1
//	ecall
//
// add_mul:
//	add  t2, t0, t1        // t2 = t0 + t1 = 0x1005
//	addi t3, x0, 3
//	mul  t3, t2, t3        // t3 = t2 * 3 = 0x300f
//	lui  t4, 3             // t4 = 0x3000
//	addi t4, t4, 15        // t4 = 0x300f (expected == t3)
//	jalr x0, ra, 0         // return to caller
var ArithmeticSectionData = []byte{
	0xb7, 0x12, 0x00, 0x00,
	0x13, 0x03, 0x50, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0xef, 0x00, 0x80, 0x01,
	0x63, 0x16, 0xde, 0x01,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0xb3, 0x83, 0x62, 0x00,
	0x13, 0x0e, 0x30, 0x00,
	0x33, 0x8e, 0xc3, 0x03,
	0xb7, 0x3e, 0x00, 0x00,
	0x93, 0x8e, 0xfe, 0x00,
	0x67, 0x80, 0x00, 0x00,
}

// ExitOneSectionData is a tiny valid RISC-V program that halts through the
// exit syscall path with a nonzero exit code, which main.zkc must reject:
//
//	addi a7, x0, 93
//	addi a0, x0, 1
//	ecall
var ExitOneSectionData = []byte{
	0x93, 0x08, 0xd0, 0x05,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// BranchesSectionData is a tiny valid RISC-V program that exercises all six
// B-type variants (BEQ, BNE, BLT, BGE, BLTU, BGEU) with t0=3, t1=-1
// (0xFFFFFFFFFFFFFFFF, i.e. a large unsigned value but a negative signed
// one), t2=3, checking both that each variant fires when its condition
// holds and that it does NOT fire when its condition doesn't hold, before
// halting through the same exit syscall path as ExitZeroSectionData:
//
//	addi t0, x0, 3
//	addi t1, x0, -1
//	addi t2, x0, 3
//	addi a7, x0, 93
//	beq  t0, t2, +8   ; true-check: must be taken (3 == 3)
//	jal  x0, fail
//	bne  t0, t1, +8   ; true-check: must be taken (3 != -1)
//	jal  x0, fail
//	blt  t1, t0, +8   ; true-check: must be taken (-1 < 3, signed)
//	jal  x0, fail
//	bge  t0, t1, +8   ; true-check: must be taken (3 >= -1, signed)
//	jal  x0, fail
//	bltu t0, t1, +8   ; true-check: must be taken (3 < huge, unsigned)
//	jal  x0, fail
//	bgeu t1, t0, +8   ; true-check: must be taken (huge >= 3, unsigned)
//	jal  x0, fail
//	beq  t0, t1, fail ; false-check: must NOT be taken (3 != -1)
//	bne  t0, t2, fail ; false-check: must NOT be taken (3 == 3)
//	blt  t0, t1, fail ; false-check: must NOT be taken (3 < -1 signed is false)
//	bge  t1, t0, fail ; false-check: must NOT be taken (-1 >= 3 signed is false)
//	bltu t1, t0, fail ; false-check: must NOT be taken (huge < 3 unsigned is false)
//	bgeu t0, t1, fail ; false-check: must NOT be taken (3 >= huge unsigned is false)
//	addi a0, x0, 0
//	ecall
//
// fail:
//	addi a0, x0, 1
//	ecall
var BranchesSectionData = []byte{
	0x93, 0x02, 0x30, 0x00,
	0x13, 0x03, 0xf0, 0xff,
	0x93, 0x03, 0x30, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x63, 0x84, 0x72, 0x00,
	0x6f, 0x00, 0xc0, 0x04,
	0x63, 0x94, 0x62, 0x00,
	0x6f, 0x00, 0x40, 0x04,
	0x63, 0x44, 0x53, 0x00,
	0x6f, 0x00, 0xc0, 0x03,
	0x63, 0xd4, 0x62, 0x00,
	0x6f, 0x00, 0x40, 0x03,
	0x63, 0xe4, 0x62, 0x00,
	0x6f, 0x00, 0xc0, 0x02,
	0x63, 0x74, 0x53, 0x00,
	0x6f, 0x00, 0x40, 0x02,
	0x63, 0x80, 0x62, 0x02,
	0x63, 0x9e, 0x72, 0x00,
	0x63, 0xcc, 0x62, 0x00,
	0x63, 0x5a, 0x53, 0x00,
	0x63, 0x68, 0x53, 0x00,
	0x63, 0xf6, 0x62, 0x00,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// LoadStoreWidthsSectionData is a tiny valid RISC-V program that exercises
// the remaining S-type/I-type width variants not covered by
// MemoryRoundTripSectionData (which only exercises SW/LW): SB/LB (with
// sign extension of a negative byte), SH/LH (with sign extension of a
// negative halfword), and SD/LD (a full 64-bit round trip). It stores to
// three non-overlapping scratch slots 96/98/104 bytes past its own entry
// point (past the end of its own code), loads each back, and exits with
// code 0 only if every round trip (including sign extension) matches,
// or code 1 otherwise:
//
//	auipc t0, 0
//	addi  a7, x0, 93
//	addi  t1, x0, 171        ; t1 = 0xAB (low byte, sign bit set)
//	sb    t1, 96(t0)
//	lb    t2, 96(t0)         ; t2 = sign_extend(0xAB) = -85
//	addi  t3, x0, -85
//	bne   t2, t3, fail
//	addi  t1, x0, -1         ; t1 = 0xFFFF...FFFF (low halfword = 0xFFFF)
//	sh    t1, 98(t0)
//	lh    t2, 98(t0)         ; t2 = sign_extend(0xFFFF) = -1
//	addi  t3, x0, -1
//	bne   t2, t3, fail
//	addi  t1, x0, 1000
//	sd    t1, 104(t0)
//	ld    t2, 104(t0)        ; t2 = 1000 (exact round trip, no extension)
//	bne   t2, t1, fail
//	addi  a0, x0, 0
//	ecall
//
// fail:
//	addi  a0, x0, 1
//	ecall
var LoadStoreWidthsSectionData = []byte{
	0x97, 0x02, 0x00, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x13, 0x03, 0xb0, 0x0a,
	0x23, 0x80, 0x62, 0x06,
	0x83, 0x83, 0x02, 0x06,
	0x13, 0x0e, 0xb0, 0xfa,
	0x63, 0x98, 0xc3, 0x03,
	0x13, 0x03, 0xf0, 0xff,
	0x23, 0x91, 0x62, 0x06,
	0x83, 0x93, 0x22, 0x06,
	0x13, 0x0e, 0xf0, 0xff,
	0x63, 0x9e, 0xc3, 0x01,
	0x13, 0x03, 0x80, 0x3e,
	0x23, 0xb4, 0x62, 0x06,
	0x83, 0xb3, 0x82, 0x06,
	0x63, 0x96, 0x39, 0x00,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// Poseidon2SectionData is a tiny valid RISC-V program that invokes the
// R_POSEIDON2 precompile (custom-1 opcode, funct3=0b001) over a 16-word
// (64-byte) all-zero input block (guest RAM starts zero-initialized, so no
// explicit stores are needed to prepare it), then checks the first output
// word against the known-good vector for permuting an all-zero KoalaBear
// state (arithmetization/src/test/zkc/poseidon2/permutation.accepts),
// before halting through the same exit syscall path as ExitZeroSectionData:
//
//	auipc t0, 0
//	addi  a7, x0, 93
//	auipc t1, 0
//	addi  t1, t1, 320        ; t1 = output_offset = t0 + 320
//	auipc t2, 0
//	addi  t2, t2, 256        ; t2 = input_offset = t0 + 256 (still zeroed)
//	.insn r CUSTOM_1, 0b001, 0, t1, t2, x0   ; poseidon2(t2, t1)
//	lw    t3, 0(t1)          ; t3 = first output word
//	lui   t4, 0x35596
//	addi  t4, t4, 407        ; t4 = 0x35596197 (expected first output word)
//	bne   t3, t4, fail
//	addi  a0, x0, 0
//	ecall
//
// fail:
//	addi  a0, x0, 1
//	ecall
var Poseidon2SectionData = []byte{
	0x97, 0x02, 0x00, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x17, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x03, 0x14,
	0x97, 0x03, 0x00, 0x00,
	0x93, 0x83, 0x03, 0x10,
	0x2b, 0x93, 0x03, 0x00,
	0x03, 0x2e, 0x03, 0x00,
	0xb7, 0x6e, 0x59, 0x35,
	0x93, 0x8e, 0x7e, 0x19,
	0x63, 0x16, 0xde, 0x01,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// KeccakSectionData is a tiny valid RISC-V program that invokes the
// R_KECCAK precompile (custom-1 opcode, funct3=0b000) over an empty message
// (msg_length=0), then checks the first output word of the resulting
// digest against the well-known Keccak-256 (not SHA3-256 — no 0x06 domain
// separator) empty-string digest
// c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470, before
// halting through the same exit syscall path as ExitZeroSectionData:
//
//	auipc t0, 0
//	addi  a7, x0, 93
//	auipc t1, 0
//	addi  t1, t1, 128        ; t1 = output_offset = t0 + 128
//	.insn r CUSTOM_1, 0b000, 0, t1, t0, x0   ; keccak(t0, x0, t1); msg_length=0
//	lw    t3, 0(t1)          ; t3 = first digest word (little-endian)
//	lui   t4, 0x146d
//	addi  t4, t4, 709        ; t4 = 0x0146d2c5 (expected first digest word)
//	bne   t3, t4, fail
//	addi  a0, x0, 0
//	ecall
//
// fail:
//	addi  a0, x0, 1
//	ecall
var KeccakSectionData = []byte{
	0x97, 0x02, 0x00, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x17, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x03, 0x08,
	0x2b, 0x83, 0x02, 0x00,
	0x03, 0x2e, 0x03, 0x00,
	0xb7, 0xde, 0x46, 0x01,
	0x93, 0x8e, 0x5e, 0x2c,
	0x63, 0x16, 0xde, 0x01,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// WriteOutputSectionData is a tiny valid RISC-V program that stores the
// bytes 'A', 'B', 'C' to scratch RAM, then invokes the R_WRITE_OUTPUT
// precompile (custom-1 opcode, funct3=0b010) to copy those 3 bytes into the
// public guest_output buffer at offset 0, before halting through the same
// exit syscall path as ExitZeroSectionData. Unlike the other fixtures,
// success here is checked externally by inspecting the zkc trace's
// "guest_output" public output, not via a guest-side branch:
//
//	auipc t0, 0
//	addi  a7, x0, 93
//	addi  t1, t0, 64          ; t1 = scratch address
//	addi  t2, x0, 0x41
//	sb    t2, 0(t1)
//	addi  t2, x0, 0x42
//	sb    t2, 1(t1)
//	addi  t2, x0, 0x43
//	sb    t2, 2(t1)
//	addi  t3, x0, 3           ; size = 3
//	.insn r CUSTOM_1, 0b010, 0, x0, t1, t3   ; write_output(t1, t3)
//	addi  a0, x0, 0
//	ecall
var WriteOutputSectionData = []byte{
	0x97, 0x02, 0x00, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x13, 0x83, 0x02, 0x04,
	0x93, 0x03, 0x10, 0x04,
	0x23, 0x00, 0x73, 0x00,
	0x93, 0x03, 0x20, 0x04,
	0xa3, 0x00, 0x73, 0x00,
	0x93, 0x03, 0x30, 0x04,
	0x23, 0x01, 0x73, 0x00,
	0x13, 0x0e, 0x30, 0x00,
	0x2b, 0x20, 0xc3, 0x01,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// ImmediateALUSectionData is a tiny valid RISC-V program that exercises the
// I-type ALU immediates not covered elsewhere (SLTI, SLTIU, XORI, ORI,
// ANDI, SLLI, SRLI, SRAI) plus the RV64 word-width I-type variants (ADDIW,
// SLLIW, SRLIW, SRAIW), checking each computed result against an
// independently loaded expected constant, before halting through the same
// exit syscall path as ExitZeroSectionData. t2=6 is the shared operand for
// the base ALU checks; SRAI uses t2=-8 to exercise sign-preserving shifts;
// the *W checks reload t2 with 32-bit boundary values (0x7FFFFFFF,
// 0xFFFFFFFF) to exercise word-width wraparound and sign extension. Because
// LUI sign-extends its 32-bit result to 64 bits, loading a value whose bit
// 31 is 0 but whose upper 20 bits (as loaded via LUI) form a pattern with
// bit 31 set (e.g. 0x7FFFFFFF, via LUI 0x80000 + ADDI -1) requires an extra
// SLLI 32 / SRLI 32 pair to zero the sign-extended upper 32 bits afterward:
//
//	addi t2, x0, 6
//	addi a7, x0, 93
//	slti  t0, t2, 10           ; expect 1  (6 < 10 signed)
//	bne   t0, 1, fail
//	sltiu t0, t2, 10           ; expect 1  (6 < 10 unsigned)
//	bne   t0, 1, fail
//	xori  t0, t2, 3            ; expect 5  (6 ^ 3)
//	bne   t0, 5, fail
//	ori   t0, t2, 1            ; expect 7  (6 | 1)
//	bne   t0, 7, fail
//	andi  t0, t2, 2            ; expect 2  (6 & 2)
//	bne   t0, 2, fail
//	slli  t0, t2, 2            ; expect 24 (6 << 2)
//	bne   t0, 24, fail
//	srli  t0, t2, 1            ; expect 3  (6 >> 1)
//	bne   t0, 3, fail
//	addi  t2, x0, -8
//	srai  t0, t2, 1            ; expect -4 (arithmetic shift preserves sign)
//	bne   t0, -4, fail
//	lui t2, 0x80000 / addi t2, t2, -1 / slli t2, t2, 32 / srli t2, t2, 32  ; t2 = 0x7FFFFFFF
//	addiw t0, t2, 10           ; expect sign_extend32(0x7FFFFFFF+10) (wraps at 32 bits)
//	bne   t0, expected, fail
//	addi  t2, x0, 6
//	slliw t0, t2, 2            ; expect 24 (word-width, still fits)
//	bne   t0, 24, fail
//	lui t2, 0x0 / addi t2, t2, -1                                          ; t2 = 0xFFFFFFFFFFFFFFFF (low32=0xFFFFFFFF)
//	srliw t0, t2, 1            ; expect 0x7FFFFFFF (logical shift within 32 bits)
//	bne   t0, expected, fail
//	sraiw t0, t2, 1            ; expect -1 (arithmetic shift of all-ones is still all-ones)
//	bne   t0, -1, fail
//	addi  a0, x0, 0
//	ecall
//
// fail:
//	addi  a0, x0, 1
//	ecall
var ImmediateALUSectionData = []byte{
	0x93, 0x03, 0x60, 0x00,
	0x93, 0x08, 0xd0, 0x05,
	0x93, 0xa2, 0xa3, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x13, 0x00,
	0x63, 0x92, 0x62, 0x0e,
	0x93, 0xb2, 0xa3, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x13, 0x00,
	0x63, 0x9a, 0x62, 0x0c,
	0x93, 0xc2, 0x33, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x53, 0x00,
	0x63, 0x92, 0x62, 0x0c,
	0x93, 0xe2, 0x13, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x73, 0x00,
	0x63, 0x9a, 0x62, 0x0a,
	0x93, 0xf2, 0x23, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x23, 0x00,
	0x63, 0x92, 0x62, 0x0a,
	0x93, 0x92, 0x23, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x83, 0x01,
	0x63, 0x9a, 0x62, 0x08,
	0x93, 0xd2, 0x13, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x33, 0x00,
	0x63, 0x92, 0x62, 0x08,
	0x93, 0x03, 0x80, 0xff,
	0x93, 0xd2, 0x13, 0x40,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0xc3, 0xff,
	0x63, 0x98, 0x62, 0x06,
	0xb7, 0x03, 0x00, 0x80,
	0x93, 0x83, 0xf3, 0xff,
	0x93, 0x93, 0x03, 0x02,
	0x93, 0xd3, 0x03, 0x02,
	0x9b, 0x82, 0xa3, 0x00,
	0x37, 0x03, 0x00, 0x80,
	0x13, 0x03, 0x93, 0x00,
	0x63, 0x98, 0x62, 0x04,
	0x93, 0x03, 0x60, 0x00,
	0x9b, 0x92, 0x23, 0x00,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0x83, 0x01,
	0x63, 0x9e, 0x62, 0x02,
	0xb7, 0x03, 0x00, 0x00,
	0x93, 0x83, 0xf3, 0xff,
	0x9b, 0xd2, 0x13, 0x00,
	0x37, 0x03, 0x00, 0x80,
	0x13, 0x03, 0xf3, 0xff,
	0x13, 0x13, 0x03, 0x02,
	0x13, 0x53, 0x03, 0x02,
	0x63, 0x9e, 0x62, 0x00,
	0x9b, 0xd2, 0x13, 0x40,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0xf3, 0xff,
	0x63, 0x96, 0x62, 0x00,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// WordWidthSectionData is a tiny valid RISC-V program that exercises the
// RV64 R-type word-width variants (ADDW, SUBW, SLLW, SRLW, SRAW), each of
// which operates on the low 32 bits of its operands and sign-extends the
// 32-bit result to 64 bits — distinct from the base 64-bit R-type ops
// already covered by ArithmeticSectionData. Operands are chosen to force
// 32-bit wraparound (ADDW, SLLW) and sign-extension of a negative 32-bit
// result (SUBW, SRAW), before halting through the same exit syscall path
// as ExitZeroSectionData. See ImmediateALUSectionData's comment for why
// loading 0x7FFFFFFF needs an extra SLLI 32 / SRLI 32 pair after LUI+ADDI:
//
//	addi a7, x0, 93
//	lui t2, 0x80000 / addi t2, t2, -1 / slli t2, t2, 32 / srli t2, t2, 32  ; t2 = 0x7FFFFFFF
//	addi t3, x0, 1
//	addw t0, t2, t3            ; expect sign_extend32(0x7FFFFFFF+1) (wraps to INT32_MIN)
//	bne  t0, expected, fail
//	addi t2, x0, 5
//	addi t3, x0, 10
//	subw t0, t2, t3            ; expect -5 (word-width subtraction underflows, sign-extends)
//	bne  t0, -5, fail
//	addi t2, x0, 1
//	addi t3, x0, 31
//	sllw t0, t2, t3            ; expect sign_extend32(1<<31) (= INT32_MIN)
//	bne  t0, expected, fail
//	lui t2, 0x0 / addi t2, t2, -1                                          ; t2 = 0xFFFFFFFFFFFFFFFF (low32=0xFFFFFFFF)
//	addi t3, x0, 1
//	srlw t0, t2, t3            ; expect 0x7FFFFFFF (logical shift within 32 bits)
//	bne  t0, expected, fail
//	sraw t0, t2, t3            ; expect -1 (arithmetic shift of all-ones is still all-ones)
//	bne  t0, -1, fail
//	addi a0, x0, 0
//	ecall
//
// fail:
//	addi a0, x0, 1
//	ecall
var WordWidthSectionData = []byte{
	0x93, 0x08, 0xd0, 0x05,
	0xb7, 0x03, 0x00, 0x80,
	0x93, 0x83, 0xf3, 0xff,
	0x93, 0x93, 0x03, 0x02,
	0x93, 0xd3, 0x03, 0x02,
	0x13, 0x0e, 0x10, 0x00,
	0xbb, 0x82, 0xc3, 0x01,
	0x37, 0x03, 0x00, 0x80,
	0x13, 0x03, 0x03, 0x00,
	0x63, 0x98, 0x62, 0x06,
	0x93, 0x03, 0x50, 0x00,
	0x13, 0x0e, 0xa0, 0x00,
	0xbb, 0x82, 0xc3, 0x41,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0xb3, 0xff,
	0x63, 0x9c, 0x62, 0x04,
	0x93, 0x03, 0x10, 0x00,
	0x13, 0x0e, 0xf0, 0x01,
	0xbb, 0x92, 0xc3, 0x01,
	0x37, 0x03, 0x00, 0x80,
	0x13, 0x03, 0x03, 0x00,
	0x63, 0x90, 0x62, 0x04,
	0xb7, 0x03, 0x00, 0x00,
	0x93, 0x83, 0xf3, 0xff,
	0x13, 0x0e, 0x10, 0x00,
	0xbb, 0xd2, 0xc3, 0x01,
	0x37, 0x03, 0x00, 0x80,
	0x13, 0x03, 0xf3, 0xff,
	0x13, 0x13, 0x03, 0x02,
	0x13, 0x53, 0x03, 0x02,
	0x63, 0x9e, 0x62, 0x00,
	0xbb, 0xd2, 0xc3, 0x41,
	0x37, 0x03, 0x00, 0x00,
	0x13, 0x03, 0xf3, 0xff,
	0x63, 0x96, 0x62, 0x00,
	0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00,
	0x13, 0x05, 0x10, 0x00,
	0x73, 0x00, 0x00, 0x00,
}

// MinimalElfProgram is a minimal valid ELF64 RISC-V binary for testing.
// It has one PT_LOAD segment containing exactly one .text section at
// sectionAddr with sectionData bytes, and an entry point of entryPoint.
//
// Layout:
//
//	0        64   ELF header
//	64       56   PT_LOAD program header
//	120      N    .text bytes  (N = len(sectionData))
//	120+N    17   .shstrtab: "\x00.text\x00.shstrtab\x00"
//	(aligned) …   padding to 8-byte boundary
//	X        64   NULL section header
//	X+64     64   .text section header
//	X+128    64   .shstrtab section header
var MinimalElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, ValidSectionData)

// ExitZeroElfProgram is a minimal valid ELF64 RISC-V binary that exits with
// code 0 through the guest syscall interface.
var ExitZeroElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, ExitZeroSectionData)

// MemoryRoundTripElfProgram is a minimal valid ELF64 RISC-V binary that
// stores and loads a scratch word before exiting through the guest syscall
// interface, exercising the S-type/I-type memory path that
// ExitZeroElfProgram never touches.
var MemoryRoundTripElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, MemoryRoundTripSectionData)

// ArithmeticElfProgram is a minimal valid ELF64 RISC-V binary that exercises
// LUI, JAL, JALR, an R-type base op, and an M-extension op before exiting
// through the guest syscall interface.
var ArithmeticElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, ArithmeticSectionData)

// ExitOneElfProgram is a minimal valid ELF64 RISC-V binary that exits with a
// nonzero code through the guest syscall interface, which main.zkc's
// process_syscall must reject rather than silently accept.
var ExitOneElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, ExitOneSectionData)

// BranchesElfProgram is a minimal valid ELF64 RISC-V binary that exercises
// all six B-type variants (BEQ, BNE, BLT, BGE, BLTU, BGEU), both taken and
// not-taken, before exiting through the guest syscall interface.
var BranchesElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, BranchesSectionData)

// LoadStoreWidthsElfProgram is a minimal valid ELF64 RISC-V binary that
// exercises SB/LB, SH/LH, and SD/LD (with sign-extension checks) before
// exiting through the guest syscall interface.
var LoadStoreWidthsElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, LoadStoreWidthsSectionData)

// Poseidon2ElfProgram is a minimal valid ELF64 RISC-V binary that invokes
// the R_POSEIDON2 precompile over an all-zero input block before exiting
// through the guest syscall interface.
var Poseidon2ElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, Poseidon2SectionData)

// KeccakElfProgram is a minimal valid ELF64 RISC-V binary that invokes the
// R_KECCAK precompile over an empty message before exiting through the
// guest syscall interface.
var KeccakElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, KeccakSectionData)

// WriteOutputElfProgram is a minimal valid ELF64 RISC-V binary that invokes
// the R_WRITE_OUTPUT precompile to copy 3 known bytes into the public
// guest_output buffer before exiting through the guest syscall interface.
var WriteOutputElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, WriteOutputSectionData)

// ImmediateALUElfProgram is a minimal valid ELF64 RISC-V binary that
// exercises SLTI, SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI, ADDIW, SLLIW,
// SRLIW, and SRAIW before exiting through the guest syscall interface.
var ImmediateALUElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, ImmediateALUSectionData)

// WordWidthElfProgram is a minimal valid ELF64 RISC-V binary that
// exercises ADDW, SUBW, SLLW, SRLW, and SRAW before exiting through the
// guest syscall interface.
var WordWidthElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, WordWidthSectionData)

// Make builds a minimal valid ELF64 RISC-V binary for testing.
// It has one PT_LOAD segment containing exactly one .text section at
// sectionAddr with sectionData bytes, and an entry point of entryPoint.
//
// Layout:
//
//	0        64   ELF header
//	64       56   PT_LOAD program header
//	120      N    .text bytes  (N = len(sectionData))
//	120+N    17   .shstrtab: "\x00.text\x00.shstrtab\x00"
//	(aligned) …   padding to 8-byte boundary
//	X        64   NULL section header
//	X+64     64   .text section header
//	X+128    64   .shstrtab section header
func Make(entryPoint, sectionAddr uint64, sectionData []byte) []byte {

	const (
		ehdrSize = 64
		phdrSize = 56
		shdrSize = 64
	)

	// section string table: index 1 = ".text", index 7 = ".shstrtab"
	shstrtab := []byte("\x00.text\x00.shstrtab\x00") // 17 bytes

	textOff := uint64(ehdrSize + phdrSize) // = 120
	shstrOff := textOff + uint64(len(sectionData))
	rawEnd := shstrOff + uint64(len(shstrtab))
	shOff := (rawEnd + 7) &^ 7 // align to 8 bytes

	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	w := func(v any) {
		if err := binary.Write(buf, le, v); err != nil {
			// This should never happen, since we're writing to a bytes.Buffer.
			panic(err)
		}
	}

	// ELF header (64 bytes)
	// e_ident (magic + class/data/version/OS/ABI)
	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	w(uint16(2))        // e_type:      ET_EXEC
	w(uint16(243))      // e_machine:   EM_RISCV
	w(uint32(1))        // e_version:   EV_CURRENT
	w(entryPoint)       // e_entry
	w(uint64(ehdrSize)) // e_phoff:     program headers start right after Ehdr
	w(shOff)            // e_shoff:     section headers after data
	w(uint32(0))        // e_flags
	w(uint16(ehdrSize)) // e_ehsize
	w(uint16(phdrSize)) // e_phentsize
	w(uint16(1))        // e_phnum
	w(uint16(shdrSize)) // e_shentsize
	w(uint16(3))        // e_shnum:     NULL + .text + .shstrtab
	w(uint16(2))        // e_shstrndx:  .shstrtab is section 2

	// PT_LOAD program header (56 bytes)
	w(uint32(1))                // p_type:   PT_LOAD
	w(uint32(5))                // p_flags:  PF_R | PF_X
	w(textOff)                  // p_offset: file offset of .text
	w(sectionAddr)              // p_vaddr
	w(sectionAddr)              // p_paddr
	w(uint64(len(sectionData))) // p_filesz
	w(uint64(len(sectionData))) // p_memsz
	w(uint64(0x1000))           // p_align

	// section data
	buf.Write(sectionData) // .text
	buf.Write(shstrtab)    // .shstrtab

	// padding to shOff
	for uint64(buf.Len()) < shOff {
		buf.WriteByte(0)
	}

	// NULL section header (index 0)
	for range shdrSize {
		buf.WriteByte(0)
	}

	// .text section header (index 1)
	w(uint32(1))                // sh_name:      ".text" at shstrtab[1]
	w(uint32(1))                // sh_type:      SHT_PROGBITS
	w(uint64(6))                // sh_flags:     SHF_ALLOC | SHF_EXECINSTR
	w(sectionAddr)              // sh_addr
	w(textOff)                  // sh_offset:    file offset
	w(uint64(len(sectionData))) // sh_size
	w(uint32(0))                // sh_link
	w(uint32(0))                // sh_info
	w(uint64(4))                // sh_addralign
	w(uint64(0))                // sh_entsize

	// .shstrtab section header (index 2)
	w(uint32(7))             // sh_name:      ".shstrtab" at shstrtab[7]
	w(uint32(3))             // sh_type:      SHT_STRTAB
	w(uint64(0))             // sh_flags
	w(uint64(0))             // sh_addr
	w(shstrOff)              // sh_offset
	w(uint64(len(shstrtab))) // sh_size
	w(uint32(0))             // sh_link
	w(uint32(0))             // sh_info
	w(uint64(1))             // sh_addralign
	w(uint64(0))             // sh_entsize

	return buf.Bytes()
}

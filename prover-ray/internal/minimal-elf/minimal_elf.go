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

// AllInOneSectionData is a tiny valid RISC-V program that exercises the full
// RV64I + M-extension + custom-precompile surface used by the RISC-V
// arithmetization in a single guest: memory round-trip (SW/LW),
// LUI/JAL/JALR/ADD/MUL, all six B-type branch variants (taken and
// not-taken), SB/LB/SH/LH/SD/LD (with sign extension), the R_POSEIDON2 and
// R_KECCAK precompiles, R_WRITE_OUTPUT, every I-type ALU immediate (SLTI,
// SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI) plus the RV64 word-width I-type
// variants (ADDIW, SLLIW, SRLIW, SRAIW), and the RV64 R-type word-width
// variants (ADDW, SUBW, SLLW, SRLW, SRAW). It halts through the Linux-style
// exit syscall path used by the arithmetization, exiting with code 1 if any
// check disagrees with its independently-computed expected value. s0 is set
// once via auipc at the top and held as a fixed base for every
// scratch-memory offset below (each check would otherwise re-auipc at its
// own, different program-counter position and address a different section
// of memory than intended); scratch regions (768, 800, 808, 816, 896, 960,
// 1088, 1216 bytes past that base) are laid out with enough spacing to stay
// non-overlapping and clear of the ~624-byte .text, while remaining within
// the 12-bit signed immediate range (max 2047) that S-type and I-type
// instructions can address in one step:
//
//	addi a7, x0, 93            ; syscall number, set once
//	auipc s0, 0                ; s0 = fixed base for every scratch offset below
//
//	; ---- memory round trip: store/load word at base+768 ----
//	addi t1, x0, 42
//	sw   t1, 768(s0)
//	lw   t2, 768(s0)
//	bne  t1, t2, fail
//
//	; ---- arithmetic: (t0+t1)*3 via subroutine, compare vs LUI-built const ----
//	lui  t0, 1
//	addi t1, x0, 5
//	jal  ra, add_mul
//	bne  t3, t4, fail
//
//	; ---- branches: all six B-type variants, taken and not-taken ----
//	addi t0, x0, 3
//	addi t1, x0, -1
//	addi t2, x0, 3
//	beq  t0, t2, br_ok1 / jal x0, fail
//	br_ok1: bne  t0, t1, br_ok2 / jal x0, fail
//	br_ok2: blt  t1, t0, br_ok3 / jal x0, fail
//	br_ok3: bge  t0, t1, br_ok4 / jal x0, fail
//	br_ok4: bltu t0, t1, br_ok5 / jal x0, fail
//	br_ok5: bgeu t1, t0, br_ok6 / jal x0, fail
//	br_ok6: beq t0,t1,fail / bne t0,t2,fail / blt t0,t1,fail
//	        bge t1,t0,fail / bltu t1,t0,fail / bgeu t0,t1,fail
//
//	; ---- load/store widths: SB/LB, SH/LH, SD/LD at base+800/808/816 ----
//	addi t1, x0, 171
//	sb   t1, 800(s0)
//	lb   t2, 800(s0)          ; t2 = sign_extend(0xAB) = -85
//	addi t3, x0, -85
//	bne  t2, t3, fail
//	addi t1, x0, -1
//	sh   t1, 808(s0)
//	lh   t2, 808(s0)          ; t2 = sign_extend(0xFFFF) = -1
//	addi t3, x0, -1
//	bne  t2, t3, fail
//	addi t1, x0, 1000
//	sd   t1, 816(s0)
//	ld   t2, 816(s0)          ; exact round trip, no extension
//	bne  t2, t1, fail
//
//	; ---- poseidon2: permute all-zero block at base+896, output at base+960 ----
//	addi t1, s0, 960          ; output_offset
//	addi t2, s0, 896          ; input_offset (still zeroed)
//	.insn r CUSTOM_1, 0b001, 0, t1, t2, x0   ; poseidon2(t2, t1)
//	lw   t3, 0(t1)
//	lui  t4, 0x35596
//	addi t4, t4, 407          ; expected first output word
//	bne  t3, t4, fail
//
//	; ---- keccak: empty message, output at base+1088 ----
//	addi t1, s0, 1088         ; output_offset
//	.insn r CUSTOM_1, 0b000, 0, t1, s0, x0  ; keccak(s0, x0, t1); msg_length=0
//	lw   t3, 0(t1)
//	lui  t4, 0x146d
//	addi t4, t4, 709          ; expected first digest word
//	bne  t3, t4, fail
//
//	; ---- write_output: write "ABC" to guest_output, scratch at base+1216 ----
//	addi t1, s0, 1216
//	addi t2, x0, 0x41 / sb t2, 0(t1)
//	addi t2, x0, 0x42 / sb t2, 1(t1)
//	addi t2, x0, 0x43 / sb t2, 2(t1)
//	addi t3, x0, 3            ; size = 3
//	.insn r CUSTOM_1, 0b010, 0, x0, t1, t3  ; write_output(t1, t3)
//
//	; ---- immediate ALU: SLTI, SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI, ----
//	; ---- plus RV64 ADDIW, SLLIW, SRLIW, SRAIW. t2=6 is the shared operand
//	; ---- for the base checks; SRAI uses t2=-8 to exercise sign-preserving
//	; ---- shifts; the *W checks reload t2 with 32-bit boundary values
//	; ---- (0x7FFFFFFF, 0xFFFFFFFF) to exercise word-width wraparound and
//	; ---- sign extension. Because LUI sign-extends its 32-bit result to 64
//	; ---- bits, loading a value whose bit 31 is 0 but whose upper 20 bits
//	; ---- (as loaded via LUI) form a pattern with bit 31 set (e.g.
//	; ---- 0x7FFFFFFF, via LUI 0x80000 + ADDI -1) requires an extra SLLI 32 /
//	; ---- SRLI 32 pair to zero the sign-extended upper 32 bits afterward.
//	; ---- SRLIW's comparison constant needs that same masking even though
//	; ---- t2 isn't reloaded: SRLIW zero-extends its 32-bit result, unlike
//	; ---- ADDIW/SLLIW/SRAIW which sign-extend, so the expected constant
//	; ---- (built via LUI+ADDI, which does sign-extend) must be masked down
//	; ---- to match:
//	addi  t2, x0, 6
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
//	srliw t0, t2, 1            ; expect 0x7FFFFFFF (logical shift within 32 bits, zero-extended)
//	bne   t0, expected, fail
//	sraiw t0, t2, 1            ; expect -1 (arithmetic shift of all-ones is still all-ones)
//	bne   t0, -1, fail
//
//	; ---- word width: ADDW, SUBW, SLLW, SRLW, SRAW, each operating on the
//	; ---- low 32 bits of its operands and sign-extending the 32-bit result
//	; ---- to 64 bits (except SRLW, which zero-extends, same as SRLIW
//	; ---- above). Operands are chosen to force 32-bit wraparound (ADDW,
//	; ---- SLLW) and sign-extension of a negative 32-bit result (SUBW, SRAW):
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
//	srlw t0, t2, t3            ; expect 0x7FFFFFFF (logical shift within 32 bits, zero-extended)
//	bne  t0, expected, fail
//	sraw t0, t2, t3            ; expect -1 (arithmetic shift of all-ones is still all-ones)
//	bne  t0, -1, fail
//
//	addi a0, x0, 0
//	ecall
//
// fail:
//
//	addi a0, x0, 1
//	ecall
//
// add_mul:
//
//	add  t2, t0, t1
//	addi t3, x0, 3
//	mul  t3, t2, t3
//	lui  t4, 3
//	addi t4, t4, 15
//	jalr x0, ra, 0
var AllInOneSectionData = []byte{
	0x93, 0x08, 0xd0, 0x05, 0x17, 0x04, 0x00, 0x00, 0x13, 0x03, 0xa0, 0x02,
	0x23, 0x20, 0x64, 0x30, 0x83, 0x23, 0x04, 0x30, 0x63, 0x1e, 0x73, 0x22,
	0xb7, 0x12, 0x00, 0x00, 0x13, 0x03, 0x50, 0x00, 0xef, 0x00, 0x80, 0x23,
	0x63, 0x16, 0xde, 0x23, 0x93, 0x02, 0x30, 0x00, 0x13, 0x03, 0xf0, 0xff,
	0x93, 0x03, 0x30, 0x00, 0x63, 0x84, 0x72, 0x00, 0x6f, 0x00, 0x80, 0x21,
	0x63, 0x94, 0x62, 0x00, 0x6f, 0x00, 0x00, 0x21, 0x63, 0x44, 0x53, 0x00,
	0x6f, 0x00, 0x80, 0x20, 0x63, 0xd4, 0x62, 0x00, 0x6f, 0x00, 0x00, 0x20,
	0x63, 0xe4, 0x62, 0x00, 0x6f, 0x00, 0x80, 0x1f, 0x63, 0x74, 0x53, 0x00,
	0x6f, 0x00, 0x00, 0x1f, 0x63, 0x86, 0x62, 0x1e, 0x63, 0x94, 0x72, 0x1e,
	0x63, 0xc2, 0x62, 0x1e, 0x63, 0x50, 0x53, 0x1e, 0x63, 0x6e, 0x53, 0x1c,
	0x63, 0xfc, 0x62, 0x1c, 0x13, 0x03, 0xb0, 0x0a, 0x23, 0x00, 0x64, 0x32,
	0x83, 0x03, 0x04, 0x32, 0x13, 0x0e, 0xb0, 0xfa, 0x63, 0x92, 0xc3, 0x1d,
	0x13, 0x03, 0xf0, 0xff, 0x23, 0x14, 0x64, 0x32, 0x83, 0x13, 0x84, 0x32,
	0x13, 0x0e, 0xf0, 0xff, 0x63, 0x98, 0xc3, 0x1b, 0x13, 0x03, 0x80, 0x3e,
	0x23, 0x38, 0x64, 0x32, 0x83, 0x33, 0x04, 0x33, 0x63, 0x90, 0x63, 0x1a,
	0x13, 0x03, 0x04, 0x3c, 0x93, 0x03, 0x04, 0x38, 0x2b, 0x93, 0x03, 0x00,
	0x03, 0x2e, 0x03, 0x00, 0xb7, 0x6e, 0x59, 0x35, 0x93, 0x8e, 0x7e, 0x19,
	0x63, 0x12, 0xde, 0x19, 0x13, 0x03, 0x04, 0x44, 0x2b, 0x03, 0x04, 0x00,
	0x03, 0x2e, 0x03, 0x00, 0xb7, 0xde, 0x46, 0x01, 0x93, 0x8e, 0x5e, 0x2c,
	0x63, 0x16, 0xde, 0x17, 0x13, 0x03, 0x04, 0x4c, 0x93, 0x03, 0x10, 0x04,
	0x23, 0x00, 0x73, 0x00, 0x93, 0x03, 0x20, 0x04, 0xa3, 0x00, 0x73, 0x00,
	0x93, 0x03, 0x30, 0x04, 0x23, 0x01, 0x73, 0x00, 0x13, 0x0e, 0x30, 0x00,
	0x2b, 0x20, 0xc3, 0x01, 0x93, 0x03, 0x60, 0x00, 0x93, 0xa2, 0xa3, 0x00,
	0x13, 0x0f, 0x10, 0x00, 0x63, 0x9c, 0xe2, 0x13, 0x93, 0xb2, 0xa3, 0x00,
	0x13, 0x0f, 0x10, 0x00, 0x63, 0x96, 0xe2, 0x13, 0x93, 0xc2, 0x33, 0x00,
	0x13, 0x0f, 0x50, 0x00, 0x63, 0x90, 0xe2, 0x13, 0x93, 0xe2, 0x13, 0x00,
	0x13, 0x0f, 0x70, 0x00, 0x63, 0x9a, 0xe2, 0x11, 0x93, 0xf2, 0x23, 0x00,
	0x13, 0x0f, 0x20, 0x00, 0x63, 0x94, 0xe2, 0x11, 0x93, 0x92, 0x23, 0x00,
	0x13, 0x0f, 0x80, 0x01, 0x63, 0x9e, 0xe2, 0x0f, 0x93, 0xd2, 0x13, 0x00,
	0x13, 0x0f, 0x30, 0x00, 0x63, 0x98, 0xe2, 0x0f, 0x93, 0x03, 0x80, 0xff,
	0x93, 0xd2, 0x13, 0x40, 0x13, 0x0f, 0xc0, 0xff, 0x63, 0x90, 0xe2, 0x0f,
	0xb7, 0x03, 0x00, 0x80, 0x93, 0x83, 0xf3, 0xff, 0x93, 0x93, 0x03, 0x02,
	0x93, 0xd3, 0x03, 0x02, 0x9b, 0x82, 0xa3, 0x00, 0xb7, 0x0e, 0x00, 0x80,
	0x93, 0x8e, 0x9e, 0x00, 0x63, 0x90, 0xd2, 0x0d, 0x93, 0x03, 0x60, 0x00,
	0x9b, 0x92, 0x23, 0x00, 0x13, 0x0f, 0x80, 0x01, 0x63, 0x98, 0xe2, 0x0b,
	0xb7, 0x03, 0x00, 0x00, 0x93, 0x83, 0xf3, 0xff, 0x9b, 0xd2, 0x13, 0x00,
	0xb7, 0x0e, 0x00, 0x80, 0x93, 0x8e, 0xfe, 0xff, 0x93, 0x9e, 0x0e, 0x02,
	0x93, 0xde, 0x0e, 0x02, 0x63, 0x98, 0xd2, 0x09, 0x9b, 0xd2, 0x13, 0x40,
	0x13, 0x0f, 0xf0, 0xff, 0x63, 0x92, 0xe2, 0x09, 0xb7, 0x03, 0x00, 0x80,
	0x93, 0x83, 0xf3, 0xff, 0x93, 0x93, 0x03, 0x02, 0x93, 0xd3, 0x03, 0x02,
	0x13, 0x0e, 0x10, 0x00, 0xbb, 0x82, 0xc3, 0x01, 0xb7, 0x0e, 0x00, 0x80,
	0x63, 0x92, 0xd2, 0x07, 0x93, 0x03, 0x50, 0x00, 0x13, 0x0e, 0xa0, 0x00,
	0xbb, 0x82, 0xc3, 0x41, 0x13, 0x0f, 0xb0, 0xff, 0x63, 0x98, 0xe2, 0x05,
	0x93, 0x03, 0x10, 0x00, 0x13, 0x0e, 0xf0, 0x01, 0xbb, 0x92, 0xc3, 0x01,
	0xb7, 0x0e, 0x00, 0x80, 0x63, 0x9e, 0xd2, 0x03, 0xb7, 0x03, 0x00, 0x00,
	0x93, 0x83, 0xf3, 0xff, 0x13, 0x0e, 0x10, 0x00, 0xbb, 0xd2, 0xc3, 0x01,
	0xb7, 0x0e, 0x00, 0x80, 0x93, 0x8e, 0xfe, 0xff, 0x93, 0x9e, 0x0e, 0x02,
	0x93, 0xde, 0x0e, 0x02, 0x63, 0x9c, 0xd2, 0x01, 0xbb, 0xd2, 0xc3, 0x41,
	0x13, 0x0f, 0xf0, 0xff, 0x63, 0x96, 0xe2, 0x01, 0x13, 0x05, 0x00, 0x00,
	0x73, 0x00, 0x00, 0x00, 0x13, 0x05, 0x10, 0x00, 0x73, 0x00, 0x00, 0x00,
	0xb3, 0x83, 0x62, 0x00, 0x13, 0x0e, 0x30, 0x00, 0x33, 0x8e, 0xc3, 0x03,
	0xb7, 0x3e, 0x00, 0x00, 0x93, 0x8e, 0xfe, 0x00, 0x67, 0x80, 0x00, 0x00,
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

// AllInOneElfProgram is a minimal valid ELF64 RISC-V binary that exercises
// the full RV64I + M-extension + custom-precompile surface used by the
// RISC-V arithmetization in a single guest before exiting through the guest
// syscall interface.
var AllInOneElfProgram = Make(DefaultEntryPoint, DefaultSectionAddr, AllInOneSectionData)

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

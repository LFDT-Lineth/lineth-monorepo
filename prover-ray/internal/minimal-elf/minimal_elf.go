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

package minimal_elf

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

// MinimalElfPrograms is a minimal valid ELF64 RISC-V binary for testing.
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
	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}) // e_ident (magic + class/data/version/OS/ABI)

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

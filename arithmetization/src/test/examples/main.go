package main

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	RISCV_PROGRAM        = "riscV_program"
	IN_BYTES             = "in_bytes"
	RISCV_PROGRAM_OFFSET = "riscV_program_offset"
	IN_BYTES_OFFSET      = "in_bytes_offset"
	ENTRY_POINT          = "entry_point"
	RISCV_PROGRAM_LENGTH = "riscV_program_length"
	IN_BYTES_LENGTH      = "in_bytes_length"
)

// The purpose of this program is simply to generate a suitable ZkC json input
// file for a given RISC-V binary program.
func main() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "usage: go run main.go <elfFile> <inBytes> <programOffset> <inputsOffset> <entryPoint>")
		os.Exit(1)
	}

	elfFile, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening ELF file: %v\n", err)
		os.Exit(1)
	}
	defer elfFile.Close()
	// inBytes
	var inBytes []byte
	inBytesString := os.Args[2]
	if strings.HasPrefix(inBytesString, "0x") || strings.HasPrefix(inBytesString, "0X") {
		inBytes, err = hex.DecodeString(inBytesString[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding hex input bytes: %v\n", err)
			os.Exit(1)
		}
	} else {
		inBytes = []byte(inBytesString)
	}
	// offsets
	var programOffset uint64
	var inputsOffset uint64
	var entryPoint uint64
	programOffset, err = strconv.ParseUint(os.Args[3], 0, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading program offset: %v\n", err)
		os.Exit(1)
	}
	inputsOffset, err = strconv.ParseUint(os.Args[4], 0, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading input bytes offset: %v\n", err)
		os.Exit(1)
	}
	entryPoint, err = strconv.ParseUint(os.Args[5], 0, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading entry point: %v\n", err)
		os.Exit(1)
	}
	// extract loadable program segments
	var program = extractProgramBytes(elfFile.Progs, programOffset)
	switch writeSections := os.Getenv("ELF2JSON_WRITE_SECTIONS"); writeSections {
	case "", "false":
	case "true":
		sectionsFile, err := os.Create(strings.TrimSuffix(os.Args[1], ".elf") + ".sections")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating ELF sections file: %v\n", err)
			os.Exit(1)
		}
		for _, s := range elfFile.Sections {
			if s.Name != "" && s.Size > 0 && s.Flags&elf.SHF_ALLOC != 0 {
				fmt.Fprintln(sectionsFile, s.Name)
			}
		}
		if err := sectionsFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error writing ELF sections file: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "ELF2JSON_WRITE_SECTIONS must be true or false, got %q\n", writeSections)
		os.Exit(1)
	}
	printJson(program, inBytes, programOffset, inputsOffset, entryPoint)
}

// Extract loadable ELF segments and assemble them into a contiguous byte slice
// starting at programOffset. The JSON format has a single riscV_program blob for
// now; supporting very sparse ELFs properly would require multiple memory blobs.
//
// Our own tests contain .text, .rodata, .data and .bss sections.
// ACT4 tests contain .text.init, .text.rvtest, .text.rvmodel, .data,
// and .tohost sections. We do not filter by section names here:
// PT_LOAD segments are what the ELF loader actually maps into memory.
func extractProgramBytes(progs []*elf.Prog, programOffset uint64) []byte {
	var maxAddr uint64 = 0
	for _, p := range progs {
		if p.Type != elf.PT_LOAD {
			continue
		}
		if p.Filesz > p.Memsz {
			panic(fmt.Sprintf("loadable segment at %#x has file size larger than memory size", p.Vaddr))
		}
		if p.Memsz == 0 {
			continue
		}
		if p.Vaddr < programOffset {
			panic(fmt.Sprintf("loadable segment starts before program offset: segment=%#x programOffset=%#x", p.Vaddr, programOffset))
		}
		end := p.Vaddr + p.Memsz
		if end < p.Vaddr {
			panic(fmt.Sprintf("loadable segment address overflow at %#x", p.Vaddr))
		}
		if end > maxAddr {
			maxAddr = end
		}
	}

	if maxAddr == 0 {
		panic("no loadable program segments found.")
	}

	buf := make([]byte, maxAddr-programOffset)

	for _, p := range progs {
		if p.Type != elf.PT_LOAD || p.Filesz == 0 {
			continue
		}
		data := make([]byte, p.Filesz)
		n, err := p.ReadAt(data, 0)
		if err != nil && err != io.EOF {
			panic(fmt.Sprintf("error reading loadable segment at %#x: %v", p.Vaddr, err))
		}
		if uint64(n) != p.Filesz {
			panic(fmt.Sprintf("short read for loadable segment at %#x: got %d bytes, expected %d", p.Vaddr, n, p.Filesz))
		}
		offset := p.Vaddr - programOffset
		copy(buf[offset:], data)
	}

	// If needed pad buffer to multiple of 4 bytes (add at most 3 zero bytes)
	for len(buf)%4 != 0 {
		buf = append(buf, 0)
	}

	return buf
}

func printJson(program, inBytes []byte, programOffset, inputsOffset, entryPoint uint64) {
	var (
		programString       = hex.EncodeToString(program)
		inBytesString       = hex.EncodeToString(inBytes)
		programOffsetString = fmt.Sprintf("%016x", programOffset)
		inputsOffsetString  = fmt.Sprintf("%016x", inputsOffset)
		entryPointString    = fmt.Sprintf("%016x", entryPoint)
		programLenString    = fmt.Sprintf("%016x", len(program))
		inputLenString      = fmt.Sprintf("%016x", len(inBytes))
	)

	if len(program)%4 != 0 {
		panic("program length not multiple of 4")
	}

	fmt.Println("{")
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM, programString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", IN_BYTES, inBytesString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM_OFFSET, programOffsetString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", IN_BYTES_OFFSET, inputsOffsetString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", ENTRY_POINT, entryPointString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM_LENGTH, programLenString)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", IN_BYTES_LENGTH, inputLenString)
	fmt.Println("}")
}

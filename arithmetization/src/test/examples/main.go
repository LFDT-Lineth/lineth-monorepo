package main

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
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
	// extract program sections
	var program = extractProgramBytes(elfFile.Sections, programOffset)
	printJson(program, inBytes, programOffset, inputsOffset, entryPoint)
}

// Extract .text, .rodata and .data sections and assemble them into a
// contiguous byte slice starting at programOffset, being writte in
// the riscV_program field of the output JSON. The .bss section is
// implicitly zeroed out in the output, so we do not need to explicitly include it.
func extractProgramBytes(sections []*elf.Section, programOffset uint64) []byte {
	programBytesSections := map[string]bool{
		".text":   true,
		".rodata": true,
		".data":   true,
	}

	// First pass: find the total size needed
	var maxAddr uint64 = 0
	for _, s := range sections {
		if programBytesSections[s.Name] {
			end := s.Addr + s.Size
			if end > maxAddr {
				maxAddr = end
			}
		}
	}

	if maxAddr == 0 {
		panic("no program sections found.")
	}

	// Allocate zeroed buffer covering the full program image (.bss is implicitly zeroed)
	buf := make([]byte, maxAddr-programOffset)

	// Second pass: copy each section into the correct offset in the buffer
	for _, s := range sections {
		if programBytesSections[s.Name] {
			data, err := s.Data()
			if err != nil {
				panic(fmt.Sprintf("error reading section %s: %v", s.Name, err))
			}
			offset := s.Addr - programOffset
			copy(buf[offset:], data)
		}
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

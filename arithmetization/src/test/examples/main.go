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
	// extract text section
	var text = extractTextBytes(elfFile.Sections)
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
	printJson(text, inBytes, programOffset, inputsOffset, entryPoint)
}

// Extract the bytes of the text section.
func extractTextBytes(sections []*elf.Section) []byte {
	for _, s := range sections {
		if s.Name == ".text" {
			data, err := s.Data()
			// Sanity check
			if err != nil {
				panic(err.Error())
			}
			//
			return data
		}
	}
	//
	panic("no text section found!")
}

func printJson(text, inBytes []byte, programOffset, inputsOffset, entryPoint uint64) {
	// Convert text bytes into a hex string
	var (
		textString          = hex.EncodeToString(text)
		inBytesString       = hex.EncodeToString(inBytes)
		programOffsetString = fmt.Sprintf("%016x", programOffset)
		inputsOffsetString  = fmt.Sprintf("%016x", inputsOffset)
		entryPointString    = fmt.Sprintf("%016x", entryPoint)
		textLenString       = fmt.Sprintf("%016x", len(text))
		inputLenString      = fmt.Sprintf("%016x", len(inBytes))
	)
	//
	if len(text)%4 != 0 {
		panic("text section length not multiple of 4")
	}
	//
	fmt.Println("{")
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM, textString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", IN_BYTES, inBytesString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM_OFFSET, programOffsetString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", IN_BYTES_OFFSET, inputsOffsetString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", ENTRY_POINT, entryPointString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM_LENGTH, textLenString)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", IN_BYTES_LENGTH, inputLenString)
	fmt.Println("}")
}

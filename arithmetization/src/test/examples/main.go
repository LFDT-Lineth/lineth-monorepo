package main

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	RISCV_PROGRAM     = "riscV_program"
	RISCV_PROGRAM_LEN = "riscV_program_length"
	RISCV_INBYTES     = "in_bytes"
	RISCV_INBYTES_LEN = "in_bytes_length"
)

// The purpose of this program is simply to generate a suitable ZkC json input
// file for a given RISC-V binary program.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: zkc-gen <elf-file>")
		os.Exit(1)
	}

	f, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	// extract text section
	var text = extractTextBytes(f.Sections)
	// inputs left empty (for now)
	var inputs []byte
	//
	printJson(text, inputs)
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

func printJson(text, input []byte) {
	// Convert text bytes into a hex string
	var (
		textString     = hex.EncodeToString(text)
		textLenString  = fmt.Sprintf("%016x", len(text))
		inputsString   = hex.EncodeToString(input)
		inputLenString = fmt.Sprintf("%016x", len(input))
	)
	//
	if len(text)%4 != 0 {
		panic("text section length not multiple of 4")
	}
	//
	fmt.Println("{")
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM, textString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_PROGRAM_LEN, textLenString)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", RISCV_INBYTES, inputsString)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", RISCV_INBYTES_LEN, inputLenString)
	fmt.Println("}")
}

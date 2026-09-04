package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

type inputBytes struct {
	data    []byte
	options []elfmapping.DataOption
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("elf_to_json", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	help := flags.Bool("help", false, "show command usage")
	shortHelp := flags.Bool("h", false, "show command usage")
	flags.Usage = func() {
		_, _ = fmt.Fprint(errorOutput, `Convert a RISC-V ELF and guest input into R5 ZkC JSON inputs.

Usage:
  elf_to_json [options] <elf-file> <input> [input-offset]

Arguments:
  elf-file     Statically linked RISC-V ELF executable
  input        Raw text, 0x hex, @file.hex, @file.ssz, or @file.bin
  input-offset Guest memory address, in decimal or 0x notation
               (default: 0x08800000)

Options:
`)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *help || *shortHelp {
		flags.Usage()
		return nil
	}
	if flags.NArg() < 2 || flags.NArg() > 3 {
		flags.Usage()
		return fmt.Errorf("expected 2 or 3 positional arguments, got %d", flags.NArg())
	}
	args = flags.Args()
	elfBytes, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("error opening ELF file: %w", err)
	}
	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		return err
	}
	input, err := parseInput(args[1])
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}
	inputOffset, err := parseInputOffset(args)
	if err != nil {
		return fmt.Errorf("error reading input bytes offset: %w", err)
	}

	additional, err := inputBlobs(inputOffset, input)
	if err != nil {
		return err
	}
	decodeOptions, err := decodeOptionsFromEnv()
	if err != nil {
		return err
	}
	decoded, err := predecoding.Predecode(program, decodeOptions...)
	if err != nil {
		return err
	}

	mappingOptions, sectionsFile, err := mappingOptionsFromEnv(args[0])
	if err != nil {
		return err
	}
	if sectionsFile != nil {
		defer func() {
			if sectionsFile != nil {
				_ = sectionsFile.Close()
			}
		}()
	}
	inputs, err := elfmapping.EncodeInputs(program, additional, mappingOptions...)
	if err != nil {
		return err
	}
	if sectionsFile != nil {
		if err := sectionsFile.Close(); err != nil {
			return fmt.Errorf("error writing ELF sections file: %w", err)
		}
		sectionsFile = nil
	}
	for name, data := range decoded.EncodeInputs() {
		inputs[name] = data
	}
	return writeJSON(output, inputs)
}

func parseInput(argument string) (inputBytes, error) {
	if path, isFile := strings.CutPrefix(argument, "@"); isFile {
		extension := filepath.Ext(path)
		if extension != ".hex" && extension != ".ssz" && extension != ".bin" {
			return inputBytes{}, fmt.Errorf("unsupported input file %q: expected .hex, .ssz, or .bin suffix", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return inputBytes{}, fmt.Errorf("reading input file %q: %w", path, err)
		}
		switch extension {
		case ".hex":
			fields := strings.Fields(string(data))
			if len(fields) != 1 {
				return inputBytes{}, fmt.Errorf("hex input file %q must contain one 0x-prefixed value, got %d fields", path, len(fields))
			}
			decoded, err := parseHexInput(fields[0])
			return inputBytes{data: decoded}, err
		case ".ssz":
			return inputBytes{
				data: data,
				options: []elfmapping.DataOption{
					elfmapping.WithName("ssz"),
					elfmapping.WithLengthPrefix(),
				},
			}, nil
		case ".bin":
			return inputBytes{data: data}, nil
		}
	}
	if strings.HasPrefix(argument, "0x") || strings.HasPrefix(argument, "0X") {
		data, err := parseHexInput(argument)
		return inputBytes{data: data}, err
	}
	return inputBytes{data: []byte(argument)}, nil
}

func parseHexInput(argument string) ([]byte, error) {
	if !strings.HasPrefix(argument, "0x") && !strings.HasPrefix(argument, "0X") {
		return nil, fmt.Errorf("expected 0x-prefixed input bytes, got %q", argument)
	}
	data, err := hex.DecodeString(argument[2:])
	if err != nil {
		return nil, fmt.Errorf("decoding hex input bytes: %w", err)
	}
	slices.Reverse(data)
	return data, nil
}

func inputBlobs(offset uint64, input inputBytes) ([]elfmapping.Blob, error) {
	return elfmapping.NewData(offset, input.data, input.options...)
}

func parseInputOffset(arguments []string) (uint64, error) {
	if len(arguments) == 2 {
		return elfmapping.DefaultInputOrigin, nil
	}
	return strconv.ParseUint(arguments[2], 0, 64)
}

func decodeOptionsFromEnv() ([]predecoding.Option, error) {
	value := os.Getenv("ELF2JSON_MAX_DECODED_RECORDS")
	if value == "" {
		return nil, nil
	}
	maximum, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ELF2JSON_MAX_DECODED_RECORDS %q: %w", value, err)
	}
	return []predecoding.Option{predecoding.WithMaxDecodedRecords(maximum)}, nil
}

func mappingOptionsFromEnv(elfPath string) ([]elfmapping.Option, *os.File, error) {
	includeExecutable, err := boolEnv("ELF2JSON_PREDECODING_PROOF")
	if err != nil {
		return nil, nil, err
	}
	writeSections, err := boolEnv("ELF2JSON_WRITE_SECTIONS")
	if err != nil {
		return nil, nil, err
	}
	var options []elfmapping.Option
	if includeExecutable {
		options = append(options, elfmapping.WithIncludeExecutable())
	}
	if !writeSections {
		return options, nil, nil
	}
	file, err := os.Create(strings.TrimSuffix(elfPath, ".elf") + ".sections")
	if err != nil {
		return nil, nil, fmt.Errorf("error creating ELF sections file: %w", err)
	}
	return append(options, elfmapping.WithSectionsWriter(file)), file, nil
}

func boolEnv(name string) (bool, error) {
	switch value := os.Getenv(name); value {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false, got %q", name, value)
	}
}

func writeJSON(output io.Writer, inputs map[string][]byte) error {
	entry := inputs[elfmapping.EntryPointAndBlobsCountInput]
	metadata := inputs[elfmapping.BlobsOffsetAndSizeInput]
	data := inputs[elfmapping.BlobsDataInput]
	if len(entry) != 16 || len(metadata)%16 != 0 {
		return fmt.Errorf("invalid encoded ELF input lengths")
	}
	metadataRecords := splitHexRecords(metadata, 16, "_")
	dataRecords, err := splitBlobData(data, metadata)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "{"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "\t%q: \"0x%s_%s\",\n", elfmapping.EntryPointAndBlobsCountInput, hex.EncodeToString(entry[:8]), hex.EncodeToString(entry[8:])); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "\t%q: \"0x%s\",\n", elfmapping.BlobsOffsetAndSizeInput, strings.Join(metadataRecords, "____")); err != nil {
		return err
	}
	if executable, ok := inputs[elfmapping.BlobsExecutableInput]; ok {
		if _, err := fmt.Fprintf(output, "\t%q: \"0x%s\",\n", elfmapping.BlobsExecutableInput, hex.EncodeToString(executable)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "\t%q: \"0x%s\",\n", elfmapping.BlobsDataInput, strings.Join(dataRecords, "____")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "\t%q: \"0x%s\",\n", predecoding.InstructionBaseInput, hex.EncodeToString(inputs[predecoding.InstructionBaseInput])); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "\t%q: \"0x%s\"\n", predecoding.DecodedInput, hex.EncodeToString(inputs[predecoding.DecodedInput])); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "}")
	return err
}

func splitHexRecords(data []byte, size int, fieldSeparator string) []string {
	records := make([]string, 0, len(data)/size)
	for len(data) != 0 {
		record := data[:size]
		if fieldSeparator == "_" {
			records = append(records, hex.EncodeToString(record[:8])+"_"+hex.EncodeToString(record[8:]))
		} else {
			records = append(records, hex.EncodeToString(record))
		}
		data = data[size:]
	}
	return records
}

func splitBlobData(data, metadata []byte) ([]string, error) {
	var records []string
	for len(metadata) != 0 {
		size := binary.BigEndian.Uint64(metadata[8:16])
		metadata = metadata[16:]
		if size > uint64(len(data)) {
			return nil, fmt.Errorf("blob metadata exceeds encoded data")
		}
		if size != 0 {
			records = append(records, hex.EncodeToString(data[:size]))
		}
		data = data[size:]
	}
	if len(data) != 0 {
		return nil, fmt.Errorf("encoded blob data exceeds metadata")
	}
	return records, nil
}

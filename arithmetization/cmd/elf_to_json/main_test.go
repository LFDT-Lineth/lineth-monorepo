package main

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

func TestRunHelp(t *testing.T) {
	for _, argument := range []string{"-h", "--help"} {
		t.Run(argument, func(t *testing.T) {
			var output, errorOutput bytes.Buffer
			if err := run([]string{argument}, &output, &errorOutput); err != nil {
				t.Fatalf("run(%s) error = %v", argument, err)
			}
			if output.Len() != 0 {
				t.Errorf("run(%s) stdout = %q, want empty", argument, output.String())
			}
			for _, text := range []string{
				"Convert a RISC-V ELF",
				"elf_to_json [options] <elf-file> <input> [input-offset]",
				"default: 0x08800000",
				"-help",
				"-h",
			} {
				if !strings.Contains(errorOutput.String(), text) {
					t.Errorf("run(%s) help omitted %q:\n%s", argument, text, errorOutput.String())
				}
			}
		})
	}
}

func TestRunMissingArgumentsShowsUsage(t *testing.T) {
	var output, errorOutput bytes.Buffer
	err := run(nil, &output, &errorOutput)
	if err == nil || !strings.Contains(err.Error(), "expected 2 or 3 positional arguments") {
		t.Fatalf("run(nil) error = %v, want positional argument error", err)
	}
	if !strings.Contains(errorOutput.String(), "Usage:") {
		t.Fatalf("run(nil) stderr omitted usage:\n%s", errorOutput.String())
	}
}

func TestParseHexInputReversesBytes(t *testing.T) {
	got, err := parseHexInput("0x0102ff")
	if err != nil {
		t.Fatalf("parseHexInput() error = %v", err)
	}
	if want := []byte{0xff, 0x02, 0x01}; !bytes.Equal(got, want) {
		t.Fatalf("parseHexInput() = %x, want %x", got, want)
	}
}

func TestParseInput(t *testing.T) {
	tempDir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return "@" + path
	}
	tests := []struct {
		name        string
		argument    string
		want        []byte
		wantOptions int
		wantError   string
	}{
		{name: "hex file", argument: write("input.hex", []byte("0x0102\n")), want: []byte{2, 1}},
		{name: "ssz file", argument: write("input.ssz", []byte{1, 2}), want: []byte{1, 2}, wantOptions: 2},
		{name: "bin file", argument: write("input.bin", []byte{3, 4}), want: []byte{3, 4}},
		{name: "unsupported file", argument: write("input.data", []byte{1}), wantError: "expected .hex, .ssz, or .bin"},
		{name: "inline hex", argument: "0x0102", want: []byte{2, 1}},
		{name: "inline binary", argument: "hello", want: []byte("hello")},
		{name: "non-at ssz is inline", argument: "input.ssz", want: []byte("input.ssz")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseInput(test.argument)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseInput() error = %v, want containing %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInput() error = %v", err)
			}
			if !bytes.Equal(got.data, test.want) || len(got.options) != test.wantOptions {
				t.Fatalf("parseInput() = {%x, %d options}, want {%x, %d options}", got.data, len(got.options), test.want, test.wantOptions)
			}
		})
	}
}

func TestInputBlobsUsesOptionalLengthPrefix(t *testing.T) {
	blobs, err := inputBlobs(0x1000, inputBytes{
		data: []byte{1, 2},
		options: []elfmapping.DataOption{
			elfmapping.WithName("ssz"),
			elfmapping.WithLengthPrefix(),
		},
	})
	if err != nil {
		t.Fatalf("inputBlobs() error = %v", err)
	}
	if len(blobs) != 2 || blobs[0].Name != "ssz_length" || blobs[1].Name != "ssz_payload" {
		t.Fatalf("inputBlobs() = %#v, want named length and payload blobs", blobs)
	}

	raw, err := inputBlobs(0x1000, inputBytes{data: []byte{1, 2}})
	if err != nil {
		t.Fatalf("inputBlobs(raw) error = %v", err)
	}
	if len(raw) != 1 || !bytes.Equal(raw[0].Data, []byte{1, 2}) {
		t.Fatalf("inputBlobs(raw) = %#v, want one unframed blob", raw)
	}
}

func TestParseInputOffset(t *testing.T) {
	got, err := parseInputOffset([]string{"guest.elf", "input"})
	if err != nil {
		t.Fatalf("parseInputOffset(default) error = %v", err)
	}
	if got != elfmapping.DefaultInputOrigin {
		t.Fatalf("parseInputOffset(default) = %#x, want %#x", got, elfmapping.DefaultInputOrigin)
	}
	got, err = parseInputOffset([]string{"guest.elf", "input", "0x1234"})
	if err != nil {
		t.Fatalf("parseInputOffset(explicit) error = %v", err)
	}
	if got != 0x1234 {
		t.Fatalf("parseInputOffset(explicit) = %#x, want 0x1234", got)
	}
}

func TestBoolEnv(t *testing.T) {
	t.Setenv("ELF2JSON_TEST_BOOL", "true")
	got, err := boolEnv("ELF2JSON_TEST_BOOL")
	if err != nil || !got {
		t.Fatalf("boolEnv(true) = %v, %v", got, err)
	}
	t.Setenv("ELF2JSON_TEST_BOOL", "invalid")
	if _, err := boolEnv("ELF2JSON_TEST_BOOL"); err == nil {
		t.Fatal("boolEnv(invalid) error = nil")
	}
}

func TestWriteJSONLegacyLayout(t *testing.T) {
	inputs := map[string][]byte{
		elfmapping.EntryPointAndBlobsCountInput: mustDecodeHex(t, "00000000008000000000000000000002"),
		elfmapping.BlobsOffsetAndSizeInput: mustDecodeHex(t, "00000000008000000000000000000004"+
			"00000000088000000000000000000001"),
		elfmapping.BlobsExecutableInput:  []byte{0x80},
		elfmapping.BlobsDataInput:        []byte{1, 2, 3, 4, 5},
		predecoding.InstructionBaseInput: mustDecodeHex(t, "0000000000800000"),
		predecoding.DecodedInput:         []byte{0xaa, 0xbb},
	}
	var output bytes.Buffer
	if err := writeJSON(&output, inputs); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	want := "{\n" +
		"\t\"entry_point_and_blobs_count\": \"0x0000000000800000_0000000000000002\",\n" +
		"\t\"blobs_offset_and_size\": \"0x0000000000800000_0000000000000004____0000000008800000_0000000000000001\",\n" +
		"\t\"blobs_executable\": \"0x80\",\n" +
		"\t\"blobs_data\": \"0x01020304____05\",\n" +
		"\t\"instruction_base\": \"0x0000000000800000\",\n" +
		"\t\"decoded\": \"0xaabb\"\n" +
		"}\n"
	if output.String() != want {
		t.Fatalf("writeJSON():\n%s\nwant:\n%s", output.String(), want)
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

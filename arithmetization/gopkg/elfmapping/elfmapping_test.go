package elfmapping_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
)

const (
	testEntryPoint  = 0x00800000
	testTextAddress = 0x00800000
)

var testText = []byte{0x97, 0x02, 0x00, 0x00}

func TestPrepareInputsMatchesExplicitPipeline(t *testing.T) {
	elfBytes := makeELF(testEntryPoint, testTextAddress, testText)
	inputData := []byte{0xaa, 0xbb}
	got, err := elfmapping.PrepareInputs(
		elfBytes,
		inputData,
		elfmapping.WithIncludeExecutable(),
	)
	if err != nil {
		t.Fatalf("PrepareInputs() error = %v", err)
	}

	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	inputBlobs, err := elfmapping.NewLengthPrefixedData(
		elfmapping.DefaultInputOrigin,
		inputData,
	)
	if err != nil {
		t.Fatalf("NewLengthPrefixedData() error = %v", err)
	}
	want, err := elfmapping.EncodeInputs(
		program,
		inputBlobs,
		elfmapping.WithIncludeExecutable(),
	)
	if err != nil {
		t.Fatalf("EncodeInputs() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrepareInputs() = %#v, want %#v", got, want)
	}
}

func TestPrepareInputsRejectsInvalidELF(t *testing.T) {
	inputs, err := elfmapping.PrepareInputs([]byte("not an ELF"), nil)
	if err == nil {
		t.Fatal("PrepareInputs() error = nil, want malformed ELF error")
	}
	if inputs != nil {
		t.Fatalf("PrepareInputs() inputs = %#v, want nil", inputs)
	}
}

func TestPrepareInputsWritesLegacySectionsTable(t *testing.T) {
	var sections bytes.Buffer
	inputs, err := elfmapping.PrepareInputs(
		makeELF(testEntryPoint, testTextAddress, testText),
		[]byte{0xaa, 0xbb},
		elfmapping.WithIncludeExecutable(),
		elfmapping.WithSectionsWriter(&sections),
	)
	if err != nil {
		t.Fatalf("PrepareInputs() error = %v", err)
	}
	if got := hex.EncodeToString(inputs[elfmapping.BlobsExecutableInput]); got != "80" {
		t.Errorf("blobs_executable = %s, want 80", got)
	}
	want := "index, offset,             size,               exec, name\n" +
		"0    , 0x0000000000800000, 0x0000000000000004, yes, .text\n" +
		"1    , 0x0000000008800000, 0x0000000000000008, no , ssz_length\n" +
		"2    , 0x0000000008800008, 0x0000000000000002, no , ssz_payload\n"
	if sections.String() != want {
		t.Errorf("sections table:\n%s\nwant:\n%s", sections.String(), want)
	}
}

func TestLoad(t *testing.T) {
	elfBytes := makeELF(testEntryPoint, testTextAddress, testText)
	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if program.EntryPoint != testEntryPoint {
		t.Fatalf("EntryPoint = %#x, want %#x", program.EntryPoint, testEntryPoint)
	}
	if len(program.Blobs) != 1 {
		t.Fatalf("len(Blobs) = %d, want 1", len(program.Blobs))
	}
	blob := program.Blobs[0]
	if blob.Address != testTextAddress {
		t.Errorf("Address = %#x, want %#x", blob.Address, testTextAddress)
	}
	if !bytes.Equal(blob.Data, testText) {
		t.Errorf("Data = %x, want %x", blob.Data, testText)
	}
	if blob.Name != ".text" {
		t.Errorf("Name = %q, want .text", blob.Name)
	}
	if !blob.Executable {
		t.Error("Executable = false, want true")
	}

	// Load promises that cached program data does not alias the ELF reader.
	for i := range elfBytes {
		elfBytes[i] = 0
	}
	if !bytes.Equal(blob.Data, testText) {
		t.Errorf("Data changed with source ELF: got %x, want %x", blob.Data, testText)
	}
}

func TestLoadRejectsInvalidELF(t *testing.T) {
	if _, err := elfmapping.Load(bytes.NewReader([]byte("not an elf"))); err == nil {
		t.Fatal("Load() error = nil, want malformed ELF error")
	}
}

func TestLoadRejectsInvalidMappings(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([]byte)
		wantErr string
	}{
		{
			name: "no loadable segments",
			mutate: func(elfBytes []byte) {
				binary.LittleEndian.PutUint32(elfBytes[64:68], 4)
			},
			wantErr: "no loadable sections",
		},
		{
			name: "file size exceeds memory size",
			mutate: func(elfBytes []byte) {
				binary.LittleEndian.PutUint64(elfBytes[64+32:64+40], 2)
				binary.LittleEndian.PutUint64(elfBytes[64+40:64+48], 1)
			},
			wantErr: "file size larger than memory size",
		},
		{
			name: "segment address overflow",
			mutate: func(elfBytes []byte) {
				binary.LittleEndian.PutUint64(elfBytes[64+16:64+24], math.MaxUint64-1)
				binary.LittleEndian.PutUint64(elfBytes[64+40:64+48], 4)
			},
			wantErr: "segment address overflow",
		},
		{
			name: "section address overflow",
			mutate: func(elfBytes []byte) {
				textHeader := textSectionHeader(elfBytes)
				binary.LittleEndian.PutUint64(elfBytes[textHeader+16:textHeader+24], math.MaxUint64-1)
				binary.LittleEndian.PutUint64(elfBytes[textHeader+32:textHeader+40], 4)
			},
			wantErr: "section .text address overflow",
		},
		{
			name: "short section read",
			mutate: func(elfBytes []byte) {
				textHeader := textSectionHeader(elfBytes)
				binary.LittleEndian.PutUint64(
					elfBytes[textHeader+24:textHeader+32],
					uint64(len(elfBytes)-2),
				)
				binary.LittleEndian.PutUint64(elfBytes[textHeader+32:textHeader+40], 4)
			},
			wantErr: "short read for section .text",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			elfBytes := makeELF(testEntryPoint, testTextAddress, testText)
			test.mutate(elfBytes)
			_, err := elfmapping.Load(bytes.NewReader(elfBytes))
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(test.wantErr)) {
				t.Fatalf("Load() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestNewLengthPrefixedData(t *testing.T) {
	payload := []byte{0xaa, 0xbb, 0xcc}
	blobs, err := elfmapping.NewLengthPrefixedData(
		elfmapping.DefaultInputOrigin,
		payload,
	)
	if err != nil {
		t.Fatalf("NewLengthPrefixedData() error = %v", err)
	}
	if len(blobs) != 2 {
		t.Fatalf("len(blobs) = %d, want 2", len(blobs))
	}
	if got := binary.LittleEndian.Uint64(blobs[0].Data); got != uint64(len(payload)) {
		t.Errorf("length prefix = %d, want %d", got, len(payload))
	}
	if blobs[0].Address != elfmapping.DefaultInputOrigin {
		t.Errorf("length address = %#x, want %#x", blobs[0].Address, elfmapping.DefaultInputOrigin)
	}
	if blobs[1].Address != elfmapping.DefaultInputOrigin+8 {
		t.Errorf("payload address = %#x, want %#x", blobs[1].Address, elfmapping.DefaultInputOrigin+8)
	}
	if !bytes.Equal(blobs[1].Data, payload) {
		t.Errorf("payload = %x, want %x", blobs[1].Data, payload)
	}
}

func TestNewLengthPrefixedDataEmpty(t *testing.T) {
	blobs, err := elfmapping.NewLengthPrefixedData(0x1000, nil)
	if err != nil {
		t.Fatalf("NewLengthPrefixedData() error = %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("len(blobs) = %d, want 1", len(blobs))
	}
	if got := binary.LittleEndian.Uint64(blobs[0].Data); got != 0 {
		t.Errorf("length prefix = %d, want 0", got)
	}
}

func TestNewLengthPrefixedDataRejectsAddressOverflow(t *testing.T) {
	_, err := elfmapping.NewLengthPrefixedData(math.MaxUint64-4, []byte{1})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("offset overflow")) {
		t.Fatalf("NewLengthPrefixedData() error = %v, want offset overflow", err)
	}
}

func TestEncodeInputsLegacyGolden(t *testing.T) {
	program := elfmapping.Program{
		EntryPoint: testEntryPoint,
		Blobs: []elfmapping.Blob{{
			Address:    testTextAddress,
			Data:       testText,
			Name:       ".text",
			Executable: true,
		}},
	}
	payload, err := elfmapping.NewLengthPrefixedData(
		elfmapping.DefaultInputOrigin,
		[]byte{0xaa, 0xbb, 0xcc},
	)
	if err != nil {
		t.Fatalf("NewLengthPrefixedData() error = %v", err)
	}
	inputs, err := elfmapping.EncodeInputs(
		program,
		payload,
		elfmapping.WithIncludeExecutable(),
	)
	if err != nil {
		t.Fatalf("EncodeInputs() error = %v", err)
	}

	want := map[string]string{
		elfmapping.EntryPointAndBlobsCountInput: "00000000008000000000000000000003",
		elfmapping.BlobsOffsetAndSizeInput: "00000000008000000000000000000004" +
			"00000000088000000000000000000008" +
			"00000000088000080000000000000003",
		elfmapping.BlobsDataInput:       "970200000300000000000000aabbcc",
		elfmapping.BlobsExecutableInput: "80",
	}
	if len(inputs) != len(want) {
		t.Fatalf("len(inputs) = %d, want %d", len(inputs), len(want))
	}
	for name, wantHex := range want {
		if got := hex.EncodeToString(inputs[name]); got != wantHex {
			t.Errorf("inputs[%q] = %s, want %s", name, got, wantHex)
		}
	}
}

func TestEncodeInputsDoesNotMutateOrAliasInputs(t *testing.T) {
	programBlobs := make([]elfmapping.Blob, 2, 4)
	programBlobs[0] = elfmapping.Blob{Address: 0x3000, Data: []byte{3}}
	programBlobs[1] = elfmapping.Blob{Address: 0x1000, Data: []byte{1}}
	additional := []elfmapping.Blob{
		{Address: 0x2000, Data: []byte{2}},
		{Address: 0x4000, Data: []byte{4}},
	}
	inputs, err := elfmapping.EncodeInputs(
		elfmapping.Program{Blobs: programBlobs},
		additional,
	)
	if err != nil {
		t.Fatalf("EncodeInputs() error = %v", err)
	}
	if got := hex.EncodeToString(inputs[elfmapping.BlobsDataInput]); got != "01020304" {
		t.Fatalf("blobs_data = %s, want 01020304", got)
	}
	if _, ok := inputs[elfmapping.BlobsExecutableInput]; ok {
		t.Error("default EncodeInputs() unexpectedly included blobs_executable")
	}
	if programBlobs[0].Address != 0x3000 || programBlobs[1].Address != 0x1000 {
		t.Errorf("program blobs reordered: %#v", programBlobs)
	}
	if additional[0].Address != 0x2000 || additional[1].Address != 0x4000 {
		t.Errorf("additional blobs reordered: %#v", additional)
	}

	programBlobs[1].Data[0] = 0xff
	additional[0].Data[0] = 0xff
	if got := hex.EncodeToString(inputs[elfmapping.BlobsDataInput]); got != "01020304" {
		t.Errorf("encoded output aliases input data: got %s", got)
	}
}

func TestEncodeInputsRejectsInvalidRanges(t *testing.T) {
	tests := []struct {
		name    string
		blobs   []elfmapping.Blob
		wantErr string
	}{
		{
			name: "overlap",
			blobs: []elfmapping.Blob{
				{Address: 0x1000, Data: []byte{1, 2}},
				{Address: 0x1001, Data: []byte{3}},
			},
			wantErr: "overlaps",
		},
		{
			name:    "overflow",
			blobs:   []elfmapping.Blob{{Address: math.MaxUint64, Data: []byte{1}}},
			wantErr: "overflows address space",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs, err := elfmapping.EncodeInputs(
				elfmapping.Program{},
				test.blobs,
			)
			if inputs != nil {
				t.Errorf("EncodeInputs() inputs = %#v, want nil", inputs)
			}
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(test.wantErr)) {
				t.Fatalf("EncodeInputs() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestEncodeInputsAllowsAdjacentAndEmptyBlobs(t *testing.T) {
	inputs, err := elfmapping.EncodeInputs(
		elfmapping.Program{Blobs: []elfmapping.Blob{
			{Address: 0x1000, Data: []byte{1, 2}},
			{Address: 0x1002, Data: nil},
		}},
		[]elfmapping.Blob{{Address: 0x1002, Data: []byte{3}}},
	)
	if err != nil {
		t.Fatalf("EncodeInputs() error = %v", err)
	}
	if got := hex.EncodeToString(inputs[elfmapping.BlobsDataInput]); got != "010203" {
		t.Errorf("blobs_data = %s, want 010203", got)
	}
	count := binary.BigEndian.Uint64(inputs[elfmapping.EntryPointAndBlobsCountInput][8:])
	if count != 3 {
		t.Errorf("blob count = %d, want 3", count)
	}
}

func textSectionHeader(elfBytes []byte) int {
	return int(binary.LittleEndian.Uint64(elfBytes[40:48])) + 64
}

// makeELF mirrors the legacy backend fixture: one PT_LOAD containing one
// allocated executable .text section and a section-name string table.
func makeELF(entryPoint, sectionAddress uint64, sectionData []byte) []byte {
	const (
		elfHeaderSize     = 64
		programHeaderSize = 56
		sectionHeaderSize = 64
	)
	stringTable := []byte("\x00.text\x00.shstrtab\x00")
	textOffset := uint64(elfHeaderSize + programHeaderSize)
	stringTableOffset := textOffset + uint64(len(sectionData))
	sectionHeaderOffset := (stringTableOffset + uint64(len(stringTable)) + 7) &^ 7
	result := make([]byte, sectionHeaderOffset+3*sectionHeaderSize)
	le := binary.LittleEndian

	copy(result, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	le.PutUint16(result[16:18], 2)
	le.PutUint16(result[18:20], 243)
	le.PutUint32(result[20:24], 1)
	le.PutUint64(result[24:32], entryPoint)
	le.PutUint64(result[32:40], elfHeaderSize)
	le.PutUint64(result[40:48], sectionHeaderOffset)
	le.PutUint16(result[52:54], elfHeaderSize)
	le.PutUint16(result[54:56], programHeaderSize)
	le.PutUint16(result[56:58], 1)
	le.PutUint16(result[58:60], sectionHeaderSize)
	le.PutUint16(result[60:62], 3)
	le.PutUint16(result[62:64], 2)

	programHeader := result[elfHeaderSize : elfHeaderSize+programHeaderSize]
	le.PutUint32(programHeader[0:4], 1)
	le.PutUint32(programHeader[4:8], 5)
	le.PutUint64(programHeader[8:16], textOffset)
	le.PutUint64(programHeader[16:24], sectionAddress)
	le.PutUint64(programHeader[24:32], sectionAddress)
	le.PutUint64(programHeader[32:40], uint64(len(sectionData)))
	le.PutUint64(programHeader[40:48], uint64(len(sectionData)))
	le.PutUint64(programHeader[48:56], 0x1000)
	copy(result[textOffset:], sectionData)
	copy(result[stringTableOffset:], stringTable)

	textHeader := result[sectionHeaderOffset+sectionHeaderSize:]
	le.PutUint32(textHeader[0:4], 1)
	le.PutUint32(textHeader[4:8], 1)
	le.PutUint64(textHeader[8:16], 6)
	le.PutUint64(textHeader[16:24], sectionAddress)
	le.PutUint64(textHeader[24:32], textOffset)
	le.PutUint64(textHeader[32:40], uint64(len(sectionData)))
	le.PutUint64(textHeader[48:56], 4)

	stringHeader := result[sectionHeaderOffset+2*sectionHeaderSize:]
	le.PutUint32(stringHeader[0:4], 7)
	le.PutUint32(stringHeader[4:8], 3)
	le.PutUint64(stringHeader[24:32], stringTableOffset)
	le.PutUint64(stringHeader[32:40], uint64(len(stringTable)))
	le.PutUint64(stringHeader[48:56], 1)
	return result
}

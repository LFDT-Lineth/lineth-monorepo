package predecoding

import (
	"bytes"
	"encoding/binary"
	"maps"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
)

func TestPrepareInputsMatchesExplicitPipeline(t *testing.T) {
	elfBytes := makePrepareInputsTestELF()
	inputData := []byte{0xaa, 0xbb}
	got, err := PrepareInputs(elfBytes, inputData)
	if err != nil {
		t.Fatalf("PrepareInputs() error = %v", err)
	}

	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	decoded, err := Predecode(program)
	if err != nil {
		t.Fatalf("Predecode() error = %v", err)
	}
	inputBlobs, err := elfmapping.NewData(
		elfmapping.DefaultInputOrigin,
		inputData,
		elfmapping.WithLengthPrefix(),
	)
	if err != nil {
		t.Fatalf("NewData() error = %v", err)
	}
	want, err := elfmapping.EncodeInputs(program, inputBlobs)
	if err != nil {
		t.Fatalf("EncodeInputs() error = %v", err)
	}
	maps.Copy(want, decoded.EncodeInputs())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrepareInputs() = %#v, want %#v", got, want)
	}
	for _, name := range []string{
		elfmapping.EntryPointAndBlobsCountInput,
		elfmapping.BlobsOffsetAndSizeInput,
		elfmapping.BlobsDataInput,
		InstructionBaseInput,
		DecodedInput,
	} {
		if _, ok := got[name]; !ok {
			t.Errorf("PrepareInputs() omitted %q", name)
		}
	}
}

func TestPrepareInputsForwardsPredecodingOptions(t *testing.T) {
	inputs, err := PrepareInputs(
		makePrepareInputsTestELF(),
		nil,
		WithMaxDecodedRecords(0),
	)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("cap 0")) {
		t.Fatalf("PrepareInputs() error = %v, want zero-cap error", err)
	}
	if inputs != nil {
		t.Fatalf("PrepareInputs() inputs = %#v, want nil", inputs)
	}
}

func TestPrepareInputsForwardsMappingOptions(t *testing.T) {
	var sections bytes.Buffer
	inputs, err := PrepareInputs(
		makePrepareInputsTestELF(),
		[]byte{0xaa},
		WithIncludeExecutable(),
		WithSectionsWriter(&sections),
	)
	if err != nil {
		t.Fatalf("PrepareInputs() error = %v", err)
	}
	if got := inputs[elfmapping.BlobsExecutableInput]; !bytes.Equal(got, []byte{0x80}) {
		t.Errorf("blobs_executable = %x, want 80", got)
	}
	if !bytes.Contains(sections.Bytes(), []byte("yes, .text")) {
		t.Errorf("sections table does not identify executable .text:\n%s", sections.String())
	}
	if !bytes.Contains(sections.Bytes(), []byte("0x0000000008800008, 0x0000000000000001, no , ")) {
		t.Errorf("sections table does not contain unnamed input payload:\n%s", sections.String())
	}
}

func TestPrepareInputsRejectsInvalidELF(t *testing.T) {
	inputs, err := PrepareInputs([]byte("not an ELF"), nil)
	if err == nil {
		t.Fatal("PrepareInputs() error = nil, want malformed ELF error")
	}
	if inputs != nil {
		t.Fatalf("PrepareInputs() inputs = %#v, want nil", inputs)
	}
}

func makePrepareInputsTestELF() []byte {
	const (
		elfHeaderSize     = 64
		programHeaderSize = 56
		sectionHeaderSize = 64
		entryPoint        = 0x00800000
	)
	text := []byte{0x97, 0x02, 0x00, 0x00}
	stringTable := []byte("\x00.text\x00.shstrtab\x00")
	textOffset := uint64(elfHeaderSize + programHeaderSize)
	stringTableOffset := textOffset + uint64(len(text))
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
	le.PutUint64(programHeader[16:24], entryPoint)
	le.PutUint64(programHeader[24:32], entryPoint)
	le.PutUint64(programHeader[32:40], uint64(len(text)))
	le.PutUint64(programHeader[40:48], uint64(len(text)))
	le.PutUint64(programHeader[48:56], 0x1000)
	copy(result[textOffset:], text)
	copy(result[stringTableOffset:], stringTable)

	textHeader := result[sectionHeaderOffset+sectionHeaderSize:]
	le.PutUint32(textHeader[0:4], 1)
	le.PutUint32(textHeader[4:8], 1)
	le.PutUint64(textHeader[8:16], 6)
	le.PutUint64(textHeader[16:24], entryPoint)
	le.PutUint64(textHeader[24:32], textOffset)
	le.PutUint64(textHeader[32:40], uint64(len(text)))
	le.PutUint64(textHeader[48:56], 4)

	stringHeader := result[sectionHeaderOffset+2*sectionHeaderSize:]
	le.PutUint32(stringHeader[0:4], 7)
	le.PutUint32(stringHeader[4:8], 3)
	le.PutUint64(stringHeader[24:32], stringTableOffset)
	le.PutUint64(stringHeader[32:40], uint64(len(stringTable)))
	le.PutUint64(stringHeader[48:56], 1)
	return result
}

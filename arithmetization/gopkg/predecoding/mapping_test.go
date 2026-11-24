package predecoding

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"slices"
	"testing"
)

const (
	testEntryPoint  = 0x00800000
	testTextAddress = 0x00800000
)

var testText = []byte{0x97, 0x02, 0x00, 0x00}

// ------------------------------------------------------------
// extractBlobs
// ------------------------------------------------------------

// TestLoadableSegments checks that only PT_LOAD segments with a non-zero memory
// size are returned, in the order of the program headers. Segments are
// identified by their virtual address.
func TestLoadableSegments(t *testing.T) {
	tests := []struct {
		name    string
		headers []elf.ProgHeader
		want    []uint64
	}{
		{name: "no segments"},
		{name: "load segment", headers: []elf.ProgHeader{
			{Type: elf.PT_LOAD, Vaddr: 0x1000, Memsz: 4},
		}, want: []uint64{0x1000}},
		{name: "load with zero memory size", headers: []elf.ProgHeader{
			{Type: elf.PT_LOAD, Vaddr: 0x1000, Memsz: 0},
		}},
		{name: "not a load segment", headers: []elf.ProgHeader{
			{Type: elf.PT_NOTE, Vaddr: 0x1000, Memsz: 4},
		}},
		{name: "mixed keeps header order", headers: []elf.ProgHeader{
			{Type: elf.PT_NOTE, Vaddr: 0x1000, Memsz: 4},
			{Type: elf.PT_LOAD, Vaddr: 0x3000, Memsz: 4},
			{Type: elf.PT_LOAD, Vaddr: 0x2000, Memsz: 0},
			{Type: elf.PT_LOAD, Vaddr: 0x2000, Memsz: 4},
		}, want: []uint64{0x3000, 0x2000}},
	}
	for _, tt := range tests {
		elfFile := &elf.File{}
		for _, header := range tt.headers {
			elfFile.Progs = append(elfFile.Progs, &elf.Prog{ProgHeader: header})
		}
		var got []uint64
		for _, segment := range loadableSegments(elfFile) {
			got = append(got, segment.Vaddr)
		}
		if !slices.Equal(got, tt.want) {
			t.Fatalf("%s: segments = %#x, want %#x", tt.name, got, tt.want)
		}
	}
}

// TestSegmentEnd checks the end of a segment's memory range, and rejects a file
// size larger than the memory size and a range that wraps the address space.
func TestSegmentEnd(t *testing.T) {
	tests := []struct {
		name    string
		vaddr   uint64
		filesz  uint64
		memsz   uint64
		wantEnd uint64
		wantErr bool
	}{
		{name: "memory equals file", vaddr: 0x1000, filesz: 4, memsz: 4, wantEnd: 0x1004},
		// bss means Block Started by Symbol, a section that starts at a symbol
		// and ends at the next symbol. It is empty and has no file size.
		{name: "bss beyond file", vaddr: 0x1000, filesz: 0, memsz: 8, wantEnd: 0x1008},
		{name: "file larger than memory", vaddr: 0x1000, filesz: 8, memsz: 4, wantErr: true},
		{name: "range ends at the top", vaddr: ^uint64(0) - 4, filesz: 0, memsz: 4, wantEnd: ^uint64(0)},
		{name: "range wraps", vaddr: ^uint64(0) - 3, filesz: 0, memsz: 4, wantErr: true},
	}
	for _, tt := range tests {
		end, err := segmentEnd(&elf.Prog{ProgHeader: elf.ProgHeader{
			Vaddr: tt.vaddr, Filesz: tt.filesz, Memsz: tt.memsz,
		}})
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && end != tt.wantEnd {
			t.Fatalf("%s: end = %#x, want %#x", tt.name, end, tt.wantEnd)
		}
	}
}

// TestIsLoadableSection checks that a section is loadable only when it is
// non-empty, is not SHT_NOBITS, and has SHF_ALLOC. The executable flag does not
// change the result.
func TestIsLoadableSection(t *testing.T) {
	tests := []struct {
		name   string
		header elf.SectionHeader
		want   bool
	}{
		{name: "allocated data", header: elf.SectionHeader{
			Type: elf.SHT_PROGBITS, Flags: elf.SHF_ALLOC, Size: 4,
		}, want: true},
		{name: "allocated executable", header: elf.SectionHeader{
			Type: elf.SHT_PROGBITS, Flags: elf.SHF_ALLOC | elf.SHF_EXECINSTR, Size: 4,
		}, want: true},
		{name: "empty", header: elf.SectionHeader{
			Type: elf.SHT_PROGBITS, Flags: elf.SHF_ALLOC, Size: 0,
		}},
		{name: "nobits bss", header: elf.SectionHeader{
			Type: elf.SHT_NOBITS, Flags: elf.SHF_ALLOC, Size: 8,
		}},
		{name: "not allocated", header: elf.SectionHeader{
			Type: elf.SHT_PROGBITS, Flags: 0, Size: 4,
		}},
	}
	for _, tt := range tests {
		section := &elf.Section{SectionHeader: tt.header}
		if got := isLoadableSection(section); got != tt.want {
			t.Fatalf("%s: isLoadableSection = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestSectionEnd checks the end of a section's address range, and rejects a
// range that wraps the address space.
func TestSectionEnd(t *testing.T) {
	tests := []struct {
		name    string
		addr    uint64
		size    uint64
		wantEnd uint64
		wantErr bool
	}{
		{name: "four bytes", addr: 0x1000, size: 4, wantEnd: 0x1004},
		{name: "empty", addr: 0x1000, size: 0, wantEnd: 0x1000},
		{name: "range ends at the top", addr: ^uint64(0) - 4, size: 4, wantEnd: ^uint64(0)},
		{name: "range wraps", addr: ^uint64(0) - 3, size: 4, wantErr: true},
	}
	for _, tt := range tests {
		end, err := sectionEnd(&elf.Section{SectionHeader: elf.SectionHeader{
			Name: tt.name, Addr: tt.addr, Size: tt.size,
		}})
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && end != tt.wantEnd {
			t.Fatalf("%s: end = %#x, want %#x", tt.name, end, tt.wantEnd)
		}
	}
}

// TestIsWithin checks that the range [start, end) lies fully inside
// [outerStart, outerEnd). Equal bounds count as inside.
func TestIsWithin(t *testing.T) {
	tests := []struct {
		name       string
		start, end uint64
		outerStart uint64
		outerEnd   uint64
		want       bool
	}{
		{name: "inside", start: 0x1004, end: 0x1008, outerStart: 0x1000, outerEnd: 0x2000, want: true},
		{name: "equal bounds", start: 0x1000, end: 0x2000, outerStart: 0x1000, outerEnd: 0x2000, want: true},
		{name: "empty at outer start", start: 0x1000, end: 0x1000, outerStart: 0x1000, outerEnd: 0x2000, want: true},
		{name: "starts before", start: 0x0fff, end: 0x1008, outerStart: 0x1000, outerEnd: 0x2000},
		{name: "ends after", start: 0x1004, end: 0x2001, outerStart: 0x1000, outerEnd: 0x2000},
		{name: "after outer", start: 0x3000, end: 0x3004, outerStart: 0x1000, outerEnd: 0x2000},
	}
	for _, tt := range tests {
		if got := isWithin(tt.start, tt.end, tt.outerStart, tt.outerEnd); got != tt.want {
			t.Fatalf("%s: isWithin = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// truncatingReaderAt serves an ELF file. The file can be parsed in full, then
// limited to fewer bytes or made to fail, to exercise section reads.
type truncatingReaderAt struct {
	data  []byte
	limit int
	fail  bool
}

func (r *truncatingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if r.fail {
		return 0, errors.New("injected read failure")
	}
	if off >= int64(r.limit) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:r.limit])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// TestReadSection checks that readSection returns the bytes of a section, and
// fails on a short read or a read error.
func TestReadSection(t *testing.T) {
	elfBytes := makeELF(testEntryPoint, testTextAddress, testText)
	tests := []struct {
		name    string
		visible int // bytes of the file readable from the section start; -1 for all
		fail    bool
		wantErr bool
	}{
		{name: "full section", visible: -1},
		{name: "short read", visible: 2, wantErr: true},
		{name: "read error", visible: -1, fail: true, wantErr: true},
	}
	for _, tt := range tests {
		reader := &truncatingReaderAt{data: elfBytes, limit: len(elfBytes)}
		elfFile, err := elf.NewFile(reader)
		if err != nil {
			t.Fatalf("%s: elf.NewFile() error = %v", tt.name, err)
		}
		var section *elf.Section
		for _, s := range elfFile.Sections {
			if s.Name == ".text" {
				section = s
			}
		}
		if section == nil {
			t.Fatalf("%s: no .text section", tt.name)
		}
		if tt.visible >= 0 {
			reader.limit = int(section.Offset) + tt.visible
		}
		reader.fail = tt.fail

		got, err := readSection(section)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && !bytes.Equal(got, testText) {
			t.Fatalf("%s: data = %x, want %x", tt.name, got, testText)
		}
	}
}

// TestBuildBlob checks that a blob takes the address, name and data of its
// section, and is executable only when the section has SHF_EXECINSTR.
func TestBuildBlob(t *testing.T) {
	tests := []struct {
		name     string
		flags    elf.SectionFlag
		wantExec bool
	}{
		{name: "data", flags: elf.SHF_ALLOC, wantExec: false},
		{name: "executable", flags: elf.SHF_ALLOC | elf.SHF_EXECINSTR, wantExec: true},
	}
	for _, tt := range tests {
		section := &elf.Section{SectionHeader: elf.SectionHeader{
			Name: ".text", Addr: 0x1000, Flags: tt.flags, Size: 4,
		}}
		blob := buildBlob(section, testText)
		if blob.Address != 0x1000 || blob.Name != ".text" || !bytes.Equal(blob.Data, testText) {
			t.Fatalf("%s: blob = %+v, want address, name and data of the section", tt.name, blob)
		}
		if blob.Executable != tt.wantExec {
			t.Fatalf("%s: Executable = %v, want %v", tt.name, blob.Executable, tt.wantExec)
		}
	}
}

// TestSortBlobsByAddress checks that blobs are ordered by ascending address, in
// place. Blobs are identified by name so that each one moves with its address.
func TestSortBlobsByAddress(t *testing.T) {
	a := Blob{Address: 0x1000, Name: "a"}
	b := Blob{Address: 0x2000, Name: "b"}
	c := Blob{Address: 0x3000, Name: "c"}
	tests := []struct {
		name  string
		blobs []Blob
		want  []string
	}{
		{name: "empty"},
		{name: "single", blobs: []Blob{a}, want: []string{"a"}},
		{name: "sorted", blobs: []Blob{a, b, c}, want: []string{"a", "b", "c"}},
		{name: "reversed", blobs: []Blob{c, b, a}, want: []string{"a", "b", "c"}},
		{name: "unsorted", blobs: []Blob{b, c, a}, want: []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		sortBlobsByAddress(tt.blobs)
		var got []string
		for _, blob := range tt.blobs {
			got = append(got, blob.Name)
		}
		if !slices.Equal(got, tt.want) {
			t.Fatalf("%s: order = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestExtractBlobs checks the paths and errors of extractBlobs.
// The ELF is the makeELF fixture (one PT_LOAD at 0x800000 holding one
// 4-byte .text section), with its parsed headers changed per case. Blobs are
// identified by name.
func TestExtractBlobs(t *testing.T) {
	// addSection adds a copy of .text, with another name and address.
	addSection := func(f *elf.File, name string, addr uint64) *elf.Section {
		section := *f.Sections[1]
		section.Name = name
		section.Addr = addr
		f.Sections = append(f.Sections, &section)
		return &section
	}
	// addSegment adds a PT_LOAD segment.
	addSegment := func(f *elf.File, vaddr, size uint64) {
		f.Progs = append(f.Progs, &elf.Prog{ProgHeader: elf.ProgHeader{
			Type: elf.PT_LOAD, Vaddr: vaddr, Filesz: size, Memsz: size,
		}})
	}
	tests := []struct {
		name    string
		mutate  func(f *elf.File)
		want    []string
		wantErr bool
	}{
		{name: "one segment", want: []string{".text"}},
		{name: "segments keep header order", mutate: func(f *elf.File) {
			addSection(f, ".text2", 0x900000)
			addSegment(f, 0x900000, 4)
			f.Progs[0], f.Progs[1] = f.Progs[1], f.Progs[0]
		}, want: []string{".text2", ".text"}},
		{name: "sections sorted in a segment", mutate: func(f *elf.File) {
			f.Progs[0].Vaddr, f.Progs[0].Filesz, f.Progs[0].Memsz = 0x7ffffc, 8, 8
			addSection(f, ".low", 0x7ffffc)
		}, want: []string{".low", ".text"}},
		{name: "section not loadable is skipped", mutate: func(f *elf.File) {
			f.Progs[0].Filesz, f.Progs[0].Memsz = 8, 8
			addSection(f, ".bss", 0x800004).Type = elf.SHT_NOBITS
		}, want: []string{".text"}},
		{name: "section outside the segment is skipped", mutate: func(f *elf.File) {
			addSection(f, ".other", 0x900000)
		}, want: []string{".text"}},
		{name: "no loadable segment", mutate: func(f *elf.File) {
			f.Progs[0].Type = elf.PT_NOTE
		}, wantErr: true},
		{name: "no loadable section", mutate: func(f *elf.File) {
			f.Sections[1].Type = elf.SHT_NOBITS
		}, wantErr: true},
		{name: "section outside every segment", mutate: func(f *elf.File) {
			f.Progs[0].Vaddr = 0x900000
		}, wantErr: true},
		{name: "segment error", mutate: func(f *elf.File) {
			f.Progs[0].Filesz = f.Progs[0].Memsz + 1
		}, wantErr: true},
		{name: "section error", mutate: func(f *elf.File) {
			f.Sections[1].Addr = ^uint64(0)
		}, wantErr: true},
		{name: "read error", mutate: func(f *elf.File) {
			f.Progs[0].Memsz = 8
			f.Sections[1].Size = 8
		}, wantErr: true},
	}
	for _, tt := range tests {
		elfFile, err := elf.NewFile(bytes.NewReader(makeELF(testEntryPoint, testTextAddress, testText)))
		if err != nil {
			t.Fatalf("%s: elf.NewFile() error = %v", tt.name, err)
		}
		if tt.mutate != nil {
			tt.mutate(elfFile)
		}
		blobs, err := extractBlobs(elfFile)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		var got []string
		for _, blob := range blobs {
			got = append(got, blob.Name)
		}
		if !slices.Equal(got, tt.want) {
			t.Fatalf("%s: blobs = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// ------------------------------------------------------------
// Load
// ------------------------------------------------------------

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

// TestLoadPaths checks that Load returns the entry point and blobs of a valid
// ELF, and fails when the input is not an ELF or extractBlobs fails.
func TestLoadPaths(t *testing.T) {
	noLoadable := makeELF(testEntryPoint, testTextAddress, testText)
	// Set p_type of the only program header to PT_NOTE, so no segment is loadable.
	binary.LittleEndian.PutUint32(noLoadable[64:68], uint32(elf.PT_NOTE))
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{name: "valid ELF", input: makeELF(testEntryPoint, testTextAddress, testText)},
		{name: "not an ELF", input: []byte("not an ELF"), wantErr: true},
		{name: "no loadable segment", input: noLoadable, wantErr: true},
	}
	for _, tt := range tests {
		program, err := Load(bytes.NewReader(tt.input))
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr {
			continue
		}
		if program.EntryPoint != testEntryPoint {
			t.Fatalf("%s: EntryPoint = %#x, want %#x", tt.name, program.EntryPoint, testEntryPoint)
		}
		if len(program.Blobs) != 1 || program.Blobs[0].Address != testTextAddress ||
			!bytes.Equal(program.Blobs[0].Data, testText) {
			t.Fatalf("%s: Blobs = %+v, want one blob at %#x with %x",
				tt.name, program.Blobs, testTextAddress, testText)
		}
	}
}

// ------------------------------------------------------------
// NewData
// ------------------------------------------------------------

// TestApplyDataOptions checks that the options set the data config, and that a
// nil option or an option that fails is an error.
func TestApplyDataOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []DataOption
		want    dataConfig
		wantErr bool
	}{
		{name: "no options", want: dataConfig{}},
		{name: "name only", options: []DataOption{WithName("input")}, want: dataConfig{name: "input"}},
		{name: "length prefix only", options: []DataOption{WithLengthPrefix()}, want: dataConfig{lengthPrefix: true}},
		{name: "name and length prefix", options: []DataOption{
			WithName("input"), WithLengthPrefix(),
		}, want: dataConfig{name: "input", lengthPrefix: true}},
		{name: "nil option", options: []DataOption{nil}, wantErr: true},
		{name: "failing option", options: []DataOption{WithName("")}, wantErr: true},
	}
	for _, tt := range tests {
		got, err := applyDataOptions(tt.options)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Fatalf("%s: config = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// TestRawBlobs checks that data becomes one blob at the address, named as given,
// and that empty data gives no blob.
func TestRawBlobs(t *testing.T) {
	tests := []struct {
		name     string
		address  uint64
		data     []byte
		blobName string
		want     []Blob
	}{
		{name: "empty data", address: 0x1000, data: nil, blobName: "input", want: nil},
		{name: "unnamed blob", address: 0x1000, data: testText, want: []Blob{
			{Address: 0x1000, Data: testText},
		}},
		{name: "named blob", address: 0x1000, data: testText, blobName: "input", want: []Blob{
			{Address: 0x1000, Data: testText, Name: "input"},
		}},
	}
	for _, tt := range tests {
		got := rawBlobs(tt.address, tt.data, tt.blobName)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: blobs = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// TestPayloadAddress checks that the payload sits eight bytes after the length
// prefix, and that an address whose payload wraps the address space is an error.
func TestPayloadAddress(t *testing.T) {
	tests := []struct {
		name    string
		address uint64
		want    uint64
		wantErr bool
	}{
		{name: "zero", address: 0, want: 0x08},
		{name: "typical", address: 0x1000, want: 0x1008},
		{name: "highest without wrap", address: ^uint64(0) - 8, want: ^uint64(0)},
		{name: "payload wraps", address: ^uint64(0) - 7, wantErr: true},
	}
	for _, tt := range tests {
		got, err := payloadAddress(tt.address)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Fatalf("%s: payload address = %#x, want %#x", tt.name, got, tt.want)
		}
	}
}

// TestNewBlob checks that a blob keeps its address and data, and that its name
// is the name with the suffix. An empty name stays empty.
func TestNewBlob(t *testing.T) {
	tests := []struct {
		name     string
		blobName string
		suffix   string
		want     Blob
	}{
		{name: "unnamed", blobName: "", suffix: "_length", want: Blob{Address: 0x1000, Data: testText}},
		{name: "length suffix", blobName: "input", suffix: "_length", want: Blob{
			Address: 0x1000, Data: testText, Name: "input_length",
		}},
		{name: "payload suffix", blobName: "input", suffix: "_payload", want: Blob{
			Address: 0x1000, Data: testText, Name: "input_payload",
		}},
		{name: "empty suffix", blobName: "input", suffix: "", want: Blob{
			Address: 0x1000, Data: testText, Name: "input",
		}},
	}
	for _, tt := range tests {
		got := newBlob(0x1000, testText, tt.blobName, tt.suffix)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: blob = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// TestLengthPrefix checks that the length is encoded as eight bytes, least
// significant byte first.
func TestLengthPrefix(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   []byte
	}{
		{name: "zero", length: 0, want: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "one", length: 1, want: []byte{1, 0, 0, 0, 0, 0, 0, 0}},
		{name: "two bytes", length: 0x0102, want: []byte{0x02, 0x01, 0, 0, 0, 0, 0, 0}},
	}
	for _, tt := range tests {
		got := lengthPrefix(tt.length)
		if !bytes.Equal(got, tt.want) {
			t.Fatalf("%s: prefix = %x, want %x", tt.name, got, tt.want)
		}
	}
}

// TestLengthPrefixedBlobs checks the blobs this function assembles: only the
// length blob when the data is empty, the length blob and the payload blob
// otherwise, and an error when the payload address overflows.
func TestLengthPrefixedBlobs(t *testing.T) {
	tests := []struct {
		name     string
		address  uint64
		data     []byte
		blobName string
		want     []Blob
		wantErr  bool
	}{
		{name: "empty data", address: 0x1000, blobName: "input", want: []Blob{
			{Address: 0x1000, Data: []byte{0, 0, 0, 0, 0, 0, 0, 0}, Name: "input_length"},
		}},
		{name: "data", address: 0x1000, data: testText, blobName: "input", want: []Blob{
			{Address: 0x1000, Data: []byte{4, 0, 0, 0, 0, 0, 0, 0}, Name: "input_length"},
			{Address: 0x1008, Data: testText, Name: "input_payload"},
		}},
		{name: "unnamed", address: 0x1000, data: testText, want: []Blob{
			{Address: 0x1000, Data: []byte{4, 0, 0, 0, 0, 0, 0, 0}},
			{Address: 0x1008, Data: testText},
		}},
		{name: "payload address overflows", address: ^uint64(0) - 7, data: testText, wantErr: true},
	}
	for _, tt := range tests {
		got, err := lengthPrefixedBlobs(tt.address, tt.data, tt.blobName)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: blobs = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// TestNewData checks which path NewData takes: one raw blob, no blob for empty
// raw data, length-prefixed blobs, an error from an option, and the length
// prefix overflow.
func TestNewData(t *testing.T) {
	tests := []struct {
		name    string
		address uint64
		data    []byte
		options []DataOption
		want    []Blob
		wantErr bool
	}{
		{name: "raw", address: 0x1000, data: testText, want: []Blob{
			{Address: 0x1000, Data: testText},
		}},
		{name: "raw empty", address: 0x1000},
		{name: "length prefix", address: 0x1000, data: testText, options: []DataOption{
			WithName("input"), WithLengthPrefix(),
		}, want: []Blob{
			{Address: 0x1000, Data: []byte{4, 0, 0, 0, 0, 0, 0, 0}, Name: "input_length"},
			{Address: 0x1008, Data: testText, Name: "input_payload"},
		}},
		{name: "nil option", options: []DataOption{nil}, wantErr: true},
		{name: "length prefix overflows", address: ^uint64(0) - 7, data: testText, options: []DataOption{
			WithLengthPrefix(),
		}, wantErr: true},
	}
	for _, tt := range tests {
		got, err := NewData(tt.address, tt.data, tt.options...)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: blobs = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// ------------------------------------------------------------
// BitArrayFromBytes
// ------------------------------------------------------------

// TestBitArrayFromBytes checks that the array holds eight bits per byte, and
// that its bytes are the input bytes.
func TestBitArrayFromBytes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "no bytes", input: nil},
		{name: "one byte", input: []byte{0xa5}},
		{name: "two bytes", input: []byte{0x12, 0x34}},
	}
	for _, tt := range tests {
		got := BitArrayFromBytes(tt.input...)
		if want := uint(len(tt.input) * 8); got.Len() != want {
			t.Fatalf("%s: Len() = %d, want %d", tt.name, got.Len(), want)
		}
		if !bytes.Equal(got.Bytes(), tt.input) {
			t.Fatalf("%s: Bytes() = %x, want %x", tt.name, got.Bytes(), tt.input)
		}
	}
}

// ------------------------------------------------------------
// encodeInputs
// ------------------------------------------------------------

// TestMergeAndSortBlobs checks that the blobs of both inputs are returned in
// ascending address order, that blobs with the same address keep the order of
// the inputs, and that the inputs are not modified.
func TestMergeAndSortBlobs(t *testing.T) {
	tests := []struct {
		name   string
		first  []Blob
		second []Blob
		want   []string
	}{
		{name: "both empty"},
		{name: "first only", first: []Blob{{Address: 0x1000, Name: "a"}}, want: []string{"a"}},
		{name: "second only", second: []Blob{{Address: 0x1000, Name: "b"}}, want: []string{"b"}},
		{name: "sorted across inputs", first: []Blob{{Address: 0x3000, Name: "a"}},
			second: []Blob{{Address: 0x1000, Name: "b"}}, want: []string{"b", "a"}},
		{name: "same address keeps first before second", first: []Blob{{Address: 0x1000, Name: "a"}},
			second: []Blob{{Address: 0x1000, Name: "b"}}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		first := slices.Clone(tt.first)
		second := slices.Clone(tt.second)
		got := mergeAndSortBlobs(tt.first, tt.second)
		var names []string
		for _, blob := range got {
			names = append(names, blob.Name)
		}
		if !slices.Equal(names, tt.want) {
			t.Fatalf("%s: names = %v, want %v", tt.name, names, tt.want)
		}
		if !reflect.DeepEqual(tt.first, first) || !reflect.DeepEqual(tt.second, second) {
			t.Fatalf("%s: an input was modified", tt.name)
		}
	}
}

// TestValidateBlobsRanges checks blobs sorted by address. Touching blobs and gaps
// are accepted. Empty blobs are ignored. A blob whose end wraps the address
// space, and a blob that starts before the end of the previous non-empty blob,
// are rejected.
func TestValidateBlobsRanges(t *testing.T) {
	blob := func(address uint64, size int) Blob {
		return Blob{Address: address, Data: make([]byte, size)}
	}
	tests := []struct {
		name    string
		blobs   []Blob
		wantErr bool
	}{
		{name: "no blobs"},
		{name: "one blob", blobs: []Blob{blob(0x1000, 4)}},
		{name: "touching blobs", blobs: []Blob{blob(0x1000, 4), blob(0x1004, 4)}},
		{name: "gap between blobs", blobs: []Blob{blob(0x1000, 4), blob(0x2000, 4)}},
		{name: "empty blob inside another", blobs: []Blob{blob(0x1000, 8), blob(0x1004, 0)}},
		{name: "empty blob at the top", blobs: []Blob{blob(^uint64(0), 0)}},
		{name: "ends at the top", blobs: []Blob{blob(^uint64(0)-4, 4)}},
		{name: "end wraps", blobs: []Blob{blob(^uint64(0)-3, 4)}, wantErr: true},
		{name: "overlap", blobs: []Blob{blob(0x1000, 8), blob(0x1004, 4)}, wantErr: true},
		{name: "same address", blobs: []Blob{blob(0x1000, 4), blob(0x1000, 4)}, wantErr: true},
		{name: "empty blob does not move the previous end", blobs: []Blob{
			blob(0x1000, 8), blob(0x1004, 0), blob(0x1006, 4),
		}, wantErr: true},
	}
	for _, tt := range tests {
		err := validateBlobsRanges(tt.blobs)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
	}
}

var errWrite = errors.New("write failed")

// errWriter is an io.Writer whose every write fails with errWrite.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

// TestWriteSectionsIfRequested checks that the sections table is written only
// when a writer is set, and that a failing write is returned wrapped.
func TestWriteSectionsIfRequested(t *testing.T) {
	tests := []struct {
		name       string
		newWriter  func(buf *bytes.Buffer) io.Writer
		wantHeader bool
		wantErr    bool
	}{
		{name: "nil writer writes nothing", newWriter: func(*bytes.Buffer) io.Writer {
			return nil
		}},
		{name: "writer receives the table", newWriter: func(buf *bytes.Buffer) io.Writer {
			return buf
		}, wantHeader: true},
		{name: "write error is returned", newWriter: func(*bytes.Buffer) io.Writer {
			return errWriter{}
		}, wantErr: true},
	}
	blobs := []Blob{{Address: 0x1000, Data: testText, Name: ".text"}}
	for _, tt := range tests {
		var buf bytes.Buffer
		err := writeSectionsIfRequested(tt.newWriter(&buf), blobs)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr && !errors.Is(err, errWrite) {
			t.Fatalf("%s: err = %v, want it to wrap %v", tt.name, err, errWrite)
		}
		if bytes.Contains(buf.Bytes(), []byte("index, offset")) != tt.wantHeader {
			t.Fatalf("%s: table written = %v, want %v", tt.name, !tt.wantHeader, tt.wantHeader)
		}
	}
}

// TestU32Layout checks the u32 entry point, addresses and data size of a valid
// layout, and that an entry point or blob outside the 32-bit address space is
// an error. The blob count and data size limit is not tested, because it needs
// more than 4 GiB of data.
func TestU32Layout(t *testing.T) {
	blob := func(address uint64, size int) Blob {
		return Blob{Address: address, Data: make([]byte, size)}
	}
	tests := []struct {
		name          string
		entryPoint    uint64
		blobs         []Blob
		wantEntry     uint32
		wantAddresses []uint32
		wantSize      int
		wantErr       bool
	}{
		{name: "no blobs", entryPoint: 0x800000, wantEntry: 0x800000},
		{name: "two blobs", entryPoint: 0x800000, blobs: []Blob{blob(0x1000, 4), blob(0x2000, 8)},
			wantEntry: 0x800000, wantAddresses: []uint32{0x1000, 0x2000}, wantSize: 12},
		{name: "entry point outside 32 bits", entryPoint: 1 << 32, wantErr: true},
		{name: "blob outside 32 bits", entryPoint: 0x800000, blobs: []Blob{blob(1<<32, 4)}, wantErr: true},
	}
	for _, tt := range tests {
		entry, addresses, size, err := u32Layout(tt.entryPoint, tt.blobs)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr {
			continue
		}
		if entry != tt.wantEntry || !slices.Equal(addresses, tt.wantAddresses) || size != tt.wantSize {
			t.Fatalf("%s: got (%#x, %v, %d), want (%#x, %v, %d)", tt.name,
				entry, addresses, size, tt.wantEntry, tt.wantAddresses, tt.wantSize)
		}
	}
}

// TestEncodeEntryAndCount checks that the entry point and the count are two
// big-endian u32 values, the entry point first.
func TestEncodeEntryAndCount(t *testing.T) {
	tests := []struct {
		name       string
		entryPoint uint32
		count      int
		want       []byte
	}{
		{name: "zero", want: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "entry point and count", entryPoint: 0x00800000, count: 3,
			want: []byte{0x00, 0x80, 0x00, 0x00, 0, 0, 0, 3}},
		{name: "byte order", entryPoint: 0x01020304, count: 0x05060708,
			want: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, tt := range tests {
		got := encodeEntryAndCount(tt.entryPoint, tt.count)
		if !bytes.Equal(got, tt.want) {
			t.Fatalf("%s: bytes = %x, want %x", tt.name, got, tt.want)
		}
	}
}

// TestEncodeBlobTables checks that each blob gets a big-endian address and size
// in the first table, and that the second table is the blob bytes in order.
func TestEncodeBlobTables(t *testing.T) {
	tests := []struct {
		name      string
		addresses []uint32
		blobs     []Blob
		wantTable []byte
		wantData  []byte
	}{
		{name: "no blobs"},
		{name: "one blob", addresses: []uint32{0x1000}, blobs: []Blob{{Data: []byte{1, 2, 3, 4}}},
			wantTable: []byte{0, 0, 0x10, 0, 0, 0, 0, 4}, wantData: []byte{1, 2, 3, 4}},
		{name: "two blobs", addresses: []uint32{0x1000, 0x2000}, blobs: []Blob{
			{Data: []byte{1, 2}}, {Data: []byte{3}},
		}, wantTable: []byte{0, 0, 0x10, 0, 0, 0, 0, 2, 0, 0, 0x20, 0, 0, 0, 0, 1},
			wantData: []byte{1, 2, 3}},
		{name: "empty blob", addresses: []uint32{0x1000}, blobs: []Blob{{}},
			wantTable: []byte{0, 0, 0x10, 0, 0, 0, 0, 0}},
	}
	for _, tt := range tests {
		dataSize := 0
		for _, blob := range tt.blobs {
			dataSize += len(blob.Data)
		}
		table, data := encodeBlobTables(tt.addresses, tt.blobs, dataSize)
		if !bytes.Equal(table, tt.wantTable) {
			t.Fatalf("%s: table = %x, want %x", tt.name, table, tt.wantTable)
		}
		if !bytes.Equal(data, tt.wantData) {
			t.Fatalf("%s: data = %x, want %x", tt.name, data, tt.wantData)
		}
	}
}

// TestBuildInputs checks that the three inputs are keyed by their names, hold
// the given bytes as bit arrays, and that no other input is added.
func TestBuildInputs(t *testing.T) {
	tests := []struct {
		name            string
		entryAndCount   []byte
		offsetsAndSizes []byte
		data            []byte
	}{
		{name: "empty"},
		{name: "filled", entryAndCount: []byte{0, 0x80, 0, 0, 0, 0, 0, 1},
			offsetsAndSizes: []byte{0, 0, 0x10, 0, 0, 0, 0, 4}, data: []byte{1, 2, 3, 4}},
	}
	for _, tt := range tests {
		inputs := buildInputs(tt.entryAndCount, tt.offsetsAndSizes, tt.data)
		want := map[string][]byte{
			EntryPointAndBlobsCountInput: tt.entryAndCount,
			BlobsOffsetAndSizeInput:      tt.offsetsAndSizes,
			BlobsDataInput:               tt.data,
		}
		if len(inputs) != len(want) {
			t.Fatalf("%s: %d inputs, want %d", tt.name, len(inputs), len(want))
		}
		for name, wantBytes := range want {
			arr, ok := inputs[name]
			if !ok {
				t.Fatalf("%s: input %s is missing", tt.name, name)
			}
			if !bytes.Equal(arr.Bytes(), wantBytes) || arr.Len() != uint(8*len(wantBytes)) {
				t.Fatalf("%s: input %s = %x with %d bits, want %x with %d bits", tt.name, name,
					arr.Bytes(), arr.Len(), wantBytes, 8*len(wantBytes))
			}
		}
	}
}

// TestEncodeInputsInternal checks the inputs of a program blob at 0x800000 and an extra
// blob at 0x1000, which sorts first. The executable input is present only with
// includeExecutable. Each failing step is an error: the ranges, the sections
// writer and the 32-bit address space. The blob count and data size limit is
// not tested, because it needs more than 4 GiB of data.
func TestEncodeInputsInternal(t *testing.T) {
	programBlobs := []Blob{{Address: 0x00800000, Data: testText, Name: ".text", Executable: true}}
	program := Program{EntryPoint: 0x00800000, Blobs: programBlobs}
	additional := []Blob{{Address: 0x1000, Data: []byte{0xaa, 0xbb}}}
	// wantInputs holds the expected bytes. The extra blob is first, so the
	// executable bits are 0 then 1, padded with zeros: 0b01000000.
	wantInputs := func(executable []byte) map[string][]byte {
		want := map[string][]byte{
			EntryPointAndBlobsCountInput: {0x00, 0x80, 0, 0, 0, 0, 0, 2},
			BlobsOffsetAndSizeInput:      {0, 0, 0x10, 0, 0, 0, 0, 2, 0, 0x80, 0, 0, 0, 0, 0, 4},
			BlobsDataInput:               {0xaa, 0xbb, 0x97, 0x02, 0, 0},
		}
		if executable != nil {
			want[BlobsExecutableInput] = executable
		}
		return want
	}
	tests := []struct {
		name       string
		program    Program
		additional []Blob
		cfg        config
		want       map[string][]byte
		wantErr    bool
	}{
		{name: "without executable", program: program, additional: additional, want: wantInputs(nil)},
		{name: "with executable", program: program, additional: additional,
			cfg: config{includeExecutable: true}, want: wantInputs([]byte{0x40})},
		{name: "ranges overlap", program: program, wantErr: true,
			additional: []Blob{{Address: 0x00800002, Data: []byte{1, 2}}}},
		{name: "sections writer fails", program: program, additional: additional,
			cfg: config{sectionsWriter: errWriter{}}, wantErr: true},
		{name: "entry point outside 32 bits", additional: additional, wantErr: true,
			program: Program{EntryPoint: 1 << 32, Blobs: programBlobs}},
	}
	for _, tt := range tests {
		inputs, err := encodeInputs(tt.program, tt.additional, tt.cfg)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr {
			continue
		}
		if len(inputs) != len(tt.want) {
			t.Fatalf("%s: %d inputs, want %d", tt.name, len(inputs), len(tt.want))
		}
		for name, wantBytes := range tt.want {
			arr, ok := inputs[name]
			if !ok {
				t.Fatalf("%s: input %s is missing", tt.name, name)
			}
			if !bytes.Equal(arr.Bytes(), wantBytes) {
				t.Fatalf("%s: input %s = %x, want %x", tt.name, name, arr.Bytes(), wantBytes)
			}
		}
	}
}

// ------------------------------------------------------------
// applyInputOptions
// ------------------------------------------------------------

// TestApplyInputOptions checks that the options set the encoding config, and
// that a nil option or a failing option is an error.
func TestApplyInputOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	tests := []struct {
		name    string
		options []Option
		want    config
		wantErr bool
	}{
		{name: "no options", want: config{}},
		{name: "include executable", options: []Option{WithIncludeExecutable()},
			want: config{includeExecutable: true}},
		{name: "sections writer", options: []Option{WithSectionsWriter(buf)},
			want: config{sectionsWriter: buf}},
		{name: "both options", options: []Option{WithIncludeExecutable(), WithSectionsWriter(buf)},
			want: config{includeExecutable: true, sectionsWriter: buf}},
		{name: "nil option", options: []Option{nil}, wantErr: true},
		{name: "failing option", options: []Option{WithSectionsWriter(nil)}, wantErr: true},
	}
	for _, tt := range tests {
		got, err := applyInputOptions(tt.options)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: config = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// ------------------------------------------------------------
// EncodeInputs
// ------------------------------------------------------------

// TestEncodeInputs checks that the options are applied, with the executable
// input added by WithIncludeExecutable, and that an error from the options is
// returned. The option errors themselves are tested in TestApplyInputOptions.
func TestEncodeInputs(t *testing.T) {
	program := Program{EntryPoint: 0x00800000, Blobs: []Blob{
		{Address: 0x00800000, Data: testText, Name: ".text", Executable: true},
	}}
	tests := []struct {
		name       string
		options    []Option
		wantInputs int
		wantErr    bool
	}{
		{name: "no options", wantInputs: 3},
		{name: "with executable", options: []Option{WithIncludeExecutable()}, wantInputs: 4},
		{name: "option error", options: []Option{nil}, wantErr: true},
	}
	for _, tt := range tests {
		inputs, err := EncodeInputs(program, nil, tt.options...)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && len(inputs) != tt.wantInputs {
			t.Fatalf("%s: %d inputs, want %d", tt.name, len(inputs), tt.wantInputs)
		}
	}
}

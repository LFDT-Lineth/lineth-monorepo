package predecoding

import (
	"bytes"
	"debug/elf"
	"errors"
	"io"
	"slices"
	"testing"
)

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

// ------------------------------------------------------------
// segmentEnd
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// isLoadableSection
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// sectionEnd
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// isWithin
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// readSection
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// buildBlob
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// sortBlobsByAddress
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// extractBlobs: paths and errors
// ------------------------------------------------------------

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

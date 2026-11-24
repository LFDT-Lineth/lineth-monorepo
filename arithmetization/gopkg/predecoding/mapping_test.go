package predecoding

import (
	"debug/elf"
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

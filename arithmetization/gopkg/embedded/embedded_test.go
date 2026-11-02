package embedded

import (
	"slices"
	"testing"
	"testing/fstest"
)

func TestCompiledBinaryFile(t *testing.T) {
	binfile, err := CompiledBinaryFile(WithAirValidation())
	if err != nil {
		t.Fatalf("failed to compile embedded R5 interpreter: %v", err)
	}
	if binfile == nil {
		t.Fatalf("compiled binary file is nil")
	}
}

// TestSourceFilesFollowIncludes checks that only the entry point and the files
// it reaches through (possibly relative or glob) includes are compiled, so a
// library that nothing includes stays out of the system.
func TestSourceFilesFollowIncludes(t *testing.T) {
	rootFs := fstest.MapFS{
		"main/riscv/main.zkc":         {Data: []byte("include \"memory.zkc\"\ninclude \"../lib/io/*.zkc\"\n")},
		"main/riscv/memory.zkc":       {Data: []byte("include \"../common/type.zkc\"\n")},
		"main/common/type.zkc":        {Data: []byte("include \"../riscv/memory.zkc\"\n")},
		"main/lib/io/write.zkc":       {Data: []byte("")},
		"main/lib/io/read.zkc":        {Data: []byte("")},
		"main/lib/keccak/impl.zkc":    {Data: []byte("include \"../../riscv/memory.zkc\"\n")},
		"main/predecoding/decode.zkc": {Data: []byte("")},
	}
	var got []string
	for _, f := range arithmetizationSourceFiles(rootFs) {
		got = append(got, f.Filename())
	}
	slices.Sort(got)
	want := []string{
		"/common/type.zkc",
		"/lib/io/read.zkc",
		"/lib/io/write.zkc",
		"/riscv/main.zkc",
		"/riscv/memory.zkc",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("source files: got %v, want %v", got, want)
	}
}

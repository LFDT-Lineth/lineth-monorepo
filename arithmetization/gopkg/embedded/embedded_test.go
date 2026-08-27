package embedded

import (
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
)

func TestCompiledBinaryFile(t *testing.T) {
	cfg := codegen.DEFAULT_CONFIG
	binfile, err := CompiledBinaryFile(cfg, nil, nil)
	if err != nil {
		t.Fatalf("failed to compile embedded R5 interpreter: %v", err)
	}
	if binfile == nil {
		t.Fatalf("compiled binary file is nil")
	}
}

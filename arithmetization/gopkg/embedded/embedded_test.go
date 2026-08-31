package embedded

import (
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
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
	air := binfile.AirConstraints()
	errs := constraints.Validate(air)
	if len(errs) > 0 {
		t.Fatalf("air constraints validation failed: %v", errs)
	}
}

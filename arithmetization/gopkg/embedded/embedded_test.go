package embedded

import (
	"testing"
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

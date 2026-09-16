package codegen

import (
	"bytes"
	"testing"
)

func TestWriteCompiledSystemBinaryRequiresPCS(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCompiledSystemBinary(&out, CompiledSystem{}); err == nil {
		t.Fatal("WriteCompiledSystemBinary accepted a system without PCS metadata")
	}
}

func TestWriteCompiledSystemBinaryIsVersionedAndDeterministic(t *testing.T) {
	system := CompiledSystem{Pcs: &PcsSystem{}}
	var first, second bytes.Buffer
	if err := WriteCompiledSystemBinary(&first, system); err != nil {
		t.Fatalf("first encoding: %v", err)
	}
	if err := WriteCompiledSystemBinary(&second, system); err != nil {
		t.Fatalf("second encoding: %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("encoding is not deterministic")
	}
	if !bytes.HasPrefix(first.Bytes(), []byte(compiledSystemBinaryMagic)) {
		t.Fatalf("encoding %x does not start with schema magic %q", first.Bytes(), compiledSystemBinaryMagic)
	}
}

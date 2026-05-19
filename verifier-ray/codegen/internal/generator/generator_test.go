package generator_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/verifier-ray/codegen/internal/generator"
)

func TestGenerate_DefaultEntryPoint(t *testing.T) {
	var buf bytes.Buffer
	err := generator.Generate(generator.System{
		Rounds: []generator.Round{
			{ID: 0, VerifierActions: []string{"commitment"}},
			{ID: 1, VerifierActions: []string{"quotient"}},
		},
	}, generator.Options{}, &buf)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	src := buf.String()
	if !strings.Contains(src, "pub fn verifyGenerated") {
		t.Fatalf("generated source is missing default entry point:\n%s", src)
	}
	if got := strings.Count(src, "rt.advanceRound();"); got != 2 {
		t.Fatalf("expected one advance per round, got %d:\n%s", got, src)
	}
}

func TestGenerate_CustomEntryPoint(t *testing.T) {
	var buf bytes.Buffer
	err := generator.Generate(generator.System{}, generator.Options{EntryPoint: "verifyFib"}, &buf)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if !strings.Contains(buf.String(), "pub fn verifyFib") {
		t.Fatalf("generated source is missing custom entry point:\n%s", buf.String())
	}
}

package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

func TestBuildDummyHonestArtifacts(t *testing.T) {
	artifacts, err := BuildDummyHonestArtifacts()
	if err != nil {
		t.Fatalf("BuildDummyHonestArtifacts: %v", err)
	}
	if len(artifacts.VerifyInput.Proof.Rounds) == 0 {
		t.Fatal("VerifyInput.Proof.Rounds is empty")
	}
	if len(artifacts.VerifyInput.PublicInputs) != DummyNumPublicInputs {
		t.Fatalf("got %d public inputs, want %d", len(artifacts.VerifyInput.PublicInputs), DummyNumPublicInputs)
	}

	encoded, err := proofserialization.Encode(artifacts.VerifyInput, proofserialization.GuestBase)
	if err != nil {
		t.Fatalf("proofserialization.Encode: %v", err)
	}
	t.Logf("single dummy proof image size: %d bytes", len(encoded))

	decoded, err := proofserialization.Decode(encoded, proofserialization.GuestBase)
	if err != nil {
		t.Fatalf("proofserialization.Decode: %v", err)
	}
	if len(decoded.Proof.Rounds) != len(artifacts.VerifyInput.Proof.Rounds) {
		t.Fatalf("round-tripped proof has %d rounds, want %d", len(decoded.Proof.Rounds), len(artifacts.VerifyInput.Proof.Rounds))
	}

	pairEncoded, err := EncodeAggregatorPair(artifacts.VerifyInput, artifacts.VerifyInput, proofserialization.GuestBase)
	if err != nil {
		t.Fatalf("EncodeAggregatorPair: %v", err)
	}
	t.Logf("pair dummy proof image size: %d bytes", len(pairEncoded))
}

func TestDummyVerifyRejectsChangedPublicInput(t *testing.T) {
	sys, err := NewDummySystem()
	if err != nil {
		t.Fatalf("NewDummySystem: %v", err)
	}
	proof, pub, err := ProveDummy(sys, SampleDummyPublicInputs())
	if err != nil {
		t.Fatalf("ProveDummy: %v", err)
	}

	changed := append(wiop.PublicInput(nil), pub...)
	changed[3] = changed[3].Add(field.ElemOne())
	if err := sys.Verify(proof, changed); err == nil {
		t.Fatal("expected Verify to reject a mutated public input")
	}
}

func BenchmarkDummyProve(b *testing.B) {
	sys, err := NewDummySystem()
	if err != nil {
		b.Fatal(err)
	}
	pub := SampleDummyPublicInputs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := ProveDummy(sys, pub); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDummyVerify(b *testing.B) {
	sys, err := NewDummySystem()
	if err != nil {
		b.Fatal(err)
	}
	proof, pub, err := ProveDummy(sys, SampleDummyPublicInputs())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if err := sys.Verify(proof, pub); err != nil {
			b.Fatal(err)
		}
	}
}

package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

func TestBuildDevmodeHonestArtifacts(t *testing.T) {
	artifacts, err := BuildDevmodeHonestArtifacts()
	if err != nil {
		t.Fatalf("BuildDevmodeHonestArtifacts: %v", err)
	}
	if len(artifacts.VerifyInput.Proof.Rounds) == 0 {
		t.Fatal("VerifyInput.Proof.Rounds is empty")
	}
	if len(artifacts.VerifyInput.PublicInputs) != devmode.NumPublicInputs {
		t.Fatalf("got %d public inputs, want %d", len(artifacts.VerifyInput.PublicInputs), devmode.NumPublicInputs)
	}

	encoded, err := proofserialization.Encode(artifacts.VerifyInput, proofserialization.GuestBase)
	if err != nil {
		t.Fatalf("proofserialization.Encode: %v", err)
	}
	t.Logf("single devmode proof image size: %d bytes", len(encoded))

	decoded, err := proofserialization.Decode(encoded, proofserialization.GuestBase)
	if err != nil {
		t.Fatalf("proofserialization.Decode: %v", err)
	}
	if len(decoded.Proof.Rounds) != len(artifacts.VerifyInput.Proof.Rounds) {
		t.Fatalf("round-tripped proof has %d rounds, want %d", len(decoded.Proof.Rounds), len(artifacts.VerifyInput.Proof.Rounds))
	}
}

func TestDevmodeVerifyRejectsChangedPublicInput(t *testing.T) {
	sys, err := devmode.NewSystem()
	if err != nil {
		t.Fatalf("devmode.NewSystem: %v", err)
	}
	proof, pub, err := devmode.Prove(sys, SampleDevmodePublicInputs())
	if err != nil {
		t.Fatalf("devmode.Prove: %v", err)
	}

	changed := append(wiop.PublicInput(nil), pub...)
	changed[3] = changed[3].Add(field.ElemOne())
	if err := devmode.Verify(sys, proof, changed); err == nil {
		t.Fatal("expected Verify to reject a mutated public input")
	}
}

func BenchmarkDevmodeProve(b *testing.B) {
	sys, err := devmode.NewSystem()
	if err != nil {
		b.Fatal(err)
	}
	pub := SampleDevmodePublicInputs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := devmode.Prove(sys, pub); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDevmodeVerify(b *testing.B) {
	sys, err := devmode.NewSystem()
	if err != nil {
		b.Fatal(err)
	}
	proof, pub, err := devmode.Prove(sys, SampleDevmodePublicInputs())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if err := devmode.Verify(sys, proof, pub); err != nil {
			b.Fatal(err)
		}
	}
}

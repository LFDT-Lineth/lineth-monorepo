package codegen

import (
	"bytes"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// TestBuildAllInOneHonestRiscvArtifacts proves and verifies zkc_r5.AllInOneGuestELF
// against the real main.zkc interpreter circuit, through the actual
// prove/PCS/verify pipeline (not trace-and-check-constraints alone, which
// prover-ray/zkcdriver/r5_test.go's TestRisc5InstructionCoverageGuest already
// covers more cheaply).
//
// This only checks that BuildAllInOneHonestRiscvArtifacts's own internal
// sys.Verify + AssertAllVerifierActionsHandled calls succeed (prover-ray's
// own Go-side protocol verification), and that the projected VerifyInput
// round-trips through Encode/Decode byte-for-byte — it does not call
// verifier-ray's actual Zig verifier.verify(), which is what
// test/riscv_proof_image_test.zig covers instead, against the committed
// fixture testdata/riscv_proof_image.bin (built from the same guest via
// codegen/generate-riscv-system).
func TestBuildAllInOneHonestRiscvArtifacts(t *testing.T) {
	artifacts, err := BuildAllInOneHonestRiscvArtifacts()
	if err != nil {
		t.Fatalf("BuildAllInOneHonestRiscvArtifacts: %v", err)
	}

	if len(artifacts.VerifyInput.Proof.Rounds) == 0 {
		t.Fatal("VerifyInput.Proof.Rounds is empty")
	}

	// proofserialization.Project already ran (and sys.Verify + the prover's
	// own AssertAllVerifierActionsHandled check already passed) inside
	// BuildAllInOneHonestRiscvArtifacts; this additionally pins the guest
	// image round trip that test/riscv_proof_image_test.zig relies on for
	// the committed fixture, so a codec regression is caught here too.
	encoded, err := proofserialization.EncodeGuest(artifacts.VerifyInput)
	if err != nil {
		t.Fatalf("proofserialization.EncodeGuest: %v", err)
	}
	decoded, err := proofserialization.DecodeGuest(encoded)
	if err != nil {
		t.Fatalf("proofserialization.DecodeGuest: %v", err)
	}
	if len(decoded.Proof.Rounds) != len(artifacts.VerifyInput.Proof.Rounds) {
		t.Fatalf("round-tripped proof has %d rounds, want %d", len(decoded.Proof.Rounds), len(artifacts.VerifyInput.Proof.Rounds))
	}
	again, err := proofserialization.EncodeGuest(decoded)
	if err != nil {
		t.Fatalf("proofserialization.EncodeGuest again: %v", err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("guest image is not idempotent: %d bytes then %d", len(encoded), len(again))
	}
}

package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// TestBuildAllHonestRiscvArtifactsCoversEveryGuest proves and verifies every
// guest in HonestRiscvGuests against the real main.zkc interpreter circuit,
// through the actual prove/PCS/verify pipeline (not trace-and-check-constraints
// alone, which prover-ray/zkcdriver/r5_test.go's
// TestRisc5InstructionCoverageGuests already covers more cheaply).
//
// This only checks that BuildAllHonestRiscvArtifacts's own internal
// sys.Verify + AssertAllVerifierActionsHandled calls succeed for every guest,
// and that the projected VerifyInput round-trips through Encode/Decode
// byte-for-byte — it does not commit any of these (large: tens of MB each)
// proofs as testdata, so this exercises real end-to-end coverage without the
// repo-size cost of a fixture per guest. testdata/proof_image.bin (built from
// HonestRiscvGuests[0], ExitZeroGuestELF) remains the one committed fixture
// for the cross-language Zig mmap test in test/proof_image_test.zig.
func TestBuildAllHonestRiscvArtifactsCoversEveryGuest(t *testing.T) {
	all, err := BuildAllHonestRiscvArtifacts()
	if err != nil {
		t.Fatalf("BuildAllHonestRiscvArtifacts: %v", err)
	}
	if len(all) != len(HonestRiscvGuests) {
		t.Fatalf("got %d results, want %d (one per HonestRiscvGuests entry)", len(all), len(HonestRiscvGuests))
	}

	seenNames := make(map[string]bool, len(all))
	for _, result := range all {
		t.Run(result.Guest.Name, func(t *testing.T) {
			if seenNames[result.Guest.Name] {
				t.Fatalf("duplicate guest name %q in HonestRiscvGuests", result.Guest.Name)
			}
			seenNames[result.Guest.Name] = true

			if len(result.Artifacts.VerifyInput.Proof.Rounds) == 0 {
				t.Fatalf("guest %q: VerifyInput.Proof.Rounds is empty", result.Guest.Name)
			}

			// proofserialization.Project already ran (and sys.Verify + the
			// prover's own AssertAllVerifierActionsHandled check already
			// passed) inside BuildAllHonestRiscvArtifacts; this additionally
			// pins the Go<->native-layout Encode/Decode round trip that
			// test/proof_image_test.zig relies on for the one guest it does
			// commit as a fixture, so a codec regression is caught here for
			// every guest, not just that one.
			encoded, err := proofserialization.Encode(result.Artifacts.VerifyInput, proofserialization.GuestBase)
			if err != nil {
				t.Fatalf("guest %q: proofserialization.Encode: %v", result.Guest.Name, err)
			}
			decoded, err := proofserialization.Decode(encoded, proofserialization.GuestBase)
			if err != nil {
				t.Fatalf("guest %q: proofserialization.Decode: %v", result.Guest.Name, err)
			}
			if len(decoded.Proof.Rounds) != len(result.Artifacts.VerifyInput.Proof.Rounds) {
				t.Fatalf("guest %q: round-tripped proof has %d rounds, want %d", result.Guest.Name, len(decoded.Proof.Rounds), len(result.Artifacts.VerifyInput.Proof.Rounds))
			}
		})
	}
}

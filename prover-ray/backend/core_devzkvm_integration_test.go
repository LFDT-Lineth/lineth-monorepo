package backend

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDevZkVM_Integration runs dev-zkvm on the real l2-execution guest and the
// guest's stateless_input.ssz block file, and checks the public inputs decoded
// from the guest output. NATIVE_RUNNER sets a native runner: the guest output
// must then equal its --ssz output byte for byte.
//
// Gated on REAL_GUEST_TESTS=1, like TestDevPartial_Integration. GUEST_ELF
// overrides the guest ELF:
//
//	make -C riscv-guests/l2_execution compile
//	REAL_GUEST_TESTS=1 go test ./backend -run TestDevZkVM_Integration -v
func TestDevZkVM_Integration(t *testing.T) {
	dir, id := realGuest(t)
	core, err := New(Config{Mode: ProverModeDevZkVM, GuestsDir: dir})
	require.NoError(t, err, "building dev-zkvm core")

	input := blockPath("stateless_input.ssz")
	extended, err := os.ReadFile(input) //nolint:gosec // G703: test-controlled path
	require.NoError(t, err, "reading block file")
	result := core.Prove(context.Background(), Job{ID: "it-1", Type: ProofTypeL2Execution, GuestProgramID: id, Payload: extended})
	require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
	t.Logf("public inputs %+v", result.PublicInputs)

	assert.Equal(t, uint64(1), result.PublicInputs.BlockCount, "a block file holds one block")
	assert.Equal(t, DevMarkerProof(ProverModeDevZkVM), result.ProofBytes)

	native := os.Getenv("NATIVE_RUNNER")
	if native == "" {
		t.Log("NATIVE_RUNNER not set: the guest output is not checked against the native runner")
		return
	}
	nativeSSZ, err := exec.Command(native, input, "--ssz").Output() //nolint:gosec // G204: test-controlled path
	require.NoError(t, err, "native runner --ssz")
	assert.True(t, bytes.Equal(result.GuestOutput, nativeSSZ),
		"guest output %x != native output %x", result.GuestOutput, nativeSSZ)
}

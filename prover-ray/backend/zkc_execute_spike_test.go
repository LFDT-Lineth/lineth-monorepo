package backend

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-artifact integration test for dev-zkvm (grew out of the Stage 0 Execute
// spike). It builds a dev-zkvm Core, runs the guest under ZkC Execute, and
// asserts its 0x0003 commitment is byte-identical to the native runner's --ssz
// output — the cross-check invariant runL2ExecutionZkVM relies on.
//
// Gated on RISCV_SPIKE=1: it compiles the R5 arithmetization (~5s) and needs the
// built guest ELF and native runner:
//
//	make -C riscv-guests/l2-execution compile
//	RISCV_SPIKE=1 go test ./backend -run TestDevZkVM_Integration -v
func TestDevZkVM_Integration(t *testing.T) {
	if os.Getenv("RISCV_SPIKE") == "" {
		t.Skip("set RISCV_SPIKE=1 to run the dev-zkvm integration test (heavy: compiles R5 arithmetization)")
	}

	const (
		guestELF  = "../../riscv-guests/l2-execution/zig-out/bin/evm_execution_guest"
		nativeBin = "../../riscv-guests/l2-execution/zig-out/bin/l2-execution-runner"
		sszInput  = "../../riscv-guests/l2-execution/test/testdata/stateless_input.ssz"
	)

	core, err := New(Config{Mode: ProverModeDevZkVM, GuestELFPath: guestELF})
	require.NoError(t, err, "building dev-zkvm core")

	extended, err := os.ReadFile(sszInput)
	require.NoError(t, err, "reading extended input fixture")
	result := core.Prove(context.Background(), Job{ID: "it-1", Type: ProofTypeL2Execution, Payload: extended})
	require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)

	require.Len(t, result.ProofBytes, guestOutputSize)
	assert.Equal(t, []byte{0x00, 0x03}, result.ProofBytes[:2], "0x0003 schema id")
	t.Logf("guest_output = %s", hex.EncodeToString(result.ProofBytes))

	// The native runner must commit to the same public inputs.
	nativeSSZ, err := exec.Command(nativeBin, sszInput, "--ssz").Output()
	require.NoError(t, err, "native runner --ssz")
	assert.True(t, bytes.Equal(result.ProofBytes, nativeSSZ),
		"cross-check: guest %x != native %x", result.ProofBytes, nativeSSZ)
}

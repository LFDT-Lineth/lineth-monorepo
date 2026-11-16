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

// defaultNativeRunner is the native runner built by
// `make -C riscv-guests/l2_execution compile`.
const defaultNativeRunner = "../../riscv-guests/l2_execution/zig-out/bin/l2_execution_runner"

// statelessInputPublicInputs are the public inputs of the guest's
// stateless_input.ssz block file, from `l2_execution_runner --json`; capture
// them again when the block file or the guest's output changes.
func statelessInputPublicInputs(t *testing.T) PublicInputs {
	t.Helper()
	hash := func(s string) (h [32]byte) {
		b, err := hex.DecodeString(s)
		require.NoError(t, err)
		require.Len(t, b, len(h))
		copy(h[:], b)
		return h
	}
	return PublicInputs{
		ParentBlockHash:          hash("ee93f90395f9384466a618018575f933dca8fa630afbcc6c048b2381a7d72af9"),
		EndBlockHash:             hash("b6107b07672ca66bd702a53c5105925002037e593f46c8cecba5af3425496d64"),
		EndBlockNumber:           1,
		EndBlockTimestamp:        12,
		DynamicChainConfigHash:   hash("4c87eca77cb88c24f2de035ddac21297991d5a1946b9e7d6a2b25375037715fa"),
		TxFromsHash:              hash("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
		BlockCount:               1,
		FilteredAddresses:        [][20]byte{},
		L2MessagingBlocksOffsets: []uint16{},
		L2L1Messages:             [][32]byte{},
	}
}

// TestDevZkVM_Integration runs dev-zkvm on the real l2-execution guest and the
// guest's stateless_input.ssz block file. The public inputs decoded from the
// guest output must be the block's, and the guest output must equal the native
// runner's --ssz output byte for byte.
//
// Gated on REAL_GUEST_TESTS=1, like TestDevPartial_Integration. GUEST_ELF
// overrides the guest ELF and NATIVE_RUNNER the native runner; with GUEST_ELF
// set and no native runner, the native runner check is skipped:
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

	assert.Equal(t, statelessInputPublicInputs(t), result.PublicInputs,
		"the public inputs decoded from the guest output should be the block's")
	assert.Equal(t, DevMarkerProof(ProverModeDevZkVM), result.ProofBytes)

	native := os.Getenv("NATIVE_RUNNER")
	if native == "" {
		if os.Getenv("GUEST_ELF") != "" {
			t.Log("GUEST_ELF set without NATIVE_RUNNER: the guest output is not checked against the native runner")
			return
		}
		native = defaultNativeRunner
	}
	nativeSSZ, err := exec.Command(native, input, "--ssz").Output() //nolint:gosec // G204: test-controlled path
	require.NoError(t, err, "native runner --ssz")
	assert.True(t, bytes.Equal(result.GuestOutput, nativeSSZ),
		"guest output %x != native output %x", result.GuestOutput, nativeSSZ)
}

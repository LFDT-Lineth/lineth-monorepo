package backend

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guestTestdata holds the l2-execution guest's block files: extended 0x0002
// inputs, named after the gas the block uses.
const guestTestdata = "../../riscv-guests/l2_execution/test/testdata"

// TestPartial_Integration runs partial on the real l2-execution guest and the
// guest's block files: it traces each block, checks every constraint and
// decodes the public inputs.
//
// Gated on RISCV_SPIKE=1, like TestDevZkVM_Integration. GUEST_ELF overrides the
// guest ELF, NATIVE_RUNNER sets a native runner to check the guest output
// against, PARTIAL_SHARD_SIZE sets the shard size (default 500000, as the guest
// trace CI and the zkcdriver benchmarks) and PARTIAL_BLOCKS picks the block
// files (comma separated, default stateless_input.ssz):
//
//	make -C riscv-guests/l2_execution compile
//	RISCV_SPIKE=1 PARTIAL_BLOCKS=stateless_input.ssz,block_amsterdam_175k.ssz \
//		go test ./backend -run TestPartial_Integration -v -timeout 0
func TestPartial_Integration(t *testing.T) {
	if os.Getenv("RISCV_SPIKE") == "" {
		t.Skip("set RISCV_SPIKE=1 to run the partial integration test (heavy: traces real blocks)")
	}

	elfPath := os.Getenv("GUEST_ELF")
	if elfPath == "" {
		elfPath = "../../riscv-guests/l2_execution/zig-out/bin/evm_execution_guest"
	}
	elf, err := os.ReadFile(elfPath) //nolint:gosec // G703: test-controlled path
	require.NoError(t, err, "reading guest ELF")
	dir := t.TempDir()
	id := writeGuest(t, dir, elf)
	// A real block does not fit in one shard: some module has more rows than
	// a column can hold.
	shardSize := uint64(500000)
	if s := os.Getenv("PARTIAL_SHARD_SIZE"); s != "" {
		shardSize, err = strconv.ParseUint(s, 10, 64)
		require.NoError(t, err, "parsing PARTIAL_SHARD_SIZE")
	}
	core, err := New(Config{Mode: ProverModePartial, GuestsDir: dir, ShardSize: shardSize})
	require.NoError(t, err, "building partial core")

	blocks := "stateless_input.ssz"
	if b := os.Getenv("PARTIAL_BLOCKS"); b != "" {
		blocks = b
	}
	for _, name := range strings.Split(blocks, ",") {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(guestTestdata, name)
			extended, err := os.ReadFile(input) //nolint:gosec // G703: test-controlled path
			require.NoError(t, err, "reading block file")

			start := time.Now()
			result := core.Prove(context.Background(), Job{ID: name, Type: ProofTypeL2Execution, GuestProgramID: id, Payload: extended})
			require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
			t.Logf("partial took %s, public inputs %+v", time.Since(start), result.PublicInputs)

			assert.Equal(t, uint64(1), result.PublicInputs.BlockCount, "a block file holds one block")
			assert.Equal(t, devmodeImage(t, result.GuestOutput[:guestOutputHashSize]), result.ProofBytes,
				"the proof should be a devmode proof of the output hash")

			if native := os.Getenv("NATIVE_RUNNER"); native != "" {
				nativeSSZ, err := exec.Command(native, input, "--ssz").Output() //nolint:gosec // G204: test-controlled path
				require.NoError(t, err, "native runner --ssz")
				assert.True(t, bytes.Equal(result.GuestOutput, nativeSSZ),
					"guest output %x != native output %x", result.GuestOutput, nativeSSZ)
			}
		})
	}
}

package backend

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rollupGuestOut holds the rollup guest ELF, and the sample request (real devmode
// proofs) and native output that `make -C riscv-guests/rollup require-input` writes.
const rollupGuestOut = "../../riscv-guests/rollup/zig-out"

// TestRollup_Integration proves the real rollup guest on its sample request.
//
// Gated on REAL_GUEST_TESTS=1. ROLLUP_MODES picks the modes (default
// dev-zkvm,dev-partial), DEV_PARTIAL_SHARD_SIZE the shard size (default 500000):
//
//	make -C riscv-guests/rollup compile require-input
//	REAL_GUEST_TESTS=1 go test ./backend -run TestRollup_Integration -v -timeout 0
func TestRollup_Integration(t *testing.T) {
	if os.Getenv("REAL_GUEST_TESTS") == "" {
		t.Skip("set REAL_GUEST_TESTS=1 to run the real guest tests (heavy: runs real blocks)")
	}
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(rollupGuestOut, name)) //nolint:gosec // G703: test-controlled path
		require.NoError(t, err, "reading %s (run make -C riscv-guests/rollup compile require-input)", name)
		return b
	}
	elf, request, want := read("bin/rollup_guest"), read("sample.request.ssz"), read("sample.output.bin")
	wantPI, err := decodeRollupPublicInputs(want[guestOutputHashSize:])
	require.NoError(t, err, "decoding the native output")

	dir := t.TempDir()
	id := writeGuest(t, dir, elf)
	shardSize := uint64(500000)
	if s := os.Getenv("DEV_PARTIAL_SHARD_SIZE"); s != "" {
		shardSize, err = strconv.ParseUint(s, 10, 64)
		require.NoError(t, err, "parsing DEV_PARTIAL_SHARD_SIZE")
	}
	modes := "dev-zkvm,dev-partial"
	if m := os.Getenv("ROLLUP_MODES"); m != "" {
		modes = m
	}

	for _, mode := range strings.Split(modes, ",") {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{Mode: ProverMode(mode), GuestsDir: dir}
			if cfg.Mode == ProverModeDevPartial {
				cfg.ShardSize = shardSize
			}
			core, err := New(cfg)
			require.NoError(t, err, "building the %s core", mode)

			start := time.Now()
			result := core.Prove(context.Background(),
				Job{ID: "rollup", Type: ProofTypeRollup, GuestProgramID: id, Payload: request})
			require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
			t.Logf("%s took %s, public inputs %+v", mode, time.Since(start), result.RollupPublicInputs)

			assert.Equal(t, want, result.GuestOutput, "the guest output should equal the native one")
			assert.Equal(t, wantPI, result.RollupPublicInputs)
			assert.Equal(t, devmodeImage(t, want[:guestOutputHashSize]), result.ProofBytes,
				"the proof should be a devmode proof of the output hash")
		})
	}
}

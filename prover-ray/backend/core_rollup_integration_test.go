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

// rollupGuestOut holds the rollup guest's build output: its ELF, and the sample
// request and output `make -C riscv-guests/rollup require-input` writes. The
// request carries real devmode proofs; the output is the one the rollup
// statement computes natively for it (hash || frame).
const rollupGuestOut = "../../riscv-guests/rollup/zig-out"

// TestRollup_Integration runs the real rollup guest on its sample request: the
// guest verifies the request's devmode proofs, and its output must equal the
// native one. It also logs how long each mode takes.
//
// Gated on REAL_GUEST_TESTS=1, like TestDevPartial_Integration. ROLLUP_MODES
// picks the modes (comma separated, default dev-zkvm,dev-partial) and
// DEV_PARTIAL_SHARD_SIZE the dev-partial shard size (default 500000):
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

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

// aggregationGuest holds the rollup-aggregation guest: the ELF that
// `make -C riscv-guests/rollup_aggregation compile` writes, and the committed
// sample request (real devmode proofs) and its output.
const aggregationGuest = "../../riscv-guests/rollup_aggregation"

// TestAggregation_Integration proves the real rollup-aggregation guest on its
// sample request.
//
// Gated on REAL_GUEST_TESTS=1. AGGREGATION_MODES picks the modes (default
// dev-zkvm,dev-partial), DEV_PARTIAL_SHARD_SIZE the shard size (default 500000):
//
//	make -C riscv-guests/rollup_aggregation compile
//	REAL_GUEST_TESTS=1 go test ./backend -run TestAggregation_Integration -v -timeout 0
func TestAggregation_Integration(t *testing.T) {
	if os.Getenv("REAL_GUEST_TESTS") == "" {
		t.Skip("set REAL_GUEST_TESTS=1 to run the real guest tests (heavy: runs real blocks)")
	}
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(aggregationGuest, name)) //nolint:gosec // G703: test-controlled path
		require.NoError(t, err, "reading %s (run make -C riscv-guests/rollup_aggregation compile)", name)
		return b
	}
	elf := read("zig-out/bin/rollup_aggregation_guest")
	request, want := read("test/testdata/sample.request.ssz"), read("test/testdata/sample.output.bin")
	wantPI, err := decodeAggregationPublicInputs(want[guestOutputHashSize:])
	require.NoError(t, err, "decoding the sample output")

	dir := t.TempDir()
	id := writeGuest(t, dir, elf)
	shardSize := uint64(500000)
	if s := os.Getenv("DEV_PARTIAL_SHARD_SIZE"); s != "" {
		shardSize, err = strconv.ParseUint(s, 10, 64)
		require.NoError(t, err, "parsing DEV_PARTIAL_SHARD_SIZE")
	}
	modes := "dev-zkvm,dev-partial"
	if m := os.Getenv("AGGREGATION_MODES"); m != "" {
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
				Job{ID: "aggregation", Type: ProofTypeRollupAggregation, GuestProgramID: id, Payload: request})
			require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
			t.Logf("%s took %s, public inputs %+v", mode, time.Since(start), result.AggregationPublicInputs)

			assert.Equal(t, want, result.GuestOutput, "the guest output should equal the sample output")
			assert.Equal(t, wantPI, result.AggregationPublicInputs)
			assert.Len(t, result.ProofBytes, solidityPLONKProofSize, "the proof should be a gnark PLONK proof for L1")
		})
	}
}

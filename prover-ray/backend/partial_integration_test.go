package backend

import (
	"bytes"
	"cmp"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guestTestdata holds the l2-execution guest's block files: extended 0x0002
// inputs, named after the gas the block uses.
const guestTestdata = "../../riscv-guests/l2_execution/test/testdata"

// TestDevPartial_Integration runs dev-partial on the real l2-execution guest and the
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
//		go test ./backend -run TestDevPartial_Integration -v -timeout 0
func TestDevPartial_Integration(t *testing.T) {
	core, id, blocks := realGuestCore(t)
	for _, name := range blocks {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(guestTestdata, name)
			extended, err := os.ReadFile(input) //nolint:gosec // G703: test-controlled path
			require.NoError(t, err, "reading block file")

			start := time.Now()
			result := core.Prove(context.Background(), Job{ID: name, Type: ProofTypeL2Execution, GuestProgramID: id, Payload: extended})
			require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
			t.Logf("dev-partial took %s, public inputs %+v", time.Since(start), result.PublicInputs)

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

// realGuestCore builds a dev-partial Core for the real l2-execution guest, with
// the settings of the environment variables of [TestDevPartial_Integration], and
// returns the guest's Program ID and the block files to run.
func realGuestCore(t *testing.T) (*Core, [32]byte, []string) {
	t.Helper()
	if os.Getenv("RISCV_SPIKE") == "" {
		t.Skip("set RISCV_SPIKE=1 to run the real guest tests (heavy: traces real blocks)")
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
	core, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir, ShardSize: shardSize})
	require.NoError(t, err, "building dev-partial core")
	blocks := "stateless_input.ssz"
	if b := os.Getenv("PARTIAL_BLOCKS"); b != "" {
		blocks = b
	}
	return core, id, strings.Split(blocks, ",")
}

// TestDevPartial_ShardRows logs the row count of the largest modules of every
// shard of the block files, against the column limit
// [wiop.ColumnSizeMaxSupported]. A shard with a module over the limit cannot
// be assigned. It only traces, it checks nothing. Same settings as
// [TestDevPartial_Integration]; SHARD_ROWS_FIRST=1 stops after shard 0:
//
//	RISCV_SPIKE=1 PARTIAL_BLOCKS=block_amsterdam_175k.ssz \
//		go test ./backend -run TestDevPartial_ShardRows -v -timeout 0
func TestDevPartial_ShardRows(t *testing.T) {
	core, id, blocks := realGuestCore(t)
	g, err := core.loadGuest(id)
	require.NoError(t, err)
	for _, name := range blocks {
		t.Run(name, func(t *testing.T) {
			extended, err := os.ReadFile(filepath.Join(guestTestdata, name)) //nolint:gosec // G703: test-controlled path
			require.NoError(t, err, "reading block file")
			inputs, err := core.encodeGuestInputs(Job{Payload: extended}, g)
			require.NoError(t, err)
			start := time.Now()
			lazyTrace, err := core.driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs})
			require.NoError(t, err, "tracing")
			numShards := lazyTrace.Len()
			t.Logf("%d shards, fast pass took %s", numShards, time.Since(start))
			if os.Getenv("SHARD_ROWS_FIRST") != "" {
				numShards = 1
			}
			for i := range numShards {
				shard, err := traceShard(lazyTrace, i)
				require.NoError(t, err)
				logLargestModules(t, i, shard)
			}
		})
	}
}

// logLargestModules logs the five largest modules of the shard that are
// assigned per shard: static modules, such as the $bit tables, are not.
func logLargestModules(t *testing.T, i uint, shard zkcdriver.Shard) {
	t.Helper()
	type module struct {
		name   string
		height uint
	}
	var modules []module
	for k := range shard.Width() {
		m := shard.Module(k)
		if strings.HasPrefix(m.Name(), "$") {
			continue
		}
		modules = append(modules, module{m.Name(), m.Height()})
	}
	slices.SortFunc(modules, func(a, b module) int { return cmp.Compare(b.height, a.height) })
	for _, m := range modules[:min(5, len(modules))] {
		over := ""
		if m.height > wiop.ColumnSizeMaxSupported {
			over = "  OVER THE LIMIT"
		}
		t.Logf("shard %d: %-30s %9d rows (limit %d)%s", i, m.name, m.height, wiop.ColumnSizeMaxSupported, over)
	}
}

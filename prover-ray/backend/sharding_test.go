package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/stretchr/testify/require"
)

// shardedTestCore builds a Core with the all-in-one minimal ELF as its only
// guest and the given shard size, and returns the guest's Program ID. The guest
// runs ~300 instructions, so a shard size of 100 yields 3 shards.
func shardedTestCore(t *testing.T, shardSize uint64) (*Core, [32]byte) {
	t.Helper()
	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	binBytes, err := binF.MarshalBinary()
	require.NoError(t, err)
	binPath := filepath.Join(t.TempDir(), "circuit.bin")
	require.NoError(t, os.WriteFile(binPath, binBytes, 0o600))

	dir := t.TempDir()
	elf, _ := minimal_elf.AllInOneElfProgram()
	id := writeGuest(t, dir, elf)

	c, err := New(Config{CircuitBinPath: binPath, GuestsDir: dir, ShardSize: shardSize})
	require.NoError(t, err)
	return c, id
}

// runProveShards runs runProve on c for the given shards of the empty job of
// guest id.
func runProveShards(t *testing.T, ctx context.Context, c *Core, id [32]byte, indexes ...uint) ([]Node, uint, error) {
	t.Helper()
	g, err := c.loadGuest(id)
	require.NoError(t, err)
	inputs, err := c.buildInputs(Job{ShardIndexes: indexes}, g)
	require.NoError(t, err)
	return c.runProve(ctx, &zkcdriver.PreReadInputs{Inputs: inputs}, indexes)
}

// TestRunProve_ShardIndexes checks that runProve lazily traces and proves the
// requested shards of a sharded trace, and rejects shard indexes the trace
// does not have.
func TestRunProve_ShardIndexes(t *testing.T) {
	c, id := shardedTestCore(t, 100)
	ctx := context.Background()

	t.Run("all shards", func(t *testing.T) {
		nodes, numShards, err := runProveShards(t, ctx, c, id)
		require.NoError(t, err)
		require.Equal(t, uint(3), numShards, "the trace should have 3 shards")
		require.Len(t, nodes, 3, "every shard should be proved")
		for i, node := range nodes {
			require.Equal(t, []uint{uint(i)}, node.Coverage, "shard proofs should be in shard order")
			require.NoError(t, c.sys.Verify(node.Proof, node.Pub), "shard %d", i)
		}
	})

	t.Run("strict subset", func(t *testing.T) {
		nodes, numShards, err := runProveShards(t, ctx, c, id, 1)
		require.NoError(t, err)
		require.Equal(t, uint(3), numShards)
		require.Len(t, nodes, 1, "only the requested shard should be proved")
		require.Equal(t, []uint{1}, nodes[0].Coverage)
	})

	t.Run("out of range", func(t *testing.T) {
		_, _, err := runProveShards(t, ctx, c, id, 3)
		require.ErrorContains(t, err, "shard index 3 out of range, the trace has 3 shards")
	})
}

// TestRunProve_Unsharded checks that without sharding the trace is a single
// shard and its proof verifies.
func TestRunProve_Unsharded(t *testing.T) {
	c, id := shardedTestCore(t, 0)
	nodes, numShards, err := runProveShards(t, context.Background(), c, id)
	require.NoError(t, err)
	require.Equal(t, uint(1), numShards, "an unsharded trace is a single shard")
	require.Len(t, nodes, 1)
	require.Equal(t, []uint{0}, nodes[0].Coverage)
	require.NoError(t, c.sys.Verify(nodes[0].Proof, nodes[0].Pub), "the shard proof should verify")
}

// TestRunProve_Cancelled checks that a cancelled context stops runProve before
// any shard is proved.
func TestRunProve_Cancelled(t *testing.T) {
	c, id := shardedTestCore(t, 100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := runProveShards(t, ctx, c, id)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "before shard 0")
}

// TestRunProve_TraceFailureIsAnError checks that a tracing failure fails the
// job instead of panicking the process.
func TestRunProve_TraceFailureIsAnError(t *testing.T) {
	c, _ := shardedTestCore(t, 100)
	_, _, err := c.runProve(context.Background(), &zkcdriver.PreReadInputs{Inputs: map[string]bit.Array{}}, nil)
	require.ErrorContains(t, err, "tracing:")
}

// TestProveFull_Sharded_NotImplemented checks that a sharded job is not
// accepted as proved until conglomeration exists.
func TestProveFull_Sharded_NotImplemented(t *testing.T) {
	c, id := shardedTestCore(t, 100)
	res := c.Prove(context.Background(), Job{ID: "job", GuestProgramID: id, ShardIndexes: []uint{1}})
	require.Equal(t, ResultStatusFailed, res.Status)
	require.ErrorIs(t, res.Err, ErrNotImplemented)
	require.Nil(t, res.ProofBytes)
}

// TestSanityCheckJobs_ShardIndexes checks the validation of [Job.ShardIndexes].
func TestSanityCheckJobs_ShardIndexes(t *testing.T) {
	cases := []struct {
		name      string
		indexes   []uint
		shardSize uint64
		wantErr   string
	}{
		{name: "empty", indexes: nil, shardSize: 0},
		{name: "unsorted", indexes: []uint{2, 0, 1}, shardSize: 10},
		{name: "non-zero without sharding", indexes: []uint{1}, shardSize: 0, wantErr: "non-zero shard size"},
		{name: "duplicate", indexes: []uint{3, 1, 3}, shardSize: 10, wantErr: "duplicate shard index 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sanityCheckJobs(Job{ShardIndexes: tc.indexes}, tc.shardSize)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

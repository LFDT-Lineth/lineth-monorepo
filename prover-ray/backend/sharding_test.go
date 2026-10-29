package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/require"
)

// TestRunProve_ShardIndexes checks that runProve lazily traces and proves the
// configured shard of a sharded trace, and rejects shard indexes the trace
// does not have.
func TestRunProve_ShardIndexes(t *testing.T) {
	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	binBytes, err := binF.MarshalBinary()
	require.NoError(t, err)
	binPath := filepath.Join(t.TempDir(), "circuit.bin")
	require.NoError(t, os.WriteFile(binPath, binBytes, 0o600))

	elf, _ := minimal_elf.AllInOneElfProgram()
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elf, 0o600))

	// The guest runs ~300 instructions, so this yields 3 shards.
	const shardSize = 100

	prove := func(indexes ...uint) (wiop.Proof, wiop.PublicInput, error) {
		c, err := New(Config{
			CircuitBinPath: binPath,
			GuestELFPath:   elfPath,
			ShardSize:      shardSize,
			ShardIndexes:   indexes,
		})
		require.NoError(t, err)
		inputs, err := c.buildInputs(Job{})
		require.NoError(t, err)
		return c.runProve(context.Background(), &zkcdriver.PreReadInputs{Inputs: inputs})
	}

	// Every shard must be provable.
	_, _, err = prove(0, 1, 2)
	require.NoError(t, err)

	_, _, err = prove(3)
	require.ErrorContains(t, err, "shard index 3 out of range, the trace has 3 shards")
}

// TestNew_ShardIndexes checks the validation of [Config.ShardIndexes].
func TestNew_ShardIndexes(t *testing.T) {
	_, err := New(Config{ShardIndexes: []uint{1}})
	require.ErrorContains(t, err, "non-zero shard size")
	_, err = New(Config{ShardSize: 10, ShardIndexes: []uint{3, 1, 3}})
	require.ErrorContains(t, err, "duplicate shard index 3")
}

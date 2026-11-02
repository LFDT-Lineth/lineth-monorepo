package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
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

// TestConglomerate checks that conglomerate reduces any number of shard proofs,
// arriving in any order, to one carrying the product of their bus products,
// checks that product at the root, and fails if some proofs never arrive.
func TestConglomerate(t *testing.T) {
	elem := func(v uint) field.Gen { return field.ElemFromBase(field.NewElement(uint64(v))) }
	// The public inputs of a shard are (its bus product, program VK, guest output).
	layout := piLayout{bus: []int{0}, programVK: []int{1}, guestOutput: []int{2}}
	shard := func(i uint, bus field.Gen) shardProof {
		return shardProof{Index: i, Pub: wiop.PublicInput{bus, elem(7), elem(9)}}
	}
	send := func(isRoot bool, n int, shards ...shardProof) (shardProof, error) {
		ch := make(chan shardProof, len(shards))
		for _, s := range shards {
			ch <- s
		}
		close(ch)
		return conglomerate(ch, n, layout, isRoot)
	}
	run := func(n int, isRoot bool, arrivals ...uint) (shardProof, error) {
		shards := make([]shardProof, len(arrivals))
		for k, i := range arrivals {
			shards[k] = shard(i, elem(i+1))
		}
		return send(isRoot, n, shards...)
	}

	for _, arrivals := range [][]uint{{0}, {2, 0, 1}, {4, 1, 3, 0, 2}, {5, 6, 0, 2, 1, 3, 4}} {
		root, err := run(len(arrivals), false, arrivals...)
		require.NoError(t, err, "arrivals %v", arrivals)
		factorial := elem(1)
		for i := range arrivals {
			factorial = factorial.Mul(elem(uint(i) + 1))
		}
		require.Equal(t, uint(0), root.Index, "arrivals %v", arrivals)
		diff := root.Pub[0].Sub(factorial)
		require.True(t, diff.IsZero(), "arrivals %v", arrivals)
	}

	// The bus products 1, 2 and 1/2 balance; 1, 2 and 3 do not.
	_, err := send(true, 3, shard(0, elem(1)), shard(1, elem(2)), shard(2, elem(2).Inverse()))
	require.NoError(t, err)
	_, err = run(3, true, 0, 1, 2)
	require.ErrorContains(t, err, "does not balance across the shards")
	_, err = run(1, true, 0) // a single shard, with bus product 1, is its own root
	require.NoError(t, err)
	_, err = run(1, true, 1)
	require.ErrorContains(t, err, "does not balance across the shards")
	_, err = run(3, false, 0, 1, 2)
	require.NoError(t, err)

	// Shards of another program or with another output are rejected at any node.
	otherVK, otherOutput := shard(1, elem(2)), shard(1, elem(2))
	otherVK.Pub[1], otherOutput.Pub[2] = elem(8), elem(8)
	_, err = send(false, 2, shard(0, elem(1)), otherVK)
	require.ErrorContains(t, err, "shards 0 and 1 have different program VK")
	_, err = send(false, 2, shard(0, elem(1)), otherOutput)
	require.ErrorContains(t, err, "shards 0 and 1 have different guest output")

	_, err = run(3, false, 1, 0)
	require.ErrorContains(t, err, "only 2 of 3 shard proofs arrived")
}

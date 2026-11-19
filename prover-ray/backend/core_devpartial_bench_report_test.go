//go:build linux

package backend

import (
	"os/exec"
	"testing"

	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderBenchReport checks the report of a full block, a block stopped
// after some shards, with its estimated full time, and a killed block.
func TestRenderBenchReport(t *testing.T) {
	shard := func(i uint, sec float64, rows uint) benchShard {
		return benchShard{
			Index: i, Trace: benchStep{Sec: sec}, Check: benchStep{Sec: sec},
			Largest: []benchModule{{Name: "ram", Rows: rows}},
		}
	}
	recs := []benchBlock{
		{
			Block: "full.ssz", NumShards: 2, Result: benchPass, ProcessPeakGB: 40,
			Steps:       []benchStep{{Name: benchExecuteStep, Sec: 5}, {Name: benchFastPass, Sec: 60, PeakRSSGB: 30}},
			ShardsStats: []benchShard{shard(0, 10, 100), shard(1, 20, 300)},
		},
		{
			Block: "big.ssz", NumShards: 100, MaxShards: 2, Result: "stopped after 2 shards",
			Steps:       []benchStep{{Name: benchExecuteStep, Sec: 10}, {Name: benchFastPass, Sec: 90}},
			ShardsStats: []benchShard{shard(0, 30, 10), shard(1, 30, 10)},
		},
		{Block: "oom.ssz", Result: "killed by SIGKILL, most likely out of memory"},
		{
			Block: "par.ssz", NumShards: 2, ShardsRun: 2, Workers: 8, Result: benchPass,
			Steps:       []benchStep{{Name: benchExecuteStep, Sec: 5}, {Name: benchFastPass, Sec: 10}, {Name: benchShardsStep, Sec: 20}},
			ShardsStats: []benchShard{shard(0, 15, 100), shard(1, 15, 300)},
		},
	}
	report := renderBenchReport("Machine: test", recs)

	assert.Contains(t, report, "Machine: test")
	assert.Contains(t, report, "| full.ssz | 2 of 2 | pass |")
	assert.Contains(t, report, "40.0 GB")
	assert.Contains(t, report, "1m0s, 30.0 GB", "the fast pass")
	assert.Contains(t, report, "| 30s |", "the average time per shard: trace and check")
	assert.Contains(t, report, "ram 300 rows")
	// 10 + 90 + 60 s per shard for 100 shards.
	assert.Contains(t, report, "| big.ssz | 2 of 100 | stopped after 2 shards |")
	assert.Contains(t, report, "1h41m40s")
	assert.Contains(t, report, "| oom.ssz | 0 of 0 | killed by SIGKILL, most likely out of memory |")
	// One shard at a time unless the record says otherwise.
	assert.Contains(t, report, "| 0 | - | 1 |", "the full block, with 1 worker")
	// In parallel, the shards overlap: the time per shard is the shards step
	// over the shards, not the 30 s each shard took.
	assert.Contains(t, report, "| par.ssz | 2 of 2 | pass |")
	assert.Contains(t, report, "| 10s | ram 300 rows | 0 | - | 8 |")
}

// TestBenchDevPartial_Parallel checks that the benchmark records the shards
// the same way with several workers as one at a time: every shard, in order,
// with its size, and checked.
func TestBenchDevPartial_Parallel(t *testing.T) {
	hash, framed := referenceFrame(t)
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	run := func(workers int) benchBlock {
		c := parallelCore(t, dir, id, workers)
		g, err := c.loadGuest(id)
		require.NoError(t, err)
		var rec benchBlock
		rec.Result = benchDevPartial(t, c, g, devPartialJob(id).Payload, 0, workers, &rec, func() {})
		return rec
	}
	seq, par := run(1), run(4)

	require.Equal(t, benchPass, seq.Result)
	require.Equal(t, benchPass, par.Result)
	assert.False(t, seq.PCS, "dev-partial has no PCS")
	assert.Equal(t, 4, par.Workers)
	assert.Equal(t, seq.NumShards, par.ShardsRun)
	require.Len(t, par.ShardsStats, len(seq.ShardsStats))
	for i, s := range par.ShardsStats {
		want := seq.ShardsStats[i]
		assert.Equal(t, want.Index, s.Index)
		assert.Equal(t, want.Cells, s.Cells, "shard %d", i)
		assert.Equal(t, want.Largest, s.Largest, "shard %d", i)
		assert.Positive(t, s.Check.Sec, "shard %d should be checked", i)
	}
}

// TestBenchDevPartial_PCS checks that with BENCH_PCS the benchmark proves the
// shards with the PCS, as full does, and that the block still passes, with the
// same merge and public input checks as dev-partial.
func TestBenchDevPartial_PCS(t *testing.T) {
	hash, framed := referenceFrame(t)
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir, ShardSize: devPartialShardSize})
	require.NoError(t, err)
	c = benchPCSCore(t, c)
	g, err := c.loadGuest(id)
	require.NoError(t, err)

	var rec benchBlock
	rec.Result = benchDevPartial(t, c, g, devPartialJob(id).Payload, 0, 1, &rec, func() {})
	require.Equal(t, benchPass, rec.Result)
	assert.True(t, rec.PCS, "the record should say the shards were proved with the PCS")
	assert.Len(t, rec.ShardsStats, 2, "devPartialShardSize splits the guest into 2 shards")
	assert.Contains(t, renderBenchReport("Machine: test", []benchBlock{rec}), "PCS on")
}

// TestBenchShardWorkers checks the number of shards in progress at once: the
// workers asked for, no more than the shards, at least 1.
func TestBenchShardWorkers(t *testing.T) {
	assert.Equal(t, 10, benchShardWorkers(10, 272))
	assert.Equal(t, 4, benchShardWorkers(10, 4), "no more workers than shards")
	assert.Equal(t, 1, benchShardWorkers(0, 10), "at least one worker")
}

// TestBenchChildFailure checks how the end of a block process that wrote no
// result is reported.
func TestBenchChildFailure(t *testing.T) {
	killed := exec.Command("sh", "-c", "kill -9 $$").Run()
	require.Error(t, killed)
	assert.Equal(t, "killed by SIGKILL, most likely out of memory", benchChildFailure(killed))

	failed := exec.Command("sh", "-c", "exit 3").Run()
	assert.Equal(t, "failed (exit status 3), see the log", benchChildFailure(failed))

	assert.Equal(t, "did not finish, see the log", benchChildFailure(nil))
}

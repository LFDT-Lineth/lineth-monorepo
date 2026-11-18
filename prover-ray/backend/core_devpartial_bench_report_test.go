//go:build linux

package backend

import (
	"os/exec"
	"testing"

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
			Steps:       []benchStep{{Name: "execute", Sec: 5}, {Name: benchFastPass, Sec: 60, PeakRSSGB: 30}},
			ShardsStats: []benchShard{shard(0, 10, 100), shard(1, 20, 300)},
		},
		{
			Block: "big.ssz", NumShards: 100, MaxShards: 2, Result: "stopped after 2 shards",
			Steps:       []benchStep{{Name: "execute", Sec: 10}, {Name: benchFastPass, Sec: 90}},
			ShardsStats: []benchShard{shard(0, 30, 10), shard(1, 30, 10)},
		},
		{Block: "oom.ssz", Result: "killed by SIGKILL, most likely out of memory"},
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

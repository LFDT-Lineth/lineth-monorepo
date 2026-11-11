package backend

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/conglomeration"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/stretchr/testify/require"
)

// benchBlock is the benchmark record of one block file.
type benchBlock struct {
	Block       string       `json:"block"`
	InputBytes  int          `json:"inputBytes"`
	ShardSize   uint64       `json:"shardSize"`
	NumShards   uint         `json:"numShards"`
	Result      string       `json:"result"`
	TotalSec    float64      `json:"totalSec"`
	PeakRSSGB   float64      `json:"peakRssGb"`
	Steps       []benchStep  `json:"steps"`
	ShardsStats []benchShard `json:"shards"`
}

// benchStep is the time and peak resident memory of one step.
type benchStep struct {
	Name      string  `json:"name"`
	Sec       float64 `json:"sec"`
	PeakRSSGB float64 `json:"peakRssGb"`
}

// benchShard is the size of one shard and the cost of tracing and checking it.
type benchShard struct {
	Index     uint          `json:"index"`
	Trace     benchStep     `json:"trace"`
	Check     benchStep     `json:"check"`
	Cells     uint64        `json:"cells"`
	Largest   []benchModule `json:"largest"`
	OverLimit []string      `json:"overLimit,omitempty"`
}

// benchModule is the size of one module of a shard.
type benchModule struct {
	Name    string `json:"name"`
	Rows    uint   `json:"rows"`
	Columns uint   `json:"columns"`
}

// TestDevPartial_Benchmark runs dev-partial step by step on the block files, as
// [Core.Prove] does, and records for each block the time and peak resident
// memory of each step, the size of each shard and the result. A shard with a
// module over [wiop.ColumnSizeMaxSupported] rows is recorded and not checked,
// as its assignment would panic. Same settings as [TestDevPartial_Integration];
// BENCH_OUT appends one JSON line per block to that file, and
// BENCH_MAX_SHARDS stops each block after that many shards. Peak memory is
// read from /proc (Linux only).
//
//	REAL_GUEST_TESTS=1 DEV_PARTIAL_BLOCKS=block_amsterdam_175k.ssz BENCH_OUT=/tmp/bench.jsonl \
//		go test ./backend -run TestDevPartial_Benchmark -v -timeout 0
func TestDevPartial_Benchmark(t *testing.T) {
	core, id, blocks := realGuestCore(t)
	g, err := core.loadGuest(id)
	require.NoError(t, err)
	maxShards := uint(0)
	if s := os.Getenv("BENCH_MAX_SHARDS"); s != "" {
		n, err := strconv.ParseUint(s, 10, 32)
		require.NoError(t, err, "parsing BENCH_MAX_SHARDS")
		maxShards = uint(n)
	}
	for _, name := range blocks {
		t.Run(name, func(t *testing.T) {
			extended, err := os.ReadFile(blockPath(name)) //nolint:gosec // G703: test-controlled path
			require.NoError(t, err, "reading block file")
			rec := benchBlock{Block: name, InputBytes: len(extended), ShardSize: core.cfg.ShardSize}
			start := time.Now()
			rec.Result = benchDevPartial(t, core, g, extended, maxShards, &rec)
			rec.TotalSec = time.Since(start).Seconds()
			for _, s := range rec.Steps {
				rec.PeakRSSGB = max(rec.PeakRSSGB, s.PeakRSSGB)
			}
			for _, s := range rec.ShardsStats {
				rec.PeakRSSGB = max(rec.PeakRSSGB, s.Trace.PeakRSSGB, s.Check.PeakRSSGB)
			}
			logBench(t, rec)
			if out := os.Getenv("BENCH_OUT"); out != "" {
				line, err := json.Marshal(rec)
				require.NoError(t, err)
				f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G703: test-controlled path
				require.NoError(t, err)
				_, err = f.Write(append(line, '\n'))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
	}
}

// benchDevPartial runs the steps of dev-partial on one input, records them in
// rec and returns the result: "pass", "over the column limit", or the error.
func benchDevPartial(t *testing.T, c *Core, g guestProgram, extended []byte, maxShards uint, rec *benchBlock) string {
	t.Helper()
	inputs, err := c.encodeGuestInputs(Job{Payload: extended}, g)
	require.NoError(t, err)

	var out []byte
	if err := benchRun(&rec.Steps, "execute", func() error {
		guestOutput, err := executeGuest(c.driver.BinaryFile, inputs)
		out = guestOutput.Bytes()
		return err
	}); err != nil {
		return err.Error()
	}
	if _, err := decodeGuestPublicInputs(out[guestOutputHashSize:]); err != nil {
		return err.Error()
	}

	var lazyTrace zkcdriver.LazyTrace
	if err := benchRun(&rec.Steps, "fast pass", func() error {
		var err error
		lazyTrace, err = c.driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs})
		return err
	}); err != nil {
		return err.Error()
	}
	rec.NumShards = lazyTrace.Len()

	sharded := c.cfg.ShardSize > 0
	var reducer *conglomeration.Reducer
	if sharded {
		reducer = conglomeration.NewReducer(c.merger, conglomeration.DefaultMaxInFlightMerges)
	}
	var pub wiop.PublicInput
	result := "pass"
	for i := range rec.NumShards {
		if maxShards > 0 && i >= maxShards {
			return fmt.Sprintf("stopped after %d shards", maxShards)
		}
		stats := benchShard{Index: i}
		var shard zkcdriver.Shard
		stats.Trace = benchStepOf("trace", func() error {
			var err error
			shard, err = traceShard(lazyTrace, i)
			return err
		})
		if strings.HasPrefix(stats.Trace.Name, "error") {
			rec.ShardsStats = append(rec.ShardsStats, stats)
			return stats.Trace.Name
		}
		benchShardSize(c, shard, &stats)
		if len(stats.OverLimit) > 0 {
			rec.ShardsStats = append(rec.ShardsStats, stats)
			result = "over the column limit"
			continue
		}
		var proof wiop.Proof
		stats.Check = benchStepOf("check", func() error {
			var err error
			proof, pub, err = c.proveShard(shard)
			return err
		})
		rec.ShardsStats = append(rec.ShardsStats, stats)
		if strings.HasPrefix(stats.Check.Name, "error") {
			return stats.Check.Name
		}
		if sharded {
			if err := reducer.Add(conglomeration.Node{Coverage: []uint{i}, Proof: proof, Pub: pub}); err != nil {
				return err.Error()
			}
		}
	}
	if result != "pass" {
		return result
	}
	if sharded {
		if err := benchRun(&rec.Steps, "merge", func() error {
			root, err := reducer.Close(func(children []conglomeration.Node) (conglomeration.Node, error) {
				return c.merger.Finalize(children, rec.NumShards)
			})
			pub = root.Pub
			return err
		}); err != nil {
			return err.Error()
		}
	}
	if _, err := c.devmodePublicInputs(pub, out[:guestOutputHashSize]); err != nil {
		return err.Error()
	}
	return result
}

// benchShardSize records the cells, the largest modules and the modules over
// the column limit of the shard. Static modules are not assigned per shard, so
// they are left out.
func benchShardSize(c *Core, shard zkcdriver.Shard, stats *benchShard) {
	schema := c.driver.BinaryFile.AirConstraints()
	var modules []benchModule
	for k := range shard.Width() {
		if schema.Module(k).IsStatic() {
			continue
		}
		m := shard.Module(k)
		mod := benchModule{Name: m.Name(), Rows: m.Height(), Columns: m.Width()}
		stats.Cells += uint64(mod.Rows) * uint64(mod.Columns)
		if mod.Rows > wiop.ColumnSizeMaxSupported {
			stats.OverLimit = append(stats.OverLimit, mod.Name)
		}
		modules = append(modules, mod)
	}
	slices.SortFunc(modules, func(a, b benchModule) int { return cmp.Compare(b.Rows, a.Rows) })
	stats.Largest = modules[:min(5, len(modules))]
}

// benchRun runs one step and appends its record to steps.
func benchRun(steps *[]benchStep, name string, step func() error) error {
	var stepErr error
	s := benchStepOf(name, func() error {
		stepErr = step()
		return stepErr
	})
	*steps = append(*steps, s)
	return stepErr
}

// benchStepOf runs one step and returns its time and peak resident memory. On
// an error the name becomes "error in <name>: <error>".
func benchStepOf(name string, step func() error) benchStep {
	// Start from a collected heap, so the peak is the step's own.
	runtime.GC()
	debug.FreeOSMemory()
	resetPeakRSS()
	start := time.Now()
	err := step()
	s := benchStep{Name: name, Sec: time.Since(start).Seconds(), PeakRSSGB: peakRSSGB()}
	if err != nil {
		s.Name = fmt.Sprintf("error in %s: %v", name, err)
	}
	return s
}

// resetPeakRSS resets the peak resident memory of the process (Linux only).
func resetPeakRSS() {
	_ = os.WriteFile("/proc/self/clear_refs", []byte("5"), 0)
}

// peakRSSGB returns the peak resident memory of the process since the last
// reset, in GB, or -1 if /proc cannot be read.
func peakRSSGB() float64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VmHWM:"); ok {
			kb, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 64)
			if err != nil {
				return -1
			}
			return kb / (1 << 20)
		}
	}
	return -1
}

// logBench logs the record of one block in a readable form.
func logBench(t *testing.T, rec benchBlock) {
	t.Helper()
	t.Logf("%s: %d bytes, %d shards of %d instructions, result %q, %.0f s, peak %.1f GB",
		rec.Block, rec.InputBytes, rec.NumShards, rec.ShardSize, rec.Result, rec.TotalSec, rec.PeakRSSGB)
	for _, s := range rec.Steps {
		t.Logf("  %-12s %8.1f s %6.1f GB", s.Name, s.Sec, s.PeakRSSGB)
	}
	for _, s := range rec.ShardsStats {
		over := ""
		if len(s.OverLimit) > 0 {
			over = fmt.Sprintf(", OVER THE LIMIT: %v", s.OverLimit)
		}
		top := s.Largest[0]
		t.Logf("  shard %3d: trace %7.1f s %5.1f GB, check %7.1f s %5.1f GB, %d cells, largest %s %d rows%s",
			s.Index, s.Trace.Sec, s.Trace.PeakRSSGB, s.Check.Sec, s.Check.PeakRSSGB, s.Cells, top.Name, top.Rows, over)
	}
}

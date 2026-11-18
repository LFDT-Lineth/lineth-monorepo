//go:build linux

package backend

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/conglomeration"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/stretchr/testify/require"
)

// benchCase is one block of the benchmark plan. A block with maxShards > 0 is
// stopped after that many shards, and its full time is estimated from them.
type benchCase struct {
	block     string
	maxShards uint
}

// benchPlan is the benchmark plan, smallest block first, every block in full.
// The shard counts are at 500000 instructions per shard.
var benchPlan = []benchCase{
	{"stateless_input.ssz", 0},            // 4 shards
	{"block_amsterdam_175k.ssz", 0},       // 57 shards
	{"block_amsterdam_786k.ssz", 0},       // 96 shards
	{"block_amsterdam_51584_500k.ssz", 0}, // 113 shards
	{"block_amsterdam_52861_2M.ssz", 0},   // 272 shards
	{"block_amsterdam_52866_17M.ssz", 0},  // not counted, maybe 2000 or more
}

// The environment of a block process, set by [TestDevPartial_Benchmark].
const (
	benchBlockEnv     = "BENCH_BLOCK"
	benchMaxShardsEnv = "BENCH_MAX_SHARDS"
	benchRecordEnv    = "BENCH_RECORD"
)

// benchWorkersEnv sets the number of shards in progress at once, instead of
// benchWorkers. 1 checks the shards one at a time and records each of them.
const benchWorkersEnv = "BENCH_SHARD_WORKERS"

// benchWorkers is the number of shards in progress at once, the same for
// every block so that the blocks compare, and a round number so that the gain
// is easy to read. 17M fits on r8a.24xlarge (742 GiB): its checkpoints take
// ~200 GiB, and a shard in progress needs up to ~32 GiB.
const benchWorkers = 10

// benchShardsStep is the name of the step that traces and checks the shards
// in parallel.
const benchShardsStep = "shards"

// benchExecuteStep is the name of the step that runs the guest for its output.
const benchExecuteStep = "execute"

// benchUnfinished is the result of a block until it ends: a block process
// killed before the end leaves it in its record.
const benchUnfinished = "did not finish"

// benchPass is the result of a block that passed.
const benchPass = "pass"

// benchFastPass is the name of the step that splits the trace into shards.
const benchFastPass = "fast pass"

// benchBlock is the benchmark record of one block file.
type benchBlock struct {
	Block         string       `json:"block"`
	Guest         string       `json:"guest"`
	InputBytes    int          `json:"inputBytes"`
	ShardSize     uint64       `json:"shardSize"`
	NumShards     uint         `json:"numShards"`
	ShardsRun     uint         `json:"shardsRun"`
	MaxShards     uint         `json:"maxShards"`
	Workers       int          `json:"workers"`
	Result        string       `json:"result"`
	TotalSec      float64      `json:"totalSec"`
	PeakRSSGB     float64      `json:"peakRssGb"`
	ProcessPeakGB float64      `json:"processPeakGb"`
	Steps         []benchStep  `json:"steps"`
	ShardsStats   []benchShard `json:"shards"`
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

// TestDevPartial_Benchmark runs dev-partial step by step on the blocks of
// [benchPlan], as [Core.Prove] does, and reports for each block the time and
// peak memory of each step and the result. Each block runs in its own process,
// so that a block killed for lack of memory is reported and the next one still
// runs. The shards are traced and checked benchWorkers at a time, with the Go
// memory limit at 90% of the machine. With BENCH_SHARD_WORKERS=1 they are checked one at a time and each
// is recorded: its size, and a shard with a module over
// [wiop.ColumnSizeMaxSupported] rows is reported and not checked, as its
// assignment would panic. Same settings as [TestDevPartial_Integration]
// (GUEST_ELF, DEV_PARTIAL_SHARD_SIZE). The report goes to BENCH_DIR, by default
// a new folder in the home folder: report.md, bench.jsonl, and the record and
// log of each block. Linux only (peak memory is read from /proc).
//
//	REAL_GUEST_TESTS=1 go test ./backend -run TestDevPartial_Benchmark -v -timeout 0
func TestDevPartial_Benchmark(t *testing.T) {
	if os.Getenv("REAL_GUEST_TESTS") == "" {
		t.Skip("set REAL_GUEST_TESTS=1 to run the real guest tests (heavy: runs real blocks)")
	}
	if block := os.Getenv(benchBlockEnv); block != "" {
		benchOneBlock(t, block)
		return
	}

	dir := benchDir(t)
	t.Logf("writing the report to %s", dir)
	machine := benchMachine()
	var recs []benchBlock
	for _, bc := range benchPlan {
		t.Logf("=== %s", bc.block)
		rec := benchInProcess(t, dir, bc)
		recs = append(recs, rec)
		line, err := json.Marshal(rec)
		require.NoError(t, err)
		appendFile(t, filepath.Join(dir, "bench.jsonl"), append(line, '\n'))
		report := []byte(renderBenchReport(machine, recs))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "report.md"), report, 0o600)) //nolint:gosec // G703: test-controlled path
		t.Logf("=== %s: %s", bc.block, rec.Result)
	}
	t.Logf("report: %s", filepath.Join(dir, "report.md"))
}

// benchDir returns the report folder: BENCH_DIR, or a new folder in the home
// folder.
func benchDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("BENCH_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		dir = filepath.Join(home, "dev-partial-bench-"+time.Now().Format("20060102-150405"))
	}
	require.NoError(t, os.MkdirAll(dir, 0o750)) //nolint:gosec // G703: test-controlled path
	return dir
}

// benchMachine describes the machine and the settings of the run.
func benchMachine() string {
	cpu := "unknown CPU"
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for line := range strings.Lines(string(data)) {
			if v, ok := strings.CutPrefix(line, "model name"); ok {
				cpu = strings.TrimSpace(strings.TrimLeft(v, " \t:"))
				break
			}
		}
	}
	memGB := -1.0
	if kb, ok := procKB("/proc/meminfo", "MemTotal:"); ok {
		memGB = kb / (1 << 20)
	}
	commit := "unknown"
	if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	elf := os.Getenv("GUEST_ELF")
	if elf == "" {
		elf = "default (riscv-guests/l2_execution/zig-out/bin/evm_execution_guest)"
	}
	return fmt.Sprintf("Machine: %d CPUs (%s), %.0f GB memory. Commit %s, %s. Guest ELF: %s.",
		runtime.NumCPU(), cpu, memGB, commit, runtime.Version(), elf)
}

// benchInProcess runs one block in a new process of this test binary and
// returns its record, with the peak memory of the whole process.
func benchInProcess(t *testing.T, dir string, bc benchCase) benchBlock {
	t.Helper()
	recordPath := filepath.Join(dir, bc.block+".json")
	logFile, err := os.Create(filepath.Join(dir, bc.block+".log")) //nolint:gosec // G304: test-controlled path
	require.NoError(t, err)
	defer func() { require.NoError(t, logFile.Close()) }()

	//nolint:gosec // G204: this test binary
	cmd := exec.Command(os.Args[0], "-test.run=^TestDevPartial_Benchmark$", "-test.v", "-test.timeout=0")
	cmd.Env = append(os.Environ(),
		benchBlockEnv+"="+bc.block,
		benchMaxShardsEnv+"="+strconv.FormatUint(uint64(bc.maxShards), 10),
		benchRecordEnv+"="+recordPath,
	)
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = cmd.Stdout
	runErr := cmd.Run()

	rec := benchBlock{Block: bc.block, MaxShards: bc.maxShards}
	if data, err := os.ReadFile(recordPath); err == nil { //nolint:gosec // G304: test-controlled path
		require.NoError(t, json.Unmarshal(data, &rec))
	}
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		rec.ProcessPeakGB = float64(ru.Maxrss) / (1 << 20) // Maxrss is in kB on Linux
	}
	if rec.Result == "" || rec.Result == benchUnfinished {
		rec.Result = benchChildFailure(runErr)
	}
	return rec
}

// benchChildFailure says how a block process that wrote no result ended.
func benchChildFailure(err error) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "did not finish, see the log"
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGKILL {
		return "killed by SIGKILL, most likely out of memory"
	}
	return fmt.Sprintf("failed (%v), see the log", err)
}

// benchOneBlock runs one block in this process and writes its record to
// BENCH_RECORD after each step, so a killed process leaves what it did.
func benchOneBlock(t *testing.T, block string) {
	t.Helper()
	core, id, _ := realGuestCore(t)
	g, err := core.loadGuest(id)
	require.NoError(t, err)
	maxShards, err := strconv.ParseUint(os.Getenv(benchMaxShardsEnv), 10, 32)
	require.NoError(t, err, "parsing "+benchMaxShardsEnv)
	workers := benchWorkers
	if s := os.Getenv(benchWorkersEnv); s != "" {
		workers, err = strconv.Atoi(s)
		require.NoError(t, err, "parsing "+benchWorkersEnv)
	}
	extended, err := os.ReadFile(blockPath(block)) //nolint:gosec // G703: test-controlled path
	require.NoError(t, err, "reading block file")

	rec := benchBlock{
		Block: block, Guest: fmt.Sprintf("%x", id), InputBytes: len(extended),
		ShardSize: core.cfg.ShardSize, MaxShards: uint(maxShards), Result: benchUnfinished,
	}
	start := time.Now()
	save := func() {
		rec.TotalSec = time.Since(start).Seconds()
		for _, s := range rec.Steps {
			rec.PeakRSSGB = max(rec.PeakRSSGB, s.PeakRSSGB)
		}
		for _, s := range rec.ShardsStats {
			rec.PeakRSSGB = max(rec.PeakRSSGB, s.Trace.PeakRSSGB, s.Check.PeakRSSGB)
		}
		line, err := json.Marshal(rec)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(os.Getenv(benchRecordEnv), line, 0o600)) //nolint:gosec // G703: test-controlled path
	}
	save()
	result := benchDevPartial(t, core, g, extended, uint(maxShards), workers, &rec, save)
	rec.Result = result
	save()
	logBench(t, rec)
}

// benchDevPartial runs the steps of dev-partial on one input, records them in
// rec, calls save after each step, and returns the result: "pass", "over the
// column limit", "stopped after N shards", or the error. workers is the number
// of shards in progress at once.
func benchDevPartial(
	t *testing.T, c *Core, g guestProgram, extended []byte, maxShards uint, workers int, rec *benchBlock, save func(),
) string {
	t.Helper()
	inputs, err := c.encodeGuestInputs(Job{Payload: extended}, g)
	require.NoError(t, err)

	var out []byte
	err = benchRun(&rec.Steps, benchExecuteStep, func() error {
		var err error
		out, _, err = executeGuestPublicInputs(c.driver.BinaryFile, inputs)
		return err
	})
	save()
	if err != nil {
		return err.Error()
	}

	var lazyTrace zkcdriver.LazyTrace
	err = benchRun(&rec.Steps, benchFastPass, func() error {
		var err error
		lazyTrace, err = c.driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs})
		return err
	})
	if err == nil {
		rec.NumShards = lazyTrace.Len()
	}
	save()
	if err != nil {
		return err.Error()
	}

	sharded := c.cfg.ShardSize > 0
	var reducer *conglomeration.Reducer
	if sharded {
		reducer = conglomeration.NewReducer(c.merger, conglomeration.DefaultMaxInFlightMerges)
	}
	workers = benchShardWorkers(workers, rec.NumShards)
	rec.Workers = workers
	if workers > 1 && sharded && maxShards == 0 {
		// The steps of the shards overlap, so they are measured as one.
		c.cfg.ShardWorkers = workers
		totalKB, _ := procKB("/proc/meminfo", "MemTotal:")
		debug.SetMemoryLimit(int64(0.9 * totalKB * 1024))
		indexes := make([]uint, rec.NumShards)
		for i := range indexes {
			indexes[i] = uint(i)
		}
		err := benchRun(&rec.Steps, benchShardsStep, func() error {
			return c.proveShards(context.Background(), lazyTrace, indexes, reducer)
		})
		save()
		if err != nil {
			return err.Error()
		}
		rec.ShardsRun = rec.NumShards
		return benchMerge(c, rec, reducer, out, save)
	}

	var pub wiop.PublicInput
	result := benchPass
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
			save()
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
		save()
		if strings.HasPrefix(stats.Check.Name, "error") {
			return stats.Check.Name
		}
		t.Logf("shard %d done: trace %.1f s, check %.1f s, peak %.1f GB: %d of %d",
			i, stats.Trace.Sec, stats.Check.Sec, max(stats.Trace.PeakRSSGB, stats.Check.PeakRSSGB), i+1, rec.NumShards)
		if sharded {
			if err := reducer.Add(conglomeration.Node{Coverage: []uint{i}, Proof: proof, Pub: pub}); err != nil {
				return err.Error()
			}
		}
		rec.ShardsRun = i + 1
	}
	if result != benchPass {
		return result
	}
	if sharded {
		return benchMerge(c, rec, reducer, out, save)
	}
	if _, err := c.devmodePublicInputs(pub, out[:guestOutputHashSize]); err != nil {
		return err.Error()
	}
	return result
}

// benchMerge runs the last merges and the root checks of the mock merger,
// then reads the public inputs from the root.
func benchMerge(c *Core, rec *benchBlock, reducer *conglomeration.Reducer, out []byte, save func()) string {
	var pub wiop.PublicInput
	err := benchRun(&rec.Steps, "merge", func() error {
		root, err := reducer.Close(func(children []conglomeration.Node) (conglomeration.Node, error) {
			return c.merger.Finalize(children, rec.NumShards)
		})
		pub = root.Pub
		return err
	})
	save()
	if err != nil {
		return err.Error()
	}
	if _, err := c.devmodePublicInputs(pub, out[:guestOutputHashSize]); err != nil {
		return err.Error()
	}
	return benchPass
}

// benchShardWorkers returns the number of shards in progress at once: the
// workers asked for, no more than the shards, at least 1.
func benchShardWorkers(workers int, shards uint) int {
	return max(1, min(workers, int(shards)))
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

// resetPeakRSS resets the peak resident memory of the process.
func resetPeakRSS() {
	_ = os.WriteFile("/proc/self/clear_refs", []byte("5"), 0)
}

// peakRSSGB returns the peak resident memory of the process since the last
// reset, in GB, or -1 if /proc cannot be read.
func peakRSSGB() float64 {
	kb, ok := procKB("/proc/self/status", "VmHWM:")
	if !ok {
		return -1
	}
	return kb / (1 << 20)
}

// procKB returns the value in kB of the line of the /proc file that starts
// with key.
func procKB(path, key string) (float64, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: /proc file
	if err != nil {
		return 0, false
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), key); ok {
			kb, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 64)
			return kb, err == nil
		}
	}
	return 0, false
}

// appendFile appends data to the file at path.
func appendFile(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G304: test-controlled path
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// renderBenchReport renders the records as a Markdown report: one row per
// block. The full time of a block stopped before its last shard is estimated
// from the time of its steps and the average time of its shards.
func renderBenchReport(machine string, recs []benchBlock) string {
	var b strings.Builder
	b.WriteString("# dev-partial benchmark\n\n" + machine + "\n\n")
	b.WriteString("| Block | Shards run | Result | Time | Peak memory | Fast pass | Time per shard " +
		"| Largest module | Shards over the limit | Full time (estimated) | Workers |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range recs {
		fastPass := "-"
		var stepsSec, parallelSec float64
		for _, s := range r.Steps {
			stepsSec += s.Sec
			switch s.Name {
			case benchFastPass:
				fastPass = fmt.Sprintf("%s, %.1f GB", benchDuration(s.Sec), s.PeakRSSGB)
			case benchShardsStep:
				parallelSec = s.Sec
			}
		}
		perShard, largest, over := "-", "-", 0
		var shardsSec float64
		var top benchModule
		for _, s := range r.ShardsStats {
			shardsSec += s.Trace.Sec + s.Check.Sec
			if len(s.Largest) > 0 && s.Largest[0].Rows > top.Rows {
				top = s.Largest[0]
			}
			if len(s.OverLimit) > 0 {
				over++
			}
		}
		run := max(r.ShardsRun, uint(len(r.ShardsStats)))
		switch {
		case len(r.ShardsStats) > 0:
			perShard = benchDuration(shardsSec / float64(len(r.ShardsStats)))
			largest = fmt.Sprintf("%s %d rows", top.Name, top.Rows)
		case run > 0:
			perShard = benchDuration(parallelSec / float64(run))
		}
		estimate := "-"
		if n := len(r.ShardsStats); n > 0 && uint(n) < r.NumShards {
			estimate = benchDuration(stepsSec + shardsSec/float64(n)*float64(r.NumShards))
		}
		peak := max(r.ProcessPeakGB, r.PeakRSSGB)
		fmt.Fprintf(&b, "| %s | %d of %d | %s | %s | %.1f GB | %s | %s | %s | %d | %s | %d |\n",
			r.Block, run, r.NumShards, benchCell(r.Result), benchDuration(r.TotalSec), peak,
			fastPass, perShard, largest, over, estimate, max(r.Workers, 1))
	}
	return b.String()
}

// benchDuration formats seconds as a duration, to the second.
func benchDuration(sec float64) string {
	return (time.Duration(sec * float64(time.Second))).Round(time.Second).String()
}

// benchCell makes s fit in one cell of a Markdown table.
func benchCell(s string) string {
	return strings.NewReplacer("|", "/", "\n", " ").Replace(s)
}

// logBench logs the record of one block in a readable form.
func logBench(t *testing.T, rec benchBlock) {
	t.Helper()
	t.Logf("%s: %d bytes, %d shards of %d instructions, %d workers, result %q, %.0f s, peak %.1f GB",
		rec.Block, rec.InputBytes, rec.NumShards, rec.ShardSize, max(rec.Workers, 1), rec.Result, rec.TotalSec, rec.PeakRSSGB)
	for _, s := range rec.Steps {
		t.Logf("  %-12s %8.1f s %6.1f GB", s.Name, s.Sec, s.PeakRSSGB)
	}
	for _, s := range rec.ShardsStats {
		over := ""
		if len(s.OverLimit) > 0 {
			over = fmt.Sprintf(", OVER THE LIMIT: %v", s.OverLimit)
		}
		// A failed trace has no module.
		largest := "no module"
		if len(s.Largest) > 0 {
			largest = fmt.Sprintf("largest %s %d rows", s.Largest[0].Name, s.Largest[0].Rows)
		}
		t.Logf("  shard %3d: trace %7.1f s %5.1f GB, check %7.1f s %5.1f GB, %d cells, %s%s",
			s.Index, s.Trace.Sec, s.Trace.PeakRSSGB, s.Check.Sec, s.Check.PeakRSSGB, s.Cells, largest, over)
	}
}

// TestLogBench_FailedTrace checks that a shard whose trace failed, so with no
// module, is logged.
func TestLogBench_FailedTrace(t *testing.T) {
	logBench(t, benchBlock{
		Block:       "block.ssz",
		Result:      "error in trace: boom",
		ShardsStats: []benchShard{{Index: 3, Trace: benchStep{Name: "error in trace: boom"}}},
	})
}

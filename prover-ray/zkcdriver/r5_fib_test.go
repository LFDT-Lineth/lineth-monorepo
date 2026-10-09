package zkcdriver_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// BenchmarkR5Fibonacci traces and proves the [minimalelf.FibonacciELF]
// guest sharded at 250K interpreter invocations per shard, reporting per-shard
// trace/proof metrics. This is a benchmark (not a CI test): it is heavy.
//
// Environment knobs:
//
//	R5_FIB_N        Fibonacci iteration count baked into the ELF
//	                (default 1,000,000; the cross-zkVM comparison uses 4M)
//	R5_FIB_PROVE    how many shards to prove (spread evenly across the run,
//	                default 1 = the first shard); proving verifies each proof.
//
// Example:
//
//	R5_FIB_N=4000000 R5_FIB_PROVE=2 go test -bench BenchmarkR5Fibonacci -benchtime 1x -run XXX ./zkcdriver/ -v -timeout 3500s
func BenchmarkR5Fibonacci(b *testing.B) {
	const instrPerShard = 250_000
	var (
		n           = fibBenchEnvInt(b, "R5_FIB_N", 1_000_000)
		proveShards = uint(fibBenchEnvInt(b, "R5_FIB_PROVE", 1))
		// Each shard covers 250K interpreter() invocations = 250K executed
		// RISC-V instructions (see r5_benchmark_test.go's fixture comment).
		tracingConfig = vm.DEFAULT_TRACE_CONFIG.
				WithSharding(vm.NewShardingStrategy("interpreter", instrPerShard)).
				WithParallelism(true)
	)

	elf, wantOutput := minimalelf.FibonacciELF(n)
	inputsMap, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		b.Fatalf("failed to prepare inputs: %v", err)
	}
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		b.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}

	b.Logf("tracing fib(N=%d) sharded", n)
	outputs, maybeTrace, errs := binf.Trace(inputsMap, tracingConfig)
	if len(errs) > 0 {
		b.Fatalf("tracing failed: %v", errs)
	} else if got := outputs["guest_output"]; !bytes.Equal(got, wantOutput) {
		b.Fatalf("unexpected guest_output: got %x, want %x", got, wantOutput)
	}
	//
	lazyTrace := maybeTrace.Unwrap()
	// Per-shard trace statistics.
	numShards := lazyTrace.Len()
	b.Logf("shards: %d (R5_FIB_N=%d, %d interpreter invocations per shard)", numShards, n, instrPerShard)
	rowLimit := 0
	// Process shards lazily
	//
	// NOTE: this forces all shards to be fully materialised.
	errs = lazyTrace.Apply(func(i uint, shard zkcdriver.Shard) {
		var rows, cells uint64
		var tallestName string
		var tallestHeight uint
		type modStat struct {
			name   string
			height uint
			width  uint
		}
		var mods = make([]modStat, shard.Width())
		//
		for moduleID := range shard.Width() {
			module := shard.Module(moduleID)
			rows += uint64(module.Height())
			cells += uint64(module.Height()) * uint64(module.Width())
			if module.Height() > tallestHeight {
				tallestHeight = module.Height()
				tallestName = module.Name()
			}
			mods[moduleID] = modStat{module.Name(), module.Height(), module.Width()}
			if module.Name() == "interpreter" {
				b.Logf("shard %d: interpreter module %d rows x %d cols", i, module.Height(), module.Width())
				b.ReportMetric(float64(module.Height()), fmt.Sprintf("shard_%d/interp-rows", i))
			}
		}
		// Top 8 modules by height (fill-rate profiling).
		sort.Slice(mods, func(a, b int) bool { return mods[a].height > mods[b].height })
		for j := 0; j < len(mods) && j < 8; j++ {
			b.Logf("shard %d top%d: %s %d rows x %d cols", i, j, mods[j].name, mods[j].height, mods[j].width)
		}
		if tallestHeight > 0 {
			b.Logf("shard %d: %d rows, %d cells, tallest module %q at %d rows (2^%d)",
				i, rows, cells, tallestName, tallestHeight, log2ceil(tallestHeight))
		}
		b.ReportMetric(float64(rows), fmt.Sprintf("shard_%d/trace-rows", i))
		b.ReportMetric(float64(cells), fmt.Sprintf("shard_%d/trace-cells", i))
		rowLimit = max(rowLimit, log2ceil(tallestHeight))
	})
	// Sanity check for tracing failures
	if len(errs) > 0 {
		b.Fatalf("tracing failed: %v", errors.Join(errs...))
	}
	//
	b.ReportMetric(float64(numShards), "shards")
	// Deterministic extrapolation to the cross-zkVM workload: the guest executes
	// exactly 14+5*N instructions (see FibonacciELF doc), so shard count scales
	// linearly from the measured interpreter rows per shard.
	b.ReportMetric(float64((14+5*uint64(4_000_000)+instrPerShard-1)/instrPerShard), "shards-at-N4M")

	if proveShards < 1 {
		return
	}

	// Compile the proving system once.
	serialized, err := binf.MarshalBinary()
	if err != nil {
		b.Fatalf("serializing R5 constraints: %v", err)
	}
	system, driver := compileR5BenchmarkSystem(b, serialized)

	if limit := int(pcs.FRIMaxCommittableSizeLog2()); rowLimit > limit {
		b.Fatalf("tallest shard module is 2^%d rows, above the 2^%d PCS limit; trace is not provable",
			rowLimit, limit)
	}

	// Prove up to R5_FIB_PROVE shards, spread evenly (first/…/last).
	step := max(numShards/proveShards, 1)
	proved := uint(0)
	for i := uint(0); i < numShards && proved < proveShards; i += step {
		// NOTE: this forces the ith shard to be retraced --- meaning it will
		// have been traced once during the metric run above, and then again
		// during the proof run here.
		shard, errs := lazyTrace.Get(i)
		// sanity check for tracing errors
		if len(errs) > 0 {
			b.Fatalf("shard %d failed to trace: %v", i, errors.Join(errs...))
		}
		proof, pub := system.Prove(func(rt *wiop.Runtime) {
			driver.AssignTraceShard(rt, shard.Unwrap(), placeholderSharedRandomness)
		})
		sizeBytes := proofserialization.Measure(system, proof, pub).Total
		if err := system.Verify(proof, pub); err != nil {
			// Cross-shard consistency is not yet wired (messagebus shared
			// randomness, init/final chaining); only shard 0 is expected to
			// verify standalone today.
			if i == 0 {
				b.Fatalf("shard 0 proof does not verify: %v", err)
			}
			b.Logf("shard %d: proof %d bytes (%.2f MiB), verify FAILED (expected until cross-shard binding lands): %v",
				i, sizeBytes, float64(sizeBytes)/float64(1<<20), err)
		} else {
			b.Logf("shard %d: proof %d bytes (%.2f MiB), verified", i, sizeBytes, float64(sizeBytes)/float64(1<<20))
		}
		b.ReportMetric(float64(sizeBytes), fmt.Sprintf("shard_%d/proof-bytes", i))
		proved++
	}
}

func fibBenchEnvInt(b *testing.B, name string, def int) int {
	b.Helper()
	if v := os.Getenv(name); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			b.Fatalf("bad %s=%q", name, v)
		}
		return n
	}
	return def
}

func log2ceil(v uint) int {
	l := 0
	for (uint(1) << l) < v {
		l++
	}
	return l
}

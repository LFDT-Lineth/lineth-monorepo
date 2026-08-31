package zkcdriver_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"

	zkcr5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

const (
	r5ZKCPath      = "../../arithmetization/src/main/riscv/main.zkc"
	r5VerifierPath = "../../verifier-ray/zig-out/bin/verifier-ray"
)

var (
	r5TraceSink trace.Trace[koalabear.Element]
	r5ProofSink []wiop.Proof
	r5PubSink   []wiop.PublicInput
)

type r5BenchmarkFixture struct {
	binFile        *constraints.BinaryFile[koalabear.Element]
	inputs         map[string][]byte
	expandedShards []trace.Shard[koalabear.Element]
	serialized     []byte
	system         *wiop.System
	driver         *zkcdriver.ZkCDriver
	traceRows      []uint64
	traceCells     []uint64
}

func loadR5BenchmarkFixture(b *testing.B) *r5BenchmarkFixture {
	var (
		// NOTE: the "sharding strategy" controls how sharding operators, and is
		// fairly simplistic (at this stage).  In essence, the strategy below
		// indicates that each shard will contain 500K invocations of the
		// "interpreter()" function.  Since this function is involved once per
		// RISC-V instruction, this indicates that each shard contains 500K
		// RISC-V instruction executions.  Note, for example, that the first
		// shard is expected to be bigger under this simplistic strategy, since
		// it will also include the initialisation phase of the interpreter
		// (i.e. loading inputs into RAM).
		shardingStrategy = vm.NewShardingStrategy("interpreter", 500000)
		// Specificy tracing options
		tracingConfig = vm.DEFAULT_TRACE_CONFIG.WithSharding(shardingStrategy).
			// Indicate shards should be traced in parallel after an initial
			// "fast mode" execution run to create checkpoints (i.e. as
			// determined by the sharding strategy).
			WithParallelism(true)
	)
	b.Helper()

	verifierELF, err := os.ReadFile(r5VerifierPath)
	if err != nil {
		b.Skipf("R5 verifier ELF unavailable at %s; run `make -C ../verifier-ray build-r5`: %v", r5VerifierPath, err)
	}
	inputs, err := zkcr5.PrepareInput(verifierELF, []byte("foobar"))
	if err != nil {
		b.Fatalf("preparing R5 input: %v", err)
	}
	binFile, err := compileBinaryConstraints(r5ZKCPath)
	if err != nil {
		b.Fatalf("compiling R5 ZKC program: %v", err)
	}
	_, expandedTrace, errs := binFile.Trace(inputs, tracingConfig)
	if len(errs) > 0 {
		b.Fatalf("tracing R5 fixture: %v", errors.Join(errs...))
	}
	serialized, err := binFile.MarshalBinary()
	if err != nil {
		b.Fatalf("serializing R5 constraints: %v", err)
	}
	fixture := &r5BenchmarkFixture{
		binFile:        binFile,
		inputs:         inputs,
		expandedShards: expandedTrace,
		serialized:     serialized,
	}
	// Initialise traceRows/traceCells
	fixture.traceRows = make([]uint64, len(expandedTrace))
	fixture.traceCells = make([]uint64, len(expandedTrace))
	// Collect per-shard metrics
	for i, shard := range expandedTrace {
		for moduleID := range shard.Width() {
			module := shard.Module(moduleID)
			fixture.traceRows[i] += uint64(module.Height())
			fixture.traceCells[i] += uint64(module.Height()) * uint64(module.Width())
		}
	}

	return fixture
}

func (f *r5BenchmarkFixture) ensureSystem(b *testing.B) {
	b.Helper()
	if f.system == nil {
		f.system, f.driver = compileR5BenchmarkSystem(b, f.serialized)
	}
}

func compileR5BenchmarkSystem(b *testing.B, serialized []byte) (*wiop.System, *zkcdriver.ZkCDriver) {
	b.Helper()

	system := wiop.NewSystemf("zkc-r5-benchmark")
	system.NewRound()
	driver := zkcdriver.NewZkCDriver(
		system,
		zkcdriver.Settings{},
		bytes.NewReader(serialized),
	)
	proverCompilePipeline(system)
	return system, driver
}

func reportR5Work(b *testing.B, fixture *r5BenchmarkFixture) {
	b.Helper()
	for i := range fixture.traceCells {
		b.ReportMetric(float64(fixture.traceRows[i]), fmt.Sprintf("shard_%d/trace-rows/op", i))
		b.ReportMetric(float64(fixture.traceCells[i]), fmt.Sprintf("shard_%d/trace-cells/op", i))
		b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
	}
}

// BenchmarkR5Trace measures RISC-V execution and AIR trace expansion. It does
// not check constraints, assign WIOP columns, prove, or verify.
func BenchmarkR5Trace(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, expandedTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			vm.DEFAULT_TRACE_CONFIG,
		)
		if len(errs) > 0 {
			b.Fatalf("tracing R5 program: %v", errors.Join(errs...))
		}
		r5TraceSink = expandedTrace
	}
	reportR5Work(b, fixture)
}

// BenchmarkR5TraceAndCheck adds validation of the expanded trace against the
// AIR constraints to BenchmarkR5Trace.
func BenchmarkR5TraceAndCheck(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, expandedTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			vm.DEFAULT_TRACE_CONFIG,
		)
		if len(errs) > 0 {
			b.Fatalf("tracing R5 program: %v", errors.Join(errs...))
		}
		if failures := fixture.binFile.Check(
			vm.DEFAULT_TRACE_CONFIG,
			expandedTrace,
		); len(failures) > 0 {
			b.Fatalf("checking R5 trace: %s", failures[0].Message())
		}
		r5TraceSink = expandedTrace
	}
	reportR5Work(b, fixture)
}

// BenchmarkR5AssignFromExpandedTrace isolates copying an already-expanded AIR
// trace into fresh WIOP runtime columns.
func BenchmarkR5AssignFromExpandedTrace(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for _, shard := range fixture.expandedShards {
			zkcdriver.AssignFromTraceShard(
				wiop.NewRuntime(fixture.system),
				shard,
				fixture.binFile.AirConstraints(),
				koalafield.Octuplet{},
			)
		}
	}

	reportR5Work(b, fixture)
}

// BenchmarkR5TraceAndAssign measures witness generation as used by Prove:
// tracing, expansion, and copying the resulting columns into a fresh runtime.
func BenchmarkR5TraceAndAssign(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	inputs := &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		traces := fixture.driver.TraceZkcInputs(inputs)
		fixture.expandedShards = traces
		for _, shard := range traces {
			zkcdriver.AssignFromTraceShard(
				wiop.NewRuntime(fixture.system),
				shard,
				fixture.binFile.AirConstraints(),
				koalafield.Octuplet{},
			)
		}
	}

	reportR5Work(b, fixture)
}

// BenchmarkR5SystemCompile measures constraint decoding, WIOP definition, and
// all compiler passes, including PCS. ZKC source compilation is excluded.
func BenchmarkR5SystemCompile(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		compileR5BenchmarkSystem(b, fixture.serialized)
	}
}

// BenchmarkR5ZKCCompile measures source-to-binary AIR compilation only.
func BenchmarkR5ZKCCompile(b *testing.B) {
	loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := compileBinaryConstraints(r5ZKCPath); err != nil {
			b.Fatalf("compiling R5 ZKC program: %v", err)
		}
	}
}

// BenchmarkR5Prove measures one warm proof on a precompiled immutable system.
// It includes trace generation and column assignment, as production Prove does,
// but excludes ZKC and WIOP compilation and excludes verification.
//
// The scope of the benchmark is a single (the first) shard
func BenchmarkR5Prove(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	inputs := &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
	traces := fixture.driver.TraceZkcInputs(inputs)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
			fixture.driver.AssignTraceShard(rt, traces[0], koalafield.Octuplet{})
		})
		r5ProofSink, r5PubSink = []wiop.Proof{proof}, []wiop.PublicInput{pub}
	}
	reportR5Work(b, fixture)
}

// BenchmarkR5Verify measures verification of one proof produced before the
// timer starts. The immutable proof and public input are reused
//
// The scope of the benchmark is a single (the first) shard
func BenchmarkR5Verify(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	inputs := &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
	traces := fixture.driver.TraceZkcInputs(inputs)

	proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
		fixture.driver.AssignTraceShard(rt, traces[0], koalafield.Octuplet{})
	})
	if err := fixture.system.Verify(proof, pub); err != nil {
		b.Fatalf("verifying setup proof: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := fixture.system.Verify(proof, pub); err != nil {
			b.Fatalf("verifying R5 proof: %v", err)
		}
	}
	reportR5Work(b, fixture)
}

// BenchmarkR5ColdEndToEnd measures ZKC source compilation, serialization,
// WIOP/PCS compilation, tracing and assignment, proof generation, and
// verification. ELF reading and input encoding are prepared outside the timer.
func BenchmarkR5ColdEndToEnd(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		binFile, err := compileBinaryConstraints(r5ZKCPath)
		if err != nil {
			b.Fatalf("compiling R5 ZKC program: %v", err)
		}
		serialized, err := binFile.MarshalBinary()
		if err != nil {
			b.Fatalf("serializing R5 constraints: %v", err)
		}

		var (
			system, driver = compileR5BenchmarkSystem(b, serialized)
			inputs         = &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
			traces         = fixture.driver.TraceZkcInputs(inputs)
			proofs         = make([]wiop.Proof, len(traces))
			pubs           = make([]wiop.PublicInput, len(traces))
		)

		for i, shard := range traces {
			proofs[i], pubs[i] = system.Prove(func(rt *wiop.Runtime) {
				driver.AssignTraceShard(rt, shard, koalafield.Octuplet{})
			})
		}

		for i := range proofs {
			if err := system.Verify(proofs[i], pubs[i]); err != nil {
				b.Fatalf("verifying R5 proof: %v", err)
			}
		}

		r5ProofSink, r5PubSink = proofs, pubs
	}
	reportR5Work(b, fixture)
}

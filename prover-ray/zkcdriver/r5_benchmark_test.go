package zkcdriver_test

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

var (
	r5TraceSink zkcdriver.LazyTrace
	r5ProofSink []wiop.Proof
	r5PubSink   []wiop.PublicInput
)

type r5BenchmarkFixture struct {
	binFile    *zkcdriver.BinaryFile
	inputs     map[string][]byte
	lazyTrace  zkcdriver.LazyTrace
	serialized []byte
	system     *wiop.System
	driver     *zkcdriver.ZkCDriver
	traceRows  []uint64
	traceCells []uint64
}

func loadR5BenchmarkFixture(b *testing.B) *r5BenchmarkFixture {

	b.Helper()

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

	guestELF, wantOutput := minimalelf.AllInOneElfProgram()
	inputs, err := predecoding.PrepareInputs(guestELF, nil)
	if err != nil {
		b.Fatalf("preparing R5 input: %v", err)
	}
	binFile, err := embedded.CompiledBinaryFile()
	if err != nil {
		b.Fatalf("compiling R5 ZKC program: %v", err)
	}
	outputs, lazyTrace, errs := binFile.Trace(inputs, tracingConfig)
	if len(errs) > 0 {
		b.Fatalf("tracing R5 fixture: %v", errors.Join(errs...))
	}
	if got := outputs["guest_output"]; !bytes.Equal(got, wantOutput) {
		b.Fatalf("unexpected guest_output: got %x, want %x", got, wantOutput)
	}
	serialized, err := binFile.MarshalBinary()
	if err != nil {
		b.Fatalf("serializing R5 constraints: %v", err)
	}
	fixture := &r5BenchmarkFixture{
		binFile:    binFile,
		inputs:     inputs,
		lazyTrace:  lazyTrace.Unwrap(),
		serialized: serialized,
	}
	// Initialise traceRows/traceCells
	fixture.traceRows = make([]uint64, fixture.lazyTrace.Len())
	fixture.traceCells = make([]uint64, fixture.lazyTrace.Len())
	// Collect per-shard metrics
	fixture.lazyTrace.Apply(func(i uint, shard zkcdriver.Shard) {
		for moduleID := range shard.Width() {
			module := shard.Module(moduleID)
			fixture.traceRows[i] += uint64(module.Height())
			fixture.traceCells[i] += uint64(module.Height()) * uint64(module.Width())
		}
	})
	//
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
		_, maybeTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			vm.DEFAULT_TRACE_CONFIG,
		)
		if len(errs) > 0 {
			b.Fatalf("tracing R5 program: %v", errors.Join(errs...))
		}
		r5TraceSink = maybeTrace.Unwrap()
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
		_, maybeTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			vm.DEFAULT_TRACE_CONFIG,
		)
		if len(errs) > 0 {
			b.Fatalf("tracing R5 program: %v", errors.Join(errs...))
		} else if failures, errs := fixture.binFile.Check(vm.DEFAULT_TRACE_CONFIG, maybeTrace.Unwrap()); len(failures) > 0 || len(errs) > 0 {
			// failures are constraint failures, whilst errs are internal ZkC problems.
			for _, e := range failures {
				errs = append(errs, errors.New(e.Message()))
			}
			b.Fatalf("checking R5 trace: %s", errors.Join(errs...))
		}
		//
		r5TraceSink = maybeTrace.Unwrap()
	}
	//
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
		fixture.lazyTrace.Apply(func(_ uint, shard zkcdriver.Shard) {
			// FIXME: this would appear to be broken, in that it is storing all
			// data from all shards in the runtime before progressing.
			// Unfortunately, this means it is memory consumpion is linear in
			// the number of shards and is not making use of the lazy tracing
			// API -- djp.
			zkcdriver.AssignFromTraceShard(
				wiop.NewRuntime(fixture.system),
				shard,
				fixture.binFile.AirConstraints(),
				koalafield.Octuplet{},
			)
		})
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
		fixture.lazyTrace = traces
		fixture.lazyTrace.Apply(func(_ uint, shard zkcdriver.Shard) {
			// FIXME: this would appear to be broken, in that it is storing all
			// data from all shards in the runtime before progressing.
			// Unfortunately, this means it is memory consumpion is linear in
			// the number of shards and is not making use of the lazy tracing
			// API -- djp.
			zkcdriver.AssignFromTraceShard(
				wiop.NewRuntime(fixture.system),
				shard,
				fixture.binFile.AirConstraints(),
				koalafield.Octuplet{},
			)
		})
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
		if _, err := embedded.CompiledBinaryFile(); err != nil {
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
	shard := traceSingleShard(b, fixture.driver, inputs)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
			fixture.driver.AssignTraceShard(rt, shard, placeholderSharedRandomness)
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
	shard := traceSingleShard(b, fixture.driver, inputs)

	proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
		fixture.driver.AssignTraceShard(rt, shard, placeholderSharedRandomness)
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
		binFile, err := embedded.CompiledBinaryFile()
		if err != nil {
			b.Fatalf("compiling R5 ZKC program: %v", err)
		}
		fixture.serialized, err = binFile.MarshalBinary()
		if err != nil {
			b.Fatalf("serializing R5 constraints: %v", err)
		}

		var (
			system, driver = compileR5BenchmarkSystem(b, fixture.serialized)
			inputs         = &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
			lazyTrace      = driver.TraceZkcInputs(inputs)
			proofs         = make([]wiop.Proof, lazyTrace.Len())
			pubs           = make([]wiop.PublicInput, lazyTrace.Len())
		)
		// Lazy proof construction for shards.
		lazyTrace.Apply(func(i uint, shard zkcdriver.Shard) {
			proofs[i], pubs[i] = system.Prove(func(rt *wiop.Runtime) {
				driver.AssignTraceShard(rt, shard, placeholderSharedRandomness)
			})
		})
		//
		for i := range proofs {
			if err := system.Verify(proofs[i], pubs[i]); err != nil {
				b.Fatalf("verifying R5 proof: %v", err)
			}
		}

		r5ProofSink, r5PubSink = proofs, pubs
	}
	reportR5Work(b, fixture)
}

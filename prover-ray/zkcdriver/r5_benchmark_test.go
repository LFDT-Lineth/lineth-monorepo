package zkcdriver_test

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"testing"

	zkcr5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

const (
	r5ZKCPath      = "../../arithmetization/src/main/riscv/main.zkc"
	r5VerifierPath = "../../verifier-ray/zig-out/bin/verifier-ray"
)

var (
	r5TraceSink trace.Trace[koalabear.Element]
	r5ProofSink wiop.Proof
	r5PubSink   wiop.PublicInput
)

type r5BenchmarkFixture struct {
	binFile       *constraints.BinaryFile[koalabear.Element]
	inputs        map[string][]byte
	expandedTrace trace.Trace[koalabear.Element]
	serialized    []byte
	system        *wiop.System
	driver        *zkcdriver.ZkCDriver
	traceRows     uint64
	traceCells    uint64
}

func loadR5BenchmarkFixture(b *testing.B) *r5BenchmarkFixture {
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
	_, _, expandedTrace, errs := binFile.Trace(inputs, constraints.DEFAULT_TRACE_CONFIG)
	if len(errs) > 0 {
		b.Fatalf("tracing R5 fixture: %v", errors.Join(errs...))
	}
	serialized, err := binFile.MarshalBinary()
	if err != nil {
		b.Fatalf("serializing R5 constraints: %v", err)
	}
	fixture := &r5BenchmarkFixture{
		binFile:       binFile,
		inputs:        inputs,
		expandedTrace: expandedTrace,
		serialized:    serialized,
	}
	for moduleID := range expandedTrace.Width() {
		module := expandedTrace.Module(moduleID)
		fixture.traceRows += uint64(module.Height())
		fixture.traceCells += uint64(module.Height()) * uint64(module.Width())
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
	b.ReportMetric(float64(fixture.traceRows), "trace-rows/op")
	b.ReportMetric(float64(fixture.traceCells), "trace-cells/op")
	b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
}

// BenchmarkR5Trace measures RISC-V execution and AIR trace expansion. It does
// not check constraints, assign WIOP columns, prove, or verify.
func BenchmarkR5Trace(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _, expandedTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			constraints.DEFAULT_TRACE_CONFIG,
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
		_, _, expandedTrace, errs := fixture.binFile.Trace(
			fixture.inputs,
			constraints.DEFAULT_TRACE_CONFIG,
		)
		if len(errs) > 0 {
			b.Fatalf("tracing R5 program: %v", errors.Join(errs...))
		}
		if failures := fixture.binFile.Check(
			expandedTrace,
			constraints.DEFAULT_TRACE_CONFIG,
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
		rt := wiop.NewRuntime(fixture.system)
		zkcdriver.AssignFromTrace(
			rt,
			fixture.expandedTrace,
			fixture.binFile.AirConstraints(),
		)
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
		rt := wiop.NewRuntime(fixture.system)
		fixture.driver.AssignWithPreRead(rt, inputs)
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
func BenchmarkR5Prove(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	inputs := &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
			fixture.driver.AssignWithPreRead(rt, inputs)
		})
		r5ProofSink, r5PubSink = proof, pub
	}
	reportR5Work(b, fixture)
}

// BenchmarkR5Verify measures verification of one proof produced before the
// timer starts. The immutable proof and public input are reused.
func BenchmarkR5Verify(b *testing.B) {
	fixture := loadR5BenchmarkFixture(b)
	fixture.ensureSystem(b)
	inputs := &zkcdriver.PreReadInputs{Inputs: fixture.inputs}
	proof, pub := fixture.system.Prove(func(rt *wiop.Runtime) {
		fixture.driver.AssignWithPreRead(rt, inputs)
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
		system, driver := compileR5BenchmarkSystem(b, serialized)
		proof, pub := system.Prove(func(rt *wiop.Runtime) {
			driver.AssignWithPreRead(rt, &zkcdriver.PreReadInputs{Inputs: fixture.inputs})
		})
		if err := system.Verify(proof, pub); err != nil {
			b.Fatalf("verifying R5 proof: %v", err)
		}
		r5ProofSink, r5PubSink = proof, pub
	}
	reportR5Work(b, fixture)
}

package zkcdriver_test

import (
	"bytes"
	"errors"
	"sync"
	"syscall"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// r5WorkloadInstrPerShard is the shard size of the workload benchmarks, in
// interpreter() invocations (= executed RISC-V instructions), matching
// BenchmarkR5Fibonacci. At 500K a full Fibonacci shard already puts
// $bit_xoa_u16 at the 2^22-row cap.
const r5WorkloadInstrPerShard = 250_000

// r5Workload is a guest exercising one corner of the R5 arithmetization. Each
// is sized to run in a single, nearly full shard: until cross-shard binding
// lands, a shard followed by others leaves its interpreter bus call open and
// does not verify standalone, so the whole program has to fit in shard 0.
type r5Workload struct {
	name string
	elf  func() (elf, output []byte)
}

var r5Workloads = []r5Workload{
	// Mixed: every instruction class once; a tiny shard (~3M cells).
	{"allinone", minimalelf.AllInOneElfProgram},
	// ALU- and branch-bound, no memory traffic: 14+5N = 245K instructions.
	{"fibonacci", func() ([]byte, []byte) { return minimalelf.FibonacciELF(49_000) }},
	// RAM-bound: N stores then N loads at distinct addresses, ~9N = 243K
	// instructions.
	{"memory", func() ([]byte, []byte) { return minimalelf.MemoryELF(27_000) }},
	// Precompile-bound: 2N Keccak permutations in ~4N instructions. Keccak
	// rows are not bounded by the instruction-count sharding, so N is kept
	// small enough for the tallest module to stay within the PCS limit.
	{"keccak", func() ([]byte, []byte) { return minimalelf.KeccakELF(100) }},
}

type r5WorkloadFixture struct {
	shard trace.Shard[koalabear.Element]
	rows  uint64
	cells uint64
}

var (
	r5WorkloadMu       sync.Mutex
	r5WorkloadFixtures = map[string]*r5WorkloadFixture{}
	r5WorkloadSystem   *wiop.System
	r5WorkloadDriver   *zkcdriver.ZkCDriver
)

// loadR5Workload traces w once per process and compiles the shared proving
// system on first use: the system depends only on the constraints, not on
// the guest, so every workload proves against the same one.
func loadR5Workload(b *testing.B, w r5Workload) *r5WorkloadFixture {
	b.Helper()
	r5WorkloadMu.Lock()
	defer r5WorkloadMu.Unlock()
	if fx, ok := r5WorkloadFixtures[w.name]; ok {
		return fx
	}

	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		b.Fatalf("compiling R5 ZKC program: %v", err)
	}
	if r5WorkloadSystem == nil {
		serialized, err := binf.MarshalBinary()
		if err != nil {
			b.Fatalf("serializing R5 constraints: %v", err)
		}
		r5WorkloadSystem, r5WorkloadDriver = compileR5BenchmarkSystem(b, serialized)
	}

	elf, want := w.elf()
	inputs, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		b.Fatalf("%s: preparing inputs: %v", w.name, err)
	}
	cfg := vm.DEFAULT_TRACE_CONFIG.
		WithSharding(vm.NewShardingStrategy("interpreter", r5WorkloadInstrPerShard)).
		WithParallelism(true)
	outputs, traces, errs := binf.Trace(inputs, cfg)
	if len(errs) > 0 {
		b.Fatalf("%s: tracing: %v", w.name, errors.Join(errs...))
	}
	if got := outputs["guest_output"]; !bytes.Equal(got, want) {
		b.Fatalf("%s: guest_output = %x, want %x", w.name, got, want)
	}

	if len(traces) != 1 {
		b.Fatalf("%s: traced into %d shards; it must fit in one to verify standalone", w.name, len(traces))
	}
	fx := &r5WorkloadFixture{shard: traces[0]}
	var tallest uint
	for m := range fx.shard.Width() {
		mod := fx.shard.Module(m)
		fx.rows += uint64(mod.Height())
		fx.cells += uint64(mod.Height()) * uint64(mod.Width())
		tallest = max(tallest, mod.Height())
	}
	if limit := int(pcs.FRIMaxCommittableSizeLog2()); log2ceil(tallest) > limit {
		b.Fatalf("%s: tallest module is 2^%d rows, above the 2^%d PCS limit", w.name, log2ceil(tallest), limit)
	}
	b.Logf("%s: %d rows, %d cells, tallest module 2^%d", w.name, fx.rows, fx.cells, log2ceil(tallest))
	r5WorkloadFixtures[w.name] = fx
	return fx
}

func (fx *r5WorkloadFixture) prove() (wiop.Proof, wiop.PublicInput) {
	return r5WorkloadSystem.Prove(func(rt *wiop.Runtime) {
		r5WorkloadDriver.AssignTraceShard(rt, fx.shard, placeholderSharedRandomness)
	})
}

// processCPU returns the user plus system CPU time consumed so far by the
// whole process, all goroutines and the GC included.
func processCPU(b *testing.B) float64 {
	b.Helper()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		b.Fatalf("getrusage: %v", err)
	}
	return float64(ru.Utime.Nano()+ru.Stime.Nano()) / 1e9
}

func (fx *r5WorkloadFixture) report(b *testing.B) {
	b.Helper()
	b.ReportMetric(float64(fx.rows), "trace-rows/op")
	b.ReportMetric(float64(fx.cells), "trace-cells/op")
}

// BenchmarkR5ProveWorkloads proves shard 0 of each [r5Workloads] guest on a
// precompiled system, including column assignment as BenchmarkR5Prove does.
// Tracing and compilation happen once, outside the timer.
func BenchmarkR5ProveWorkloads(b *testing.B) {
	for _, w := range r5Workloads {
		b.Run(w.name, func(b *testing.B) {
			fx := loadR5Workload(b, w)
			b.ReportAllocs()
			b.ResetTimer()
			cpu0 := processCPU(b)
			for b.Loop() {
				proof, pub := fx.prove()
				r5ProofSink, r5PubSink = []wiop.Proof{proof}, []wiop.PublicInput{pub}
			}
			b.ReportMetric((processCPU(b)-cpu0)/float64(b.N), "cpu-s/op")
			fx.report(b)
		})
	}
}

// BenchmarkR5VerifyWorkloads verifies one shard-0 proof of each
// [r5Workloads] guest, produced before the timer starts.
func BenchmarkR5VerifyWorkloads(b *testing.B) {
	for _, w := range r5Workloads {
		b.Run(w.name, func(b *testing.B) {
			fx := loadR5Workload(b, w)
			proof, pub := fx.prove()
			if err := r5WorkloadSystem.Verify(proof, pub); err != nil {
				b.Fatalf("%s: verifying setup proof: %v", w.name, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := r5WorkloadSystem.Verify(proof, pub); err != nil {
					b.Fatalf("%s: verifying: %v", w.name, err)
				}
			}
			fx.report(b)
		})
	}
}

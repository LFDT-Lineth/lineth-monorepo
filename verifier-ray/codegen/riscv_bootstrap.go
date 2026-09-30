package codegen

import (
	"bytes"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
)

// HonestRiscvArtifacts are the verifier-facing outputs from compiling the real
// RISC-V main.zkc entrypoint and proving an honest minimal guest witness.
type HonestRiscvArtifacts struct {
	CompiledSystem CompiledSystem
	VerifyInput    proofserialization.VerifyInput
}

// honestSharedRandomness is the γ seed passed to AssignTraceShard, whose
// signature requires one.
//
// It is currently inert: runCompilePipeline compiles with
// messagebus.CompileOptions.SharedRandomness off (see the comment there), so no
// γ cell is declared and zkcdriver.AssignFromTraceShard — which writes the seed
// only when messagebus.HasSharedRandomness reports one — skips it. The shard
// derives α and β from its own Fiat-Shamir transcript instead, which is correct
// for the single shard this fixture proves.
//
// The value is kept rather than zeroed so that re-enabling the option needs no
// new constant. It is arbitrary; it only has to be a fixed non-zero constant so
// the artifacts stay byte-reproducible. Real shards get their γ from the
// aggregation layer, which is what makes the sampled coins agree across shards.
var honestSharedRandomness = koalafield.Octuplet{
	koalafield.NewElement(11), koalafield.NewElement(22),
	koalafield.NewElement(33), koalafield.NewElement(44),
	koalafield.NewElement(55), koalafield.NewElement(66),
	koalafield.NewElement(77), koalafield.NewElement(88),
}

// BuildAllInOneHonestRiscvArtifacts compiles the real main.zkc entrypoint,
// proves zkc_r5.AllInOneGuestELF against it — a single honest, halting guest
// that concatenates the full RV64I + M-extension + custom-precompile surface
// exercised piecemeal by prover-ray/zkcdriver/r5_test.go's
// TestRisc5InstructionCoverageGuest, but through the real prove/PCS/verify
// pipeline rather than trace-and-check-constraints alone — and returns the
// verifier-facing artifacts derived from it.
func BuildAllInOneHonestRiscvArtifacts() (HonestRiscvArtifacts, error) {
	binF, err := embedded.CompiledBinaryFile()
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("compiling embedded binary: %w", err)
	}
	compiledConstraints, err := binF.MarshalBinary()
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("marshaling embedded binary constraints: %w", err)
	}

	sys := wiop.NewSystemf("zkc-riscv-system")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	runCompilePipeline(sys)

	compiledSystem, err := BuildCompiledSystem(sys)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("BuildCompiledSystem: %w", err)
	}

	// The witness is a real halting guest ELF, not a synthetic verifier fixture.
	honestInputs, err := predecoding.PrepareInputs(minimalelf.AllInOneElfProgram, nil)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("PrepareInput: %w", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: honestInputs}

	// Tracing is hoisted out of the assignment closure: it depends only on the
	// inputs, not on the runtime. This guest fits a single shard.
	traces := driver.TraceZkcInputs(inputs)
	if len(traces) != 1 {
		return HonestRiscvArtifacts{}, fmt.Errorf("expected a single trace shard, got %d", len(traces))
	}

	proof, pub := sys.Prove(
		func(assignRt *wiop.Runtime) {
			driver.AssignTraceShard(assignRt, traces[0], honestSharedRandomness)
		},
		wiop.ProveOptions{CheckUnreducedQueries: true},
	)
	if err := sys.Verify(proof, pub); err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("verifying proof: %w", err)
	}
	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("AssertAllVerifierActionsHandled: %w", err)
	}

	verifyInput, err := proofserialization.Project(sys, proof, pub)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("proofserialization.Project: %w", err)
	}

	return HonestRiscvArtifacts{
		CompiledSystem: compiledSystem,
		VerifyInput:    verifyInput,
	}, nil
}

func runCompilePipeline(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	// SharedRandomness is deliberately off. It obliges every bus column to sit on
	// the message-bus coin round, so that the round's commitment — hashed into
	// this shard's contribution to γ — binds the columns the coins are then used
	// to evaluate. zkcdriver declares every trace column on round 0
	// (zkcdriver/definition.go, zkcdriver/native_modules.go), while the coin round
	// is round 0's successor, so the layouts are incompatible and the option
	// panics here.
	//
	// Turning it on would be vacuous anyway: BuildAllInOneHonestRiscvArtifacts
	// asserts a single trace shard, and shared randomness exists to make several
	// shards agree on α and β. With one shard there is no one to agree with, and
	// the coins are correctly drawn from this shard's own transcript.
	//
	// Re-enabling it requires moving the driver's bus columns onto the coin round,
	// which is blocked on dynamic-module sizing: wiop.Runtime.AssignColumn fixes a
	// dynamic module's size from its round-0 assignment and panics if a later round
	// grows it, because the sizes go into round 0's Fiat-Shamir transcript.
	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: false})
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)
}

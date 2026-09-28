package codegen

import (
	"bytes"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
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

// honestSharedRandomness is the γ seed handed to the shard being proved.
//
// sys is compiled with the message-bus pass's shared-randomness mode disabled
// (the ZKC driver builds every bus participant column on round 0, while the
// seeded mode requires them all on the coin round), so this value never
// reaches a γ cell; AssignTraceShard only uses it when
// messagebus.HasSharedRandomness(sys) holds. It stays a fixed non-zero
// constant so the artifacts remain byte-reproducible.
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
	// The driver puts bus participant columns on round 0, so seeded mode's
	// coin-round placement is not applicable to this unsharded protocol.
	if err := compilers.CompileFull(sys,
		compilers.WithMessageBusOption(messagebus.WithoutSharedRandomness())); err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("compiling pipeline: %w", err)
	}

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

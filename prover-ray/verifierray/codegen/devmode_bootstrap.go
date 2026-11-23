package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// SampleDevmodePublicInputs returns deterministic base-field public inputs
// (1000 + i) for devmode fixture generation and tests.
func SampleDevmodePublicInputs() wiop.PublicInput {
	pub := make(wiop.PublicInput, devmode.NumPublicInputs)
	for i := range pub {
		var e field.Element
		e.SetUint64(uint64(1000 + i))
		pub[i] = field.ElemFromBase(e)
	}
	return pub
}

// BuildDevmodeHonestArtifacts compiles the devmode circuit from prover-ray,
// proves a deterministic 40-cell statement, verifies it in Go, and returns both
// the extracted CompiledSystem and the projected VerifyInput.
func BuildDevmodeHonestArtifacts() (HonestRiscvArtifacts, error) {
	sys, err := devmode.NewSystem()
	if err != nil {
		return HonestRiscvArtifacts{}, err
	}
	compiledSystem, err := BuildCompiledSystem(sys)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("BuildCompiledSystem: %w", err)
	}

	proof, pub, err := devmode.Prove(sys, SampleDevmodePublicInputs())
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("devmode.Prove: %w", err)
	}
	if err := devmode.Verify(sys, proof, pub); err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("verifying devmode proof: %w", err)
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

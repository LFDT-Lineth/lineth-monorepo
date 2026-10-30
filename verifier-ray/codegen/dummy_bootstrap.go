package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

const (
	// DummyNumGuestOutputCells is the number of guest-output public inputs:
	// one per bound output byte.
	DummyNumGuestOutputCells = 32
	// DummyNumProgramVKCells is the number of program-VK public inputs:
	// one per limb of the program-round commitment.
	DummyNumProgramVKCells = 8
	// DummyNumPublicInputs is the total number of public inputs of the dummy
	// circuit, matching the RISC-V conglomeration public-input layout (40 cells).
	DummyNumPublicInputs = DummyNumGuestOutputCells + DummyNumProgramVKCells

	// DummyModuleSize is the static domain size of the single dummy module.
	DummyModuleSize = 2

	// DummyFRINumQueries is the number of FRI query openings used by the dummy PCS.
	DummyFRINumQueries = 1
)

// NewDummySystem builds and compiles a minimal wiop.System through the full
// compilation pipeline (localvanishing -> global -> pcs) whose public-input
// layout matches the production RISC-V system (32 guest-output cells + 8
// program-VK cells).
//
// It uses a single static module of size 2 with 1 base-field column on round 0
// and a single vanishing query binding the 40 public-input cells. Because it
// goes through compilers.CompileFull, verifier-ray's codegen and Zig verifier
// (`verifier.verify` and `verifier.verifyPair`) verify its proofs directly with
// zero verifier code duplication.
func NewDummySystem() (*wiop.System, error) {
	sys := wiop.NewSystemf("dummy")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf("dummy")
	mod := sys.NewSizedModule(ctx.Childf("mod"), DummyModuleSize, wiop.PaddingDirectionNone)
	col := mod.NewColumn(ctx.Childf("col"), r0)

	var sum wiop.Expression
	for i := range DummyNumPublicInputs {
		tag, n := wiop.PublicInputTag("guest_public_outputs"), i
		if i >= DummyNumGuestOutputCells {
			tag, n = wiop.PublicInputTag("program_vk"), i-DummyNumGuestOutputCells
		}
		cell := r0.NewCell(ctx.Childf("x-%d", i), false)
		sys.RegisterPublicInputs(tag, cell, n)
		if i == 0 {
			sum = cell
		} else {
			sum = wiop.Add(sum, cell)
		}
	}
	mod.NewVanishing(ctx.Childf("vanishing"), wiop.Sub(col.View(), sum))
	if err := compilers.CompileFull(sys,
		compilers.ConfigurePCS(
			pcs.WithFRINumQueries(DummyFRINumQueries),
			pcs.WithColumnElision(),
		),
	); err != nil {
		return nil, fmt.Errorf("compiling dummy system: %w", err)
	}
	return sys, nil
}

// ProveDummy produces a valid proof for the compiled dummy system and the given
// 40 base-field public inputs.
func ProveDummy(sys *wiop.System, pub wiop.PublicInput) (wiop.Proof, wiop.PublicInput, error) {
	if len(pub) != DummyNumPublicInputs {
		return wiop.Proof{}, nil, fmt.Errorf("got %d public inputs, want %d", len(pub), DummyNumPublicInputs)
	}
	for i, x := range pub {
		if !x.IsBase() {
			return wiop.Proof{}, nil, fmt.Errorf("public input %d must be a base-field element", i)
		}
	}
	proof, provedPub := sys.Prove(func(rt *wiop.Runtime) {
		var sum field.Element
		for i, x := range pub {
			rt.AssignCell(sys.Rounds[0].Cells[i], x)
			v := x.AsBase()
			sum.Add(&sum, &v)
		}
		vals := make([]field.Element, DummyModuleSize)
		for r := range vals {
			vals[r] = sum
		}
		rt.AssignColumn(sys.Rounds[0].Columns[0], &wiop.ConcreteVector{Plain: field.VecFromBase(vals)})
	})
	return proof, provedPub, nil
}

// SampleDummyPublicInputs returns deterministic base-field public inputs
// (1000 + i) for dummy fixture generation and tests.
func SampleDummyPublicInputs() wiop.PublicInput {
	pub := make(wiop.PublicInput, DummyNumPublicInputs)
	for i := range pub {
		var e field.Element
		e.SetUint64(uint64(1000 + i))
		pub[i] = field.ElemFromBase(e)
	}
	return pub
}

// BuildDummyHonestArtifacts compiles the dummy circuit, proves a deterministic
// 40-cell statement, verifies it in Go, and returns both the extracted
// CompiledSystem and the projected VerifyInput.
func BuildDummyHonestArtifacts() (HonestRiscvArtifacts, error) {
	sys, err := NewDummySystem()
	if err != nil {
		return HonestRiscvArtifacts{}, err
	}
	compiledSystem, err := BuildCompiledSystem(sys)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("BuildCompiledSystem: %w", err)
	}

	proof, pub, err := ProveDummy(sys, SampleDummyPublicInputs())
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("ProveDummy: %w", err)
	}
	if err := sys.Verify(proof, pub); err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("verifying dummy proof: %w", err)
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

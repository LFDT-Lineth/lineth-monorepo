// Package devmode provides a minimal wiop proof system for devmode.
//
// The wiop proof system is a fully-compiled wiop proof system for dev-mode and
// test builds whose public-input layout matches the root of the conglomeration
// tree: the guest-output hash (32 cells, one per bound output byte) followed by
// the program verification key (8 cells, one per limb of the program-round
// commitment).
//
// Because the circuit goes through the standard prover-ray compilation pipeline
// (localvanishing -> global -> pcs), verifier-ray's codegen and Zig verifier
// (`verifier.verify`) verify its proofs directly without a separate or
// duplicated Zig verifier implementation, and callers in prover-ray can
// construct and prove the circuit without importing verifier-ray.
//
// The circuit consists of a single static module of size 2 with one base-field
// column on round 0 and a single vanishing constraint (`col - sum(x_0..x_39) = 0`)
// binding the 40 public-input cells, compiled with 1 FRI query, column elision,
// and optimal Merkle cap height.
//
// Test/dev builds only: anyone who knows the public inputs X can produce a
// valid devmode proof for X, so it is not sound for proving guest execution.
package devmode

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

const (
	// NumGuestOutputCells is the number of guest-output public inputs:
	// one per bound output byte.
	NumGuestOutputCells = risc5.NumGuestPublicOutputs
	// NumProgramVKCells is the number of program-VK public inputs:
	// one per limb of the program-round commitment.
	NumProgramVKCells = risc5.NumProgramVK
	// NumPublicInputs is the total number of public inputs of the devmode
	// circuit, matching the RISC-V conglomeration public-input layout (40 cells).
	NumPublicInputs = NumGuestOutputCells + NumProgramVKCells

	// ModuleSize is the static domain size of the single devmode module.
	ModuleSize = 2

	// FRINumQueries is the number of FRI query openings used by the devmode PCS.
	FRINumQueries = 1

	// GuestBase is the default base address images are encoded for: the start
	// of the guest's input region.
	GuestBase = proofserialization.GuestBase
)

// NewSystem builds and compiles the minimal devmode wiop.System through the
// full compilation pipeline (localvanishing -> global -> pcs) whose
// public-input layout matches the production RISC-V system (32 guest-output
// cells + 8 program-VK cells).
//
// If no compiler options are passed, NewSystem uses the small devmode PCS
// defaults (FRINumQueries = 1, column elision, and optimal Merkle cap height).
// See also the restrictions on the PCS options in verifier-ray.
func NewSystem(opts ...compilers.Option) (*wiop.System, error) {
	sys := wiop.NewSystemf("devmode")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf("devmode")
	mod := sys.NewSizedModule(ctx.Childf("mod"), ModuleSize, wiop.PaddingDirectionNone)
	col := mod.NewColumn(ctx.Childf("col"), r0)

	var sum wiop.Expression
	for i := range NumPublicInputs {
		tag, n := risc5.GuestPublicOutputsPI, i
		if i >= NumGuestOutputCells {
			tag, n = risc5.ProgramVKPI, i-NumGuestOutputCells
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
	if len(opts) == 0 {
		opts = []compilers.Option{
			compilers.ConfigurePCS(
				pcs.WithFRINumQueries(FRINumQueries),
				pcs.WithColumnElision(),
				pcs.WithOptimalMerkleCap(),
			),
		}
	}
	if err := compilers.CompileFull(sys, opts...); err != nil {
		return nil, fmt.Errorf("devmode: CompileFull: %w", err)
	}
	return sys, nil
}

// Prove produces a valid proof for the compiled devmode system and the given
// 40 base-field public inputs.
func Prove(sys *wiop.System, pub wiop.PublicInput) (wiop.Proof, wiop.PublicInput, error) {
	if len(pub) != NumPublicInputs {
		return wiop.Proof{}, nil, fmt.Errorf("devmode: Prove: got %d public inputs, want %d", len(pub), NumPublicInputs)
	}
	for i, x := range pub {
		if !x.IsBase() {
			return wiop.Proof{}, nil, fmt.Errorf("devmode: Prove: public input %d must be a base-field element", i)
		}
	}
	proof, provedPub := sys.Prove(func(rt *wiop.Runtime) {
		var sum field.Element
		for i, x := range pub {
			rt.AssignCell(sys.Rounds[0].Cells[i], x)
			v := x.AsBase()
			sum.Add(&sum, &v)
		}
		vals := make([]field.Element, ModuleSize)
		for r := range vals {
			vals[r] = sum
		}
		rt.AssignColumn(sys.Rounds[0].Columns[0], &wiop.ConcreteVector{Plain: field.VecFromBase(vals)})
	})
	return proof, provedPub, nil
}

// Verify is the Go reference verifier of a devmode proof.
func Verify(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) error {
	return sys.Verify(proof, pub)
}

// Encode writes the image of a devmode proof and its public inputs, encoded for
// base, with proofserialization like any ray proof.
//
// Argument base corresponds to the base address of the proof for efficient
// zero-decoding format of the proof for downstream verifier-ray usage. Users
// should use [GuestBase] when not specified differently.
func Encode(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput, base uint64) ([]byte, error) {
	input, err := proofserialization.Project(sys, proof, pub)
	if err != nil {
		return nil, err
	}
	return proofserialization.Encode(input, base)
}

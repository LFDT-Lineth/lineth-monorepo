package main

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

// recursionE6Tag is the public-input tag of the forged E6 value.
const recursionE6Tag wiop.PublicInputTag = "Y"

// buildRecursionE6Case builds the RecursionE6PublicInput fixture: a recursion
// program (package recursion) exposing a scalar S, an E6 input X and the E6
// product Y = S·X as public inputs, so that it registers one
// recursion.E6Check per E6 value.
//
// The honest proof exercises e6check.verify through verifier.verify. The
// self-consistent failing proof (invalidVerify) forges Y through its cell's
// assigner, as prover-ray's TestPublic_Rejects does: the prover then commits
// to and opens the honest lanes and claims Y+1 in a consistent transcript, so
// the PCS, vanishing and every other sub-verifier accept and only the E6 check
// can reject — error.E6Mismatch pins that it does.
func buildRecursionE6Case() (fixtureCase, codegen.CompiledSystem, error) {
	const source, name = "Recursion", "RecursionE6PublicInput"
	m := &recursion.Machine{}
	s, x := m.InputScalar(), m.InputE6()
	y := m.Mul(s, x)
	m.Public(s, recursion.KindScalar, "S", 0)
	m.Public(x, recursion.KindE6, "X", 0)
	m.Public(y, recursion.KindE6, recursionE6Tag, 0)

	// The fixture PCS options of compileFullPipeline.
	p, err := m.Compile(compilers.ConfigurePCS(
		pcscompiler.WithFRINumQueries(4),
		pcscompiler.WithOptimalMerkleCap(),
	))
	if err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("compile %s/%s: %w", source, name, err)
	}
	if err := codegen.AssertAllVerifierActionsHandled(p.Sys); err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("verifier actions %s/%s: %w", source, name, err)
	}
	compiled, err := codegen.BuildCompiledSystem(p.Sys)
	if err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("build compiled system %s/%s: %w", source, name, err)
	}
	if got := len(compiled.E6Check.Checks); got != 2 {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("%s/%s: %d E6 checks, want 2", source, name, got)
	}

	var xv recursion.Cell
	for i := range recursion.KindE6 {
		xv[i].SetUint64(uint64(i) + 11)
	}
	run, err := p.Execute([]recursion.Cell{recursion.ScalarCell(field.NewElement(4)), xv})
	if err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("execute %s/%s: %w", source, name, err)
	}

	proof, pub := p.Prove(run)
	honest, err := proofFixtureFromProof(p.Sys, proof, pub, source, name, "honest")
	if err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, err
	}

	cell, _ := p.Sys.LookupPublicInputByTag(recursionE6Tag, 0)
	if cell == nil || cell.Assigner == nil {
		return fixtureCase{}, codegen.CompiledSystem{}, fmt.Errorf("%s/%s: public input %s has no assigner", source, name, recursionE6Tag)
	}
	honestAssigner := cell.Assigner
	cell.Assigner = func(rt *wiop.Runtime) field.Gen { return honestAssigner(rt).Add(field.ElemOne()) }
	proof, pub = p.Prove(run)
	forged, err := proofFixtureFromProof(p.Sys, proof, pub, source, name, "invalid-verify")
	cell.Assigner = honestAssigner
	if err != nil {
		return fixtureCase{}, codegen.CompiledSystem{}, err
	}

	tc := fixtureCase{
		name:               name,
		pcs:                compiled.Pcs,
		honest:             honest,
		invalidVerify:      &forged,
		invalidVerifyError: "E6Mismatch",
	}
	return tc, compiled, nil
}

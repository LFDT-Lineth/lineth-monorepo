// Package main generates the end-to-end verifier fixtures from prover-ray.
//
// The generator is intentionally written in Go because prover-ray is the source
// of truth for protocol behavior. Each fixture case compiles a small protocol
// through prover-ray's full pipeline, proves it, and is written with its
// compiled verifier system to testdata/generated/verify.zig (-out). The Zig
// tests drive verifier.verify over every case: honest proofs must verify,
// tampered ones must be rejected. Run it from verifier-ray/ (make
// generate-testdata).
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/verifierray/codegen"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
)

func main() {
	out := flag.String("out", filepath.Join("testdata", "generated", "verify.zig"), "path of the generated fixtures")
	imageOut := flag.String("image-out", filepath.Join("testdata", "fixture_proof_image.bin"),
		"path of the proof image of one fixture case (see writeFixtureProofImage)")
	flag.Parse()

	fixtureCases, compiledSystems, err := buildCompiledFixtureCases()
	if err != nil {
		panic(err)
	}
	imageCase, err := writeFixtureProofImage(*imageOut, fixtureCases, compiledSystems)
	if err != nil {
		panic(err)
	}
	if err := writeVerifyFixtures(*out, fixtureCases, compiledSystems, imageCase); err != nil {
		panic(err)
	}
}

// runtimeTraceRound is the verifier-visible transcript input for one WIOP
// round. prover-ray columns never travel raw: every committed round is represented by
// a single Merkle root, and non-committed rounds carry none at all.
type runtimeTraceRound struct {
	commitment *field.Octuplet
	cells      []runtimeTraceCell
}

// runtimeTraceCell records a scalar value opened in the round. Unlike internal
// columns, cells are always verifier-visible in prover-ray AdvanceRound and must
// be assigned.
type runtimeTraceCell struct {
	baseValue *field.Element
	extValue  *field.Ext
}

func baseTraceCell(value field.Element) runtimeTraceCell {
	return runtimeTraceCell{baseValue: &value}
}

func extTraceCell(value field.Ext) runtimeTraceCell {
	return runtimeTraceCell{extValue: &value}
}

type fixtureCase struct {
	name string
	// pcs is the compile-time PCS System baked once for this case; every proof
	// of this case (honest, invalid, alt) verifies against it.
	pcs     *codegen.PcsSystem
	honest  proofFixture
	invalid *proofFixture
	// invalidVerify is a SELF-CONSISTENT failing proof. Unlike invalid — which
	// breaks the vanishing quotient identity — an invalidVerify proof is PCS- and vanishing-valid and fails specifically
	// in a later sub-verifier (e.g. grandproduct.verify). It exists so a
	// sub-verifier wired into verifier.Systems (such as .grandproduct) is
	// exercised as the actual point of rejection, which a PCS-stage rejection
	// (the shared .invalid path) cannot demonstrate.
	invalidVerify *proofFixture
	// invalidVerifyError is the exact error verify.zig must return for
	// invalidVerify (e.g. "FinalProductMismatch"), so the regression pins the
	// sub-verifier that fires rather than accepting any rejection.
	invalidVerifyError string
	// alt is a second honest proof of the same compiled protocol at a
	// different dynamic-module size, verified against the same baked
	// PcsSystem. Multi-size cases only: hasAlt is the multi-size predicate.
	alt *proofFixture
	// sys and assign reproduce the honest proof, for the proof image.
	sys    *wiop.System
	assign assignFn
}

type vanishingProofView struct {
	rounds       []runtimeTraceRound
	publicInputs []runtimeTraceCell
	moduleSizes  []int
}

type assignFn func(rt *wiop.Runtime)

// proofFixture is everything a verify.zig fixture needs from one proving run:
// the vanishing proof view and the FRI opening proof. The PCS entry claims are
// NOT here: verify.zig's verifier now reconstructs them itself from
// rounds[*].cells (already carried by view), so there is nothing left for this
// fixture to precompute for them.
type proofFixture struct {
	view    vanishingProofView
	opening fri.OpeningProof
}

// buildProofFixture runs the prover for one assignment and extracts the
// fixture data that proof yields. label names the proof (e.g. "honest",
// "invalid", "alt") for error messages.
func buildProofFixture(sys *wiop.System, assign assignFn, source, name, label string) (proofFixture, error) {
	rt := runProver(sys, assign)
	if rt.PCSOpeningProof == nil {
		return proofFixture{}, fmt.Errorf("pcs %s/%s: %s runtime has no PCSOpeningProof", source, name, label)
	}
	return proofFixture{
		view:    extractVanishingProofView(sys, rt),
		opening: *rt.PCSOpeningProof,
	}, nil
}

// compileFullPipeline compiles sys through the canonical
// compilers.CompileFull pipeline, shrinking the FRI query count to 4 for
// fixtures: the production default (229) would emit ~229 Merkle branches per
// opening, ballooning verify.zig to hundreds of MB and making Zig compilation
// intractable. Four queries is sufficient to test the Merkle capping without
// ballooning the fixture sizes. pcsOpts, when given, replaces the query count
// (and adds e.g. an RS blowup); the optimal Merkle cap is always kept.
func compileFullPipeline(sys *wiop.System, pcsOpts ...pcscompiler.Option) error {
	if len(pcsOpts) == 0 {
		pcsOpts = []pcscompiler.Option{pcscompiler.WithFRINumQueries(4)}
	}
	pcsOpts = append(pcsOpts, pcscompiler.WithOptimalMerkleCap())
	return compilers.CompileFull(sys, compilers.ConfigurePCS(pcsOpts...))
}

func buildCompiledFixtureCases() ([]fixtureCase, []codegen.CompiledSystem, error) {
	var cases []fixtureCase
	var systems []codegen.CompiledSystem

	add := func(source, name string, sys *wiop.System, honest assignFn, invalid assignFn) error {
		if err := compileFullPipeline(sys); err != nil {
			return fmt.Errorf("compile %s/%s: %w", source, name, err)
		}
		// Fail closed on any verifier action the codegen does not know how to emit.
		if err := codegen.AssertAllVerifierActionsHandled(sys); err != nil {
			return fmt.Errorf("verifier actions %s/%s: %w", source, name, err)
		}
		compiled, err := codegen.BuildCompiledSystem(sys)
		if err != nil {
			return fmt.Errorf("build compiled system %s/%s: %w", source, name, err)
		}
		// Every scenario must reach verify.zig: a protocol that compiles to no
		// vanishing modules would otherwise drop out of the end-to-end sweep
		// without a generator failure.
		if len(compiled.Vanishing.Modules) == 0 {
			return fmt.Errorf("compiled system %s/%s has no vanishing modules", source, name)
		}

		honestFixture, err := buildProofFixture(sys, honest, source, name, "honest")
		if err != nil {
			return err
		}
		tc := fixtureCase{name: name, pcs: compiled.Pcs, honest: honestFixture, sys: sys, assign: honest}
		if invalid != nil {
			invalidFixture, err := buildProofFixture(sys, invalid, source, name, "invalid")
			if err != nil {
				return err
			}
			tc.invalid = &invalidFixture
		}

		cases = append(cases, tc)
		systems = append(systems, compiled)
		return nil
	}

	// addMultiSizeWith emits one baked PcsSystem plus two honest proofs at
	// different runtime sizes, compiled with pcsOpts (see compileFullPipeline).
	addMultiSizeWith := func(source, name string, sys *wiop.System, honest, alt assignFn, pcsOpts ...pcscompiler.Option) error {
		if err := compileFullPipeline(sys, pcsOpts...); err != nil {
			return fmt.Errorf("compile %s/%s: %w", source, name, err)
		}
		if err := codegen.AssertAllVerifierActionsHandled(sys); err != nil {
			return fmt.Errorf("verifier actions %s/%s: %w", source, name, err)
		}
		compiled, err := codegen.BuildCompiledSystem(sys)
		if err != nil {
			return fmt.Errorf("build compiled system %s/%s: %w", source, name, err)
		}

		honestFixture, err := buildProofFixture(sys, honest, source, name, "honest")
		if err != nil {
			return err
		}
		altFixture, err := buildProofFixture(sys, alt, source, name, "alt")
		if err != nil {
			return err
		}

		tc := fixtureCase{
			name: name, pcs: compiled.Pcs, honest: honestFixture, alt: &altFixture, sys: sys, assign: honest,
		}

		cases = append(cases, tc)
		systems = append(systems, compiled)
		return nil
	}
	addMultiSize := func(source, name string, sys *wiop.System, honest, alt assignFn) error {
		return addMultiSizeWith(source, name, sys, honest, alt)
	}

	// addPrecompiled is like add, but for a sys the caller has already
	// compiled through compilers.CompileFull itself (mirroring
	// buildMessageBusFixtureCase): it skips compileFullPipeline, which would
	// wrongly compile the already-compiled system a second time.
	addPrecompiled := func(source, name string, sys *wiop.System, honest, invalid assignFn) error {
		if err := codegen.AssertAllVerifierActionsHandled(sys); err != nil {
			return fmt.Errorf("verifier actions %s/%s: %w", source, name, err)
		}
		compiled, err := codegen.BuildCompiledSystem(sys)
		if err != nil {
			return fmt.Errorf("build compiled system %s/%s: %w", source, name, err)
		}

		honestFixture, err := buildProofFixture(sys, honest, source, name, "honest")
		if err != nil {
			return err
		}
		tc := fixtureCase{name: name, pcs: compiled.Pcs, honest: honestFixture, sys: sys, assign: honest}
		if invalid != nil {
			invalidFixture, err := buildProofFixture(sys, invalid, source, name, "invalid")
			if err != nil {
				return err
			}
			tc.invalid = &invalidFixture
		}

		cases = append(cases, tc)
		systems = append(systems, compiled)
		return nil
	}

	for _, factory := range wioptest.VanishingScenarios() {
		sc := factory()
		if err := add("Vanishing", sc.Name, sc.Sys, sc.AssignHonest, sc.AssignInvalid); err != nil {
			return nil, nil, err
		}
	}

	// Multi-size committed-dynamic coverage for one baked PcsSystem.
	if err := addMultiSizeDynFib("DynamicFibonacciMultiSize", addMultiSize); err != nil {
		return nil, nil, err
	}
	// Two independent committed dynamic modules against one baked PcsSystem.
	if err := addMultiSizeTwoDynModules(addMultiSize); err != nil {
		return nil, nil, err
	}
	// LocalVanishing is intentionally NOT generated: at the verifier level every
	// LocalVanishing scenario reduces to a vanishing System whose shape (module
	// count, dynamic modules, batch count, empty logderiv) is a strict subset of
	// the Vanishing set above. The local-vanishing *compiler* is exercised by
	// prover-ray's own tests; the verifier sees nothing new here.
	for _, factory := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := factory()
		if err := add("LogDerivativeSumCompiler", sc.Name, sc.Sys, sc.AssignWitness, nil); err != nil {
			return nil, nil, err
		}
	}
	for _, factory := range wioptest.LookupScenarios() {
		sc := factory()
		if err := add("Lookup", sc.Name, sc.Sys, sc.AssignWitness, nil); err != nil {
			return nil, nil, err
		}
	}
	// Keep one RangeCheck scenario for the 3-module / 4-batch verifier shape
	// that Lookup does not produce.
	for _, factory := range wioptest.RangeCheckCompilerScenarios() {
		sc := factory()
		if sc.Name != "DistinctBounds" {
			continue
		}
		if err := add("RangeCheckCompiler", sc.Name, sc.Sys, sc.AssignWitness, nil); err != nil {
			return nil, nil, err
		}
	}

	// Public-input end-to-end coverage. The honest fixture carries the opened cell
	// only in `public_inputs`, never in the proof rounds; the failing fixture
	// keeps the proof identical and tampers JUST the public-input statement.
	{
		sys, col := buildPublicInputSystem()
		honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(10, 20, 30, 40))) }
		if err := add("PublicInput", "OpenedCellPublicInput", sys, honest, nil); err != nil {
			return nil, nil, err
		}

		last := len(cases) - 1
		if len(cases[last].honest.view.publicInputs) == 0 {
			return nil, nil, fmt.Errorf("public input fixture produced no public inputs")
		}

		invalid := cases[last].honest
		invalid.view.publicInputs = append([]runtimeTraceCell(nil), invalid.view.publicInputs...)
		invalid.view.publicInputs[0] = baseTraceCell(elem(99))
		cases[last].invalid = &invalid

	}
	// Dynamic-module twin of the public-input scenario. The opened cell is still
	// carried only in `public_inputs`, but the module size now round-trips
	// through the proof at runtime (4, from the witness column length).
	{
		sys, col := buildDynamicPublicInputSystem()
		honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(30, 20, 10, 40))) }
		if err := add("PublicInput", "OpenedCellPublicInputDynamic", sys, honest, nil); err != nil {
			return nil, nil, err
		}

		last := len(cases) - 1
		if len(cases[last].honest.view.publicInputs) == 0 {
			return nil, nil, fmt.Errorf("dynamic public input fixture produced no public inputs")
		}

		invalid := cases[last].honest
		invalid.view.publicInputs = append([]runtimeTraceCell(nil), invalid.view.publicInputs...)
		invalid.view.publicInputs[0] = baseTraceCell(elem(99))
		cases[last].invalid = &invalid
	}

	// LagrangeSelector boundary scenario. It is constructed here rather than in
	// prover-ray's wioptest because it exercises the lagrange_selector codegen
	// and Zig evaluator path, which the pinned prover-ray module already
	// supports via wiop.NewLagrangeSelector; building it locally keeps the
	// fixture reproducible from the pinned module (see `make verify-testdata`).
	{
		sys, col := buildLagrangeSelectorBoundarySystem()
		honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(7, 99, 7, 7))) }
		invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(7, 98, 7, 7))) }
		if err := add("Vanishing", "LagrangeSelectorBoundary", sys, honest, invalid); err != nil {
			return nil, nil, err
		}
	}

	// Dynamic-module variant of the LagrangeSelectorBoundary scenario. Its
	// module size is resolved at runtime (4, from the assigned column length),
	// so it drives the Zig verifier's dynamic selector branch that the static
	// fixture above leaves uncovered.
	{
		sys, col := buildDynamicLagrangeSelectorBoundarySystem()
		honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(7, 99, 7, 7))) }
		invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, concreteBase(elems(7, 98, 7, 7))) }
		if err := add("Vanishing", "DynamicLagrangeSelectorBoundary", sys, honest, invalid); err != nil {
			return nil, nil, err
		}
	}
	// Larger MultiColumn scenario for stress profiling.
	sys, honest := buildLookupMultiColumnBenchSystem()
	if err := add("Lookup", "MultiColumnBench", sys, honest, nil); err != nil {
		return nil, nil, err
	}

	// Permutation end-to-end coverage: a real wiop.NewPermutation reduced by
	// grandproduct.Compile directly (not via the message-bus path, which is the
	// only other caller of grandproduct.Compile in this generator). This
	// exercises CheckResultIsOne and grandproduct.RowLimitAction — the
	// row-limit action was, until it was exported for the verifier-ray codegen,
	// unreachable from any fixture, so AssertAllVerifierActionsHandled below is
	// the first time this allowlist entry is actually exercised end-to-end.
	//
	// This case carries TWO failing fixtures, exercising two distinct rejection
	// paths:
	//
	// 1. invalid: built by tampering the HONEST
	//    proof's transcript — flipping the committed Z-endpoint cell for
	//    module A. That scalar is read both by the vanishing boundary
	//    constraint (pins the Z column's OOD claim to this transcript cell at
	//    the module's last row) and by grandproduct.verify (feeds
	//    ∏Z == Result), so one flip breaks the vanishing quotient identity.
	//    (In the full verifier this same
	//    proof is rejected at the PCS stage, before grandproduct.verify runs,
	//    so it does NOT exercise the .grandproduct wiring end-to-end.)
	//
	// 2. invalidVerify: a SELF-CONSISTENT proof produced by
	//    re-running the prover on a mismatched A/B witness ([1,2,3,4] vs
	//    [4,3,2,2] — not a permutation). The prover's Z column is always the
	//    honest running product OF WHATEVER witness it is given, so its own
	//    recurrence and row-0 boundary constraints still hold: the proof is
	//    PCS-valid and vanishing-valid, and is rejected specifically by
	//    grandproduct.verify's Result == 1 boundary (error.ResultMismatch).
	//    This is the fixture that actually exercises the .grandproduct system
	//    end-to-end, closing the coverage gap the shared .invalid path leaves.
	{
		sys, colA, colB, err := buildPermutationSystem()
		if err != nil {
			return nil, nil, fmt.Errorf("build permutation system: %w", err)
		}
		honest := func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, concreteBase(elems(1, 2, 3, 4)))
			rt.AssignColumn(colB, concreteBase(elems(4, 3, 2, 1)))
		}
		if err := addPrecompiled("Permutation", "GrandProductPermutation", sys, honest, nil); err != nil {
			return nil, nil, err
		}

		last := len(cases) - 1
		honestFixture := cases[last].honest
		// Round 2 ("result round") carries, in order: the shared GrandProduct
		// Result cell (index 0, expected to equal one), then one Z-endpoint cell
		// per module (index 1 for modA's z-m0-k0, index 2 for modB's z-m1-k0) —
		// see the generated grandproduct/vanishing systems for this case. Index 1
		// is modA's Z endpoint, the one this fixture flips.
		const resultRoundIdx, zEndpointModAIdx = 2, 1
		if len(honestFixture.view.rounds) <= resultRoundIdx ||
			len(honestFixture.view.rounds[resultRoundIdx].cells) <= zEndpointModAIdx {
			return nil, nil, fmt.Errorf("Permutation/GrandProductPermutation: result round has no Z-endpoint cell at index %d", zEndpointModAIdx)
		}

		invalidFixture := honestFixture
		invalidRounds := append([]runtimeTraceRound(nil), honestFixture.view.rounds...)
		invalidCells := append([]runtimeTraceCell(nil), honestFixture.view.rounds[resultRoundIdx].cells...)
		invalidCells[zEndpointModAIdx] = flippedExtTraceCell(invalidCells[zEndpointModAIdx])
		invalidRounds[resultRoundIdx] = runtimeTraceRound{
			commitment: honestFixture.view.rounds[resultRoundIdx].commitment,
			cells:      invalidCells,
		}
		invalidFixture.view.rounds = invalidRounds
		cases[last].invalid = &invalidFixture

		// Self-consistent grandproduct-failing proof: re-run the prover on a
		// mismatched A/B witness. colB is colA rotated by one EXCEPT for the
		// last element (2 instead of 1), so the two sides are not a
		// permutation of each other and the grand product does not cancel to
		// 1. PCS and vanishing still pass; only grandproduct.verify rejects.
		mismatched := func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, concreteBase(elems(1, 2, 3, 4)))
			rt.AssignColumn(colB, concreteBase(elems(4, 3, 2, 2)))
		}
		invalidVerifyFixture, err := buildProofFixture(sys, mismatched, "Permutation", "GrandProductPermutation", "invalid-verify")
		if err != nil {
			return nil, nil, err
		}
		cases[last].invalidVerify = &invalidVerifyFixture
		cases[last].invalidVerifyError = "ResultMismatch"
	}

	// Message-bus + SharedRandomness end-to-end coverage: the full
	// messagebus (seeded shared randomness by default) -> grandproduct ->
	// localvanishing -> global -> pcscompiler pipeline, run through a single
	// compilers.CompileFull call, BuildCompiledSystem and verifier.verify.
	// The invalid fixture
	// flips exactly one of the 328 shared-randomness contribution limbs in the
	// public-input statement, which shared_randomness.zig's verify() must now
	// reject.
	if err := addMessageBusSharedRandomness(func(name string, sys *wiop.System, honest assignFn) error {
		compiled, err := buildMessageBusFixtureCase(name, sys, honest)
		if err != nil {
			return err
		}
		cases = append(cases, compiled.tc)
		systems = append(systems, compiled.compiled)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// The same multi-size Fibonacci System at every non-default RS blowup, with
	// the query count prover-ray pairs it with. Every other fixture uses the
	// default blowup 2; these check that the verifier's domain sizing, its
	// envelope (FRIMaxCommittableSizeLog2 shrinks at high blowup) and query
	// sampling agree with the prover at the other rates, at two runtime sizes
	// so restrictTo runs at each rate too.
	for _, cfg := range wioptest.RSBlowupBenchConfigs() {
		if cfg.Blowup == 2 {
			continue
		}
		name := fmt.Sprintf("DynamicFibonacciMultiSizeBlowup%d", cfg.Blowup)
		if err := addMultiSizeDynFib(name, func(source, caseName string, sys *wiop.System, honest, alt assignFn) error {
			return addMultiSizeWith(source, caseName, sys, honest, alt, cfg.PCSOptions()...)
		}); err != nil {
			return nil, nil, err
		}
	}

	// Every committed column has size 1: the PCS keeps one FRI fold anyway.
	{
		sys, honest, invalid := buildAllSizeOneSystem()
		if err := add("AllSizeOne", "AllSizeOneColumns", sys, honest, invalid); err != nil {
			return nil, nil, err
		}
	}
	return cases, systems, nil
}

// buildPermutationSystem builds a size-4 single-column permutation A ~ B,
// reduced by grandproduct.Compile via the canonical compilers.CompileFull
// pipeline. Compile order: grandproduct's Z-final endpoint is a scalar
// (ColumnPosition-based) local opening, so localvanishing.Compile must run
// BEFORE global.Compile to lift it to a multi-valued vanishing that
// global.Compile can discharge; pcscompiler.Compile runs last so it sees
// every committed column and LagrangeEval opening the earlier passes
// registered — the same ordering compilers.CompileFull uses. This system has
// no message-bus entries, so the messagebus pass is a no-op and the default
// seeded shared-randomness mode is vacuous for it.
func buildPermutationSystem() (*wiop.System, *wiop.Column, *wiop.Column, error) {
	sys := wiop.NewSystemf("permutation")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	if err := compileFullPipeline(sys); err != nil {
		return nil, nil, nil, err
	}
	return sys, colA, colB, nil
}

// messageBusFixtureCase bundles a built fixtureCase with the CompiledSystem it
// was built against, so buildMessageBusFixtureCase's caller can append both to
// the parallel cases/systems slices buildCompiledFixtureCases maintains.
type messageBusFixtureCase struct {
	tc       fixtureCase
	compiled codegen.CompiledSystem
}

// buildMessageBusSharedRandomnessSystem builds a size-4 Send/Receive
// message-bus handle compiled with messagebus's default seeded shared
// randomness, mirroring codegen's newSharedRandomnessMessageBusHandle test
// helper but local to the fixture generator. The whole pipeline — messagebus,
// grandproduct, localvanishing, global, pcscompiler — runs through the single
// compilers.CompileFull call below.
func buildMessageBusSharedRandomnessSystem() (*wiop.System, *wiop.Column, *wiop.Column, error) {
	sys := wiop.NewSystemf("mb-shared-randomness")
	r0 := sys.NewRound()
	// Bus columns live on the coin round, not on r0. SharedRandomness requires
	// it: the coin round's commitment is hashed into this shard's contribution to
	// γ, so a column on any other round would not be bound by the γ that seeds
	// the α and β the bus is evaluated with. r0 carries γ itself, which is why
	// the coin round is r0's successor.
	coinRound := r0.EnsureNext()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), coinRound)
	colB := modB.NewColumn(sys.Context.Childf("B"), coinRound)

	sys.NewMessageBusSend(sys.Context.Childf("send"), "shard", "route", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(sys.Context.Childf("recv"), "shard", "route", wiop.NewTable(colB.View()))

	if err := compileFullPipeline(sys); err != nil {
		return nil, nil, nil, err
	}
	return sys, colA, colB, nil
}

// flippedExtTraceCell returns an extension-field trace cell with a value
// guaranteed to differ from cell's, for tampering a single committed cell in
// a proof's rounds. Used for the grandproduct permutation fixture's invalid
// proof: it flips the committed Z-endpoint cell that both the vanishing
// boundary constraint (Z's OOD claim == this transcript cell, at the module's
// last row) and grandproduct.verify (this cell feeds ∏Z == Result) read, so
// ONE flip breaks both sub-verifiers' checks.
func flippedExtTraceCell(cell runtimeTraceCell) runtimeTraceCell {
	if cell.extValue == nil {
		panic("flippedExtTraceCell: expected an extension-field trace cell")
	}
	one := elem(1)
	var flipped field.Ext
	field.AddByBase(&flipped, cell.extValue, &one)
	return extTraceCell(flipped)
}

// addMessageBusSharedRandomness registers the message-bus + SharedRandomness
// scenario with reg, which is responsible for turning (name, sys, honest) into
// a fixtureCase/CompiledSystem pair and appending it to the caller's slices.
func addMessageBusSharedRandomness(reg func(name string, sys *wiop.System, honest assignFn) error) error {
	sys, colA, colB, err := buildMessageBusSharedRandomnessSystem()
	if err != nil {
		return fmt.Errorf("build message-bus shared-randomness system: %w", err)
	}
	honest := func(rt *wiop.Runtime) {
		// γ lives on round 0 and must be written while the runtime is still there.
		messagebus.AssignSharedRandomnessSeed(rt, octuplet(11, 22, 33, 44, 55, 66, 77, 88))
		// The bus columns live on the coin round, so step onto it before assigning
		// them: AssignColumn requires the runtime's current round to be the
		// column's own. Advancing here is what absorbs γ into the transcript that
		// α and β are then drawn from.
		rt.AdvanceRound()
		rt.AssignColumn(colA, concreteBase(elems(1, 2, 3, 4)))
		rt.AssignColumn(colB, concreteBase(elems(1, 2, 3, 4)))
	}
	return reg("SharedRandomnessContribution", sys, honest)
}

// buildMessageBusFixtureCase runs the messagebus/SharedRandomness pipeline's
// own build steps (AssertAllVerifierActionsHandled, BuildCompiledSystem,
// buildProofFixture) rather than the add() closure's compileFullPipeline,
// since this system is already fully compiled by
// buildMessageBusSharedRandomnessSystem. The invalid fixture is the honest
// proof's fixture with exactly one of the 328 shared-randomness contribution
// public-input cells flipped — codegen.BuildSharedRandomnessSystem's
// ContributionRefs give the (round, index) transcript coordinates of those
// cells, and sys.LookupPublicInputByTag resolves the matching index into
// view.publicInputs (registration order == statement order, per
// wiop.System.PublicInputs/LookupPublicInputByTag).
func buildMessageBusFixtureCase(name string, sys *wiop.System, honest assignFn) (messageBusFixtureCase, error) {
	if err := codegen.AssertAllVerifierActionsHandled(sys); err != nil {
		return messageBusFixtureCase{}, fmt.Errorf("verifier actions MessageBus/%s: %w", name, err)
	}
	compiled, err := codegen.BuildCompiledSystem(sys)
	if err != nil {
		return messageBusFixtureCase{}, fmt.Errorf("build compiled system MessageBus/%s: %w", name, err)
	}
	if len(compiled.SharedRandomness.ContributionRefs) != messagebus.NumSharedRandomnessContribution {
		return messageBusFixtureCase{}, fmt.Errorf(
			"MessageBus/%s: expected %d shared-randomness contribution refs, got %d",
			name, messagebus.NumSharedRandomnessContribution, len(compiled.SharedRandomness.ContributionRefs),
		)
	}

	honestFixture, err := buildProofFixture(sys, honest, "MessageBus", name, "honest")
	if err != nil {
		return messageBusFixtureCase{}, err
	}

	_, statementIndex := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedContributionPI, 0)
	if statementIndex < 0 {
		return messageBusFixtureCase{}, fmt.Errorf("MessageBus/%s: missing contribution-0 public input", name)
	}
	if statementIndex >= len(honestFixture.view.publicInputs) {
		return messageBusFixtureCase{}, fmt.Errorf(
			"MessageBus/%s: contribution-0 statement index %d outside public_inputs (len %d)",
			name, statementIndex, len(honestFixture.view.publicInputs),
		)
	}

	invalidFixture := honestFixture
	invalidFixture.view.publicInputs = append([]runtimeTraceCell(nil), honestFixture.view.publicInputs...)
	invalidFixture.view.publicInputs[statementIndex] = flippedTraceCell(invalidFixture.view.publicInputs[statementIndex])

	tc := fixtureCase{
		name: name, pcs: compiled.Pcs, honest: honestFixture, invalid: &invalidFixture, sys: sys, assign: honest,
	}
	return messageBusFixtureCase{tc: tc, compiled: compiled}, nil
}

// flippedTraceCell returns a base-field trace cell with a value guaranteed to
// differ from cell's, for tampering a single public-input statement entry.
// The shared-randomness contribution cells are always base-field (never
// extension-field — see messagebus.contributionCell's IsExtension panic), so
// cell.baseValue is always set here.
func flippedTraceCell(cell runtimeTraceCell) runtimeTraceCell {
	if cell.baseValue == nil {
		panic("flippedTraceCell: expected a base-field trace cell")
	}
	flipped := elem(u(*cell.baseValue) + 1)
	return baseTraceCell(flipped)
}

// buildLookupMultiColumnBenchSystem is a verifier-ray-local stress fixture for
// profiling a wider lookup/log-derivative/vanishing pipeline. It stays local to
// verifier-ray because its purpose is benchmark shape, not reusable prover-ray
// scenario coverage.
func buildLookupMultiColumnBenchSystem() (*wiop.System, assignFn) {
	const (
		bigSize = 1 << 10
		numCols = 5
	)
	var (
		tCols        = make([]*wiop.Column, numCols)
		sCols        = make([]*wiop.Column, numCols)
		tTableView   = make([]*wiop.ColumnView, numCols)
		sTableView   = make([]*wiop.ColumnView, numCols)
		tAssignments = make([][]field.Element, numCols)
		sAssignments = make([][]field.Element, numCols)
	)

	sys := wiop.NewSystemf("lk-multi-col-bench")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), bigSize, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), bigSize, wiop.PaddingDirectionNone)
	for i := range tCols {
		tCols[i] = modT.NewColumn(sys.Context.Childf("T%d", i), r0)
		sCols[i] = modS.NewColumn(sys.Context.Childf("S%d", i), r0)
		tTableView[i] = tCols[i].View()
		sTableView[i] = sCols[i].View()
	}
	sys.NewInclusion(
		sys.Context.Childf("inc-big-table-bench"),
		[]wiop.Table{wiop.NewTable(sTableView...)},
		[]wiop.Table{wiop.NewTable(tTableView...)},
	)

	for i := range tCols {
		tAssignments[i] = make([]field.Element, bigSize)
		sAssignments[i] = make([]field.Element, bigSize)
		for j := range bigSize {
			tAssignments[i][j] = elem(uint64(i + j + 1))
			sAssignments[i][j] = elem(uint64(i + (j % (bigSize / 2)) + 1))
		}
	}

	honest := func(rt *wiop.Runtime) {
		for i := range tCols {
			rt.AssignColumn(tCols[i], concreteBase(tAssignments[i]))
			rt.AssignColumn(sCols[i], concreteBase(sAssignments[i]))
		}
	}
	return sys, honest
}

// buildLagrangeSelectorBoundarySystem builds a size-4 module with the single
// vanishing L_1·(col − 99) = 0. The Lagrange selector L_1 is 1 at row 1 and 0
// elsewhere on the domain, so the constraint pins col[1] = 99 while leaving the
// other rows free. It exercises the verifier's out-of-domain selector
// evaluation L_1(r) = ω·(r⁴−1)/(4·(r−ω)). The column is returned so the caller
// can assign witnesses.
func buildLagrangeSelectorBoundarySystem() (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("lagrange-sel")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	selector := wiop.NewLagrangeSelector(mod, 1)
	value := wiop.NewConstantField(elem(99))
	mod.NewVanishing(sys.Context.Childf("boundary"), wiop.Mul(selector, wiop.Sub(col.View(), value)))
	return sys, col
}

// buildDynamicLagrangeSelectorBoundarySystem is the dynamic-module twin of
// buildLagrangeSelectorBoundarySystem: same L_1·(col − 99) = 0 constraint, but
// on a dynamic module whose domain size is fixed only at runtime (here 4, from
// the assigned column length). Because the module is unsized at codegen time,
// the Zig verifier takes the dynamic selector path — deriving ω from
// ctx.dynamic_n via field.rootOfUnityBy and computing ω^position at runtime —
// rather than folding ω^position to a comptime constant. The static fixture
// never exercises that branch, so this one guards the path production dynamic
// modules actually hit. The column is returned so the caller can assign
// witnesses.
func buildDynamicLagrangeSelectorBoundarySystem() (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("lagrange-sel-dyn")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	selector := wiop.NewLagrangeSelector(mod, 1)
	value := wiop.NewConstantField(elem(99))
	mod.NewVanishing(sys.Context.Childf("boundary"), wiop.Mul(selector, wiop.Sub(col.View(), value)))
	return sys, col
}

// buildAllSizeOneSystem has only size-1 committed columns: a and b are free
// and c is constrained to be their sum. prover-ray raises the FRI top size
// to 2^MinFoldedSizeLog2 so the proof still folds once (#4322); the verifier
// must derive the same fold challenge count and query domain from the
// transcript. The invalid witness breaks the sum.
//
// This case does not guard the boundary alpha_DEEP: size-1 columns equal their
// claims, so honest DEEP quotients are identically zero and the honest proof
// verifies even with alpha_DEEP forced to zero, while the invalid witness is
// rejected by the quotient identity, not by the PCS.
func buildAllSizeOneSystem() (*wiop.System, assignFn, assignFn) {
	sys := wiop.NewSystemf("all-size-one")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	c := mod.NewColumn(sys.Context.Childf("c"), r0)
	mod.NewVanishing(sys.Context.Childf("sum"), wiop.Sub(c.View(), wiop.Add(a.View(), b.View())))
	assign := func(av, bv, cv uint64) assignFn {
		return func(rt *wiop.Runtime) {
			rt.AssignColumn(a, concreteBase(elems(av)))
			rt.AssignColumn(b, concreteBase(elems(bv)))
			rt.AssignColumn(c, concreteBase(elems(cv)))
		}
	}
	return sys, assign(3, 4, 7), assign(3, 4, 8)
}

// buildPublicInputSystem mirrors prover-ray's basic public-input flow: open one
// oracle column position into a cell, register that cell as a public input, and
// let the local-vanishing compiler turn the opening equality into a verifier
// check. The verifier fixture then proves two properties end-to-end:
//  1. the opened cell is omitted from the proof rounds and carried separately in
//     the flat public-input vector;
//  2. tampering only that public-input vector is rejected.
func buildPublicInputSystem() (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("public-input")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	cell := col.At(2).Open(sys.Context.Childf("public"))
	sys.RegisterPublicInputs("PublicInput", cell)
	return sys, col
}

// buildDynamicPublicInputSystem is the dynamic-module twin of
// buildPublicInputSystem: same opened-cell-as-public-input flow, but the
// containing module size is inferred from the runtime witness assignment and
// round-trips through the proof's module_sizes.
func buildDynamicPublicInputSystem() (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("public-input-dyn")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	cell := col.At(0).Open(sys.Context.Childf("public"))
	sys.RegisterPublicInputs("PublicInput", cell)
	return sys, col
}

// runProver creates a runtime for sys, applies assign, advances all rounds
// running every prover action, and returns the completed runtime.
func runProver(sys *wiop.System, assign assignFn) *wiop.Runtime {
	rt := wiop.NewRuntime(sys)
	assign(rt)
	for _, action := range rt.CurrentRound().ProverActions {
		action.Run(rt)
	}
	for rt.CurrentRound().ID < len(rt.System.Rounds)-1 {
		rt.AdvanceRound()
		for _, action := range rt.CurrentRound().ProverActions {
			action.Run(rt)
		}
	}
	return rt
}

// extractVanishingProofView reads the public inputs, round trace data and
// dynamic module sizes from an already-completed runtime.
func extractVanishingProofView(sys *wiop.System, rt *wiop.Runtime) vanishingProofView {
	publicInputs := extractPublicInputs(sys, rt)
	publicInputByID := publicInputIndex(sys)

	rounds := make([]runtimeTraceRound, 0, len(sys.Rounds)-1)
	for i := 0; i < len(sys.Rounds)-1; i++ {
		rounds = append(rounds, runtimeTraceRoundFromRuntime(rt, sys.Rounds[i], publicInputByID))
	}

	return vanishingProofView{
		rounds:       rounds,
		publicInputs: publicInputs,
		moduleSizes:  codegen.DynamicModuleSizes(sys, rt),
	}
}

// addMultiSizeDynFib constructs the dyn-fib protocol (a single committed dynamic
// column with the Fibonacci recurrence A[i] − A[i−1] − A[i−2] = 0) and registers
// it via `reg` (addMultiSize) with two honest witnesses of DIFFERENT lengths
// (8 and 16). Rows 0 and 1 are auto-cancelled by the back-shifts, so any
// Fibonacci prefix is valid. This exercises verifying two dynamic sizes against
// one baked PcsSystem, registered under name.
func addMultiSizeDynFib(name string, reg func(source, name string, sys *wiop.System, honest, alt assignFn) error) error {
	sys := wiop.NewSystemf("dyn-fib-multisize")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(
		sys.Context.Childf("fib"),
		wiop.Sub(
			wiop.Sub(col.View(), col.View().Shift(-1)),
			col.View().Shift(-2),
		),
	)

	honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, fibVector(8)) }
	alt := func(rt *wiop.Runtime) { rt.AssignColumn(col, fibVector(16)) }
	return reg("MultiSize", name, sys, honest, alt)
}

// fibVector builds a length-n Fibonacci column (rows 0 and 1 = 1), used as an
// honest witness for the committed-dynamic Fibonacci recurrence.
func fibVector(n int) *wiop.ConcreteVector {
	vals := make([]uint64, n)
	vals[0], vals[1] = 1, 1
	for i := 2; i < n; i++ {
		vals[i] = vals[i-1] + vals[i-2]
	}
	elems := make([]field.Element, n)
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// addMultiSizeTwoDynModules builds a protocol with TWO independent committed
// dynamic modules (each a Fibonacci column), proven at two size configurations
// against ONE baked PcsSystem. The alt witness resizes BOTH modules to different
// lengths than honest, so the verifier must (a) absorb each module's size into
// the transcript in sys.Modules order, (b) index module_sizes[i] per dynamic
// module correctly, and (c) reconstruct two independently-sized layouts that
// coexist in the shared column schedule. Single-module scenarios cannot catch a
// swapped module index or a cross-module absorption-order bug — this one does.
func addMultiSizeTwoDynModules(reg func(source, name string, sys *wiop.System, honest, alt assignFn) error) error {
	sys := wiop.NewSystemf("dyn-fib-two-modules")
	r0 := sys.NewRound()

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("colA"), r0)
	modA.NewVanishing(
		sys.Context.Childf("fibA"),
		wiop.Sub(wiop.Sub(colA.View(), colA.View().Shift(-1)), colA.View().Shift(-2)),
	)

	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colB := modB.NewColumn(sys.Context.Childf("colB"), r0)
	modB.NewVanishing(
		sys.Context.Childf("fibB"),
		wiop.Sub(wiop.Sub(colB.View(), colB.View().Shift(-1)), colB.View().Shift(-2)),
	)

	// honest: A at 8, B at 16. alt: A at 16, B at 8 — both modules change size AND
	// their relative sizes swap, so an off-by-one module index would derive wrong
	// coins and reject an honest proof.
	honest := func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, fibVector(8))
		rt.AssignColumn(colB, fibVector(16))
	}
	alt := func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, fibVector(16))
		rt.AssignColumn(colB, fibVector(8))
	}
	return reg("MultiSize", "DynamicFibonacciTwoModules", sys, honest, alt)
}

func publicInputIndex(sys *wiop.System) map[wiop.ObjectID]int {
	out := make(map[wiop.ObjectID]int, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		out[cell.Context.ID] = i
	}
	return out
}

func extractPublicInputs(sys *wiop.System, rt *wiop.Runtime) []runtimeTraceCell {
	out := make([]runtimeTraceCell, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		value := rt.GetCellValue(cell)
		if value.IsBase() {
			out[i] = baseTraceCell(value.AsBase())
		} else {
			out[i] = extTraceCell(value.AsExt())
		}
	}
	return out
}

func runtimeTraceRoundFromRuntime(rt *wiop.Runtime, round *wiop.Round, publicInputByID map[wiop.ObjectID]int) runtimeTraceRound {
	var trace runtimeTraceRound
	// A committed (interactive PCS) round carries its Merkle root as the sole
	// verifier-visible message for that round, absorbed by AdvanceRound in place
	// of the round's columns (which never travel raw). Columns themselves are
	// never part of the trace — see prover-ray's wiop_prove_verify.go Proof
	// comment: binding them is the commitment scheme's job, not the transcript's.
	if round.HasCommitment {
		root, ok := rt.Commitments[round.ID]
		if !ok {
			panic(fmt.Sprintf("round %d HasCommitment but no root in runtime", round.ID))
		}
		trace.commitment = &root
	}
	for _, cell := range round.Cells {
		if !rt.HasCellValue(cell) {
			continue
		}
		if _, isPI := publicInputByID[cell.Context.ID]; isPI {
			continue
		}
		value := rt.GetCellValue(cell)
		if value.IsBase() {
			trace.cells = append(trace.cells, baseTraceCell(value.AsBase()))
		} else {
			trace.cells = append(trace.cells, extTraceCell(value.AsExt()))
		}
	}
	return trace
}

func writeVerifyFixtures(path string, cases []fixtureCase, systems []codegen.CompiledSystem, imageCase int) error {
	// verify.zig drives the mandatory-PCS verifier.verify, the only consumer of
	// these fixtures: a case without a PCS System could not be verified, so it
	// is an error rather than silently dropped.
	for i := range cases {
		if cases[i].pcs == nil {
			return fmt.Errorf("fixture case %q has no PCS System", cases[i].name)
		}
	}

	var out bytes.Buffer
	writeVerifyHeader(&out, len(cases))
	opts := codegen.CompiledSystemZigOptions{
		EmitHeader:         false,
		ProtocolImport:     "protocol",
		FieldImport:        "field",
		VanishingImport:    "vanishing",
		LogDerivImport:     "logderivativesum",
		GrandProductImport: "grandproduct",
		RowLimitImport:     "rowlimit",
	}

	for i := range cases {
		if err := codegen.WriteCompiledSystemZig(&out, i, systems[i], opts); err != nil {
			return err
		}
		writeVerifyCase(&out, i, cases[i])
	}
	writeVerifyMetadata(&out, cases, systems)
	writeVerifyCaseSwitch(&out, cases)
	writeVerifyInputSwitch(&out, cases)
	writeVerifyFailingInputSwitch(&out, cases)
	fmt.Fprintf(&out, "\n/// The case whose honest proof testdata/fixture_proof_image.bin encodes (%s).\n", cases[imageCase].name)
	fmt.Fprintf(&out, "pub const image_case_index: usize = %d;\n", imageCase)

	data, err := codegen.FormatZig(out.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeVerifyHeader(out *bytes.Buffer, count int) {
	fmt.Fprintln(out, "// Code generated by verifier-ray/cmd/generate-testdata; DO NOT EDIT.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "const verifier_ray = @import(\"verifier_ray\");")
	fmt.Fprintln(out, "const field = verifier_ray.field.koalabear;")
	fmt.Fprintln(out, "const ext = verifier_ray.field.koalabear_ext;")
	fmt.Fprintln(out, "const protocol = verifier_ray.protocol;")
	fmt.Fprintln(out, "const commitment = verifier_ray.crypto.commitment;")
	fmt.Fprintln(out, "const merkle = verifier_ray.crypto.merkle;")
	fmt.Fprintln(out, "const vanishing = verifier_ray.query.vanishing;")
	fmt.Fprintln(out, "const logderivativesum = verifier_ray.query.logderivativesum;")
	fmt.Fprintln(out, "const grandproduct = verifier_ray.query.grandproduct;")
	fmt.Fprintln(out, "const rowlimit = verifier_ray.query.rowlimit;")
	fmt.Fprintln(out, "const shared_randomness = verifier_ray.query.shared_randomness;")
	fmt.Fprintln(out, "const pcs = verifier_ray.query.pcs;")
	fmt.Fprintln(out, "const fri = verifier_ray.query.fri;")
	fmt.Fprintln(out, "const verifier = verifier_ray.verifier;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "pub const VerifyCase = struct {")
	fmt.Fprintln(out, "    name: []const u8,")
	fmt.Fprintln(out, "    spec: protocol.Spec,")
	fmt.Fprintln(out, "    systems: verifier.Systems,")
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "pub const VerifyCaseMetadata = struct {")
	fmt.Fprintln(out, "    name: []const u8,")
	fmt.Fprintln(out, "    module_count: usize,")
	fmt.Fprintln(out, "    dynamic_module_count: usize,")
	fmt.Fprintln(out, "    round_count: usize,")
	fmt.Fprintln(out, "    expression_count: usize,")
	fmt.Fprintln(out, "    bucket_count: usize,")
	fmt.Fprintln(out, "    vanishing_count: usize,")
	fmt.Fprintln(out, "    total_witness_claims: usize,")
	fmt.Fprintln(out, "    total_quotient_claims: usize,")
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "pub const case_count: usize = %d;\n", count)
	fmt.Fprintln(out)
}

func writeVerifyCase(out *bytes.Buffer, idx int, tc fixtureCase) {
	// The compile-time PCS system for this case (mandatory). Its
	// batch_roots/maps/zeta are what verifier.Systems.pcs points at; each proof
	// carries only its own runtime opening.
	pcsName := writePcsSystemZig(out, fmt.Sprintf("verify_case_%d", idx), tc.pcs)

	writeVerifyProof(out, fmt.Sprintf("verify_case_%d", idx), tc.honest)
	if tc.invalid != nil {
		writeVerifyProof(out, fmt.Sprintf("verify_case_%d_failing", idx), *tc.invalid)
	}
	// Self-consistent sub-verifier-failing proof (verify.zig only). Distinct
	// from _failing (the vanishing-breaking, PCS-rejected .invalid): this one
	// is PCS- and vanishing-valid and fails specifically in a later
	// sub-verifier, so it exercises that sub-verifier's wiring end-to-end.
	if tc.invalidVerify != nil {
		writeVerifyProof(out, fmt.Sprintf("verify_case_%d_failing_verify", idx), *tc.invalidVerify)
	}
	// The alt (second-size) honest proof, verified against the SAME System's .pcs.
	if tc.alt != nil {
		writeVerifyProof(out, fmt.Sprintf("verify_case_%d_alt", idx), *tc.alt)
	}
	fmt.Fprintf(
		out,
		"const verify_case_%d_systems = verifier.Systems{ .public_input = system_%d_public_input, .vanishing = system_%d, .logderivativesum = system_%d_logderiv, .grandproduct = system_%d_grandproduct, .rowlimit = system_%d_rowlimit, .shared_randomness = system_%d_shared_randomness, .pcs = %s };\n",
		idx, idx, idx, idx, idx, idx, idx, pcsName,
	)
	fmt.Fprintln(out)
}

func writeVerifyProof(out *bytes.Buffer, prefix string, fixture proofFixture) {
	proof := fixture.view
	fmt.Fprintf(out, "const %s_module_sizes = [_]usize%s;\n", prefix, intArrayLiteral(proof.moduleSizes))
	fmt.Fprintln(out)

	for roundIdx, round := range proof.rounds {
		writeVerifyRoundData(out, prefix, roundIdx, round)
	}

	fmt.Fprintf(out, "const %s_rounds = [_]protocol.RoundMessage{\n", prefix)
	for roundIdx, round := range proof.rounds {
		if round.commitment != nil {
			fmt.Fprintf(out, "    .{ .commitment = %s, .cells = &%s_round_%d_cells },\n", commitmentValueLiteral(*round.commitment), prefix, roundIdx)
		} else {
			fmt.Fprintf(out, "    .{ .cells = &%s_round_%d_cells },\n", prefix, roundIdx)
		}
	}
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)

	// The PCS opening: just the FRI opening proof. No entry_claims — the
	// verifier reconstructs those from rounds[*].cells (emitted above) via the
	// compiled pcs.System's claim_cells table — and no witness/quotient_claims,
	// which are derived by the verifier from entry_claims via the claim maps.
	fmt.Fprintf(out, "const %s_pcs_opening = %s;\n", prefix, pcsOpeningZigLiteral(fixture.opening))
	fmt.Fprintln(out)

	fmt.Fprintf(out, "const %s_proof = verifier.Proof{\n", prefix)
	fmt.Fprintf(out, "    .rounds = &%s_rounds,\n", prefix)
	fmt.Fprintf(out, "    .module_sizes = &%s_module_sizes,\n", prefix)
	fmt.Fprintf(out, "    .pcs_opening = %s_pcs_opening,\n", prefix)
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "const %s_public_inputs = [_]protocol.Scalar{\n", prefix)
	for _, cell := range proof.publicInputs {
		switch {
		case cell.baseValue != nil:
			fmt.Fprintf(out, "    .{ .base = %s },\n", fieldValueLiteral(*cell.baseValue))
		case cell.extValue != nil:
			fmt.Fprintf(out, "    .{ .ext = %s },\n", extValueLiteral(*cell.extValue))
		default:
			panic("runtime trace cell has no data variant")
		}
	}
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "const %s_input = verifier.VerifyInput{\n", prefix)
	fmt.Fprintf(out, "    .proof = %s_proof,\n", prefix)
	fmt.Fprintf(out, "    .public_inputs = &%s_public_inputs,\n", prefix)
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)
}

func writeVerifyRoundData(out *bytes.Buffer, prefix string, roundIdx int, round runtimeTraceRound) {
	fmt.Fprintf(out, "const %s_round_%d_cells = [_]protocol.Scalar{\n", prefix, roundIdx)
	for _, cell := range round.cells {
		switch {
		case cell.baseValue != nil:
			fmt.Fprintf(out, "    .{ .base = %s },\n", fieldValueLiteral(*cell.baseValue))
		case cell.extValue != nil:
			fmt.Fprintf(out, "    .{ .ext = %s },\n", extValueLiteral(*cell.extValue))
		default:
			panic("runtime trace cell has no data variant")
		}
	}
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)
}

func writeVerifyMetadata(out *bytes.Buffer, cases []fixtureCase, systems []codegen.CompiledSystem) {
	fmt.Fprintln(out, "pub const metadata = [_]VerifyCaseMetadata{")
	for i, tc := range cases {
		vanishingSystem := systems[i].Vanishing
		expressionCount, bucketCount, vanishingCount := vanishingBenchCounts(vanishingSystem)
		fmt.Fprintf(out, "    .{ .name = \"%s\", .module_count = %d, .dynamic_module_count = %d, .round_count = %d, .expression_count = %d, .bucket_count = %d, .vanishing_count = %d, .total_witness_claims = %d, .total_quotient_claims = %d },\n",
			codegen.ZigString(tc.name),
			len(vanishingSystem.Modules),
			vanishingSystem.DynamicModuleCount,
			len(systems[i].Routing.RoundCoinCounts),
			expressionCount,
			bucketCount,
			vanishingCount,
			vanishingSystem.TotalWitnessClaims,
			vanishingSystem.TotalQuotientClaims,
		)
	}
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)
}

func writeVerifyCaseSwitch(out *bytes.Buffer, cases []fixtureCase) {
	fmt.Fprintln(out, "pub fn get(comptime index: usize) VerifyCase {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		fmt.Fprintf(
			out,
			"        %d => .{ .name = \"%s\", .spec = system_%d_spec, .systems = verify_case_%d_systems },\n",
			i,
			codegen.ZigString(tc.name),
			i,
			i,
		)
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
}

// writeVerifyInputSwitch emits the getInput accessor, which returns the full
// verifier input (proof + separate public-input vector) for a fixture case. The
// input is kept separate from VerifyCase (see writeVerifyHeader) so callers that
// only need the spec/systems don't pull it in, and so it can be supplied
// independently at runtime.
func writeVerifyInputSwitch(out *bytes.Buffer, cases []fixtureCase) {
	fmt.Fprintln(out, "pub fn getInput(comptime index: usize) verifier.VerifyInput {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i := range cases {
		fmt.Fprintf(out, "        %d => verify_case_%d_input,\n", i, i)
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
}

// writeVerifyFailingInputSwitch emits the getInputFailing accessor, returning
// the failing verifier input for a fixture case. Not every case defines one, so
// cases without one produce a comptime error when requested.
func writeVerifyFailingInputSwitch(out *bytes.Buffer, cases []fixtureCase) {
	fmt.Fprintln(out, "pub fn getInputFailing(comptime index: usize) verifier.VerifyInput {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		if tc.invalid != nil {
			fmt.Fprintf(out, "        %d => verify_case_%d_failing_input,\n", i, i)
		} else {
			fmt.Fprintf(out, "        %d => @compileError(\"verifier fixture case %d (%s) has no failing input\"),\n", i, i, codegen.ZigString(tc.name))
		}
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// hasFailing lets a comptime sweep skip getInputFailing for cases that have
	// no failing input (calling it there is a @compileError).
	fmt.Fprintln(out, "pub fn hasFailing(comptime index: usize) bool {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		fmt.Fprintf(out, "        %d => %t,\n", i, tc.invalid != nil)
	}
	fmt.Fprintln(out, "        else => false,")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// getInputFailingVerify returns a case's SELF-CONSISTENT failing input: a
	// proof that is PCS- and vanishing-valid and is rejected specifically by a
	// later sub-verifier (e.g. grandproduct.verify). Unlike getInputFailing
	// (which shares the vanishing-breaking .invalid fixture and is rejected at
	// the PCS stage), this input reaches the targeted sub-verifier, so the
	// caller can assert the EXACT error that sub-verifier returns.
	fmt.Fprintln(out, "pub fn getInputFailingVerify(comptime index: usize) verifier.VerifyInput {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		if tc.invalidVerify != nil {
			fmt.Fprintf(out, "        %d => verify_case_%d_failing_verify_input,\n", i, i)
		} else {
			fmt.Fprintf(out, "        %d => @compileError(\"verifier fixture case %d (%s) has no self-consistent failing input\"),\n", i, i, codegen.ZigString(tc.name))
		}
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// hasFailingVerify gates getInputFailingVerify for cases without one.
	fmt.Fprintln(out, "pub fn hasFailingVerify(comptime index: usize) bool {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		fmt.Fprintf(out, "        %d => %t,\n", i, tc.invalidVerify != nil)
	}
	fmt.Fprintln(out, "        else => false,")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// failingVerifyError is the exact error name verifier.verify must return
	// for the self-consistent failing input, pinning which sub-verifier fires.
	fmt.Fprintln(out, "pub fn failingVerifyError(comptime index: usize) []const u8 {")
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		if tc.invalidVerify != nil {
			fmt.Fprintf(out, "        %d => \"%s\",\n", i, codegen.ZigString(tc.invalidVerifyError))
		} else {
			fmt.Fprintf(out, "        %d => @compileError(\"verifier fixture case %d (%s) has no self-consistent failing input\"),\n", i, i, codegen.ZigString(tc.name))
		}
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// The optional second-proof getter, gated by hasAlt: getInputAlt is a
	// proof at a different dynamic-module SIZE (multi-size cases only; hasAlt
	// is the multi-size predicate), checked against the SAME case systems
	// (verify_case_i_systems.pcs) as the primary proof — one baked System
	// verifying a second, differently-shaped proof.
	writeAltGetter(out, cases, "Alt", "alt", "alt input", func(tc fixtureCase) *proofFixture { return tc.alt })
}

// writeAltGetter emits a `getInput<suffix>`/`has<suffix>` pair gated by
// whether `get(tc)` is non-nil for each case. `fieldPrefix` is the zig
// identifier prefix baked into the `_input` const names (written by
// writeVerifyProof) and `missingLabel` the compile-error label.
func writeAltGetter(out *bytes.Buffer, cases []fixtureCase, suffix, fieldPrefix, missingLabel string, get func(fixtureCase) *proofFixture) {
	fmt.Fprintf(out, "pub fn getInput%s(comptime index: usize) verifier.VerifyInput {\n", suffix)
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		if get(tc) != nil {
			fmt.Fprintf(out, "        %d => verify_case_%d_%s_input,\n", i, i, fieldPrefix)
		} else {
			fmt.Fprintf(out, "        %d => @compileError(\"verifier fixture case %d (%s) has no %s\"),\n", i, i, codegen.ZigString(tc.name), missingLabel)
		}
	}
	fmt.Fprintln(out, "        else => @compileError(\"unknown verifier fixture case index\"),")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "pub fn has%s(comptime index: usize) bool {\n", suffix)
	fmt.Fprintln(out, "    return switch (index) {")
	for i, tc := range cases {
		fmt.Fprintf(out, "        %d => %t,\n", i, get(tc) != nil)
	}
	fmt.Fprintln(out, "        else => false,")
	fmt.Fprintln(out, "    };")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)
}

func vanishingBenchCounts(system codegen.VanishingSystem) (expressionCount, bucketCount, vanishingCount int) {
	for _, module := range system.Modules {
		expressionCount += len(module.Expressions)
		bucketCount += len(module.Buckets)
		for _, bucket := range module.Buckets {
			vanishingCount += len(bucket.Vanishings)
		}
	}
	return expressionCount, bucketCount, vanishingCount
}

func fieldValueLiteral(value field.Element) string {
	return fmt.Sprintf(".{ .value = %d }", u(value))
}

func extValueLiteral(value field.Ext) string {
	a0, a1, b0, b1, c0, c1 := field.ExtToUint64s(&value)
	return fmt.Sprintf(
		"ext.Ext{ .B0 = .{ .a0 = .{ .value = %d }, .a1 = .{ .value = %d } }, .B1 = .{ .a0 = .{ .value = %d }, .a1 = .{ .value = %d } }, .B2 = .{ .a0 = .{ .value = %d }, .a1 = .{ .value = %d } } }",
		a0, a1, b0, b1, c0, c1,
	)
}

func commitmentValueLiteral(value field.Octuplet) string {
	parts := make([]string, len(value))
	for i, elem := range value {
		parts[i] = fieldValueLiteral(elem)
	}
	return "commitment.Commitment{ " + strings.Join(parts, ", ") + " }"
}

func intArrayLiteral(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func elem(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

func elems(values ...uint64) []field.Element {
	out := make([]field.Element, len(values))
	for i, value := range values {
		out[i] = elem(value)
	}
	return out
}

func concreteBase(values []field.Element) *wiop.ConcreteVector {
	return &wiop.ConcreteVector{Plain: field.VecFromBase(values)}
}

func octuplet(values ...uint64) field.Octuplet {
	if len(values) != 8 {
		panic("octuplet requires 8 values")
	}
	var out field.Octuplet
	for i, value := range values {
		out[i] = elem(value)
	}
	return out
}

func u(e field.Element) uint64 {
	return e.Uint64()
}

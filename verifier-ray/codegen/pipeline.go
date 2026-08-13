package codegen

import (
	"fmt"
	"io"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// CompileToZigOptions configures CompileToZig's Zig output. It bundles
// CompiledSystemZigOptions (spec/public-input/vanishing/logderiv/grandproduct/
// rowlimit) with the PCS-specific options WriteCompiledSystemZig does not
// cover on its own, plus the verifier module's own import path (for the
// consolidated verifier.Systems const CompileToZig emits after them).
type CompileToZigOptions struct {
	CompiledSystemZigOptions
	Pcs PcsZigOptions
	// VerifierImport is the Zig import expression exposing `verifier.Systems`
	// (e.g. `@import("verifier_ray").verifier`). Defaults to
	// `@import("verifier_ray").verifier` when empty.
	VerifierImport string
}

// CompileToZig runs the full CompiledSystem extraction pipeline
// (BuildCoinRouting -> BuildPublicInputSystem -> BuildVanishingSystem ->
// BuildLogDerivSystem -> BuildGrandProductSystem -> BuildRowLimitSystem ->
// BuildPcsSystem) against sys, fails closed via AssertAllVerifierActionsHandled,
// and writes the resulting CompiledSystem (plus its PCS system, which
// WriteCompiledSystemZig does not emit on its own) to w. Finally, it emits one
// consolidated `pub const system_<index>_systems = verifier.Systems{...}`
// value combining every sub-const just written — those sub-consts
// (system_<index>, system_<index>_logderiv, etc.) are package-private to the
// generated file by convention (mirroring testdata/generate/main.go's own
// verify_case_N_systems consolidation), so a caller (main_recursive.zig) has
// exactly one pub name to import spec/systems from.
//
// sys must already have the full prover compiler pipeline run on it
// (nonnative, rangecheck, lookuptologderivsum, messagebus, grandproduct,
// logderivativesum, localvanishing, global, pcs — mirroring prover-ray's
// zkcdriver/zkcpipeline.RunCompilePipeline).
//
// rt must be a Runtime that has resolved every dynamic module's size and
// every LagrangeEval claim — i.e. driven through the full round loop at
// least once (see zkcpipeline.RunProve or the manual round-loop pattern used
// in this package's own tests). It does NOT need to be an honest witness FOR
// THE SPECIFIC PROOF this compiled system will later verify: CompileToZig
// only extracts structural/positional metadata from it. BuildPcsSystem's
// EntryClaims are read internally but never written to w — a real proof's
// entry_claims travel in that proof's own wire-format encoding
// (see EncodeProof/WriteProof), not baked into the compiled system.
func CompileToZig(sys *wiop.System, rt *wiop.Runtime, index int, w io.Writer, opts CompileToZigOptions) error {
	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		return fmt.Errorf("codegen: CompileToZig: %w", err)
	}

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildCoinRouting: %w", err)
	}
	publicInput, err := BuildPublicInputSystem(sys)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildPublicInputSystem: %w", err)
	}
	vanishing, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildVanishingSystem: %w", err)
	}
	logDeriv, err := BuildLogDerivSystem(sys)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildLogDerivSystem: %w", err)
	}
	grandProduct, err := BuildGrandProductSystem(sys)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildGrandProductSystem: %w", err)
	}
	rowLimit, err := BuildRowLimitSystem(sys)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildRowLimitSystem: %w", err)
	}
	pcsSys, err := BuildPcsSystem(sys, rt, routing)
	if err != nil {
		return fmt.Errorf("codegen: CompileToZig: BuildPcsSystem: %w", err)
	}

	system := CompiledSystem{
		Routing:      routing,
		PublicInput:  publicInput,
		Vanishing:    vanishing,
		LogDeriv:     logDeriv,
		GrandProduct: grandProduct,
		RowLimit:     rowLimit,
		Pcs:          &pcsSys,
	}

	if err := WriteCompiledSystemZig(w, index, system, opts.CompiledSystemZigOptions); err != nil {
		return fmt.Errorf("codegen: CompileToZig: WriteCompiledSystemZig: %w", err)
	}
	if err := WritePcsSystemZigWithOptions(w, index, pcsSys, opts.Pcs); err != nil {
		return fmt.Errorf("codegen: CompileToZig: WritePcsSystemZig: %w", err)
	}

	pcsConstName := opts.Pcs.ConstName
	if pcsConstName == "" {
		pcsConstName = fmt.Sprintf("pcs_system_%d", index)
	}
	verifierImport := opts.VerifierImport
	if verifierImport == "" {
		verifierImport = `@import("verifier_ray").verifier`
	}
	if _, err := fmt.Fprintf(w,
		"\nconst verifier = %s;\npub const system_%d_systems = verifier.Systems{ .public_input = system_%d_public_input, .vanishing = system_%d, .logderivativesum = system_%d_logderiv, .grandproduct = system_%d_grandproduct, .rowlimit = system_%d_rowlimit, .pcs = %s };\n",
		verifierImport, index, index, index, index, index, index, pcsConstName,
	); err != nil {
		return fmt.Errorf("codegen: CompileToZig: writing consolidated Systems const: %w", err)
	}
	return nil
}

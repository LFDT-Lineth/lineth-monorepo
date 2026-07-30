package codegen

import (
	"fmt"
	"io"
)

// CompiledSystem bundles all sub-verifier metadata derived from a single
// wiop.System after the full compiler pipeline has run. Each field covers one
// sub-verifier; fields are zero-valued (empty) when the system has no queries
// of that kind.
type CompiledSystem struct {
	Routing   CoinRouting
	Vanishing VanishingSystem
	LogDeriv  LogDerivSystem
	// PCS is the FRI/PCS opening System. HasPCS gates it: a protocol with no
	// committed columns (no LagrangeEvals) has no PCS System, and the emitted
	// verifier.Systems then omits the (mandatory) .pcs field — which is a Zig
	// compile error by design, surfacing "this protocol commits nothing to open"
	// at generation review time rather than as a confusing missing-field error.
	PCS    PcsSystem
	HasPCS bool
}

// CompiledSystemZigOptions configures WriteCompiledSystemZig.
type CompiledSystemZigOptions struct {
	// EmitHeader, when true, prepends all necessary import declarations
	// (protocol, field, vanishing, logderivativesum). Set to false when writing
	// multiple systems under a shared file header.
	EmitHeader      bool
	ProtocolImport  string
	FieldImport     string
	VanishingImport string
	LogDerivImport  string
	// PCS import prefixes. Empty → verifier-ray-relative defaults.
	PcsImport    string
	LayoutImport string
	VerifyImport string
	// VerifierImport is the Zig `verifier.zig` module the combined
	// `verifier.Systems` literal is built from. Empty → the default relative path.
	VerifierImport string
	// EmitSystems, when true, appends the combined `system_<index>_systems`
	// verifier.Systems literal after every sub-verifier. Requires HasPCS (the
	// .pcs field is mandatory). A file emitting several systems under one header
	// sets this per-system; a single-system file leaves it true.
	EmitSystems bool
}

// WriteCompiledSystemZig writes the spec, vanishing system, and logderiv
// system for a single CompiledSystem at the given index. It is a convenience
// wrapper around WriteSpecZigWithOptions, WriteVanishingSystemZigWithOptions,
// and WriteLogDerivSystemZigWithOptions that handles the EmitHeader/EmitImport
// flags automatically from a single opts.EmitHeader flag.
func WriteCompiledSystemZig(w io.Writer, index int, system CompiledSystem, opts CompiledSystemZigOptions) error {
	if err := WriteSpecZigWithOptions(w, system.Routing, SpecZigOptions{
		ProtocolImport: opts.ProtocolImport,
		ConstName:      fmt.Sprintf("system_%d_spec", index),
		EmitHeader:     opts.EmitHeader,
	}); err != nil {
		return err
	}
	if err := WriteVanishingSystemZigWithOptions(w, index, system.Vanishing, VanishingZigOptions{
		FieldImport:     opts.FieldImport,
		VanishingImport: opts.VanishingImport,
		EmitHeader:      opts.EmitHeader,
		EmitSystemsList: false,
	}); err != nil {
		return err
	}
	if err := WriteLogDerivSystemZigWithOptions(w, index, system.LogDeriv, LogDerivZigOptions{
		EmitImport:     opts.EmitHeader,
		LogDerivImport: opts.LogDerivImport,
	}); err != nil {
		return err
	}

	// PCS System (params/shapes/shifts/claim-maps/pcs_system), namespaced per
	// system index so several systems can share one file.
	if system.HasPCS {
		if err := WritePcsSystemZig(w, system.PCS, PcsZigOptions{
			PcsImport:    opts.PcsImport,
			LayoutImport: opts.LayoutImport,
			VerifyImport: opts.VerifyImport,
			ConstPrefix:  fmt.Sprintf("system_%d_", index),
		}); err != nil {
			return err
		}
	}

	// Combined verifier.Systems literal wiring every sub-verifier together. This
	// is the whole point of the exercise: the .pcs field authenticates the
	// entry_claims that verifier.verify re-slices into the vanishing witness /
	// quotient claims, so the two sub-verifiers provably consume the same values.
	if opts.EmitSystems {
		if !system.HasPCS {
			return fmt.Errorf(
				"codegen: system %d has no PCS System but EmitSystems is set; "+
					"verifier.Systems.pcs is mandatory — a protocol that commits nothing to open cannot be verified",
				index)
		}
		verifierImport := opts.VerifierImport
		if verifierImport == "" {
			verifierImport = `@import("../verifier.zig")`
		}
		fmt.Fprintf(w, "pub const system_%d_systems = %s.Systems{ .vanishing = system_%d", index, verifierImport, index)
		if len(system.LogDeriv.Queries) > 0 {
			fmt.Fprintf(w, ", .logderivativesum = system_%d_logderiv", index)
		}
		fmt.Fprintf(w, ", .pcs = system_%d_pcs_system };\n", index)
	}
	return nil
}

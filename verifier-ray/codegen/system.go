package codegen

import (
	"fmt"
	"io"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// CompiledSystem bundles all sub-verifier metadata derived from a single
// wiop.System after the full compiler pipeline has run. Each field covers one
// sub-verifier; fields are zero-valued (empty) when the system has no queries
// of that kind.
type CompiledSystem struct {
	Routing      CoinRouting
	PublicInput  PublicInputSystem
	Vanishing    VanishingSystem
	LogDeriv     LogDerivSystem
	GrandProduct GrandProductSystem
	RowLimit     RowLimitSystem
	// Pcs is the extracted PCS descriptor. Mandatory in the full verifier.verify
	// path (every protocol commits columns); nil only for callers that emit the
	// vanishing/logderiv systems standalone.
	Pcs *PcsSystem
}

// BuildCompiledSystem builds every sub-verifier system a compiled protocol
// needs — coin routing, public-input layout, vanishing, log-derivative,
// grand-product, row-limit, and PCS — from sys alone. PCS is mandatory in the
// full verifier.verify path, so this errors if sys has no committed batches;
// callers that legitimately emit the vanishing/logderiv systems standalone
// should call the individual Build*System functions instead.
func BuildCompiledSystem(sys *wiop.System) (CompiledSystem, error) {
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	publicInput, err := BuildPublicInputSystem(sys)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	vanishing, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	logDeriv, err := BuildLogDerivSystem(sys)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	grandProduct, err := BuildGrandProductSystem(sys)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	rowLimit, err := BuildRowLimitSystem(sys)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	pcs, err := BuildPcsSystem(sys, routing)
	if err != nil {
		return CompiledSystem{}, fmt.Errorf("codegen: BuildCompiledSystem: %w", err)
	}
	return CompiledSystem{
		Routing:      routing,
		PublicInput:  publicInput,
		Vanishing:    vanishing,
		LogDeriv:     logDeriv,
		GrandProduct: grandProduct,
		RowLimit:     rowLimit,
		Pcs:          &pcs,
	}, nil
}

// CompiledSystemZigOptions configures WriteCompiledSystemZig.
type CompiledSystemZigOptions struct {
	// EmitHeader, when true, prepends all necessary import declarations
	// (protocol, field, vanishing, logderivativesum, rowlimit). Set to false
	// when writing multiple systems under a shared file header.
	EmitHeader         bool
	ProtocolImport     string
	FieldImport        string
	VanishingImport    string
	LogDerivImport     string
	GrandProductImport string
	RowLimitImport     string
	// WritePcs, when true, additionally writes system.Pcs (which must be
	// non-nil) via WritePcsSystemZigWithOptions, using PcsImport/FriImport and
	// PcsConstPrefix below. Defaults to false: most call sites emit PCS
	// separately (e.g. to apply a per-scenario ConstPrefix), so leaving this
	// unset preserves their existing behavior.
	WritePcs       bool
	PcsImport      string
	FriImport      string
	PcsConstPrefix string
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
	if err := WritePublicInputSystemZigWithOptions(w, system.PublicInput, PublicInputZigOptions{
		ProtocolImport: opts.ProtocolImport,
		ConstName:      fmt.Sprintf("system_%d_public_input", index),
		EmitHeader:     false,
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
	if err := WriteGrandProductSystemZigWithOptions(w, index, system.GrandProduct, GrandProductZigOptions{
		EmitImport:         opts.EmitHeader,
		GrandProductImport: opts.GrandProductImport,
	}); err != nil {
		return err
	}
	if err := WriteRowLimitSystemZigWithOptions(w, index, system.RowLimit, RowLimitZigOptions{
		EmitImport: opts.EmitHeader,
		Import:     opts.RowLimitImport,
	}); err != nil {
		return err
	}
	if !opts.WritePcs {
		return nil
	}
	if system.Pcs == nil {
		return fmt.Errorf("codegen: WriteCompiledSystemZig: opts.WritePcs set but system.Pcs is nil")
	}
	return WritePcsSystemZigWithOptions(w, index, *system.Pcs, PcsZigOptions{
		PcsImport:   opts.PcsImport,
		FriImport:   opts.FriImport,
		FieldImport: opts.FieldImport,
		ConstPrefix: opts.PcsConstPrefix,
		EmitHeader:  opts.EmitHeader,
	})
}

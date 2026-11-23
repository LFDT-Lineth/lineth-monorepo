package codegen

import (
	"io"
)

// GrandProductZigOptions configures generated grandproduct.System data.
type GrandProductZigOptions struct {
	// EmitImport, when true, prepends `const grandproduct = <import>;`. The
	// fixture generator declares the import once in its header, so it leaves
	// this false; standalone callers set it true.
	EmitImport         bool
	GrandProductImport string
}

func defaultGrandProductZigOptions() GrandProductZigOptions {
	return GrandProductZigOptions{
		EmitImport:         true,
		GrandProductImport: `@import("../query/grandproduct.zig")`,
	}
}

// WriteGrandProductSystemZig writes the Zig source for a single
// GrandProductSystem, emitting `system_<index>_grandproduct` (plus its
// backing arrays). It emits data only; the Zig sub-verifier owns the
// boundary-check implementation.
func WriteGrandProductSystemZig(w io.Writer, index int, system GrandProductSystem) error {
	return WriteGrandProductSystemZigWithOptions(w, index, system, defaultGrandProductZigOptions())
}

func WriteGrandProductSystemZigWithOptions(
	w io.Writer, index int, system GrandProductSystem, opts GrandProductZigOptions,
) error {
	return executeZigTemplate(w, "grandproduct", grandProductTemplateData{Options: opts, Index: index, System: system})
}

type grandProductTemplateData struct {
	Options GrandProductZigOptions
	Index   int
	System  GrandProductSystem
}

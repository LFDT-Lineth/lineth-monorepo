package codegen

import (
	"io"
)

// LogDerivZigOptions configures generated logderivativesum.System data.
type LogDerivZigOptions struct {
	// EmitImport, when true, prepends `const logderivativesum = <import>;`.
	// The fixture generator declares the import once in its header, so it
	// leaves this false; standalone callers set it true.
	EmitImport     bool
	LogDerivImport string
}

func defaultLogDerivZigOptions() LogDerivZigOptions {
	return LogDerivZigOptions{
		EmitImport:     true,
		LogDerivImport: `@import("../query/logderivativesum.zig")`,
	}
}

// WriteLogDerivSystemZig writes the Zig source for a single LogDerivSystem,
// emitting `system_<index>_logderiv` (plus its backing arrays). It emits data
// only; the Zig sub-verifier owns the boundary-check implementation.
func WriteLogDerivSystemZig(w io.Writer, index int, system LogDerivSystem) error {
	return WriteLogDerivSystemZigWithOptions(w, index, system, defaultLogDerivZigOptions())
}

func WriteLogDerivSystemZigWithOptions(w io.Writer, index int, system LogDerivSystem, opts LogDerivZigOptions) error {
	return executeZigTemplate(w, "logderivativesum", logDerivTemplateData{Options: opts, Index: index, System: system})
}

type logDerivTemplateData struct {
	Options LogDerivZigOptions
	Index   int
	System  LogDerivSystem
}

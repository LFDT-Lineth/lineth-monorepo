package codegen

import (
	"io"
)

// RowLimitZigOptions configures WriteRowLimitSystemZigWithOptions.
type RowLimitZigOptions struct {
	// EmitImport, when true, prepends `const rowlimit = <import>;`. The
	// fixture generator declares the import once in its header, so it leaves
	// this false; standalone callers set it true.
	EmitImport bool
	Import     string
}

func defaultRowLimitZigOptions() RowLimitZigOptions {
	return RowLimitZigOptions{
		EmitImport: true,
		Import:     `@import("../query/rowlimit.zig")`,
	}
}

// WriteRowLimitSystemZig writes the Zig source for a single RowLimitSystem,
// emitting `system_<index>_rowlimit` (plus its backing arrays). It emits data
// only; the Zig sub-verifier owns the row-sum/limit-comparison implementation.
func WriteRowLimitSystemZig(w io.Writer, index int, system RowLimitSystem) error {
	return WriteRowLimitSystemZigWithOptions(w, index, system, defaultRowLimitZigOptions())
}

func WriteRowLimitSystemZigWithOptions(w io.Writer, index int, system RowLimitSystem, opts RowLimitZigOptions) error {
	return executeZigTemplate(w, "rowlimit", rowLimitTemplateData{Options: opts, Index: index, System: system})
}

type rowLimitTemplateData struct {
	Options RowLimitZigOptions
	Index   int
	System  RowLimitSystem
}

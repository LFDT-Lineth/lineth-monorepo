package codegen

import (
	"fmt"
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
	const ld = "logderivativesum"
	if opts.EmitImport {
		if _, err := fmt.Fprintf(w, "const %s = %s;\n\n", ld, opts.LogDerivImport); err != nil {
			return err
		}
	}

	// Per-query z_final_refs arrays.
	for q, query := range system.Queries {
		if _, err := fmt.Fprintf(w, "const system_%d_logderiv_query_%d_zfinal_refs = [_]%s.ScalarRef{\n", index, q, ld); err != nil {
			return err
		}
		for _, ref := range query.ZFinalRefs {
			if _, err := fmt.Fprintf(w, "    .{ .round = %d, .index = %d },\n", ref.Round, ref.Index); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "};\n\n"); err != nil {
			return err
		}
	}

	// Queries array.
	if _, err := fmt.Fprintf(w, "const system_%d_logderiv_queries = [_]%s.Query{\n", index, ld); err != nil {
		return err
	}
	for q, query := range system.Queries {
		if _, err := fmt.Fprintf(w,
			"    .{ .z_final_refs = &system_%d_logderiv_query_%d_zfinal_refs, .result_ref = .{ .round = %d, .index = %d }, .result_is_zero = %t }, // query: \"%s\"\n",
			index, q, query.ResultRef.Round, query.ResultRef.Index, query.ResultIsZero, zigString(query.SourceName)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "};\n\n"); err != nil {
		return err
	}

	// System value.
	if _, err := fmt.Fprintf(w,
		"// logderiv system: \"%s\"\nconst system_%d_logderiv = %s.System{ .queries = &system_%d_logderiv_queries };\n\n",
		zigString(system.SourceName), index, ld, index); err != nil {
		return err
	}
	return nil
}

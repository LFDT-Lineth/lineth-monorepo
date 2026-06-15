package codegen

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
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

	// Per-query z_finals arrays.
	for q, query := range system.Queries {
		if _, err := fmt.Fprintf(w, "const system_%d_logderiv_query_%d_zfinals = [_][6]u32{\n", index, q); err != nil {
			return err
		}
		for _, val := range query.ZFinals {
			if _, err := fmt.Fprintf(w, "    %s,\n", extLimbs(val)); err != nil {
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
			"    .{ .z_finals = &system_%d_logderiv_query_%d_zfinals, .result = %s, .result_is_zero = %t }, // query: \"%s\"\n",
			index, q, extLimbs(query.Result), query.ResultIsZero, zigString(query.SourceName)); err != nil {
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

func extLimbs(e field.Ext) string {
	a0, a1, b0, b1, c0, c1 := field.ExtToUint64s(&e)
	return fmt.Sprintf(".{ %d, %d, %d, %d, %d, %d }", a0, a1, b0, b1, c0, c1)
}

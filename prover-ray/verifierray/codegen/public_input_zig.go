package codegen

import (
	"fmt"
	"io"
	"strings"
)

// PublicInputZigOptions configures generated protocol.public_input.Spec data.
type PublicInputZigOptions struct {
	ProtocolImport string
	ConstName      string
	EmitHeader     bool
}

func defaultPublicInputZigOptions() PublicInputZigOptions {
	return PublicInputZigOptions{
		ProtocolImport: `@import("../protocol/root.zig")`,
		ConstName:      "public_input",
		EmitHeader:     true,
	}
}

// WritePublicInputSystemZig writes generated Zig source containing the
// protocol.public_input.Spec value for the supplied public-input layout.
func WritePublicInputSystemZig(w io.Writer, system PublicInputSystem) error {
	return WritePublicInputSystemZigWithOptions(w, system, defaultPublicInputZigOptions())
}

func WritePublicInputSystemZigWithOptions(w io.Writer, system PublicInputSystem, opts PublicInputZigOptions) error {
	return executeZigTemplate(w, "public_input", publicInputZigData{Options: opts, System: system})
}

type publicInputZigData struct {
	Options PublicInputZigOptions
	System  PublicInputSystem
}

func publicInputRefs(refs []PublicInputRef) string {
	if len(refs) == 0 {
		return "{}"
	}

	var out strings.Builder
	out.WriteString("{ ")
	for i, ref := range refs {
		if i > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(
			&out,
			".{ .statement_index = %d, .round = %d, .index = %d }",
			ref.StatementIndex,
			ref.Round,
			ref.Index,
		)
	}
	out.WriteString(" }")
	return out.String()
}

// bitWords renders bit-set words as a Zig array literal body, in hex so the
// set bits stay readable.
func bitWords(words []uint64) string {
	if len(words) == 0 {
		return "{}"
	}
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = fmt.Sprintf("0x%x", w)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

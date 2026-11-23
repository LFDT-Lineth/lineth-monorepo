package codegen

import (
	"io"
)

// SharedRandomnessZigOptions configures generated shared_randomness.System data.
type SharedRandomnessZigOptions struct {
	// EmitImport, when true, prepends `const shared_randomness = <import>;`. The
	// fixture generator declares the import once in its header, so it leaves
	// this false; standalone callers set it true.
	EmitImport             bool
	SharedRandomnessImport string
}

func defaultSharedRandomnessZigOptions() SharedRandomnessZigOptions {
	return SharedRandomnessZigOptions{
		EmitImport:             true,
		SharedRandomnessImport: `@import("../query/shared_randomness.zig")`,
	}
}

// WriteSharedRandomnessSystemZig writes the Zig source for a single
// SharedRandomnessSystem, emitting `system_<index>_shared_randomness` (plus
// its backing arrays). It emits data only; the Zig sub-verifier owns the
// sponge-recompute/comparison implementation.
func WriteSharedRandomnessSystemZig(w io.Writer, index int, system SharedRandomnessSystem) error {
	return WriteSharedRandomnessSystemZigWithOptions(w, index, system, defaultSharedRandomnessZigOptions())
}

func WriteSharedRandomnessSystemZigWithOptions(
	w io.Writer, index int, system SharedRandomnessSystem, opts SharedRandomnessZigOptions,
) error {
	data := sharedRandomnessTemplateData{Options: opts, Index: index, System: system}
	return executeZigTemplate(w, "shared_randomness", data)
}

type sharedRandomnessTemplateData struct {
	Options SharedRandomnessZigOptions
	Index   int
	System  SharedRandomnessSystem
}

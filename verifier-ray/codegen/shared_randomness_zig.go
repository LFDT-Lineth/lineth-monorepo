package codegen

import (
	"io"
	"text/template"
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

func WriteSharedRandomnessSystemZigWithOptions(w io.Writer, index int, system SharedRandomnessSystem, opts SharedRandomnessZigOptions) error {
	tmpl, err := template.New("shared_randomness").Funcs(template.FuncMap{
		"zig": ZigString,
	}).Parse(sharedRandomnessZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, sharedRandomnessTemplateData{Options: opts, Index: index, System: system})
}

type sharedRandomnessTemplateData struct {
	Options SharedRandomnessZigOptions
	Index   int
	System  SharedRandomnessSystem
}

const sharedRandomnessZigTemplate = `{{if .Options.EmitImport}}const shared_randomness = {{.Options.SharedRandomnessImport}};

{{end}}// shared-randomness system: "{{zig .System.SourceName}}"
const system_{{.Index}}_shared_randomness_rounds = [_]shared_randomness.Round{
{{range .System.Rounds}}    .{ .has_commitment = {{.HasCommitment}}, .round = {{.RoundIndex}} },
{{end}}};

const system_{{.Index}}_shared_randomness_contribution_refs = [_]shared_randomness.ScalarRef{
{{range .System.ContributionRefs}}    .{ .round = {{.Round}}, .index = {{.Index}} },
{{end}}};

const system_{{.Index}}_shared_randomness = shared_randomness.System{ .rounds = &system_{{.Index}}_shared_randomness_rounds, .contribution_refs = &system_{{.Index}}_shared_randomness_contribution_refs };
`

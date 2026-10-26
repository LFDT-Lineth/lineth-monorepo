package codegen

import (
	"io"
	"text/template"
)

// ProgramVKZigOptions configures generated program_vk.System data.
type ProgramVKZigOptions struct {
	// EmitImport, when true, prepends `const program_vk = <import>;`. The
	// fixture generator declares the import once in its header, so it leaves
	// this false; standalone callers set it true.
	EmitImport      bool
	ProgramVKImport string
}

func defaultProgramVKZigOptions() ProgramVKZigOptions {
	return ProgramVKZigOptions{
		EmitImport:      true,
		ProgramVKImport: `@import("../query/program_vk.zig")`,
	}
}

// WriteProgramVKSystemZig writes the Zig source for a single ProgramVKSystem,
// emitting `system_<index>_program_vk` (plus its backing array). It emits data
// only; the Zig sub-verifier owns the comparison.
func WriteProgramVKSystemZig(w io.Writer, index int, system ProgramVKSystem) error {
	return WriteProgramVKSystemZigWithOptions(w, index, system, defaultProgramVKZigOptions())
}

func WriteProgramVKSystemZigWithOptions(w io.Writer, index int, system ProgramVKSystem, opts ProgramVKZigOptions) error {
	tmpl, err := template.New("program_vk").Funcs(template.FuncMap{
		"zig": ZigString,
	}).Parse(programVKZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, programVKTemplateData{Options: opts, Index: index, System: system})
}

type programVKTemplateData struct {
	Options ProgramVKZigOptions
	Index   int
	System  ProgramVKSystem
}

const programVKZigTemplate = `{{if .Options.EmitImport}}const program_vk = {{.Options.ProgramVKImport}};

{{end}}// program-vk system: "{{zig .System.SourceName}}"
const system_{{.Index}}_program_vk_refs = [_]program_vk.ScalarRef{
{{range .System.VKRefs}}    .{ .round = {{.Round}}, .index = {{.Index}} },
{{end}}};

const system_{{.Index}}_program_vk = program_vk.System{ .commitment_round = .{ .round = {{.System.CommitmentRound.RoundIndex}}, .has_commitment = {{.System.CommitmentRound.HasCommitment}} }, .vk_refs = &system_{{.Index}}_program_vk_refs };
`

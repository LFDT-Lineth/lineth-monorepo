package codegen

import (
	"io"
	"text/template"
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
	tmpl, err := template.New("logderiv").Funcs(template.FuncMap{
		"zig": ZigString,
	}).Parse(logDerivZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, logDerivTemplateData{Options: opts, Index: index, System: system})
}

type logDerivTemplateData struct {
	Options LogDerivZigOptions
	Index   int
	System  LogDerivSystem
}

const logDerivZigTemplate = `{{if .Options.EmitImport}}const logderivativesum = {{.Options.LogDerivImport}};

{{end}}{{range $q, $query := .System.Queries}}const system_{{$.Index}}_logderiv_query_{{$q}}_zfinal_refs = [_]logderivativesum.ScalarRef{
{{range $query.ZFinalRefs}}    .{ .round = {{.Round}}, .index = {{.Index}} },
{{end}}};

{{end}}// logderiv system: "{{zig .System.SourceName}}"
const system_{{.Index}}_logderiv_queries = [_]logderivativesum.Query{
{{range $q, $query := .System.Queries}}    .{ .z_final_refs = &system_{{$.Index}}_logderiv_query_{{$q}}_zfinal_refs, .result_ref = .{ .round = {{$query.ResultRef.Round}}, .index = {{$query.ResultRef.Index}} }, .result_is_zero = {{$query.ResultIsZero}} }, // query: "{{zig $query.SourceName}}"
{{end}}};

const system_{{.Index}}_logderiv = logderivativesum.System{ .queries = &system_{{.Index}}_logderiv_queries };
`

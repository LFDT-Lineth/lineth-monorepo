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
		"zig":        ZigString,
		"moduleSize": moduleSizeLiteral,
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

{{end}}{{range $q, $query := .System.Queries}}const system_{{$.Index}}_logderiv_query_{{$q}}_entries = [_]logderivativesum.Entry{
{{range $query.Entries}}    .{ .average_ref = .{ .round = {{.AverageRef.Round}}, .index = {{.AverageRef.Index}} }, .size = {{moduleSize .Size}} },
{{end}}};

{{end}}// logderiv system: "{{zig .System.SourceName}}"
const system_{{.Index}}_logderiv_queries = [_]logderivativesum.Query{
{{range $q, $query := .System.Queries}}    .{ .entries = &system_{{$.Index}}_logderiv_query_{{$q}}_entries, .result_ref = .{ .round = {{$query.ResultRef.Round}}, .index = {{$query.ResultRef.Index}} }, .result_is_zero = {{$query.ResultIsZero}} }, // query: "{{zig $query.SourceName}}"
{{end}}};

const system_{{.Index}}_logderiv = logderivativesum.System{ .queries = &system_{{.Index}}_logderiv_queries };
`

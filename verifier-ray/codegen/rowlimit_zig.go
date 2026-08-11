package codegen

import (
	"io"
	"text/template"
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
	tmpl, err := template.New("rowlimit").Funcs(template.FuncMap{
		"zig":        ZigString,
		"moduleSize": moduleSizeLiteral,
	}).Parse(rowLimitZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, rowLimitTemplateData{Options: opts, Index: index, System: system})
}

type rowLimitTemplateData struct {
	Options RowLimitZigOptions
	Index   int
	System  RowLimitSystem
}

const rowLimitZigTemplate = `{{if .Options.EmitImport}}const rowlimit = {{.Options.Import}};

{{end}}{{range $c, $check := .System.Checks}}const system_{{$.Index}}_rowlimit_check_{{$c}}_a_modules = [_]rowlimit.ModuleSize{
{{range $check.IncludedModules}}    {{moduleSize .}},
{{end}}};

const system_{{$.Index}}_rowlimit_check_{{$c}}_b_modules = [_]rowlimit.ModuleSize{
{{range $check.IncludingsModules}}    {{moduleSize .}},
{{end}}};

{{end}}// rowlimit system: "{{zig .System.SourceName}}"
const system_{{.Index}}_rowlimit_checks = [_]rowlimit.Check{
{{range $c, $check := .System.Checks}}    .{ .a_modules = &system_{{$.Index}}_rowlimit_check_{{$c}}_a_modules, .b_modules = &system_{{$.Index}}_rowlimit_check_{{$c}}_b_modules, .limit = {{$check.Limit}} },
{{end}}};

const system_{{.Index}}_rowlimit = rowlimit.System{ .checks = &system_{{.Index}}_rowlimit_checks };
`

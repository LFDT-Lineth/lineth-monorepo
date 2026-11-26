package codegen

import (
	"io"
	"text/template"
)

// E6CheckZigOptions configures generated e6check.System data.
type E6CheckZigOptions struct {
	// EmitImport, when true, prepends `const e6check = <import>;`. The fixture
	// generator declares the import once in its header, so it leaves this
	// false; standalone callers set it true.
	EmitImport    bool
	E6CheckImport string
}

func defaultE6CheckZigOptions() E6CheckZigOptions {
	return E6CheckZigOptions{
		EmitImport:    true,
		E6CheckImport: `@import("../query/e6check.zig")`,
	}
}

// WriteE6CheckSystemZig writes the Zig source for a single E6CheckSystem,
// emitting `system_<index>_e6check` (plus its backing array). It emits data
// only; the Zig sub-verifier owns the recomposition and comparison.
func WriteE6CheckSystemZig(w io.Writer, index int, system E6CheckSystem) error {
	return WriteE6CheckSystemZigWithOptions(w, index, system, defaultE6CheckZigOptions())
}

func WriteE6CheckSystemZigWithOptions(w io.Writer, index int, system E6CheckSystem, opts E6CheckZigOptions) error {
	tmpl, err := template.New("e6check").Funcs(template.FuncMap{
		"zig": ZigString,
	}).Parse(e6CheckZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, e6CheckTemplateData{Options: opts, Index: index, System: system})
}

type e6CheckTemplateData struct {
	Options E6CheckZigOptions
	Index   int
	System  E6CheckSystem
}

const e6CheckZigTemplate = `{{if .Options.EmitImport}}const e6check = {{.Options.E6CheckImport}};

{{end}}// e6check system: "{{zig .System.SourceName}}"
const system_{{.Index}}_e6check_checks = [_]e6check.Check{
{{range .System.Checks}}    .{ .cell = .{ .round = {{.Cell.Round}}, .index = {{.Cell.Index}} }, .lanes = .{ {{range $i, $l := .Lanes}}{{if $i}}, {{end}}.{ .round = {{$l.Round}}, .index = {{$l.Index}} }{{end}} } },
{{end}}};

const system_{{.Index}}_e6check = e6check.System{ .checks = &system_{{.Index}}_e6check_checks };
`

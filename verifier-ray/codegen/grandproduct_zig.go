package codegen

import (
	"io"
	"text/template"
)

// GrandProductZigOptions configures generated grandproduct.System data.
type GrandProductZigOptions struct {
	// EmitImport, when true, prepends `const grandproduct = <import>;`. The
	// fixture generator declares the import once in its header, so it leaves
	// this false; standalone callers set it true.
	EmitImport         bool
	GrandProductImport string
}

func defaultGrandProductZigOptions() GrandProductZigOptions {
	return GrandProductZigOptions{
		EmitImport:         true,
		GrandProductImport: `@import("../query/grandproduct.zig")`,
	}
}

// WriteGrandProductSystemZig writes the Zig source for a single
// GrandProductSystem, emitting `system_<index>_grandproduct` (plus its
// backing arrays). It emits data only; the Zig sub-verifier owns the
// boundary-check implementation.
func WriteGrandProductSystemZig(w io.Writer, index int, system GrandProductSystem) error {
	return WriteGrandProductSystemZigWithOptions(w, index, system, defaultGrandProductZigOptions())
}

func WriteGrandProductSystemZigWithOptions(w io.Writer, index int, system GrandProductSystem, opts GrandProductZigOptions) error {
	tmpl, err := template.New("grandproduct").Funcs(template.FuncMap{
		"zig": ZigString,
	}).Parse(grandProductZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, grandProductTemplateData{Options: opts, Index: index, System: system})
}

type grandProductTemplateData struct {
	Options GrandProductZigOptions
	Index   int
	System  GrandProductSystem
}

const grandProductZigTemplate = `{{if .Options.EmitImport}}const grandproduct = {{.Options.GrandProductImport}};

{{end}}{{range $q, $query := .System.Queries}}const system_{{$.Index}}_grandproduct_query_{{$q}}_zfinal_refs = [_]grandproduct.ScalarRef{
{{range $query.ZFinalRefs}}    .{ .round = {{.Round}}, .index = {{.Index}} },
{{end}}};

{{end}}// grandproduct system: "{{zig .System.SourceName}}"
const system_{{.Index}}_grandproduct_queries = [_]grandproduct.Query{
{{range $q, $query := .System.Queries}}    .{ .z_final_refs = &system_{{$.Index}}_grandproduct_query_{{$q}}_zfinal_refs, .result_ref = .{ .round = {{$query.ResultRef.Round}}, .index = {{$query.ResultRef.Index}} }, .expected = {{if $query.HasExpected}}{{$query.Expected}}{{else}}null{{end}} }, // query: "{{zig $query.SourceName}}"
{{end}}};

const system_{{.Index}}_grandproduct = grandproduct.System{ .queries = &system_{{.Index}}_grandproduct_queries };
`

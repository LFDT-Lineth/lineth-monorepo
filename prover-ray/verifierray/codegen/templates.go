package codegen

import (
	"embed"
	"io"
	"text/template"
)

// zigTemplates are the Zig source templates of the emitters, one per sub-verifier system.
//
//go:embed templates/*.zig.tmpl
var zigTemplates embed.FS

// zigTemplateFuncs are the helpers every Zig template may call.
var zigTemplateFuncs = template.FuncMap{
	"bitWords":   bitWords,
	"expr":       exprNodeLiteral,
	"ints":       IntSlice,
	"intArray":   intArray,
	"moduleSize": moduleSizeLiteral,
	"octuplet":   pcsOctupletLiteral,
	"refs":       publicInputRefs,
	"shifts":     intArray,
	"zig":        ZigString,
}

// executeZigTemplate renders templates/<name>.zig.tmpl with data to w.
func executeZigTemplate(w io.Writer, name string, data any) error {
	file := name + ".zig.tmpl"
	tmpl, err := template.New(file).Funcs(zigTemplateFuncs).ParseFS(zigTemplates, "templates/"+file)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

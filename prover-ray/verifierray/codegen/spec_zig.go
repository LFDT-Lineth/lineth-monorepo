package codegen

import (
	"io"
)

// SpecZigOptions configures the import and wrapping for generated protocol.Spec
// data.
type SpecZigOptions struct {
	ProtocolImport string
	ConstName      string
	EmitHeader     bool
}

func defaultSpecZigOptions() SpecZigOptions {
	return SpecZigOptions{
		ProtocolImport: `@import("../protocol/root.zig")`,
		ConstName:      "spec",
		EmitHeader:     true,
	}
}

// WriteSpecZig writes generated Zig source containing the protocol.Spec value
// for the supplied coin routing. It emits data only; Zig owns replay().
func WriteSpecZig(w io.Writer, routing CoinRouting) error {
	return WriteSpecZigWithOptions(w, routing, defaultSpecZigOptions())
}

func WriteSpecZigWithOptions(w io.Writer, routing CoinRouting, opts SpecZigOptions) error {
	return executeZigTemplate(w, "spec", specZigData{
		Options: opts,
		Routing: routing,
	})
}

type specZigData struct {
	Options SpecZigOptions
	Routing CoinRouting
}

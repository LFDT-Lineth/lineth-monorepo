// Package codegen generates a flat Go execution function from a compiled
// [wiop.System]. The output makes the prover's round-by-round structure
// explicit and human-readable, and eliminates the generic action-dispatch loop.
//
// Usage: go run ./wiop/codegen -pkg <pkg> -out <file> [-func <name>]
// The caller is responsible for building and compiling the System before
// invoking Generate.
package codegen

import (
	"fmt"
	"go/format"
	"io"
	"strings"
)

// CodeWriter is a small helper that accumulates Go source lines with
// consistent indentation, then formats the result with go/format.
type CodeWriter struct {
	buf    strings.Builder
	indent int
}

// Line appends a formatted line with the current indentation level.
func (w *CodeWriter) Line(format string, args ...any) {
	fmt.Fprintf(&w.buf, "%s%s\n", strings.Repeat("\t", w.indent), fmt.Sprintf(format, args...))
}

// Blank appends a blank line.
func (w *CodeWriter) Blank() { w.buf.WriteByte('\n') }

// In increases the indentation level by one.
func (w *CodeWriter) In() { w.indent++ }

// Out decreases the indentation level by one.
func (w *CodeWriter) Out() { w.indent-- }

// Bytes returns the gofmt-formatted source. Returns raw bytes on format error
// so callers can inspect malformed output.
func (w *CodeWriter) Bytes() []byte {
	src := w.buf.String()
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return []byte(src)
	}
	return formatted
}

// WriteTo writes the formatted source to dst.
func (w *CodeWriter) WriteTo(dst io.Writer) (int64, error) {
	b := w.Bytes()
	n, err := dst.Write(b)
	return int64(n), err
}

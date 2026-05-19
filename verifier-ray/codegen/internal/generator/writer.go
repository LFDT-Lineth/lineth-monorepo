package generator

import (
	"fmt"
	"strings"
)

// Writer accumulates generated Zig source with spaces for indentation.
type Writer struct {
	buf    strings.Builder
	indent int
}

func (w *Writer) Line(format string, args ...any) {
	fmt.Fprintf(&w.buf, "%s%s\n", strings.Repeat("    ", w.indent), fmt.Sprintf(format, args...))
}

func (w *Writer) Blank() {
	w.buf.WriteByte('\n')
}

func (w *Writer) In() {
	w.indent++
}

func (w *Writer) Out() {
	if w.indent == 0 {
		panic("generator.Writer.Out called with zero indentation")
	}
	w.indent--
}

func (w *Writer) String() string {
	return w.buf.String()
}

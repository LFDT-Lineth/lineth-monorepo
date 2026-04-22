// Package wiop contains the core types for the Wizard IOP framework.
package wiop

import (
	"fmt"
	"runtime"
	"strings"
)

// ContextFrame is a node in a tree of labelled contexts. It serves as the
// identity and provenance descriptor for all named objects in the framework
// (columns, queries, coins). Each frame records its parent frame, a
// human-readable label, and the program counter of the call site that created
// it via [ContextFrame.Childf].
//
// Identity comparison is by pointer: two *ContextFrame values are the same
// object only if they are the same pointer. [ContextFrame.Path] provides the
// string representation for display, serialisation and map keys where
// content-based equality is needed.
//
// The zero value is not useful; construct frames with [NewRootFrame] or
// [ContextFrame.Childf].
type ContextFrame struct {
	// Parent is the enclosing frame. It is nil for root frames created with
	// [NewRootFrame].
	Parent *ContextFrame
	// PC is the program counter of the call site that created this frame via
	// [ContextFrame.Childf]. It is zero for root frames. Use
	// [ContextFrame.CallerInfo] for a human-readable representation, or
	// [runtime.FuncForPC] for lower-level access.
	PC uintptr
	// Label is the human-readable name segment of this frame.
	Label string
	// ID is the structured identifier assigned to this frame when it is
	// registered with a [System] (via [Module.NewColumn], [Round.NewCell], or
	// [Round.NewCoinField]). The zero value (ObjectID(0)) means the frame has
	// not yet been registered. Use the [ObjectID] methods to extract the kind,
	// slot, and position encoded in the identifier.
	ID ObjectID
}

// NewRootFramef creates a root ContextFrame with no parent. The label is
// formatted with [fmt.Sprintf]. Root frames have a zero PC.
func NewRootFramef(msg string, args ...any) *ContextFrame {
	return &ContextFrame{
		Label: fmt.Sprintf(msg, args...),
	}
}

// Childf creates a new ContextFrame whose parent is f. The label is formatted
// with fmt.Sprintf(msg, args...). The PC is set to the program counter of the
// direct caller of Childf, allowing the declaration site to be recovered later
// via [ContextFrame.CallerInfo].
//
// Panics if f is nil.
func (f *ContextFrame) Childf(msg string, args ...any) *ContextFrame {
	if f == nil {
		panic("wiop: Childf called on a nil ContextFrame")
	}
	// runtime.Caller(1) captures the immediate caller of this function, i.e.
	// the site in user code that declared the child frame.
	pc, _, _, _ := runtime.Caller(1)
	return &ContextFrame{
		Parent: f,
		PC:     pc,
		Label:  fmt.Sprintf(msg, args...),
	}
}

// Path returns the full slash-separated path from the root frame down to f,
// by concatenating all ancestor labels. For example, a frame with ancestry
// "linea" → "permutation" → "acc" returns "linea/permutation/acc".
//
// Returns the empty string on a nil receiver.
func (f *ContextFrame) Path() string {
	if f == nil {
		return ""
	}
	if f.Parent == nil {
		return f.Label
	}
	return f.Parent.Path() + "/" + f.Label
}

// String implements [fmt.Stringer] and returns the same value as [ContextFrame.Path].
func (f *ContextFrame) String() string {
	return f.Path()
}

// CallerInfo returns a human-readable description of the call site recorded in
// [ContextFrame.PC], formatted as "file:line". Only the last two components of
// the file path are included for readability. Returns the empty string if PC
// is zero (e.g. for root frames) or if the PC cannot be resolved.
func (f *ContextFrame) CallerInfo() string {
	if f == nil || f.PC == 0 {
		return ""
	}
	fn := runtime.FuncForPC(f.PC)
	if fn == nil {
		return ""
	}
	file, line := fn.FileLine(f.PC)
	// Keep only the last two path components to avoid overly long strings
	// while still providing enough context to locate the source.
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return fmt.Sprintf("%s:%d", strings.Join(parts, "/"), line)
}

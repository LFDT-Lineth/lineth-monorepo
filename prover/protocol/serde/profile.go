package serde

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// ProfileNode is one node in the serialization size tree.
// Bytes is the total number of bytes written to the buffer while serializing
// this value, including all transitively reachable indirect children.
//
// For struct nodes the count includes:
//   - the fixed inline reservation (writeZeros) for the struct's own fields, and
//   - all heap bytes appended for indirect (pointer/slice/map/interface) fields.
//
// For inline struct fields (non-indirect structs nested inside another struct),
// Bytes captures only the heap contributions from their own indirect children,
// since the fixed inline bytes were already counted by the parent struct.
//
// Consequence: a parent's Bytes is NOT necessarily equal to the sum of its
// children's Bytes.  The difference is always the parent struct's own
// fixed-size inline footprint.
type ProfileNode struct {
	Label    string         // e.g. "Columns [column.Store]"
	Bytes    int64          // bytes contributed by this subtree
	Children []*ProfileNode // child nodes, sorted largest-first by WriteProfileReport
}

// profilerFrame is one entry on the sizeProfiler call stack.
type profilerFrame struct {
	node        *ProfileNode
	startOffset int64
}

// sizeProfiler tracks the recursive encoder call stack and builds a ProfileNode tree.
// It is attached to an encoder via the profiler field and is nil in normal (non-profiling) runs.
type sizeProfiler struct {
	root      *ProfileNode
	stack     []profilerFrame
	nextLabel string // field name set by patchStructBody; consumed on the next push
}

// push creates a new ProfileNode labeled with nextLabel (if set) + typeName,
// appends it as a child of the current top-of-stack, and pushes it.
func (p *sizeProfiler) push(typeName string, offset int64) {
	label := "[" + typeName + "]"
	if p.nextLabel != "" {
		label = p.nextLabel + " [" + typeName + "]"
		p.nextLabel = ""
	}
	node := &ProfileNode{Label: label}
	if len(p.stack) > 0 {
		parent := p.stack[len(p.stack)-1].node
		parent.Children = append(parent.Children, node)
	} else {
		p.root = node
	}
	p.stack = append(p.stack, profilerFrame{node: node, startOffset: offset})
}

// pop records node.Bytes = endOffset - startOffset and removes the top frame.
func (p *sizeProfiler) pop(endOffset int64) {
	n := len(p.stack)
	if n == 0 {
		return
	}
	frame := p.stack[n-1]
	p.stack = p.stack[:n-1]
	frame.node.Bytes = endOffset - frame.startOffset
}

// Profile runs serialization on v with size profiling enabled and returns the
// root ProfileNode containing the full tree.  The resulting bytes are discarded.
// The API is otherwise identical to Serialize.
func Profile(v any) (*ProfileNode, error) {
	enc := newEncoder()
	enc.profiler = &sizeProfiler{}

	_ = enc.write(FileHeader{})
	payloadStart := enc.offset

	_, err := encode(enc, reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}

	if len(enc.missingTypes) > 0 {
		names := make([]string, 0, len(enc.missingTypes))
		for t := range enc.missingTypes {
			names = append(names, t.String())
		}
		sort.Strings(names)
		return nil, fmt.Errorf(
			"profiling failed: %d unregistered concrete type(s) need to be added to impl_registry.go:\n  - %s",
			len(names),
			strings.Join(names, "\n  - "),
		)
	}

	// Primitives and pointers to primitives never push a profiler node inside
	// linearize.  Synthesise a root node so callers always get a non-nil tree.
	if enc.profiler.root == nil {
		enc.profiler.root = &ProfileNode{
			Label: "[" + reflect.TypeOf(v).String() + "]",
			Bytes: enc.offset - payloadStart,
		}
	}

	return enc.profiler.root, nil
}

// WriteProfileTo writes the ProfileNode tree to w.
// It sorts children largest-first, then emits a header and the full tree.
// Use WriteProfileReport for the common case of writing to a named file.
func WriteProfileTo(node *ProfileNode, w io.Writer) error {
	if node == nil {
		fmt.Fprintf(w, "Serialization Size Profile\n==========================\n\n(empty)\n")
		return nil
	}
	sortTree(node)
	fmt.Fprintf(w, "Serialization Size Profile\n")
	fmt.Fprintf(w, "==========================\n\n")
	writeProfileNode(w, node, "", true, node.Bytes)
	return nil
}

// PruneTree removes every node whose Bytes is strictly less than minBytes,
// together with all of its descendants.  The root is never removed; if its
// Bytes fall below the threshold the tree is returned unchanged.
// The function modifies the tree in place and returns the (possibly trimmed)
// root for convenience.
//
// Typical usage:
//
//	root, _ := serde.Profile(v)
//	root.PruneTree(1<<20) // hide everything below 1 MB
//	serde.WriteProfileReport(root, "profile.txt")
func (node *ProfileNode) PruneTree(minBytes int64) *ProfileNode {
	if node == nil {
		return nil
	}
	pruneChildren(node, minBytes)
	return node
}

// pruneChildren filters node.Children in-place, keeping only those whose
// Bytes >= minBytes, and recurses into the survivors.
func pruneChildren(node *ProfileNode, minBytes int64) {
	kept := node.Children[:0]
	for _, child := range node.Children {
		if child.Bytes >= minBytes {
			pruneChildren(child, minBytes)
			kept = append(kept, child)
		}
	}
	node.Children = kept
}

// sortTree sorts each node's children by Bytes descending, recursively.
func sortTree(n *ProfileNode) {
	if n == nil {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Bytes > n.Children[j].Bytes
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}

// writeProfileNode renders one tree node and recurses into its children.
//
// prefix is the accumulated indentation string for THIS node's line.
// isLast controls which branch connector to use (└─ vs ├─).
// total is the root's Bytes, used to compute percentage of the whole.
//
// The root call uses prefix="" and no branch connector so the top-level
// label is flush-left.  Every subsequent level gets a proper prefix so the
// tree connectors are correctly drawn.
func writeProfileNode(w io.Writer, n *ProfileNode, prefix string, isLast bool, total int64) {
	// Choose the connector that goes before this node's label.
	// The root (prefix=="") has no connector; all other nodes do.
	connector := "├─ "
	if prefix == "" {
		connector = ""
	} else if isLast {
		connector = "└─ "
	}

	pct := 0.0
	if total > 0 {
		pct = float64(n.Bytes) / float64(total) * 100
	}

	fmt.Fprintf(w, "%s%s%s: %s (%.1f%%)\n",
		prefix, connector, n.Label, fmtProfileBytes(n.Bytes), pct)

	// Build the prefix that children will receive.
	// Root's children start the indentation chain with no vertical bar on the
	// left (there is no parent branch to continue).
	// Deeper nodes extend the chain: non-last siblings carry "│   " so their
	// descendants draw the vertical continuation line; last siblings carry
	// "    " (spaces only) to leave the column blank.
	var childPrefix string
	switch {
	case prefix == "":
		childPrefix = "    " // root → depth-1 children indented by 4 spaces
	case isLast:
		childPrefix = prefix + "    "
	default:
		childPrefix = prefix + "│   "
	}

	for i, child := range n.Children {
		writeProfileNode(w, child, childPrefix, i == len(n.Children)-1, total)
	}
}

// fmtProfileBytes formats a byte count as a human-readable string (B/KB/MB/GB…).
func fmtProfileBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

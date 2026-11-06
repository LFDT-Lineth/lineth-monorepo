// Package node builds, on the recursion machine, the layer of a node of the
// conglomeration tree that combines its children's propagated public inputs:
// it takes the public inputs of up to [Arity] children, checks that the
// children agree on the values every shard shares (the program VK, the guest
// output, the shared randomness seed), multiplies their message-bus products,
// adds their shared randomness contributions, and exposes the result as its
// own public inputs under the children's tags, in the children's order,
// followed by the node's key box (see below), so a node proof has the
// public-input layout of a shard proof plus the key box.
//
// Three programs make the tree, one per role, so that one verification key
// serves every merge of a role whatever the depth of the tree:
//
//   - the leaf program merges shard proofs and exposes an empty key box;
//   - the internal program merges node proofs, of the leaf program or of
//     itself. It cannot hold its own key as a constant (the key commits to
//     the program), so the key it accepts for a node child is the one it
//     publishes in its key box, [KeyPI]: a leaf child is accepted under the
//     leaf program's key, a constant, and a node child must carry the same
//     key box as the node. The box is a claim: no node checks it;
//   - the root program is the internal program plus the root checks: every
//     bus product is one, and the key box is the internal program's key, a
//     constant, as the root is compiled after it. A bogus key used anywhere
//     in the tree would be the box the root sees.
//
// The programs verify nothing about the children: their values are plain
// inputs, and the key a child is verified under, the leaf key or the key
// box, is not even computed. It is the layer of the verifier programs that
// survives once the children's proofs are verified in the machine, where the
// inputs become the values read from the verified proofs; meanwhile it settles
// and exercises the interface with conglomeration. Dev and test only.
//
// A node has a fixed number of children, [Arity], each with an active flag: a
// merge of fewer children leaves the other slots inactive, so one program (one
// verification key) serves every merge.
package node

import (
	"errors"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
)

// Arity is the number of children of a node program: the most proofs one
// node of the conglomeration tree merges.
const Arity = 6

// KeyPI is the tag of the key box of a node proof: the verification key the
// node accepts its node children under, as [NumKey] base public inputs
// KeyPI_0 … KeyPI_7, the last public inputs of a node proof. A leaf node's box
// is empty (zero), a root's is the internal program's key.
const KeyPI wiop.PublicInputTag = "NodeVerificationKey"

// NumKey is the number of limbs of the key box: a [field.Octuplet].
const NumKey = 8

// Op is how a node combines the children's values of a public-input role.
type Op int

const (
	OpMul   Op = iota // the product of the children's values: a message-bus product
	OpEqual           // the same on every child, propagated: the program VK, the guest output, the seed
	OpAdd             // the sum of the children's values: a shared randomness contribution
	OpKey             // the key box: equal to the node's on a node child, empty on a leaf child
)

// Role is one public input of a child proof: its tag and index, whether it
// is an extension value, and how the node combines it.
type Role struct {
	Tag   wiop.PublicInputTag
	Index int
	Ext   bool
	Op    Op
}

// Layout is the public-input layout of a proof, one role per position: of a
// shard proof (see [LayoutOf]), or of a node proof (see [Layout.WithKey]).
type Layout []Role

// LayoutOf returns the layout of the proofs of sys, giving the public inputs
// of each tag the op of ops. It fails on a public input whose tag is not in
// ops: every public input of a child must be combined somehow.
func LayoutOf(sys *wiop.System, ops map[wiop.PublicInputTag]Op) (Layout, error) {
	l := make(Layout, len(sys.PublicInputs))
	seen := make([]bool, len(l))
	for tag, op := range ops {
		for i := 0; ; i++ {
			cell, pos := sys.LookupPublicInputByTag(tag, i)
			if pos < 0 {
				break
			}
			l[pos] = Role{Tag: tag, Index: i, Ext: cell.IsExtension(), Op: op}
			seen[pos] = true
		}
	}
	for pos, ok := range seen {
		if !ok {
			tag, _ := wiop.PublicInputTagOf(sys.PublicInputs[pos])
			return nil, fmt.Errorf("node: public input %d (%s) has no role", pos, tag)
		}
	}
	if len(l) == 0 {
		return nil, errors.New("node: the system has no public input")
	}
	return l, nil
}

// WithKey returns the layout of a node proof over the shard layout l: l
// followed by the key box.
func (l Layout) WithKey() Layout {
	n := make(Layout, 0, len(l)+NumKey)
	n = append(n, l...)
	for i := range NumKey {
		n = append(n, Role{Tag: KeyPI, Index: i, Op: OpKey})
	}
	return n
}

// Cells lays the public inputs of a child proof out as input cells, one per
// role, checking their number and that a base role holds a base value.
func (l Layout) Cells(pub wiop.PublicInput) ([]recursion.Cell, error) {
	if len(pub) != len(l) {
		return nil, fmt.Errorf("%d public inputs, want %d", len(pub), len(l))
	}
	cells := make([]recursion.Cell, len(l))
	for pos, role := range l {
		if role.Ext {
			cells[pos] = recursion.E6Cell(pub[pos].AsExt())
			continue
		}
		if !pub[pos].IsBase() {
			return nil, fmt.Errorf("public input %s_%d is not a base field element", role.Tag, role.Index)
		}
		cells[pos] = recursion.ScalarCell(pub[pos].AsBase())
	}
	return cells, nil
}

// KeyOf returns the key box of a node proof: its last [NumKey] public inputs.
func KeyOf(pub wiop.PublicInput) (field.Octuplet, error) {
	var key field.Octuplet
	if len(pub) < NumKey {
		return key, fmt.Errorf("node: %d public inputs, no key box", len(pub))
	}
	for i := range key {
		v := pub[len(pub)-NumKey+i]
		if !v.IsBase() {
			return key, fmt.Errorf("node: %s_%d is not a base field element", KeyPI, i)
		}
		key[i] = v.AsBase()
	}
	return key, nil
}

// Kind is the role of a node program in the tree.
type Kind int

const (
	Leaf     Kind = iota // merges shard proofs
	Internal             // merges node proofs
	Root                 // finalizes the tree: an internal node with the root checks
)

// Program is a compiled node program.
type Program struct {
	*recursion.Program
	Kind Kind
	// Child is the layout of a child's public inputs: the shard layout for
	// a leaf program, the node layout for the others.
	Child Layout
	// Layout is the layout of the program's public inputs: the node layout.
	Layout Layout
}

// Child is one child of a node: its public inputs and, for an internal or
// root program, whether it is a leaf node, whose key box is empty and whose
// proof is accepted under the leaf program's key rather than the key box. A
// leaf program ignores Leaf.
type Child struct {
	Pub  wiop.PublicInput
	Leaf bool
}

// Key returns the verification key of the program: the commitment to its
// setup, which holds the program.
func (p *Program) Key() field.Octuplet { return p.Sys.PrecomputedCommitment }

// NewLeaf builds and compiles the leaf program of the shard layout l.
func NewLeaf(l Layout, opts ...compilers.Option) (*Program, error) {
	return build(l, Leaf, field.Octuplet{}, opts...)
}

// NewInternal builds and compiles the internal program of the shard layout
// l. The verifier program will also take the leaf program's key, to verify
// leaf children under; this layer verifies nothing, so it does not.
func NewInternal(l Layout, opts ...compilers.Option) (*Program, error) {
	return build(l, Internal, field.Octuplet{}, opts...)
}

// NewRoot builds and compiles the root program of the shard layout l,
// pinning the key box to nodeKey, the internal program's key.
func NewRoot(l Layout, nodeKey field.Octuplet, opts ...compilers.Option) (*Program, error) {
	return build(l, Root, nodeKey, opts...)
}

// build builds the program of a kind. Its inputs are the key box (internal
// and root programs; a leaf's is empty) then, for each of the [Arity]
// children, an active flag (0 or 1), a leaf flag (internal and root
// programs) and the child's public inputs in the child layout's order; the
// first child is always active. An inactive child contributes the neutral
// value of every role.
func build(l Layout, kind Kind, nodeKey field.Octuplet, opts ...compilers.Option) (*Program, error) {
	if len(l) == 0 {
		return nil, errors.New("node: empty layout")
	}
	for _, role := range l {
		if role.Op == OpKey {
			return nil, errors.New("node: the shard layout has a key box")
		}
	}
	child := l
	if kind != Leaf {
		child = l.WithKey()
	}
	m := &recursion.Machine{}
	one := m.Const(recursion.ScalarCell(field.One()))
	zero := m.Const(recursion.Cell{})

	// The key box: an input the internal program publishes as its claim, which
	// the root pins to the internal program's key; a leaf's is empty.
	box := make([]int, NumKey)
	for j := range box {
		box[j] = zero
		if kind != Leaf {
			box[j] = m.InputScalar()
		}
	}
	if kind == Root {
		for j := range box {
			m.AssertEqE6(box[j], m.Const(recursion.ScalarCell(nodeKey[j])))
		}
	}

	active := make([]int, Arity)
	leaf := make([]int, Arity)
	values := make([][]int, Arity)
	for c := range Arity {
		active[c] = m.InputScalar()
		if kind != Leaf {
			leaf[c] = m.InputScalar()
		}
		values[c] = make([]int, len(child))
		for pos, role := range child {
			if role.Ext {
				values[c][pos] = m.InputE6()
			} else {
				values[c][pos] = m.InputScalar()
			}
		}
	}
	m.AssertEqE6(active[0], one)

	// Fold every role over the children: an inactive child's value is
	// replaced by the role's neutral value, or compared with itself.
	results := make([]int, len(child))
	for pos, role := range child {
		acc := values[0][pos]
		for c := 1; c < Arity; c++ {
			v := values[c][pos]
			switch role.Op {
			case OpEqual:
				m.AssertEqE6(m.Select(active[c], v, acc), acc)
			case OpMul:
				acc = m.Mul(acc, m.Select(active[c], v, one))
			case OpAdd:
				acc = m.Add(acc, m.Select(active[c], v, zero))
			case OpKey:
				// Checked below, for every child.
			default:
				return nil, fmt.Errorf("node: unknown op %d of %s_%d", role.Op, role.Tag, role.Index)
			}
		}
		if kind == Root && role.Op == OpMul {
			m.AssertEqE6(acc, one)
		}
		results[pos] = acc
	}

	// The key rule of an internal or root node: a node child carries the
	// node's key box, a leaf child an empty one. The key the child's proof is
	// verified under, the leaf program's key or the box by the leaf flag, is
	// left to the verifier program: this layer verifies nothing.
	if kind != Leaf {
		for c := range Arity {
			for j := range NumKey {
				v := values[c][len(l)+j]
				want := m.Select(leaf[c], zero, box[j])
				m.AssertEqE6(m.Select(active[c], want, v), v)
			}
		}
		for j := range NumKey {
			results[len(l)+j] = box[j]
		}
	}

	layout := l.WithKey()
	for pos, role := range l {
		kind := recursion.KindScalar
		if role.Ext {
			kind = recursion.KindE6
		}
		m.Public(results[pos], kind, role.Tag, role.Index)
	}
	for j := range NumKey {
		m.Public(box[j], recursion.KindScalar, KeyPI, j)
	}

	rp, err := m.Compile(opts...)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	p := &Program{Program: rp, Kind: kind, Child: child, Layout: layout}
	return p, p.checkLayout()
}

// checkLayout checks that the compiled program exposes the layout: every
// role at its position, with its width.
func (p *Program) checkLayout() error {
	if n := len(p.Sys.PublicInputs); n != len(p.Layout) {
		return fmt.Errorf("node: %d public inputs, want %d", n, len(p.Layout))
	}
	for pos, role := range p.Layout {
		cell, got := p.Sys.LookupPublicInputByTag(role.Tag, role.Index)
		if got != pos {
			return fmt.Errorf("node: %s_%d is public input %d, want %d", role.Tag, role.Index, got, pos)
		}
		if cell.IsExtension() != role.Ext {
			return fmt.Errorf("node: %s_%d has the wrong width", role.Tag, role.Index)
		}
	}
	return nil
}

// Inputs lays the key box and 1 to [Arity] children out as the program's
// input cells: the given children active with their values, the other slots
// inactive and zero. A leaf program ignores key.
func (p *Program) Inputs(key field.Octuplet, children ...Child) ([]recursion.Cell, error) {
	if len(children) < 1 || len(children) > Arity {
		return nil, fmt.Errorf("node: %d children, want 1 to %d", len(children), Arity)
	}
	perChild := 1 + len(p.Child)
	if p.Kind != Leaf {
		perChild++
	}
	cells := make([]recursion.Cell, 0, NumKey+Arity*perChild)
	if p.Kind != Leaf {
		for _, limb := range key {
			cells = append(cells, recursion.ScalarCell(limb))
		}
	}
	for c := range Arity {
		if c >= len(children) {
			cells = append(cells, make([]recursion.Cell, perChild)...)
			continue
		}
		values, err := p.Child.Cells(children[c].Pub)
		if err != nil {
			return nil, fmt.Errorf("node: child %d: %w", c, err)
		}
		cells = append(cells, recursion.ScalarCell(field.One()))
		if p.Kind != Leaf {
			var leaf field.Element
			if children[c].Leaf {
				leaf.SetOne()
			}
			cells = append(cells, recursion.ScalarCell(leaf))
		}
		cells = append(cells, values...)
	}
	return cells, nil
}

// ProveChildren executes the program on the key box and the children, proves
// the execution and verifies the proof. Children that disagree on a
// propagated value, a key box breaking the key rule, or an unbalanced bus at
// the root, fail at verification: the machine's equality assertions are
// checked by the proof, not by the execution.
func (p *Program) ProveChildren(key field.Octuplet, children ...Child) (wiop.Proof, wiop.PublicInput, error) {
	cells, err := p.Inputs(key, children...)
	if err != nil {
		return wiop.Proof{}, nil, err
	}
	run, err := p.Execute(cells)
	if err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("node: %w", err)
	}
	proof, pub := p.Prove(run)
	if err := p.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("node: verifying the node proof: %w", err)
	}
	return proof, pub, nil
}

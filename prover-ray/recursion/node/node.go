// Package node builds, on the recursion machine, the layer of a node of the
// conglomeration tree that combines its children's propagated public inputs:
// it takes the public inputs of up to [Arity] children, checks that the
// children agree on the values every shard shares (the program VK, the guest
// output, the shared randomness seed), multiplies their message-bus products,
// adds their shared randomness contributions, and exposes the result as its
// own public inputs under the children's tags, in the children's order, so a
// node proof has the public-input layout of a shard proof. The root variant
// additionally checks that every bus product is one.
//
// The program verifies nothing about the children: their values are plain
// inputs. It is the layer of the verifier program that survives once the
// children's proofs are verified in the machine, where the inputs become the
// values read from the verified proofs; meanwhile it settles and exercises the
// interface with conglomeration. Dev and test only.
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

// Op is how a node combines the children's values of a public-input role.
type Op int

const (
	OpMul   Op = iota // the product of the children's values: a message-bus product
	OpEqual           // the same on every child, propagated: the program VK, the guest output, the seed
	OpAdd             // the sum of the children's values: a shared randomness contribution
)

// Role is one public input of a child proof: its tag and index, whether it
// is an extension value, and how the node combines it.
type Role struct {
	Tag   wiop.PublicInputTag
	Index int
	Ext   bool
	Op    Op
}

// Layout is the public-input layout of a child proof, one role per position,
// which a node proof reproduces: same tags, same order, same widths.
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

// Program is a compiled node program, or root program when Root.
type Program struct {
	*recursion.Program
	Layout Layout
	Root   bool
}

// New builds and compiles the node program of the layout. Its inputs are, for
// each of the [Arity] children, an active flag (0 or 1) then the child's
// public inputs in layout order; the first child is always active. An
// inactive child contributes the neutral value of every role. The root
// program also asserts that every [OpMul] result is one.
func New(l Layout, root bool, opts ...compilers.Option) (*Program, error) {
	if len(l) == 0 {
		return nil, errors.New("node: empty layout")
	}
	m := &recursion.Machine{}
	one := m.Const(recursion.ScalarCell(field.One()))
	zero := m.Const(recursion.Cell{})
	active := make([]int, Arity)
	values := make([][]int, Arity)
	for c := range Arity {
		active[c] = m.InputScalar()
		values[c] = make([]int, len(l))
		for pos, role := range l {
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
	results := make([]int, len(l))
	for pos, role := range l {
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
			default:
				return nil, fmt.Errorf("node: unknown op %d of %s_%d", role.Op, role.Tag, role.Index)
			}
		}
		if root && role.Op == OpMul {
			m.AssertEqE6(acc, one)
		}
		results[pos] = acc
	}
	for pos, role := range l {
		kind := recursion.KindScalar
		if role.Ext {
			kind = recursion.KindE6
		}
		m.Public(results[pos], kind, role.Tag, role.Index)
	}

	rp, err := m.Compile(opts...)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	p := &Program{Program: rp, Layout: l, Root: root}
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

// Inputs lays the public inputs of 1 to [Arity] children out as the
// program's input cells: the given children active with their values, the
// other slots inactive and zero.
func (p *Program) Inputs(pubs ...wiop.PublicInput) ([]recursion.Cell, error) {
	if len(pubs) < 1 || len(pubs) > Arity {
		return nil, fmt.Errorf("node: %d children, want 1 to %d", len(pubs), Arity)
	}
	cells := make([]recursion.Cell, 0, Arity*(1+len(p.Layout)))
	for c := range Arity {
		if c >= len(pubs) {
			cells = append(cells, make([]recursion.Cell, 1+len(p.Layout))...)
			continue
		}
		values, err := p.Layout.Cells(pubs[c])
		if err != nil {
			return nil, fmt.Errorf("node: child %d: %w", c, err)
		}
		cells = append(cells, recursion.ScalarCell(field.One()))
		cells = append(cells, values...)
	}
	return cells, nil
}

// ProveChildren executes the program on the children's public inputs, proves
// the execution and verifies the proof. Children that disagree on a
// propagated value, or an unbalanced bus at the root, fail at verification:
// the machine's equality assertions are checked by the proof, not by the
// execution.
func (p *Program) ProveChildren(pubs ...wiop.PublicInput) (wiop.Proof, wiop.PublicInput, error) {
	cells, err := p.Inputs(pubs...)
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

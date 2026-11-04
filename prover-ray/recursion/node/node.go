// Package node builds, on the recursion machine, the program of a node of the
// conglomeration tree: it takes the propagated public inputs of one or two
// children, checks that the children agree on the program VK and the guest
// output, multiplies their message-bus products, and exposes the result as
// its own public inputs, in the same roles. The root variant additionally
// checks that every bus product is one.
//
// The program verifies nothing about the children: their values are plain
// inputs. It is the layer of the verifier program that survives once the
// children's proofs are verified in the machine, and it serves meanwhile to
// settle and exercise the interface with conglomeration. Not sound until then.
//
// A bus product is an E6 value exposed as six base public inputs (lane l of
// handle h is BusTag_(6h+l)), because wiop has no extension public input bound
// to base lanes; a shard proof exposes one extension cell per handle instead.
package node

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
)

// The public-input tags of a node proof. They are the node system's own; a
// child's values are located by the caller, by whatever layout its proof has.
const (
	BusTag wiop.PublicInputTag = "NodeMessageBus"
	VKTag  wiop.PublicInputTag = "NodeProgramVK"
	OutTag wiop.PublicInputTag = "NodeGuestOutput"
)

// Layout is the shape of the propagated public inputs: the number of
// message-bus handles, of program VK limbs and of guest output bytes.
type Layout struct {
	NumBus, NumVK, NumOut int
}

// Child is one child's propagated values, as decoded from its proof's public
// inputs: one bus product per handle, the program VK limbs and the guest
// output bytes.
type Child struct {
	Bus []field.Ext
	VK  []field.Element
	Out []field.Element
}

// Positions are the positions, in a node proof's public inputs, of each role:
// the six lanes of each bus product, the VK limbs and the output bytes.
type Positions struct {
	Bus     [][]int
	VK, Out []int
}

// Program is a compiled node program (two children) or root program (one
// child, whose bus products must all be one).
type Program struct {
	*recursion.Program
	Layout   Layout
	Children int
	Root     bool
	pos      Positions
}

// New builds and compiles the program for children (1 or 2) of the given
// layout. Every child's values are inputs, in child order: the bus products,
// the VK limbs, then the output bytes.
func New(l Layout, children int, root bool, opts ...compilers.Option) (*Program, error) {
	if children < 1 || children > 2 {
		return nil, fmt.Errorf("node: %d children, want 1 or 2", children)
	}
	if l.NumBus < 1 || l.NumVK < 0 || l.NumOut < 0 {
		return nil, fmt.Errorf("node: invalid layout %+v", l)
	}

	m := &recursion.Machine{}
	bus := make([][]int, children)
	vk := make([][]int, children)
	out := make([][]int, children)
	for c := range children {
		bus[c] = inputs(m.InputE6, l.NumBus)
		vk[c] = inputs(m.InputScalar, l.NumVK)
		out[c] = inputs(m.InputScalar, l.NumOut)
	}

	prod := bus[0]
	for c := 1; c < children; c++ {
		for i := range l.NumVK {
			m.AssertEqE6(vk[0][i], vk[c][i])
		}
		for k := range l.NumOut {
			m.AssertEqE6(out[0][k], out[c][k])
		}
		next := make([]int, l.NumBus)
		for h := range l.NumBus {
			next[h] = m.Mul(prod[h], bus[c][h])
		}
		prod = next
	}
	if root {
		one := m.Const(recursion.ScalarCell(field.One()))
		for h := range l.NumBus {
			m.AssertEqE6(prod[h], one)
		}
	}

	// The public inputs, in role order: bus products, VK, output.
	for h := range l.NumBus {
		m.Public(prod[h], recursion.KindE6, BusTag, recursion.KindE6*h)
	}
	for i := range l.NumVK {
		m.Public(vk[0][i], recursion.KindScalar, VKTag, i)
	}
	for k := range l.NumOut {
		m.Public(out[0][k], recursion.KindScalar, OutTag, k)
	}

	rp, err := m.Compile(opts...)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	p := &Program{Program: rp, Layout: l, Children: children, Root: root}
	p.pos = Positions{
		Bus: make([][]int, l.NumBus),
		VK:  positions(rp.Sys, VKTag, 0, l.NumVK),
		Out: positions(rp.Sys, OutTag, 0, l.NumOut),
	}
	for h := range l.NumBus {
		p.pos.Bus[h] = positions(rp.Sys, BusTag, recursion.KindE6*h, recursion.KindE6)
	}
	return p, nil
}

// inputs allocates n input cells with alloc.
func inputs(alloc func() int, n int) []int {
	addrs := make([]int, n)
	for i := range addrs {
		addrs[i] = alloc()
	}
	return addrs
}

// positions returns the positions of the public inputs tag_first..tag_first+n-1.
func positions(sys *wiop.System, tag wiop.PublicInputTag, first, n int) []int {
	pos := make([]int, n)
	for i := range pos {
		_, pos[i] = sys.LookupPublicInputByTag(tag, first+i)
	}
	return pos
}

// NumPublicInputs returns the number of public inputs of a proof of p.
func (p *Program) NumPublicInputs() int { return len(p.Sys.PublicInputs) }

// Positions returns the positions of each role in a proof's public inputs.
func (p *Program) Positions() Positions { return p.pos }

// Inputs lays the children's values out as the program's input cells.
func (p *Program) Inputs(children ...Child) ([]recursion.Cell, error) {
	if len(children) != p.Children {
		return nil, fmt.Errorf("node: %d children, the program takes %d", len(children), p.Children)
	}
	l := p.Layout
	cells := make([]recursion.Cell, 0, p.Children*(l.NumBus+l.NumVK+l.NumOut))
	for c, child := range children {
		if len(child.Bus) != l.NumBus || len(child.VK) != l.NumVK || len(child.Out) != l.NumOut {
			return nil, fmt.Errorf("node: child %d has %d bus products, %d VK limbs and %d output bytes, want %d, %d and %d",
				c, len(child.Bus), len(child.VK), len(child.Out), l.NumBus, l.NumVK, l.NumOut)
		}
		for _, b := range child.Bus {
			cells = append(cells, recursion.E6Cell(b))
		}
		for _, v := range child.VK {
			cells = append(cells, recursion.ScalarCell(v))
		}
		for _, o := range child.Out {
			cells = append(cells, recursion.ScalarCell(o))
		}
	}
	return cells, nil
}

// Decode reads the propagated values from the public inputs of a proof of p.
func (p *Program) Decode(pub wiop.PublicInput) (Child, error) {
	if len(pub) != p.NumPublicInputs() {
		return Child{}, fmt.Errorf("node: %d public inputs, want %d", len(pub), p.NumPublicInputs())
	}
	base := func(pos int) (field.Element, error) {
		if !pub[pos].IsBase() {
			return field.Element{}, fmt.Errorf("node: public input %d is not a base field element", pos)
		}
		return pub[pos].AsBase(), nil
	}
	var (
		child = Child{
			Bus: make([]field.Ext, p.Layout.NumBus),
			VK:  make([]field.Element, p.Layout.NumVK),
			Out: make([]field.Element, p.Layout.NumOut),
		}
		err error
	)
	for h, lanes := range p.pos.Bus {
		var cell recursion.Cell
		for l, pos := range lanes {
			if cell[l], err = base(pos); err != nil {
				return Child{}, err
			}
		}
		child.Bus[h] = recursion.CellE6(cell)
	}
	for i, pos := range p.pos.VK {
		if child.VK[i], err = base(pos); err != nil {
			return Child{}, err
		}
	}
	for k, pos := range p.pos.Out {
		if child.Out[k], err = base(pos); err != nil {
			return Child{}, err
		}
	}
	return child, nil
}

// ProveChildren executes the program on the children's values, proves the
// execution and verifies the proof. Children that disagree on the program VK
// or the guest output, or an unbalanced bus at the root, fail at verification:
// the machine's equality assertions are checked by the proof, not by the
// execution.
func (p *Program) ProveChildren(children ...Child) (wiop.Proof, wiop.PublicInput, error) {
	cells, err := p.Inputs(children...)
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

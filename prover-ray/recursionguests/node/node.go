// Package node builds, on the recursion machine, a node of the conglomeration
// tree: a program that verifies the proofs of up to [Arity] children and
// combines their public inputs. It checks that the children agree on the
// values every shard shares (the program VK, the guest output, the shared
// randomness seed), multiplies their message-bus products, adds their shared
// randomness contributions, and exposes the result as its own public inputs
// under the children's tags, in the children's order, followed by the node's
// key box (see below), so a node proof has the public-input layout of a shard
// proof plus the key box.
//
// A program is the in-circuit layer of one merge step of the conglomeration
// tree. The host side of a merge, which picks the program by the children's
// kind, checks their coverage and shape and proves with the program, is a
// conglomeration.Merger (see conglomeration.RecursionMerger).
//
// Three keys appear here and must be told apart:
//
//   - the program key, [Program.Key], is the verification key of a node
//     program, the commitment to its compiled setup. The leaf, internal and
//     root programs are three programs with three program keys. Their node
//     proofs share the layout, not the key: a leaf node verifies under the
//     leaf key, an internal node under the internal key;
//   - the child key is the key a child's proof is verified under: the shard
//     system's key for a shard child, the leaf program's key for a leaf
//     child, the key box for a node child. It is never an input of a
//     program, which knows it from the child's kind and leaf flag;
//   - the guest program VK (risc5.ProgramVKPI) is a public input of a shard
//     proof naming the guest binary the zkVM ran. It is propagated like any
//     other value shared by the shards and plays no part in verification here.
//
// Three programs make the tree, one per role, so that one program key
// serves every merge of a role whatever the depth of the tree: the leaf
// program merges shard proofs, the internal program merges node proofs, of
// the leaf program or of itself, and the root program is the internal
// program plus the root checks: every bus product is one, the key box is the
// internal program's key and the shared randomness seed is the digest of the
// contributions. Each program knows the child key of each child it accepts:
//
//	program   child          child key               known as
//	leaf      shard proof    shard system's key      a constant
//	internal  leaf node      leaf program's key      a constant
//	internal  internal node  internal program's key  the key box
//	root      leaf node      leaf program's key      a constant
//	root      internal node  internal program's key  a constant
//
// Every child key is a constant of the program, compiled after the child's,
// but one: the internal program verifying a node of itself, whose child key
// is its own program key, which it cannot hold as a constant since the key
// commits to the program. The key box, [KeyPI], exists for that one case. The
// internal program takes the key as an input, publishes it as a claim it
// cannot check, and demands that every node child carries the same box, so
// the value propagates unchanged to the root, which pins it once to the
// internal program's key, a constant there as the root is compiled after the
// internal program. A bogus key used anywhere in the tree would be the box
// the root sees. A leaf node's box is empty: a leaf has no node child. Were
// the tree two levels deep, leaves under a root, no program would verify a
// proof of itself and there would be no box.
//
// The internal program tells a leaf child from a node child by a per-child
// leaf flag, which selects the child key: the constant or the box. That is a
// select over a key value fed to one verifier, cheap because leaf and
// internal proofs are proofs of the same machine; with the real verifier it
// requires the two programs to compile to the same shape, that is the same
// chips at the same heights, which they will have to pad to, and, as the
// internal program verifies proofs of itself, that the verifier of that shape
// fits in a program of that shape: a fixed point, reached by padding after
// compiling. A program
// accepting shard proofs and node proofs alike would instead select between
// two verifiers, of two systems, and pay for both in every slot.
//
// A program verifies each child's proof with the in-machine verifier of its
// children's systems (package recursionguests/verifier) under the child key and
// folds the public inputs the verifier yields, so a child's values enter the
// node only through its verified proof. The caller chooses the verifier with
// a verifier.Constructor: verifier.New in production, which fails until the
// wiop verifier is written in the machine's instructions, and
// verifier.NewMock in tests.
//
// A node has a fixed number of children, [Arity], each with an active flag: a
// merge of fewer children leaves the other slots inactive, so one program (one
// verification key) serves every merge. An inactive slot holds a copy of the
// first child, whose proof the verifier checks as any other, so that the
// verifier needs no gate; its values are left out of the node's.
package node

import (
	"errors"
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/verifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
)

// Arity is the number of children of a node program: the most proofs one
// node of the conglomeration tree merges.
const Arity = 6

// KeyPI is the tag of the key box of a node proof, its last [NumKey] base
// public inputs KeyPI_0 … KeyPI_7: the program key the node accepts its node
// children under (see the package doc). Zero for a leaf node, the internal
// program's key otherwise.
const KeyPI wiop.PublicInputTag = "NodeVerificationKey"

// NumKey is the number of limbs of the key box: a [field.Octuplet].
const NumKey = 8

// Op is how a node combines the children's values of a public-input role.
type Op int

const (
	OpMul   Op = iota // the product of the children's values: a message-bus product
	OpEqual           // the same on every child, propagated: the program VK, the guest output
	OpAdd             // the sum of the children's values: a limb of the shared randomness contribution
	OpSeed            // a limb of the shared randomness seed γ: propagated as OpEqual, checked at the root (see seedRoles)
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

// seedRoles returns the positions of the OpSeed roles of l, the seed γ, and
// of its OpAdd roles, the contributions, both in index order. The root checks
// that γ is the digest of the contributions added over every shard, taken in
// index order, as multisethashing.ToSeed derives it, so a layout with OpSeed
// roles has [NumKey] of them and at least one OpAdd role, all base values. A
// layout without OpSeed roles has no seed check: both are nil.
func seedRoles(l Layout) (seed, contributions []int, err error) {
	for pos, role := range l {
		switch role.Op {
		case OpSeed:
			seed = append(seed, pos)
		case OpAdd:
			contributions = append(contributions, pos)
		}
	}
	if len(seed) == 0 {
		return nil, nil, nil
	}
	if len(seed) != NumKey {
		return nil, nil, fmt.Errorf("node: %d seed limbs, want %d", len(seed), NumKey)
	}
	if len(contributions) == 0 {
		return nil, nil, errors.New("node: a seed with no contribution to derive it from")
	}
	for _, positions := range [][]int{seed, contributions} {
		slices.SortFunc(positions, func(a, b int) int { return l[a].Index - l[b].Index })
		for i, pos := range positions {
			if l[pos].Ext {
				return nil, nil, fmt.Errorf("node: %s_%d is not a base value", l[pos].Tag, l[pos].Index)
			}
			if l[pos].Index != i {
				return nil, nil, fmt.Errorf("node: %s has no limb %d", l[pos].Tag, i)
			}
		}
	}
	return seed, contributions, nil
}

// Kind is the role of a node program in the tree: a property of the program,
// so each kind has its own program key.
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
	// Layout is the layout of the program's public inputs: the node layout.
	Layout Layout

	verifier verifier.Verifier
	// childSys is the system of the children verified under the constant
	// child key: the shard system for a leaf program, the leaf program's
	// for the others, whose node children are proofs of nodeSys, verified
	// under the key box instead: the internal program's system.
	childSys, nodeSys *wiop.System
	numProofCells     int // the cells the verifier allocates for one child
}

// Child is one child of a node: its proof and public inputs and, for an
// internal or root program, whether it is a leaf node, whose key box is empty
// and whose proof is verified under the leaf program's key rather than the
// key box. A leaf program ignores Leaf.
type Child struct {
	Proof wiop.Proof
	Pub   wiop.PublicInput
	Leaf  bool
}

// Key returns the program key, the verification key of the program: the key
// a proof of it is verified under.
func (p *Program) Key() field.Octuplet { return p.Sys.PrecomputedCommitment }

// NewLeaf builds and compiles the leaf program of the shard layout l, the
// layout of shardSys, whose children are shard proofs: proofs of shardSys,
// verified under its key, with the verifier newVerifier returns.
func NewLeaf(
	l Layout, shardSys *wiop.System, newVerifier verifier.Constructor, opts ...compilers.Option,
) (*Program, error) {
	if shardSys == nil {
		return nil, errors.New("node: no shard system")
	}
	return build(l, Leaf, newVerifier, shardSys, field.Octuplet{}, opts...)
}

// NewInternal builds and compiles the internal program of the shard layout
// l over the leaf program, whose children are node proofs: a leaf child is
// verified under the leaf program's key, a node child, a proof of the
// internal program itself, under the key box, with the verifier newVerifier
// returns.
func NewInternal(
	l Layout, leaf *Program, newVerifier verifier.Constructor, opts ...compilers.Option,
) (*Program, error) {
	if leaf == nil || leaf.Kind != Leaf {
		return nil, errors.New("node: no leaf program")
	}
	p, err := build(l, Internal, newVerifier, leaf.Sys, field.Octuplet{}, opts...)
	if err != nil {
		return nil, err
	}
	// The internal program verifies proofs of itself.
	p.nodeSys = p.Sys
	return p, nil
}

// NewRoot builds and compiles the root program of the shard layout l over
// the leaf and internal programs: the internal program, pinning the key box
// to the internal program's key, and checking every bus product is one and,
// when l has a seed, that it is the digest of the contributions. It verifies
// its children with the verifier newVerifier returns.
func NewRoot(
	l Layout, leaf, internal *Program, newVerifier verifier.Constructor, opts ...compilers.Option,
) (*Program, error) {
	if leaf == nil || leaf.Kind != Leaf {
		return nil, errors.New("node: no leaf program")
	}
	if internal == nil || internal.Kind != Internal {
		return nil, errors.New("node: no internal program")
	}
	p, err := build(l, Root, newVerifier, leaf.Sys, internal.Key(), opts...)
	if err != nil {
		return nil, err
	}
	p.nodeSys = internal.Sys
	return p, nil
}

// build builds the program of a kind over the verifier newVerifier returns
// for childSys, the system of the children verified under the constant child
// key, childSys's, pinning the key box to internalKey for the root. Its
// inputs are laid out as [Program.Inputs] describes; the first child is
// always active, and an inactive child contributes the neutral value of every
// role.
func build(
	l Layout, kind Kind, newVerifier verifier.Constructor,
	childSys *wiop.System, internalKey field.Octuplet, opts ...compilers.Option,
) (*Program, error) {
	if len(l) == 0 {
		return nil, errors.New("node: empty layout")
	}
	if newVerifier == nil {
		return nil, errors.New("node: no verifier")
	}
	v, err := newVerifier(childSys)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	for _, role := range l {
		if role.Op == OpKey {
			return nil, errors.New("node: the shard layout has a key box")
		}
	}
	seed, contributions, err := seedRoles(l)
	if err != nil {
		return nil, err
	}
	child := l
	if kind != Leaf {
		child = l.WithKey()
	}
	m := &recursion.Machine{}
	one := m.Const(recursion.ScalarCell(field.One()))
	zero := m.Const(recursion.Cell{})

	// The key box: a digest the internal program takes as an input and
	// publishes as its claim, which the root pins to the internal program's
	// key; a leaf's is empty. Its limbs are the public inputs.
	box := zero
	if kind != Leaf {
		box = m.InputDigest()
	}
	if kind == Root {
		verifier.AssertEqDigest(m, box, m.Const(recursion.Cell(internalKey)))
	}
	limbs := make([]int, NumKey)
	for j := range limbs {
		limbs[j] = zero
		if kind != Leaf {
			limbs[j] = m.Lanes(box, j, recursion.KindScalar)
		}
	}

	// The children: each one's proof is verified under the child key, the
	// constant or, for a node child of an internal or root program, the key
	// box, by the leaf flag; its public inputs are what the verifier yields.
	constKey := m.Const(recursion.Cell(childSys.PrecomputedCommitment))
	active := make([]int, Arity)
	leaf := make([]int, Arity)
	values := make([][]int, Arity)
	numProofCells := 0
	for c := range Arity {
		active[c] = m.InputScalar()
		if kind != Leaf {
			leaf[c] = m.InputScalar()
		}
		proof := v.Allocate(m)
		if c == 0 {
			numProofCells = len(proof)
		} else if len(proof) != numProofCells {
			return nil, fmt.Errorf("node: the verifier allocates %d cells for a child, then %d", numProofCells, len(proof))
		}
		key := constKey
		if kind != Leaf {
			key = m.Select(leaf[c], constKey, box)
		}
		values[c] = v.Define(m, key, proof)
		if len(values[c]) != len(child) {
			return nil, fmt.Errorf("node: the verifier yields %d public inputs, want %d", len(values[c]), len(child))
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
			case OpEqual, OpSeed:
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

	// The seed rule of the root: γ is the digest of the contributions added
	// over every shard, as multisethashing.ToSeed derives it, ray's
	// Merkle–Damgård hash of the limbs in index order, which Pack and MDHash
	// reproduce. Its lanes are compared to the seed's limbs one by one.
	if kind == Root && len(seed) > 0 {
		items := make([]int, len(contributions))
		kinds := make([]int, len(contributions))
		for i, pos := range contributions {
			items[i], kinds[i] = results[pos], recursion.KindScalar
		}
		digest := m.MDHash(m.Pack(items, kinds))
		for j, pos := range seed {
			m.AssertEqE6(m.Lanes(digest, j, recursion.KindScalar), results[pos])
		}
	}

	// The key rule of an internal or root node: a node child carries the
	// node's key box, a leaf child an empty one. This is what propagates the
	// box unchanged to the root, which is the only place it is checked.
	if kind != Leaf {
		for c := range Arity {
			for j := range NumKey {
				v := values[c][len(l)+j]
				want := m.Select(leaf[c], zero, limbs[j])
				m.AssertEqE6(m.Select(active[c], want, v), v)
			}
		}
		for j := range NumKey {
			results[len(l)+j] = limbs[j]
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
		m.Public(limbs[j], recursion.KindScalar, KeyPI, j)
	}

	rp, err := m.Compile(opts...)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	p := &Program{Program: rp, Kind: kind, Layout: layout, verifier: v, childSys: childSys, numProofCells: numProofCells}
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
// input cells: the key box (internal and root programs; a leaf's is empty)
// then, for each of the [Arity] slots, an active flag (0 or 1), a leaf flag
// (internal and root programs) and the cells of the child's proof as the
// verifier assigns them; the given children fill the first slots, the other
// slots are inactive copies of the first child. box is the node's key box,
// the internal program's key; a leaf program ignores it. It fails on a child
// whose proof the verifier does not accept.
func (p *Program) Inputs(box field.Octuplet, children ...Child) ([]recursion.Cell, error) {
	if len(children) < 1 || len(children) > Arity {
		return nil, fmt.Errorf("node: %d children, want 1 to %d", len(children), Arity)
	}
	// slots[c] is the slot of child c after its active flag: the leaf flag
	// (internal and root programs) and the proof cells.
	slots := make([][]recursion.Cell, len(children))
	for c, child := range children {
		sys := p.childSys
		if p.Kind != Leaf && !child.Leaf {
			sys = p.nodeSys
		}
		proof, err := p.verifier.Assign(sys, child.Proof, child.Pub)
		if err != nil {
			return nil, fmt.Errorf("node: child %d: %w", c, err)
		}
		if len(proof) != p.numProofCells {
			return nil, fmt.Errorf("node: child %d: the verifier assigns %d cells, want %d", c, len(proof), p.numProofCells)
		}
		if p.Kind != Leaf {
			var leaf field.Element
			if child.Leaf {
				leaf.SetOne()
			}
			slots[c] = append(slots[c], recursion.ScalarCell(leaf))
		}
		slots[c] = append(slots[c], proof...)
	}

	cells := make([]recursion.Cell, 0, 1+Arity*(1+len(slots[0])))
	if p.Kind != Leaf {
		cells = append(cells, recursion.Cell(box))
	}
	for c := range Arity {
		var active field.Element
		slot := slots[0]
		if c < len(children) {
			active.SetOne()
			slot = slots[c]
		}
		cells = append(cells, recursion.ScalarCell(active))
		cells = append(cells, slot...)
	}
	return cells, nil
}

// ProveChildren executes the program on the key box box and the children,
// proves the execution and verifies the proof under the program key. Children
// that disagree on a propagated value, a key box breaking the key rule, or at
// the root an unbalanced bus or a seed that is not the digest of the
// contributions, fail at verification: the machine's equality assertions are
// checked by the proof, not by the execution.
func (p *Program) ProveChildren(box field.Octuplet, children ...Child) (wiop.Proof, wiop.PublicInput, error) {
	cells, err := p.Inputs(box, children...)
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

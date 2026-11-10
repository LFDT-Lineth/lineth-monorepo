package gkrverifier

import (
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
)

// CompressionTable is a table of Poseidon2 compressions that the caller proves: the columns hold, in
// order, the eight elements of the left input, the eight of the right input, and the eight of the
// output of the compression C(left, right) = P(left ‖ right)[8..16] + right.
type CompressionTable [3 * poseidon2.BlockLen]*wiop.Column

// Inputs are the values that the caller holds in its own system and that the verifier is given, each
// as base cells.
type Inputs struct {
	// Seed holds the coordinates of the seed σ, an element of E₆, which the transcript writes before
	// anything else.
	Seed [6]*wiop.Cell
	// Claims holds the claims that gnark's Verify returns, by wire, in the order it returns them.
	Claims map[int][]Claim
}

// Verifier is the generated verifier of a circuit and a schedule for n variables. See [Define].
type Verifier struct {
	layout     *layout
	proof      proofModule
	transcript transcriptModule
	gate       gateModule
	fold       foldModule
	eq         eqModule
	rounds     roundModule
	inputs     inputsModule
}

// busModule is a module that exposes values to the value bus and reads values from it.
type busModule interface {
	producers() []wiop.Table
	consumers() []wiop.Table
}

// proofModule has one row per element of the flattened proof.
type proofModule struct {
	module *wiop.Module
	elem   [6]*wiop.Column // witness: the coordinates of the element
	id     *wiop.Column    // precomputed: the id of the element
	active *wiop.Column    // precomputed: 1 on the rows of the proof
}

// transcriptModule has one row per compression of the transcript.
type transcriptModule struct {
	module *wiop.Module
	state  [poseidon2.BlockLen]*wiop.Column // witness
	block  [poseidon2.BlockLen]*wiop.Column
	next   [poseidon2.BlockLen]*wiop.Column
	a, b   [6]*wiop.Column // the elements that the block takes its coordinates from
	// precomputed
	active, first, chain *wiop.Column // 1 on the rows of the transcript, on the first, and on all but the first
	shape                [len(blockShapes)]*wiop.Column
	readA, idA           *wiop.Column // 1 where the row reads A, and the id of the element
	readB, idB           *wiop.Column
	export, exportID     *wiop.Column // 1 on the rows that end a challenge, and the id of the challenge
}

// gateModule has one row per step of the gates' bytecode: o = k_m·a·b + k_a·a + k_b·b + k_o·o⁻ + k_c.
type gateModule struct {
	module  *wiop.Module
	a, b, o [6]*wiop.Column // witness
	// precomputed
	active                     *wiop.Column
	id                         *wiop.Column // the id of the row's result
	kMul, kA, kB, kOut, kConst *wiop.Column
	readA, readB, check        *wiop.Column // 1 where the row reads a, reads b, checks its result
	idA, idB, checkID          *wiop.Column // the ids of the values read, and of the value checked against
}

// moduleSize is the size of a module of the given number of rows: a power of two, at least 2.
func moduleSize(rows int) int { return max(2, utils.NextPowerOfTwo(rows)) }

// NbCompressions returns the number of compressions that the verifier of circuit and schedule for n
// variables looks up, which is the least number of rows of a table that holds them.
func NbCompressions(circuit gkr.SerializableCircuit, schedule constraint.GkrProvingSchedule, n int) (int, error) {
	l, err := newLayout(circuit, schedule, n)
	if err != nil {
		return 0, err
	}
	return len(l.rows), nil
}

// Define adds to sys the tables and constraints of the verifier of circuit and schedule for n
// variables, with the witness columns in round. The verifier checks that gnark's Verify accepts a
// flattened proof, given to [Verifier.Assign], with the seed in inputs, and that it returns the claims
// in inputs.
//
// Each of the verifier's compressions is a lookup into table, whose rows the caller must prove
// to be compressions: [Verifier.Compressions] returns the rows it needs.
func Define(
	sys *wiop.System,
	round *wiop.Round,
	circuit gkr.SerializableCircuit,
	schedule constraint.GkrProvingSchedule,
	n int,
	inputs Inputs,
	table CompressionTable,
) (*Verifier, error) {
	if table[0] == nil {
		panic("gkrverifier: Define needs a table of Poseidon2 compressions: constraining the compression " +
			"in place needs the Poseidon2 AIR of the recursion package to be exported")
	}
	l, err := newLayout(circuit, schedule, n)
	if err != nil {
		return nil, err
	}

	v := &Verifier{layout: l}
	ctx := sys.Context.Childf("gkrverifier")
	v.defineProof(sys, ctx.Childf("proof"), round)
	v.defineTranscript(sys, ctx.Childf("transcript"), round, table)
	v.defineGates(sys, ctx.Childf("gates"), round)
	v.defineFolds(sys, ctx.Childf("folds"), round)
	v.defineEqs(sys, ctx.Childf("eqs"), round)
	v.defineRounds(sys, ctx.Childf("rounds"), round)
	if err := v.defineInputs(sys, ctx.Childf("inputs"), round, inputs); err != nil {
		return nil, err
	}
	v.defineBus(sys, ctx.Childf("bus"))
	return v, nil
}

// precomputed declares a precomputed column of the module, holding values on its first rows and
// zero after.
func precomputed(ctx *wiop.ContextFrame, m *wiop.Module, values []int) *wiop.Column {
	elements := make([]field.Element, len(values))
	for i, x := range values {
		elements[i].SetUint64(uint64(x))
	}
	return precomputedElements(ctx, m, elements)
}

// precomputedElements is [precomputed] for values of the base field.
func precomputedElements(ctx *wiop.ContextFrame, m *wiop.Module, values []field.Element) *wiop.Column {
	vec := make([]field.Element, m.Size())
	copy(vec, values)
	return m.NewPrecomputedColumn(ctx, &wiop.ConcreteVector{Plain: field.VecFromBase(vec)})
}

// idColumns declares the precomputed columns of a selector and of ids for the value that some of the
// first rows of the module read: ids[i] is the id that row i reads, or -1 if it reads none, in which
// case the selector is 0 and the id is 0.
func idColumns(ctx *wiop.ContextFrame, m *wiop.Module, ids []int) (selector, id *wiop.Column) {
	selected, values := make([]int, len(ids)), make([]int, len(ids))
	for i, x := range ids {
		if x >= 0 {
			selected[i], values[i] = 1, x
		}
	}
	return precomputed(ctx.Childf("read"), m, selected), precomputed(ctx.Childf("id"), m, values)
}

func (v *Verifier) defineProof(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, p := v.layout, &v.proof
	p.module = sys.NewSizedModule(ctx, moduleSize(l.nbProof), wiop.PaddingDirectionNone)
	for k := range p.elem {
		p.elem[k] = p.module.NewColumn(ctx.Childf("x%d", k), round)
	}
	p.id = precomputed(ctx.Childf("id"), p.module, rangeOf(0, l.nbProof))
	p.active = precomputed(ctx.Childf("active"), p.module, slices.Repeat([]int{1}, l.nbProof))
}

func (v *Verifier) defineTranscript(
	sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round, table CompressionTable,
) {
	l, t := v.layout, &v.transcript
	nb := len(l.rows)
	t.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
	for i := range t.state {
		t.state[i] = t.module.NewColumn(ctx.Childf("state%d", i), round)
		t.block[i] = t.module.NewColumn(ctx.Childf("block%d", i), round)
		t.next[i] = t.module.NewColumn(ctx.Childf("next%d", i), round)
	}
	for k := range t.a {
		t.a[k] = t.module.NewColumn(ctx.Childf("a%d", k), round)
		t.b[k] = t.module.NewColumn(ctx.Childf("b%d", k), round)
	}

	// the precomputed columns
	t.active = precomputed(ctx.Childf("active"), t.module, slices.Repeat([]int{1}, nb))
	t.first = precomputed(ctx.Childf("first"), t.module, []int{1})
	t.chain = precomputed(ctx.Childf("chain"), t.module, append([]int{0}, slices.Repeat([]int{1}, nb-1)...))
	idsA, idsB, challenges := make([]int, nb), make([]int, nb), make([]int, nb)
	for s := range t.shape {
		flags := make([]int, nb)
		for r, row := range l.rows {
			if row.shape == s {
				flags[r] = 1
			}
		}
		t.shape[s] = precomputed(ctx.Childf("shape%d", s), t.module, flags)
	}
	for r, row := range l.rows {
		idsA[r], idsB[r], challenges[r] = row.elems[0], row.elems[1], row.challenge
	}
	t.readA, t.idA = idColumns(ctx.Childf("a"), t.module, idsA)
	t.readB, t.idB = idColumns(ctx.Childf("b"), t.module, idsB)
	t.export, t.exportID = idColumns(ctx.Childf("export"), t.module, challenges)

	// the constraints, which hold on every row: the first state is zero, every other row starts from
	// the state that the row before it ends with, and the elements that are not read are zero.
	one := baseConstant(1)
	for i := range t.state {
		t.module.NewVanishingManual(ctx.Childf("first-state%d", i),
			wiop.Mul(t.first.View(), t.state[i].View()))
		t.module.NewVanishingManual(ctx.Childf("chain%d", i),
			wiop.Mul(t.chain.View(), wiop.Sub(t.state[i].View(), t.next[i].View().Shift(-1))))
	}
	for k := range t.a {
		t.module.NewVanishingManual(ctx.Childf("zero-a%d", k),
			wiop.Mul(wiop.Sub(one, t.readA.View()), t.a[k].View()))
		t.module.NewVanishingManual(ctx.Childf("zero-b%d", k),
			wiop.Mul(wiop.Sub(one, t.readB.View()), t.b[k].View()))
	}
	// every slot of the block is the coordinate of A or B, or zero, that the row's shape gives it
	for i := range t.block {
		var slot wiop.Expression
		for s := range t.shape {
			which, k := blockSlot(s, i)
			if which < 0 {
				continue
			}
			element := t.a
			if which == 1 {
				element = t.b
			}
			term := wiop.Mul(t.shape[s].View(), element[k].View())
			if slot == nil {
				slot = term
			} else {
				slot = wiop.Add(slot, term)
			}
		}
		t.module.NewVanishingManual(ctx.Childf("block%d", i), wiop.Sub(t.block[i].View(), slot))
	}

	// every compression is in the caller's table
	compression := make([]*wiop.ColumnView, 0, len(table))
	for _, cols := range [3][poseidon2.BlockLen]*wiop.Column{t.state, t.block, t.next} {
		for _, col := range cols {
			compression = append(compression, col.View())
		}
	}
	caller := make([]*wiop.ColumnView, 0, len(table))
	for _, col := range table {
		caller = append(caller, col.View())
	}
	sys.NewInclusion(ctx.Childf("compressions"),
		[]wiop.Table{wiop.NewFilteredTable(t.active.View(), compression...)},
		[]wiop.Table{wiop.NewTable(caller...)})
}

func (v *Verifier) defineGates(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, g := v.layout, &v.gate
	nb := len(l.gateRows)
	g.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
	for k := range g.o {
		g.a[k] = g.module.NewColumn(ctx.Childf("a%d", k), round)
		g.b[k] = g.module.NewColumn(ctx.Childf("b%d", k), round)
		g.o[k] = g.module.NewColumn(ctx.Childf("o%d", k), round)
	}

	g.active = precomputed(ctx.Childf("active"), g.module, slices.Repeat([]int{1}, nb))
	g.id = precomputed(ctx.Childf("id"), g.module, rangeOf(l.firstGate, nb))
	kMul, kA, kB := make([]field.Element, nb), make([]field.Element, nb), make([]field.Element, nb)
	kOut, kConst := make([]field.Element, nb), make([]field.Element, nb)
	idsA, idsB, checks := make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.gateRows {
		kMul[i], kA[i], kB[i], kOut[i], kConst[i] = row.kMul, row.kA, row.kB, row.kOut, row.kConst
		idsA[i], idsB[i], checks[i] = row.a, row.b, row.check
	}
	g.kMul = precomputedElements(ctx.Childf("k-mul"), g.module, kMul)
	g.kA = precomputedElements(ctx.Childf("k-a"), g.module, kA)
	g.kB = precomputedElements(ctx.Childf("k-b"), g.module, kB)
	g.kOut = precomputedElements(ctx.Childf("k-out"), g.module, kOut)
	g.kConst = precomputedElements(ctx.Childf("k-const"), g.module, kConst)
	g.readA, g.idA = idColumns(ctx.Childf("a"), g.module, idsA)
	g.readB, g.idB = idColumns(ctx.Childf("b"), g.module, idsB)
	g.check, g.checkID = idColumns(ctx.Childf("check"), g.module, checks)

	// the relation of the module, on every row
	a, b, o := columnViews(g.a), columnViews(g.b), columnViews(g.o)
	rhs := a.Mul(b).MulByBase(g.kMul.View()).
		Add(a.MulByBase(g.kA.View())).
		Add(b.MulByBase(g.kB.View())).
		Add(shifted(g.o).MulByBase(g.kOut.View()))
	rhs[0] = wiop.Add(rhs[0], g.kConst.View())
	for k, diff := range o.Equal(rhs) {
		g.module.NewVanishingManual(ctx.Childf("row%d", k), diff)
	}
}

// columnViews returns the E₆ element whose coordinates are the columns.
func columnViews(cols [6]*wiop.Column) E6 {
	var x E6
	for k, c := range cols {
		x[k] = c.View()
	}
	return x
}

// shifted returns the E₆ element of the previous row of the columns.
func shifted(cols [6]*wiop.Column) E6 {
	var x E6
	for k, c := range cols {
		x[k] = c.View().Shift(-1)
	}
	return x
}

// busElement is the tuple (id, x₀, …, x₅) of the rows of the module where the selector is 1.
func busElement(m *wiop.Module, selector, id *wiop.Column, x [6]*wiop.Column) wiop.Table {
	entries := make([]wiop.TableEntry, 0, 1+len(x))
	entries = append(entries, id.View())
	for _, c := range x {
		entries = append(entries, c.View())
	}
	return wiop.NewTableEntries(m, selector.View(), entries...)
}

func (p *proofModule) producers() []wiop.Table {
	return []wiop.Table{busElement(p.module, p.active, p.id, p.elem)}
}

func (p *proofModule) consumers() []wiop.Table { return nil }

func (t *transcriptModule) producers() []wiop.Table {
	return []wiop.Table{busElement(t.module, t.export, t.exportID, [6]*wiop.Column(t.next[:6]))}
}

func (t *transcriptModule) consumers() []wiop.Table {
	return []wiop.Table{
		busElement(t.module, t.readA, t.idA, t.a),
		busElement(t.module, t.readB, t.idB, t.b),
	}
}

func (g *gateModule) producers() []wiop.Table {
	return []wiop.Table{busElement(g.module, g.active, g.id, g.o)}
}

func (g *gateModule) consumers() []wiop.Table {
	return []wiop.Table{
		busElement(g.module, g.readA, g.idA, g.a),
		busElement(g.module, g.readB, g.idB, g.b),
		busElement(g.module, g.check, g.checkID, g.o),
	}
}

// defineBus declares the value bus: every read of a value is a tuple (id, x₀, …, x₅) that some
// producer exposes.
func (v *Verifier) defineBus(sys *wiop.System, ctx *wiop.ContextFrame) {
	var producers, consumers []wiop.Table
	for _, m := range []busModule{&v.proof, &v.transcript, &v.gate, &v.fold, &v.eq, &v.rounds, &v.inputs} {
		producers = append(producers, m.producers()...)
		consumers = append(consumers, m.consumers()...)
	}
	sys.NewInclusion(ctx, consumers, producers)
}

// Compressions returns the compressions that the verifier looks up for a seed and a flattened proof,
// as rows of a [CompressionTable]: the left input, the right input, then the output.
func (v *Verifier) Compressions(
	seed [6]field.Element, proof []field.Ext,
) ([][3 * poseidon2.BlockLen]field.Element, error) {
	w, err := v.layout.evaluate(seed, proof)
	if err != nil {
		return nil, err
	}
	rows := make([][3 * poseidon2.BlockLen]field.Element, len(w.rows))
	for i, c := range w.rows {
		copy(rows[i][:poseidon2.BlockLen], c.state[:])
		copy(rows[i][poseidon2.BlockLen:2*poseidon2.BlockLen], c.block[:])
		copy(rows[i][2*poseidon2.BlockLen:], c.next[:])
	}
	return rows, nil
}

// Assign assigns the witness columns of the verifier from a flattened proof, in the order of gnark's
// Proof.Flatten, and the seed and the claims that the cells of the inputs hold. It must be called by a
// prover action of the round that Define was given, in which the cells are known.
func (v *Verifier) Assign(run *wiop.Runtime, proof []field.Ext) error {
	seed, err := v.inputs.assign(run)
	if err != nil {
		return err
	}
	w, err := v.layout.evaluate(seed, proof)
	if err != nil {
		return err
	}
	values := v.layout.values(seed, proof, w)

	v.proof.assign(run, proof)
	v.transcript.assign(run, w)
	v.gate.assign(run, v.layout.evaluateGates(values))
	v.eq.assign(run, v.layout.evaluateEqs(values))
	v.fold.assign(run, v.layout.evaluateFolds(values))
	v.rounds.assign(run, v.layout.evaluateRounds(values))
	return nil
}

// assignColumn assigns the values to the first rows of the column, and zero to the others.
func assignColumn(run *wiop.Runtime, col *wiop.Column, values []field.Element) {
	vec := make([]field.Element, col.Module.Size())
	copy(vec, values)
	run.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vec)})
}

// assignExt assigns the coordinates of the elements of E₆ to the first rows of the six columns, and zero
// to the others.
func assignExt(run *wiop.Runtime, cols [6]*wiop.Column, x []field.Ext) {
	var coords [6][]field.Element
	for k := range coords {
		coords[k] = make([]field.Element, len(x))
	}
	for i, e := range x {
		for k, c := range coordinates(e) {
			coords[k][i] = c
		}
	}
	for k, col := range cols {
		assignColumn(run, col, coords[k])
	}
}

func (p *proofModule) assign(run *wiop.Runtime, proof []field.Ext) {
	assignExt(run, p.elem, proof)
}

func (t *transcriptModule) assign(run *wiop.Runtime, w *transcriptWitness) {
	nb := len(w.rows)
	a, b := make([]field.Ext, nb), make([]field.Ext, nb)
	for r, c := range w.rows {
		a[r], b[r] = c.a, c.b
	}
	assignExt(run, t.a, a)
	assignExt(run, t.b, b)
	for i := range poseidon2.BlockLen {
		state, block, next := make([]field.Element, nb), make([]field.Element, nb), make([]field.Element, nb)
		for r, c := range w.rows {
			state[r], block[r], next[r] = c.state[i], c.block[i], c.next[i]
		}
		assignColumn(run, t.state[i], state)
		assignColumn(run, t.block[i], block)
		assignColumn(run, t.next[i], next)
	}
}

func (g *gateModule) assign(run *wiop.Runtime, w *gateWitness) {
	assignExt(run, g.a, w.a)
	assignExt(run, g.b, w.b)
	assignExt(run, g.o, w.o)
}

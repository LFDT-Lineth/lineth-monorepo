package gkrverifier

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
)

// CompressionTable is a table of Poseidon2 compressions that the caller proves: the columns hold, in
// order, the eight elements of the left input, the eight of the right input, and the eight of the
// output of the compression C(left, right) = P(left ‖ right)[8..16] + right.
type CompressionTable [3 * blockLen]*wiop.Column

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
	round      *wiop.Round
	proof      proofModule
	transcript transcriptModule
	gate       gateModule
	fold       foldModule
	eq         eqModule
	rounds     roundModule
	inputs     inputsModule
}

// proofModule has one row per element of the flattened proof.
type proofModule struct {
	module  *wiop.Module
	size    int
	elem    [6]*wiop.Column // witness: the coordinates of the element
	id      *wiop.Column    // precomputed: the id of the element
	coordID [6]*wiop.Column // precomputed: the ids of its coordinates
	active  *wiop.Column    // precomputed: 1 on the rows of the proof
}

// transcriptModule has one row per compression of the transcript.
type transcriptModule struct {
	module *wiop.Module
	size   int
	state  [blockLen]*wiop.Column // witness
	block  [blockLen]*wiop.Column
	next   [blockLen]*wiop.Column
	// precomputed
	active   *wiop.Column
	first    *wiop.Column           // 1 on the first row
	chain    *wiop.Column           // 1 on every row but the first
	read     [blockLen]*wiop.Column // 1 where the slot is read from the bus, 0 where it is zero
	slotID   [blockLen]*wiop.Column // the id of the coordinate that the slot reads
	export   *wiop.Column           // 1 on the rows that end a challenge
	exportID *wiop.Column           // the id of the challenge
}

// gateModule has one row per step of the gates' bytecode: o = k_m·a·b + k_a·a + k_b·b + k_o·o⁻ + k_c.
type gateModule struct {
	module  *wiop.Module
	size    int
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
	for i, cell := range inputs.Seed {
		if cell == nil {
			return nil, fmt.Errorf("gkrverifier: seed cell %d is nil", i)
		}
	}
	l, err := newLayout(circuit, schedule, n)
	if err != nil {
		return nil, err
	}

	v := &Verifier{layout: l, round: round}
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
func precomputed(ctx *wiop.ContextFrame, m *wiop.Module, size int, values []int) *wiop.Column {
	elements := make([]field.Element, len(values))
	for i, x := range values {
		elements[i].SetUint64(uint64(x))
	}
	return precomputedElements(ctx, m, size, elements)
}

// precomputedElements is [precomputed] for values of the base field.
func precomputedElements(ctx *wiop.ContextFrame, m *wiop.Module, size int, values []field.Element) *wiop.Column {
	vec := make([]field.Element, size)
	copy(vec, values)
	return m.NewPrecomputedColumn(ctx, &wiop.ConcreteVector{Plain: field.VecFromBase(vec)})
}

// idColumns declares the precomputed columns of a selector and of ids for the value that some of the
// first nb rows of the module read: id gives the id that row i reads, or -1 if it reads none, in which
// case the selector is 0 and the id is 0.
func idColumns(
	ctx *wiop.ContextFrame, m *wiop.Module, size, nb int, id func(i int) int,
) (selector, ids *wiop.Column) {
	selected, values := make([]int, nb), make([]int, nb)
	for i := range values {
		if x := id(i); x >= 0 {
			selected[i], values[i] = 1, x
		}
	}
	return precomputed(ctx.Childf("read"), m, size, selected), precomputed(ctx.Childf("id"), m, size, values)
}

// ones returns count ones.
func ones(count int) []int {
	r := make([]int, count)
	for i := range r {
		r[i] = 1
	}
	return r
}

func (v *Verifier) defineProof(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, p := v.layout, &v.proof
	p.size = moduleSize(l.nbProof)
	p.module = sys.NewSizedModule(ctx, p.size, wiop.PaddingDirectionNone)
	for k := range p.elem {
		p.elem[k] = p.module.NewColumn(ctx.Childf("x%d", k), round)
	}
	p.id = precomputed(ctx.Childf("id"), p.module, p.size, rangeOf(0, l.nbProof))
	for k := range p.coordID {
		ids := make([]int, l.nbProof)
		for i := range ids {
			ids[i] = l.coordinateID(i, k)
		}
		p.coordID[k] = precomputed(ctx.Childf("coordinate-id%d", k), p.module, p.size, ids)
	}
	p.active = precomputed(ctx.Childf("active"), p.module, p.size, ones(l.nbProof))
}

func (v *Verifier) defineTranscript(
	sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round, table CompressionTable,
) {
	l, t := v.layout, &v.transcript
	t.size = moduleSize(len(l.rows))
	t.module = sys.NewSizedModule(ctx, t.size, wiop.PaddingDirectionNone)
	for i := range t.state {
		t.state[i] = t.module.NewColumn(ctx.Childf("state%d", i), round)
		t.block[i] = t.module.NewColumn(ctx.Childf("block%d", i), round)
		t.next[i] = t.module.NewColumn(ctx.Childf("next%d", i), round)
	}

	// the precomputed columns
	t.active = precomputed(ctx.Childf("active"), t.module, t.size, ones(len(l.rows)))
	t.first = precomputed(ctx.Childf("first"), t.module, t.size, ones(1))
	t.chain = precomputed(ctx.Childf("chain"), t.module, t.size, append([]int{0}, ones(len(l.rows)-1)...))
	for i := range t.read {
		read, id := make([]int, len(l.rows)), make([]int, len(l.rows))
		for r, row := range l.rows {
			if row.slots[i].read {
				read[r], id[r] = 1, row.slots[i].coord
			}
		}
		t.read[i] = precomputed(ctx.Childf("read%d", i), t.module, t.size, read)
		t.slotID[i] = precomputed(ctx.Childf("slot-id%d", i), t.module, t.size, id)
	}
	export, exportID := make([]int, len(l.rows)), make([]int, len(l.rows))
	for r, row := range l.rows {
		if row.challenge >= 0 {
			export[r], exportID[r] = 1, row.challenge
		}
	}
	t.export = precomputed(ctx.Childf("export"), t.module, t.size, export)
	t.exportID = precomputed(ctx.Childf("export-id"), t.module, t.size, exportID)

	// the constraints, which hold on every row: the first state is zero, every other row starts
	// from the state that the row before it ends with, and a block element that is not read is zero.
	one := wiop.NewConstantField(field.One())
	for i := range t.state {
		t.module.NewVanishingManual(ctx.Childf("first-state%d", i),
			wiop.Mul(t.first.View(), t.state[i].View()))
		t.module.NewVanishingManual(ctx.Childf("chain%d", i),
			wiop.Mul(t.chain.View(), wiop.Sub(t.state[i].View(), t.next[i].View().Shift(-1))))
		t.module.NewVanishingManual(ctx.Childf("zero-block%d", i),
			wiop.Mul(wiop.Sub(one, t.read[i].View()), t.block[i].View()))
	}

	// every compression is in the caller's table
	compression := make([]*wiop.ColumnView, 0, len(t.state)+len(t.block)+len(t.next))
	for i := range t.state {
		compression = append(compression, t.state[i].View())
	}
	for i := range t.block {
		compression = append(compression, t.block[i].View())
	}
	for i := range t.next {
		compression = append(compression, t.next[i].View())
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
	g.size = moduleSize(len(l.gateRows))
	g.module = sys.NewSizedModule(ctx, g.size, wiop.PaddingDirectionNone)
	for k := range g.o {
		g.a[k] = g.module.NewColumn(ctx.Childf("a%d", k), round)
		g.b[k] = g.module.NewColumn(ctx.Childf("b%d", k), round)
		g.o[k] = g.module.NewColumn(ctx.Childf("o%d", k), round)
	}

	nb := len(l.gateRows)
	g.active = precomputed(ctx.Childf("active"), g.module, g.size, ones(nb))
	g.id = precomputed(ctx.Childf("id"), g.module, g.size, rangeOf(l.firstGate, nb))
	var constants [5][]field.Element
	for k := range constants {
		constants[k] = make([]field.Element, nb)
	}
	for i, row := range l.gateRows {
		constants[0][i], constants[1][i], constants[2][i] = row.kMul, row.kA, row.kB
		constants[3][i], constants[4][i] = row.kOut, row.kConst
	}
	g.kMul = precomputedElements(ctx.Childf("k-mul"), g.module, g.size, constants[0])
	g.kA = precomputedElements(ctx.Childf("k-a"), g.module, g.size, constants[1])
	g.kB = precomputedElements(ctx.Childf("k-b"), g.module, g.size, constants[2])
	g.kOut = precomputedElements(ctx.Childf("k-out"), g.module, g.size, constants[3])
	g.kConst = precomputedElements(ctx.Childf("k-const"), g.module, g.size, constants[4])
	g.readA, g.idA = idColumns(ctx.Childf("a"), g.module, g.size, nb, func(i int) int { return l.gateRows[i].a })
	g.readB, g.idB = idColumns(ctx.Childf("b"), g.module, g.size, nb, func(i int) int { return l.gateRows[i].b })
	g.check, g.checkID = idColumns(ctx.Childf("check"), g.module, g.size, nb,
		func(i int) int { return l.gateRows[i].check })

	// every row, which holds on all of them, is the relation of its constants
	a, b, o := columnViews(g.a), columnViews(g.b), columnViews(g.o)
	var previous E6
	for k := range previous {
		previous[k] = g.o[k].View().Shift(-1)
	}
	rhs := a.Mul(b).MulByBase(g.kMul.View()).
		Add(a.MulByBase(g.kA.View())).
		Add(b.MulByBase(g.kB.View())).
		Add(previous.MulByBase(g.kOut.View()))
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

// busElement is the tuple (id, x₀, …, x₅) of the rows of the module where the selector is 1.
func busElement(m *wiop.Module, selector, id *wiop.Column, x [6]*wiop.Column) wiop.Table {
	entries := make([]wiop.TableEntry, 0, 1+len(x))
	entries = append(entries, id.View())
	for _, c := range x {
		entries = append(entries, c.View())
	}
	return wiop.NewTableEntries(m, selector.View(), entries...)
}

// busCoordinate is the tuple (id, x, 0, 0, 0, 0, 0) of the rows of the module where the selector is 1.
func busCoordinate(m *wiop.Module, selector, id, x *wiop.Column) wiop.Table {
	zero := wiop.NewConstantVector(m, field.Zero())
	return wiop.NewTableEntries(m, selector.View(), id.View(), x.View(), zero, zero, zero, zero, zero)
}

// defineBus declares the value bus: every read of a value is a tuple (id, x₀, …, x₅) that some
// producer exposes. A coordinate is read and exposed as (id, x, 0, 0, 0, 0, 0).
func (v *Verifier) defineBus(sys *wiop.System, ctx *wiop.ContextFrame) {
	p, t, g := &v.proof, &v.transcript, &v.gate

	producers := make([]wiop.Table, 0, 1+6+2)
	producers = append(producers,
		busElement(p.module, p.active, p.id, p.elem),
		busElement(g.module, g.active, g.id, g.o))
	for k := range 6 {
		producers = append(producers, busCoordinate(p.module, p.active, p.coordID[k], p.elem[k]))
	}
	challenge := make([]wiop.TableEntry, 0, 7)
	challenge = append(challenge, t.exportID.View())
	for k := range 6 {
		challenge = append(challenge, t.next[k].View())
	}
	producers = append(producers, wiop.NewTableEntries(t.module, t.export.View(), challenge...))

	consumers := make([]wiop.Table, 0, len(t.block)+3)
	for i := range t.block {
		consumers = append(consumers, busCoordinate(t.module, t.read[i], t.slotID[i], t.block[i]))
	}
	consumers = append(consumers,
		busElement(g.module, g.readA, g.idA, g.a),
		busElement(g.module, g.readB, g.idB, g.b),
		busElement(g.module, g.check, g.checkID, g.o))
	producers = append(producers, v.inputs.producers()...)
	producers = append(producers, v.fold.producers()...)
	producers = append(producers, v.eq.producers()...)
	consumers = append(consumers, v.inputs.consumers()...)
	consumers = append(consumers, v.fold.consumers()...)
	consumers = append(consumers, v.eq.consumers()...)
	consumers = append(consumers, v.rounds.consumers()...)
	sys.NewInclusion(ctx, consumers, producers)
}

// Compressions returns the compressions that the verifier looks up for a seed and a flattened proof,
// as rows of a [CompressionTable]: the left input, the right input, then the output.
func (v *Verifier) Compressions(seed [6]field.Element, proof []field.Ext) ([][3 * blockLen]field.Element, error) {
	w, err := v.layout.evaluate(seed, proof)
	if err != nil {
		return nil, err
	}
	rows := make([][3 * blockLen]field.Element, len(w.rows))
	for i, c := range w.rows {
		copy(rows[i][:blockLen], c.state[:])
		copy(rows[i][blockLen:2*blockLen], c.block[:])
		copy(rows[i][2*blockLen:], c.next[:])
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
func assignColumn(run *wiop.Runtime, col *wiop.Column, size int, values []field.Element) {
	vec := make([]field.Element, size)
	copy(vec, values)
	run.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vec)})
}

// assignExt assigns the coordinates of the elements of E₆ to the first rows of the six columns, and zero
// to the others.
func assignExt(run *wiop.Runtime, cols [6]*wiop.Column, size int, x []field.Ext) {
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
		assignColumn(run, col, size, coords[k])
	}
}

func (p *proofModule) assign(run *wiop.Runtime, proof []field.Ext) {
	assignExt(run, p.elem, p.size, proof)
}

func (t *transcriptModule) assign(run *wiop.Runtime, w *transcriptWitness) {
	for i := range blockLen {
		state, block, next := make([]field.Element, len(w.rows)), make([]field.Element, len(w.rows)),
			make([]field.Element, len(w.rows))
		for r, c := range w.rows {
			state[r], block[r], next[r] = c.state[i], c.block[i], c.next[i]
		}
		assignColumn(run, t.state[i], t.size, state)
		assignColumn(run, t.block[i], t.size, block)
		assignColumn(run, t.next[i], t.size, next)
	}
}

func (g *gateModule) assign(run *wiop.Runtime, w *gateWitness) {
	assignExt(run, g.a, g.size, w.a)
	assignExt(run, g.b, g.size, w.b)
	assignExt(run, g.o, g.size, w.o)
}

package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// cols holds the values of a chip's columns, by name: its setup columns (from
// the program) or its witness columns (from an execution). names lists the
// columns in declaration order, so that the setup columns are created in a
// deterministic order.
type cols struct {
	n     int
	vals  map[string][]field.Element
	names []string
}

func newCols(n int) *cols { return &cols{n: n, vals: map[string][]field.Element{}} }

func (c *cols) declare(names ...string) {
	for _, n := range names {
		c.get(n)
	}
}

func (c *cols) declareLanes(prefix string, n int) {
	for i := range n {
		c.declare(laneName(prefix, i))
	}
}

func (c *cols) get(name string) []field.Element {
	v, ok := c.vals[name]
	if !ok {
		v = make([]field.Element, c.n)
		c.vals[name] = v
		c.names = append(c.names, name)
	}
	return v
}

func (c *cols) set(name string, row int, v uint64) { c.get(name)[row].SetUint64(v) }

func (c *cols) setInt(name string, row int, v int64) { c.get(name)[row].SetInt64(v) }

func (c *cols) setElem(name string, row int, v field.Element) { c.get(name)[row] = v }

func (c *cols) setCell(prefix string, row int, v Cell) {
	for i := range CellWidth {
		c.setElem(laneName(prefix, i), row, v[i])
	}
}

// laneName returns the name of lane i of the columns prefix: prefix<i>.
func laneName(prefix string, i int) string { return fmt.Sprintf("%s%d", prefix, i) }

// module is a chip's wiop module while the chip defines its constraints: its
// setup columns (precomputed, from the program's setup values) and its
// witness columns, created in the witness round on first use (witnessNames
// lists them in creation order).
type module struct {
	name         string
	sys          *wiop.System
	mod          *wiop.Module
	round        *wiop.Round
	setup        map[string]*wiop.Column
	witness      map[string]*wiop.Column
	witnessNames []string
	nAssert      int
}

func newModule(sys *wiop.System, round *wiop.Round, name string, n int, setup *cols) *module {
	m := &module{
		name: name, sys: sys, round: round,
		mod:     sys.NewSizedModule(sys.Context.Childf("%s", name), n, wiop.PaddingDirectionNone),
		setup:   map[string]*wiop.Column{},
		witness: map[string]*wiop.Column{},
	}
	for _, colName := range setup.names {
		v := &wiop.ConcreteVector{Plain: field.VecFromBase(setup.vals[colName])}
		m.setup[colName] = m.mod.NewPrecomputedColumn(sys.Context.Childf("%s.%s", name, colName), v)
	}
	return m
}

// s returns a setup column, which the chip's setup declared.
func (m *module) s(name string) *wiop.ColumnView {
	c, ok := m.setup[name]
	if !ok {
		panic(fmt.Sprintf("module %s: no setup column %q", m.name, name))
	}
	return c.View()
}

// w returns a witness column, creating it on first use.
func (m *module) w(name string) *wiop.ColumnView {
	c, ok := m.witness[name]
	if !ok {
		c = m.mod.NewColumn(m.sys.Context.Childf("%s.%s", m.name, name), m.round)
		m.witness[name] = c
		m.witnessNames = append(m.witnessNames, name)
	}
	return c.View()
}

// wShift returns a witness column shifted by k (cyclically: row i reads row
// i + k mod N).
func (m *module) wShift(name string, k int) *wiop.ColumnView { return m.w(name).Shift(k) }

// lanes returns the witness columns prefix0..n−1.
func (m *module) lanes(prefix string, n int) []wiop.Expression {
	res := make([]wiop.Expression, n)
	for i := range res {
		res[i] = m.w(laneName(prefix, i))
	}
	return res
}

// setupLanes returns the setup columns prefix0..n−1.
func (m *module) setupLanes(prefix string, n int) []wiop.Expression {
	res := make([]wiop.Expression, n)
	for i := range res {
		res[i] = m.s(laneName(prefix, i))
	}
	return res
}

// assertZero constrains expr to vanish on every row. No row is cancelled:
// shifts are cyclic, and the chips gate their transitions with selectors
// themselves, so the first and last rows stay constrained (wiop.NewVanishing
// would cancel the rows a shift crosses).
func (m *module) assertZero(expr wiop.Expression) {
	m.mod.NewVanishingManual(m.sys.Context.Childf("%s.assert-%d", m.name, m.nAssert), expr)
	m.nAssert++
}

// constant returns the constant v, as a scalar expression.
func constant(v int64) wiop.Expression {
	var x field.Element
	x.SetInt64(v)
	return wiop.NewConstantField(x)
}

// cellOf pads lanes to a cell tuple with zero lanes.
func cellOf(lanes ...wiop.Expression) [CellWidth]wiop.Expression {
	var res [CellWidth]wiop.Expression
	for i := range res {
		if i < len(lanes) {
			res[i] = lanes[i]
		} else {
			res[i] = constant(0)
		}
	}
	return res
}

// cellLanes returns a cell tuple of the 8 given lanes.
func cellLanes(lanes []wiop.Expression) [CellWidth]wiop.Expression {
	if len(lanes) != CellWidth {
		panic(fmt.Sprintf("cellLanes: %d lanes", len(lanes)))
	}
	return [CellWidth]wiop.Expression(lanes)
}

// elemConst returns the constant v, as a scalar expression.
func elemConst(v field.Element) wiop.Expression { return wiop.NewConstantField(v) }

// sum returns the sum of exprs (at least one).
func sum(exprs ...wiop.Expression) wiop.Expression {
	res := exprs[0]
	for _, e := range exprs[1:] {
		res = wiop.Add(res, e)
	}
	return res
}

// prod returns the product of exprs (at least one).
func prod(exprs ...wiop.Expression) wiop.Expression {
	res := exprs[0]
	for _, e := range exprs[1:] {
		res = wiop.Mul(res, e)
	}
	return res
}

func (c *cols) setE6(prefix string, row int, x field.Ext) {
	cell := E6Cell(x)
	for i := range KindE6 {
		c.setElem(laneName(prefix, i), row, cell[i])
	}
}

package gkrverifier

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark/gkr"
)

// identityGate is the gate that level 0's view of the circuit gives to its wires.
func identityGate() gkr.SerializableGate {
	return gkr.SerializableGate{Evaluate: gkr.IdentityBytecode(), NbIn: 1, Degree: 1}
}

// A gate's bytecode is lowered to rows of the gate module, each of which is the relation
//
//	o = k_m·a·b + k_a·a + k_b·b + k_o·o⁻ + k_c
//
// of E₆ elements, with o⁻ the result of the previous row, a and b read from the value bus, and the
// k base field constants. Every row exposes its result under its own id.

// operand is an operand of an instruction: a base field constant c, or the value of the bus with the
// given id times the base field coefficient coef.
type operand struct {
	isConst bool
	c       field.Element
	id      int
	coef    field.Element
}

// value returns the operand of the value with the given id.
func value(id int) operand { return operand{id: id, coef: field.One()} }

// gateRow is a row of the gate module.
type gateRow struct {
	kMul, kA, kB, kOut, kConst field.Element
	// a and b are the ids of the values read as the operands a and b, or -1 if the row reads none.
	a, b int
	// check is the id of the value that the result must equal, or -1.
	check int
}

// gateBuilder lowers gate evaluations to rows. The row i has the id firstID + i.
type gateBuilder struct {
	firstID int
	rows    []gateRow
}

func (g *gateBuilder) emit(r gateRow) operand {
	g.rows = append(g.rows, r)
	return value(g.firstID + len(g.rows) - 1)
}

func newRow() gateRow { return gateRow{a: -1, b: -1, check: -1} }

// base returns the element of the base field that c reduces to.
func base(c *big.Int) field.Element {
	var e field.Element
	e.SetBigInt(c)
	return e
}

// signed returns x, or −x.
func signed(x field.Element, negative bool) field.Element {
	if negative {
		x.Neg(&x)
	}
	return x
}

// lowerGate lowers the evaluation of the gate on the values with the given ids, and returns the
// operand of its result.
func (g *gateBuilder) lowerGate(gate gkr.GateBytecode, inputs []int) operand {
	space := make([]operand, 0, len(gate.Constants)+len(inputs)+len(gate.Instructions))
	for _, c := range gate.Constants {
		space = append(space, operand{isConst: true, c: base(c)})
	}
	for _, id := range inputs {
		space = append(space, value(id))
	}
	for _, inst := range gate.Instructions {
		ops := make([]operand, len(inst.Inputs))
		for i, index := range inst.Inputs {
			ops[i] = space[index]
		}
		space = append(space, g.lowerInstruction(inst.Op, ops))
	}
	return space[len(space)-1]
}

func (g *gateBuilder) lowerInstruction(op gkr.GateOp, ops []operand) operand {
	switch op {
	case gkr.OpAdd:
		return g.sum(ops, nil)
	case gkr.OpSub:
		negative := make([]bool, len(ops))
		for i := 1; i < len(ops); i++ {
			negative[i] = true
		}
		return g.sum(ops, negative)
	case gkr.OpNeg:
		return g.sum(ops[:1], []bool{true})
	case gkr.OpMul:
		return g.product(ops)
	case gkr.OpMulAcc:
		return g.sum([]operand{g.product(ops[:2]), ops[2]}, nil)
	case gkr.OpSumExp17:
		s := g.sum(ops[:3], nil)
		x := s
		for range 4 {
			x = g.product([]operand{x, x})
		}
		return g.product([]operand{x, s})
	}
	panic("gkrverifier: unknown gate operation " + op.String())
}

// sum returns Σ ±ops[i], with the sign of ops[i] negative if negative[i], or all positive if negative
// is nil. The constants are summed into one k_c, and the values are chained: the first row takes two
// of them, each other row takes one and the previous row's result. The coefficient of a value is a
// factor of the k_a or k_b that reads it.
func (g *gateBuilder) sum(ops []operand, negative []bool) operand {
	var (
		constant field.Element
		values   []operand
		coefs    []field.Element
		negs     []bool
	)
	for i, o := range ops {
		neg := negative != nil && negative[i]
		if o.isConst {
			term := signed(o.c, neg)
			constant.Add(&constant, &term)
		} else {
			values = append(values, o)
			coefs = append(coefs, signed(o.coef, neg))
			negs = append(negs, neg)
		}
	}
	switch {
	case len(values) == 0:
		return operand{isConst: true, c: constant}
	case len(values) == 1 && !negs[0] && constant.IsZero():
		return values[0]
	}

	first := newRow()
	first.kA, first.a = coefs[0], values[0].id
	if len(values) >= 2 {
		first.kB, first.b = coefs[1], values[1].id
	}
	first.kConst = constant
	result := g.emit(first)
	for i := 2; i < len(values); i++ {
		row := newRow()
		row.kOut, row.kA, row.a = field.One(), coefs[i], values[i].id
		result = g.emit(row)
	}
	return result
}

// product returns Πops[i]. The constants and the coefficients of the values are multiplied into one
// factor. A single value is returned with that factor as its coefficient, without a row; several are
// chained: the first row takes two of them, with the factor as its k_m, and each other row takes one
// and the previous row's result.
func (g *gateBuilder) product(ops []operand) operand {
	factor := field.One()
	var values []operand
	for _, o := range ops {
		if o.isConst {
			factor.Mul(&factor, &o.c)
		} else {
			factor.Mul(&factor, &o.coef)
			values = append(values, o)
		}
	}
	switch len(values) {
	case 0:
		return operand{isConst: true, c: factor}
	case 1:
		return operand{id: values[0].id, coef: factor}
	}

	first := newRow()
	first.kMul, first.a, first.b = factor, values[0].id, values[1].id
	result := g.emit(first)
	for _, v := range values[2:] {
		row := newRow()
		row.kMul, row.a, row.b = field.One(), v.id, result.id
		result = g.emit(row)
	}
	return result
}

// evaluate lowers the evaluation of the gate on the values with the given ids, and returns the id of
// the value of its result: the id of an input if the gate is the identity, and that of a row otherwise.
// A result that is a constant or a scaled value gets a row of its own.
func (g *gateBuilder) evaluate(gate gkr.GateBytecode, inputs []int) int {
	result := g.lowerGate(gate, inputs)
	if result.isConst || !result.coef.IsOne() {
		return g.emit(copyRow(result)).id
	}
	return result.id
}

// copyRow is the row whose result is the operand.
func copyRow(o operand) gateRow {
	row := newRow()
	if o.isConst {
		row.kConst = o.c
	} else {
		row.kA, row.a = o.coef, o.id
	}
	return row
}

// evaluateChecked lowers the evaluation of the gate on the values with the given ids, ending on a row
// that requires its result to equal the value with the id claimed.
func (g *gateBuilder) evaluateChecked(gate gkr.GateBytecode, inputs []int, claimed int) {
	start := len(g.rows)
	result := g.lowerGate(gate, inputs)
	if result.isConst || !result.coef.IsOne() || len(g.rows) == start || result.id != g.firstID+len(g.rows)-1 {
		g.emit(copyRow(result))
	}
	g.rows[len(g.rows)-1].check = claimed
}

// evaluate returns the result of the row, with its operands a and b, from the values it reads.
func (r *gateRow) evaluate(a, b, previous field.Ext) field.Ext {
	var o, product, term field.Ext
	product.Mul(&a, &b)
	o.MulByElement(&product, &r.kMul)
	term.MulByElement(&a, &r.kA)
	o.Add(&o, &term)
	term.MulByElement(&b, &r.kB)
	o.Add(&o, &term)
	term.MulByElement(&previous, &r.kOut)
	o.Add(&o, &term)
	o.B0.A0.Add(&o.B0.A0, &r.kConst)
	return o
}

// lowerGates lowers the gate evaluations of every level, in the order of gnark's Verify: at a sumcheck
// or single-source level, each wire's gate on the level's final evaluations, whose result ids it
// records in the level; at a skip level, each wire's gate on the final evaluations at the point of
// each claim, checked against the claimed value.
func (l *layout) lowerGates() {
	g := &gateBuilder{firstID: l.firstGate}
	for levelI, level := range l.schedule {
		lv := &l.levels[levelI]
		levelCircuit := l.circuit.LevelCircuit(l.schedule, levelI, identityGate())
		_, inputIndices := levelCircuit.InputMapping(level)
		finals := func(levelWireI, claimI int) []int {
			ids := make([]int, len(inputIndices[levelWireI]))
			for i, uniqueI := range inputIndices[levelWireI] {
				ids[i] = lv.finals + level.FinalEvalProofIndex(uniqueI, claimI)
			}
			return ids
		}

		levelWireI := 0
		for _, group := range level.ClaimGroups() {
			for _, wireI := range group.Wires {
				gate := levelCircuit[wireI].Gate.Evaluate
				if !lv.skip {
					lv.results = append(lv.results, g.evaluate(gate, finals(levelWireI, 0)))
				} else {
					for claimI, source := range group.ClaimSources {
						claimed := l.claimPosition(source, l.claimValueIndices[wireI][claimI])
						g.evaluateChecked(gate, finals(levelWireI, claimI), claimed)
					}
				}
				levelWireI++
			}
		}
	}
	l.gateRows = g.rows
}

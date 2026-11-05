package gkrverifier

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark/gkr"
	"github.com/stretchr/testify/require"
)

// evaluateBytecode is a reference evaluation of a gate in E₆, the operations being those of gnark's
// evaluator.
func evaluateBytecode(gate gkr.GateBytecode, inputs []field.Ext) field.Ext {
	space := make([]field.Ext, 0, len(gate.Constants)+len(inputs)+len(gate.Instructions))
	for _, c := range gate.Constants {
		var e field.Ext
		e.B0.A0.SetBigInt(c)
		space = append(space, e)
	}
	space = append(space, inputs...)
	for _, inst := range gate.Instructions {
		x := make([]field.Ext, len(inst.Inputs))
		for i, index := range inst.Inputs {
			x[i] = space[index]
		}
		var r field.Ext
		switch inst.Op {
		case gkr.OpAdd:
			for _, e := range x {
				r.Add(&r, &e)
			}
		case gkr.OpSub:
			r = x[0]
			for _, e := range x[1:] {
				r.Sub(&r, &e)
			}
		case gkr.OpNeg:
			r.Neg(&x[0])
		case gkr.OpMul:
			r = x[0]
			for _, e := range x[1:] {
				r.Mul(&r, &e)
			}
		case gkr.OpMulAcc:
			r.Mul(&x[0], &x[1])
			r.Add(&r, &x[2])
		case gkr.OpSumExp17:
			var s field.Ext
			s.Add(&x[0], &x[1])
			s.Add(&s, &x[2])
			r.Mul(&s, &s)
			for range 3 {
				r.Mul(&r, &r)
			}
			r.Mul(&r, &s)
		}
		space = append(space, r)
	}
	return space[len(space)-1]
}

// TestLowerGate checks that the rows that a gate is lowered to compute its evaluation, for gates that
// use every operation, with constants as any operand, operands that repeat, and values scaled by a
// constant before they are summed, multiplied, or returned.
func TestLowerGate(t *testing.T) {
	const (
		c0, c1  = 0, 1    // the constants
		x, y, z = 2, 3, 4 // the inputs
		r0, r1  = 5, 6    // the results of the first instructions
	)
	constants := []*big.Int{big.NewInt(7), big.NewInt(-3)}
	gates := map[string][]gkr.GateInstruction{
		"identity":          nil,
		"add":               {{Op: gkr.OpAdd, Inputs: []uint16{x, y, z, c0}}},
		"add one":           {{Op: gkr.OpAdd, Inputs: []uint16{x}}},
		"add const":         {{Op: gkr.OpAdd, Inputs: []uint16{c0, c1}}},
		"sub":               {{Op: gkr.OpSub, Inputs: []uint16{c0, x, y, c1, z}}},
		"neg":               {{Op: gkr.OpNeg, Inputs: []uint16{x}}},
		"mul":               {{Op: gkr.OpMul, Inputs: []uint16{x, c0, y, x, c1}}},
		"mul one":           {{Op: gkr.OpMul, Inputs: []uint16{c1, x}}},
		"mulacc":            {{Op: gkr.OpMulAcc, Inputs: []uint16{x, y, z}}},
		"mulacc constant":   {{Op: gkr.OpMulAcc, Inputs: []uint16{c0, y, c1}}},
		"sumexp17":          {{Op: gkr.OpSumExp17, Inputs: []uint16{x, y, z}}},
		"sumexp17 constant": {{Op: gkr.OpSumExp17, Inputs: []uint16{c0, y, c1}}},
		"chain": {
			{Op: gkr.OpMul, Inputs: []uint16{x, y}},
			{Op: gkr.OpMul, Inputs: []uint16{z, z}},
			{Op: gkr.OpAdd, Inputs: []uint16{r0}},
		},
		"scaled sum": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpMul, Inputs: []uint16{c1, y}},
			{Op: gkr.OpAdd, Inputs: []uint16{r0, r1, z, c0}},
		},
		"scaled sub": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpSub, Inputs: []uint16{r0, y, r0}},
		},
		"scaled neg": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpNeg, Inputs: []uint16{r0}},
		},
		"scaled product": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpMul, Inputs: []uint16{c1, y}},
			{Op: gkr.OpMul, Inputs: []uint16{r0, r1}},
		},
		"scaled chain": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpMul, Inputs: []uint16{y, r0, z}},
		},
		"scaled square": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpMul, Inputs: []uint16{r0, r0}},
		},
		"scaled mulacc": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpMulAcc, Inputs: []uint16{r0, y, z}},
		},
		"scaled sumexp17": {
			{Op: gkr.OpMul, Inputs: []uint16{c0, x}},
			{Op: gkr.OpSumExp17, Inputs: []uint16{r0, y, z}},
		},
		"scaled result": {{Op: gkr.OpMul, Inputs: []uint16{c0, x}}},
		"scaled result of a sum": {
			{Op: gkr.OpAdd, Inputs: []uint16{x, y}},
			{Op: gkr.OpMul, Inputs: []uint16{c1, r0}},
		},
	}

	inputs := make([]field.Ext, 3)
	for i := range inputs {
		inputs[i].MustSetRandom()
	}
	for name, instructions := range gates {
		t.Run(name, func(t *testing.T) {
			gate := gkr.GateBytecode{Instructions: instructions, Constants: constants}
			const firstID = 100
			// run returns the value of an id, with the rows' results computed
			run := func(g *gateBuilder) func(id int) field.Ext {
				o := make([]field.Ext, len(g.rows))
				at := func(id int) field.Ext {
					if id < len(inputs) {
						return inputs[id]
					}
					return o[id-firstID]
				}
				for i, row := range g.rows {
					var a, b, previous field.Ext
					if row.a >= 0 {
						a = at(row.a)
					}
					if row.b >= 0 {
						b = at(row.b)
					}
					if i > 0 {
						previous = o[i-1]
					}
					o[i] = row.evaluate(a, b, previous)
				}
				return at
			}
			want := evaluateBytecode(gate, inputs)

			g := &gateBuilder{firstID: firstID}
			id := g.evaluate(gate, []int{0, 1, 2})
			require.Equal(t, want, run(g)(id))

			const claimed = 7
			g = &gateBuilder{firstID: firstID}
			g.evaluateChecked(gate, []int{0, 1, 2}, claimed)
			last := g.rows[len(g.rows)-1]
			require.Equal(t, claimed, last.check)
			require.Equal(t, want, run(g)(firstID+len(g.rows)-1))
		})
	}
}

// skipChecks returns the claimed positions of the gate rows that check one, by row.
func (l *layout) skipChecks() []int {
	var checks []int
	for _, row := range l.gateRows {
		if row.check >= 0 {
			checks = append(checks, row.check)
		}
	}
	return checks
}

// TestGates checks the gate rows on gnark's proofs of the Poseidon2 compression circuit: the system
// proves and verifies, with every claim of a skip level checked; and changing a final evaluation of a
// skip level, or the claimed value that a skip level checks, is rejected. The compressions of the
// changed proof are put in the table, so that the gates are what rejects it.
func TestGates(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)
			l, err := newLayout(p.circuit, p.schedule, n)
			require.NoError(t, err)

			nbSkipClaims := 0
			var skipFinals []int
			for _, lv := range l.levels {
				if lv.skip {
					nbSkipClaims += lv.nbClaims
					skipFinals = append(skipFinals, lv.finals)
				}
			}
			checks := l.skipChecks()
			require.NotEmpty(t, checks)
			require.Len(t, checks, nbSkipClaims)

			t.Run("honest", func(t *testing.T) {
				require.NoError(t, newTranscriptSystem(t, p).prove(t))
			})

			tamper := func(t *testing.T, position int) {
				t.Helper()
				s := newTranscriptSystem(t, p)
				one := field.OneExt()
				s.flat = append([]field.Ext(nil), p.flat...)
				s.flat[position].Add(&s.flat[position], &one)
				s.rows, err = s.verifier.Compressions(s.seedValue, s.flat)
				require.NoError(t, err)
				require.Error(t, s.prove(t))
			}
			for _, i := range []int{0, len(skipFinals) / 2, len(skipFinals) - 1} {
				t.Run(fmt.Sprintf("changed final evaluation of skip level %d", i), func(t *testing.T) { tamper(t, skipFinals[i]) })
			}
			for _, i := range []int{0, len(checks) / 2, len(checks) - 1} {
				t.Run(fmt.Sprintf("changed claimed value %d", i), func(t *testing.T) { tamper(t, checks[i]) })
			}
		})
	}
}

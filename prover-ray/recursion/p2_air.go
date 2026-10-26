package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// The AIR of a Poseidon2 permutation, one permutation per row, laid out as
// Plonky3's poseidon2-air with S-box degree 3 and no S-box registers. Its
// witness columns are
//
//	in0..15                 the inputs
//	post<r>_0..15           the state after full round r, every full round
//	                        but the last (whose state is the outputs)
//	sbox<r>                 the S-box output of partial round r
//	out0..15                the outputs
//
// i.e. 16 + 5·16 + 21 + 16 = 133 columns, and every constraint has degree 3.
// Between committed columns the state is tracked as linear forms, so the
// linear layers cost no columns.

// Witness columns of the Poseidon2 AIR.
const (
	p2In   = "in"  // lanes 0..15
	p2Out  = "out" // lanes 0..15
	p2Post = "post"
	p2SBox = "sbox"
)

func p2PostName(r, i int) string { return p2PostNames[r][i] }
func p2SBoxName(r int) string    { return p2SBoxNames[r] }

// The AIR's column names, formatted once, and their order: p2Cols lists them
// (in0..15, the full rounds' posts, the partial S-boxes, out0..15), and the
// *Idx tables give a column's position in it, which p2Trace fills.
var (
	p2PostNames [P2FullRounds + P2PartialRounds][P2Width]string
	p2SBoxNames [P2FullRounds + P2PartialRounds]string
	p2Cols      []string
	p2InIdx     [P2Width]int
	p2OutIdx    [P2Width]int
	p2PostIdx   [P2FullRounds + P2PartialRounds][P2Width]int
	p2SBoxIdx   [P2FullRounds + P2PartialRounds]int
)

func init() {
	add := func(name string) int {
		p2Cols = append(p2Cols, name)
		return len(p2Cols) - 1
	}
	for i := range P2Width {
		p2InIdx[i] = add(laneName(p2In, i))
	}
	last := P2FullRounds + P2PartialRounds - 1
	for r := range last {
		if isFullRound(r) {
			for i := range P2Width {
				p2PostNames[r][i] = fmt.Sprintf("%s%d_%d", p2Post, r, i)
				p2PostIdx[r][i] = add(p2PostNames[r][i])
			}
		} else {
			p2SBoxNames[r] = fmt.Sprintf("%s%d", p2SBox, r)
			p2SBoxIdx[r] = add(p2SBoxNames[r])
		}
	}
	for i := range P2Width {
		p2OutIdx[i] = add(laneName(p2Out, i))
	}
}

// linForm is a linear form over witness columns: Σ coef·column.
type linForm struct {
	names []string
	coefs []field.Element
}

func linCol(name string) linForm {
	return linForm{names: []string{name}, coefs: []field.Element{field.One()}}
}

// combine returns Σ_j row[j]·xs[j].
func combine(row [P2Width]field.Element, xs []linForm) linForm {
	acc := map[string]field.Element{}
	var order []string
	for j, x := range xs {
		if row[j].IsZero() {
			continue
		}
		for k, n := range x.names {
			var t field.Element
			t.Mul(&row[j], &x.coefs[k])
			v, ok := acc[n]
			if !ok {
				order = append(order, n)
			}
			v.Add(&v, &t)
			acc[n] = v
		}
	}
	var res linForm
	for _, n := range order {
		if v := acc[n]; !v.IsZero() {
			res.names = append(res.names, n)
			res.coefs = append(res.coefs, v)
		}
	}
	return res
}

func (l linForm) expr(m *module) wiop.Expression {
	terms := make([]wiop.Expression, len(l.names))
	for i, n := range l.names {
		terms[i] = wiop.Mul(elemConst(l.coefs[i]), m.w(n))
	}
	if len(terms) == 0 {
		return constant(0)
	}
	return sum(terms...)
}

// cube returns (l + k)³.
func cube(m *module, l linForm, k field.Element) wiop.Expression {
	x := wiop.Add(l.expr(m), elemConst(k))
	return prod(x, x, x)
}

// defineP2 constrains, on every row of m, the columns out to be the
// permutation of the columns in.
func defineP2(m *module) {
	state := make([]linForm, P2Width)
	in := make([]linForm, P2Width)
	for i := range in {
		in[i] = linCol(laneName(p2In, i))
	}
	for i := range state {
		state[i] = combine(p2ME[i], in)
	}
	last := P2FullRounds + P2PartialRounds - 1
	for r := 0; r <= last; r++ {
		rk := p2Params.RoundKeys[r]
		if isFullRound(r) {
			// next_i = Σ_j M_E[i][j]·(state_j + rk_j)³
			next := make([]string, P2Width)
			for i := range next {
				next[i] = p2PostName(r, i)
				if r == last {
					next[i] = laneName(p2Out, i)
				}
			}
			cubes := make([]wiop.Expression, P2Width)
			for j := range cubes {
				cubes[j] = cube(m, state[j], rk[j])
			}
			for i := range P2Width {
				var terms []wiop.Expression
				for j := range P2Width {
					if !p2ME[i][j].IsZero() {
						terms = append(terms, wiop.Mul(elemConst(p2ME[i][j]), cubes[j]))
					}
				}
				m.assertZero(wiop.Sub(m.w(next[i]), sum(terms...)))
			}
			for i := range state {
				state[i] = linCol(next[i])
			}
			continue
		}
		// sbox_r = (state_0 + rk_0)³, then state = M_I · state
		m.assertZero(wiop.Sub(m.w(p2SBoxName(r)), cube(m, state[0], rk[0])))
		state[0] = linCol(p2SBoxName(r))
		next := make([]linForm, P2Width)
		for i := range next {
			next[i] = combine(p2MI[i], state)
		}
		state = next
	}
}

// p2Trace writes into res (of length len(p2Cols)) the AIR's columns for the
// permutation of in, in p2Cols order.
func p2Trace(in [P2Width]field.Element, res []field.Element) {
	for i := range in {
		res[p2InIdx[i]] = in[i]
	}
	state := in
	matExternal(state[:])
	last := P2FullRounds + P2PartialRounds - 1
	for r := 0; r <= last; r++ {
		rk := p2Params.RoundKeys[r]
		if isFullRound(r) {
			for j := range state {
				state[j].Add(&state[j], &rk[j])
				t := state[j]
				state[j].Square(&state[j]).Mul(&state[j], &t)
			}
			matExternal(state[:])
			for i := range state {
				if r == last {
					res[p2OutIdx[i]] = state[i]
				} else {
					res[p2PostIdx[r][i]] = state[i]
				}
			}
			continue
		}
		state[0].Add(&state[0], &rk[0])
		t := state[0]
		state[0].Square(&state[0]).Mul(&state[0], &t)
		res[p2SBoxIdx[r]] = state[0]
		matInternal(state[:])
	}
}

package gkrverifier

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	"github.com/stretchr/testify/require"
)

// newClaimCells returns the inputs that hold the claims of gnark, as cells of the round, and their
// values. The claims that share a point share its cells.
func newClaimCells(sys *wiop.System, round *wiop.Round, claims gkrkoalabear.Claims) (map[int][]Claim, map[*wiop.Cell]field.Element) {
	inputs := make(map[int][]Claim)
	values := make(map[*wiop.Cell]field.Element)
	cells := func(name string, e field.Ext) (c [6]*wiop.Cell) {
		coordinates := coordinates(e)
		for k := range c {
			c[k] = round.NewCell(sys.Context.Childf("%s-%d", name, k), false)
			values[c[k]] = coordinates[k]
		}
		return c
	}
	points := make(map[*field.Ext]*Point)
	for wireI, list := range claims {
		for i, claim := range list {
			point, ok := points[&claim.EvaluationPoint[0]]
			if !ok {
				point = new(Point)
				for j, coordinate := range claim.EvaluationPoint {
					*point = append(*point, cells(fmt.Sprintf("point%d-%d-%d", wireI, i, j), coordinate))
				}
				points[&claim.EvaluationPoint[0]] = point
			}
			inputs[wireI] = append(inputs[wireI], Claim{Point: point, Value: cells(fmt.Sprintf("value%d-%d", wireI, i), claim.Evaluation)})
		}
	}
	return inputs, values
}

// TestClaimsLayout checks the claims of the layout against those that gnark's prover returns: for each
// wire, the claims in the same order, at the same point, with the same value.
func TestClaimsLayout(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)
			l, err := newLayout(p.circuit, p.schedule, n)
			require.NoError(t, err)
			w, err := l.evaluate(p.seed, p.flat)
			require.NoError(t, err)
			values := l.values(p.seed, p.flat, w)

			count := make(map[int]int)
			for _, claim := range l.claims {
				want := p.claims[claim.wire][claim.index]
				require.Equal(t, want.Evaluation, values[claim.value], "wire %d claim %d", claim.wire, claim.index)
				require.Len(t, claim.point, len(want.EvaluationPoint))
				for j, id := range claim.point {
					require.Equal(t, want.EvaluationPoint[j], values[id], "wire %d claim %d coordinate %d", claim.wire, claim.index, j)
				}
				count[claim.wire]++
			}
			for wireI, list := range p.claims {
				require.Len(t, list, count[wireI], "wire %d", wireI)
			}
		})
	}
}

// TestSumchecks checks the whole verifier on gnark's proofs of the Poseidon2 compression circuit: the
// system proves and verifies; and changing a round coefficient, a final evaluation of a sumcheck level
// or of level 0, or a point or value of the returned claims, is rejected. The compressions of the
// changed proof are put in the table, so that it is the sumchecks, the gates and the claims that reject
// it.
func TestSumchecks(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)
			l, err := newLayout(p.circuit, p.schedule, n)
			require.NoError(t, err)

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
			for levelI, lv := range l.levels {
				if lv.skip || (levelI > 3 && levelI != 6 && levelI != len(l.levels)-1) {
					continue
				}
				t.Run(fmt.Sprintf("level %d changed first coefficient", levelI), func(t *testing.T) { tamper(t, lv.start) })
				t.Run(fmt.Sprintf("level %d changed last coefficient", levelI), func(t *testing.T) { tamper(t, lv.finals-1) })
				t.Run(fmt.Sprintf("level %d changed final evaluation", levelI), func(t *testing.T) { tamper(t, lv.finals) })
			}

			// a claim's cell
			for _, claim := range []struct {
				name  string
				point bool
			}{{"point", true}, {"value", false}} {
				t.Run("changed claim "+claim.name, func(t *testing.T) {
					s := newTranscriptSystem(t, p)
					one := field.One()
					var changed bool
					for _, list := range s.claims {
						cell := list[0].Value[0]
						if claim.point {
							cell = (*list[0].Point)[0][0]
						}
						value := s.claimValues[cell]
						value.Add(&value, &one)
						s.claimValues[cell] = value
						changed = true
						break
					}
					require.True(t, changed)
					require.Error(t, s.prove(t))
				})
			}
		})
	}
}

// TestCorruptedWitness checks that the constraints of the modules, not only the lookups, reject: with
// an honest proof, one value of a witness column that is not read from the bus is changed after the
// assignment.
func TestCorruptedWitness(t *testing.T) {
	p := newGnarkProof(t, 2)
	for _, target := range []struct {
		name   string
		column func(v *Verifier) *wiop.Column
		row    int
	}{
		{"gate result", func(v *Verifier) *wiop.Column { return v.gate.o[0] }, 100},
		{"fold sum", func(v *Verifier) *wiop.Column { return v.fold.sum[0] }, 0},
		{"fold final", func(v *Verifier) *wiop.Column { return v.fold.final[3] }, 0},
		{"eq factor", func(v *Verifier) *wiop.Column { return v.eq.factor[0] }, 0},
		{"eq product", func(v *Verifier) *wiop.Column { return v.eq.product[5] }, 0},
		{"round c0", func(v *Verifier) *wiop.Column { return v.rounds.c0[1] }, 0},
		{"round h", func(v *Verifier) *wiop.Column { return v.rounds.h[0][2] }, 0},
		{"round out", func(v *Verifier) *wiop.Column { return v.rounds.out[0] }, 0},
		{"round a", func(v *Verifier) *wiop.Column { return v.rounds.a[4] }, 1},
		{"round mu", func(v *Verifier) *wiop.Column { return v.rounds.mu[0] }, 0},
		{"claim", func(v *Verifier) *wiop.Column { return v.claims.x[0] }, 0},
		{"transcript block", func(v *Verifier) *wiop.Column { return v.transcript.block[0] }, 3},
	} {
		t.Run(target.name, func(t *testing.T) {
			s := newTranscriptSystem(t, p)
			col := target.column(s.verifier)
			s.verifier.corrupt = func(c *wiop.Column, values []field.Element) {
				if c == col {
					one := field.One()
					values[target.row].Add(&values[target.row], &one)
				}
			}
			s.anyPanic = true
			require.Error(t, s.prove(t))
		})
	}
}

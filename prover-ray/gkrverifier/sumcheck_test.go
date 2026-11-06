package gkrverifier

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	"github.com/stretchr/testify/require"
)

// newClaimCells returns the inputs that hold the claims of gnark, as cells of the round, and their
// values. The claims that share a point share its cells.
func newClaimCells(
	sys *wiop.System, round *wiop.Round, claims gkrkoalabear.Claims,
) (map[int][]Claim, map[*wiop.Cell]field.Element) {
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
			value := cells(fmt.Sprintf("value%d-%d", wireI, i), claim.Evaluation)
			inputs[wireI] = append(inputs[wireI], Claim{Point: point, Value: value})
		}
	}
	return inputs, values
}

// TestSumchecks checks the whole verifier on gnark's proof of the Poseidon2 compression circuit: changing a
// round coefficient or a final evaluation of a sumcheck level and of a single-source level is rejected, as
// is changing a cell of a returned claim. The compressions of the changed proof are put in the table, so
// that the compressions are not what rejects it; a changed cell is no proof element, and is rejected by the
// claim's row of the inputs.
func TestSumchecks(t *testing.T) {
	p := newGnarkProof(t, 2)
	l, err := newLayout(p.circuit, p.schedule, 2)
	require.NoError(t, err)

	// the first level of each type that has rounds
	levels := map[string]int{"sumcheck": -1, "single-source": -1}
	for levelI, lv := range l.levels {
		switch {
		case lv.skip:
		case lv.single:
			if levels["single-source"] < 0 {
				levels["single-source"] = levelI
			}
		default:
			if levels["sumcheck"] < 0 {
				levels["sumcheck"] = levelI
			}
		}
	}
	for typ, levelI := range levels {
		require.GreaterOrEqual(t, levelI, 0, typ)
		lv := l.levels[levelI]
		for name, position := range map[string]int{
			"first coefficient": lv.start, "last coefficient": lv.finals - 1, "final evaluation": lv.finals,
		} {
			t.Run(fmt.Sprintf("%s level, changed %s", typ, name), func(t *testing.T) {
				s := newTranscriptSystem(t, p)
				s.changeProofElement(t, position)
				err := s.prove(t)
				require.Error(t, err)
				require.False(t, missesCompression(err))
			})
		}
	}

	// the first claim of the wire with the smallest id
	wire := slices.Min(slices.Collect(maps.Keys(p.claims)))
	for name, cell := range map[string]func(c Claim) *wiop.Cell{
		"point": func(c Claim) *wiop.Cell { return (*c.Point)[0][0] },
		"value": func(c Claim) *wiop.Cell { return c.Value[0] },
	} {
		t.Run("changed claim "+name, func(t *testing.T) {
			s := newTranscriptSystem(t, p)
			c := cell(s.claims[wire][0])
			value, one := s.claimValues[c], field.One()
			value.Add(&value, &one)
			s.claimValues[c] = value
			require.ErrorIs(t, s.prove(t), errMissingRow)
		})
	}
}

// override adds one to the value of the column at row, after the verifier's assignment.
func (s *transcriptSystem) override(col *wiop.Column, row int) {
	s.round.RegisterAction(actionFunc(func(rt *wiop.Runtime) {
		values := slices.Clone(rt.GetColumnAssignment(col).Plain.AsBase())
		one := field.One()
		values[row].Add(&values[row], &one)
		rt.OverrideColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
	}))
}

// overrideCell adds one to the value of the cell, which has the given value, after the verifier's
// assignment.
func (s *transcriptSystem) overrideCell(cell *wiop.Cell, value field.Element) {
	s.round.RegisterAction(actionFunc(func(rt *wiop.Runtime) {
		one := field.One()
		value.Add(&value, &one)
		rt.OverrideCell(cell, field.ElemFromBase(value))
	}))
}

// overrideCompression adds one to a column of the transcript at row, which is the entry tuple of the
// compression's row of the caller's table, and adds the changed compression to the table, so that the
// lookup of the compressions still holds.
func (s *transcriptSystem) overrideCompression(col *wiop.Column, tuple, row int) {
	changed, one := s.rows[row], field.One()
	changed[tuple].Add(&changed[tuple], &one)
	s.rows = append(slices.Clone(s.rows), changed)
	s.override(col, row)
}

// rowWhere returns the first row below n where f holds.
func rowWhere(t *testing.T, n int, f func(row int) bool) int {
	t.Helper()
	for row := range n {
		if f(row) {
			return row
		}
	}
	t.Fatal("no row")
	return -1
}

// TestCorruptedWitness checks that the constraints of the modules reject, not only the lookups: with an
// honest proof, one value is changed after the assignment, in a row that no lookup reads, or in a cell that
// the constraints bind to a column. Each case isolates a family of constraints; the rows are located from the
// layout.
func TestCorruptedWitness(t *testing.T) {
	p := newGnarkProof(t, 2)
	for _, c := range []struct {
		name    string
		corrupt func(t *testing.T, s *transcriptSystem, l *layout)
	}{
		{"gate relation", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			// a gate row whose result neither another row, nor a fold, nor a check reads
			read := map[int]bool{}
			for _, row := range l.gateRows {
				read[row.a], read[row.b] = true, true
			}
			for _, lv := range l.levels {
				for _, id := range lv.results {
					read[id] = true
				}
			}
			row := rowWhere(t, len(l.gateRows), func(i int) bool { return !read[l.firstGate+i] && l.gateRows[i].check < 0 })
			s.override(s.verifier.gate.o[0], row)
		}},
		{"fold sum", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.fold.sum[0], rowWhere(t, len(l.foldRows), func(i int) bool { return !l.foldRows[i].last }))
		}},
		{"fold final", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.fold.final[3], rowWhere(t, len(l.foldRows), func(i int) bool { return !l.foldRows[i].last }))
		}},
		{"fold e is one", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.fold.e[0], rowWhere(t, len(l.foldRows), func(i int) bool { return l.foldRows[i].eq < 0 }))
		}},
		{"fold c is zero", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.fold.c[0], rowWhere(t, len(l.foldRows), func(i int) bool { return l.foldRows[i].fold < 0 }))
		}},
		{"eq factors", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.override(s.verifier.eq.h[0], 0)
		}},
		{"eq product of the factors", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.override(s.verifier.eq.g[1], 0)
		}},
		{"eq unread slot", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.eq.p[eqSlots-1][0], rowWhere(t, len(l.eqRows), func(i int) bool { return l.eqRows[i].p[eqSlots-1] < 0 }))
		}},
		{"round c0", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.override(s.verifier.rounds.c0[1], 0)
		}},
		{"round h", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.override(s.verifier.rounds.h[0][2], 0)
		}},
		{"round out", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.rounds.out[0], rowWhere(t, len(l.roundRows), func(i int) bool { return !l.roundRows[i].last }))
		}},
		{"round a", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.rounds.a[4], rowWhere(t, len(l.roundRows), func(i int) bool { return !l.roundRows[i].first }))
		}},
		{"round mu is one", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.rounds.mu[0], rowWhere(t, len(l.roundRows), func(i int) bool { return l.roundRows[i].mu < 0 }))
		}},
		{"round c is zero", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			row := rowWhere(t, len(l.roundRows), func(i int) bool { return len(l.roundRows[i].coeffs) < l.maxDegree })
			s.override(s.verifier.rounds.c[len(l.roundRows[row].coeffs)][0], row)
		}},
		{"transcript first state", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.overrideCompression(s.verifier.transcript.state[0], 0, 0)
		}},
		{"transcript chain", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.overrideCompression(s.verifier.transcript.state[0], 0, 1)
		}},
		{"transcript block", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.overrideCompression(s.verifier.transcript.block[0], poseidon2.BlockLen, 0)
		}},
		{"transcript zero a", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.transcript.a[0], rowWhere(t, len(l.rows), func(i int) bool { return l.rows[i].elems[0] < 0 }))
		}},
		{"transcript zero b", func(t *testing.T, s *transcriptSystem, l *layout) {
			t.Helper()
			s.override(s.verifier.transcript.b[0], rowWhere(t, len(l.rows), func(i int) bool { return l.rows[i].elems[1] < 0 }))
		}},
		{"seed cell", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			s.overrideCell(s.seed[0], s.seedValue[0])
		}},
		{"point cell", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			cell := (*s.claims[slices.Min(slices.Collect(maps.Keys(s.claims)))][0].Point)[0][0]
			s.overrideCell(cell, s.claimValues[cell])
		}},
		{"value cell", func(_ *testing.T, s *transcriptSystem, _ *layout) {
			cell := s.claims[slices.Min(slices.Collect(maps.Keys(s.claims)))][0].Value[0]
			s.overrideCell(cell, s.claimValues[cell])
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newTranscriptSystem(t, p)
			c.corrupt(t, s, s.verifier.layout)
			err := s.prove(t)
			require.Error(t, err)
			require.NotErrorIs(t, err, errMissingRow)
		})
	}
}

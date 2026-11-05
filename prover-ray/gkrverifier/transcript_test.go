package gkrverifier

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	gkrposeidon2 "github.com/consensys/gnark/gkr/permutation/gkr-poseidon2/koalabear"
	"github.com/stretchr/testify/require"
)

// gnarkProof is a proof of gnark's GKR prover for the Poseidon2 compression circuit on 2ⁿ random
// instances, with its seed.
type gnarkProof struct {
	compressor *gkrposeidon2.Compressor
	n          int
	seed       [6]field.Element
	// flat is the flattened proof, and claims the claims that the prover returns.
	flat   []field.Ext
	claims gkrkoalabear.Claims
}

func newGnarkProof(t *testing.T, n int) *gnarkProof {
	t.Helper()
	rng := rand.New(rand.NewPCG(uint64(n), 99))
	random := func() (e field.Element) {
		e.SetUint64(uint64(rng.Uint32()))
		return
	}

	compressor, err := gkrposeidon2.NewCompressor(poseidon2.GetDefaultParameters())
	require.NoError(t, err)

	p := &gnarkProof{compressor: compressor, n: n}
	for k := range p.seed {
		p.seed[k] = random()
	}
	columns := func() [][]field.Element {
		res := make([][]field.Element, len(compressor.Left))
		for i := range res {
			res[i] = make([]field.Element, 1<<n)
			for j := range res[i] {
				res[i][j] = random()
			}
		}
		return res
	}
	hasher := poseidon2.NewHasher()
	hasher.WriteElements(p.seed[:]...)
	proof, claims, err := gkrkoalabear.Prove(compressor.Circuit, compressor.Schedule, compressor.Assignment(columns(), columns()), hasher)
	require.NoError(t, err)
	for _, e := range proof.Flatten() {
		p.flat = append(p.flat, *e)
	}
	p.claims = claims
	return p
}

// TestTranscriptLayout checks the layout against gnark's prover: the challenges that the layout
// gives for the seed and the flattened proof are those of gnark's transcript. The point of the
// returned claims is made of the challenges of level 0's rounds, so it depends on every
// element absorbed before.
func TestTranscriptLayout(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)
			l, err := newLayout(p.compressor.Circuit, p.compressor.Schedule, n)
			require.NoError(t, err)
			require.Equal(t, len(p.flat), l.nbProof)

			w, err := l.evaluate(p.seed, p.flat)
			require.NoError(t, err)

			point := p.claims[int(p.compressor.Left[0])][0].EvaluationPoint
			require.Len(t, point, n)
			for j, id := range l.levels[0].roundChallenges {
				require.Equal(t, point[j], w.challenges[id-l.firstChallenge], "coordinate %d of the point", j)
			}
		})
	}
}

// transcriptSystem is a system with a seed, a table of compressions and the verifier, together with
// the values that its prover assigns, which a test changes.
type transcriptSystem struct {
	sys       *wiop.System
	verifier  *Verifier
	seed      [6]*wiop.Cell
	table     CompressionTable
	tableSize int
	// the values to assign
	seedValue [6]field.Element
	flat      []field.Ext
	rows      [][3 * blockLen]field.Element
}

// newTranscriptSystem defines the verifier of the proof's circuit with a table of exactly the
// compressions that it looks up, padded with the first of them. The seed, the proof and the rows are
// the proof's, for the test to change before proving.
func newTranscriptSystem(t *testing.T, p *gnarkProof) *transcriptSystem {
	t.Helper()
	s := &transcriptSystem{sys: wiop.NewSystemf("gkrverifier-transcript"), seedValue: p.seed, flat: p.flat}
	round := s.sys.NewRound()

	nbRows, err := NbCompressions(p.compressor.Circuit, p.compressor.Schedule, p.n)
	require.NoError(t, err)
	s.tableSize = utils.NextPowerOfTwo(nbRows)
	tableModule := s.sys.NewSizedModule(s.sys.Context.Childf("compressions"), s.tableSize, wiop.PaddingDirectionNone)
	for i := range s.table {
		s.table[i] = tableModule.NewColumn(s.sys.Context.Childf("compression%d", i), round)
	}
	for k := range s.seed {
		s.seed[k] = round.NewCell(s.sys.Context.Childf("seed%d", k), false)
	}

	v, err := Define(s.sys, round, p.compressor.Circuit, p.compressor.Schedule, p.n, Inputs{Seed: s.seed}, s.table)
	require.NoError(t, err)
	s.verifier = v
	s.rows, err = v.Compressions(p.seed, p.flat)
	require.NoError(t, err)
	require.Len(t, s.rows, nbRows)

	round.RegisterAction(actionFunc(func(rt *wiop.Runtime) {
		require.NoError(t, v.Assign(rt, s.flat))
	}))
	return s
}

// actionFunc is a prover action.
type actionFunc func(rt *wiop.Runtime)

func (f actionFunc) Run(rt *wiop.Runtime) { f(rt) }

// prove compiles the system and proves it with the system's values, and returns the result of the
// verification. A prover that holds a compression missing from the table fails while it builds its
// lookup, which is reported as an error too.
func (s *transcriptSystem) prove(t *testing.T) (err error) {
	t.Helper()
	require.NoError(t, compilers.CompileFull(s.sys))
	defer func() {
		if r := recover(); r != nil {
			require.Contains(t, fmt.Sprint(r), "has no match in the lookup table")
			err = fmt.Errorf("the prover has a row that is not in the table: %v", r)
		}
	}()
	proof, pub := s.sys.Prove(func(rt *wiop.Runtime) {
		for k, cell := range s.seed {
			rt.AssignCell(cell, field.ElemFromBase(s.seedValue[k]))
		}
		for i, col := range s.table {
			values := make([]field.Element, s.tableSize)
			for r := range values {
				if r < len(s.rows) {
					values[r] = s.rows[r][i]
				} else {
					values[r] = s.rows[0][i]
				}
			}
			rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
		}
	})
	return s.sys.Verify(proof, pub)
}

// TestTranscript checks the proof, seed and transcript tables, on gnark's proofs of the Poseidon2
// compression circuit: with a table that has exactly the compressions of the verifier, the system proves
// and verifies; and changing an element of the proof, a cell of the seed, or a row of the table is
// rejected, as each leaves a compression of the transcript missing from the table.
func TestTranscript(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)

			t.Run("honest", func(t *testing.T) {
				require.NoError(t, newTranscriptSystem(t, p).prove(t))
			})

			one := field.OneExt()
			for _, position := range []int{0, len(p.flat) / 2, len(p.flat) - 1} {
				t.Run(fmt.Sprintf("changed proof element %d", position), func(t *testing.T) {
					s := newTranscriptSystem(t, p)
					s.flat = append([]field.Ext(nil), p.flat...)
					s.flat[position].Add(&s.flat[position], &one)
					require.Error(t, s.prove(t))
				})
			}

			for k := range 6 {
				t.Run(fmt.Sprintf("changed seed %d", k), func(t *testing.T) {
					s := newTranscriptSystem(t, p)
					s.seedValue[k].Add(&s.seedValue[k], &one.B0.A0)
					require.Error(t, s.prove(t))
				})
			}

			for _, column := range []int{0, 9, 17} {
				t.Run(fmt.Sprintf("changed table column %d", column), func(t *testing.T) {
					s := newTranscriptSystem(t, p)
					s.rows = append([][3 * blockLen]field.Element(nil), s.rows...)
					s.rows[0][column].Add(&s.rows[0][column], &one.B0.A0)
					require.Error(t, s.prove(t))
				})
			}
		})
	}
}

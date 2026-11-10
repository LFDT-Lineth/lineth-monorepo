package gkrverifier

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	gkrposeidon2 "github.com/consensys/gnark/gkr/permutation/gkr-poseidon2/koalabear"
	"github.com/stretchr/testify/require"
)

// gnarkProof is a proof of gnark's GKR prover for a circuit on 2ⁿ random instances, with its seed. The
// circuit is the Poseidon2 compression's, unless built by [newCircuitProof].
type gnarkProof struct {
	compressor *gkrposeidon2.Compressor
	circuit    gkr.SerializableCircuit
	schedule   constraint.GkrProvingSchedule
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
	p.circuit, p.schedule = compressor.Circuit, compressor.Schedule
	p.prove(t, compressor.Assignment(columns(), columns()))
	return p
}

// prove proves the assignment with gnark's prover, seeded with the proof's seed, and sets the
// flattened proof and the claims.
func (p *gnarkProof) prove(t *testing.T, assignment gkrkoalabear.WireAssignment) {
	t.Helper()
	hasher := poseidon2.NewHasher()
	hasher.WriteElements(p.seed[:]...)
	proof, claims, err := gkrkoalabear.Prove(p.circuit, p.schedule, assignment, hasher)
	require.NoError(t, err)
	require.NoError(t, claims.Check(assignment))
	for _, e := range proof.Flatten() {
		p.flat = append(p.flat, *e)
	}
	p.claims = claims
}

// checkClaims checks the claims of the layout against those that gnark's prover returns: for each wire,
// the claims in the same order, at the same point, with the same value. values holds the value of every
// id for the proof and its seed.
func checkClaims(t *testing.T, p *gnarkProof, l *layout, values []field.Ext) {
	t.Helper()
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
}

// TestLayout checks the layout against gnark's prover on the Poseidon2 compression circuit: the challenges
// that the layout gives for the seed and the flattened proof are those of gnark's transcript, as the point
// of the returned claims is made of the challenges of level 0's rounds, so it depends on every element
// absorbed before; and the claims of the layout are those that gnark returns.
func TestLayout(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)
			l, err := newLayout(p.circuit, p.schedule, n)
			require.NoError(t, err)
			require.Len(t, p.flat, l.nbProof)

			w, err := l.evaluate(p.seed, p.flat)
			require.NoError(t, err)

			point := p.claims[int(p.compressor.Left[0])][0].EvaluationPoint
			require.Len(t, point, n)
			for j, id := range l.levels[0].roundChallenges {
				require.Equal(t, point[j], w.challenges[id-l.firstChallenge], "coordinate %d of the point", j)
			}
			checkClaims(t, p, l, l.values(p.seed, p.flat, w))
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
	// claimValues holds the values of the cells of the returned claims.
	claimValues map[*wiop.Cell]field.Element
	claims      map[int][]Claim
	round       *wiop.Round
	flat        []field.Ext
	rows        [][3 * poseidon2.BlockLen]field.Element
}

// newTranscriptSystem defines the verifier of the proof's circuit with a table of exactly the
// compressions that it looks up, padded with the first of them. The seed, the proof and the rows are
// the proof's, for the test to change before proving.
func newTranscriptSystem(t *testing.T, p *gnarkProof) *transcriptSystem {
	t.Helper()
	s := &transcriptSystem{sys: wiop.NewSystemf("gkrverifier-transcript"), seedValue: p.seed, flat: p.flat}
	round := s.sys.NewRound()
	s.round = round

	nbRows, err := NbCompressions(p.circuit, p.schedule, p.n)
	require.NoError(t, err)
	s.tableSize = utils.NextPowerOfTwo(nbRows)
	tableModule := s.sys.NewSizedModule(s.sys.Context.Childf("compressions"), s.tableSize, wiop.PaddingDirectionNone)
	for i := range s.table {
		s.table[i] = tableModule.NewColumn(s.sys.Context.Childf("compression%d", i), round)
	}
	s.seed = wioptest.NewE6Cells(s.sys.Context, round, "seed")

	s.claims, s.claimValues = newClaimCells(s.sys, round, p.claims)
	v, err := Define(s.sys, round, p.circuit, p.schedule, p.n, Inputs{Seed: s.seed, Claims: s.claims}, s.table)
	require.NoError(t, err)
	s.verifier = v
	s.rows, err = v.Compressions(p.seed, p.flat)
	require.NoError(t, err)
	require.Len(t, s.rows, nbRows)

	round.RegisterAction(wioptest.ActionFunc(func(rt *wiop.Runtime) {
		require.NoError(t, v.Assign(rt, s.flat))
	}))
	return s
}

// changeProofElement adds one to the element of the proof at position, and rebuilds the table's rows from
// the changed proof, so that no compression is missing from the table.
func (s *transcriptSystem) changeProofElement(t *testing.T, position int) {
	t.Helper()
	one := field.OneExt()
	s.flat = append([]field.Ext(nil), s.flat...)
	s.flat[position].Add(&s.flat[position], &one)
	var err error
	s.rows, err = s.verifier.Compressions(s.seedValue, s.flat)
	require.NoError(t, err)
}

// missesCompression says that err is a row of the verifier's compressions that the caller's table lacks.
func missesCompression(err error) bool {
	return errors.Is(err, wioptest.ErrMissingRow) && strings.Contains(err.Error(), "/transcript/state0")
}

// prove compiles the system and proves it with the system's values, and returns the result of the
// verification, or [wioptest.ErrMissingRow] if the prover holds a row missing from the table of a lookup.
func (s *transcriptSystem) prove(t *testing.T) (err error) {
	t.Helper()
	require.NoError(t, compilers.CompileFull(s.sys))
	defer wioptest.RecoverMissingRow(t, &err)
	proof, pub := s.sys.Prove(func(rt *wiop.Runtime) {
		for k, cell := range s.seed {
			rt.AssignCell(cell, field.ElemFromBase(s.seedValue[k]))
		}
		for cell, value := range s.claimValues {
			rt.AssignCell(cell, field.ElemFromBase(value))
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

// The coordinate of the seed and the column of the table that TestTranscript changes.
const (
	changedSeed   = 3
	changedColumn = 9
)

// TestTranscript checks the proof, transcript and compression table, on gnark's proofs of the Poseidon2
// compression circuit: with a table that has exactly the compressions of the verifier, the system proves
// and verifies; and, for n = 2, changing an element of the proof, a coordinate of the seed, or a column of
// the table leaves a compression of the transcript missing from the table.
func TestTranscript(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			p := newGnarkProof(t, n)

			t.Run("honest", func(t *testing.T) {
				require.NoError(t, newTranscriptSystem(t, p).prove(t))
			})
			if n != 2 {
				return
			}

			one := field.OneExt()
			t.Run("changed proof element", func(t *testing.T) {
				s := newTranscriptSystem(t, p)
				position := len(p.flat) / 2
				s.flat = append([]field.Ext(nil), p.flat...)
				s.flat[position].Add(&s.flat[position], &one)
				require.True(t, missesCompression(s.prove(t)))
			})

			t.Run("changed seed", func(t *testing.T) {
				s := newTranscriptSystem(t, p)
				s.seedValue[changedSeed].Add(&s.seedValue[changedSeed], &one.B0.A0)
				require.True(t, missesCompression(s.prove(t)))
			})

			t.Run("changed table column", func(t *testing.T) {
				s := newTranscriptSystem(t, p)
				s.rows = append([][3 * poseidon2.BlockLen]field.Element(nil), s.rows...)
				s.rows[0][changedColumn].Add(&s.rows[0][changedColumn], &one.B0.A0)
				require.True(t, missesCompression(s.prove(t)))
			})
		})
	}
}

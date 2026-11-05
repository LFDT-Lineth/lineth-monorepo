package gkrverifier

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/gkr"
	"github.com/consensys/gnark/gkr/gkrapi"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// newCircuitProof proves a circuit of gates of every operation, of several depths and with outputs at
// two of them, which reads its three inputs at several levels.
func newCircuitProof(t *testing.T, n int, mode gkrapi.ConsolidationMode) *gnarkProof {
	t.Helper()
	api := gkrapi.New()
	x0, x1, x2 := api.NewInput(), api.NewInput(), api.NewInput()
	sum := api.Gate(func(a gkr.GateAPI, in ...frontend.Variable) frontend.Variable {
		return a.Add(in[0], in[1], in[2], 5)
	}, x0, x1, x2)
	diff := api.Gate(func(a gkr.GateAPI, in ...frontend.Variable) frontend.Variable {
		return a.Neg(a.Sub(in[0], in[1], 7))
	}, x0, x1)
	acc := api.Gate(func(a gkr.GateAPI, in ...frontend.Variable) frontend.Variable {
		return a.MulAcc(a.Mul(in[2], 3), in[0], in[1])
	}, sum, diff, x2)
	power := api.Gate(func(a gkr.GateAPI, in ...frontend.Variable) frontend.Variable {
		return a.SumExp17(in[0], in[1], 5)
	}, sum, x1)
	product := api.Gate(func(a gkr.GateAPI, in ...frontend.Variable) frontend.Variable {
		return a.Mul(in[0], in[1], in[2], 3)
	}, acc, power, x0)
	api.Export(acc, product)

	circuit, schedule, err := api.Compile(gkr.KoalaBearE6(), mode)
	require.NoError(t, err)

	rng := rand.New(rand.NewPCG(uint64(n), 7))
	random := func() (e field.Element) {
		e.SetUint64(uint64(rng.Uint32()))
		return
	}
	p := &gnarkProof{circuit: circuit, schedule: schedule, n: n}
	for k := range p.seed {
		p.seed[k] = random()
	}
	assignment := make(gkrkoalabear.WireAssignment, len(circuit))
	for _, x := range []gkr.Variable{x0, x1, x2} {
		assignment[x] = make([]koalabear.Element, 1<<n)
		for i := range assignment[x] {
			assignment[x][i] = random()
		}
	}
	p.prove(t, assignment)
	return p
}

// TestCircuits checks the verifier on a circuit of every operation, under each consolidation mode: the
// layout returns gnark's claims, and the system accepts; and changing a round coefficient or the final
// evaluations of each level, or a claim, is rejected.
func TestCircuits(t *testing.T) {
	modes := map[string]gkrapi.ConsolidationMode{
		"all":                gkrapi.ConsolidateAll,
		"multi-claim inputs": gkrapi.ConsolidateMultiClaimInputsOnly,
		"none":               gkrapi.ConsolidateNone,
	}
	for name, mode := range modes {
		for _, n := range []int{1, 2, 3} {
			t.Run(fmt.Sprintf("%s/n=%d", name, n), func(t *testing.T) {
				p := newCircuitProof(t, n, mode)
				l, err := newLayout(p.circuit, p.schedule, n)
				require.NoError(t, err)
				var kinds []string
				for _, level := range p.schedule {
					switch level.(type) {
					case *constraint.GkrSkipLevel:
						kinds = append(kinds, "skip")
					case *constraint.GkrSingleSourceZeroCheckLevel:
						kinds = append(kinds, "single")
					default:
						kinds = append(kinds, "sumcheck")
					}
				}
				t.Log(kinds)

				w, err := l.evaluate(p.seed, p.flat)
				require.NoError(t, err)
				values := l.values(p.seed, p.flat, w)
				count := make(map[int]int)
				for _, claim := range l.claims {
					want := p.claims[claim.wire][claim.index]
					require.Equal(t, want.Evaluation, values[claim.value])
					for j, id := range claim.point {
						require.Equal(t, want.EvaluationPoint[j], values[id])
					}
					count[claim.wire]++
				}
				for wireI, list := range p.claims {
					require.Len(t, list, count[wireI])
				}

				require.NoError(t, newTranscriptSystem(t, p).prove(t))

				for levelI, lv := range l.levels {
					var positions []int
					if lv.nbFinals > 0 {
						positions = []int{lv.finals, lv.finals + lv.nbFinals - 1}
					}
					if !lv.skip {
						positions = append(positions, lv.start, lv.finals-1)
					}
					for _, position := range positions {
						s := newTranscriptSystem(t, p)
						one := field.OneExt()
						s.flat = append([]field.Ext(nil), p.flat...)
						s.flat[position].Add(&s.flat[position], &one)
						s.rows, err = s.verifier.Compressions(s.seedValue, s.flat)
						require.NoError(t, err)
						require.Error(t, s.prove(t), "level %d position %d", levelI, position)
					}
				}
			})
		}
	}
}

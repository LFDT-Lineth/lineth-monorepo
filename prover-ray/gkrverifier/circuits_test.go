package gkrverifier

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/gkr"
	"github.com/consensys/gnark/gkr/gkrapi"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
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
// layout returns gnark's claims, and the system proves and verifies.
func TestCircuits(t *testing.T) {
	for name, mode := range map[string]gkrapi.ConsolidationMode{
		"all":                gkrapi.ConsolidateAll,
		"multi-claim inputs": gkrapi.ConsolidateMultiClaimInputsOnly,
		"none":               gkrapi.ConsolidateNone,
	} {
		t.Run(name, func(t *testing.T) {
			p := newCircuitProof(t, 2, mode)
			l, err := newLayout(p.circuit, p.schedule, 2)
			require.NoError(t, err)
			w, err := l.evaluate(p.seed, p.flat)
			require.NoError(t, err)
			checkClaims(t, p, l, l.values(p.seed, p.flat, w))

			require.NoError(t, newTranscriptSystem(t, p).prove(t))
		})
	}
}

// TestDefineInputs checks the errors of Define on the claims of its inputs: a point shared by claims of
// different points, and a wire with more or fewer claims than the verifier returns.
func TestDefineInputs(t *testing.T) {
	p := newCircuitProof(t, 2, gkrapi.ConsolidateNone)
	l, err := newLayout(p.circuit, p.schedule, 2)
	require.NoError(t, err)

	// two claims of different points
	var first, other claimLayout
	for _, claim := range l.claims[1:] {
		if !slices.Equal(claim.point, l.claims[0].point) {
			first, other = l.claims[0], claim
			break
		}
	}
	require.NotEmpty(t, other.point, "the circuit has a single point")

	define := func(mutate func(claims map[int][]Claim)) error {
		sys := wiop.NewSystemf("define-inputs")
		round := sys.NewRound()
		var table CompressionTable
		module := sys.NewSizedModule(sys.Context.Childf("compressions"), 2, wiop.PaddingDirectionNone)
		for i := range table {
			table[i] = module.NewColumn(sys.Context.Childf("compression%d", i), round)
		}
		var seed [6]*wiop.Cell
		for k := range seed {
			seed[k] = round.NewCell(sys.Context.Childf("seed%d", k), false)
		}
		claims, _ := newClaimCells(sys, round, p.claims)
		mutate(claims)
		_, err := Define(sys, round, p.circuit, p.schedule, p.n, Inputs{Seed: seed, Claims: claims}, table)
		return err
	}

	require.ErrorContains(t, define(func(claims map[int][]Claim) {
		claims[other.wire][other.index].Point = claims[first.wire][first.index].Point
	}), "shared with a claim of another point")
	require.ErrorContains(t, define(func(claims map[int][]Claim) {
		claims[first.wire] = claims[first.wire][1:]
	}), "claims, the verifier returns")
	require.ErrorContains(t, define(func(claims map[int][]Claim) {
		claims[first.wire] = append(claims[first.wire], claims[first.wire][0])
	}), "claims, the verifier returns")
}

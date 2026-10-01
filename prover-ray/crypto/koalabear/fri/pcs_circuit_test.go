package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// This file checks VerifyGnark against PCS.Verify on the same opening proof.
// It complements tree_circuit_test.go, which covers the capping primitives in
// isolation: here the whole capped verifier runs, input trees included, so the
// revealed-table reconstruction and the per-tree frontier selection are
// exercised the way the recursion actually uses them.
//
// The query count matters. Cap depth is min(log2(numQueries), height-1), so a
// one- or two-query fixture gives a depth-0 or depth-1 cap whose frontier
// selection can be satisfied by coincidence. These fixtures use four queries,
// putting the input-tree frontier at depth 2 and making the selector bits
// observable.

const pcsCircuitNumQueries = 4

// pcsVerifyCircuit runs the in-circuit PCS verifier over a witness-carried
// opening proof.
type pcsVerifyCircuit struct {
	Proof  GnarkOpeningProof
	Roots  []circuit.Octuplet
	Claims []GnarkBatchClaimedValues
	Zeta   circuit.Ext
	Alphas []circuit.Ext
	Pos    []frontend.Variable

	pcs    *PCS
	shapes []Shape
	shifts []BatchShifts
}

// Define wires the capped verifier into constraints: the exported fields come
// in as witness variables, the unexported ones are native values baked in at
// compile time. VerifyGnark asserts instead of returning a verdict, so a bad
// proof surfaces as an unsatisfiable constraint when the solver runs.
func (c *pcsVerifyCircuit) Define(api frontend.API) error {
	c.pcs.VerifyGnark(api, GnarkVerifyInputs{
		Roots:          c.Roots,
		Shapes:         c.shapes,
		Shifts:         c.shifts,
		ClaimedValues:  c.Claims,
		Zeta:           c.Zeta,
		FoldAlphas:     c.Alphas,
		QueryPositions: c.Pos,
	}, c.Proof)
	return nil
}

// newPCSCircuitFixture builds two multi-size witness batches of different
// heights, opens them at four query positions, and checks the native verifier
// accepts before any circuit runs. Two batches give two input trees, so the
// circuit must match each opening to its own cap.
func newPCSCircuitFixture(t *testing.T) pcsOpenVerifyFixture {
	t.Helper()

	// prepare the PCS setup
	params, err := NewParams(4, 3, pcsCircuitNumQueries)
	require.NoError(t, err)
	pcs, err := NewPCS(params, makeEncoders(int(params.numRounds()+1), 2))
	require.NoError(t, err)

	// build the witness
	// witness[3] has two rows at the top size so one can be opened at shift 1 below: at shift 0
	// the rotation is omega^0 = 1 and a dropped or wrong rotation would go unseen.
	// different sizes to showcase the multi-size batching
	prng := rand.New(utils.NewRandSource(20260924))
	witness := make(Batch, 4)
	witness[1] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 2)}}
	witness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	witness[3] = SizedTable{Ext: [][]field.Ext{
		field.VecPseudoRandExt(prng, 8),
		field.VecPseudoRandExt(prng, 8),
	}}
	// a second, shorter batch commits to its own input tree, so the proof
	// carries two input caps
	otherWitness := make(Batch, 3)
	otherWitness[1] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 2)}}
	otherWitness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	witnesses := []Batch{witness, otherWitness}
	committed := []CommitterState{pcs.Commit(witness), pcs.Commit(otherWitness)}

	// prepare the shifts at what positions we open the rows
	batchShifts := make(BatchShifts, 4)
	batchShifts[1] = SizedShifts{Ext: [][]int{{0}}}
	batchShifts[2] = SizedShifts{Ext: [][]int{{0}}}
	batchShifts[3] = SizedShifts{Ext: [][]int{{0}, {1}}}
	otherBatchShifts := make(BatchShifts, 3)
	otherBatchShifts[1] = SizedShifts{Ext: [][]int{{0}}}
	otherBatchShifts[2] = SizedShifts{Ext: [][]int{{1}}}
	shifts := []BatchShifts{batchShifts, otherBatchShifts}

	// prepare a fixed zeta by writing down all coordinates
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	// prepare the FRI folding alphas
	// and hardcode the query positions to avoid a dependency on the PRNG in the test
	challenges := Challenges{
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0),
			field.UintsToExt(31, 0, 1, 0, 0, 0),
			field.UintsToExt(37, 0, 0, 1, 0, 0),
		},
		QueryPositions: []int{3, 9, 12, 6},
	}
	// compute the opening proof and the claimed values for the verifier
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zeta:       zeta,
		Challenges: challenges,
	})

	// prepare the verify fixture struct
	fx := pcsOpenVerifyFixture{
		pcs:       pcs,
		committed: committed,
		input: VerifyInputs{
			Roots:         utils.Map(func(c CommitterState) field.Octuplet { return c.Tree.Root() }, committed),
			Shapes:        utils.Map(Batch.Shape, witnesses),
			Shifts:        shifts,
			ClaimedValues: claimed,
			Zeta:          zeta,
			Challenges:    challenges,
		},
		proof: proof,
	}
	// check verification, first the native verifier, then the gnark circuit verifier, to avoid a false positive if the fixture is invalid
	require.NoError(t, fx.pcs.Verify(fx.input, fx.proof), "native verifier must accept the fixture")
	return fx
}

// circuitFor builds the template and the assignment for a fixture.
func circuitFor(fx pcsOpenVerifyFixture, proof OpeningProof) (template, assignment *pcsVerifyCircuit) {
	build := func(p OpeningProof, withValues bool) *pcsVerifyCircuit {
		convert := AllocateGnarkOpeningProof
		if withValues {
			convert = NewGnarkOpeningProof
		}
		c := &pcsVerifyCircuit{
			Proof:  convert(p),
			Roots:  make([]circuit.Octuplet, len(fx.input.Roots)),
			Claims: make([]GnarkBatchClaimedValues, len(fx.input.ClaimedValues)),
			Alphas: make([]circuit.Ext, len(fx.input.Challenges.FoldAlphas)),
			Pos:    make([]frontend.Variable, fx.pcs.Params.NumQueries),
			pcs:    fx.pcs,
			shapes: fx.input.Shapes,
			shifts: fx.input.Shifts,
		}
		for i, root := range fx.input.Roots {
			c.Roots[i] = convertOctuplet(root, withValues)
		}
		for i, batch := range fx.input.ClaimedValues {
			c.Claims[i] = convertBatchClaims(batch, withValues)
		}
		for i, a := range fx.input.Challenges.FoldAlphas {
			c.Alphas[i] = convertExt(a, withValues)
		}
		c.Zeta = convertExt(fx.input.Zeta, withValues)
		if withValues {
			for i := range c.Pos {
				c.Pos[i] = fx.input.Challenges.QueryPositions[i]
			}
		}
		return c
	}
	return build(fx.proof, false), build(proof, true)
}

func convertBatchClaims(batch BatchClaimedValues, withValues bool) GnarkBatchClaimedValues {
	res := make(GnarkBatchClaimedValues, len(batch))
	for sizeLog2, sized := range batch {
		res[sizeLog2] = GnarkSizedClaimedValues{
			Base: convertClaimRows(sized.Base, withValues),
			Ext:  convertClaimRows(sized.Ext, withValues),
		}
	}
	return res
}

func convertClaimRows(rows [][]field.Ext, withValues bool) [][]circuit.Ext {
	if rows == nil {
		return nil
	}
	res := make([][]circuit.Ext, len(rows))
	for i, row := range rows {
		res[i] = make([]circuit.Ext, len(row))
		for j, v := range row {
			res[i][j] = convertExt(v, withValues)
		}
	}
	return res
}

// solvePCSCircuit compiles the verifier circuit and reports whether the
// assignment satisfies it.
func solvePCSCircuit(t *testing.T, template, assignment *pcsVerifyCircuit) error {
	t.Helper()
	modulus := koalabear.Modulus()
	ccs, err := frontend.CompileU32(modulus, scs.NewBuilder, template)
	require.NoError(t, err, "verifier circuit must compile")
	w, err := frontend.NewWitness(assignment, modulus)
	require.NoError(t, err, "witness must build")
	_, err = ccs.Solve(w)
	return err
}

// TestPCSVerifyGnarkMatchesNative is the honest path: the circuit must accept
// exactly the proof the native verifier accepts.
func TestPCSVerifyGnarkMatchesNative(t *testing.T) {
	fx := newPCSCircuitFixture(t)

	// The fixture must actually exercise capping, or this test would pass
	// against a verifier that ignores caps entirely.
	require.GreaterOrEqual(t, len(fx.proof.InputCaps), 2,
		"fixture must carry several input caps, so openings are matched to their own tree")
	require.NotEmpty(t, fx.proof.InputCaps[0].Nodes, "input cap must be non-trivial")
	require.GreaterOrEqual(t, len(fx.proof.InputCaps[0].Nodes), 4,
		"input cap must be at least depth 2, so the frontier selector uses more than one bit")

	template, assignment := circuitFor(fx, fx.proof)
	require.NoError(t, solvePCSCircuit(t, template, assignment),
		"honest proof must satisfy the verifier circuit")
}

// TestPCSVerifyGnarkRejectsTamperedInputCap is the soundness direction for the
// input-tree cap: corrupting a frontier node must break the circuit, exactly as
// it breaks the native verifier.
func TestPCSVerifyGnarkRejectsTamperedInputCap(t *testing.T) {
	fx := newPCSCircuitFixture(t)
	prng := rand.New(utils.NewRandSource(4242))

	for tree := range fx.proof.InputCaps {
		for node := range fx.proof.InputCaps[tree].Nodes {
			// get a copy of the opening proof where one frontier node of this input
			// tree's cap is replaced with a random octuplet
			tampered := cloneProofWithInputCapNode(fx.proof, tree, node, field.PseudoRandOctuplet(prng))

			// sanity check: the tampering must be rejected by the native verifier
			require.Error(t, fx.pcs.Verify(fx.input, tampered),
				"native verifier must reject a tampered input cap %d node %d", tree, node)

			// property under test: the circuit must agree with the native verifier
			template, assignment := circuitFor(fx, tampered)
			require.Error(t, solvePCSCircuit(t, template, assignment),
				"circuit must reject a tampered input cap %d node %d", tree, node)
		}
	}
}

// TestPCSVerifyGnarkRejectsTamperedRoundCap does the same for the running-layer
// caps.
func TestPCSVerifyGnarkRejectsTamperedRoundCap(t *testing.T) {
	fx := newPCSCircuitFixture(t)
	prng := rand.New(utils.NewRandSource(909))

	for round := range fx.proof.FRIProof.RoundCaps {
		if len(fx.proof.FRIProof.RoundCaps[round].Nodes) == 0 {
			continue
		}
		tampered := cloneProofWithRoundCapNode(fx.proof, round, 0, field.PseudoRandOctuplet(prng))

		require.Error(t, fx.pcs.Verify(fx.input, tampered),
			"native verifier must reject a tampered round %d cap", round)

		template, assignment := circuitFor(fx, tampered)
		require.Error(t, solvePCSCircuit(t, template, assignment),
			"circuit must reject a tampered round %d cap", round)
	}
}

// cloneProofWithInputCapNode returns a copy of p with one input-cap frontier
// node replaced, sharing nothing mutable with the original.
func cloneProofWithInputCapNode(p OpeningProof, tree, node int, value field.Octuplet) OpeningProof {
	res := p
	res.InputCaps = append([]InputCap(nil), p.InputCaps...)
	res.InputCaps[tree].Nodes = append([]field.Octuplet(nil), p.InputCaps[tree].Nodes...)
	res.InputCaps[tree].Nodes[node] = value
	return res
}

// cloneProofWithRoundCapNode is the running-layer analogue.
func cloneProofWithRoundCapNode(p OpeningProof, round, node int, value field.Octuplet) OpeningProof {
	res := p
	res.FRIProof.RoundCaps = append([]MerkleCap(nil), p.FRIProof.RoundCaps...)
	res.FRIProof.RoundCaps[round].Nodes = append([]field.Octuplet(nil), p.FRIProof.RoundCaps[round].Nodes...)
	res.FRIProof.RoundCaps[round].Nodes[node] = value
	return res
}

//go:build !fuzzlight

package vortex_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/require"
)

/*
Wraps the wizard verification gnark into a circuit
*/
type VortexTestCircuit struct {
	C wizard.VerifierCircuit
}

/*
Just verify the wizard-IOP equation, also verifies
that the "x" is correctly set.
*/
func (c *VortexTestCircuit) Define(api frontend.API) error {
	c.C.Verify(api)
	return nil
}

/*
Returns an assignment from a wizard proof
*/
func assignTestCircuit(comp *wizard.CompiledIOP, proof wizard.Proof) *VortexTestCircuit {
	return &VortexTestCircuit{
		C: *wizard.AssignVerifierCircuit(comp, proof, 0),
	}
}

func TestVortexGnarkVerifier(t *testing.T) {

	polSize := 1 << 4
	nPols := 16
	numRounds := 3
	numPrecomputeds := 4
	rows := make([][]ifaces.Column, numRounds)

	define := func(b *wizard.Builder) {
		for round := 0; round < numRounds; round++ {
			// trigger the creation of a new round by declaring a dummy coin
			if round != 0 {
				_ = b.RegisterRandomCoin(coin.Namef("COIN_%v", round), coin.Field)
			}

			rows[round] = make([]ifaces.Column, nPols)
			if round == 0 {
				for i := 0; i < numPrecomputeds; i++ {
					p := smartvectors.Rand(polSize)
					rows[round][i] = b.RegisterPrecomputed(ifaces.ColIDf("PRE_COMP_%v", i), p)
				}
				for i := numPrecomputeds; i < nPols; i++ {
					rows[round][i] = b.RegisterCommit(ifaces.ColIDf("P_%v", i), polSize)
				}
				continue
			}
			for i := range rows[round] {
				rows[round][i] = b.RegisterCommit(ifaces.ColIDf("P_%v", round*nPols+i), polSize)
			}
		}

		b.UnivariateEval("EVAL", utils.Join(rows...)...)
	}

	prove := func(pr *wizard.ProverRuntime) {
		ys := make([]field.Element, len(rows)*len(rows[0]))
		x := field.NewElement(57) // the evaluation point

		// assign the rows with random polynomials and collect the ys
		for round := range rows {
			// let the prover know that it is free to go to the next
			// round by sampling the coin.
			if round != 0 {
				_ = pr.GetRandomCoinField(coin.Namef("COIN_%v", round))
			}
			for i, row := range rows[round] {
				// For round 0 we need (numPolys - numPrecomputeds) polys, as the precomputed are
				// assigned in the define phase
				if i < numPrecomputeds && round == 0 {
					p := pr.Spec.Precomputed.MustGet(row.GetColID())
					ys[round*nPols+i] = smartvectors.Interpolate(p, x)
					continue
				}
				p := smartvectors.Rand(polSize)
				ys[round*nPols+i] = smartvectors.Interpolate(p, x)
				pr.AssignColumn(row.GetColID(), p)
			}
		}

		pr.AssignUnivariate("EVAL", x, ys...)
	}

	compiled := wizard.Compile(
		define,
		vortex.Compile(4,
			vortex.WithOptionalSISHashingThreshold(1<<20))) // Set to a high value to disable SIS hashing, because gnark does not support SIS hashing
	proof := wizard.Prove(compiled, prove)

	// Just as a sanity check, do not run the Plonk
	valid := wizard.Verify(compiled, proof)
	require.NoErrorf(t, valid, "the proof did not pass")

	// Run the proof verifier

	// Allocate the circuit
	circ := VortexTestCircuit{}
	{
		c := wizard.AllocateWizardCircuit(compiled, 0)
		circ.C = *c
	}

	// Compile the circuit
	scs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		&circ,
		frontend.IgnoreUnconstrainedInputs(),
	)

	if err != nil {
		// When the error string is too large `require.NoError` does not print
		// the error.
		t.Logf("circuit construction failed : %v\n", err)
		t.FailNow()
	}

	// Checks that the proof makes a satifying assignment
	assignment := assignTestCircuit(compiled, proof)
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = scs.IsSolved(witness)

	if err != nil {
		// When the error string is too large `require.NoError` does not print
		// the error.
		t.Logf("circuit solving failed : %v\n", err)
		t.FailNow()
	}

}

//go:build !fuzzlight

package vortex_test

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
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
func assignTestCircuit(comp *wizard.CompiledIOP, proof wizard.Proof, isBLS bool) *VortexTestCircuit {
	return &VortexTestCircuit{
		C: *wizard.AssignVerifierCircuit(comp, proof, 0, isBLS),
	}
}

func TestVortexGnarkVerifier(t *testing.T) {
	tests := []struct {
		name  string
		isBLS bool
	}{
		{"BLS12-377", true},
		{"KoalaBear", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runVortexGnarkVerifier(t, tc.isBLS)
		})
	}
}

func runVortexGnarkVerifier(t *testing.T, isBLS bool) {
	polSize := 1 << 4
	nPols := 16
	numRounds := 3
	numPrecomputeds := 4
	rows := make([][]ifaces.Column, numRounds)

	define := func(b *wizard.Builder) {
		for round := 0; round < numRounds; round++ {
			// trigger the creation of a new round by declaring a dummy coin
			if round != 0 {
				_ = b.RegisterRandomCoin(coin.Namef("COIN_%v", round), coin.FieldExt)
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

		rng := rand.New(rand.NewPCG(0, 0)) // #nosec G404 -- test only

		ys := make([]fext.Element, len(rows)*len(rows[0]))
		x := fext.PseudoRand(rng) // the evaluation point

		// assign the rows with random polynomials and collect the ys
		for round := range rows {
			// let the prover know that it is free to go to the next
			// round by sampling the coin.
			if round != 0 {
				_ = pr.GetRandomCoinFieldExt(coin.Namef("COIN_%v", round))
			}
			for i, row := range rows[round] {
				// For round 0 we need (numPolys - numPrecomputeds) polys, as the precomputed are
				// assigned in the define phase
				if i < numPrecomputeds && round == 0 {
					p := pr.Spec.Precomputed.MustGet(row.GetColID())
					ys[round*nPols+i] = smartvectors.EvaluateBasePolyLagrange(p, x)
					continue
				}
				p := smartvectors.PseudoRand(rng, polSize)
				ys[round*nPols+i] = smartvectors.EvaluateBasePolyLagrange(p, x)
				pr.AssignColumn(row.GetColID(), p)
			}
		}

		pr.AssignUnivariateExt("EVAL", x, ys...)
	}

	// Set SISHashingThreshold to a high value to disable SIS hashing, because gnark does not support SIS hashing
	compiled := wizard.Compile(
		define,
		vortex.Compile(4,
			isBLS,
			vortex.WithOptionalSISHashingThreshold(1<<20)))

	proof := wizard.Prove(compiled, prove, isBLS)

	// Just as a sanity check, do not run the Plonk
	valid := wizard.Verify(compiled, proof, isBLS)
	require.NoErrorf(t, valid, "the proof did not pass")

	// Run the proof verifier

	// Allocate the circuit
	circ := VortexTestCircuit{}
	{
		c := wizard.AllocateWizardCircuit(compiled, 0, isBLS)
		circ.C = *c
	}

	// Checks that the proof makes a satisfying assignment
	assignment := assignTestCircuit(compiled, proof, isBLS)

	if isBLS {
		cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder,
			&circ,
			frontend.IgnoreUnconstrainedInputs())

		if err != nil {
			t.Logf("circuit construction failed : %v\n", err)
			t.FailNow()
		}

		witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
		require.NoError(t, err)

		err = cs.IsSolved(witness)
		if err != nil {
			t.Logf("circuit solving failed : %v. Retrying with test engine\n", err)
			errDetail := test.IsSolved(assignment, assignment, cs.Field())
			t.Logf("while running the plonk prover: %v", errDetail)
			t.FailNow()
		}

	} else {
		cs, err := frontend.CompileU32(koalabear.Modulus(), gnarkutil.NewMockBuilder(scs.NewBuilder),
			&circ,
			frontend.IgnoreUnconstrainedInputs())

		if err != nil {
			t.Logf("circuit construction failed : %v\n", err)
			t.FailNow()
		}

		witness, err := frontend.NewWitness(assignment, koalabear.Modulus())
		require.NoError(t, err)

		err = cs.IsSolved(witness)
		if err != nil {
			t.Logf("circuit solving failed : %v. Retrying with test engine\n", err)
			errDetail := test.IsSolved(assignment, assignment, cs.Field())
			t.Logf("while running the plonk prover: %v", errDetail)
			t.FailNow()
		}
	}
}

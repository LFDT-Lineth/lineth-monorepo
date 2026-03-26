package plonkinternal_test

import (
	"testing"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/stretchr/testify/require"
)

// Circuit defines a simple circuit for test
// x**3 + x + 5 == y
type MyCircuit struct {
	// struct tags on a variable is optional
	// default uses variable name and secret visibility.
	X frontend.Variable `gnark:"x,public"`
	Y frontend.Variable `gnark:",public"`
}

// Define declares the circuit constraints
// x**3 + x + 5 == y
func (circuit *MyCircuit) Define(api frontend.API) error {
	x3 := api.Mul(circuit.X, circuit.X, circuit.X)
	api.AssertIsEqual(circuit.Y, api.Add(x3, circuit.X, 5))
	return nil
}

func TestPlonkWizard(t *testing.T) {

	circuit := &MyCircuit{}

	var pa plonkinternal.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonkinternal.PlonkCheck(build.CompiledIOP, "PLONK", 0, circuit, 1)
			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{
			gnarkutil.WitnessFromNativeKoala(
				field.NewElement(0),
				field.NewElement(5),
			),
		})
	})

	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

func TestPlonkWizardWithFixedNbRow(t *testing.T) {

	circuit := &MyCircuit{}

	var pa plonkinternal.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonkinternal.PlonkCheck(build.CompiledIOP, "PLONK", 0, circuit, 1, plonkinternal.WithFixedNbRows(256))
			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{gnarkutil.WitnessFromNativeKoala(
			field.NewElement(0),
			field.NewElement(5),
		)})
	})

	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

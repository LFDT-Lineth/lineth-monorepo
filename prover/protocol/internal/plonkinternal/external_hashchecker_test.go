package plonkinternal

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

// externalMiMCFactoryTestLinear is used to test the external MiMC factory
// and is a gnark circuit implementing a linear hash.
type externalMimcFactoryTestLinear struct {
	Inp [16]frontend.Variable
}

// Define implements the gnark frontend.Circuit interface.
// It is a test circuit to compare the ExternalHasherFactory with the BasicHasherFactory.
// It takes 16 inputs and compute the MiMC hash of the inputs using both factories.
// The two results are then compared to ensure they are equal.
func (circuit *externalMimcFactoryTestLinear) Define(api frontend.API) error {

	var (
		factory      = &mimc.ExternalHasherFactory{Api: api}
		factoryBasic = &mimc.BasicHasherFactory{Api: api}
		hasher       = factory.NewHasher()
		hasherBasic  = factoryBasic.NewHasher()
	)

	hasher.Write(circuit.Inp[:]...)
	hasherBasic.Write(circuit.Inp[:]...)
	hsum := hasher.Sum()
	hsumBasic := hasherBasic.Sum()
	api.AssertIsEqual(hsum, hsumBasic)

	return nil
}

func TestMiMCFactories(t *testing.T) {

	solver.RegisterHint(mimc.MimcHintfunc)

	var (
		blsField   = ecc.BLS12_377.ScalarField()
		circuit    = &externalMimcFactoryTestLinear{}
		assignment = &externalMimcFactoryTestLinear{Inp: [16]frontend.Variable{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
		wit, wErr  = frontend.NewWitness(assignment, blsField)
	)

	if wErr != nil {
		t.Fatalf("unexpected witness error: %v", wErr)
	}

	var pa PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit, 1,
				WithExternalHasher(32),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{wit})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)

}

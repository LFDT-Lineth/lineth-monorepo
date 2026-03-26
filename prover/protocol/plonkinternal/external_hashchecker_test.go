package plonkinternal

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	hasherfactory "github.com/consensys/linea-monorepo/prover/crypto/hasherfactory_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

// externalPoseidon2FactoryTestLinear is used to test the external Poseidon2 factory
// and is a gnark circuit implementing a linear hash.
type externalPoseidon2FactoryTestLinear struct {
	Inp [16]frontend.Variable
}

// Define implements the gnark frontend.Circuit interface.
// It is a test circuit to compare the ExternalHasherFactory with the BasicHasherFactory.
// It takes 16 inputs and compute the Poseidon2 hash of the inputs using both factories.
// The two results are then compared to ensure they are equal.
func (circuit *externalPoseidon2FactoryTestLinear) Define(api frontend.API) error {

	var (
		factory      = &hasherfactory.ExternalHasherFactory{Api: api}
		factoryBasic = &hasherfactory.BasicHasherFactory{Api: api}
		hasher       = factory.NewHasher()
		hasherBasic  = factoryBasic.NewHasher()
	)

	hasher.Write(circuit.Inp[:]...)
	hasherBasic.Write(circuit.Inp[:]...)
	hsum := hasher.Sum()
	hsumBasic := hasherBasic.Sum()
	for i := 0; i < poseidon2_koalabear.BlockSize; i++ {
		api.AssertIsEqual(hsum[i], hsumBasic[i])
	}

	return nil
}

func TestPoseidon2Factories(t *testing.T) {

	solver.RegisterHint(hasherfactory.Poseidon2Hintfunc)

	var (
		koalaField = koalabear.Modulus()
		circuit    = &externalPoseidon2FactoryTestLinear{}
		assignment = &externalPoseidon2FactoryTestLinear{Inp: [16]frontend.Variable{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}
		wit, wErr  = frontend.NewWitness(assignment, koalaField)
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
				WithExternalHasher(16),
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

package mimc

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	cs "github.com/consensys/gnark/constraint/bls12-377"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
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
		factory      = &ExternalHasherFactory{Api: api}
		factoryBasic = &BasicHasherFactory{Api: api}
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

	var (
		circuit            = &externalMimcFactoryTestLinear{}
		builder, hshGetter = NewExternalHasherBuilder(true)
		blsField           = ecc.BLS12_377.ScalarField()
		assignment         = &externalMimcFactoryTestLinear{Inp: [16]frontend.Variable{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
	)

	solver.RegisterHint(MimcHintfunc)

	ccs_, compileErr := frontend.Compile(blsField, builder, circuit)

	if compileErr != nil {
		t.Fatalf("unexpected compile error: %v", compileErr)
	}

	ccs := ccs_.(*cs.SparseR1CS)

	witness, wErr := frontend.NewWitness(assignment, blsField)

	if wErr != nil {
		t.Fatalf("unexpected witness error: %v", wErr)
	}

	sol_, solErr := ccs.Solve(witness)

	if solErr != nil {
		t.Fatalf("unexpected solution error: %v", solErr)
	}

	var (
		sol        = sol_.(*cs.SparseR1CSSolution)
		hshWires   = hshGetter()
		getFromLRO = func(csID, colID int) field.Element {
			if colID == 0 {
				return sol.L[csID]
			}

			if colID == 1 {
				return sol.R[csID]
			}

			return sol.O[csID]
		}
	)

	for _, triplet := range hshWires {

		var (
			oldState  = getFromLRO(triplet[0][0], triplet[0][1])
			block     = getFromLRO(triplet[1][0], triplet[1][1])
			newState  = getFromLRO(triplet[2][0], triplet[2][1])
			newState_ = BlockCompression(oldState, block)
		)

		if newState != newState_ {
			t.Errorf("expected %v, got %v", newState, newState_)
		}
	}
}

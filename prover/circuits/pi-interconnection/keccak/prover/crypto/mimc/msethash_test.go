package mimc_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	cs "github.com/consensys/gnark/constraint/bls12-377"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
)

type MsetOfSingletonGnarkTestCircuit struct {
	X [12]frontend.Variable
	R [mimc.MSetHashSize]frontend.Variable
}

func (circuit *MsetOfSingletonGnarkTestCircuit) Define(api frontend.API) error {
	r := mimc.MsetOfSingletonGnark(api, nil, circuit.X[:]...)
	r.AssertEqualRaw(api, circuit.R[:])
	return nil
}

func TestMSetHash(t *testing.T) {

	var (
		circuit  = &MsetOfSingletonGnarkTestCircuit{}
		assigned = &MsetOfSingletonGnarkTestCircuit{}
		blsField = ecc.BLS12_377.ScalarField()
		builder  = scs.NewBuilder[constraint.U64]
		msg      = [12]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
		msgField = []field.Element{}
		mset     = mimc.MSetHash{}
	)

	for i := range msg {
		msgField = append(msgField, field.NewElement(msg[i]))
		assigned.X[i] = msg[i]
	}

	mset.Insert(msgField...)

	for i := range mset {
		assigned.R[i] = mset[i]
	}

	ccs_, compileErr := frontend.Compile(blsField, builder, circuit)
	if compileErr != nil {
		t.Fatalf("unexpected compile error: %v", compileErr)
	}

	ccs := ccs_.(*cs.SparseR1CS)

	witness, wErr := frontend.NewWitness(assigned, blsField)
	if wErr != nil {
		t.Fatalf("unexpected witness error: %v", wErr)
	}

	_, solErr := ccs.Solve(witness)
	if solErr != nil {
		t.Fatalf("unexpected solution error: %v", solErr)
	}
}

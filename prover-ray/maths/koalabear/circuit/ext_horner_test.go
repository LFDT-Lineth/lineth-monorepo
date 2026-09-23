package circuit

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

const hornerTestDegree = 5

// TestHornerCircuit checks [API.HornerExt] against a witness-computed
// evaluation, and [API.SubByBaseExt] against the general [API.SubExt] it is a
// specialization of.
type TestHornerCircuit struct {
	Coeffs [hornerTestDegree]Ext
	X      Ext
	Eval   Ext
	Base   Element
	SubBXB Ext
}

func (c *TestHornerCircuit) Define(api frontend.API) error {
	f := NewAPI(api)

	f.AssertIsEqualExt(f.HornerExt(c.Coeffs[:], c.X), c.Eval)

	// The empty and single-coefficient sums are the degenerate cases.
	f.AssertIsEqualExt(f.HornerExt(nil, c.X), f.ZeroExt())
	f.AssertIsEqualExt(f.HornerExt(c.Coeffs[:1], c.X), c.Coeffs[0])

	// SubByBaseExt must agree with the full SubExt against the lift of Base.
	subByBase := f.SubByBaseExt(c.X, c.Base)
	f.AssertIsEqualExt(subByBase, f.SubExt(c.X, f.FromBaseExt(c.Base)))
	f.AssertIsEqualExt(subByBase, c.SubBXB)

	return nil
}

func getHornerWitness() *TestHornerCircuit {
	var x, eval field.Ext
	if _, err := x.SetRandom(); err != nil {
		panic(err)
	}

	var coeffs [hornerTestDegree]field.Ext
	circ := &TestHornerCircuit{X: NewExt(x)}
	for i := range coeffs {
		if _, err := coeffs[i].SetRandom(); err != nil {
			panic(err)
		}
		circ.Coeffs[i] = NewExt(coeffs[i])
	}

	// eval = Σ_i coeffs[i]·x^i, accumulated from the top down.
	eval = coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		eval.Mul(&eval, &x)
		eval.Add(&eval, &coeffs[i])
	}
	circ.Eval = NewExt(eval)

	var base field.Element
	if _, err := base.SetRandom(); err != nil {
		panic(err)
	}
	baseLifted := field.Lift(base)
	var subBXB field.Ext
	subBXB.Sub(&x, &baseLifted)
	circ.Base = NewElementFromKoala(base)
	circ.SubBXB = NewExt(subBXB)

	return circ
}

func TestHornerNative(t *testing.T) {
	witness := getHornerWitness()
	var circuit TestHornerCircuit

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	require.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, koalabear.Modulus())
	require.NoError(t, err)

	require.NoError(t, ccs.IsSolved(fullWitness))
}

func TestHornerEmulated(t *testing.T) {
	witness := getHornerWitness()
	var circuit TestHornerCircuit

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	require.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	require.NoError(t, ccs.IsSolved(fullWitness))
}

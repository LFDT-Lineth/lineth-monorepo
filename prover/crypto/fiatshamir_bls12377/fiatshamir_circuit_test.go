package fiatshamir_bls12377

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type FSCircuit struct {
	// frElements
	A, B   frontend.Variable
	R1, R2 frontend.Variable

	// koalabear octuplet
	C  [2]koalagnark.Element
	D  [10]koalagnark.Element
	R3 koalagnark.Octuplet

	// random many integers
	R4    []frontend.Variable
	n     int
	bound int

	// set state, get state
	SetState, GetState koalagnark.Octuplet
}

func (c *FSCircuit) Define(api frontend.API) error {

	fs := NewGnarkFS(api)

	// frElements
	fs.UpdateFrElmt(c.A)
	a := fs.RandomFrElmt()
	fs.UpdateFrElmt(c.B)
	b := fs.RandomFrElmt()
	api.AssertIsEqual(a, c.R1)
	api.AssertIsEqual(b, c.R2)

	// koalabear octuplet
	fs.Update(c.C[:]...)
	e := fs.RandomField()
	koalaAPI := koalagnark.NewAPI(api)
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(e[i], c.R3[i])
	}

	// random many integers
	fs.Update(c.D[:]...)
	res := fs.RandomManyIntegers(c.n, c.bound)
	for i := 0; i < len(res); i++ {
		api.AssertIsEqual(res[i], c.R4[i])
	}

	// set state, get state
	fs.SetState(c.SetState)
	getState := fs.State()
	for i := 0; i < len(getState); i++ {
		koalaAPI.AssertIsEqual(getState[i], c.GetState[i])
	}

	return nil
}

func GetCircuitWitnessFSCircuit() (*FSCircuit, *FSCircuit) {

	fs := NewFS()

	// fr element
	var a, b fr.Element
	a.SetRandom()
	b.SetRandom()
	fs.UpdateFrElmt(a)
	r1 := fs.RandomFieldFrElmt()
	fs.UpdateFrElmt(b)
	r2 := fs.RandomFieldFrElmt()

	// koalabear element
	var c [2]field.Element
	c[0].SetRandom()
	c[1].SetRandom()
	var d [10]field.Element
	for i := 0; i < 10; i++ {
		d[i].SetRandom()
	}
	fs.Update(c[:]...)
	r3 := fs.RandomField()

	// random many integers
	fs.Update(d[:]...)
	n := 5
	bound := 8
	r4 := fs.RandomManyIntegers(n, bound)

	// set state, get state
	var setState field.Octuplet
	for i := 0; i < 8; i++ {
		setState[i].SetRandom()
	}
	fs.SetState(setState)
	getState := fs.State()

	var circuit, witness FSCircuit
	witness.A = a.String()
	witness.B = b.String()
	witness.R1 = r1.String()
	witness.R2 = r2.String()
	witness.C[0] = koalagnark.NewElementFromKoala(c[0])
	witness.C[1] = koalagnark.NewElementFromKoala(c[1])
	for i := 0; i < 10; i++ {
		witness.D[i] = koalagnark.NewElementFromKoala(d[i])
	}
	for i := 0; i < 8; i++ {
		witness.R3[i] = koalagnark.NewElementFromKoala(r3[i])
		witness.SetState[i] = koalagnark.NewElementFromKoala(setState[i])
		witness.GetState[i] = koalagnark.NewElementFromKoala(getState[i])
	}

	circuit.n = n
	circuit.bound = bound
	witness.R4 = make([]frontend.Variable, n)
	circuit.R4 = make([]frontend.Variable, n)
	for i := 0; i < n; i++ {
		witness.R4[i] = r4[i]
	}

	return &circuit, &witness
}

func TestFSCircuit(t *testing.T) {

	circuit, witness := GetCircuitWitnessFSCircuit()
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

type KoalaFlushSpecificCircuit struct {
	Input   [8]koalagnark.Element
	Output  koalagnark.Octuplet
	isKoala bool
}

func (c *KoalaFlushSpecificCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)

	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < len(res); i++ {
		koalaAPI.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func getKoalaFlushSpecificWitness(isKoala bool) (*KoalaFlushSpecificCircuit, *KoalaFlushSpecificCircuit) {
	var circuit, witness KoalaFlushSpecificCircuit
	circuit.isKoala = isKoala
	witness.isKoala = isKoala
	fs := NewFS()

	var input [8]field.Element
	var one field.Element
	one.SetOne()

	for i := 0; i < 4; i++ {
		// input[2*i] = -1
		input[2*i] = *field.MaxVal
		// input[2*i+1] = 0
		input[2*i+1].SetZero()
	}

	fs.Update(input[:]...)
	output := fs.RandomField()

	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = koalagnark.NewElementFromKoala(output[i])
	}

	return &circuit, &witness
}

func TestKoalaFlushSpecific(t *testing.T) {

	// compile on bls
	{
		circuit, witness := getKoalaFlushSpecificWitness(false)
		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
}

// Test for UpdateExt with field extension elements
type UpdateExtCircuit struct {
	ExtInputs [3]koalagnark.Ext
	Output    koalagnark.Octuplet
}

func (c *UpdateExtCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.UpdateExt(c.ExtInputs[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestUpdateExt(t *testing.T) {
	fs := NewFS()

	var extInputs [3]fext.Element
	for i := 0; i < 3; i++ {
		extInputs[i].SetRandom()
	}

	fs.UpdateExt(extInputs[:]...)
	output := fs.RandomField()

	var circuit, witness UpdateExtCircuit
	for i := 0; i < 3; i++ {
		witness.ExtInputs[i] = koalagnark.NewExt(extInputs[i])
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = koalagnark.NewElementFromKoala(output[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for RandomFieldExt
type RandomFieldExtCircuit struct {
	Input     [2]koalagnark.Element
	OutputExt koalagnark.Ext
}

func (c *RandomFieldExtCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomFieldExt()

	koalaAPI.AssertIsEqual(res.B0.A0, c.OutputExt.B0.A0)
	koalaAPI.AssertIsEqual(res.B0.A1, c.OutputExt.B0.A1)
	koalaAPI.AssertIsEqual(res.B1.A0, c.OutputExt.B1.A0)
	koalaAPI.AssertIsEqual(res.B1.A1, c.OutputExt.B1.A1)
	return nil
}

func TestRandomFieldExt(t *testing.T) {
	fs := NewFS()

	var input [2]field.Element
	input[0].SetRandom()
	input[1].SetRandom()

	fs.Update(input[:]...)
	output := fs.RandomFext()

	var circuit, witness RandomFieldExtCircuit
	witness.Input[0] = koalagnark.NewElementFromKoala(input[0])
	witness.Input[1] = koalagnark.NewElementFromKoala(input[1])
	witness.OutputExt = koalagnark.NewExt(output)

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for RandomManyIntegers with different bounds
type RandomManyIntegersCircuit struct {
	Input  [5]koalagnark.Element
	Output []frontend.Variable
	n      int
	bound  int
}

func (c *RandomManyIntegersCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomManyIntegers(c.n, c.bound)
	for i := 0; i < len(res); i++ {
		api.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestRandomManyIntegersVariousBounds(t *testing.T) {
	testCases := []struct {
		name  string
		n     int
		bound int
	}{
		{"small_bound", 3, 4},
		{"medium_bound", 5, 16},
		{"large_bound", 10, 128},
		{"power_of_two_bound", 8, 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := NewFS()

			var input [5]field.Element
			for i := 0; i < 5; i++ {
				input[i].SetRandom()
			}

			fs.Update(input[:]...)
			output := fs.RandomManyIntegers(tc.n, tc.bound)

			var circuit, witness RandomManyIntegersCircuit
			circuit.n = tc.n
			circuit.bound = tc.bound
			circuit.Output = make([]frontend.Variable, tc.n)
			witness.Output = make([]frontend.Variable, tc.n)

			for i := 0; i < 5; i++ {
				witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
			}
			for i := 0; i < tc.n; i++ {
				witness.Output[i] = output[i]
			}

			ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
			assert.NoError(t, err)

			fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
			assert.NoError(t, err)
			err = ccs.IsSolved(fullWitness)
			assert.NoError(t, err)
		})
	}
}

// Test for SetState and State round-trip
type StateRoundTripCircuit struct {
	InitialState koalagnark.Octuplet
	FinalState   koalagnark.Octuplet
}

func (c *StateRoundTripCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.SetState(c.InitialState)
	state := fs.State()

	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(state[i], c.FinalState[i])
	}
	return nil
}

func TestStateRoundTrip(t *testing.T) {
	fs := NewFS()

	var initialState field.Octuplet
	for i := 0; i < 8; i++ {
		initialState[i].SetRandom()
	}

	fs.SetState(initialState)
	finalState := fs.State()

	var circuit, witness StateRoundTripCircuit
	for i := 0; i < 8; i++ {
		witness.InitialState[i] = koalagnark.NewElementFromKoala(initialState[i])
		witness.FinalState[i] = koalagnark.NewElementFromKoala(finalState[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for empty buffer flush (edge case)
type EmptyFlushCircuit struct {
	Input  frontend.Variable
	Output frontend.Variable
}

func (c *EmptyFlushCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)

	// Update with FrElmt only (no koala buffer)
	fs.UpdateFrElmt(c.Input)
	res := fs.RandomFrElmt()

	api.AssertIsEqual(res, c.Output)
	return nil
}

func TestEmptyKoalaBufferFlush(t *testing.T) {
	fs := NewFS()

	var input fr.Element
	input.SetRandom()

	fs.UpdateFrElmt(input)
	output := fs.RandomFieldFrElmt()

	var circuit, witness EmptyFlushCircuit
	witness.Input = input.String()
	witness.Output = output.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for multiple sequential RandomField calls
type MultipleRandomFieldCircuit struct {
	Input   [4]koalagnark.Element
	Output1 koalagnark.Octuplet
	Output2 koalagnark.Octuplet
}

func (c *MultipleRandomFieldCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.Input[:]...)
	res1 := fs.RandomField()
	res2 := fs.RandomField()

	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(res1[i], c.Output1[i])
		koalaAPI.AssertIsEqual(res2[i], c.Output2[i])
	}
	return nil
}

func TestMultipleRandomFieldCalls(t *testing.T) {
	fs := NewFS()

	var input [4]field.Element
	for i := 0; i < 4; i++ {
		input[i].SetRandom()
	}

	fs.Update(input[:]...)
	output1 := fs.RandomField()
	output2 := fs.RandomField()

	var circuit, witness MultipleRandomFieldCircuit
	for i := 0; i < 4; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	for i := 0; i < 8; i++ {
		witness.Output1[i] = koalagnark.NewElementFromKoala(output1[i])
		witness.Output2[i] = koalagnark.NewElementFromKoala(output2[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for mixed Update and UpdateFrElmt
type MixedUpdateCircuit struct {
	KoalaInput [2]koalagnark.Element
	FrInput    frontend.Variable
	Output     koalagnark.Octuplet
}

func (c *MixedUpdateCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.KoalaInput[:]...)
	fs.UpdateFrElmt(c.FrInput)
	res := fs.RandomField()

	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestMixedUpdate(t *testing.T) {
	fs := NewFS()

	var koalaInput [2]field.Element
	koalaInput[0].SetRandom()
	koalaInput[1].SetRandom()

	var frInput fr.Element
	frInput.SetRandom()

	fs.Update(koalaInput[:]...)
	fs.UpdateFrElmt(frInput)
	output := fs.RandomField()

	var circuit, witness MixedUpdateCircuit
	witness.KoalaInput[0] = koalagnark.NewElementFromKoala(koalaInput[0])
	witness.KoalaInput[1] = koalagnark.NewElementFromKoala(koalaInput[1])
	witness.FrInput = frInput.String()
	for i := 0; i < 8; i++ {
		witness.Output[i] = koalagnark.NewElementFromKoala(output[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for UpdateVecFrElmt
type UpdateVecFrElmtCircuit struct {
	Vec1   [2]frontend.Variable
	Vec2   [3]frontend.Variable
	Output frontend.Variable
}

func (c *UpdateVecFrElmtCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)

	fs.UpdateVecFrElmt(c.Vec1[:], c.Vec2[:])
	res := fs.RandomFrElmt()

	api.AssertIsEqual(res, c.Output)
	return nil
}

func TestUpdateVecFrElmt(t *testing.T) {
	fs := NewFS()

	var vec1 [2]fr.Element
	var vec2 [3]fr.Element
	for i := 0; i < 2; i++ {
		vec1[i].SetRandom()
	}
	for i := 0; i < 3; i++ {
		vec2[i].SetRandom()
	}

	fs.UpdateVecFrElmt(vec1[:], vec2[:])
	output := fs.RandomFieldFrElmt()

	var circuit, witness UpdateVecFrElmtCircuit
	for i := 0; i < 2; i++ {
		witness.Vec1[i] = vec1[i].String()
	}
	for i := 0; i < 3; i++ {
		witness.Vec2[i] = vec2[i].String()
	}
	witness.Output = output.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test edge case with zero values
type ZeroValuesCircuit struct {
	Input  [4]koalagnark.Element
	Output koalagnark.Octuplet
}

func (c *ZeroValuesCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestZeroValues(t *testing.T) {
	fs := NewFS()

	var input [4]field.Element
	// All zeros
	for i := 0; i < 4; i++ {
		input[i].SetZero()
	}

	fs.Update(input[:]...)
	output := fs.RandomField()

	var circuit, witness ZeroValuesCircuit
	for i := 0; i < 4; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = koalagnark.NewElementFromKoala(output[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test edge case with max values
type MaxValuesCircuit struct {
	Input  [4]koalagnark.Element
	Output koalagnark.Octuplet
}

func (c *MaxValuesCircuit) Define(api frontend.API) error {
	fs := NewGnarkFS(api)
	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestMaxValues(t *testing.T) {
	fs := NewFS()

	var input [4]field.Element
	// All max values (p-1)
	for i := 0; i < 4; i++ {
		input[i] = *field.MaxVal
	}

	fs.Update(input[:]...)
	output := fs.RandomField()

	var circuit, witness MaxValuesCircuit
	for i := 0; i < 4; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = koalagnark.NewElementFromKoala(output[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

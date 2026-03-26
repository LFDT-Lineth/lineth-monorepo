package fiatshamir_koalabear

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

// Test for UpdateExt with field extension elements
type UpdateExtCircuit struct {
	ExtInputs [3]koalagnark.Ext
	Output    poseidon2_koalabear.GnarkOctuplet
}

func (c *UpdateExtCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.UpdateExt(c.ExtInputs[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res[i].Native(), c.Output[i])
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
		witness.Output[i] = output[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
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
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomFieldExt()

	koalaAPI := koalagnark.NewAPI(api)
	koalaAPI.AssertIsEqualExt(res, c.OutputExt)
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
	witness.Input[0] = koalagnark.NewElement(input[0].String())
	witness.Input[1] = koalagnark.NewElement(input[1].String())
	witness.OutputExt = koalagnark.NewExt(output)

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for UpdateVec with multiple vectors
type UpdateVecCircuit struct {
	Vec1   [3]koalagnark.Element
	Vec2   [4]koalagnark.Element
	Output poseidon2_koalabear.GnarkOctuplet
}

func (c *UpdateVecCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.UpdateVec(c.Vec1[:], c.Vec2[:])
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res[i].Native(), c.Output[i])
	}
	return nil
}

func TestUpdateVec(t *testing.T) {
	fs := NewFS()

	var vec1 [3]field.Element
	var vec2 [4]field.Element
	for i := 0; i < 3; i++ {
		vec1[i].SetRandom()
	}
	for i := 0; i < 4; i++ {
		vec2[i].SetRandom()
	}

	fs.UpdateVec(vec1[:], vec2[:])
	output := fs.RandomField()

	var circuit, witness UpdateVecCircuit
	for i := 0; i < 3; i++ {
		witness.Vec1[i] = koalagnark.NewElement(vec1[i].String())
	}
	for i := 0; i < 4; i++ {
		witness.Vec2[i] = koalagnark.NewElement(vec2[i].String())
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = output[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for RandomManyIntegers with different bounds
type RandomManyIntegersCircuit struct {
	Input  [5]koalagnark.Element
	Output []koalagnark.Element
	n      int
	bound  int
}

func (c *RandomManyIntegersCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomManyIntegers(c.n, c.bound)
	for i := 0; i < len(res); i++ {
		api.AssertIsEqual(res[i], c.Output[i].Native())
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
			circuit.Output = make([]koalagnark.Element, tc.n)
			witness.Output = make([]koalagnark.Element, tc.n)

			for i := 0; i < 5; i++ {
				witness.Input[i] = koalagnark.NewElement(input[i].String())
			}
			for i := 0; i < tc.n; i++ {
				witness.Output[i] = koalagnark.NewElement(output[i])
			}

			ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
			assert.NoError(t, err)

			fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
			assert.NoError(t, err)
			err = ccs.IsSolved(fullWitness)
			assert.NoError(t, err)
		})
	}
}

// Test for SetState and State round-trip
type StateRoundTripCircuit struct {
	InitialState poseidon2_koalabear.GnarkOctuplet
	FinalState   poseidon2_koalabear.GnarkOctuplet
}

func (c *StateRoundTripCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	var oct koalagnark.Octuplet
	for i := range oct {
		oct[i] = koalagnark.WrapFrontendVariable(c.InitialState[i])
	}
	fs.SetState(oct)
	state := fs.State()

	for i := 0; i < 8; i++ {
		api.AssertIsEqual(state[i].Native(), c.FinalState[i])
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
		witness.InitialState[i] = initialState[i]
		witness.FinalState[i] = finalState[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for multiple sequential RandomField calls
type MultipleRandomFieldCircuit struct {
	Input   [4]koalagnark.Element
	Output1 poseidon2_koalabear.GnarkOctuplet
	Output2 poseidon2_koalabear.GnarkOctuplet
}

func (c *MultipleRandomFieldCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res1 := fs.RandomField()
	res2 := fs.RandomField()

	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res1[i].Native(), c.Output1[i])
		api.AssertIsEqual(res2[i].Native(), c.Output2[i])
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
		witness.Input[i] = koalagnark.NewElement(input[i].String())
	}
	for i := 0; i < 8; i++ {
		witness.Output1[i] = output1[i]
		witness.Output2[i] = output2[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test edge case with zero values
type ZeroValuesCircuit struct {
	Input  [4]koalagnark.Element
	Output poseidon2_koalabear.GnarkOctuplet
}

func (c *ZeroValuesCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res[i].Native(), c.Output[i])
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
		witness.Input[i] = koalagnark.NewElement(input[i].String())
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = output[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test edge case with max values
type MaxValuesCircuit struct {
	Input  [4]koalagnark.Element
	Output poseidon2_koalabear.GnarkOctuplet
}

func (c *MaxValuesCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res[i].Native(), c.Output[i])
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
		witness.Input[i] = koalagnark.NewElement(input[i].String())
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = output[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test for mixed zero and max values
type MixedValuesCircuit struct {
	Input  [8]koalagnark.Element
	Output poseidon2_koalabear.GnarkOctuplet
}

func (c *MixedValuesCircuit) Define(api frontend.API) error {
	fs := NewGnarkFSWV(api)

	fs.Update(c.Input[:]...)
	res := fs.RandomField()
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(res[i].Native(), c.Output[i])
	}
	return nil
}

func TestMixedValues(t *testing.T) {
	fs := NewFS()

	var input [8]field.Element
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			input[i].SetZero()
		} else {
			input[i] = *field.MaxVal
		}
	}

	fs.Update(input[:]...)
	output := fs.RandomField()

	var circuit, witness MixedValuesCircuit
	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElement(input[i].String())
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = output[i]
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

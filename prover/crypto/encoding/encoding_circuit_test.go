package encoding

import (
	"errors"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Circuits
// =============================================================================

type EncodingCircuit struct {
	ToEncode1 [8]koalagnark.Element
	ToEncode2 [12]koalagnark.Element
	ToEncode3 frontend.Variable
	R1        frontend.Variable
	R2        [2]frontend.Variable
	R3        koalagnark.Octuplet
}

func (c *EncodingCircuit) Define(api frontend.API) error {

	a := EncodeWVsToFVs(api, c.ToEncode1[:])
	b := EncodeWVsToFVs(api, c.ToEncode2[:])
	d := EncodeFVTo8WVs(api, c.ToEncode3)
	if len(a) != 1 {
		return errors.New("ToEncode1 should correspond to a single frelement")
	}
	if len(b) != 2 {
		return errors.New("ToEncode2should correspond to 2 frelement")
	}

	api.AssertIsEqual(a[0], c.R1)
	api.AssertIsEqual(b[0], c.R2[0])
	api.AssertIsEqual(b[1], c.R2[1])

	koalaAPI := koalagnark.NewAPI(api)
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(c.R3[i], d[i])
	}

	return nil
}

// Circuit for testing Encode8WVsToFV directly
type Encode8WVsCircuit struct {
	Input  [8]koalagnark.Element
	Output frontend.Variable
}

func (c *Encode8WVsCircuit) Define(api frontend.API) error {
	result := Encode8WVsToFV(api, c.Input)
	api.AssertIsEqual(result, c.Output)
	return nil
}

// Circuit for testing Encode9WVsToFV (BLS to Koalabear encoding)
type Encode9WVsCircuit struct {
	Input  [KoalabearChunks]koalagnark.Element
	Output frontend.Variable
}

func (c *Encode9WVsCircuit) Define(api frontend.API) error {
	result := Encode9WVsToFV(api, c.Input)
	api.AssertIsEqual(result, c.Output)
	return nil
}

// Circuit for testing round-trip: BLS -> 9 Koalabear -> BLS
type RoundTripBLS9KoalaCircuit struct {
	Original   frontend.Variable
	Decomposed [KoalabearChunks]koalagnark.Element
}

func (c *RoundTripBLS9KoalaCircuit) Define(api frontend.API) error {
	// Encode the decomposed values back to BLS
	reconstructed := Encode9WVsToFV(api, c.Decomposed)
	api.AssertIsEqual(c.Original, reconstructed)
	return nil
}

// Circuit for testing EncodeWVsToFVs with various input sizes
type EncodeWVsToFVsCircuit struct {
	Input  []koalagnark.Element
	Output []frontend.Variable
	size   int
}

func (c *EncodeWVsToFVsCircuit) Define(api frontend.API) error {
	result := EncodeWVsToFVs(api, c.Input)
	if len(result) != len(c.Output) {
		return errors.New("output length mismatch")
	}
	for i := 0; i < len(result); i++ {
		api.AssertIsEqual(result[i], c.Output[i])
	}
	return nil
}

// Circuit for testing zero values
type ZeroValuesCircuit struct {
	Input  [8]koalagnark.Element
	Output frontend.Variable
}

func (c *ZeroValuesCircuit) Define(api frontend.API) error {
	result := Encode8WVsToFV(api, c.Input)
	api.AssertIsEqual(result, c.Output)
	return nil
}

// Circuit for testing max values (p-1 for koalabear field)
type MaxValuesCircuit struct {
	Input  [8]koalagnark.Element
	Output frontend.Variable
}

func (c *MaxValuesCircuit) Define(api frontend.API) error {
	result := Encode8WVsToFV(api, c.Input)
	api.AssertIsEqual(result, c.Output)
	return nil
}

// Circuit for consistency between native and circuit encoding
type ConsistencyCircuit struct {
	KoalaInput [8]koalagnark.Element
	FrOutput   frontend.Variable
}

func (c *ConsistencyCircuit) Define(api frontend.API) error {
	result := Encode8WVsToFV(api, c.KoalaInput)
	api.AssertIsEqual(result, c.FrOutput)
	return nil
}

// =============================================================================
// Tests
// =============================================================================

func TestEncoding(t *testing.T) {
	// get witness
	var witness EncodingCircuit
	var toEncode1 [8]field.Element
	for i := 0; i < 8; i++ {
		toEncode1[i].SetRandom()
		witness.ToEncode1[i] = koalagnark.NewElementFromKoala(toEncode1[i])
	}
	var toEncode2 [12]field.Element
	for i := 0; i < 12; i++ {
		toEncode2[i].SetRandom()
		witness.ToEncode2[i] = koalagnark.NewElementFromKoala(toEncode2[i])
	}
	var toEncode3 fr.Element
	toEncode3.SetRandom()
	witness.ToEncode3 = toEncode3.String()

	r1 := EncodeKoalabearsToFrElement(toEncode1[:])
	witness.R1 = r1[0].String()
	r2 := EncodeKoalabearsToFrElement(toEncode2[:])
	witness.R2[0] = r2[0].String()
	witness.R2[1] = r2[1].String()
	r3 := EncodeFrElementToOctuplet(toEncode3)
	for i := 0; i < 8; i++ {
		witness.R3[i] = koalagnark.NewElementFromKoala(r3[i])
	}

	var circuit EncodingCircuit

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)
	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test Encode8WVsToFV with random values
func TestEncode8WVsToFV(t *testing.T) {
	var input [8]field.Element
	for i := 0; i < 8; i++ {
		input[i].SetRandom()
	}

	// Compute expected output using native encoding
	expected := EncodeKoalabearOctupletToFrElement(input)

	var circuit, witness Encode8WVsCircuit
	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test Encode9WVsToFV (BLS to Koalabear encoding)
func TestEncode9WVsToFV(t *testing.T) {
	// Create a random BLS element and decompose it
	var original fr.Element
	original.SetRandom()

	// Decompose to 9 koalabear elements
	decomposed := EncodeBLS12RootToKoalabear(original)

	// Reconstruct using native function
	reconstructed := DecodeKoalabearToBLS12Root(decomposed)

	// Verify native round-trip works
	assert.Equal(t, original, reconstructed, "Native round-trip should preserve value")

	// Now test circuit
	var circuit, witness Encode9WVsCircuit
	for i := 0; i < KoalabearChunks; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(decomposed[i])
	}
	witness.Output = reconstructed.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test round-trip encoding: BLS -> 9 Koalabear -> BLS
func TestRoundTripBLS9Koalabear(t *testing.T) {
	var original fr.Element
	original.SetRandom()

	decomposed := EncodeBLS12RootToKoalabear(original)

	var circuit, witness RoundTripBLS9KoalaCircuit
	witness.Original = original.String()
	for i := 0; i < KoalabearChunks; i++ {
		witness.Decomposed[i] = koalagnark.NewElementFromKoala(decomposed[i])
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test with zero values
func TestEncode8WVsToFVZeroValues(t *testing.T) {
	var input [8]field.Element
	for i := 0; i < 8; i++ {
		input[i].SetZero()
	}

	expected := EncodeKoalabearOctupletToFrElement(input)

	var circuit, witness ZeroValuesCircuit
	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test with max values (p-1)
func TestEncode8WVsToFVMaxValues(t *testing.T) {
	var input [8]field.Element
	for i := 0; i < 8; i++ {
		input[i] = *field.MaxVal
	}

	expected := EncodeKoalabearOctupletToFrElement(input)

	var circuit, witness MaxValuesCircuit
	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test with mixed zero and max values
func TestEncode8WVsToFVMixedValues(t *testing.T) {
	var input [8]field.Element
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			input[i].SetZero()
		} else {
			input[i] = *field.MaxVal
		}
	}

	expected := EncodeKoalabearOctupletToFrElement(input)

	var circuit, witness Encode8WVsCircuit
	for i := 0; i < 8; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test EncodeWVsToFVs with various input sizes
func TestEncodeWVsToFVsVariousSizes(t *testing.T) {
	testCases := []struct {
		name           string
		inputSize      int
		expectedOutput int
	}{
		{"size_1", 1, 1},
		{"size_4", 4, 1},
		{"size_8", 8, 1},
		{"size_9", 9, 2},
		{"size_16", 16, 2},
		{"size_17", 17, 3},
		{"size_24", 24, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := make([]field.Element, tc.inputSize)
			for i := 0; i < tc.inputSize; i++ {
				input[i].SetRandom()
			}

			// Compute expected output using native encoding
			expected := EncodeKoalabearsToFrElement(input)
			assert.Equal(t, tc.expectedOutput, len(expected), "Expected output length mismatch")

			var circuit, witness EncodeWVsToFVsCircuit
			circuit.Input = make([]koalagnark.Element, tc.inputSize)
			circuit.Output = make([]frontend.Variable, tc.expectedOutput)
			witness.Input = make([]koalagnark.Element, tc.inputSize)
			witness.Output = make([]frontend.Variable, tc.expectedOutput)

			for i := 0; i < tc.inputSize; i++ {
				witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
			}
			for i := 0; i < tc.expectedOutput; i++ {
				witness.Output[i] = expected[i].String()
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

// Test consistency between native and circuit encoding
func TestConsistencyNativeVsCircuit(t *testing.T) {
	// Run multiple iterations to catch any inconsistencies
	for iter := 0; iter < 10; iter++ {
		var input [8]field.Element
		for i := 0; i < 8; i++ {
			input[i].SetRandom()
		}

		// Native encoding
		nativeResult := EncodeKoalabearOctupletToFrElement(input)

		var circuit, witness ConsistencyCircuit
		for i := 0; i < 8; i++ {
			witness.KoalaInput[i] = koalagnark.NewElementFromKoala(input[i])
		}
		witness.FrOutput = nativeResult.String()

		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err, "Iteration %d failed", iter)
	}
}

// Test Encode9WVsToFV with zero values
func TestEncode9WVsToFVZeroValues(t *testing.T) {
	var input [KoalabearChunks]field.Element
	for i := 0; i < KoalabearChunks; i++ {
		input[i].SetZero()
	}

	expected := DecodeKoalabearToBLS12Root(input)

	var circuit, witness Encode9WVsCircuit
	for i := 0; i < KoalabearChunks; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test Encode9WVsToFV with specific known values
func TestEncode9WVsToFVKnownValues(t *testing.T) {
	// Test with small known values
	var input [KoalabearChunks]field.Element
	for i := 0; i < KoalabearChunks; i++ {
		input[i].SetUint64(uint64(i + 1))
	}

	expected := DecodeKoalabearToBLS12Root(input)

	var circuit, witness Encode9WVsCircuit
	for i := 0; i < KoalabearChunks; i++ {
		witness.Input[i] = koalagnark.NewElementFromKoala(input[i])
	}
	witness.Output = expected.String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Test multiple round trips to ensure stability
func TestMultipleRoundTrips(t *testing.T) {
	for iter := 0; iter < 5; iter++ {
		var original fr.Element
		original.SetRandom()

		// First round trip
		decomposed1 := EncodeBLS12RootToKoalabear(original)
		reconstructed1 := DecodeKoalabearToBLS12Root(decomposed1)

		// Second round trip
		decomposed2 := EncodeBLS12RootToKoalabear(reconstructed1)
		reconstructed2 := DecodeKoalabearToBLS12Root(decomposed2)

		assert.Equal(t, original, reconstructed1, "First round trip failed at iteration %d", iter)
		assert.Equal(t, reconstructed1, reconstructed2, "Second round trip failed at iteration %d", iter)

		// Test in circuit
		var circuit, witness RoundTripBLS9KoalaCircuit
		witness.Original = original.String()
		for i := 0; i < KoalabearChunks; i++ {
			witness.Decomposed[i] = koalagnark.NewElementFromKoala(decomposed1[i])
		}

		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err, "Circuit round trip failed at iteration %d", iter)
	}
}

// Test that encoding preserves ordering
func TestEncodingPreservesOrdering(t *testing.T) {
	// Create two different inputs where input1 < input2
	var input1, input2 [8]field.Element
	for i := 0; i < 8; i++ {
		input1[i].SetUint64(uint64(i))
		input2[i].SetUint64(uint64(i + 100))
	}

	result1 := EncodeKoalabearOctupletToFrElement(input1)
	result2 := EncodeKoalabearOctupletToFrElement(input2)

	// They should be different
	assert.NotEqual(t, result1, result2, "Different inputs should produce different outputs")

	// Test both in circuit
	var circuit1, witness1 Encode8WVsCircuit
	for i := 0; i < 8; i++ {
		witness1.Input[i] = koalagnark.NewElementFromKoala(input1[i])
	}
	witness1.Output = result1.String()

	ccs1, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit1)
	assert.NoError(t, err)
	fullWitness1, err := frontend.NewWitness(&witness1, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs1.IsSolved(fullWitness1)
	assert.NoError(t, err)

	var circuit2, witness2 Encode8WVsCircuit
	for i := 0; i < 8; i++ {
		witness2.Input[i] = koalagnark.NewElementFromKoala(input2[i])
	}
	witness2.Output = result2.String()

	ccs2, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit2)
	assert.NoError(t, err)
	fullWitness2, err := frontend.NewWitness(&witness2, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs2.IsSolved(fullWitness2)
	assert.NoError(t, err)
}

// Test single element encoding (edge case for EncodeWVsToFVs)
func TestEncodeWVsToFVsSingleElement(t *testing.T) {
	var input [1]field.Element
	input[0].SetRandom()

	expected := EncodeKoalabearsToFrElement(input[:])
	assert.Equal(t, 1, len(expected), "Single element should produce single output")

	var circuit, witness EncodeWVsToFVsCircuit
	circuit.Input = make([]koalagnark.Element, 1)
	circuit.Output = make([]frontend.Variable, 1)
	witness.Input = make([]koalagnark.Element, 1)
	witness.Output = make([]frontend.Variable, 1)

	witness.Input[0] = koalagnark.NewElementFromKoala(input[0])
	witness.Output[0] = expected[0].String()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// Benchmark for Encode8WVsToFV circuit compilation
func BenchmarkEncode8WVsToFVCompile(b *testing.B) {
	var circuit Encode8WVsCircuit
	for i := 0; i < b.N; i++ {
		_, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark for Encode9WVsToFV circuit compilation
func BenchmarkEncode9WVsToFVCompile(b *testing.B) {
	var circuit Encode9WVsCircuit
	for i := 0; i < b.N; i++ {
		_, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark for witness generation
func BenchmarkEncode8WVsToFVWitness(b *testing.B) {
	var input [8]field.Element
	for i := 0; i < 8; i++ {
		input[i].SetRandom()
	}
	expected := EncodeKoalabearOctupletToFrElement(input)

	var circuit Encode8WVsCircuit
	ccs, _ := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var witness Encode8WVsCircuit
		for j := 0; j < 8; j++ {
			witness.Input[j] = koalagnark.NewElementFromKoala(input[j])
		}
		witness.Output = expected.String()

		fullWitness, _ := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		_ = ccs.IsSolved(fullWitness)
	}
}

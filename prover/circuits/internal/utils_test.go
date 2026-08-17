package internal_test

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	gchash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/math/emulated"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	snarkTestUtils "github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecksumSubSlices(t *testing.T) {
	testChecksumSubSlices(t, 2, 1, 1)
	testChecksumSubSlices(t, 2, 1, 2)
	testChecksumSubSlices(t, 2, 2, 1, 1)
	testChecksumSubSlices(t, 8, 6, 1, 2, 3)
}

func testChecksumSubSlices(t *testing.T, bigSliceLength, lengthsSliceLength int, lengths ...int) {
	bigSliceInts := utils.RangeSlice[uint64](bigSliceLength)

	bigSliceBytes := internal.MapSlice(internal.Uint64To32Bytes, bigSliceInts...)
	endPoints := internal.PartialSumsInt(lengths)
	assert.LessOrEqual(t, endPoints[len(endPoints)-1], bigSliceLength)
	assert.LessOrEqual(t, len(lengths), lengthsSliceLength)

	hsh := gchash.POSEIDON2_BLS12_377.New()
	sums := make([]frontend.Variable, lengthsSliceLength)
	start := 0
	for i := range lengths {
		hsh.Reset()
		for j := start; j < endPoints[i]; j++ {
			hsh.Write(bigSliceBytes[j][:])
		}
		sums[i] = hsh.Sum(nil)
		start = endPoints[i]
	}

	endPointsSnark := make([]frontend.Variable, lengthsSliceLength)
	for n := utils.Copy(endPointsSnark, endPoints); n < lengthsSliceLength; n++ {
		endPointsSnark[n] = n - len(lengths) + 1
		sums[n] = n
	}

	t.Run(fmt.Sprintf("%d,%d,%v", bigSliceLength, lengthsSliceLength, lengths), snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		compressor, err := poseidon2permutation.NewCompressor(api)
		if err != nil {
			panic(err)
		}
		require.NoError(t, internal.MerkleDamgardChecksumSubSlices(api, compressor, 0, utils.ToVariableSlice(bigSliceInts),
			internal.VarSlice{
				Values: endPointsSnark,
				Length: len(lengths),
			}, sums))
		return nil
	}))
}

func TestConcat(t *testing.T) {
	snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		res := internal.Concat(api, 3, internal.VarSlice{[]frontend.Variable{2}, 1}, internal.VarSlice{[]frontend.Variable{3}, 0})
		return append(res.Values, res.Length)
	}, 2, 0, 0, 1)(t)
}

func TestReduceBytes(t *testing.T) {

	var cases [100][]byte
	b, err := utils.HexDecodeString("0x8de61d4d9c4891236da9646d464bd9b5757991e201678679f3f2abec6df666b8")
	assert.NoError(t, err)
	cases[0] = b

	for i := 1; i < len(cases); i++ {
		var b [32]byte
		_, err = rand.Read(b[:])
		assert.NoError(t, err)
		cases[i] = b[:]
	}

	reduced := make([][]byte, len(cases))
	var x bn254fr.Element
	for i := range cases {
		x.SetBytes(cases[i])
		reducedI := x.Bytes()
		reduced[i] = reducedI[:]
	}

	snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		for i := range cases {
			got := utils.ReduceBytes[emulated.BN254Fr](api, utils.ToVariableSlice(cases[i]))
			internal.AssertSliceEquals(api,
				got,
				utils.ToVariableSlice(reduced[i]),
			)
		}

		return nil
	})(t)
}

func TestPartitionSliceEmulated(t *testing.T) {
	selectors := []int{1, 0, 2, 2, 1}

	s := make([]fr381.Element, len(selectors))
	for i := range s {
		_, err := s[i].SetRandom()
		assert.NoError(t, err)
	}

	subs := make([][]fr381.Element, 3)
	for i := range subs {
		subs[i] = make([]fr381.Element, 0, len(selectors)-1)
	}

	for i := range s {
		subs[selectors[i]] = append(subs[selectors[i]], s[i])
	}

	snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {

		field, err := emulated.NewField[emulated.BLS12381Fr](api)
		assert.NoError(t, err)

		// convert randomized elements to emulated
		sEmulated := elementsToEmulated(field, s)

		subsEmulatedExpected := internal.MapSlice(func(s []fr381.Element) []emulated.Element[emulated.BLS12381Fr] {
			return elementsToEmulated(field, append(s, make([]fr381.Element, cap(s)-len(s))...)) // pad with zeros to see if padding is done correctly
		}, subs...)

		subsEmulated := internal.PartitionSliceEmulated(api, sEmulated, utils.ToVariableSlice(selectors), internal.MapSlice(func(s []fr381.Element) int { return cap(s) }, subs...)...)

		assert.Equal(t, len(subsEmulatedExpected), len(subsEmulated))
		for i := range subsEmulated {
			assert.Equal(t, len(subsEmulatedExpected[i]), len(subsEmulated[i]))
			for j := range subsEmulated[i] {
				field.AssertIsEqual(&subsEmulated[i][j], &subsEmulatedExpected[i][j])
			}
		}

		return nil
	})(t)
}

func elementsToEmulated(field *emulated.Field[emulated.BLS12381Fr], s []fr381.Element) []emulated.Element[emulated.BLS12381Fr] {
	return internal.MapSlice(func(element fr381.Element) emulated.Element[emulated.BLS12381Fr] {
		return *field.NewElement(internal.MapSlice(func(x uint64) frontend.Variable { return x }, element[:]...))
	}, s...)
}

func TestPartitionSlice(t *testing.T) {
	const (
		nbSubs   = 3
		sliceLen = 10
	)

	test := func(slice []frontend.Variable, selectors []int, subsSlack []int) func(*testing.T) {
		assert.Equal(t, len(selectors), len(slice))
		assert.Equal(t, len(subsSlack), nbSubs)

		subs := make([][]frontend.Variable, nbSubs)
		for j := range subs {
			subs[j] = make([]frontend.Variable, 0, sliceLen)
		}

		for j := range slice {
			subs[selectors[j]] = append(subs[selectors[j]], slice[j])
		}

		for j := range subs {
			subs[j] = append(subs[j], utils.ToVariableSlice(make([]int, subsSlack[j]))...) // add some padding
		}

		return snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {

			slice := utils.ToVariableSlice(slice)

			subsEncountered := internal.MapSlice(func(s []frontend.Variable) []frontend.Variable { return make([]frontend.Variable, len(s)) }, subs...)
			internal.PartitionSlice(api, slice, utils.ToVariableSlice(selectors), subsEncountered...)

			assert.Equal(t, len(subs), len(subsEncountered))
			for j := range subsEncountered {
				internal.AssertSliceEquals(api, subsEncountered[j], subs[j])
			}

			return nil
		})
	}

	test([]frontend.Variable{5}, []int{2}, []int{1, 0, 0})(t)
	test([]frontend.Variable{1, 2, 3}, []int{0, 1, 2}, []int{0, 0, 0})
	test(utils.ToVariableSlice(utils.RangeSlice[int](10)), []int{0, 1, 2, 0, 0, 0, 1, 1, 1, 2}, []int{0, 0, 0})

	for i := 0; i < 200; i++ {

		slice := make([]frontend.Variable, sliceLen)
		for j := range slice {
			var x fr377.Element
			_, err := x.SetRandom()
			slice[j] = &x
			assert.NoError(t, err)
		}

		selectors := test_utils.RandIntSliceN(sliceLen, nbSubs)
		subsSlack := test_utils.RandIntSliceN(nbSubs, 2)

		t.Run(fmt.Sprintf("selectors=%v,slack=%v", selectors, subsSlack), test(slice, selectors, subsSlack))
	}
}

// toCrumbsWrapperCircuit wraps a single ToCrumbs call so we can test soundness.
type toCrumbsWrapperCircuit struct {
	V frontend.Variable
}

func (c *toCrumbsWrapperCircuit) Define(api frontend.API) error {
	internal.ToCrumbs(api, c.V, 64)
	return nil
}

// TestToCrumbsRecompositionEnforced verifies that ToCrumbs rejects a forged hint
// whose crumbs do not recompose to the input value.
func TestToCrumbsRecompositionEnforced(t *testing.T) {
	internal.RegisterHints()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &toCrumbsWrapperCircuit{})
	require.NoError(t, err)

	witness, err := frontend.NewWitness(&toCrumbsWrapperCircuit{V: 0}, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	// Honest hint (all-zero crumbs for V=0) must be accepted.
	require.NoError(t, ccs.IsSolved(witness), "honest all-zero crumbs for V=0 must be accepted")

	// Forged hint: crumbs[0]=1 makes sum=1 ≠ V=0; must be rejected.
	hintID := findToCrumbsHintID(t)
	forgedHint := func(_ *big.Int, _ []*big.Int, outputs []*big.Int) error {
		for i := range outputs {
			outputs[i].SetUint64(0)
		}
		outputs[0].SetUint64(1)
		return nil
	}
	require.Error(
		t,
		ccs.IsSolved(witness, solver.OverrideHint(hintID, forgedHint)),
		"crumbs summing to 1 must be rejected when V=0",
	)
}

func findToCrumbsHintID(t *testing.T) solver.HintID {
	t.Helper()
	const name = "github.com/consensys/linea-monorepo/prover/circuits/internal.toCrumbsHint"
	for _, hintFn := range solver.GetRegisteredHints() {
		if solver.GetHintName(hintFn) == name {
			return solver.GetHintID(hintFn)
		}
	}
	t.Fatalf("hint %q is not registered", name)
	return 0
}

type divEuclideanWrapperCircuit struct {
	A, B frontend.Variable
}

func (c *divEuclideanWrapperCircuit) Define(api frontend.API) error {
	internal.DivEuclidean(api, c.A, c.B)
	return nil
}

// TestDivEuclideanRecompositionEnforced verifies that DivEuclidean rejects a
// forged hint whose (quotient, remainder) do not satisfy a = q*b + r, even when
// the bound constraints (r < b and q <= a) are individually satisfied.
func TestDivEuclideanRecompositionEnforced(t *testing.T) {
	internal.RegisterHints()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &divEuclideanWrapperCircuit{})
	require.NoError(t, err)

	// a=10, b=3: honest (q=3, r=1).
	witness, err := frontend.NewWitness(&divEuclideanWrapperCircuit{A: 10, B: 3}, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	require.NoError(t, ccs.IsSolved(witness), "honest (q=3, r=1) for a=10, b=3 must be accepted")

	hintID := findDivEuclideanHintID(t)

	// Forged (q=0, r=0): r < b and q <= a both hold, but 0*3 + 0 ≠ 10.
	forgedZero := func(_ *big.Int, _ []*big.Int, outputs []*big.Int) error {
		outputs[0].SetUint64(0)
		outputs[1].SetUint64(0)
		return nil
	}
	require.Error(
		t,
		ccs.IsSolved(witness, solver.OverrideHint(hintID, forgedZero)),
		"forged (q=0, r=0) for a=10, b=3 must be rejected",
	)

	// Forged (q=10, r=0): r < b and q <= a both hold, but 10*3 + 0 ≠ 10.
	forgedMax := func(_ *big.Int, _ []*big.Int, outputs []*big.Int) error {
		outputs[0].SetUint64(10)
		outputs[1].SetUint64(0)
		return nil
	}
	require.Error(
		t,
		ccs.IsSolved(witness, solver.OverrideHint(hintID, forgedMax)),
		"forged (q=10, r=0) for a=10, b=3 must be rejected",
	)
}

func findDivEuclideanHintID(t *testing.T) solver.HintID {
	t.Helper()
	const name = "github.com/consensys/linea-monorepo/prover/circuits/internal.divEuclideanHint"
	for _, hintFn := range solver.GetRegisteredHints() {
		if solver.GetHintName(hintFn) == name {
			return solver.GetHintID(hintFn)
		}
	}
	t.Fatalf("hint %q is not registered", name)
	return 0
}

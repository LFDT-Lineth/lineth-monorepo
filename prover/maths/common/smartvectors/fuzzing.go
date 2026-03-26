package smartvectors

import (
	"fmt"
	"math/big"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type smartVecType int

// The order matters : combining type x with type y implies that the result
// will be of type max(x, y)
const (
	constantT smartVecType = iota
	windowT
	regularT
	rotatedT
)

var smartVecTypeList = []smartVecType{constantT, windowT, regularT, rotatedT}

type testCase struct {
	name            string
	svecs           []SmartVector
	coeffs          []int
	expectedValue   SmartVector
	evaluationPoint field.Element // Only used for polynomial evaluation
}

func (tc testCase) String() string {
	res := "Testcase:\n"
	res += "\tSVECS:\n"
	for i := range tc.svecs {
		res += fmt.Sprintf("\t\t %v : %v\n", i, tc.svecs[i].Pretty())
	}
	res += fmt.Sprintf("\tCOEFFs: %v\n", tc.coeffs)
	res += fmt.Sprintf("\tEXPECTED_VALUE: %v\n", tc.expectedValue.Pretty())
	return res
}

type testCaseGen struct {
	// Randomness parameters
	seed int
	gen  *rand.Rand
	// Length and number of target vectors
	fullLen, numVec int
	// Parameters relevant for creating windows. This enforces the windows
	// to be included in a certain (which can possible roll over fullLen)
	windowWithLen        int
	windowMustStartAfter int
	// Allowed smart-vector types for this testcase
	allowedTypes []smartVecType
}

func newTestBuilder(seed int) *testCaseGen {
	// Use a deterministic randomness source
	res := &testCaseGen{seed: seed}
	// #nosec G404 --we don't need a cryptographic RNG for fuzzing purpose
	res.gen = rand.New(utils.NewRandSource(int64(seed)))

	// We should have some quarantee that the length is not too small
	// for the test generation
	res.fullLen = 1 << (res.gen.IntN(5) + 3)
	res.numVec = res.gen.IntN(8) + 1

	// In the test, we may restrict the inputs vectors to have a certain type
	allowedTypes := append([]smartVecType{}, smartVecTypeList...)
	res.gen.Shuffle(len(allowedTypes), func(i, j int) {
		allowedTypes[i], allowedTypes[j] = allowedTypes[j], allowedTypes[i]
	})
	res.allowedTypes = allowedTypes[:res.gen.IntN(len(allowedTypes)-1)+1]

	// Generating the window : it should be roughly half of the total length
	// this aims at maximizing the coverage.
	res.windowWithLen = res.gen.IntN(res.fullLen-4)/2 + 2
	res.windowMustStartAfter = res.gen.IntN(res.fullLen)
	return res
}

func (gen *testCaseGen) NewTestCaseForProd() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-prod", gen.seed)
	tcase.svecs = make([]SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)

	// resVal will contain the value of the repeated in the expected result
	// we will compute its value as we instantiate test vectors.
	resVal := field.One()
	maxType := constantT

	// For the windows, we need to track the dimension of the windows
	winMinStart := gen.fullLen
	winMaxStop := 0

	// Has constant vec keeps track of the case where we incluse a constant
	// vector equal to zero in the testcases
	hasConstZero := false

	for i := 0; i < gen.numVec; i++ {
		// Generate one by one the different vectors
		val := gen.genValue()
		tcase.coeffs[i] = gen.gen.IntN(5)
		chosenType := gen.allowedTypes[gen.gen.IntN(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		// Update the expected res value
		var tmp field.Element
		tmp.Exp(val, big.NewInt(int64(tcase.coeffs[i])))
		resVal.Mul(&resVal, &tmp)

		switch chosenType {
		case constantT:
			// Our implementation uses the convention that 0^0 == 0
			// Even though, this case is avoided by the calling code.
			if val.IsZero() && tcase.coeffs[i] != 0 {
				hasConstZero = true
			}
			tcase.svecs[i] = NewConstant(val, gen.fullLen)
		case windowT:
			v := gen.genWindow(val, val)
			tcase.svecs[i] = v
			start := normalize(v.interval().Start(), gen.windowMustStartAfter, gen.fullLen)
			winMinStart = utils.Min(winMinStart, start)
			stop := normalize(v.interval().Stop(), gen.windowMustStartAfter, gen.fullLen)
			if stop < start {
				stop += gen.fullLen
			}
			winMaxStop = utils.Max(winMaxStop, stop)
		case regularT:
			tcase.svecs[i] = gen.genRegular(val)
		case rotatedT:
			tcase.svecs[i] = gen.genRotated(val)
		default:
			utils.Panic("unexpected type %T", chosenType)
		}
	}

	// If there are no windows, then the initial condition that we use
	// do pass this sanity-check
	if winMaxStop-winMinStart > gen.windowWithLen {
		utils.Panic("inconsistent window dimension %v %v with gen %++v", winMinStart, winMaxStop, gen)
	}

	// This switch statement resolves the type of smart-vector that we are
	// expected for the result. It crucially relies on the number associated
	// to the variants of the smartVecTypes enum.
	switch {
	case hasConstZero:
		tcase.expectedValue = NewConstant(field.Zero(), gen.fullLen)
	case maxType == constantT:
		tcase.expectedValue = NewConstant(resVal, gen.fullLen)
	case maxType == windowT:
		tcase.expectedValue = NewPaddedCircularWindow(
			vector.Repeat(resVal, winMaxStop-winMinStart),
			resVal,
			normalize(winMinStart, -gen.windowMustStartAfter, gen.fullLen),
			gen.fullLen,
		)
	case maxType == regularT || maxType == rotatedT:
		tcase.expectedValue = NewRegular(vector.Repeat(resVal, gen.fullLen))
	default:
		panic("unexpected case")
	}

	return tcase
}

func (gen *testCaseGen) NewTestCaseForLinComb() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-lincomb", gen.seed)
	tcase.svecs = make([]SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)

	// resVal will contain the value of the repeated in the expected result
	// we will compute its value as we instantiate test vectors.
	resVal := field.Zero()
	maxType := constantT

	// For the windows, we need to track the dimension of the windows
	winMinStart := gen.fullLen
	winMaxStop := 0

	for i := 0; i < gen.numVec; i++ {
		// Generate one by one the different vectors
		val := gen.genValue()
		tcase.coeffs[i] = gen.gen.IntN(10) - 5
		chosenType := gen.allowedTypes[gen.gen.IntN(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		// Update the expected res value
		var tmp, coeffField field.Element
		coeffField.SetInt64(int64(tcase.coeffs[i]))
		tmp.Mul(&val, &coeffField)
		resVal.Add(&resVal, &tmp)

		switch chosenType {
		case constantT:
			tcase.svecs[i] = NewConstant(val, gen.fullLen)
		case windowT:
			v := gen.genWindow(val, val)
			tcase.svecs[i] = v
			start := normalize(v.interval().Start(), gen.windowMustStartAfter, gen.fullLen)
			winMinStart = utils.Min(winMinStart, start)

			stop := normalize(v.interval().Stop(), gen.windowMustStartAfter, gen.fullLen)
			if stop < start {
				stop += gen.fullLen
			}
			winMaxStop = utils.Max(winMaxStop, stop)
		case regularT:
			tcase.svecs[i] = gen.genRegular(val)
		case rotatedT:
			tcase.svecs[i] = gen.genRotated(val)
		default:
			utils.Panic("unexpected type %T", chosenType)
		}
	}

	// If there are no windows, then the initial condition that we use
	// do pass this sanity-check
	if winMaxStop-winMinStart > gen.windowWithLen {
		utils.Panic("inconsistent window dimension %v %v with gen %++v", winMinStart, winMaxStop, gen)
	}

	switch {
	case maxType == constantT:
		tcase.expectedValue = NewConstant(resVal, gen.fullLen)
	case maxType == windowT:
		tcase.expectedValue = NewPaddedCircularWindow(
			vector.Repeat(resVal, winMaxStop-winMinStart),
			resVal,
			normalize(winMinStart, -gen.windowMustStartAfter, gen.fullLen),
			gen.fullLen,
		)
	case maxType == regularT || maxType == rotatedT:
		tcase.expectedValue = NewRegular(vector.Repeat(resVal, gen.fullLen))
	default:
		panic("unexpected case")
	}

	return tcase
}

func (gen *testCaseGen) NewTestCaseForLinearCombination() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-poly-eval", gen.seed)
	tcase.svecs = make([]SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)
	tcase.evaluationPoint.SetRandom()
	x := tcase.evaluationPoint
	vals := []field.Element{}

	// MaxType is used to determine what type should the result be
	maxType := constantT

	// For the windows, we need to track the dimension of the windows
	winMinStart := gen.fullLen
	winMaxStop := 0

	for i := 0; i < gen.numVec; i++ {
		// Generate one by one the different vectors
		val := gen.genValue()
		vals = append(vals, val)
		tcase.coeffs[i] = gen.gen.IntN(10) - 5
		chosenType := gen.allowedTypes[gen.gen.IntN(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		switch chosenType {
		case constantT:
			tcase.svecs[i] = NewConstant(val, gen.fullLen)
		case windowT:
			v := gen.genWindow(val, val)
			tcase.svecs[i] = v
			start := normalize(v.interval().Start(), gen.windowMustStartAfter, gen.fullLen)
			winMinStart = utils.Min(winMinStart, start)

			stop := normalize(v.interval().Stop(), gen.windowMustStartAfter, gen.fullLen)
			if stop < start {
				stop += gen.fullLen
			}
			winMaxStop = utils.Max(winMaxStop, stop)
		case regularT:
			tcase.svecs[i] = gen.genRegular(val)
		case rotatedT:
			tcase.svecs[i] = gen.genRotated(val)
		default:
			utils.Panic("unexpected type %T", chosenType)
		}
	}

	// If there are no windows, then the initial condition that we use
	// do pass this sanity-check
	if winMaxStop-winMinStart > gen.windowWithLen {
		utils.Panic("inconsistent window dimension %v %v with gen %++v", winMinStart, winMaxStop, gen)
	}

	resVal := poly.Eval(vals, x)

	switch {
	case maxType == constantT:
		tcase.expectedValue = NewConstant(resVal, gen.fullLen)
	case maxType == regularT || maxType == windowT || maxType == rotatedT:
		tcase.expectedValue = NewRegular(vector.Repeat(resVal, gen.fullLen))
	default:
		panic("unexpected case")
	}

	return tcase
}

func (gen *testCaseGen) genValue() field.Element {
	// May increase the ceil of the generator to increase the probability to pick
	// an actually random value.
	switch gen.gen.IntN(4) {
	case 0:
		return field.Zero()
	case 1:
		return field.One()
	default:
		return field.NewElement(uint64(gen.gen.Uint64()))
	}

}

func (gen *testCaseGen) genWindow(val, paddingVal field.Element) *PaddedCircularWindow {
	start := gen.windowMustStartAfter + gen.gen.IntN(gen.windowWithLen)/2
	maxStop := gen.windowWithLen + gen.windowMustStartAfter
	winLen := gen.gen.IntN(maxStop - start)
	if winLen == 0 {
		winLen = 1
	}
	return NewPaddedCircularWindow(vector.Repeat(val, winLen), paddingVal, start, gen.fullLen)
}

func (gen *testCaseGen) genRegular(val field.Element) *Regular {
	return NewRegular(vector.Repeat(val, gen.fullLen))
}

func (gen *testCaseGen) genRotated(val field.Element) *Rotated {
	offset := gen.gen.IntN(gen.fullLen)
	return NewRotated(*gen.genRegular(val), offset)
}

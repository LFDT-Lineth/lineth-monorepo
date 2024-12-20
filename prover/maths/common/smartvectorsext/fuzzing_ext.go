package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/polyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"math/big"
	"math/rand"

	"github.com/consensys/linea-monorepo/prover/utils"
)

type smartVecType int

// The order matters : combining type x with type y implies that the result
// will be of type max(x, y)
const (
	constantT smartVecType = iota
	windowT
	RegularExtT
	RotatedExtT
)

var smartVecTypeList = []smartVecType{constantT, windowT, RegularExtT, RotatedExtT}

type testCase struct {
	name            string
	svecs           []smartvectors.SmartVector
	coeffs          []int
	expectedValue   smartvectors.SmartVector
	evaluationPoint fext.Element // Only used for polynomial evaluation
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
	res.gen = rand.New(rand.NewSource(int64(seed)))

	// We should have some quarantee that the length is not too small
	// for the test generation
	res.fullLen = 1 << (res.gen.Intn(5) + 3)
	res.numVec = res.gen.Intn(8) + 1

	// In the test, we may restrict the inputs vectors to have a certain type
	allowedTypes := append([]smartVecType{}, smartVecTypeList...)
	res.gen.Shuffle(len(allowedTypes), func(i, j int) {
		allowedTypes[i], allowedTypes[j] = allowedTypes[j], allowedTypes[i]
	})
	res.allowedTypes = allowedTypes[:res.gen.Intn(len(allowedTypes)-1)+1]

	// Generating the window : it should be roughly half of the total length
	// this aims at maximizing the coverage.
	res.windowWithLen = res.gen.Intn(res.fullLen-4)/2 + 2
	res.windowMustStartAfter = res.gen.Intn(res.fullLen)
	return res
}

func (gen *testCaseGen) NewTestCaseForProd() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-prod", gen.seed)
	tcase.svecs = make([]smartvectors.SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)

	// resVal will contain the value of the repeated in the expected result
	// we will compute its value as we instantiate test vectors.
	resVal := fext.One()
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
		tcase.coeffs[i] = gen.gen.Intn(5)
		chosenType := gen.allowedTypes[gen.gen.Intn(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		// Update the expected res value
		var tmp fext.Element
		tmp.Exp(val, big.NewInt(int64(tcase.coeffs[i])))
		resVal.Mul(&resVal, &tmp)

		switch chosenType {
		case constantT:
			// Our implementation uses the convention that 0^0 == 0
			// Even though, this case is avoided by the calling code.
			if val.IsZero() && tcase.coeffs[i] != 0 {
				hasConstZero = true
			}
			tcase.svecs[i] = NewConstantExt(val, gen.fullLen)
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
		case RegularExtT:
			tcase.svecs[i] = gen.genRegularExt(val)
		case RotatedExtT:
			tcase.svecs[i] = gen.genRotatedExt(val)
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
		tcase.expectedValue = NewConstantExt(fext.Zero(), gen.fullLen)
	case maxType == constantT:
		tcase.expectedValue = NewConstantExt(resVal, gen.fullLen)
	case maxType == windowT:
		tcase.expectedValue = NewPaddedCircularWindowExt(
			vectorext.Repeat(resVal, winMaxStop-winMinStart),
			resVal,
			normalize(winMinStart, -gen.windowMustStartAfter, gen.fullLen),
			gen.fullLen,
		)
	case maxType == RegularExtT || maxType == RotatedExtT:
		tcase.expectedValue = NewRegularExt(vectorext.Repeat(resVal, gen.fullLen))
	}

	return tcase
}

func (gen *testCaseGen) NewTestCaseForLinComb() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-lincomb", gen.seed)
	tcase.svecs = make([]smartvectors.SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)

	// resVal will contain the value of the repeated in the expected result
	// we will compute its value as we instantiate test vectors.
	resVal := fext.Zero()
	maxType := constantT

	// For the windows, we need to track the dimension of the windows
	winMinStart := gen.fullLen
	winMaxStop := 0

	for i := 0; i < gen.numVec; i++ {
		// Generate one by one the different vectors
		val := gen.genValue()
		tcase.coeffs[i] = gen.gen.Intn(10) - 5
		chosenType := gen.allowedTypes[gen.gen.Intn(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		// Update the expected res value
		var tmp, coeffField fext.Element
		coeffField.SetInt64(int64(tcase.coeffs[i]))
		tmp.Mul(&val, &coeffField)
		resVal.Add(&resVal, &tmp)

		switch chosenType {
		case constantT:
			tcase.svecs[i] = NewConstantExt(val, gen.fullLen)
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
		case RegularExtT:
			tcase.svecs[i] = gen.genRegularExt(val)
		case RotatedExtT:
			tcase.svecs[i] = gen.genRotatedExt(val)
		}
	}

	// If there are no windows, then the initial condition that we use
	// do pass this sanity-check
	if winMaxStop-winMinStart > gen.windowWithLen {
		utils.Panic("inconsistent window dimension %v %v with gen %++v", winMinStart, winMaxStop, gen)
	}

	switch {
	case maxType == constantT:
		tcase.expectedValue = NewConstantExt(resVal, gen.fullLen)
	case maxType == windowT:
		tcase.expectedValue = NewPaddedCircularWindowExt(
			vectorext.Repeat(resVal, winMaxStop-winMinStart),
			resVal,
			normalize(winMinStart, -gen.windowMustStartAfter, gen.fullLen),
			gen.fullLen,
		)
	case maxType == RegularExtT || maxType == RotatedExtT:
		tcase.expectedValue = NewRegularExt(vectorext.Repeat(resVal, gen.fullLen))
	}

	return tcase
}

func (gen *testCaseGen) NewTestCaseForPolyEval() (tcase testCase) {

	tcase.name = fmt.Sprintf("fuzzy-with-seed-%v-poly-eval", gen.seed)
	tcase.svecs = make([]smartvectors.SmartVector, gen.numVec)
	tcase.coeffs = make([]int, gen.numVec)
	tcase.evaluationPoint.SetRandom()
	x := tcase.evaluationPoint
	vals := []fext.Element{}

	// MaxType is used to determine what type should the result be
	maxType := constantT

	// For the windows, we need to track the dimension of the windows
	winMinStart := gen.fullLen
	winMaxStop := 0

	for i := 0; i < gen.numVec; i++ {
		// Generate one by one the different vectors
		val := gen.genValue()
		vals = append(vals, val)
		tcase.coeffs[i] = gen.gen.Intn(10) - 5
		chosenType := gen.allowedTypes[gen.gen.Intn(len(gen.allowedTypes))]
		maxType = utils.Max(maxType, chosenType)

		switch chosenType {
		case constantT:
			tcase.svecs[i] = NewConstantExt(val, gen.fullLen)
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
		case RegularExtT:
			tcase.svecs[i] = gen.genRegularExt(val)
		case RotatedExtT:
			tcase.svecs[i] = gen.genRotatedExt(val)
		}
	}

	// If there are no windows, then the initial condition that we use
	// do pass this sanity-check
	if winMaxStop-winMinStart > gen.windowWithLen {
		utils.Panic("inconsistent window dimension %v %v with gen %++v", winMinStart, winMaxStop, gen)
	}
	resVal := polyext.EvalUnivariate(vals, x)

	switch {
	case maxType == constantT:
		tcase.expectedValue = NewConstantExt(resVal, gen.fullLen)
	case maxType == RegularExtT || maxType == windowT || maxType == RotatedExtT:
		tcase.expectedValue = NewRegularExt(vectorext.Repeat(resVal, gen.fullLen))
	}

	return tcase
}

func (gen *testCaseGen) genValue() fext.Element {
	// May increase the ceil of the generator to increase the probability to pick
	// an actually random value.
	switch gen.gen.Intn(4) {
	case 0:
		return fext.Zero()
	case 1:
		return fext.One()
	default:
		return fext.NewElement(uint64(gen.gen.Uint64()), fieldPaddingInt())
	}

}

func (gen *testCaseGen) genWindow(val, paddingVal fext.Element) *PaddedCircularWindowExt {
	start := gen.windowMustStartAfter + gen.gen.Intn(gen.windowWithLen)/2
	maxStop := gen.windowWithLen + gen.windowMustStartAfter
	winLen := gen.gen.Intn(maxStop - start)
	if winLen == 0 {
		winLen = 1
	}
	return NewPaddedCircularWindowExt(vectorext.Repeat(val, winLen), paddingVal, start, gen.fullLen)
}

func (gen *testCaseGen) genRegularExt(val fext.Element) *RegularExt {
	return NewRegularExt(vectorext.Repeat(val, gen.fullLen))
}

func (gen *testCaseGen) genRotatedExt(val fext.Element) *RotatedExt {
	offset := gen.gen.Intn(gen.fullLen)
	return NewRotatedExt(*gen.genRegularExt(val), offset)
}

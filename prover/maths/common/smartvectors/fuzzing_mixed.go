package smartvectors

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/utils"
)

type smartVecTypeMixed int

// The order matters : combining type x with type y implies that the result
// will be of type max(x, y)
const (
	constantMixedT smartVecTypeMixed = iota
	windowMixedT
	RegularMixedT
	RotatedMixedT
)

type testCaseMixed struct {
	name            string
	svecs           []SmartVector
	coeffs          []int
	expectedValue   SmartVector
	evaluationPoint fext.Element // Only used for polynomial evaluation
}

func (tc testCaseMixed) String() string {
	res := "Testcase:\n"
	res += "\tSVECS:\n"
	for i := range tc.svecs {
		res += fmt.Sprintf("\t\t %v : %v\n", i, tc.svecs[i].Pretty())
	}
	res += fmt.Sprintf("\tCOEFFs: %v\n", tc.coeffs)
	res += fmt.Sprintf("\tEXPECTED_VALUE: %v\n", tc.expectedValue.Pretty())
	return res
}

func (gen *testCaseGen) NewTestCaseForLinearCombinationMixed() (tcase testCaseMixed) {

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
	resVal := vortex.EvalBasePolyHorner(vals, x)

	switch {
	case maxType == constantT:
		tcase.expectedValue = NewConstantExt(resVal, gen.fullLen)
	case maxType == regularT || maxType == windowT || maxType == rotatedT:
		tcase.expectedValue = NewRegularExt(vectorext.Repeat(resVal, gen.fullLen))
	default:
		panic("unexpected case")
	}

	return tcase
}

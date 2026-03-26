package innerproduct

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestInnerProduct(t *testing.T) {

	define := func(b *wizard.Builder) {
		for i, c := range testCases {
			bs := make([]ifaces.Column, len(c.bName))
			a := b.RegisterCommitExt(c.aName, c.size)
			for i, name := range c.bName {
				bs[i] = b.RegisterCommitExt(name, c.size)
			}
			b.InnerProduct(c.qName, a, bs...)
			// go to the next round
			_ = b.RegisterRandomCoin(coin.Namef("Coin_%v", i), coin.FieldExt)
		}
	}

	prover := func(run *wizard.ProverRuntime) {
		for j, c := range testCases {
			run.AssignColumn(c.aName, c.a)
			for i, name := range c.bName {
				run.AssignColumn(name, c.b[i])
			}

			run.AssignInnerProduct(c.qName, c.expected...)
			run.GetRandomCoinFieldExt(coin.Namef("Coin_%v", j))
		}
	}

	comp := wizard.Compile(define, Compile(), dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

func innerProductExt(a, b []fext.Element) fext.Element {

	if len(a) != len(b) {
		panic("<a, b> with len(a) != len(b)")
	}
	var res, tmp fext.Element
	for i := 0; i < len(a); i++ {
		tmp.Mul(&a[i], &b[i])
		res.Add(&res, &tmp)
	}
	return res

}

func innerProduct(a, b []field.Element) fext.Element {

	if len(a) != len(b) {
		panic("<a, b> with len(a) != len(b)")
	}
	var res, tmp field.Element
	for i := 0; i < len(a); i++ {
		tmp.Mul(&a[i], &b[i])
		res.Add(&res, &tmp)
	}
	return fext.Lift(res)
}

var testCases = []struct {
	qName    ifaces.QueryID
	aName    ifaces.ColID
	bName    []ifaces.ColID
	size     int
	a        smartvectors.SmartVector
	b        []smartvectors.SmartVector
	expected []fext.Element
}{
	// Single InnerProduct
	createMultiIPTestCase("Quey1", "ColA1", []ifaces.ColID{"ColB1"}, 4, 1, false),
	createMultiIPTestCase("Quey2", "ColA2", []ifaces.ColID{"ColB2"}, 4, 1, true),

	// Linear Combine Multiple InnerProducts
	createMultiIPTestCase("Quey3", "ColA3", []ifaces.ColID{"ColB3_0", "ColB3_1"}, 8, 2, false),
	createMultiIPTestCase("Quey4", "ColA4", []ifaces.ColID{"ColB4_0", "ColB4_1"}, 8, 2, true),
	createMultiIPTestCase("Quey5", "ColA5", []ifaces.ColID{"ColB5_0", "ColB5_1"}, 16, 2, false),
}

type testCase struct {
	qName    ifaces.QueryID
	aName    ifaces.ColID
	bName    []ifaces.ColID
	size     int
	a        smartvectors.SmartVector
	b        []smartvectors.SmartVector
	expected []fext.Element
}

// createMultiIPTestCase generates a testCase for multiple inner products (linear combination).
func createMultiIPTestCase(
	qName ifaces.QueryID,
	aName ifaces.ColID,
	bName []ifaces.ColID,
	size int,
	bRows int,
	isBase bool,
) testCase {
	if !isBase {
		aValues := vectorext.ForRandTestFromLen(size)
		aVec := smartvectors.NewRegularExt(aValues)
		bVec := make([]smartvectors.SmartVector, bRows)

		expected_Vec := make([]fext.Element, bRows)
		bValues := make([][]fext.Element, bRows)
		for i := 0; i < bRows; i++ {
			bValues[i] = vectorext.ForRandTestFromLen(size)
			bVec[i] = smartvectors.NewRegularExt(bValues[i])
			expected_Vec[i] = innerProductExt(aValues, bValues[i])
		}
		return testCase{
			qName:    qName,
			aName:    aName,
			bName:    bName,
			size:     size,
			a:        aVec,
			b:        bVec,
			expected: expected_Vec,
		}
	} else {
		aValues := vector.ForRandTestFromLen(size)
		aVec := smartvectors.NewRegular(aValues)
		bVec := make([]smartvectors.SmartVector, bRows)

		expected_Vec := make([]fext.Element, bRows)
		bValues := make([][]field.Element, bRows)
		for i := 0; i < bRows; i++ {
			bValues[i] = vector.ForRandTestFromLen(size)
			bVec[i] = smartvectors.NewRegular(bValues[i])
			expected_Vec[i] = innerProduct(aValues, bValues[i])

		}
		return testCase{
			qName:    qName,
			aName:    aName,
			bName:    bName,
			size:     size,
			a:        aVec,
			b:        bVec,
			expected: expected_Vec,
		}
	}

}

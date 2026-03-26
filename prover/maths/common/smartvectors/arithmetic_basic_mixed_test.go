package smartvectors

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

func TestLinearCombination(t *testing.T) {

	// sample polys
	nbPolys := 10
	sizePoly := 64
	polys := make([]SmartVector, nbPolys)
	for i := 0; i < nbPolys; i++ {
		polys[i] = Rand(sizePoly)
	}

	// extract the columns
	columns := make([]SmartVector, sizePoly)
	for i := 0; i < sizePoly; i++ {
		tmp := make([]field.Element, nbPolys)
		for j := 0; j < nbPolys; j++ {
			tmp[j] = polys[j].Get(i)
		}
		columns[i] = NewRegular(tmp)
	}

	// fold the columns using canonical eval
	randomCoin := fext.RandomElement()
	x := fext.RandomElement()
	manuallyFoldedColumns := make([]fext.Element, sizePoly)
	for i := 0; i < sizePoly; i++ {
		manuallyFoldedColumns[i] = EvalCoeffExt(columns[i], randomCoin)
	}
	svManuallyFoldedColumns := NewRegularExt(manuallyFoldedColumns)

	// fold the polys using LinearCombinationExt
	foldedPolys := LinearCombinationMixed(polys, randomCoin)

	// check equality of svManuallyFoldedColumns and foldedPolys, by evaluating them at x
	y := EvalCoeffExt(svManuallyFoldedColumns, x)
	yPrime := EvalCoeffExt(foldedPolys, x)

	if !y.Equal(&yPrime) {
		t.Fatal("LinearCombinationMixed")
	}
}

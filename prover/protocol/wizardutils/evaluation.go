package wizardutils

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// RandLinCombColSymbolic generates a symbolic expression representing a random
// linear combination of columns (hs) using a random coin value (x). It converts
// each column to a symbolic variable and returns a polynomial expression with
// the coin value and each columns as the variables.
func RandLinCombColSymbolic(x coin.Info, hs []ifaces.Column) *symbolic.Expression {
	cols := make([]*symbolic.Expression, len(hs))
	for c := range cols {
		cols[c] = ifaces.ColumnAsVariable(hs[c])
	}
	expr := symbolic.NewPolyEval(x.AsVariable(), cols)
	return expr
}

// RandLinCombColAssignment computes the runtime assignment of a linear combination
// of columns. It takes a wizard.ProverRuntime, a random coin value, and a slice of
// columns as input. The function iteratively multiplies each column's assignment
// by a cumulative product of the coin value and adds the results to a running total,
// effectively computing a weighted sum of the columns. The weights are powers of the
// coin value. The function returns the resulting linear combination as a
// [smartvectors.SmartVector].
func RandLinCombColAssignment(run *wizard.ProverRuntime, coinVal fext.Element, hs []ifaces.Column) smartvectors.SmartVector {
	n := len(hs)
	if n == 0 {
		panic("cannot compute random linear combination of zero columns")
	}

	size := hs[0].Size()
	vWitness := make(vectorext.Vector, size)

	// Use Horner's method to compute the linear combination:
	// result = (...((col[n-1]*x + col[n-2])*x + ...)*x + col[0]
	// This avoids computing powers of x explicitly and reduces memory writes
	// by accumulating in-place.

	// Initialize accumulator with the highest power term (hs[n-1])
	lastCol := hs[n-1].GetColAssignment(run)
	if r, ok := lastCol.(*smartvectors.RegularExt); ok {
		copy(vWitness, *r)
	} else {
		lastCol.WriteInSliceExt(vWitness)
	}

	var vColumn vectorext.Vector // Lazy allocation for scratch buffer

	for i := n - 2; i >= 0; i-- {
		vWitness.ScalarMul(vWitness, &coinVal)

		sv := hs[i].GetColAssignment(run)
		var src vectorext.Vector
		if r, ok := sv.(*smartvectors.RegularExt); ok {
			src = vectorext.Vector(*r)
		} else {
			if vColumn == nil {
				vColumn = make(vectorext.Vector, size)
			}
			sv.WriteInSliceExt(vColumn)
			src = vColumn
		}
		vWitness.Add(vWitness, src)
	}
	return smartvectors.NewRegularExt(vWitness)
}

// LinCombExpr generates a symbolic expression representing a linear combination
// the expression can be evaluated at point x over the columns hs via [column.EvalExprColumn].
func LinCombExpr(x int, hs []ifaces.Column) *symbolic.Expression {
	cols := make([]*symbolic.Expression, len(hs))
	xExpr := symbolic.NewConstant(int64(x))
	for c := range cols {
		cols[c] = ifaces.ColumnAsVariable(hs[c])
	}
	expr := symbolic.NewPolyEval(xExpr, cols)
	return expr
}

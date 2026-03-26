package wizardutils

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
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
func RandLinCombColAssignment(run *wizard.ProverRuntime, coinVal field.Element, hs []ifaces.Column) smartvectors.SmartVector {
	var colTableWit smartvectors.SmartVector
	var witnessCollapsed smartvectors.SmartVector
	x := field.One()
	witnessCollapsed = smartvectors.NewConstant(field.Zero(), hs[0].Size())
	for tableCol := range hs {
		colTableWit = hs[tableCol].GetColAssignment(run)
		witnessCollapsed = smartvectors.Add(witnessCollapsed, smartvectors.Mul(colTableWit, smartvectors.NewConstant(x, hs[0].Size())))
		x.Mul(&x, &coinVal)
	}
	return witnessCollapsed
}

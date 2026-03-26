package expr_handle

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// returns a column obtained as a random linear combinations of differents
// handles. If the function is provided zero column, then the linear combination
// will panic as it cannot return a zero column of unknonwn size.
func RandLinCombCol(comp *wizard.CompiledIOP, x ifaces.Accessor, hs []ifaces.Column, name ...string) ifaces.Column {

	if len(hs) == 0 {
		utils.Panic("RandLinCombCol requires at least one input column, as it cannot guess the size of the resulting column")
	}

	cols := make([]*symbolic.Expression, len(hs))
	for c := range cols {
		cols[c] = ifaces.ColumnAsVariable(hs[c])
	}
	expr := symbolic.NewPolyEval(x.AsVariable(), cols)
	return ExprHandle(comp, expr, name...)
}

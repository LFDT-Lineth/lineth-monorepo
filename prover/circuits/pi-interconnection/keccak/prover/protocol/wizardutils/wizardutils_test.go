package wizardutils

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/assert"
)

func TestWizarldutils(t *testing.T) {
	var res1, res11, res2, res22 *symbolic.Expression
	define := func(b *wizard.Builder) {
		var (
			size = 4
			col1 = b.RegisterCommit("P1", size)
			col2 = b.RegisterCommit("P2", size)

			col5 = b.RegisterPrecomputed("P3", smartvectors.ForTest(1, 0, 1, 1))
			col6 = verifiercol.NewConstantCol(field.NewElement(3), size, "")

			coin = b.RegisterRandomCoin(coin.Namef("Coin"), coin.Field)
		)

		// PolyEval over columns
		res1 = symbolic.NewPolyEval(coin.AsVariable(), []*symbolic.Expression{ifaces.ColumnAsVariable(col1), ifaces.ColumnAsVariable(col2)})
		res11 = linCom(coin.AsVariable(), []*symbolic.Expression{ifaces.ColumnAsVariable(col1), ifaces.ColumnAsVariable(col2)})

		// PolyEval over PolyEval and Mul.
		expr := symbolic.Mul(col6, col5, coin)
		res2 = symbolic.NewPolyEval(coin.AsVariable(), []*symbolic.Expression{res1, expr})
		res22 = linCom(coin.AsVariable(), []*symbolic.Expression{res1, expr})

	}
	prover := func(run *wizard.ProverRuntime) {
		var (
			col1 = smartvectors.ForTest(1, 2, 1, 0)
			col2 = smartvectors.ForTest(1, 1, 3, 1)
		)
		run.AssignColumn("P1", col1)
		run.AssignColumn("P2", col2)

		run.GetRandomCoinField(coin.Namef("Coin"))

		res1Wit := column.EvalExprColumn(run, res1.Board()).IntoRegVecSaveAlloc()
		res11Wit := column.EvalExprColumn(run, res11.Board()).IntoRegVecSaveAlloc()
		for i := range res11Wit {
			if res1Wit[i].Cmp(&res11Wit[i]) != 0 {
				panic("err")
			}
		}

		res2Wit := column.EvalExprColumn(run, res2.Board()).IntoRegVecSaveAlloc()
		res22Wit := column.EvalExprColumn(run, res22.Board()).IntoRegVecSaveAlloc()
		for i := range res11Wit {
			if res2Wit[i].Cmp(&res22Wit[i]) != 0 {
				panic("err")
			}
		}

	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

func linCom(x *symbolic.Expression, coeff []*symbolic.Expression) *symbolic.Expression {
	res := symbolic.NewConstant(0)
	for i := len(coeff) - 1; i >= 0; i-- {
		res = symbolic.Mul(res, x)
		res = symbolic.Add(res, coeff[i])
	}
	return res
}

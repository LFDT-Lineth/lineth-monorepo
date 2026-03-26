package simplify

import (
	"testing"

	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/assert"
)

// Set of common dummy variables. Useful for testing
var (
	a = sym.NewDummyVar("a")
	b = sym.NewDummyVar("b")
	c = sym.NewDummyVar("c")
	d = sym.NewDummyVar("d")
	e = sym.NewDummyVar("e")
)

// Build expressions and checks that cost-stats works properly on them
func TestCostStat(t *testing.T) {

	testcases := []struct {
		Expr      *sym.Expression
		CostStats costStats
	}{
		{
			Expr: a.Mul(b),
			CostStats: costStats{
				NumAdd: 0,
				NumMul: 1,
			},
		},
		{
			Expr: a.Add(b),
			CostStats: costStats{
				NumAdd: 1,
				NumMul: 0,
			},
		},
		{
			Expr: a.Mul(b).Add(a),
			CostStats: costStats{
				NumMul: 1,
				NumAdd: 1,
			},
		},
		{
			Expr: a.Mul(b).Mul(c).Mul(d).Mul(e),
			CostStats: costStats{
				NumAdd: 0,
				NumMul: 4,
			},
		},
		{
			Expr:      a.Mul(b).Mul(c).Add(a.Mul(b).Mul(c).Mul(d)),
			CostStats: costStats{NumAdd: 1, NumMul: 5},
		},
	}

	for _, c := range testcases {
		actual := evaluateCostStat(c.Expr)
		assert.Equalf(t, c.CostStats, actual, "wrong cost stat obtained")
	}

}

package simplify

import (
	"fmt"
	"testing"

	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFactored(t *testing.T) {

	testcases := []struct {
		Expr       *sym.Expression
		By         *sym.Expression
		IsFactored bool
		Factor     *sym.Expression
	}{
		{
			Expr:       sym.Mul(a, b, c),
			By:         sym.Mul(a, b),
			IsFactored: true,
			Factor:     c,
		},
		{
			Expr:       sym.Mul(a, b, c),
			By:         sym.Mul(a, c),
			IsFactored: true,
			Factor:     b,
		},
		{
			Expr:       sym.Mul(a, b, c),
			By:         sym.Mul(a, d),
			IsFactored: false,
			Factor:     nil,
		},
		{
			Expr:       sym.Add(a, b, c),
			By:         sym.Add(a, d),
			IsFactored: false,
			Factor:     nil,
		},
		{
			Expr:       sym.Add(a, b, c),
			By:         sym.Add(a, b, b),
			IsFactored: false,
			Factor:     nil,
		},
		{
			Expr:       sym.Mul(a, b),
			By:         sym.Mul(a, b),
			IsFactored: true,
			Factor:     sym.NewConstant(1),
		},
		{
			Expr:       sym.Mul(a, a),
			By:         a,
			IsFactored: true,
			Factor:     a,
		},
	}

	for i, tc := range testcases {

		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {
			// Build the group exponent map. If by is a product, we use directly
			// the exponents it contains. Otherwise, we say this is a single
			// term product with an exponent of 1.
			groupedExp := map[uint64]int{}
			if byProd, ok := tc.By.Operator.(sym.Product); ok {
				for i, ex := range byProd.Exponents {
					groupedExp[tc.By.Children[i].ESHash[0]] = ex
				}
			} else {
				groupedExp[tc.By.ESHash[0]] = 1
			}

			factored, isFactored := isFactored(tc.Expr, groupedExp)
			assert.Equalf(t, tc.IsFactored, isFactored, "missed factor identification")

			if isFactored && tc.IsFactored {
				assert.Equalf(t, tc.Factor.ESHash.String(), factored.ESHash.String(), "wrong factor")
			}
		})
	}
}

func TestFactorization(t *testing.T) {

	var (
		a = sym.NewDummyVar("a")
		b = sym.NewDummyVar("b")
		c = sym.NewDummyVar("c")
		d = sym.NewDummyVar("d")
	)

	testCases := []struct {
		Origin   *sym.Expression
		Factored *sym.Expression
	}{
		{
			Origin: sym.Add(
				sym.Mul(a, b),
				sym.Mul(a, c),
				sym.Mul(a, d),
			),
			Factored: sym.Mul(
				a,
				sym.Add(b, c, d),
			),
		},
		{
			Origin: sym.Add(
				sym.Mul(a, b, b),
				sym.Mul(a, b, c),
				sym.Mul(a, b, d),
			),
			Factored: sym.Mul(
				a,
				b,
				sym.Add(b, c, d),
			),
		},
		{
			Origin: sym.Add(
				sym.Mul(a, b),
				sym.Mul(a, b, c),
				sym.Mul(a, b, d),
			),
			Factored: sym.Mul(
				a,
				b,
				sym.Add(1, c, d),
			),
		},
		{
			Origin: sym.Add(
				sym.Mul(a, b),
				sym.Mul(a, c),
				sym.Mul(d),
			),
			Factored: sym.Add(
				sym.Mul(a, sym.Add(b, c)),
				d,
			),
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {
			factored := factorizeExpression(testCase.Origin, 10)
			require.Equal(t, testCase.Origin.ESHash.String(), factored.ESHash.String())
			require.NoError(t, factored.Validate())
			assert.Equal(t, evaluateCostStat(testCase.Factored), evaluateCostStat(factored))
		})
	}
}

func TestFactorLinCompFromGroup(t *testing.T) {

	testCases := []struct {
		LinComb *sym.Expression
		Group   []*sym.Expression
		Res     *sym.Expression
	}{
		{
			LinComb: sym.Add(
				sym.Mul(a, b),
				sym.Mul(a, b, c),
				sym.Mul(a, b, d),
			),
			Group: []*sym.Expression{a, b},
			Res: sym.Mul(
				a,
				b,
				sym.Add(1, c, d),
			),
		},
		{
			LinComb: sym.Add(
				sym.Mul(a, c),
				sym.Mul(a, a, b),
				1,
			),
			Group: []*sym.Expression{a},
			Res: sym.Add(
				sym.Mul(
					a,
					sym.Add(
						sym.Mul(a, b),
						c,
					),
				),
				1,
			),
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			group := map[uint64]*sym.Expression{}
			for _, e := range testCase.Group {
				group[e.ESHash[0]] = e
			}

			factored := factorLinCompFromGroup(testCase.LinComb, group)
			assert.Equal(t, testCase.LinComb.ESHash.String(), factored.ESHash.String())

			if t.Failed() {
				fmt.Printf("res=%v\n", testCase.Res.MarshalJSONString())
				fmt.Printf("factored=%v\n", factored.MarshalJSONString())
				t.Fatal()
			}

			require.NoError(t, factored.Validate())
			assert.Equal(t, evaluateCostStat(testCase.Res), evaluateCostStat(factored))

			if t.Failed() {
				fmt.Printf("res=%v\n", testCase.Res.MarshalJSONString())
				fmt.Printf("factored=%v\n", factored.MarshalJSONString())
			}
		})
	}

}

package functionals

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
)

const (
	EVAL_BIVARIATE_POLY                 string = "HORNER"
	EVAL_BIVARIATE_LOCAL_CONSTRAINT_END string = "LOCAL_CONSTRAINT_END"
	EVAL_BIVARIATE_FIXED_POINT_BEGIN    string = "FIXED_POINT_BEGIN"
	EVAL_BIVARIATE_GLOBAL               string = "GLOBAL"
)

type EvalBivariateProverAction struct {
	Name     string
	PCom     ifaces.Column
	X        ifaces.Accessor
	YXPow1mk XYPow1MinNAccessor
	Length   int
	NPowX    int
}

func (a *EvalBivariateProverAction) Run(assi *wizard.ProverRuntime) {
	xVal := a.X.GetVal(assi)
	yxPow1mkVal := a.YXPow1mk.GetVal(assi)
	p := a.PCom.GetColAssignment(assi)

	h := make([]field.Element, a.Length)
	h[a.Length-1] = p.Get(a.Length - 1)

	for i := a.Length - 2; i >= 0; i-- {
		pi := p.Get(i)
		if (i+1)%a.NPowX == 0 {
			h[i].Mul(&h[i+1], &yxPow1mkVal).Add(&h[i], &pi)
			continue
		}
		h[i].Mul(&h[i+1], &xVal).Add(&h[i], &pi)
	}

	assi.AssignColumn(ifaces.ColIDf("%v_%v", a.Name, EVAL_BIVARIATE_POLY), smartvectors.NewRegular(h))
	assi.AssignLocalPoint(ifaces.QueryIDf("%v_%v", a.Name, EVAL_BIVARIATE_FIXED_POINT_BEGIN), h[0])
}

/*
EvalCoeffBivariate creates a dedicated wizard to perform an evaluation in coefficient form
of a bivariate polynomial in x and y of degree respectively k and l.

Example : p commits to [1, 2, 3, 4, 5, 6, 7, 8]
and we want to evaluate p for (X=2, nPowX=4) (Y=3, nPowY=2)

This will return an accessor to value worth

	$\sum_{j=0}^{nPowY-1} Y^j \sum_{i=0}^{nPowX-1} X^i p_{i,j}
	= 1 + 2X + 3X^2 + 4X^3 + 5Y + 6XY + 7X^2Y + 8X^3Y = 376$
*/
func EvalCoeffBivariate(
	comp *wizard.CompiledIOP, name string,
	pCom ifaces.Column,
	x, y ifaces.Accessor,
	nPowX, nPowY int, // corresponds to the degrees + 1 in each variable respectively
) ifaces.Accessor {

	// sanity-check the dimensions of the vector
	if nPowX*nPowY != pCom.Size() {
		utils.Panic("mismatch in the dimensions (%v * %v) != %v", nPowX, nPowY, pCom.Size())
	}

	length := pCom.Size()
	maxRound := utils.Max(
		x.Round(),
		y.Round(),
		pCom.Round(),
	)

	hCom := comp.InsertCommit(
		maxRound,
		ifaces.ColIDf("%v_%v", name, EVAL_BIVARIATE_POLY),
		length,
	)

	/*
		Prepare a "bank" with all the variable we will need to encode the following constraints

		Global : H[i] = P[i] + H[i+1] (x + Zl,n[i-1] * (yx^{−k+1} - x))
		Local Constraint : H[−1]=P[−1]
		Local Opening : H[0] (will contain the result)
	*/

	h := ifaces.ColumnAsVariable(hCom)
	p := ifaces.ColumnAsVariable(pCom)
	hNext := ifaces.ColumnAsVariable(column.Shift(hCom, 1))
	zPrev := variables.NewPeriodicSample(nPowX, nPowX-1)
	x_v := x.AsVariable()

	yx_pow_1mk_acc := XYPow1MinNAccessor{X: x, Y: y, N: nPowX, AccessName: name}
	yx_pow_1mk := yx_pow_1mk_acc.AsVariable()
	h_prev := ifaces.ColumnAsVariable(column.Shift(hCom, -1))
	p_prev := ifaces.ColumnAsVariable(column.Shift(pCom, -1))

	/*
		start with the global constraints
		factor := x + Zl,n[i-1] * (yx^{−k+1} - x)
	*/
	factor := yx_pow_1mk.Sub(x_v).Mul(zPrev).Add(x_v)

	globalExpr := hNext.Mul(factor).Add(p).Sub(h)
	comp.InsertGlobal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_BIVARIATE_GLOBAL),
		globalExpr,
	)

	// Local constraint : p[-1] = h[-1]
	comp.InsertLocal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_BIVARIATE_LOCAL_CONSTRAINT_END),
		p_prev.Sub(h_prev),
	)

	// Local opening containing the result
	finalLocalOpening := comp.InsertLocalOpening(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_BIVARIATE_FIXED_POINT_BEGIN),
		hCom,
	)

	comp.RegisterProverAction(maxRound, &EvalBivariateProverAction{
		Name:     name,
		PCom:     pCom,
		X:        x,
		YXPow1mk: yx_pow_1mk_acc,
		Length:   length,
		NPowX:    nPowX,
	})

	return accessors.NewLocalOpeningAccessor(finalLocalOpening, maxRound)
}

// xYPower1MinAccessor implements [ifaces.Accessor] and computes X^(1-N) * Y
// where x and y are input accessors. It is exported so that the [wizard.CompiledIOP]
// serializer can access it.
type XYPow1MinNAccessor struct {
	// X and Y are the input accessors
	X, Y ifaces.Accessor
	// N is as in the formula
	N int
	// AccessName is used to derive a unique name to the accessor.s
	AccessName string
}

// String implements [symbolic.Metadata] and thus [ifaces.Accessor].
func (a *XYPow1MinNAccessor) String() string {
	return fmt.Sprintf("YX_POW_1_MIN_n_%v", a.AccessName)
}

// Name implements the [ifaces.Accessor] interface.
func (a *XYPow1MinNAccessor) Name() string {
	return a.String()
}

// GetVal implements the [ifaces.Accessor] interface.
func (a *XYPow1MinNAccessor) GetVal(run ifaces.Runtime) field.Element {
	x := a.X.GetVal(run)
	y := a.Y.GetVal(run)
	var res field.Element
	res.Exp(x, big.NewInt(int64(1-a.N)))
	res.Mul(&res, &y)
	return res
}

// GetFrontendVariable implements the [ifaces.Accessor] interface.
func (a *XYPow1MinNAccessor) GetFrontendVariable(api frontend.API, run ifaces.GnarkRuntime) frontend.Variable {
	x := a.X.GetFrontendVariable(api, run)
	y := a.Y.GetFrontendVariable(api, run)
	res := gnarkutil.Exp(api, x, 1-a.N)
	res = api.Mul(res, y)
	return res
}

// AsVariable implements the [ifaces.Accessor] interface.
func (a *XYPow1MinNAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(a)
}

// Round implements the [ifaces.Accessor] interface.
func (a *XYPow1MinNAccessor) Round() int {
	return utils.Max(a.X.Round(), a.Y.Round())
}

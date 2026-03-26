package permutation

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

// The verifier gets all the query openings and multiply them together and
// expect them to be one. It is represented by an array of ZCtx holding for
// the same round. (we have the guarantee that they come from the same query).
type VerifierCtx struct {
	Ctxs    []*ZCtx
	skipped bool
}

// Run implements the [wizard.VerifierAction] interface and checks that the
// product of the products given by the ZCtx is equal to one.
func (v *VerifierCtx) Run(run wizard.Runtime) error {

	mustBeOne := field.One()

	for _, zCtx := range v.Ctxs {
		for _, opening := range zCtx.ZOpenings {
			y := run.GetLocalPointEvalParams(opening.ID).Y
			mustBeOne.Mul(&mustBeOne, &y)
		}
	}

	if mustBeOne != field.One() {
		return errors.New("the permutation check compiler did not pass")
	}

	return nil
}

// Run implements the [wizard.VerifierAction] interface and is as
// [VerifierCtx.Run] but in the context of a gnark circuit.
func (v *VerifierCtx) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	mustBeOne := frontend.Variable(1)

	for _, zCtx := range v.Ctxs {
		for _, opening := range zCtx.ZOpenings {
			y := run.GetLocalPointEvalParams(opening.ID).Y
			mustBeOne = api.Mul(mustBeOne, y)
		}
	}

	api.AssertIsEqual(mustBeOne, frontend.Variable(1))
}

func (v *VerifierCtx) Skip() {
	v.skipped = true
}

func (v *VerifierCtx) IsSkipped() bool {
	return v.skipped
}

// CheckGrandProductIsOne is a verifier action checking that the grand product
// is one.
type CheckGrandProductIsOne struct {
	Query       *query.GrandProduct
	ExplicitNum []*symbolic.Expression
	ExplicitDen []*symbolic.Expression
	Skipped     bool
}

func (c *CheckGrandProductIsOne) Run(run wizard.Runtime) error {

	var (
		y = run.GetGrandProductParams(c.Query.ID).Y
		d = field.One()
	)

	for _, e := range c.ExplicitNum {

		var (
			col = column.EvalExprColumn(run, e.Board()).IntoRegVecSaveAlloc()
			tmp = field.One()
		)

		for i := range col {
			tmp.Mul(&tmp, &col[i])
		}

		y.Mul(&y, &tmp)
	}

	for _, e := range c.ExplicitDen {

		var (
			col = column.EvalExprColumn(run, e.Board()).IntoRegVecSaveAlloc()
			tmp = field.One()
		)

		for i := range col {
			tmp.Mul(&tmp, &col[i])
		}

		d.Mul(&d, &tmp)
	}

	y.Div(&y, &d)

	if !y.IsOne() {
		return fmt.Errorf("[CheckGrandProductIsOne -> GrandProduct] the outcome of the grand-product query should be one")
	}
	return nil
}

func (c *CheckGrandProductIsOne) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	var (
		y = run.GetGrandProductParams(c.Query.ID).Prod
		d = frontend.Variable(1)
	)

	for _, e := range c.ExplicitNum {

		var (
			col = column.GnarkEvalExprColumn(api, run, e.Board())
			tmp = frontend.Variable(1)
		)

		for i := range col {
			tmp = api.Mul(tmp, col[i])
		}

		y = api.Mul(y, tmp)
	}

	for _, e := range c.ExplicitDen {

		var (
			col = column.GnarkEvalExprColumn(api, run, e.Board())
			tmp = frontend.Variable(1)
		)

		for i := range col {
			tmp = api.Mul(tmp, col[i])
		}

		d = api.Mul(d, tmp)
	}

	y = api.Div(y, d)

	api.AssertIsEqual(y, frontend.Variable(1))
}

func (c *CheckGrandProductIsOne) Skip() {
	c.Skipped = true
}

func (c *CheckGrandProductIsOne) IsSkipped() bool {
	return c.Skipped
}

// FinalProductCheck mutiplies the last entries of the z columns
// and check that it is equal to the query param, implementing the [wizard.VerifierAction]
type FinalProductCheck struct {
	// ZOpenings lists all the openings of all the zCtx
	ZOpenings []query.LocalOpening
	// query ID
	GrandProductID ifaces.QueryID
	// skip the verifer action
	skipped bool `serde:"omit"`
	// ToExplicitlyEvaluate list all the terms that are publicly
	// evaluated by the verifier.
	ToExplicitlyEvaluate []*symbolic.Expression
}

// Run implements the [wizard.VerifierAction]
func (f *FinalProductCheck) Run(run wizard.Runtime) error {

	// zProd stores the product of the ending values of the zs as queried
	// in the protocol via the local opening queries.
	zProd := field.One()
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		zProd.Mul(&zProd, &temp)
	}

	for _, e := range f.ToExplicitlyEvaluate {
		c := column.EvalExprColumn(run, e.Board()).IntoRegVecSaveAlloc()
		for i := range c {
			zProd.Mul(&zProd, &c[i])
		}
	}

	claimedProd := run.GetGrandProductParams(f.GrandProductID).Y
	if zProd != claimedProd {
		return fmt.Errorf("grand product: the final evaluation check failed for %v\n"+
			"given %v but calculated %v,",
			f.GrandProductID, claimedProd.String(), zProd.String())
	}

	return nil
}

// RunGnark implements the [wizard.VerifierAction]
func (f *FinalProductCheck) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	claimedProd := run.GetGrandProductParams(f.GrandProductID).Prod

	// zProd stores the product of the ending values of the z columns
	zProd := frontend.Variable(field.One())
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		zProd = api.Mul(zProd, temp)
	}

	api.AssertIsEqual(zProd, claimedProd)
}

func (f *FinalProductCheck) Skip() {
	f.skipped = true
}

func (f *FinalProductCheck) IsSkipped() bool {
	return f.skipped
}

package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

// The GrandProduct query is obtained by processing all the permuation queries specific to a target module.
// We store the randomised symbolic products of A and B of permuation queries combinedly
// into the Numerators and the Denominators of the GrandProduct query
type GrandProductInput struct {
	Size         int
	Numerators   []*symbolic.Expression // stores A as multi-column
	Denominators []*symbolic.Expression // stores B as multi-column
}

// GrandProduct is a query for computing the grand-product of several vector expressions. The
// query returns a unique field element result.
type GrandProduct struct {
	Round int
	ID    ifaces.QueryID
	// The list of the inputs of the query, grouped by sizes
	Inputs map[int]*GrandProductInput
	uuid   uuid.UUID `serde:"omit"`
}

type GrandProductParams struct {
	BaseY  field.Element
	ExtY   fext.Element
	IsBase bool
}

// NewGrandProduct creates a new instance of a GrandProduct query.
//
// Parameters:
// - round: The round number of the query.
// - id: The unique identifier for the query.
// - numerators: A slice of symbolic expressions representing the numerators of the permutation queries.
// - denominators: A slice of symbolic expressions representing the denominators of the permutation queries.
//
// Returns:
// - A pointer to a new instance of GrandProduct.
func NewGrandProduct(round int, inp map[int]*GrandProductInput, id ifaces.QueryID) GrandProduct {
	// check the length consistency
	for key := range inp {
		for i, num := range inp[key].Numerators {
			if err := num.Validate(); err != nil {
				utils.Panic(" Numerator[%v] is not a valid expression", i)
			}

			if rs := column.ColumnsOfExpression(num); len(rs) == 0 {
				continue
			}

			b := num.Board()
			if key != column.ExprIsOnSameLengthHandles(&b) {
				utils.Panic("expression size mismatch")
			}
		}

		for i, den := range inp[key].Denominators {
			if err := den.Validate(); err != nil {
				utils.Panic(" Denominator[%v] is not a valid expression", i)
			}

			if rs := column.ColumnsOfExpression(den); len(rs) == 0 {
				continue
			}

			b := den.Board()
			if key != column.ExprIsOnSameLengthHandles(&b) {
				utils.Panic("expression size mismatch")
			}
		}
	}

	return GrandProduct{
		Round:  round,
		Inputs: inp,
		ID:     id,
		uuid:   uuid.New(),
	}
}

// Constructor for grand product query parameters
func NewGrandProductParams(y field.Element) GrandProductParams {
	return GrandProductParams{
		BaseY:  y,
		ExtY:   *fext.SetFromBase(new(fext.Element), &y),
		IsBase: true,
	}
}

func NewGrandProductParamsExt(yExt fext.Element) GrandProductParams {
	return GrandProductParams{
		BaseY:  field.Zero(),
		ExtY:   yExt,
		IsBase: false,
	}
}

// Name returns the unique identifier of the GrandProduct query.
func (g GrandProduct) Name() ifaces.QueryID {
	return g.ID
}

// Updates a Fiat-Shamir state
func (gp GrandProductParams) UpdateFS(fs fiatshamir.FS) {
	fs.UpdateExt(gp.ExtY)
}

// Compute returns the result value of the [GrandProduct] query. It
// should be run by a runtime with access to the query columns. i.e
// either by a [wizard.ProverRuntime] or a [wizard.VerifierRuntime]
// but then the involved columns should all be public.
func (g GrandProduct) Compute(run ifaces.Runtime) fext.GenericFieldElem {

	result := fext.GenericFieldOne()

	for size := range g.Inputs {
		for _, factor := range g.Inputs[size].Numerators {

			var (
				numBoard           = factor.Board()
				numeratorMetadata  = numBoard.ListVariableMetadata()
				numerator          smartvectors.SmartVector
				intermediateResult = fext.GenericFieldOne()
			)

			if len(numeratorMetadata) == 0 {
				panic("unreachable")
			}

			if len(numeratorMetadata) > 0 {
				numerator = column.EvalExprColumn(run, numBoard)
			}

			if smartvectors.IsBase(numerator) {
				numeratorSlice, _ := numerator.IntoRegVecSaveAllocBase()
				tempResult := field.One()
				for k := range numeratorSlice {
					tempResult.Mul(&tempResult, &numeratorSlice[k])
				}
				intermediateResult = fext.NewGenFieldFromBase(tempResult)
			} else {
				// for field extensions
				numeratorSlice := numerator.IntoRegVecSaveAllocExt()
				tempResult := fext.One()
				for k := range numeratorSlice {
					tempResult.Mul(&tempResult, &numeratorSlice[k])
				}
				intermediateResult = fext.NewGenFieldFromExt(tempResult)
			}
			result.Mul(&intermediateResult)
		}

		for _, factor := range g.Inputs[size].Denominators {

			var (
				denBoard            = factor.Board()
				denominatorMetadata = denBoard.ListVariableMetadata()
				denominator         smartvectors.SmartVector
				intermediateResult  = fext.GenericFieldOne()
			)

			if len(denominatorMetadata) == 0 {
				panic("unreachable")
			}

			if len(denominatorMetadata) > 0 {
				denominator = column.EvalExprColumn(run, denBoard)
			}

			if smartvectors.IsBase(denominator) {
				tmp := field.NewElement(1)
				denominatorSlice, _ := denominator.IntoRegVecSaveAllocBase()
				for k := range denominatorSlice {

					if denominatorSlice[k].IsZero() {
						panic("denominator contains zeroes")
					}

					tmp.Mul(&tmp, &denominatorSlice[k])
				}
				intermediateResult = fext.NewGenFieldFromBase(tmp)
			} else {
				// for field extensions
				tmp := fext.One()
				denominatorSlice := denominator.IntoRegVecSaveAllocExt()
				for k := range denominatorSlice {

					if denominatorSlice[k].IsZero() {
						panic("denominator contains zeroes")
					}

					tmp.Mul(&tmp, &denominatorSlice[k])
				}
				intermediateResult = fext.NewGenFieldFromExt(tmp)
			}

			result.Div(&intermediateResult)

		}
	}

	return result
}

// Check verifies the satisfaction of the GrandProduct query using the provided runtime.
// It calculates the product of numerators and denominators, and checks
// if prod(Numerators) == Prod(Denominators)*ParamY, and returns an error if the condition is not satisfied.
// The function also returns an error if the denominator contains a zero.
//
// Parameters:
// - run: The runtime interface providing access to the query parameter for query verification.
//
// Returns:
// - An error if the grand product query is not satisfied, or nil if it is satisfied.
func (g GrandProduct) Check(run ifaces.Runtime) error {

	var (
		params     = run.GetParams(g.ID).(GrandProductParams)
		actualProd = g.Compute(run)
	)

	for size := range g.Inputs {
		input := g.Inputs[size]
		for i := range input.Denominators {
			denominator := column.EvalExprColumn(run, input.Denominators[i].Board()).IntoRegVecSaveAllocExt()
			for k := range denominator {
				if denominator[k].IsZero() {
					return fmt.Errorf("the grand product query %v is not satisfied, (size=%v, denominator n°%v) denominator[%v] is zero", g.ID, size, i, k)
				}
			}
		}
	}
	actualProdElem := actualProd.GetExt()
	if actualProdElem != params.ExtY {
		return fmt.Errorf("the grand product query %v is not satisfied, actualProdElem = %v, params.ExtY = %v", g.ID, actualProdElem.String(), params.ExtY.String())
	}

	return nil
}

func (g GrandProduct) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	utils.Panic("Unimplemented")
}

func (g GrandProduct) UUID() uuid.UUID {
	return g.uuid
}

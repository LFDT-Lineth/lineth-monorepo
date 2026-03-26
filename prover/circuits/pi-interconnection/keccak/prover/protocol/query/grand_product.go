package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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
	Y field.Element
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
	return GrandProductParams{Y: y}
}

// Name returns the unique identifier of the GrandProduct query.
func (g GrandProduct) Name() ifaces.QueryID {
	return g.ID
}

// Updates a Fiat-Shamir state
func (gp GrandProductParams) UpdateFS(fs *fiatshamir.State) {
	fs.Update(gp.Y)
}

// Compute returns the result value of the [GrandProduct] query. It
// should be run by a runtime with access to the query columns. i.e
// either by a [wizard.ProverRuntime] or a [wizard.VerifierRuntime]
// but then the involved columns should all be public.
func (g GrandProduct) Compute(run ifaces.Runtime) field.Element {

	result := field.One()

	for size := range g.Inputs {
		for _, factor := range g.Inputs[size].Numerators {

			var (
				numBoard          = factor.Board()
				numeratorMetadata = numBoard.ListVariableMetadata()
				numerator         []field.Element
			)

			if len(numeratorMetadata) == 0 {
				panic("unreachable")
			}

			if len(numeratorMetadata) > 0 {
				numerator = column.EvalExprColumn(run, numBoard).IntoRegVecSaveAlloc()
			}

			for k := range numerator {
				result.Mul(&result, &numerator[k])
			}
		}

		for _, factor := range g.Inputs[size].Denominators {

			var (
				denBoard            = factor.Board()
				denominatorMetadata = denBoard.ListVariableMetadata()
				denominator         []field.Element
				tmp                 = field.One()
			)

			if len(denominatorMetadata) == 0 {
				panic("unreachable")
			}

			if len(denominatorMetadata) > 0 {
				denominator = column.EvalExprColumn(run, denBoard).IntoRegVecSaveAlloc()
			}

			for k := range denominator {

				if denominator[k].IsZero() {
					panic("denominator contains zeroes")
				}

				tmp.Mul(&tmp, &denominator[k])
			}

			result.Div(&result, &tmp)
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
			denominator := column.EvalExprColumn(run, input.Denominators[i].Board()).IntoRegVecSaveAlloc()
			for k := range denominator {
				if denominator[k].IsZero() {
					return fmt.Errorf("the grand product query %v is not satisfied, (size=%v, denominator n°%v) denominator[%v] is zero", g.ID, size, i, k)
				}
			}
		}
	}

	if actualProd != params.Y {
		return fmt.Errorf("the grand product query %v is not satisfied, actualProd = %v, param.Y = %v", g.ID, actualProd.String(), params.Y.String())
	}

	return nil
}

func (g GrandProduct) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	utils.Panic("Unimplemented")
}

func (g GrandProduct) UUID() uuid.UUID {
	return g.uuid
}

package query

import (
	"errors"
	"fmt"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
	"github.com/google/uuid"
)

// LogDerivativeSumInput stores the input to the query
type LogDerivativeSumInput struct {
	Parts []LogDerivativeSumPart
}

// LogDerivativeSumPart is a part of the LogDerivativeSum query.
type LogDerivativeSumPart struct {
	Size int
	Num  *sym.Expression
	Den  *sym.Expression
	Name string
}

// LogDerivativeSum is the context of LogDerivativeSum query.
// The fields are maps from [round, size].
// the aim of the query is to compute:
// \sum_{i,j} N_{i,j}/D_{i,j} where
// N_{i,j} is  the i-th element of the underlying column of  j-th Numerator
// D_{i,j} is  the i-th element of the underlying column of  j-th Denominator
type LogDerivativeSum struct {
	Round  int
	Inputs LogDerivativeSumInput
	ID     ifaces.QueryID
	uuid   uuid.UUID `serde:"omit"`
}

// the result of the global Sum
type LogDerivSumParams struct {
	Sum field.Element // the sum of all the ZOpenings from different [round,size].
}

// Updates a Fiat-Shamir state
func (l LogDerivSumParams) UpdateFS(fs *fiatshamir.State) {
	fs.Update(l.Sum)
}

// NewLogDerivativeSum creates the new context LogDerivativeSum.
func NewLogDerivativeSum(round int, inp LogDerivativeSumInput, id ifaces.QueryID) LogDerivativeSum {

	if len(inp.Parts) == 0 {
		utils.Panic("Numerator and Denominator should have the same (no-zero) length, %v", len(inp.Parts))
	}

	// check the length consistency
	for i, part := range inp.Parts {

		if err := part.Num.Validate(); err != nil {
			utils.Panic(" Numerator[%v] is not a valid expression", i)
		}

		if rs := column.ColumnsOfExpression(part.Num); len(rs) == 0 {
			continue
		}

		numBoard := part.Num.Board()
		if part.Size != column.ExprIsOnSameLengthHandles(&numBoard) {
			utils.Panic("expression size mismatch")
		}

		if err := part.Den.Validate(); err != nil {
			utils.Panic(" Denominator[%v] is not a valid expression", i)
		}

		if rs := column.ColumnsOfExpression(part.Den); len(rs) == 0 {
			continue
		}

		denBoard := part.Den.Board()
		if part.Size != column.ExprIsOnSameLengthHandles(&denBoard) {
			utils.Panic("expression size mismatch: qname=%v expression-size=%v expected-size=%v", id, column.ExprIsOnSameLengthHandles(&numBoard), part.Size)
		}
	}

	return LogDerivativeSum{
		Round:  round,
		Inputs: inp,
		ID:     id,
		uuid:   uuid.New(),
	}
}

// Name implements the [ifaces.Query] interface
func (r LogDerivativeSum) Name() ifaces.QueryID {
	return r.ID
}

// Constructor for the query parameters/result
func NewLogDerivSumParams(sum field.Element) LogDerivSumParams {
	return LogDerivSumParams{Sum: sum}
}

// Compute returns the result value of the [LogDerivativeSum] query. It
// should be run by a runtime with access to the query columns. i.e
// either by a [wizard.ProverRuntime] or a [wizard.VerifierRuntime]
// but then the involved columns should all be public.
func (r LogDerivativeSum) Compute(run ifaces.Runtime) (field.Element, error) {

	// compute the actual sum from the Numerator and Denominator
	var (
		err       error
		actualSum = field.Zero()
		resLock   = &sync.Mutex{}
	)

	parallel.Execute(len(r.Inputs.Parts), func(start, stop int) {

		for k := start; k < stop; k++ {
			// Stopping the goroutine if an error has been encountered
			resLock.Lock()
			if err != nil {
				resLock.Unlock()
				return
			}
			resLock.Unlock()

			var (
				inputs = r.Inputs.Parts
				size   = inputs[k].Size
				num    = inputs[k].Num
				den    = inputs[k].Den
				res, e = computeLogDerivativeSumPair(run, num, den, size)
			)

			if e != nil {
				resLock.Lock()
				err = e
				resLock.Unlock()
				return
			}

			resLock.Lock()
			actualSum.Add(&actualSum, &res)
			resLock.Unlock()
		}
	})

	if err != nil {
		return field.Element{}, err
	}

	return actualSum, nil
}

// Test that global sum is correct
func (r LogDerivativeSum) Check(run ifaces.Runtime) error {

	var (
		params         = run.GetParams(r.ID).(LogDerivSumParams)
		actualSum, err = r.Compute(run)
	)

	if err != nil {
		return fmt.Errorf("expected a denominator without zeroes: %w", err)
	}

	if actualSum != params.Sum {
		return fmt.Errorf("expected LogDerivativeSum = %s but got %s for the query %v", params.Sum.String(), actualSum.String(), r.ID)
	}

	return nil
}

// Test that global sum is correct
func (r LogDerivativeSum) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("unexpected call")
}

// computeLogDerivativeSumPair computes the log derivative sum for a couple
// of numerator and denominator.
func computeLogDerivativeSumPair(run ifaces.Runtime, num, den *sym.Expression, size int) (field.Element, error) {

	var (
		numBoard            = num.Board()
		denBoard            = den.Board()
		numeratorMetadata   = numBoard.ListVariableMetadata()
		denominatorMetadata = denBoard.ListVariableMetadata()
		numerator           smartvectors.SmartVector
		denominator         smartvectors.SmartVector
		noNumerator         = len(numeratorMetadata) == 0
		noDenominator       = len(denominatorMetadata) == 0
		res                 field.Element
	)

	if noNumerator && noDenominator {
		return field.NewElement(uint64(size)), nil
	}

	if !noNumerator {
		numerator = column.EvalExprColumn(run, numBoard)
		numerator, _ = smartvectors.TryReduceSizeRight(numerator)

		// If the denominator resolves into a constant equal to 1, then
		// this is identical to not having any denominator. If it is
		// zero, then we can directly return 0.
		if numC, ok := numerator.(*smartvectors.Constant); ok {

			v := numC.Val()

			if v.IsZero() {
				return field.Zero(), nil
			}

			if v.IsOne() {
				noNumerator = true
			}
		}
	}

	if !noDenominator {
		denominator = column.EvalExprColumn(run, denBoard)
		denominator, _ = smartvectors.TryReduceSizeRight(denominator)

		for d := range denominator.IterateCompact() {
			if d.IsZero() {
				return field.Zero(), errors.New("denominator is zero")
			}
		}
	}

	if noNumerator {

		var (
			denominatorWindow = smartvectors.Window(denominator)
			res               = field.Zero()
		)

		denominatorWindow = field.BatchInvert(denominatorWindow)
		for i := range denominatorWindow {
			if denominatorWindow[i].IsZero() {
				return field.Element{}, errors.New("denominator is zero")
			}
			res.Add(&res, &denominatorWindow[i])
		}

		denominatorPadding, denominatorHasPadding := smartvectors.PaddingVal(denominator)
		if denominatorHasPadding {

			if denominatorPadding.IsZero() {
				return field.Zero(), fmt.Errorf("denominator padding is zero")
			}

			denominatorPadding.Inverse(&denominatorPadding)

			var (
				nbPadding        = denominator.Len() - len(denominatorWindow)
				nbPaddingAsField field.Element
			)

			nbPaddingAsField.SetInt64(int64(nbPadding))
			denominatorPadding.Mul(&denominatorPadding, &nbPaddingAsField)
			res.Add(&res, &denominatorPadding)
		}

		return res, nil
	}

	if noDenominator {

		var (
			numeratorWindow = smartvectors.Window(numerator)
			res             = field.Zero()
		)

		for i := range numeratorWindow {
			fmt.Printf("numeratorWindow[%v] = %v\n", i, numeratorWindow[i].String())
			res.Add(&res, &numeratorWindow[i])
		}

		numeratorPadding, numeratorHasPadding := smartvectors.PaddingVal(numerator)
		if numeratorHasPadding && !numeratorPadding.IsZero() {

			var (
				nbPadding        = numerator.Len() - len(numeratorWindow)
				nbPaddingAsField field.Element
			)

			nbPaddingAsField.SetInt64(int64(nbPadding))
			numeratorPadding.Mul(&numeratorPadding, &nbPaddingAsField)
			res.Add(&res, &numeratorPadding)
		}

		return res, nil
	}

	// This implementation should catch 99% of the remaining cases. This follows
	// from the fact that most vectors have a padded structure and we can take
	// advantage of that in the implementation.
	numeratorPCW, ok := numerator.(*smartvectors.PaddedCircularWindow)
	if ok {
		var (
			pv     = numeratorPCW.PaddingVal()
			offset = numeratorPCW.Offset()
			window = numeratorPCW.Window()
			size   = numeratorPCW.Len()
		)

		if pv.IsZero() && offset+len(window) <= size {

			var (
				start              = offset
				stop               = offset + len(window)
				denominatorNonZero = make([]field.Element, 0, len(window))
				numeratorNonZero   = make([]field.Element, 0, len(window))
			)

			for i := start; i < stop; i++ {

				if denominator.GetPtr(i).IsZero() {
					return field.Element{}, errors.New("denominator is zero")
				}

				if !numerator.GetPtr(i).IsZero() {
					denominatorNonZero = append(denominatorNonZero, denominator.Get(i))
					numeratorNonZero = append(numeratorNonZero, numerator.Get(i))
				}
			}

			denominatorNonZero = field.BatchInvert(denominatorNonZero)

			for i := range denominatorNonZero {
				var tmp field.Element
				tmp.Mul(&denominatorNonZero[i], &numeratorNonZero[i])
				res.Add(&res, &tmp)
			}

			return res, nil
		}
	}

	// This is the "main" case which corresponds to when both numerator and the
	// numerator are defined. In this case, we iterate over the numerator and
	// record all the pairs num/den such that num != 0. The initial capacity is
	// empirically divided by 16 as we will never need more than that 99% of
	// the time.
	//
	// The implementation is based on the following observation: the numerator
	// is always full of zeroes.
	var (
		denominatorNonZero = make([]field.Element, 0, denominator.Len()/16)
		numeratorNonZero   = make([]field.Element, 0, numerator.Len()/16)
	)

	for i := 0; i < numerator.Len(); i++ {

		if denominator.GetPtr(i).IsZero() {
			return field.Element{}, errors.New("denominator is zero")
		}

		if !numerator.GetPtr(i).IsZero() {

			denominatorNonZero = append(denominatorNonZero, denominator.Get(i))
			numeratorNonZero = append(numeratorNonZero, numerator.Get(i))
		}
	}

	denominatorNonZero = field.BatchInvert(denominatorNonZero)

	for i := range denominatorNonZero {
		var tmp field.Element
		tmp.Mul(&denominatorNonZero[i], &numeratorNonZero[i])
		res.Add(&res, &tmp)
	}

	return res, nil
}

func (q LogDerivativeSum) UUID() uuid.UUID {
	return q.uuid
}

func fieldAsInt(f field.Element) int {

	if f.IsUint64() {
		return int(f.Uint64())
	}

	f_ := f
	f_.Neg(&f_)

	if f_.IsUint64() {
		return -int(f_.Uint64())
	}

	// utils.Panic("unsupported field type: %v", f.String())
	return 0
}

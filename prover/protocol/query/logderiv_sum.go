package query

import (
	"errors"
	"fmt"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
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
	Sum fext.GenericFieldElem // the sum of all the ZOpenings from different [round,size].
}

// Updates a Fiat-Shamir state
func (l LogDerivSumParams) UpdateFS(fs fiatshamir.FS) {
	fs.UpdateGeneric(l.Sum)
}

// NewLogDerivativeSum creates the new context LogDerivativeSum.
func NewLogDerivativeSum(round int, inp LogDerivativeSumInput, id ifaces.QueryID) LogDerivativeSum {

	if len(inp.Parts) == 0 {
		utils.Panic("LogDerivativeSum must have at least one part, name=%v", id)
	}

	// check the length consistency
	for k, part := range inp.Parts {

		if err := part.Num.Validate(); err != nil {
			utils.Panic(" Numerator[%v] is not a valid expression", k)
		}

		if rs := column.ColumnsOfExpression(part.Num); len(rs) == 0 {
			continue
		}

		b := part.Num.Board()
		if part.Size != column.ExprIsOnSameLengthHandles(&b) {
			cols := column.ColumnsOfExpression(part.Num)
			cols = append(cols, column.ColumnsOfExpression(part.Den)...)
			for _, col := range cols {
				switch c := col.(type) {
				case column.Natural:
					logrus.Infof("col = %v size = %v, status = %v, numCol = %v, partSize = %v \n", c.GetColID(), c.Size(), c.Status(), len(cols), part.Size)
				}
			}
			utils.Panic("expression size mismatch, partSize %v != expectedSize %v", part.Size, column.ExprIsOnSameLengthHandles(&b))
		}

		if err := part.Den.Validate(); err != nil {
			utils.Panic(" Denominator[%v] is not a valid expression", k)
		}

		if rs := column.ColumnsOfExpression(part.Den); len(rs) == 0 {
			continue
		}

		b = part.Den.Board()
		if part.Size != column.ExprIsOnSameLengthHandles(&b) {
			utils.Panic("expression size mismatch: qname=%v expression-size=%v expected-size=%v", id, column.ExprIsOnSameLengthHandles(&b), k)
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
func NewLogDerivSumParams(sum fext.GenericFieldElem) LogDerivSumParams {
	return LogDerivSumParams{Sum: sum}
}

// Compute returns the result value of the [LogDerivativeSum] query. It
// should be run by a runtime with access to the query columns. i.e
// either by a [wizard.ProverRuntime] or a [wizard.VerifierRuntime]
// but then the involved columns should all be public.
func (r LogDerivativeSum) Compute(run ifaces.Runtime) (fext.GenericFieldElem, error) {

	// compute the actual sum from the Numerator and Denominator
	var (
		err       error
		actualSum = fext.GenericFieldElem{}
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
			if err != nil {
				resLock.Unlock()
				return
			}
			actualSum.Add(&res)
			resLock.Unlock()
		}
	})

	if err != nil {
		return fext.GenericFieldElem{}, err
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

	if !actualSum.IsEqual(&params.Sum) {
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
func computeLogDerivativeSumPair(run ifaces.Runtime, num, den *sym.Expression, size int) (fext.GenericFieldElem, error) {

	var (
		numBoard            = num.Board()
		denBoard            = den.Board()
		numeratorMetadata   = numBoard.ListVariableMetadata()
		denominatorMetadata = denBoard.ListVariableMetadata()
		numerator           smartvectors.SmartVector
		denominator         smartvectors.SmartVector
		noNumerator         = len(numeratorMetadata) == 0
		noDenominator       = len(denominatorMetadata) == 0
		res                 fext.Element
	)

	if noNumerator && noDenominator {
		elem := field.NewElement(uint64(size))
		return fext.NewGenFieldFromBase(elem), nil
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
				return fext.GenericFieldElem{}, nil
			}

			if v.IsOne() {
				noNumerator = true
			}
		}
	}

	if !noDenominator {
		denominator = column.EvalExprColumn(run, denBoard)
		denominator, _ = smartvectors.TryReduceSizeLeft(denominator)

		for _, d := range denominator.IntoRegVecSaveAllocExt() {
			if d.IsZero() {
				return fext.GenericFieldElem{}, errors.New("denominator is zero")
			}
		}

	}

	if noNumerator {

		var (
			denominatorWindow = smartvectors.WindowExt(denominator)
			res               = fext.GenericFieldElem{}
		)

		denominatorWindow = fext.BatchInvert(denominatorWindow)
		for i := range denominatorWindow {
			if denominatorWindow[i].IsZero() {
				return fext.GenericFieldElem{}, errors.New("denominator is zero")
			}

			elemDenominatorWindow := fext.NewGenFieldFromExt(denominatorWindow[i])
			res.Add(&elemDenominatorWindow)
		}

		denominatorPadding, denominatorHasPadding := smartvectors.PaddingValGeneric(denominator)

		if denominatorHasPadding {
			if denominatorPadding.IsZero() {
				return fext.GenericFieldElem{}, fmt.Errorf("denominator padding is zero")
			}

			denominatorPadding.Inverse(&denominatorPadding)

			var (
				nbPadding        = denominator.Len() - len(denominatorWindow)
				nbPaddingAsField field.Element
			)

			nbPaddingAsField.SetInt64(int64(nbPadding))
			genericNbPaddingAsField := fext.NewGenFieldFromBase(nbPaddingAsField)
			denominatorPadding.Mul(&genericNbPaddingAsField)
			res.Add(&denominatorPadding)
		}

		return res, nil
	}

	if noDenominator {

		var (
			numeratorWindow = smartvectors.WindowExt(numerator)
			res             = fext.Zero()
		)

		for i := range numeratorWindow {
			res.Add(&res, &numeratorWindow[i])
		}

		numeratorPadding, numeratorHasPadding := smartvectors.PaddingValExt(numerator)
		if numeratorHasPadding && !numeratorPadding.IsZero() {

			var (
				nbPadding        = numerator.Len() - len(numeratorWindow)
				nbPaddingAsField fext.Element
			)

			fext.SetFromIntBase(&nbPaddingAsField, int64(nbPadding))
			numeratorPadding.Mul(&numeratorPadding, &nbPaddingAsField)
			res.Add(&res, &numeratorPadding)
		}

		return fext.NewGenFieldFromExt(res), nil
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
				denominatorNonZero = make([]fext.Element, 0, len(window))
				numeratorNonZero   = make([]field.Element, 0, len(window))
			)

			for i := start; i < stop; i++ {

				den := denominator.GetExt(i)
				if den.IsZero() {
					return fext.GenericFieldElem{}, errors.New("denominator is zero")
				}

				if num := &window[i-start]; !num.IsZero() {
					denominatorNonZero = append(denominatorNonZero, den)
					numeratorNonZero = append(numeratorNonZero, *num)
				}
			}

			denominatorNonZero = fext.BatchInvert(denominatorNonZero)

			for i := range denominatorNonZero {
				var tmp fext.Element
				tmp.MulByElement(&denominatorNonZero[i], &numeratorNonZero[i])
				res.Add(&res, &tmp)
			}

			return fext.NewGenFieldFromExt(res), nil
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
		denominatorNonZero = make([]fext.Element, 0, denominator.Len()/16)
		numeratorNonZero   = make([]fext.Element, 0, numerator.Len()/16)
	)

	for i := 0; i < numerator.Len(); i++ {

		a := denominator.GetExt(i)
		if a.IsZero() {
			return fext.GenericFieldElem{}, errors.New("denominator is zero")
		}

		if x := numerator.GetExt(i); !x.IsZero() {
			denominatorNonZero = append(denominatorNonZero, a)
			numeratorNonZero = append(numeratorNonZero, x)
		}
	}

	denominatorNonZero = fext.BatchInvert(denominatorNonZero)

	for i := range denominatorNonZero {
		var tmp fext.Element
		tmp.Mul(&denominatorNonZero[i], &numeratorNonZero[i])
		res.Add(&res, &tmp)
	}

	return fext.NewGenFieldFromExt(res), nil
}

// PerPartResult holds the name and the log-derivative sum contribution of a
// single part.
type PerPartResult struct {
	Name string
	Sum  fext.GenericFieldElem
}

// ComputePerPart returns the log-derivative sum contribution of each part
// individually. This is useful for debugging: when the overall sum does not
// cancel, this identifies which specific lookup query is broken.
func (r LogDerivativeSum) ComputePerPart(run ifaces.Runtime) ([]PerPartResult, error) {
	results := make([]PerPartResult, len(r.Inputs.Parts))
	for k := range r.Inputs.Parts {
		part := &r.Inputs.Parts[k]
		val, err := computeLogDerivativeSumPair(run, part.Num, part.Den, part.Size)
		if err != nil {
			return nil, fmt.Errorf("part %q (size=%d): %w", part.Name, part.Size, err)
		}
		results[k] = PerPartResult{Name: part.Name, Sum: val}
	}
	return results, nil
}

func (q LogDerivativeSum) UUID() uuid.UUID {
	return q.uuid
}

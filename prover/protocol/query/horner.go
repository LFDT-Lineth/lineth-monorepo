package query

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/google/uuid"
)

// HornerPart represents a part of a Horner evaluation query.
type HornerPart struct {
	// Name is an optional name for the part. It does not play a role in itself
	// but comes up in potential error messages to help figuring where the part
	// originates in case the Horner parts comes from a projection.
	Name string
	// SignNegative indicates that the result should be negated.
	SignNegative bool
	// Coefficient is the coefficient of the term. It may be a
	// column or random linear combination of columns. Each entry
	// in the table corresponds to a multi-ary entry.
	Coefficients []*symbolic.Expression
	// Selector is a boolean indicator column telling which terms
	// or [Coefficients] should be included in the Horner evaluation.
	// Each entry in the list corresponds to a multi-ary entry.
	Selectors []ifaces.Column
	// X is the "x" value in the Horner evaluation query. Most of the
	// time, the accessor will be a random coin. The typing to accessor
	// allows for more flexibility.
	X ifaces.Accessor
	// size indicates the size of which the horner part is running.
	// It is lazily computed thanks to the Size() column.
	size int `serde:"omit"`
}

// Horner represents a Horner evaluation query. The query returns
// a sum (for every part) of values that are each evaluated as:
//
// ```
//
//	 def horner(selector, value, x, n0):
//			res := 0
//	 		count := 0
//			// iteration in reverse order
//			for sel, val in zip(selector, value).reverse() {
//	 	  	if sel {
//			  		res *= x
//			   		res += val
//			   		count += 1
//				}
//			}
//			return sign * res * (x ** n0), n0 + count
//
// ```
//
// where x and n0 are parameters of the query.
//
// As an addtional feature, the Horner query may be "multi-ary" meaning
// that the summation is not just computed vertically but left-to-right
// then top-to-bottom over a range of expressions.
type Horner struct {
	// Round is the round of definition of the query
	Round int
	// ID is the identifier of the query in the [wizard.CompiledIOP]
	ID ifaces.QueryID
	// Parts are the parts of the query
	Parts []HornerPart
	uuid  uuid.UUID `serde:"omit"`
}

// HornerParamsParts represents the parameters for a part of a [Horner]
// evaluation query.
type HornerParamsPart struct {
	// N0 is an initial offset of the Horner query
	N0 int
	// N1 is the second offset of the Horner query
	N1 int
}

// HornerParams represents the parameters of the Horner evaluation query
type HornerParams struct {
	// Final result is the result of summing the Horner parts for every
	// queries.
	FinalResult fext.Element
	// Parts are the parameters of the Horner parts
	Parts []HornerParamsPart
}

// NewHorner constructs a new Horner evaluation query. The function also
// performs all the well-formedness sanity-checks. In case of failure, it
// panics. In particular, it will panic if:
//
//   - the pairs [HornerPart.Selector] and [HornerPart.Coefficient] do not
//     share the same length.
func NewHorner(round int, id ifaces.QueryID, parts []HornerPart) Horner {

	for i := range parts {

		for j := range parts[i].Selectors {
			var (
				board = parts[i].Coefficients[j].Board()
				size  = column.ExprIsOnSameLengthHandles(&board)
			)

			if size == 0 {
				utils.Panic("Horner part %v has a coefficient of size 0, part=%v", i, parts[i].Name)
			}

			if parts[i].Selectors[j].Size() != size {
				utils.Panic("Horner part %v has a selector of size %v and a coefficient of size %v, part=%v", i, parts[i].Selectors[j].Size(), size, parts[i].Name)
			}

			if parts[i].size > 0 && size != parts[i].size {
				utils.Panic("Horner part %v has a selector of size %v and a coefficient of size %v, part=%v", i, parts[i].Selectors[j].Size(), size, parts[i].Name)
			}

			parts[i].size = size
		}
	}

	return Horner{
		Round: round,
		ID:    id,
		Parts: parts,
		uuid:  uuid.New(),
	}
}

// NewIncompleteHornerParams returns an incomplete [HornerParams] by using
// the [HornerParamsParts] as inputs. The function ignores the N1's values
// if they are provided and does not set
func NewHornerParamsFromIncomplete(run ifaces.Runtime, q Horner, parts []HornerParamsPart) HornerParams {
	res := HornerParams{
		Parts: parts,
	}
	res.SetResult(run, q)
	return res
}

// SetResult computes the result parameters of the Horner evaluation from
// an incomplete [HornerParams]. The object should however have a the X's
// and the N0's set. The function returns a pointer to its receiver so the
// call can be chained.
func (p *HornerParams) SetResult(run ifaces.Runtime, q Horner) *HornerParams {
	n1s, finalResult := p.GetResult(run, q)
	for i := range p.Parts {
		p.Parts[i].N1 = n1s[i]
	}
	p.FinalResult = finalResult
	return p
}

// GetResult computes and returns the result of the Horner evaluation
// from the (possibly incomplete) [HornerParams]. The function returns
// the list of the N1 values and the final result of the Horner evaluation
// query.
func (p *HornerParams) GetResult(run ifaces.Runtime, q Horner) (n1s []int, finalResult fext.Element) {

	// note: this is ineffective as the final result is already allocated
	// and assigned with 0. The line is here for clarity.
	finalResult = fext.Zero()
	n1s = make([]int, len(p.Parts))

	if len(q.Parts) != len(p.Parts) {
		utils.Panic("Horner query has %v parts but HornerParams has %v", len(q.Parts), len(p.Parts))
	}

	for i, part := range q.Parts {

		var (
			n0         = p.Parts[i].N0
			res, count = getResultOfParts(run, &part)
			x          = part.X.GetValExt(run)
			xN0        = new(fext.Element).Exp(x, big.NewInt(int64(n0)))
		)

		res.Mul(&res, xN0)

		if part.SignNegative {
			res.Neg(&res)
		}

		n1s[i] = n0 + count
		finalResult.Add(&finalResult, &res)
	}

	return n1s, finalResult
}

// HornerPartResult holds per-part diagnostic data from a Horner evaluation.
type HornerPartResult struct {
	Name         string
	SignNegative bool
	RawResult    fext.Element
	Count        int
	N0           int
	N1           int
	Contribution fext.Element
	DataSize     int
}

// GetPerPartResults computes per-part Horner results without aggregating them. This is used for debugging purposes.
func (p *HornerParams) GetPerPartResults(run ifaces.Runtime, q Horner) []HornerPartResult {
	results := make([]HornerPartResult, len(q.Parts))
	for i, part := range q.Parts {
		res, count := getResultOfParts(run, &part)

		n0 := p.Parts[i].N0
		x := part.X.GetValExt(run)
		xN0 := new(fext.Element).Exp(x, big.NewInt(int64(n0)))

		contribution := res
		contribution.Mul(&contribution, xN0)
		if part.SignNegative {
			contribution.Neg(&contribution)
		}

		results[i] = HornerPartResult{
			Name:         part.Name,
			SignNegative: part.SignNegative,
			RawResult:    res,
			Count:        count,
			N0:           n0,
			N1:           n0 + count,
			Contribution: contribution,
			DataSize:     part.Size(),
		}
	}
	return results
}

// getResultOfParts computes the result of a part i of the [HornerQuery]. It
// returns the result of the evaluation and the selector count.
func getResultOfParts(run ifaces.Runtime, q *HornerPart) (fext.Element, int) {

	datas := make([]smartvectors.SmartVector, len(q.Coefficients))
	selectors := make([]smartvectors.SmartVector, len(q.Coefficients))

	for i := 0; i < len(q.Coefficients); i++ {
		selector := q.Selectors[i].GetColAssignment(run)

		board := q.Coefficients[i].Board()
		data := column.EvalExprColumn(run, board)

		datas[i] = data
		selectors[i] = selector

	}

	// fast path when there is only one coefficient and the selector is all-ones
	// TODO @gbotrel not sure we should keep that, makes code not readable and gain is about 5%
	if len(datas) == 1 {
		if _, ok := selectors[0].(*smartvectors.Constant); ok && selectors[0].GetPtr(0).IsOne() {
			if d, ok := datas[0].(*smartvectors.RegularExt); ok {

				size := datas[0].Len()
				count := size
				acc := fext.Zero()
				x := q.X.GetValExt(run)
				vx := make(extensions.Vector, size)
				var lock sync.Mutex
				parallel.Execute(size, func(start, stop int) {
					vxl := vx[start:stop]
					vxl[0].ExpInt64(x, int64(start))
					for i := 1; i < (stop - start); i++ {
						vxl[i].Mul(&vxl[i-1], &x)
					}
					vd := extensions.Vector((*d)[start:stop])
					localAcc := vxl.InnerProduct(vd)
					lock.Lock()
					acc.Add(&acc, &localAcc)
					lock.Unlock()
				})

				return acc, count
			}
		}
	}

	size := datas[0].Len()
	count := 0
	acc := fext.Zero()
	x := q.X.GetValExt(run)
	for row := size - 1; row >= 0; row-- {
		for i := 0; i < len(datas); i++ {

			if selectors[i].GetPtr(row).IsZero() {
				continue
			}

			count++
			acc.Mul(&acc, &x)
			other := datas[i].GetExt(row)
			acc.Add(&acc, &other)
		}
	}

	return acc, count
}

// Name implements the [ifaces.Query] interface
func (h *Horner) Name() ifaces.QueryID {
	return h.ID
}

// UpdateFS implements the [ifaces.QueryParams] interface. It updates
// FS with the parameters of the query.
func (h HornerParams) UpdateFS(fs fiatshamir.FS) {

	fs.UpdateExt(h.FinalResult)

	for _, part := range h.Parts {
		fs.Update(
			field.NewElement(uint64(part.N0)),
			field.NewElement(uint64(part.N1)),
		)
	}
}

// Check implements the [ifaces.Query] interface
func (h Horner) Check(run ifaces.Runtime) error {

	var (
		params           = run.GetParams(h.ID).(HornerParams)
		n1s, finalResult = params.GetResult(run, h)
	)

	if finalResult != params.FinalResult {
		return fmt.Errorf("expected final result %s but got %s", params.FinalResult.String(), finalResult.String())
	}

	for i, n1 := range n1s {
		if n1 != params.Parts[i].N1 {
			return fmt.Errorf("expected N1 %v but got %v, (part %v)", params.Parts[i].N1, n1, h.Parts[i].Name)
		}
	}

	return nil
}

// CheckGnark implements the [ifaces.Query] interface. It will panic
// when called as this query is not intended to be explicitly verified
// by the verifier in a gnark circuit.
func (h *Horner) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("Horner query is not intended to be explicitly verified by the verifier in a gnark circuit")
}

// Size returns the size of the columns taking part in a [HornerPart].
func (h *HornerPart) Size() int {
	if h.size == 0 {
		board := h.Coefficients[0].Board()
		h.size = column.ExprIsOnSameLengthHandles(&board)
	}
	return h.size
}

func (h *Horner) UUID() uuid.UUID {
	return h.uuid
}

package horner

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// HornerCtx is a compilation artefact generated during the execution of the
// [CompileHorner] compiler.
type HornerCtx struct {
	// Column is the accumulating column used to check the computation of a
	// horner value for one [HornerPart].
	AccumulatingCols [][]ifaces.Column

	// CountingInnerProduct is the inner-product query used to check the
	// counting for each selector. Each entry of the inner-product query
	// maps to a selector.
	CountingInnerProducts []query.InnerProduct

	// LocOpenings are the local openings used to check the first value of
	// Columns[i]
	LocOpenings []query.LocalOpening

	// Q is the Horner query
	Q *query.Horner
}

// AssignHornerCtx is a [wizard.ProverAction] assigning the local openings of
// the Horner accumulating columns and the accumulating columns themselves.
// The function also sanity-checks the parameters assignment.
type AssignHornerCtx struct {
	HornerCtx
}

// AssignHornerIP is a [wizard.ProverAction] assigning the inner-products of
// the Horner compilation.
type AssignHornerIP struct {
	HornerCtx
}

// CheckHornerResult is [wizard.VerifierAction] responsible for checking that
// the values of n1 are correct (by checking the consistency with the IP queries)
// and checking that the final result is correctly computed by inspecting the
// local openings.
type CheckHornerResult struct {
	HornerCtx
	skipped bool `serde:"omit"`
}

// CompileHoner compiles the [query.Horner] queries one by one by "transforming"
// them into [query.InnerProduct], [query.LocalOpening], [query.Global] and
// [query.Local].
func CompileHorner(comp *wizard.CompiledIOP) {

	qNames := comp.QueriesParams.AllUnignoredKeys()

	for i := range qNames {

		q, ok := comp.QueriesParams.Data(qNames[i]).(*query.Horner)
		if !ok {
			continue
		}

		comp.QueriesParams.MarkAsIgnored(qNames[i])

		compileHornerQuery(comp, q)
	}
}

func compileHornerQuery(comp *wizard.CompiledIOP, q *query.Horner) {

	var (
		round = q.Round
		ctx   = HornerCtx{
			Q: q,
		}
		iPRound = 0
	)

	for i, part := range q.Parts {

		var (
			numList      = len(q.Parts[i].Coefficients)
			accumulators = make([]ifaces.Column, numList)
		)

		for j := 0; j < numList; j++ {

			accumulators[j] = comp.InsertCommit(
				round,
				ifaces.ColIDf("HORNER_%v_PART_%v_COLUMN_%v", q.ID, i, j),
				part.Size(),
				false,
			)
		}

		for j := 0; j < numList-1; j++ {

			prevAcc := column.Shift(accumulators[numList-1], 1)
			if j > 0 {
				prevAcc = accumulators[j-1]
			}

			comp.InsertGlobal(
				round,
				ifaces.QueryIDf("HORNER_%v_PART_%v_GLOBAL_%v", q.ID, i, j),
				sym.Sub(
					accumulators[j],
					microAccumulate(
						part.Selectors[j],
						prevAcc,
						part.X,
						part.Coefficients[j],
					),
				),
			)
		}

		loc := comp.InsertLocalOpening(
			round,
			ifaces.QueryIDf("HORNER_%v_PART_%v_LOCAL_OPENING", q.ID, i),
			accumulators[numList-1],
		)

		// This query takes care of checking the initial value of the column
		comp.InsertLocal(
			round,
			ifaces.QueryIDf("HORNER_%v_PART_%v_LOCAL", q.ID, i),
			sym.Sub(
				column.Shift(accumulators[0], -1),
				sym.Mul(
					column.Shift(q.Parts[i].Selectors[0], -1),
					column.ShiftExpr(q.Parts[i].Coefficients[0], -1),
				),
			),
		)

		ctx.AccumulatingCols = append(ctx.AccumulatingCols, accumulators)
		ctx.LocOpenings = append(ctx.LocOpenings, loc)
		iPRound = max(iPRound, q.Parts[i].Selectors[0].Round())
	}

	for i, part := range q.Parts {

		size := part.Size()

		// In theory, it would be more efficient to batch everything in a single
		// inner-product by size. But that would make the code harder to understand
		// and would require backtracking which result of the query corresponds to
		// which part of the Horner query.
		ip := comp.InsertInnerProduct(
			iPRound,
			ifaces.QueryIDf("HORNER_%v_COUNTING_%v_SRCNT_%v_%v", q.ID, size, comp.SelfRecursionCount, i),
			verifiercol.NewConstantCol(field.One(), size, ""),
			ctx.Q.Parts[i].Selectors,
		)

		ctx.CountingInnerProducts = append(ctx.CountingInnerProducts, ip)
	}

	comp.RegisterProverAction(iPRound, AssignHornerIP{ctx})
	comp.RegisterProverAction(q.Round, AssignHornerCtx{ctx})
	comp.RegisterVerifierAction(q.Round, &CheckHornerResult{HornerCtx: ctx})
}

func (a AssignHornerCtx) Run(run *wizard.ProverRuntime) {

	var (
		params = run.GetHornerParams(a.Q.ID)
		res    = fext.Zero()
		lock   sync.Mutex
	)

	parallel.Execute(len(a.Q.Parts), func(start, end int) {
		// for i, part := range a.Q.Parts {
		for i := start; i < end; i++ {

			part := a.Q.Parts[i]

			var (
				arity        = len(part.Selectors)
				datas        = make([]smartvectors.SmartVector, arity)
				selectors    = make([]smartvectors.SmartVector, arity)
				x            = part.X.GetValExt(run)
				n0           = params.Parts[i].N0
				count        = 0
				numRow       = part.Size()
				acc          = fext.Zero()
				accumulators = make([][]fext.Element, arity)
			)

			for k := 0; k < arity; k++ {
				board := part.Coefficients[k].Board()
				datas[k] = column.EvalExprColumn(run, board)
				selectors[k] = part.Selectors[k].GetColAssignment(run)
				accumulators[k] = make([]fext.Element, numRow)
			}

			for row := numRow - 1; row >= 0; row-- {
				for k := 0; k < arity; k++ {
					sel := selectors[k].Get(row)

					if sel.IsOne() {
						count++
						p := datas[k].GetExt(row)
						acc.Mul(&x, &acc)
						acc.Add(&acc, &p)
						accumulators[k][row] = acc
					} else if sel.IsZero() {
						accumulators[k][row] = acc
					} else {
						panic(fmt.Sprintf("selector is non-binary: part=%v selector[%d] row=%d value=%v", part.Name, k, row, sel.String()))
					}
				}
			}

			if n0+count != params.Parts[i].N1 {
				// To update once we merge with the "code 78" branch as it means that a constraint is not satisfied.
				utils.Panic("the counting of the 1s in the filter does not match the one in the local-opening: (%v-%v) != %v, part=%v", params.Parts[i].N1, n0, count, part.Name)
			}

			for k := 0; k < arity; k++ {
				run.AssignColumn(a.AccumulatingCols[i][k].GetColID(), smartvectors.NewRegularExt(accumulators[k]))
			}

			tmp := accumulators[arity-1][0]
			run.AssignLocalPointExt(a.LocOpenings[i].ID, tmp)

			if n0 > 0 {
				xN0 := new(fext.Element).Exp(x, big.NewInt(int64(n0)))
				tmp.Mul(&tmp, xN0)
			}

			if a.Q.Parts[i].SignNegative {
				tmp.Neg(&tmp)
			}

			lock.Lock()
			res.Add(&res, &tmp)
			lock.Unlock()
		}
	})

	// }

	if res != params.FinalResult {
		utils.Panic("the horner query %v is assigned a final result of %v but we recomputed %v\n", a.Q.ID, res.String(), params.FinalResult.String())
	}
}

func (a AssignHornerIP) Run(run *wizard.ProverRuntime) {

	for i := range a.Q.Parts {

		var (
			ip        = a.CountingInnerProducts[i]
			selectors = a.Q.Parts[i].Selectors
			res       = make([]fext.Element, len(selectors))
		)

		for j, selector := range selectors {
			sv := selector.GetColAssignment(run)
			switch sv := sv.(type) {
			case *smartvectors.Constant:
				// we just multiply the constant by the size
				size := field.NewElement(uint64(sv.Len()))
				cst := sv.Get(0)
				res[j].B0.A0.Mul(&cst, &size)
				continue
			}
			sel := sv.IntoRegVecSaveAlloc()
			vs := field.Vector(sel)
			res[j].B0.A0 = vs.Sum()
		}

		run.AssignInnerProduct(ip.ID, res...)
	}

}

func (c *CheckHornerResult) Run(run wizard.Runtime) error {

	var (
		hornerQuery  = c.Q
		hornerParams = run.GetHornerParams(hornerQuery.ID)
		res          = fext.Zero()
	)

	for i := range c.Q.Parts {

		var (
			ipQuery  = c.CountingInnerProducts[i]
			ipParams = run.GetInnerProductParams(c.CountingInnerProducts[i].ID)
			ipCount  = 0
		)

		for k := range ipQuery.Bs {
			y := ipParams.Ys[k]
			if !fext.IsBase(&y) {
				return fmt.Errorf("the y of the inner product %v is not a base element", ipQuery.ID)
			}
			ipCount += int(y.B0.A0.Uint64())
		}

		if hornerParams.Parts[i].N0+ipCount != hornerParams.Parts[i].N1 {
			return fmt.Errorf("the counting of the 1s in the filter does not match the one in the local-opening: (%v-%v) != %v", hornerParams.Parts[i].N1, hornerParams.Parts[i].N0, ipCount)
		}
	}

	// This loop is responsible for checking that the final result is correctly
	// computed by inspecting the local openings.
	for i, lo := range c.LocOpenings {

		var (
			tmp = run.GetLocalPointEvalParams(lo.ID).ExtY
			n0  = hornerParams.Parts[i].N0
			x   = hornerQuery.Parts[i].X.GetValExt(run)
		)

		if n0 > 0 {
			xN0 := new(fext.Element).Exp(x, big.NewInt(int64(n0)))
			tmp.Mul(&tmp, xN0)
		}

		if hornerQuery.Parts[i].SignNegative {
			tmp.Neg(&tmp)
		}

		res.Add(&res, &tmp)
	}

	if res != hornerParams.FinalResult {
		return fmt.Errorf("horner query has finalResult %v but we recovered %v", res.String(), hornerParams.FinalResult.String())
	}

	return nil
}

func (c *CheckHornerResult) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	hornerQuery := c.Q
	hornerParams := run.GetHornerParams(hornerQuery.ID)

	koalaAPI := koalagnark.NewAPI(api)
	res := koalaAPI.ZeroExt()

	for i := range c.Q.Parts {

		ipQuery := c.CountingInnerProducts[i]
		ipParams := run.GetInnerProductParams(c.CountingInnerProducts[i].ID)
		ipCount := koalaAPI.ZeroExt()

		for k := range ipQuery.Bs {
			ipCount = koalaAPI.AddExt(ipCount, ipParams.Ys[k])
		}

		// api.AssertIsEqual(api.Add(hornerParams.Parts[i].N0, ipCount), hornerParams.Parts[i].N1)
		// TODO @thomas fixme (ext vs base)
		extN0 := koalagnark.NewExtFromFrontendVar(hornerParams.Parts[i].N0)
		extN0 = koalaAPI.AddExt(extN0, ipCount)
		extN1 := koalagnark.NewExtFromFrontendVar(hornerParams.Parts[i].N1)
		koalaAPI.AssertIsEqualExt(extN0, extN1)
	}

	// This loop is responsible for checking that the final result is correctly
	// computed by inspecting the local openings.
	for i, lo := range c.LocOpenings {

		tmp := run.GetLocalPointEvalParams(lo.ID).ExtY
		n0 := hornerParams.Parts[i].N0
		x := hornerQuery.Parts[i].X.GetFrontendVariableExt(api, run)

		xN0 := koalaAPI.ExpVariableExponentExt(x, n0, 64)
		tmp = koalaAPI.MulExt(tmp, xN0)

		if hornerQuery.Parts[i].SignNegative {
			tmp = koalaAPI.NegExt(tmp)
		}

		res = koalaAPI.AddExt(res, tmp)
	}
	koalaAPI.AssertIsEqualExt(res, hornerParams.FinalResult)
}

func (c *CheckHornerResult) Skip() {
	c.skipped = true
}

func (c *CheckHornerResult) IsSkipped() bool {
	return c.skipped
}

// microAccumulate returns an atomic accumulator update expression.
//
// the returned expression evaluates to:
//
// ```
//
//	sel == 1 => acc.X + p
//	sel == 0 => acc
//
// ```
func microAccumulate(sel, acc, x, p any) *sym.Expression {
	return sym.Add(
		sym.Mul(
			sel,
			sym.Add(p, sym.Mul(x, acc)),
		),
		sym.Mul(
			sym.Sub(1, sel),
			acc,
		),
	)
}

func computeMicroAccumulate(sel field.Element, acc, x, p fext.Element) fext.Element {

	if sel.IsZero() {
		return acc
	}

	if sel.IsOne() {
		var tmp fext.Element
		tmp.Mul(&x, &acc)
		tmp.Add(&tmp, &p)
		return tmp
	}

	panic("selector is non-binary")
}

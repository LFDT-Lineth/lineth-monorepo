// Package splitextension implements the field extension to base field splitting compiler.
// It decomposes polynomials over field extensions into their base field components,
// enabling separate handling of extension elements during the proving and verification process.
//
// The compiler converts field extension polynomials (Fext) into base field polynomials
// by splitting each extension element into 4 base field limbs according to the field
// extension basis {1, u, v, uv}.
//
// Key concepts:
//   - ToSplitPolynomials: polynomials defined over field extensions that need decomposition
//   - SplittedPolynomials: resulting base field polynomials (4 per original polynomial)
//   - AlreadyOnBasePolynomials: polynomials already defined on the base field
//
// The compilation process:
//  1. Identifies field extension polynomials in the univariate query
//  2. Creates 4 new base field columns for each extension polynomial
//  3. Registers prover actions to split and evaluate polynomials
//  4. Registers verifier actions to reconstruct and verify the split claims
//
// The verifier checks that the original extension evaluation equals the reconstruction:
//
//	P(x) = P_0(x) + u*P_1(x) + v*P_2(x) + u*v*P_3(x)
//
// where P_i correspond to the imaginary parts of P in the extension basis.
package splitextension

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors_mixed"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

var (
	fextSplitTag     = "FEXT2BASE"
	errInconsistentX = errors.New("inconsistent evaluation point")
)

// AssignUnivProverAction implements the [wizard.ProverAction] interface and is
// responsible for assigning the new univariate query.
type AssignUnivProverAction struct {
	Ctx SplitCtx
}

// AssignSplitColumnProverAction implements the [wizard.ProverAction] interface
// and is responsible for assigning the splitted columns.
type AssignSplitColumnProverAction struct {
	Round int
	Ctx   SplitCtx
}

type VerifierCtx struct {
	Ctx SplitCtx
}

// SplitCtx context for splitting columns
type SplitCtx struct {

	// QueryFext Univariate query for field ext columns
	QueryFext query.UnivariateEval

	// QueryBaseField Univariate query for base field columns. The first
	// polynomials showing up in the query are the splitted ones and they are
	// followed by those that were already on the base field.
	QueryBaseField query.UnivariateEval

	// ToSplitPolynomials polynomials to split
	// i-th column goes to 4*i, 4*i+1, etc
	ToSplitPolynomials []ifaces.Column

	// SplitMap maps the split column to its "base" coordinates columns.
	SplitMap map[ifaces.ColID][4]ifaces.Column

	// AlreadyOnBasePolynomials is the list of columns that were already on
	// the base field in the order in which they show up in QueryFext.
	AlreadyOnBasePolynomials []ifaces.Column

	// SplittedPolynomials splitted polynomials
	SplittedPolynomials []ifaces.Column
}

func CompileSplitExtToBase(comp *wizard.CompiledIOP) {

	logrus.Trace("started naturalization compiler")
	defer logrus.Trace("finished naturalization compiler")

	// Expectedly, there should be only one query to compile as this compiler
	// is meant to be run after the MPTS compiler. This variable will detect
	// if more than one queries have been compiled.
	var numFoundQueries int

	// The compilation process is applied separately for each query
	for roundID := 0; roundID < comp.NumRounds(); roundID++ {
		for _, qName := range comp.QueriesParams.AllKeysAt(roundID) {

			if comp.QueriesParams.IsIgnored(qName) {
				continue
			}

			q, ok := comp.QueriesParams.Data(qName).(query.UnivariateEval)
			if !ok {
				utils.Panic("query %v has type %v expected only univariate", qName, reflect.TypeOf(q))
			}

			numFoundQueries++
			if numFoundQueries > 1 {
				utils.Panic("expected only one query to compile")
			}

			comp.QueriesParams.MarkAsIgnored(qName)

			basefieldQName := ifaces.QueryIDf("%s_%s", qName, fextSplitTag)

			ctx := SplitCtx{
				QueryFext:           q,
				ToSplitPolynomials:  make([]ifaces.Column, 0, len(q.Pols)),
				SplittedPolynomials: make([]ifaces.Column, 0, 4*len(q.Pols)),
				SplitMap:            make(map[ifaces.ColID][4]ifaces.Column),
			}

			for i, pol := range q.Pols {
				if pol.IsComposite() {
					utils.Panic("column %d should be natural", i)
				}

				if pol.IsBase() {
					ctx.AlreadyOnBasePolynomials = append(ctx.AlreadyOnBasePolynomials, pol)
					continue
				}

				comp.Columns.MarkAsIgnored(pol.GetColID())
				ctx.ToSplitPolynomials = append(ctx.ToSplitPolynomials, pol)
				polRound := pol.Round()

				splitMap := [4]ifaces.Column{}

				for j := 0; j < 4; j++ {
					splittedColName := ifaces.ColIDf("%s_%s_%d", pol.String(), fextSplitTag, 4*i+j)
					newSplitted := comp.InsertCommit(polRound, ifaces.ColID(splittedColName), pol.Size(), true)
					ctx.SplittedPolynomials = append(ctx.SplittedPolynomials, newSplitted)
					splitMap[j] = newSplitted
				}

				ctx.SplitMap[pol.GetColID()] = splitMap
			}

			ctx.QueryBaseField = comp.InsertUnivariate(
				roundID,
				basefieldQName,
				sortColumnsByRound(comp, slices.Concat(ctx.SplittedPolynomials, ctx.AlreadyOnBasePolynomials)),
			)

			for r := 0; r <= roundID; r++ {
				comp.RegisterProverAction(r, &AssignSplitColumnProverAction{
					Ctx:   ctx,
					Round: r,
				})
			}

			comp.RegisterProverAction(roundID, &AssignUnivProverAction{
				Ctx: ctx,
			})

			comp.RegisterVerifierAction(roundID, &VerifierCtx{
				Ctx: ctx,
			})
		}
	}
}

// Run implements the [wizard.ProverAction] interface
func (a *AssignSplitColumnProverAction) Run(runtime *wizard.ProverRuntime) {

	ctx := a.Ctx

	parallel.Execute(len(ctx.ToSplitPolynomials), func(start, end int) {
		for i := start; i < end; i++ {
			pol := ctx.ToSplitPolynomials[i]
			if pol.Round() != a.Round {
				continue
			}

			cc := pol.GetColAssignment(runtime)
			sv := splitVector(cc)

			runtime.AssignColumn(ctx.SplittedPolynomials[4*i].GetColID(), sv[0])
			runtime.AssignColumn(ctx.SplittedPolynomials[4*i+1].GetColID(), sv[1])
			runtime.AssignColumn(ctx.SplittedPolynomials[4*i+2].GetColID(), sv[2])
			runtime.AssignColumn(ctx.SplittedPolynomials[4*i+3].GetColID(), sv[3])
		}
	})
}

// Run implements ProverAction interface
func (pctx *AssignUnivProverAction) Run(runtime *wizard.ProverRuntime) {

	var (
		ctx            = pctx.Ctx
		evalFextParams = runtime.GetUnivariateParams(ctx.QueryFext.Name())
		x              = evalFextParams.ExtX
		svToEval       = make([]smartvectors.SmartVector, 0, len(ctx.QueryBaseField.Pols))
		evalMapFext    = make(map[ifaces.ColID]fext.Element)
		evalMapBase    = make(map[ifaces.ColID]fext.Element)
	)

	for i, col := range ctx.QueryFext.Pols {
		colID := col.GetColID()
		evalMapFext[colID] = evalFextParams.ExtYs[i]
	}

	// This loop evaluates and assigns the polynomials that have been split and
	// append their evaluation "y" the assignment to the evaluation on the new
	// query. The implementation relies on the fact that these polynomials are
	// positionned at the beginning of the list of evaluated polynomials in the
	// new query.
	for _, pol := range ctx.SplittedPolynomials {
		sv := pol.GetColAssignment(runtime)
		svToEval = append(svToEval, sv)
	}

	ySplitted := smartvectors_mixed.BatchEvaluateLagrange(svToEval, x)

	// This feeds the computed values to the evalMapBase
	for i, pol := range ctx.SplittedPolynomials {
		colID := pol.GetColID()
		evalMapBase[colID] = ySplitted[i]
	}

	yBase := make([]fext.Element, len(ctx.QueryBaseField.Pols))
	for i, pol := range ctx.QueryBaseField.Pols {

		// If the column is the result of splitting a field extension column.
		if y, isFound := evalMapBase[pol.GetColID()]; isFound {
			yBase[i] = y
			continue
		}

		// If the poly was already defined on base-field
		if y, isFound := evalMapFext[pol.GetColID()]; isFound {
			yBase[i] = y
			continue
		}

		panic("not found")
	}

	runtime.AssignUnivariateExt(ctx.QueryBaseField.QueryID, evalFextParams.ExtX, yBase...)
}

var (
	_one = field.One()
	// fieldExtensionBasis is the list of the field extension basis elements.
	// 1, u, v, uv. The limbs are decomposed according to this basis.
	fieldExtensionBasis = [4]fext.Element{
		{B0: extensions.E2{A0: _one}},
		{B0: extensions.E2{A1: _one}},
		{B1: extensions.E2{A0: _one}},
		{B1: extensions.E2{A1: _one}},
	}

	fieldExtensionBasisGnark = [4]koalagnark.Ext{
		koalagnark.NewExt(fieldExtensionBasis[0]),
		koalagnark.NewExt(fieldExtensionBasis[1]),
		koalagnark.NewExt(fieldExtensionBasis[2]),
		koalagnark.NewExt(fieldExtensionBasis[3]),
	}
)

func (vctx *VerifierCtx) Run(run wizard.Runtime) error {
	ctx := vctx.Ctx

	// checks that P(x) = P_0(x) + w*P_1(x) + w**2*P_2(x) + w**3*P_3(x)
	// where P is the polynomial to split, and the P_i are the splitted
	// polynomials, corrersponding to the imaginary parts of P
	var (
		evalFextParams      = run.GetUnivariateParams(ctx.QueryFext.QueryID)
		evalBaseFieldParams = run.GetUnivariateParams(ctx.QueryBaseField.QueryID)
		mainErr             error
		mapNewEval          = make(map[ifaces.ColID]fext.Element)
	)

	if evalBaseFieldParams.ExtX != evalFextParams.ExtX {
		return errInconsistentX
	}

	// This builds the evaluation map for the splitted polynomials
	for i, col := range ctx.QueryBaseField.Pols {
		colID := col.GetColID()
		mapNewEval[colID] = evalBaseFieldParams.ExtYs[i]
	}

	for i, col := range ctx.QueryFext.Pols {

		switch {
		case col.IsBase():
			// In that case, the Y value must the same as in the original
			// query.
			y, ok := mapNewEval[col.GetColID()]
			if !ok {
				panic("inconsistent query; the compiler should not have produced that")
			}

			if !y.Equal(&evalFextParams.ExtYs[i]) {
				err := fmt.Errorf("inconsistent evaluation claim, position [%v]: %v != %v", i, y.String(), evalBaseFieldParams.ExtYs[i].String())
				mainErr = errors.Join(mainErr, err)
			}

		default:
			// If the column is the result of splitting a field extension column.
			// In that case, we reconstruct the value from the limbs.
			var (
				reconstructedValue fext.Element
				splitted           = vctx.Ctx.SplitMap[col.GetColID()]
			)

			for j := 0; j < 4; j++ {
				var tmp fext.Element
				splittedYJ, ok := mapNewEval[splitted[j].GetColID()]
				if !ok {
					panic("inconsistent query; the compiler should not have produced that")
				}
				tmp.Mul(&fieldExtensionBasis[j], &splittedYJ)
				reconstructedValue.Add(&reconstructedValue, &tmp)
			}

			if !reconstructedValue.Equal(&evalFextParams.ExtYs[i]) {
				err := fmt.Errorf(
					"inconsistent evaluation claim, position [%v]: %v != %v", i,
					reconstructedValue.String(), evalFextParams.ExtYs[i].String(),
				)
				mainErr = errors.Join(mainErr, err)
			}
		}
	}

	if mainErr != nil {
		return mainErr
	}

	return nil
}

func (vctx *VerifierCtx) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	ctx := vctx.Ctx
	koalaAPI := koalagnark.NewAPI(api)

	// checks that P(x) = P_0(x) + w*P_1(x) + w**2*P_2(x) + w**3*P_3(x)
	// where P is the polynomial to split, and the P_i are the splitted
	// polynomials, corrersponding to the imaginary parts of P
	var (
		evalFextParams      = run.GetUnivariateParams(ctx.QueryFext.QueryID)
		evalBaseFieldParams = run.GetUnivariateParams(ctx.QueryBaseField.QueryID)
		mapNewEval          = make(map[ifaces.ColID]koalagnark.Ext)
	)

	koalaAPI.AssertIsEqualExt(evalBaseFieldParams.ExtX, evalFextParams.ExtX)

	// This builds the evaluation map for the splitted polynomials
	for i, col := range ctx.QueryBaseField.Pols {
		colID := col.GetColID()
		mapNewEval[colID] = evalBaseFieldParams.ExtYs[i]
	}

	for i, col := range ctx.QueryFext.Pols {

		switch {
		case col.IsBase():
			// In that case, the Y value must the same as in the original
			// query.
			y, ok := mapNewEval[col.GetColID()]
			if !ok {
				panic("inconsistent query; the compiler should not have produced that")
			}

			koalaAPI.AssertIsEqualExt(y, evalFextParams.ExtYs[i])

		default:
			// If the column is the result of splitting a field extension column.
			// In that case, we reconstruct the value from the limbs.
			var (
				reconstructedValue = koalagnark.NewExt(fext.Zero())
				splitted           = vctx.Ctx.SplitMap[col.GetColID()]
			)

			for j := 0; j < 4; j++ {
				splittedYJ := mapNewEval[splitted[j].GetColID()]
				tmp := koalaAPI.MulExt(fieldExtensionBasisGnark[j], splittedYJ)
				reconstructedValue = koalaAPI.AddExt(reconstructedValue, tmp)
			}

			koalaAPI.AssertIsEqualExt(reconstructedValue, evalFextParams.ExtYs[i])
		}
	}
}

func splitVector(sv smartvectors.SmartVector) [4]smartvectors.SmartVector {

	size := sv.Len()

	if smartvectors.IsBase(sv) {
		return [4]smartvectors.SmartVector{
			sv,
			smartvectors.NewConstant(field.Zero(), size),
			smartvectors.NewConstant(field.Zero(), size),
			smartvectors.NewConstant(field.Zero(), size),
		}
	}

	// if it's a constant (ext) we don't need to allocate.
	if _, ok := sv.(*smartvectors.ConstantExt); ok {
		elmt := sv.GetExt(0)
		r0 := smartvectors.NewConstant(elmt.B0.A0, size)
		r1 := smartvectors.NewConstant(elmt.B0.A1, size)
		r2 := smartvectors.NewConstant(elmt.B1.A0, size)
		r3 := smartvectors.NewConstant(elmt.B1.A1, size)
		return [4]smartvectors.SmartVector{r0, r1, r2, r3}
	}

	r0 := make([]field.Element, size)
	r1 := make([]field.Element, size)
	r2 := make([]field.Element, size)
	r3 := make([]field.Element, size)

	for i := 0; i < size; i++ {
		elmt := sv.GetExt(i)
		r0[i] = elmt.B0.A0
		r1[i] = elmt.B0.A1
		r2[i] = elmt.B1.A0
		r3[i] = elmt.B1.A1
	}
	return [4]smartvectors.SmartVector{
		smartvectors.NewRegular(r0),
		smartvectors.NewRegular(r1),
		smartvectors.NewRegular(r2),
		smartvectors.NewRegular(r3),
	}
}

// sortColumnsByRound generates a sorted list of columns by round, preserving
// the original order (e.g. stable sort). The precomputed columns are sorted
// as if their round number was -1.
func sortColumnsByRound(comp *wizard.CompiledIOP, columns []ifaces.Column) []ifaces.Column {

	res := slices.Clone(columns)

	sort.SliceStable(res, func(i, j int) bool {
		var (
			roundI = res[i].Round()
			roundJ = res[j].Round()
		)
		if comp.Precomputed.Exists(res[i].GetColID()) {
			roundI = -1
		}
		if comp.Precomputed.Exists(res[j].GetColID()) {
			roundJ = -1
		}
		return roundI < roundJ
	})

	return res
}

package distributed

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// ModuleTranslator is a utily struct wrapping a [wizard.CompiledIOP] and
// implements the logic to build it and to translate it.
type ModuleTranslator struct {
	Disc *StandardModuleDiscoverer
	Wiop *wizard.CompiledIOP
}

// InsertColumn inserts a new column in the target compiled IOP. The column
// name the column names is kept identical to the original. The size is
// adjusted to match the one of the module following the [*StandardModuleDiscoverer].
//
// The function returns the inserted column. If the column had alreadu been
// inserted the function is a no-op and returns the already inserted one.
//
// The function assumes and asserts that col.Round() == 0. Otherwise, it panics.
//
// IsLPP indicates if the column is part of the LPP part of the module and is
// inserted at round 1. Otherwise, it is inserted at round 0.
func (mt *ModuleTranslator) InsertColumn(col column.Natural, atRound int) ifaces.Column {

	if col.Round() != 0 {
		utils.Panic("cannot translate a column with non-zero round %v", col.Round())
	}

	if col.Status() == column.Precomputed || col.Status() == column.VerifyingKey {
		panic("use [InsertPrecomputed] for precomputed columns")
	}

	if mt.Wiop.Columns.Exists(col.ID) {
		return mt.Wiop.Columns.GetHandle(col.ID)
	}

	newSize := NewSizeOfColumn(mt.Disc, col)
	return mt.Wiop.InsertColumn(atRound, col.ID, newSize, col.Status())
}

// InsertPrecomputed is as [InsertColumn] but specificially works for precomputed
// columns.
func (mt *ModuleTranslator) InsertPrecomputed(col column.Natural, data smartvectors.SmartVector) ifaces.Column {

	if col.Round() != 0 {
		utils.Panic("cannot translate a column with non-zero round %v", col.Round())
	}

	if col.Status() != column.Precomputed && col.Status() != column.VerifyingKey {
		panic("use [InsertPrecomputed] for precomputed columns")
	}

	if mt.Wiop.Columns.Exists(col.ID) {
		return mt.Wiop.Columns.GetHandle(col.ID)
	}

	mt.Wiop.Precomputed.InsertNew(col.ID, data)
	return mt.Wiop.InsertColumn(0, col.ID, data.Len(), col.Status())
}

// TranslateColumn returns an equivalent column from the new module. The
// function panics if the column cannot be resolved. It will happen if the
// column has an expected type or is defined from not resolvable items.
//
// The function will also not support [verifiercol.VerifierCol] columns and
// will panic if encountering them.
func (mt *ModuleTranslator) TranslateColumn(col ifaces.Column) ifaces.Column {
	return mt.TranslateColumnWithSizeHint(col, -1)
}

// TranslateColumnWithSizeHint is as [TranslateColumn] but it supports
// [verifiercol.ConstCol] and uses the provided size-hint to give them the
// proper size. The function also supports shifting of constant columns and will
// preserve that structure.
//
// Passing a negative hint is equivalent to calling [ModuleTranslator.TranslateColumn].
func (mt *ModuleTranslator) TranslateColumnWithSizeHint(col ifaces.Column, sizeHint int) ifaces.Column {

	switch c := col.(type) {
	case column.Natural:
		return mt.Wiop.Columns.GetHandle(c.ID)
	case column.Shifted:
		return column.Shifted{
			Parent: mt.TranslateColumnWithSizeHint(c.Parent, sizeHint),
			Offset: c.Offset,
		}
	case verifiercol.ConstCol:
		if sizeHint < 0 {
			utils.Panic("called TranslateColumnWithSizeHint with a negative offset on a constant col")
		}
		return verifiercol.NewConstantCol(c.F, sizeHint, "")
	default:
		utils.Panic("unexpected type of column: type: %T, name: %v", col, col.GetColID())
	}

	return nil
}

// TranslateColumnList returns a list of equivalent columns from the new module.
// The function panics if the column cannot be resolved. It will happen if the
// column has an expected type or is defined from not resolvable items.
func (mt *ModuleTranslator) TranslateColumnList(cols []ifaces.Column) []ifaces.Column {
	res := make([]ifaces.Column, len(cols))
	for i := range res {
		res[i] = mt.TranslateColumn(cols[i])
	}
	return res
}

// TranslateExpressionWithSizeHint is as [TranslateExpression] but uses a hint
// to help translating the sub-components.
func (mt *ModuleTranslator) TranslateExpressionWithHint(expr *symbolic.Expression, sizeHint int) *symbolic.Expression {
	return expr.ReconstructBottomUpSingleThreaded(
		func(e *symbolic.Expression, children []*symbolic.Expression) *symbolic.Expression {
			switch op := e.Operator.(type) {
			case symbolic.Variable:
				switch m := op.Metadata.(type) {
				case ifaces.Accessor:
					newAcc := mt.TranslateAccessor(m)
					return symbolic.NewVariable(newAcc)
				case ifaces.Column:
					// @alex:
					//
					// There used to a tentative optimization where we would
					// detect and simplify [verifiercol.ConstantCol] into
					// constant expression but we removed it because sometime
					// we have shifting of verifiercol inside of expressions as
					// a way to cancel them at the first or the last position.
					// This is hacky but doing the conversion would remove that
					// cancellation so we removed it.
					newCol := mt.TranslateColumnWithSizeHint(m, sizeHint)
					return symbolic.NewVariable(newCol)
				case coin.Info:
					newCoin := mt.TranslateCoin(m)
					return symbolic.NewVariable(newCoin)
				default:
					return e.SameWithNewChildren(children)
				}
			default:
				return e.SameWithNewChildren(children)
			}
		},
	)
}

// TranslateExpression returns an expression corresponding to the provided
// expression but in term of the input module. The function will attempt resolving
// the domain size of the new expression but might fail if the expression only
// contains [verifiercol.ConstCol]. In that case, it is better to use TranslateExpressionWithHint.
func (mt *ModuleTranslator) TranslateExpression(expr *symbolic.Expression) *symbolic.Expression {
	// The initial step is there to capture the column size so that we can
	// translate verifiercol.ConstCol properly. If the expression contains only
	// verifiercol, the sizeHint value will be left as -1 and this will panic
	// when attempting to call [TranslateColumn] inside the function call.
	cols := column.ColumnsOfExpression(expr)
	sizeHint := -1
	for _, col := range cols {
		root := column.RootParents(col)
		if nat, isNat := root.(column.Natural); isNat {
			sizeHint = mt.Disc.NewSizeOf(nat)
			break
		}
	}
	return mt.TranslateExpressionWithHint(expr, sizeHint)
}

// TranslateExpressionList returns a list of equivalent expressions from the new
// module.
func (mt *ModuleTranslator) TranslateExpressionList(exprs []*symbolic.Expression) []*symbolic.Expression {
	res := make([]*symbolic.Expression, len(exprs))
	for i := range res {
		res[i] = mt.TranslateExpression(exprs[i])
	}
	return res
}

// TranslateCoin returns the equivalent coin from the new module.
// The function looks for a coin with the same name and inserts it
// as a [coin.FieldFromSeed] if it is not found.
func (mt *ModuleTranslator) TranslateCoin(info coin.Info) coin.Info {
	if !mt.Wiop.Coins.Exists(info.Name) {
		mt.Wiop.InsertCoin(1, info.Name, coin.FieldFromSeed)
	}
	return mt.Wiop.Coins.Data(info.Name)
}

// TranslateAccessor returns an equivalent from the new module.
func (mt *ModuleTranslator) TranslateAccessor(acc ifaces.Accessor) ifaces.Accessor {

	switch a := acc.(type) {

	case *accessors.FromConstAccessor:
		// The constant accessor has no information to translate
		// (it's just a field.Element) so we just return it.
		return a

	case *accessors.FromExprAccessor:
		newExpr := mt.TranslateExpression(a.Expr)
		return accessors.NewFromExpression(newExpr, a.ExprName)

	case *accessors.FromCoinAccessor:
		newCoin := mt.TranslateCoin(a.Info)
		return accessors.NewFromCoin(newCoin)

	case *accessors.FromPublicColumn:
		newCol := mt.TranslateColumn(a.Col)
		return accessors.NewFromPublicColumn(newCol, a.Pos)

	case *accessors.FromLocalOpeningYAccessor:
		newLo := mt.TranslateQueryParam(a.Q).(query.LocalOpening)
		return accessors.NewLocalOpeningAccessor(newLo, a.QRound)

	default:
		utils.Panic("unexpected type of accessor: %T", acc)
	}

	return nil
}

// TranslateQueryParam returns an equivalent query from the new module.
// The function will only look for queries with the same name.
func (mt *ModuleTranslator) TranslateQueryParam(query ifaces.Query) ifaces.Query {
	return mt.Wiop.QueriesParams.Data(query.Name())
}

// InsertPlonkInWizard inserts a new PlonkInWizard query in the target compiled IOP
// by translating all the related columns
func (mt *ModuleTranslator) InsertPlonkInWizard(oldQuery *query.PlonkInWizard) *query.PlonkInWizard {

	newQuery := query.NewPlonkInWizard(
		oldQuery.ID,
		mt.TranslateColumn(oldQuery.Data),
		mt.TranslateColumn(oldQuery.Selector),
		oldQuery.Circuit,
		oldQuery.PlonkOptions,
	)

	mt.Wiop.InsertPlonkInWizard(newQuery)
	return newQuery
}

// InsertLogDerivative inserts a new LogDerivative query in the target compiled IOP
// by translating all the related columns
func (mt *ModuleLPP) InsertLogDerivative(
	round int,
	id ifaces.QueryID,
	logDerivativeArgs []query.LogDerivativeSumPart,
) query.LogDerivativeSum {

	resInputs := []query.LogDerivativeSumPart{}

	for _, part := range logDerivativeArgs {

		size := NewSizeOfList(mt.Disc, part.Num, part.Den)

		mt.addCoinFromExpression(part.Num)
		mt.addCoinFromExpression(part.Den)

		resInputs = append(resInputs, query.LogDerivativeSumPart{
			Size: size,
			Num:  mt.TranslateExpressionWithHint(part.Num, size),
			Den:  mt.TranslateExpressionWithHint(part.Den, size),
			Name: part.Name,
		})
	}

	return mt.Wiop.InsertLogDerivativeSum(
		round,
		id,
		query.LogDerivativeSumInput{Parts: resInputs},
	)
}

// InsertGrandProduct inserts a new GrandProduct query in the target compiled IOP
// by translating all the related columns
func (mt *ModuleLPP) InsertGrandProduct(
	round int,
	id ifaces.QueryID,
	args [][2]*symbolic.Expression,
) query.GrandProduct {

	resInputs := make(map[int]*query.GrandProductInput)

	for _, numDenPair := range args {

		var (
			num  = numDenPair[0]
			den  = numDenPair[1]
			size = NewSizeOfList(mt.Disc, num, den)
		)

		mt.addCoinFromExpression(num)
		mt.addCoinFromExpression(den)

		if _, hasSize := resInputs[size]; !hasSize {
			resInputs[size] = &query.GrandProductInput{
				Size: size,
			}
		}

		newInp := resInputs[size]
		newInp.Numerators = append(newInp.Numerators, mt.TranslateExpression(num))
		newInp.Denominators = append(newInp.Denominators, mt.TranslateExpression(den))
	}

	return mt.Wiop.InsertGrandProduct(round, id, resInputs)
}

// InsertHorner inserts a new Horner query in the target compiled IOP
// by translating all the related columns
func (mt *ModuleLPP) InsertHorner(
	round int,
	id ifaces.QueryID,
	parts []query.HornerPart,
) query.Horner {

	newParts := []query.HornerPart{}

	for _, oldPart := range parts {

		newPart := query.HornerPart{
			Name:         oldPart.Name,
			Coefficients: mt.TranslateExpressionList(oldPart.Coefficients),
			SignNegative: oldPart.SignNegative,
			Selectors:    mt.TranslateColumnList(oldPart.Selectors),
			X:            mt.TranslateAccessor(oldPart.X),
		}

		mt.addCoinFromExpression(newPart.Coefficients...)
		mt.addCoinFromAccessor(newPart.X)
		newParts = append(newParts, newPart)
	}

	return mt.Wiop.InsertHornerQuery(round, id, newParts)
}

func (mt *ModuleTranslator) InsertCoin(name coin.Name, round int) {

	if mt.Wiop.Coins.Exists(name) {
		return
	}

	newInfo := coin.NewInfo(name, coin.FieldFromSeed, round)
	mt.Wiop.Coins.AddToRound(round, name, newInfo)
}

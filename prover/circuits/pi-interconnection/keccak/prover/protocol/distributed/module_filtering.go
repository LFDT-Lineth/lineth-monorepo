package distributed

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// FilteredModuleInputs represents the items collected in order to
// instanstiate a particular module.
//
// The items that are collected in the struct are those of the original
// CompiledIOP and not the distributed ones.
type FilteredModuleInputs struct {

	// ModuleIndex is the integer index of the module
	ModuleIndex int

	// ModuleName is the name of the current module.
	ModuleName ModuleName

	// LogDerivativeArgs are the arguments of the log-derivative
	// query (expectedly, the only one) for the current module.
	//
	// Each entry of the slice corresponds to a pair of numerator
	// and denominator in that order. Unlike in the log-derivative
	// query. The pairs are not sorted by size because the sizes
	// of each pair can be potentially changed during the distribution
	// phase.
	LogDerivativeArgs []query.LogDerivativeSumPart

	// GrandProductArgs are the arguments of the grand product
	// query (expectedly, the only one) for the current module.
	GrandProductArgs [][2]*symbolic.Expression

	// HornerArgs are the [query.HornerParts] of the Horner queries
	// (expetedly, the one one) for the current module.
	HornerArgs []query.HornerPart

	// GlobalConstraints are the global constraints
	// for the current module.
	GlobalConstraints []*query.GlobalConstraint

	// LocalConstraints are the local constraints for the current
	// module
	LocalConstraints []*query.LocalConstraint

	// LocalOpenings are the list of the local-opening queries
	// found for a module.
	LocalOpenings []*query.LocalOpening

	// PlonkInWizard are the list of the PlonkInWizard queries
	// found for a module.
	PlonkInWizard []*query.PlonkInWizard

	// Range are the [query.Range] constraints to apply for the
	// current module.
	Range []*query.Range

	// ColumnsLPP are the columns that are touched by the LPP
	// part of the module
	ColumnsLPP    []*column.Natural
	ColumnsLPPSet map[ifaces.ColID]struct{}

	// Columns is the list of all the columns (LPP and GL) in the
	// module.
	Columns    []*column.Natural
	ColumnsSet map[ifaces.ColID]struct{}

	// ColumnsPrecomputed is a map of the precomputed columns found
	// in the original compiled-IOP mapping to their assignment.
	ColumnsPrecomputed map[ifaces.ColID]ifaces.ColAssignment

	// PublicInputs lists the public-inputs of the in-bound original
	// compiled-IOP. The vector is a list of accessor and has one
	// entry for EVERY public-input of the input module. However, the
	// positions corresponding to public-inputs that are not related
	// to the said module are replaced with 'nil'.
	PublicInputs []wizard.PublicInput

	// Disc is the module discoverer used to determine the module's
	// scope
	Disc *StandardModuleDiscoverer
}

// FilterCompiledIOP returns a FilteredModuleInputs for the given compiled IOP
// corresponding to the filter parameters. comp should the original (prepared)
// compiled IOP.
//
// The first step is to collect all of the columns that are relevant to the module
// in [Columns] and [ColumsSet]. All the columns must have a round of zero in
// principle, the asserts it.
//
// After that, we scan all the queries and we check if:
//
//   - the queries are all of the expected type (global/local/log-derivative/grand-product/range)
//   - it panics otherwise
//   - their components resolve to the target module
//   - they are rejected otherwise
//   - there should not be any verifier action
func (mf moduleFilter) FilterCompiledIOP(comp *wizard.CompiledIOP) FilteredModuleInputs {

	var (
		// fmt is as in filterModuleInputs. This is the return of the
		// function.
		fmi = FilteredModuleInputs{
			ModuleName:         mf.Module,
			ColumnsLPPSet:      map[ifaces.ColID]struct{}{},
			ColumnsSet:         map[ifaces.ColID]struct{}{},
			Disc:               mf.Disc,
			ColumnsPrecomputed: map[ifaces.ColID]ifaces.ColAssignment{},
		}

		// columnNames lists the columns in comp.
		columnNames       = comp.Columns.AllKeys()
		queryParamsName   = comp.QueriesParams.AllUnignoredKeys()
		queryNoParamsName = comp.QueriesNoParams.AllUnignoredKeys()
	)

	// This sets the correct module index
	for i, name := range mf.Disc.ModuleList() {
		if name == mf.Module {
			fmi.ModuleIndex = i
			break
		}
	}

	for _, columnName := range columnNames {

		col := comp.Columns.GetHandle(columnName)
		resolvedModule := ModuleOfColumn(mf.Disc, col)

		if col.Round() != 0 {
			utils.Panic("all the columns must have a round of 0, colName=%v round=%v", columnName, col.Round())
		}

		resolvedModule.MustBeResolved()
		if resolvedModule != mf.Module {
			continue
		}
		fmi.addColumn(col)

		if data, isPrecomp := comp.Precomputed.TryGet(columnName); isPrecomp {
			fmi.ColumnsPrecomputed[columnName] = data
		}
	}

	for _, qName := range queryParamsName {

		q_ := comp.QueriesParams.Data(qName)

		switch q := q_.(type) {

		case query.LocalOpening:
			resolvedModule := ModuleOfColumn(mf.Disc, q.Pol)
			resolvedModule.MustBeResolved()
			if resolvedModule != mf.Module {
				continue
			}
			fmi.LocalOpenings = append(fmi.LocalOpenings, &q)

		case query.LogDerivativeSum:
			if len(fmi.LogDerivativeArgs) > 0 {
				utils.Panic("expected there would be only one log-derivative-sum query")
			}
			args := mf.FilterLogDerivativeInputs(&q)
			// This loops adds the involved columns in the lpp set
			for i := range args {
				for _, e := range []*symbolic.Expression{args[i].Num, args[i].Den} {
					cols := column.ColumnsOfExpression(e)
					roots := column.RootsOf(cols, true)
					for _, root := range roots {
						fmi.addColumnLPP(root)
					}
				}
			}
			fmi.LogDerivativeArgs = args

		case query.GrandProduct:
			if len(fmi.GrandProductArgs) > 0 {
				utils.Panic("expected there would be only one grand-product query")
			}
			args := mf.FilterGrandProductInputs(&q)
			for i := range args {
				for j := range args[i] {
					cols := column.ColumnsOfExpression(args[i][j])
					roots := column.RootsOf(cols, true)
					for _, root := range roots {
						fmi.addColumnLPP(root)
					}
				}
			}
			fmi.GrandProductArgs = args

		case *query.Horner:
			if len(fmi.HornerArgs) > 0 {
				utils.Panic("expected there would be only one horner query")
			}
			args := mf.FilterHornerParts(q)
			for i := range args {
				cols := []ifaces.Column{}
				for k := range args[i].Selectors {
					cols = append(cols, column.ColumnsOfExpression(args[i].Coefficients[k])...)
					cols = append(cols, args[i].Selectors[k])
				}
				roots := column.RootsOf(cols, true)
				for _, root := range roots {
					fmi.addColumnLPP(root)
				}
			}
			fmi.HornerArgs = args

		default:
			utils.Panic("unexpected type of query: type=%T name=%v", q, qName)
		}
	}

	for _, qName := range queryNoParamsName {

		q_ := comp.QueriesNoParams.Data(qName)

		switch q := q_.(type) {

		case query.GlobalConstraint:
			resolvedModule := ModuleOfExpr(mf.Disc, q.Expression)
			if resolvedModule == NoModuleFound {
				fmt.Printf("[moduleFilter.FilterCompiledIOP] q.Expression = %v\n", q.Name())
			}
			resolvedModule.MustBeResolved()
			if resolvedModule != mf.Module {
				continue
			}
			fmi.GlobalConstraints = append(fmi.GlobalConstraints, &q)

		case query.LocalConstraint:
			resolvedModule := ModuleOfExpr(mf.Disc, q.Expression)
			resolvedModule.MustBeResolved()
			if resolvedModule != mf.Module {
				continue
			}
			fmi.LocalConstraints = append(fmi.LocalConstraints, &q)

		case query.Range:
			resolvedModule := ModuleOfColumn(mf.Disc, q.Handle)
			resolvedModule.MustBeResolved()
			if resolvedModule != mf.Module {
				continue
			}
			fmi.Range = append(fmi.Range, &q)

		case *query.PlonkInWizard:
			items := []ifaces.Column{q.Selector, q.Data}
			resolvedModule := ModuleOfList(mf.Disc, items...)
			resolvedModule.MustBeResolved()
			if resolvedModule != mf.Module {
				continue
			}
			fmi.PlonkInWizard = append(fmi.PlonkInWizard, q)

		default:
			utils.Panic("unexpected type of query: type=%T name=%v", q, qName)
		}
	}

	for i := range comp.PublicInputs {

		originalPublicInput := comp.PublicInputs[i]
		resolvedModule := ModuleOfAccessor(mf.Disc, originalPublicInput.Acc)
		resolvedModule.MustBeResolved()

		newPublicInputs := wizard.PublicInput{
			Name: originalPublicInput.Name,
			Acc:  originalPublicInput.Acc,
		}

		if resolvedModule != mf.Module {
			newPublicInputs.Acc = nil
		}

		fmi.PublicInputs = append(fmi.PublicInputs, newPublicInputs)
	}

	return fmi
}

// moduleFilter is a struct implementing utility methods allowing
// to filter items related to a target module. It is used to help
// constructing ModuleToDistribute.
//
// The struct is not exported and should not be.
type moduleFilter struct {
	Module ModuleName
	Disc   *StandardModuleDiscoverer
}

// addGL adds a column to the GL part of the module and returns if
// the column was already present or not.
func (m *FilteredModuleInputs) addColumn(col ifaces.Column) bool {

	nat, isNat := col.(column.Natural)
	if !isNat {
		utils.Panic("expected a [%T], got [%T]", column.Natural{}, col)
	}

	if _, ok := m.ColumnsSet[col.GetColID()]; ok {
		return true
	}

	m.Columns = append(m.Columns, &nat)
	m.ColumnsSet[col.GetColID()] = struct{}{}
	return false
}

// addLPP adds a column to the LPP part of the module and returns if
// the column was already present or not. The function will skip and
// return false if called on a [verifiercol.ConstCol] column. If the
// column is neither a [verifiercol.ConstCol] nor a [column.Natural],
// the function will panic.
func (m *FilteredModuleInputs) addColumnLPP(col ifaces.Column) bool {

	nat, isNat := col.(column.Natural)
	if !isNat {
		utils.Panic("expected a [%T], got [%T]. name=%v", column.Natural{}, col, col.GetColID())
	}

	if _, ok := m.ColumnsLPPSet[col.GetColID()]; ok {
		return true
	}

	m.ColumnsLPP = append(m.Columns, &nat)
	m.ColumnsLPPSet[col.GetColID()] = struct{}{}
	return false
}

// FilterLogDerivativeInputs scans q and look for the pairs of numerator
// and denominator who belong to the target module. The function panics if it
// fails to resolve the target module of a pair of numerator/denominator.
//
// The matched pairs of numerators/denominators are returned as slices of
// [numerator, denominator] of mixed sizes. The function is deterministic and
// the expressions are not "translated": meaning that the returned expressions
// are exactly those found in the query and are not redefined in term of items
// from the target module.
//
// The returned pair are sorted by increasing size and then by order of appearance
// in each [query.LogDerivativeInput]
//
// The function returns nil if no column are matched.
func (filter moduleFilter) FilterLogDerivativeInputs(q *query.LogDerivativeSum) []query.LogDerivativeSumPart {

	// It's important to sort to ensure that the iteration happens in
	// deterministic order.
	res := []query.LogDerivativeSumPart{}

	for _, part := range q.Inputs.Parts {

		resolvedMod := ModuleOfList(filter.Disc, part.Num, part.Den)
		resolvedMod.MustBeResolved()
		if resolvedMod != filter.Module {
			continue
		}

		res = append(res, part)
	}

	return res
}

// FilterGrandProductInputs scans q and look for pairs or numerators and
// denominators who belongs to the target module. The function panics if it
// fails to resolve the target module of a pair of numerator/denominator.
//
// The matched pairs of numerators/denominators are returned as slices of
// [numerator, denominator] of mixed sizes. The function is deterministic and
// the expressions are not "translated": meaning that the returned expressions
// are exactly those found in the query and are not redefined in term of items
// from the target module.
//
// The returned pair are sorted by increasing size and then by order of appearance
// in each [query.LogDerivativeInput]
//
// The function returns nil if no column are matched.
func (filter moduleFilter) FilterGrandProductInputs(q *query.GrandProduct) [][2]*symbolic.Expression {

	// It's important to sort to ensure that the iteration happens in
	// deterministic order.
	sizes := utils.SortedKeysOf(q.Inputs, func(a, b int) bool { return a < b })

	var res [][2]*symbolic.Expression

	for _, size := range sizes {

		gdProductInput := q.Inputs[size]

		for i := range gdProductInput.Numerators {

			resolvedMod := ModuleOfList(
				filter.Disc,
				gdProductInput.Numerators[i],
				gdProductInput.Denominators[i],
			)

			resolvedMod.MustBeResolved()

			if resolvedMod != filter.Module {
				continue
			}

			res = append(
				res,
				[2]*symbolic.Expression{
					gdProductInput.Numerators[i],
					gdProductInput.Denominators[i],
				},
			)
		}
	}

	return res
}

// FilterHornerParts returns a list of [query.HornerPart] who can be resolved
// to the current module. The function panics if one part could not be resolved.
func (filter moduleFilter) FilterHornerParts(q *query.Horner) []query.HornerPart {

	var res = make([]query.HornerPart, 0, len(q.Parts))

	for _, part := range q.Parts {

		exprList := []*symbolic.Expression{}
		for k := range part.Selectors {
			exprList = append(exprList, part.Coefficients[k])
			exprList = append(exprList, symbolic.NewVariable(part.Selectors[k]))
		}

		resolvedMod := ModuleOfList(filter.Disc, exprList...)
		resolvedMod.MustBeResolved()
		if resolvedMod != filter.Module {
			continue
		}

		res = append(res, part)
	}

	return res
}

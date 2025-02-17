package arithmetization

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/consensys/go-corset/pkg/air"
	"github.com/consensys/go-corset/pkg/schema"
	"github.com/consensys/go-corset/pkg/schema/assignment"
	"github.com/consensys/go-corset/pkg/schema/constraint"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// schemaScanner is a transient scanner structure whose goal is to port the
// content of an [air.Schema] inside of a pre-initialized [wizard.CompiledIOP]
type schemaScanner struct {
	LimitMap           map[string]int
	Comp               *wizard.CompiledIOP
	Schema             *air.Schema
	Modules            []schema.Module
	InterleavedColumns map[string]*assignment.Interleaving
}

// Define registers the arithmetization from a corset air.Schema and trace limits
// from config.
func Define(comp *wizard.CompiledIOP, schema *air.Schema, limits *config.TracesLimits) {

	scanner := &schemaScanner{
		LimitMap:           mapModuleLimits(limits),
		Comp:               comp,
		Schema:             schema,
		Modules:            schema.Modules().Collect(),
		InterleavedColumns: map[string]*assignment.Interleaving{},
	}

	scanner.scanColumns()
	scanner.scanConstraints()
}

// scanColumns scans the column declaration of the corset [air.Schema] into the
// [wizard.CompiledIOP] object.
func (s *schemaScanner) scanColumns() {

	var (
		schCol  = s.Schema.Columns().Collect()
		schAssi = s.Schema.Assignments().Collect()
	)

	for _, colAssi := range schAssi {
		if il, isIL := colAssi.(*assignment.Interleaving); isIL {
			col := il.Columns().Nth(0) // There is only a single column here
			wName := wizardName(getModuleNameFromColumn(s.Schema, col), col.Name)
			s.InterleavedColumns[wName] = il
		}
	}

	for _, colDecl := range schCol {

		var (
			name        = wizardName(getModuleNameFromColumn(s.Schema, colDecl), colDecl.Name)
			ctx         = colDecl.Context
			module      = s.Modules[ctx.Module()]
			moduleLimit = s.LimitMap[module.Name]
			mult        = ctx.LengthMultiplier()
			size        = int(mult) * moduleLimit
		)

		// Adjust the size for interleaved columns and their permuted versions.
		// Since these are the only columns from corset with a non-power-of-two size.
		if !utils.IsPowerOfTwo(size) {
			newSize := utils.NextPowerOfTwo(int(mult) * moduleLimit)
			logrus.Debug("Adjusting size for column: ", name, " in module: ", module.Name, " from ", size, " to ", newSize)
			size = newSize
		}

		// #nosec G115 -- this bound will not overflow
		s.Comp.InsertCommit(0, ifaces.ColID(name), size)
	}
}

// scanConstraints scans the constraint declaration from a corset schema into
// the [wizard.CompiledIOP] object.
func (s *schemaScanner) scanConstraints() {

	corsetCSs := s.Schema.Constraints().Collect()

	for _, corsetCS := range corsetCSs {
		name := fmt.Sprintf("%v", corsetCS.Lisp(s.Schema).String(false))
		if s.Comp.QueriesNoParams.Exists(ifaces.QueryID(name)) {
			continue
		}
		s.addConstraintInComp(name, corsetCS)
	}
}

// addCsInComp adds a corset constraint into the [wizard.CompiledIOP]
func (s *schemaScanner) addConstraintInComp(name string, corsetCS schema.Constraint) {

	switch cs := corsetCS.(type) {

	case air.LookupConstraint:

		var (
			numCol   = len(cs.Sources)
			cSources = cs.Sources
			cTargets = cs.Targets
			wSources = make([]ifaces.Column, numCol)
			wTargets = make([]ifaces.Column, numCol)
		)

		// this will panic over interleaved columns, we can debug that later
		for i := 0; i < numCol; i++ {
			wSources[i] = s.compColumnByCorsetID(cSources[i].Column)
			wTargets[i] = s.compColumnByCorsetID(cTargets[i].Column)
		}

		s.Comp.InsertInclusion(0, ifaces.QueryID(name), wTargets, wSources)

	case *constraint.PermutationConstraint:

		var (
			numCol   = len(cs.Sources)
			cSources = cs.Sources
			cTargets = cs.Targets
			wSources = make([]ifaces.Column, numCol)
			wTargets = make([]ifaces.Column, numCol)
		)

		// this will panic over interleaved columns, we can debug that later
		for i := 0; i < numCol; i++ {
			wSources[i] = s.compColumnByCorsetID(cSources[i])
			wTargets[i] = s.compColumnByCorsetID(cTargets[i])
		}

		s.Comp.InsertPermutation(0, ifaces.QueryID(name), wTargets, wSources)

	case air.VanishingConstraint:

		var (
			wExpr  = s.castExpression(cs.Constraint.Expr)
			wBoard = wExpr.Board()
			wMeta  = wBoard.ListVariableMetadata()
		)

		if len(wMeta) == 0 {
			// Sometime, it just so happens that corset gives constant expressions
			return
		}

		if cs.Domain.IsEmpty() {
			s.Comp.InsertGlobal(0, ifaces.QueryID(name), wExpr)
			return
		}

		domain := cs.Domain.Unwrap()

		// This applies the shift to all the leaves of the expression
		wExpr = wExpr.ReconstructBottomUp(
			func(e *symbolic.Expression, children []*symbolic.Expression) (new *symbolic.Expression) {

				v, isV := e.Operator.(symbolic.Variable)
				if !isV {
					return e.SameWithNewChildren(children)
				}

				col, isCol := v.Metadata.(ifaces.Column)
				if !isCol {
					return e
				}

				return symbolic.NewVariable(column.Shift(col, domain))
			},
		)

		s.Comp.InsertLocal(0, ifaces.QueryID(name), wExpr)

	case *constraint.RangeConstraint[*air.ColumnAccess]:

		bound := cs.Bound
		// #nosec G115 -- this bound will not overflow
		s.Comp.InsertRange(0, ifaces.QueryID(name), s.compColumnByCorsetID(cs.Expr.Column), int(bound.Uint64()))

	default:

		utils.Panic("unexpected constraint type: %T", cs)
	}
}

// castExpression turns a corset expression into a [symbolic.Expression] whose
// variables are [wizard.CompiledIOP] components.
func (s *schemaScanner) castExpression(expr air.Expr) *symbolic.Expression {

	switch e := expr.(type) {

	case *air.Add:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(e.Args[i])
		}
		return symbolic.Add(args...)

	case *air.Sub:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(e.Args[i])
		}
		return symbolic.Sub(args[0], args[1:]...)

	case *air.Mul:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(e.Args[i])
		}
		return symbolic.Mul(args...)

	case *air.Constant:

		return symbolic.NewConstant(e.Value)

	case *air.ColumnAccess:

		c := s.compColumnByCorsetID(e.Column)
		if e.Shift != 0 {
			c = column.Shift(c, e.Shift)
		}
		return symbolic.NewVariable(c)

	default:
		eStr := fmt.Sprintf("%v", e)
		panic(fmt.Sprintf("unsupported type: %T for %v", e, eStr))
	}
}

type corsetNamed interface {
	Context() trace.Context
}

func getModuleName(schema *air.Schema, v corsetNamed) string {
	var (
		moduleID = v.Context().Module()
		module   = schema.Modules().Nth(moduleID)
	)

	return module.Name
}

func getModuleNameFromColumn(schema *air.Schema, col schema.Column) string {
	var (
		moduleID = col.Context.Module()
		module   = schema.Modules().Nth(moduleID)
	)

	return module.Name
}

// wizardName formats a name to be used on the wizard side as an identifier for
// either constraints or columns
func wizardName(moduleName, objectName string) string {
	return moduleName + "." + objectName
}

// compColumnByCorsetID returns an [ifaces.Column] that has already been
// registered inside of the [wizard.CompiledIOP] from its index in the corset
// [air.Schema].
func (s *schemaScanner) compColumnByCorsetID(corsetID uint) ifaces.Column {
	var (
		cCol  = s.Schema.Columns().Nth(corsetID)
		cName = ifaces.ColID(wizardName(getModuleNameFromColumn(s.Schema, cCol), cCol.Name))
		wCol  = s.Comp.Columns.GetHandle(cName)
	)
	return wCol
}

// mapModuleLimits returns a map of the module by limit in lower-case.
func mapModuleLimits(limit *config.TracesLimits) map[string]int {

	var (
		res       = make(map[string]int, 100)
		limitVal  = reflect.ValueOf(limit).Elem() // since we pass a pointer, we dereference it then
		limitType = limitVal.Type()
		numField  = limitType.NumField()
	)

	for i := 0; i < numField; i++ {

		var (
			corsetTag = limitType.Field(i).Tag.Get("corset")
			limit     = limitVal.Field(i).Interface().(int)
		)

		if len(corsetTag) == 0 {
			corsetTag = strings.ToLower(limitType.Field(i).Name)
		}

		res[corsetTag] = limit
	}

	return res
}

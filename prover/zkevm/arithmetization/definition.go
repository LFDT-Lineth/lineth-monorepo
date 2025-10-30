package arithmetization

import (
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/field"

	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/schema"
	"github.com/consensys/go-corset/pkg/util/field/bls12_377"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
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
	Schema             *air.Schema[bls12_377.Element]
	Modules            []schema.Module[bls12_377.Element]
	InterleavedColumns map[string]air.InterleavingConstraint[bls12_377.Element]
}

// Define registers the arithmetization from a corset air.Schema and trace limits
// from config.
func Define(comp *wizard.CompiledIOP, schema *air.Schema[bls12_377.Element], limits *config.TracesLimits) {

	// Collect modules and sort them by name to ensure deterministic processing order
	modules := schema.Modules().Collect()
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name() < modules[j].Name()
	})

	scanner := &schemaScanner{
		LimitMap:           mapModuleLimits(limits),
		Comp:               comp,
		Schema:             schema,
		Modules:            modules,
		InterleavedColumns: map[string]air.InterleavingConstraint[bls12_377.Element]{},
	}

	scanner.scanColumns()
	scanner.scanConstraints()
}

// scanColumns scans the column declaration of the corset [air.Schema] into the
// [wizard.CompiledIOP] object.
func (s *schemaScanner) scanColumns() {
	// Use the pre-sorted modules from the scanner to ensure deterministic ordering
	// Iterate each declared module
	for _, modDecl := range s.Modules {
		// Identify limits for this module
		var (
			moduleLimit = s.LimitMap[modDecl.Name()]
			mult        = modDecl.LengthMultiplier()
			size        = int(mult) * moduleLimit
		)
		// Adjust the size for interleaved columns and their permuted versions.
		// Since these are the only columns from corset with a non-power-of-two size.
		if !utils.IsPowerOfTwo(size) {
			newSize := utils.NextPowerOfTwo(int(mult) * moduleLimit)
			logrus.Debug("Adjusting size for module: ", modDecl.Name(), " from ", size, " to ", newSize)
			size = newSize
		}
		// #nosec G115 -- this bound will not overflow
		if size == 0 && modDecl.Name() != "" {
			logrus.Infof("Module %s has size 0", modDecl.Name())
		}
		// Iterate each register (i.e. column) in that module
		for _, colDecl := range modDecl.Registers() {
			// Construct corresponding register name
			var name = wizardName(modDecl.Name(), colDecl.Name)
			//
			col := s.Comp.InsertCommit(0, ifaces.ColID(name), size)
			pragmas.MarkLeftPadded(col)
		}
	}
}

// scanConstraints scans the constraint declaration from a corset schema into
// the [wizard.CompiledIOP] object.
func (s *schemaScanner) scanConstraints() {

	corsetCSs := s.Schema.Constraints().Collect()

	// Create a stable ordering based on constraint names to ensure deterministic processing
	// We use a slice of indices and sort those instead of the constraints themselves
	// to preserve any internal dependency ordering within constraint types
	indices := make([]int, len(corsetCSs))
	for i := range indices {
		indices[i] = i
	}

	// Sort indices by constraint name for deterministic ordering
	sort.Slice(indices, func(i, j int) bool {
		nameI := fmt.Sprintf("%v", corsetCSs[indices[i]].Lisp(s.Schema).String(false))
		nameJ := fmt.Sprintf("%v", corsetCSs[indices[j]].Lisp(s.Schema).String(false))
		return nameI < nameJ
	})

	// Process constraints in the sorted order
	for _, idx := range indices {
		corsetCS := corsetCSs[idx]
		name := fmt.Sprintf("%v", corsetCS.Lisp(s.Schema).String(false))
		if s.Comp.QueriesNoParams.Exists(ifaces.QueryID(name)) {
			continue
		}
		s.addConstraintInComp(name, corsetCS)
	}
}

// addCsInComp adds a corset constraint into the [wizard.CompiledIOP]
func (s *schemaScanner) addConstraintInComp(name string, corsetCS schema.Constraint[bls12_377.Element]) {

	switch cs := corsetCS.(type) {

	case air.InterleavingConstraint[bls12_377.Element]:
		// Identify all interleaved columns
		var (
			ic = cs.Unwrap()
			// construct reference (which uniquely identifies register / column)
			targetRef = schema.NewRegisterRef(ic.TargetContext, ic.Target.Register)
			// extract register
			col = s.Schema.Register(targetRef)
		)
		// Construct wizard name of target column
		wName := wizardName(s.Schema.Module(ic.TargetContext).Name(), col.Name)
		// Record interleaving constraint
		s.InterleavedColumns[wName] = cs

	case air.LookupConstraint[bls12_377.Element]:
		var (
			cSource  = cs.Unwrap().Sources[0]
			cTarget  = cs.Unwrap().Targets[0]
			numCol   = cSource.Len()
			wSources = make([]ifaces.Column, numCol)
			wTargets = make([]ifaces.Column, numCol)
		)
		// Sanity check for fragment lookup
		if len(cs.Unwrap().Sources) != 1 {
			// Indicates more than one fragment.  For now just fail, as this
			// cannot (yet) arise in practice.
			panic("unreachable")
		}
		// this will panic over interleaved columns, we can debug that later
		for i := range numCol {
			wSources[i] = s.compColumnByCorsetID(cSource.Module, cSource.Terms[i].Register)
			wTargets[i] = s.compColumnByCorsetID(cTarget.Module, cTarget.Terms[i].Register)
		}

		if !cSource.HasSelector() && !cTarget.HasSelector() {
			// Neither source nor target vector has selector
			s.Comp.InsertInclusion(0, ifaces.QueryID(name), wTargets, wSources)
		} else if cSource.HasSelector() && !cTarget.HasSelector() {
			// source vector only has selector
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetID(cSource.Module, selectorSourceRaw.Register)
			s.Comp.InsertInclusionConditionalOnIncluded(0, ifaces.QueryID(name), wTargets, wSources, selectorSource)
		} else if !cSource.HasSelector() && cTarget.HasSelector() {
			// target vector only has selector
			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetID(cTarget.Module, selectorTargetRaw.Register)
			s.Comp.InsertInclusionConditionalOnIncluding(0, ifaces.QueryID(name), wTargets, wSources, selectorTarget)
		} else {
			// both source and target vectors have selectors
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetID(cSource.Module, selectorSourceRaw.Register)

			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetID(cTarget.Module, selectorTargetRaw.Register)

			s.Comp.InsertInclusionDoubleConditional(0, ifaces.QueryID(name), wTargets, wSources, selectorTarget, selectorSource)
		}

	case air.PermutationConstraint[bls12_377.Element]:

		var (
			pc       = cs.Unwrap()
			numCol   = len(pc.Sources)
			cSources = pc.Sources
			cTargets = pc.Targets
			wSources = make([]ifaces.Column, numCol)
			wTargets = make([]ifaces.Column, numCol)
		)

		// this will panic over interleaved columns, we can debug that later
		for i := 0; i < numCol; i++ {
			wSources[i] = s.compColumnByCorsetID(pc.Context, cSources[i])
			wTargets[i] = s.compColumnByCorsetID(pc.Context, cTargets[i])
		}

		s.Comp.InsertPermutation(0, ifaces.QueryID(name), wTargets, wSources)

	case air.VanishingConstraint[bls12_377.Element]:

		var (
			vc    = cs.Unwrap()
			wExpr = s.castExpression(vc.Context, vc.Constraint.Term)
			wMeta = wExpr.BoardListVariableMetadata()
		)

		if len(wMeta) == 0 {
			// Sometime, it just so happens that corset gives constant expressions
			return
		}

		if vc.Domain.IsEmpty() {
			s.Comp.InsertGlobal(0, ifaces.QueryID(name), wExpr)
			return
		}

		domain := vc.Domain.Unwrap()

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

	case air.RangeConstraint[bls12_377.Element]:
		rc := cs.Unwrap()
		bound := field.NewElement(2)
		bound.Exp(bound, big.NewInt(int64(rc.Bitwidth)))
		// #nosec G115 -- this bound will not overflow
		s.Comp.InsertRange(0, ifaces.QueryID(name), s.compColumnByCorsetID(rc.Context, rc.Expr.Register), int(bound.Uint64()))

	case air.Assertion[bls12_377.Element]:
		// Property assertions can be ignored, as they are a debugging tool and
		// not part of the constraints proper.
	default:
		utils.Panic("unexpected constraint type: %s", cs.Lisp(s.Schema).String(false))
	}
}

// castExpression turns a corset expression into a [symbolic.Expression] whose
// variables are [wizard.CompiledIOP] components.
func (s *schemaScanner) castExpression(context schema.ModuleId, expr air.Term[bls12_377.Element]) *symbolic.Expression {

	switch e := expr.(type) {

	case *air.Add[bls12_377.Element]:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return symbolic.Add(args...)

	case *air.Sub[bls12_377.Element]:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return symbolic.Sub(args[0], args[1:]...)

	case *air.Mul[bls12_377.Element]:

		args := make([]any, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return symbolic.Mul(args...)

	case *air.Constant[bls12_377.Element]:

		return symbolic.NewConstant(e.Value.Element)

	case *air.ColumnAccess[bls12_377.Element]:

		c := s.compColumnByCorsetID(context, e.Register)
		if e.Shift != 0 {
			c = column.Shift(c, e.Shift)
		}
		return symbolic.NewVariable(c)

	default:
		eStr := fmt.Sprintf("%v", e)
		panic(fmt.Sprintf("unsupported type: %T for %v", e, eStr))
	}
}

// wizardName formats a name to be used on the wizard side as an identifier for
// either constraints or columns
func wizardName(moduleName, objectName string) string {
	return moduleName + "." + objectName
}

// compColumnByCorsetID returns an [ifaces.Column] that has already been
// registered inside of the [wizard.CompiledIOP] from its index in the corset
// [air.Schema].
func (s *schemaScanner) compColumnByCorsetID(modId schema.ModuleId, regId schema.RegisterId) ifaces.Column {
	var (
		// construct register reference which uniquely identifies the column
		ref = schema.NewRegisterRef(modId, regId)
		// extract register
		cCol = s.Schema.Register(ref)
		// identify module name
		modName = s.Schema.Module(modId).Name()
		// convert name to prover column id
		cName = ifaces.ColID(wizardName(modName, cCol.Name))
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
		// FIXME: following hack for limits of interleaved modules.
		res[fmt.Sprintf("%s×3", corsetTag)] = limit * 3
		res[fmt.Sprintf("%s×4", corsetTag)] = limit * 4
	}

	return res
}

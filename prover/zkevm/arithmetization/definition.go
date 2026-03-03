package arithmetization

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/consensys/linea-monorepo/prover/maths/field"

	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/schema"
	"github.com/consensys/go-corset/pkg/schema/register"
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

// DefaultLimit is the default limit that we use for all modules that don't
// have a stated limit.
const DefaultLimit = 1 << 17

// schemaScanner is a transient scanner structure whose goal is to port the
// content of an [air.Schema] inside of a pre-initialized [wizard.CompiledIOP]
type schemaScanner struct {
	LimitMap           *config.TracesLimits
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
		return modules[i].Name().String() < modules[j].Name().String()
	})

	scanner := &schemaScanner{
		LimitMap:           limits,
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

		// The "root" module is part of the if the list of the modules. It
		// expectedly does not contains any column. We need to skip it because
		// we would not be able to find its name.
		if modDecl.Name().String() == "" {
			if modDecl.Width() != 0 {
				utils.Panic("found a module with no names but with columns")
			}
			continue
		}

		// Identify limits for this module
		var (
			moduleLimit = uint(s.LimitMap.GetLimit(modDecl.Name().Name))
			mult        = modDecl.Name().Multiplier
			size        = int(mult * moduleLimit)
		)
		// Adjust the size for interleaved columns and their permuted versions.
		// Since these are the only columns from corset with a non-power-of-two size.
		if !utils.IsPowerOfTwo(size) {
			newSize := utils.NextPowerOfTwo(size)
			logrus.Debug("Adjusting size for module: ", modDecl.Name(), " from ", size, " to ", newSize)
			size = newSize
		}
		// #nosec G115 -- this bound will not overflow
		if size == 0 && modDecl.Name().String() != "" {
			utils.Panic("Module %s has size 0", modDecl.Name())
		}

		// Iterate each register (i.e. column) in that module
		for _, colDecl := range modDecl.Registers() {
			// Construct corresponding register name
			var name = wizardName(modDecl.Name().String(), colDecl.Name())
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
			targetRef = register.NewRef(ic.TargetContext, ic.Target.Register())
			// extract register
			col = s.Schema.Register(targetRef)
		)
		// Construct wizard name of target column
		wName := wizardName(s.Schema.Module(ic.TargetContext).Name().String(), col.Name())
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
			wSources[i] = s.compColumnByCorsetColumnAccess(cSource.Module, cSource.Terms[i])
			wTargets[i] = s.compColumnByCorsetColumnAccess(cTarget.Module, cTarget.Terms[i])
		}

		if !cSource.HasSelector() && !cTarget.HasSelector() {
			// Neither source nor target vector has selector
			s.Comp.InsertInclusion(0, ifaces.QueryID(name), wTargets, wSources)
		} else if cSource.HasSelector() && !cTarget.HasSelector() {
			// source vector only has selector
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetColumnAccess(cSource.Module, selectorSourceRaw)
			s.Comp.InsertInclusionConditionalOnIncluded(0, ifaces.QueryID(name), wTargets, wSources, selectorSource)
		} else if !cSource.HasSelector() && cTarget.HasSelector() {
			// target vector only has selector
			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetColumnAccess(cTarget.Module, selectorTargetRaw)
			s.Comp.InsertInclusionConditionalOnIncluding(0, ifaces.QueryID(name), wTargets, wSources, selectorTarget)
		} else {
			// both source and target vectors have selectors
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetColumnAccess(cSource.Module, selectorSourceRaw)

			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetColumnAccess(cTarget.Module, selectorTargetRaw)

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
		// Sanity check:  If a RangeConstraint ever has more than one source/bitwidth, the second iteration will panic
		// because the first iteration already registered that QueryID in the CompiledIOP. In practice
		// the len is always expected to be either 0 (no-op) or 1 (single pass).
		if len(rc.Bitwidths) > 1 {
			utils.Panic("multiple bitwidths for range constraints not supported")
		}
		for i, bitwidth := range rc.Bitwidths {
			// Determine bound for this range constraint
			bound := field.NewElement(2)
			bound.Exp(bound, big.NewInt(int64(bitwidth)))
			// #nosec G115 -- this bound will not overflow
			s.Comp.InsertRange(0, ifaces.QueryID(name), s.compColumnByCorsetID(rc.Context, rc.Sources[i].Register()), int(bound.Uint64()))
		}

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
		var val field.Element
		//
		return symbolic.NewConstant(val.SetBytes(e.Value.Bytes()))

	case *air.ColumnAccess[bls12_377.Element]:

		c := s.compColumnByCorsetID(context, e.Register())
		if e.RelativeShift() != 0 {
			c = column.Shift(c, e.RelativeShift())
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
func (s *schemaScanner) compColumnByCorsetColumnAccess(modId schema.ModuleId, colAccess *air.ColumnAccess[bls12_377.Element]) ifaces.Column {
	var (
		regId = colAccess.Register()
		// construct register reference which uniquely identifies the column
		ref = register.NewRef(modId, regId)
		// extract register
		cCol = s.Schema.Register(ref)
		// identify module name
		modName = s.Schema.Module(modId).Name().String()
		// convert name to prover column id
		cName = ifaces.ColID(wizardName(modName, cCol.Name()))
		wCol  = s.Comp.Columns.GetHandle(cName)
	)

	if colAccess.RelativeShift() != 0 {
		wCol = column.Shift(wCol, colAccess.RelativeShift())
	}

	return wCol
}

// compColumnByCorsetID returns an [ifaces.Column] that has already been
// registered inside of the [wizard.CompiledIOP] from its index in the corset
// [air.Schema].
func (s *schemaScanner) compColumnByCorsetID(modId schema.ModuleId, regId register.Id) ifaces.Column {
	var (
		// construct register reference which uniquely identifies the column
		ref = register.NewRef(modId, regId)
		// extract register
		cCol = s.Schema.Register(ref)
		// identify module name
		modName = s.Schema.Module(modId).Name().String()
		// convert name to prover column id
		cName = ifaces.ColID(wizardName(modName, cCol.Name()))
		wCol  = s.Comp.Columns.GetHandle(cName)
	)

	return wCol
}

package zkcdriver

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/zkc/pkg/ir/air"
	"github.com/LFDT-Lineth/zkc/pkg/schema"
	"github.com/LFDT-Lineth/zkc/pkg/schema/register"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
)

const (
	// corsetColumnMap is an annotation to help seeking column from their corset
	// name.
	corsetColumnMapAnnotationKey = "corset-column-map"
)

// schemaScanner is a transient scanner structure whose goal is to port the
// content of an [air.Schema] inside of a pre-initialized [wiop.System]
type schemaScanner struct {
	Sys     *wiop.System
	Schema  *air.Schema[koalabear.Element]
	Modules []schema.Module[koalabear.Element]
	// ModulesIDsWiop maps corset-module names to their wiop module IDs
	ModulesIDsWiop map[string]int
	// ColumnIds maps the concatenation of the module name and the column name to the ObjectID of the corresponding
	// wizard column name.
	ColumnIDs map[string]wiop.ObjectID
}

// Define registers the arithmetization from a corset air.Schema and trace limits
// from config.
func Define(sys *wiop.System, schema *air.Schema[koalabear.Element]) {

	// Collect modules and sort them by name to ensure deterministic processing order
	modules := schema.Modules().Collect()
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name().String() < modules[j].Name().String()
	})

	scanner := &schemaScanner{
		Sys:            sys,
		Schema:         schema,
		Modules:        modules,
		ModulesIDsWiop: map[string]int{},
		ColumnIDs:      map[string]wiop.ObjectID{},
	}

	scanner.scanColumns()
	scanner.scanConstraints()

	sys.Annotations[corsetColumnMapAnnotationKey] = scanner.ColumnIDs
}

// scanColumns scans the column declaration of the corset [air.Schema] into the
// [wiop.System] object. Only columns referenced by at least one constraint are
// registered; dangling columns (present in the schema but absent from every
// constraint) are silently skipped.
func (s *schemaScanner) scanColumns() {

	referenced := s.collectReferencedColumns()

	// Use the pre-sorted modules from the scanner to ensure deterministic ordering
	// Iterate each declared module
	for _, modDecl := range s.Modules {
		moduleName := modDecl.Name().String()

		// Skip modules whose every column is dangling to avoid creating empty
		// wiop modules whose dynamic size would never be set.
		if !s.moduleHasReferencedColumn(modDecl, moduleName, referenced) {
			logrus.Warnf("zkcdriver: scanColumns: skipping module %q (no referenced columns)", moduleName)
			continue
		}

		// Check for special cases
		if modDecl.IsStatic() {

			content := modDecl.StaticContents()
			moduleWIOP := s.Sys.NewSizedModule(
				s.Sys.Context.Childf("module-%v", moduleName),
				len(content),
				wiop.PaddingDirectionLeft,
			)

			// This works assuming the [System] appends-only to the list of modules.
			s.ModulesIDsWiop[moduleName] = len(s.Sys.Modules) - 1

			for i, colDecl := range modDecl.Registers() {
				colQualifiedName := qualifiedCorsetName(moduleName, colDecl.Name())
				if _, ok := referenced[colQualifiedName]; !ok {
					logrus.Warnf("zkcdriver: scanColumns: skipping dangling column %q", colQualifiedName)
					continue
				}

				vec := make([]field.Element, len(content))
				for j := range content {
					vec[j] = field.Element(content[j][i])
				}

				col := moduleWIOP.NewPrecomputedColumn(
					moduleWIOP.Context.Childf("column-%v", colDecl.Name()),
					wiop.VisibilityOracle,
					&wiop.ConcreteVector{Plain: field.VecFromBase(vec)},
				)

				s.ColumnIDs[colQualifiedName] = col.Context.ID
			}

			continue
		}

		if modDecl.IsNative() {
			// @david: need to add support for native modules.  These correspond
			// to ZkC functions declared with the "native" attribute".  The
			// expectation is that the prover will maintain a list of supported
			// native modules.  Each of these will have an expected number of
			// columns (which the prover may wish to check matches the
			// declaration here).  These columns correspond to the input/output
			// registers of the corresponding ZkC function.
			//
			// A key aspect of native modules is that ZkC will not generate any
			// constraints for them.  Instead, the expectation is that whatever
			// constraints are required will be added somehow / somewhere by the
			// prover.  Since forgetting to do this is a critical soundness
			// issue, care must be taken to ensure it really happens (e.g.
			// through testing negative cases which should cause constraint
			// failures).

			if err := s.defineNativeModule(modDecl); err != nil {
				logrus.Panicf("zkcdriver: failed to define native module %s: %v", modDecl.Name(), err)
			}
			continue
		}

		moduleWIOP := s.Sys.NewDynamicModule(
			s.Sys.Context.Childf("module-%v", moduleName),
			wiop.PaddingDirectionLeft)

		// This works assuming the [System] appends-only to the list of modules.
		s.ModulesIDsWiop[moduleName] = len(s.Sys.Modules) - 1

		// Iterate each register (i.e. column) in that module
		for _, colDecl := range modDecl.Registers() {
			colQualifiedName := qualifiedCorsetName(moduleName, colDecl.Name())
			if _, ok := referenced[colQualifiedName]; !ok {
				logrus.Warnf("zkcdriver: scanColumns: skipping dangling column %q", colQualifiedName)
				continue
			}

			col := moduleWIOP.NewColumn(
				moduleWIOP.Context.Childf("column-%v", colDecl.Name()),
				wiop.VisibilityOracle,
				s.Sys.Rounds[0],
			)

			s.ColumnIDs[colQualifiedName] = col.Context.ID
		}
	}
}

// collectReferencedColumns does a read-only pass over every constraint and
// returns the set of qualified column names (module.column) that appear in at
// least one constraint expression. Columns absent from this set are dangling
// and can be safely omitted from the wiop System.
func (s *schemaScanner) collectReferencedColumns() map[string]struct{} {
	referenced := make(map[string]struct{})

	for _, cs := range s.Schema.Constraints().Collect() {
		switch cs := cs.(type) {

		case air.VanishingConstraint[koalabear.Element]:
			vc := cs.Unwrap()
			s.collectFromTerm(vc.Context, vc.Constraint.Term, referenced)

		case air.LookupConstraint[koalabear.Element]:
			lc := cs.Unwrap()
			for _, frag := range lc.Sources {
				for _, term := range frag.Terms {
					s.addColRef(frag.Module, term.Register(), referenced)
				}
				if frag.HasSelector() {
					s.addColRef(frag.Module, frag.Selector.Unwrap().Register(), referenced)
				}
			}
			for _, frag := range lc.Targets {
				for _, term := range frag.Terms {
					s.addColRef(frag.Module, term.Register(), referenced)
				}
				if frag.HasSelector() {
					s.addColRef(frag.Module, frag.Selector.Unwrap().Register(), referenced)
				}
			}

		case air.RangeConstraint[koalabear.Element]:
			rc := cs.Unwrap()
			for i := range rc.Bitwidths {
				s.addColRef(rc.Context, rc.Sources[i].Register(), referenced)
			}

		case air.Assertion[koalabear.Element]:
			// Assertions are debug-only and not part of the proof; ignore.
		}
	}

	return referenced
}

// collectFromTerm recursively walks a constraint term and adds every column
// access it finds to out. It mirrors the structure of castExpression.
func (s *schemaScanner) collectFromTerm(modID schema.ModuleId, term air.Term[koalabear.Element], out map[string]struct{}) {
	switch e := term.(type) {
	case *air.Add[koalabear.Element]:
		for _, arg := range e.Args {
			s.collectFromTerm(modID, arg, out)
		}
	case *air.Sub[koalabear.Element]:
		for _, arg := range e.Args {
			s.collectFromTerm(modID, arg, out)
		}
	case *air.Mul[koalabear.Element]:
		for _, arg := range e.Args {
			s.collectFromTerm(modID, arg, out)
		}
	case *air.Constant[koalabear.Element]:
		// no column reference
	case *air.ColumnAccess[koalabear.Element]:
		s.addColRef(modID, e.Register(), out)
	default:
		utils.Panic("zkcdriver: collectFromTerm: unsupported term type %T", term)
	}
}

// addColRef resolves a (moduleID, registerID) pair to its qualified column name
// and records it in out.
func (s *schemaScanner) addColRef(modID schema.ModuleId, regID register.Id, out map[string]struct{}) {
	ref := register.NewRef(modID, regID)
	cCol := s.Schema.Register(ref)
	moduleName := s.Schema.Module(modID).Name().String()
	out[qualifiedCorsetName(moduleName, cCol.Name())] = struct{}{}
}

// moduleHasReferencedColumn reports whether any column in modDecl appears in
// the referenced set.
func (s *schemaScanner) moduleHasReferencedColumn(modDecl schema.Module[koalabear.Element], moduleName string, referenced map[string]struct{}) bool {
	for _, colDecl := range modDecl.Registers() {
		if _, ok := referenced[qualifiedCorsetName(moduleName, colDecl.Name())]; ok {
			return true
		}
	}
	return false
}

// scanConstraints scans the constraint declaration from a corset schema into
// the [wiop.System] object.
func (s *schemaScanner) scanConstraints() {

	corsetCSS := s.Schema.Constraints().Collect()

	// Create a stable ordering based on constraint names to ensure deterministic processing
	// We use a slice of indices and sort those instead of the constraints themselves
	// to preserve any internal dependency ordering within constraint types
	indices := make([]int, len(corsetCSS))
	for i := range indices {
		indices[i] = i
	}

	// Pre-compute Lisp names to avoid O(n²) serialisation inside the comparator.
	names := make([]string, len(corsetCSS))
	for i, cs := range corsetCSS {
		names[i] = cs.Lisp(s.Schema).String(false)
	}

	sort.Slice(indices, func(i, j int) bool {
		return names[indices[i]] < names[indices[j]]
	})

	// Process constraints in the sorted order
	for _, idx := range indices {
		s.addConstraintInComp(names[idx], corsetCSS[idx])
	}
}

// addCsInComp adds a corset constraint into the [wiop.System]
func (s *schemaScanner) addConstraintInComp(name string, corsetCS schema.Constraint[koalabear.Element]) {

	switch cs := corsetCS.(type) {

	case air.LookupConstraint[koalabear.Element]:

		var (
			cSource                  = cs.Unwrap().Sources[0]
			cTarget                  = cs.Unwrap().Targets[0]
			numCol                   = cSource.Len()
			wSources                 = make([]*wiop.ColumnView, numCol)
			wTargets                 = make([]*wiop.ColumnView, numCol)
			tableSource, tableTarget wiop.Table
		)

		if numCol == 0 {
			// @alex Technically, this should be a panic but for now the
			// arithmetization may give us lookup tables with 0 columns. We
			// just skip those.
			logrus.Warnf("zkcdriver: inclusion constraint %q has zero columns; skipping", name)
			return
		}

		// Sanity check for fragment lookup
		if len(cs.Unwrap().Sources) != 1 {
			utils.Panic("lookup %q has %d source fragments; only single-fragment lookups are supported", name, len(cs.Unwrap().Sources))
		}

		// this will panic over interleaved columns, we can debug that later
		for i := range numCol {
			wSources[i] = s.compColumnByCorsetColumnAccess(cSource.Module, cSource.Terms[i])
			wTargets[i] = s.compColumnByCorsetColumnAccess(cTarget.Module, cTarget.Terms[i])
		}

		if cSource.HasSelector() {
			// source vector only has selector
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetColumnAccess(cSource.Module, selectorSourceRaw)
			tableSource = wiop.NewFilteredTable(selectorSource, wSources...)
		} else {
			tableSource = wiop.NewTable(wSources...)
		}

		if cTarget.HasSelector() {
			// Target vector only has selector
			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetColumnAccess(cTarget.Module, selectorTargetRaw)
			tableTarget = wiop.NewFilteredTable(selectorTarget, wTargets...)
		} else {
			tableTarget = wiop.NewTable(wTargets...)
		}

		_ = s.Sys.NewInclusion(
			s.Sys.Context.Childf("lookup-%v", name),
			[]wiop.Table{tableSource},
			[]wiop.Table{tableTarget},
		)

	case air.VanishingConstraint[koalabear.Element]:

		var (
			vc     = cs.Unwrap()
			wExpr  = s.castExpression(vc.Context, vc.Constraint.Term)
			module = wExpr.Module()
		)

		if module == nil {
			utils.Panic("wiop: VanishingConstraint has no module : %v", name)
		}

		if vc.Domain.IsEmpty() {
			if !wExpr.IsMultiValued() {
				utils.Panic("wiop: VanishingConstraint has no domain and no multi-valued expression : %v", name)
			}
			module.NewVanishing(module.Context.Childf("global-%v", name), wExpr)
			return
		}

		// If the domain is not empty, then the constraint is a local constraint
		// and the domain is the position of the vanishing vector.
		position := vc.Domain.Unwrap()

		wExpr = wiop.EditExpression(wExpr,
			func(e wiop.Expression, children []wiop.Expression) wiop.Expression {
				switch e := e.(type) {
				case *wiop.ColumnView:
					return e.Column.At(position + e.ShiftingOffset)
				default:
					return wiop.DefaultConstruct(e, children)
				}
			})

		module.NewVanishing(module.Context.Childf("local-%v", name), wExpr)

	case air.RangeConstraint[koalabear.Element]:

		rc := cs.Unwrap()

		// Sanity check:  If a RangeConstraint ever has more than one source/bitwidth, the second iteration will panic
		// because the first iteration already registered that QueryID in the CompiledIOP. In practice
		// the len is always expected to be either 0 (no-op) or 1 (single pass).
		if len(rc.Bitwidths) > 1 {
			utils.Panic("multiple bitwidths for range constraints not supported")
		}

		for i, bitwidth := range rc.Bitwidths {
			// Determine bound for this range constraint
			bound := 1 << bitwidth
			col := s.compColumnByCorsetID(rc.Context, rc.Sources[i].Register())
			col.Module.NewRangeCheck(col.Context.Childf("range-%v", name), col, bound)
		}

	case air.Assertion[koalabear.Element]:
		// Property assertions can be ignored, as they are a debugging tool and
		// not part of the constraints proper.

	default:
		utils.Panic("unexpected constraint type: %s", cs.Lisp(s.Schema).String(false))
	}
}

// castExpression turns a corset expression into a [symbolic.Expression] whose
// variables are [wiop.System] components.
func (s *schemaScanner) castExpression(context schema.ModuleId, expr air.Term[koalabear.Element]) wiop.Expression {

	switch e := expr.(type) {

	case *air.Add[koalabear.Element]:

		args := make([]wiop.Expression, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return wiop.Sum(args...)

	case *air.Sub[koalabear.Element]:

		res := s.castExpression(context, e.Args[0])
		for i := 1; i < len(e.Args); i++ {
			res = wiop.Sub(res, s.castExpression(context, e.Args[i]))
		}
		return res

	case *air.Mul[koalabear.Element]:

		args := make([]wiop.Expression, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return wiop.Product(args...)

	case *air.Constant[koalabear.Element]:
		// @alex: this bit is a bit hacky, because corset's koalabear.Element
		// implementation is a copy-paste from gnark and thus have the same
		// layout. Ideally, both dependencies should converge toward using the
		// exact same implementation. This would remove this kind of hacky
		// conversions.
		return wiop.NewConstantField(field.Element(e.Value))

	case *air.ColumnAccess[koalabear.Element]:

		return s.compColumnByCorsetColumnAccess(context, e)

	default:
		eStr := fmt.Sprintf("%v", e)
		panic(fmt.Sprintf("unsupported type: %T for %v", e, eStr))
	}
}

// qualifiedCorsetName formats a name to be used in the scanner side as an identifier for
// either constraints or columns
func qualifiedCorsetName(moduleName, objectName string) string {
	return moduleName + "." + objectName
}

// compColumnByCorsetID returns an [*wiop.Column] that has already been
// registered inside of the [wiop.System] from its index in the corset
// [air.Schema].
func (s *schemaScanner) compColumnByCorsetColumnAccess(
	modID schema.ModuleId,
	colAccess *air.ColumnAccess[koalabear.Element],
) *wiop.ColumnView {

	var (
		regID      = colAccess.Register()
		columnView = s.compColumnByCorsetID(modID, regID).View()
	)

	if colAccess.RelativeShift() != 0 {
		columnView = columnView.Shift(colAccess.RelativeShift())
	}

	return columnView
}

// compColumnByCorsetID returns an [*wiop.Column] that has already been
// registered inside of the [wiop.System] from its index in the corset
// [air.Schema].
func (s *schemaScanner) compColumnByCorsetID(
	modID schema.ModuleId,
	regID register.Id,
) *wiop.Column {

	var (
		// construct register reference which uniquely identifies the column
		ref        = register.NewRef(modID, regID)
		cCol       = s.Schema.Register(ref)
		moduleName = s.Schema.Module(modID).Name().String()
		columnName = qualifiedCorsetName(moduleName, cCol.Name())
	)

	return s.Sys.LookupColumn(s.ColumnIDs[columnName])
}

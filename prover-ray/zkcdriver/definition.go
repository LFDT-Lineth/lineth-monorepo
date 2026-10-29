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
	ctrace "github.com/LFDT-Lineth/zkc/pkg/zkc/constraints/trace"
)

const (
	// corsetColumnMap is an annotation to help seeking column from their corset
	// name.
	corsetColumnMapAnnotationKey = "corset-column-map"
	// publicOutputsAnnotationKey is an annotation holding the arithmetization's
	// [PublicOutput].
	publicOutputsAnnotationKey = "corset-public-outputs"
	// guestOutputModule is the one public output the prover binds: the memory
	// holding the guest program's output, one byte per address.
	guestOutputModule = "guest_output"
	// programModule is the module committed in [ProgramRound]: the memory
	// holding the pre-decoded instruction table of the guest program.
	programModule = "decoded"
)

// ProgramRound is the round holding the columns of [programModule], which
// depend only on the guest program. The rest of the trace is in
// [Settings.TraceRound].
const ProgramRound = 0

// roundOf returns the round in which the columns of the given corset module
// are committed.
func (s *schemaScanner) roundOf(moduleName string) int {
	if moduleName == programModule {
		return ProgramRound
	}
	return s.TraceRound
}

// PublicOutput locates the columns of the arithmetization's public output memory
// — the memory named [guestOutputModule], declared with `pub output` and
// flagged by the schema via IsPublicOutput see [schemaScanner.collectPublicOutputs].
type PublicOutput struct {
	// Name is the corset name of the memory. It is empty when the
	// arithmetization exposes no public output of the described shape.
	Name string
	// Address holds the limbs of the memory's address, most significant first:
	// an address wider than the field's register width is split by the schema.
	// The memory's own constraints make it vanish on padding rows, be zero on the
	// first row carrying an element and increment from there, so the elements
	// are laid out in address order on the last rows of the module.
	Address []wiop.ObjectID
	// Data is the memory's data column, holding one byte per address.
	Data wiop.ObjectID
	// AccessBit is the memory's access bit: zero on padding rows and one on the
	// rows carrying an element.
	AccessBit wiop.ObjectID
}

// PublicOutputs returns the public output memory [Define] found in the
// arithmetization. Its Name is empty when there is none, or when the one found
// had a shape [schemaScanner.collectPublicOutputs] cannot describe.
func PublicOutputs(sys *wiop.System) PublicOutput {
	outputs, _ := sys.Annotations[publicOutputsAnnotationKey].(PublicOutput)
	return outputs
}

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
	// TraceRound is the round of every column outside of [programModule].
	TraceRound int
	// ExprCache interns the expression nodes built by [castExpression], per
	// module, so that structurally identical subexpressions share one pointer
	// instead of being re-allocated at every occurrence. See [intern].
	//
	// This is a fallback for zkc not deduplicating its own [air.Term] DAG; it
	// is the designated removal point once zkc interns natively (see
	// LFDT-Lineth/lineth-monorepo#4038). The map is used for lookup only and
	// is never iterated, so it does not affect output ordering.
	//
	// Removing this structural fallback alone does not complete the native-zkc
	// migration. ExprCache dedupes the *output*, but every traversal of the
	// *input* still descends into each occurrence of a term, so a shared
	// [air.Term] DAG would be expanded as a tree, which is exponential in DAG
	// depth (a chain of 19 nested Add(t, t) costs ~786K allocations per
	// conversion). Before relying on native sharing:
	//   - [castExpression] must keep a per-module map[air.Term]wiop.Expression,
	//     keyed on the original n-ary term and checked before descending;
	//   - [collectFromTerm] must memoise visited terms in the same way;
	//   - the Lisp rendering used to order constraints in [scanConstraints]
	//     must not expand shared subterms (or must use another ordering key).
	ExprCache map[schema.ModuleId]map[exprKey]wiop.Expression
}

// Define registers the arithmetization from a corset air.Schema and trace limits
// from config. The columns of [programModule] are put in [ProgramRound] and
// all the others in traceRound, where the sizes of all dynamic modules are also
// fed into the Fiat-Shamir transcript.
func Define(sys *wiop.System, schema *air.Schema[koalabear.Element], traceRound int) {

	for len(sys.Rounds) <= traceRound {
		sys.NewRound()
	}
	sys.DynamicSizeRound = traceRound

	// Collect modules and sort them by name to ensure deterministic processing order
	modules := schema.Modules().Collect()
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name() < modules[j].Name()
	})

	scanner := &schemaScanner{
		Sys:            sys,
		Schema:         schema,
		Modules:        modules,
		ModulesIDsWiop: map[string]int{},
		ColumnIDs:      map[string]wiop.ObjectID{},
		TraceRound:     traceRound,
		ExprCache:      newExprCache(),
	}

	scanner.scanColumns()
	scanner.scanConstraints()

	if traceRound != ProgramRound {
		if _, ok := scanner.ModulesIDsWiop[programModule]; !ok {
			logrus.Warnf("zkcdriver: Define: no module %q found, round %d is empty", programModule, ProgramRound)
		}

		// Registered before any compiler pass, so that it runs first in the round.
		sys.Rounds[traceRound].RegisterAction(&assignTraceRoundAction{Schema: *schema, Round: traceRound})
	}

	sys.Annotations[corsetColumnMapAnnotationKey] = scanner.ColumnIDs
	sys.Annotations[publicOutputsAnnotationKey] = scanner.collectPublicOutputs()
}

// collectPublicOutputs resolves the address and data columns of the schema's
// public output memory. It runs after scanColumns so that it can map the declared
// registers onto the wiop columns that were actually created; a memory whose
// columns were all dropped as unreferenced is skipped, as there is nothing left to
// point at.
func (s *schemaScanner) collectPublicOutputs() PublicOutput {

	var output PublicOutput

	for _, modDecl := range s.Modules {

		if !modDecl.IsPublicOutput() {
			continue
		}

		moduleName := modDecl.Name()

		// Only one public output is supported, so a slot already claimed by an
		// earlier module is refused rather than silently resolved.
		if output.Name != "" {
			utils.Panic(
				"zkcdriver: collectPublicOutputs: expected a single public output, found %q and %q",
				output.Name, moduleName,
			)
		}

		var (
			address, data []wiop.ObjectID
			accessBit     wiop.ObjectID
			hasAccessBit  bool
		)

		for _, reg := range modDecl.Registers() {

			id, ok := s.ColumnIDs[qualifiedCorsetName(moduleName, reg.Name())]
			if !ok {
				continue
			}

			// A memory's address lines are exactly its input registers and its data
			// lines exactly its output registers, so the two are told apart by
			// register kind rather than by name. Of the computed registers, only the
			// access bit is of use here; the address selectors are not.
			//
			// Only [guestOutputModule] is collected. Any other public output leaves
			// the lists empty and is skipped just below, so no memory becomes a
			// public input merely by being declared `pub output`.
			switch {
			case moduleName != guestOutputModule:
			case reg.IsInput():
				address = append(address, id)
			case reg.IsOutput():
				data = append(data, id)
			case reg.Name() == ctrace.ACCESS_BIT_NAME:
				accessBit, hasAccessBit = id, true
			}
		}

		// A memory holding anything other than a single byte column, or lacking
		// an address or an access bit, is not describable by [PublicOutput], so it
		// is reported and skipped rather than half-described.
		if len(address) == 0 || len(data) != 1 || !hasAccessBit {
			logrus.Warnf(
				"zkcdriver: collectPublicOutputs: skipping public output %q: "+
					"has %d address and %d data columns (access bit found: %v), "+
					"expected at least one address, one datum and an access bit",
				moduleName, len(address), len(data), hasAccessBit,
			)
			continue
		}

		output = PublicOutput{Name: moduleName, Address: address, Data: data[0], AccessBit: accessBit}
	}

	return output
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
		moduleName := modDecl.Name()

		// Skip non-native modules whose every column is dangling to avoid creating empty
		// wiop modules whose dynamic size would never be set.
		if !modDecl.IsNative() && !s.moduleHasReferencedColumn(modDecl, moduleName, referenced) {
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
				s.Sys.Rounds[s.roundOf(moduleName)],
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

		case *air.VanishingConstraint[koalabear.Element]:
			s.collectFromTerm(cs.Context, cs.Constraint.Term, referenced)

		case *air.LookupConstraint[koalabear.Element]:
			for _, frag := range cs.Sources {
				for _, reg := range frag.Registers {
					s.addColRef(frag.Module, reg, referenced)
				}
				if frag.HasSelector() {
					s.addColRef(frag.Module, frag.Selector.Unwrap(), referenced)
				}
			}
			for _, frag := range cs.Targets {
				for _, reg := range frag.Registers {
					s.addColRef(frag.Module, reg, referenced)
				}
				if frag.HasSelector() {
					s.addColRef(frag.Module, frag.Selector.Unwrap(), referenced)
				}
			}
		}
	}

	return referenced
}

// collectFromTerm recursively walks a constraint term and adds every column
// access it finds to out. It mirrors the structure of castExpression.
//
// It does not memoise visited terms, so a shared [air.Term] is re-walked at
// every occurrence. That is fine for the tree-shaped terms zkc emits today;
// see [schemaScanner.ExprCache] for what native zkc interning requires.
func (s *schemaScanner) collectFromTerm(
	modID schema.ModuleId,
	term air.Term[koalabear.Element],
	out map[string]struct{}) {

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
	moduleName := s.Schema.Module(modID).Name()
	out[qualifiedCorsetName(moduleName, cCol.Name())] = struct{}{}
}

// moduleHasReferencedColumn reports whether any column in modDecl appears in
// the referenced set.
func (s *schemaScanner) moduleHasReferencedColumn(
	modDecl schema.Module[koalabear.Element],
	moduleName string,
	referenced map[string]struct{},
) bool {

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
	// The rendering expands shared subterms, so it would blow up on a native
	// zkc DAG; see [schemaScanner.ExprCache].
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

	case *air.BusConstraint[koalabear.Element]:
		// Iterate receive ports, each of which identifies a set of columns in a
		// given module (including a selector) which determine the message being
		// received.
		for _, recvPort := range cs.Receives {

			cols := make([]*wiop.ColumnView, len(recvPort.Registers))
			for i := range cols {
				cols[i] = s.compColumnByCorsetID(recvPort.Module, recvPort.Registers[i]).View()
			}
			table := wiop.NewFilteredTable(s.compColumnByCorsetID(recvPort.Module, recvPort.Selector).View(), cols...)

			s.Sys.NewMessageBusReceive(s.Sys.Context.Childf("bus-%v", name), "0", name, table)
		}

		// Iterate send ports, each of which identifies a set of columns in a
		// given module (including a selector) which determine the message being
		// sent.
		for _, sendPort := range cs.Sends {
			cols := make([]*wiop.ColumnView, len(sendPort.Registers))
			for i := range cols {
				cols[i] = s.compColumnByCorsetID(sendPort.Module, sendPort.Registers[i]).View()
			}
			table := wiop.NewFilteredTable(s.compColumnByCorsetID(sendPort.Module, sendPort.Selector).View(), cols...)

			s.Sys.NewMessageBusSend(s.Sys.Context.Childf("bus-%v", name), "0", name, table)
		}

	case *air.LookupConstraint[koalabear.Element]:

		var (
			cSource                  = cs.Sources[0]
			cTarget                  = cs.Targets[0]
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
		if len(cs.Sources) != 1 {
			utils.Panic("lookup %q has %d source fragments; only single-fragment lookups are supported", name, len(cs.Sources))
		}

		// this will panic over interleaved columns, we can debug that later
		for i := range numCol {
			wSources[i] = s.compColumnByCorsetID(cSource.Module, cSource.Registers[i]).View()
			wTargets[i] = s.compColumnByCorsetID(cTarget.Module, cTarget.Registers[i]).View()
		}

		if cSource.HasSelector() {
			// source vector only has selector
			selectorSourceRaw := cSource.Selector.Unwrap()
			selectorSource := s.compColumnByCorsetID(cSource.Module, selectorSourceRaw).View()
			tableSource = wiop.NewFilteredTable(selectorSource, wSources...)
		} else {
			tableSource = wiop.NewTable(wSources...)
		}

		if cTarget.HasSelector() {
			// Target vector only has selector
			selectorTargetRaw := cTarget.Selector.Unwrap()
			selectorTarget := s.compColumnByCorsetID(cTarget.Module, selectorTargetRaw).View()
			tableTarget = wiop.NewFilteredTable(selectorTarget, wTargets...)
		} else {
			tableTarget = wiop.NewTable(wTargets...)
		}

		_ = s.Sys.NewInclusion(
			s.Sys.Context.Childf("lookup-%v", name),
			[]wiop.Table{tableSource},
			[]wiop.Table{tableTarget},
		)

	case *air.VanishingConstraint[koalabear.Element]:

		var (
			wExpr  = s.castExpression(cs.Context, cs.Constraint.Term)
			module = wExpr.Module()
		)

		if module == nil {
			utils.Panic("wiop: VanishingConstraint has no module : %v", name)
		}

		if cs.Domain.IsEmpty() {
			if !wExpr.IsMultiValued() {
				utils.Panic("wiop: VanishingConstraint has no domain and no multi-valued expression : %v", name)
			}
			module.NewVanishing(module.Context.Childf("global-%v", name), wExpr)
			return
		}

		// If the domain is not empty, then the constraint is a local constraint
		// and the domain is the position of the vanishing vector.
		position := cs.Domain.Unwrap()

		// The rewrite allocates fresh nodes throughout — [wiop.Column.At] for
		// every leaf, [wiop.DefaultConstruct] for every compound — so the
		// result is re-interned to let local constraints pinned at the same
		// position share their lifted subexpressions. [wiop.EditExpression]
		// visits bottom-up, which is the order [schemaScanner.intern] requires.
		wExpr = wiop.EditExpression(wExpr,
			func(e wiop.Expression, children []wiop.Expression) wiop.Expression {
				switch e := e.(type) {
				case *wiop.ColumnView:
					return s.intern(cs.Context, e.Column.At(position+e.ShiftingOffset))
				default:
					return s.intern(cs.Context, wiop.DefaultConstruct(e, children))
				}
			})

		module.NewVanishing(module.Context.Childf("local-%v", name), wExpr)

	default:
		utils.Panic("unexpected constraint type: %s", cs.Lisp(s.Schema).String(false))
	}
}

// castExpression turns a corset expression into a [symbolic.Expression] whose
// variables are [wiop.System] components.
//
// Every node it returns is interned per module (see [schemaScanner.intern]):
// zkc hands out a tree in which structurally identical subexpressions are
// distinct [air.Term] values, and allocating a fresh [wiop.Expression] for
// each occurrence would starve every pointer-keyed cache downstream. Leaves
// are interned too, and necessarily so: [wiop.Column.View] and
// [wiop.ColumnView.Shift] allocate a new pointer on every call, so without
// leaf interning no compound key could ever match.
//
// Interning dedupes the output only: the recursion still descends into every
// occurrence of an input term before the lookup. This is linear for today's
// tree-shaped input but exponential in depth for a shared [air.Term] DAG, so
// native zkc interning needs an input-term memo here (see
// [schemaScanner.ExprCache]).
func (s *schemaScanner) castExpression(context schema.ModuleId, expr air.Term[koalabear.Element]) wiop.Expression {

	switch e := expr.(type) {

	case *air.Add[koalabear.Element]:

		args := make([]wiop.Expression, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return s.internedSum(context, args...)

	case *air.Sub[koalabear.Element]:

		res := s.castExpression(context, e.Args[0])
		for i := 1; i < len(e.Args); i++ {
			res = s.intern(context, wiop.Sub(res, s.castExpression(context, e.Args[i])))
		}
		return res

	case *air.Mul[koalabear.Element]:

		args := make([]wiop.Expression, len(e.Args))
		for i := range args {
			args[i] = s.castExpression(context, e.Args[i])
		}
		return s.internedProduct(context, args...)

	case *air.Constant[koalabear.Element]:
		// @alex: this bit is a bit hacky, because corset's koalabear.Element
		// implementation is a copy-paste from gnark and thus have the same
		// layout. Ideally, both dependencies should converge toward using the
		// exact same implementation. This would remove this kind of hacky
		// conversions.
		return s.intern(context, wiop.NewConstantField(field.Element(e.Value)))

	case *air.ColumnAccess[koalabear.Element]:

		return s.intern(context, s.compColumnByCorsetColumnAccess(context, e))

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
		moduleName = s.Schema.Module(modID).Name()
		columnName = qualifiedCorsetName(moduleName, cCol.Name())
	)

	return s.Sys.LookupColumn(s.ColumnIDs[columnName])
}

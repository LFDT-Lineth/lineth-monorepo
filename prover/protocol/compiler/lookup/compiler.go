package lookup

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// table is an alias for a list of column. We use it in the scope of the lookup
// compiler as a shorthand to make the code more eye-parseable.
type table = []ifaces.Column

// CompileLogDerivative scans `comp“, looking for Inclusion queries and compile
// them using the LogDerivativeLookup technique. The compiler attempt to group
// queries relating to the same table. This allows saving in commitment because
// when grouping is possible, then we only need to commit to a single
// extract the table and the checked from the lookup query and ensures that the
// table are in canonical order. That is because we want to group lookups into
// the same columns but not in the same order.
func CompileLogDerivative(comp *wizard.CompiledIOP) {

	var (
		mainLookupCtx = captureLookupTables(comp)
		lastRound     = comp.NumRounds() - 1
		proverActions = make([]proverTaskAtRound, comp.NumRounds()+1)
		// zCatalog stores a mapping (round, size) into ZCtx and helps finding
		// which Z context should be used to handle a part of a given permutation
		// query.
		zCatalog = map[[2]int]*zCtx{}
		zEntries = [][2]int{}
		// verifier actions
		va = &finalEvaluationCheck{}
	)

	// Skip the compilation phase if no lookup constraint is being used. Otherwise
	// it will register a verifier action that is not required and will be bugged.
	if len(mainLookupCtx.lookupTables) == 0 {
		return
	}

	// Step 1. construct the "per table" contexts and pack the Sigma's into
	// zCatalog.
	for _, lookupTable := range mainLookupCtx.lookupTables {

		var (
			// get checkedTables, rounds, Filters by lookupTableName
			lookupTableName = nameTable(lookupTable)
			checkTable      = mainLookupCtx.checkedTables[lookupTableName]
			round           = mainLookupCtx.rounds[lookupTableName]
			includedFilters = mainLookupCtx.includedFilters[lookupTableName]
			tableCtx        = compileLookupTable(comp, round, lookupTable, checkTable, includedFilters)
		)

		// push to zCatalog
		tableCtx.pushToZCatalog(zCatalog)

		proverActions[round].pushMAssignment(
			mAssignmentTask{
				M:       tableCtx.M,
				S:       checkTable,
				T:       lookupTable,
				SFilter: includedFilters,
			},
		)
	}

	// This loops is necessary to build a sorted list of the entries of zCatalog.
	// Without it, if we tried to loop over zCatalog directly, the entries would
	// be processed in a non-deterministic order. The sorting order itself is
	// without importance, what matters is that zEntries is in deterministic
	// order.
	for entry := range zCatalog {
		zEntries = append(zEntries, entry)
	}

	slices.SortFunc(zEntries, func(a, b [2]int) int {
		switch {
		case a[0] < b[0]:
			return -1
		case a[0] > b[0]:
			return 1
		case a[1] < b[1]:
			return -1
		case a[1] > b[1]:
			return 1
		default:
			return 0
		}
	})

	// compile zCatalog
	for _, entry := range zEntries {
		zC := zCatalog[entry]
		// z-packing compile
		zC.compile(comp)
		// entry[0]:round, entry[1]: size
		// the round that Gamma was registered.
		round := entry[0]
		proverActions[round].pushZAssignment(zAssignmentTask(*zC))
		va.ZOpenings = append(va.ZOpenings, zC.ZOpenings...)
		va.Name = zC.Name
	}

	for round := range proverActions {
		// It would not be a bugged to include a proverAction that does nothing
		// but this pollutes the performance analysis of the prover and logs.
		if proverActions[round].numTasks() > 0 {
			comp.RegisterProverAction(round, proverActions[round])
		}
	}

	comp.RegisterVerifierAction(lastRound, va)
}

// captureLookupTables inspects comp and look for Inclusion queries that are not
// marked as ignored yet. All the queries matched queries are grouped by look-up
// table (e.g. all the queries that use the same lookup table). All the matched
// queries are marked as ignored. The function returns the thereby-initialized
// [mainLookupCtx].
//
// This step does not directly mutate the wizard (apart from marking the queries
// as ignored) and it prepares the next table-by-table compilation step.
//
// The function also implictly reduces the conditionals over the Including table
// be appending a "one" column on the included side and the filter on the
// including side.
func captureLookupTables(comp *wizard.CompiledIOP) mainLookupCtx {

	ctx := mainLookupCtx{
		lookupTables:    [][]table{},
		checkedTables:   map[string][]table{},
		includedFilters: map[string][]ifaces.Column{},
		rounds:          map[string]int{},
	}

	// Collect all the lookup queries into "lookups"
	for _, qName := range comp.QueriesNoParams.AllUnignoredKeys() {

		// Filter out non lookup queries
		lookup, ok := comp.QueriesNoParams.Data(qName).(query.Inclusion)
		if !ok {
			continue
		}

		// This ensures that the lookup query is not used again in the
		// compilation process. We know that the query was already ignored at
		// the beginning because we are iterating over the unignored keys.
		comp.QueriesNoParams.MarkAsIgnored(qName)

		var (
			// checkedTable corresponds to the "included" table and lookupTable
			// corresponds to the including table.
			checkedTable, lookupTable = getTableCanonicalOrder(lookup)
			tableName                 = nameTable(lookupTable)
			// includedFilters stores the query.IncludedFilter parameter. If the
			// query has no includedFilters on the Included side. Then this is
			// left as nil.
			includedFilter ifaces.Column
		)

		if lookup.IsFilteredOnIncluding() {
			var (
				checkedLen = checkedTable[0].Size()
				ones       = verifiercol.NewConstantCol(field.One(), checkedLen)
			)

			checkedTable = append([]ifaces.Column{ones}, checkedTable...)
			for frag := range lookupTable {
				lookupTable[frag] = append([]ifaces.Column{lookup.IncludingFilter[frag]}, lookupTable[frag]...)
			}

			tableName = nameTable(lookupTable)
		}

		if lookup.IsFilteredOnIncluded() {
			includedFilter = lookup.IncludedFilter
		}

		// In case this is the first iteration where we encounter the lookupTable
		// we need to add entries in the registering maps.
		if _, ok := ctx.checkedTables[tableName]; !ok {
			ctx.includedFilters[tableName] = []ifaces.Column{}
			ctx.checkedTables[tableName] = []table{}
			ctx.lookupTables = append(ctx.lookupTables, lookupTable)
			ctx.rounds[tableName] = 0
		}

		ctx.includedFilters[tableName] = append(ctx.includedFilters[tableName], includedFilter)
		ctx.checkedTables[tableName] = append(ctx.checkedTables[tableName], checkedTable)
		ctx.rounds[tableName] = max(ctx.rounds[tableName], comp.QueriesNoParams.Round(lookup.ID))

	}

	return ctx
}

// compileLookupTable applies the log-derivative lookup compilation context to
// the supplied table. round denotes the interaction round in which to start the
// compilation.
//
// It registers the following queries
//   - (1) The verifier queries that $\sum_{k=0\ldots n-1} (\Sigma_{S,k})[|S_k| - 1] == (\Sigma_T)[|T| - 1]$. Namely, the sum of the last entry of all $\Sigma_{S,k}$ equals the last entry of $\Sigma_T$
//   - (2) **(For all k)** the verifier makes a `Local` query : $(\Sigma_{S,k})[0] = \frac{1}{S_{k,0} + \gamma}$
//   - (3) The verifier makes a `Local` query : $(\Sigma_T)[0] = \frac{M_0}{T_0 + \gamma}$
//   - (4) **(For all k)** The verifier makes a `Global` query : $\left((\Sigma_{S,k})[i] - (\Sigma_{S,k})[i-1]\right)(S_{k,i} + \gamma) = 1$
//   - (5) The verier makes a `Global` query : $\left((\Sigma_T)[i] - (\Sigma_T)[i-1]\right)(T_i + \gamma) = M_i$

// here we are looking up set of columns S in a single column T
func compileLookupTable(
	comp *wizard.CompiledIOP,
	round int,
	lookupTable []table,
	checkedTables []table,
	includedFilters []ifaces.Column,
) (ctx singleTableCtx) {

	ctx = singleTableCtx{
		TableName: nameTable(lookupTable),
		S:         make([]*symbolic.Expression, len(checkedTables)),
		SFilters:  includedFilters,
		T:         make([]*symbolic.Expression, len(lookupTable)),
		M:         make([]ifaces.Column, len(lookupTable)),
	}

	var (
		// isMultiColumn indicates whether the lookup table (and thus the
		// checked tables) have the same number of
		isMultiColumn = len(lookupTable[0]) > 1
	)

	if !isMultiColumn {
		for frag := range ctx.T {
			ctx.T[frag] = symbolic.NewVariable(lookupTable[frag][0])
			ctx.M[frag] = comp.InsertCommit(
				round,
				deriveTableNameWithIndex[ifaces.ColID](logDerivativePrefix, lookupTable, frag, "M"),
				lookupTable[frag][0].Size(),
			)

		}

		for i := range ctx.S {
			ctx.S[i] = symbolic.NewVariable(checkedTables[i][0])
		}
	}

	if isMultiColumn {

		// alpha is the coin used to compute the linear combination of the
		// columns of T and S when they are (both) multi-columns.
		alpha := comp.InsertCoin(
			round+1,
			deriveTableName[coin.Name](logDerivativePrefix, lookupTable, "ALPHA"),
			coin.Field,
		)

		for frag := range ctx.T {
			ctx.T[frag] = wizardutils.RandLinCombColSymbolic(alpha, lookupTable[frag])
			ctx.M[frag] = comp.InsertCommit(
				round,
				deriveTableNameWithIndex[ifaces.ColID](logDerivativePrefix, lookupTable, frag, "M"),
				lookupTable[frag][0].Size(),
			)
		}

		for i := range ctx.S {
			ctx.S[i] = wizardutils.RandLinCombColSymbolic(alpha, checkedTables[i])
		}
	}

	ctx.Gamma = comp.InsertCoin(
		round+1,
		deriveTableName[coin.Name](logDerivativePrefix, lookupTable, "GAMMA"),
		coin.Field,
	)

	return ctx
}

// pushToZCatalog constructs the numerators and denominators for S and T of the
// stc into zCatalog for their corresponding rounds and size.
func (stc *singleTableCtx) pushToZCatalog(zCatalog map[[2]int]*zCtx) {

	var (
		round = stc.Gamma.Round
	)

	// tableCtx push to -> zCtx
	// Process the T columns
	for frag := range stc.T {
		size := stc.M[frag].Size()

		key := [2]int{round, size}
		if zCatalog[key] == nil {
			zCatalog[key] = &zCtx{
				Size:  size,
				Round: round,
				Name:  stc.TableName,
			}
		}

		zCtxEntry := zCatalog[key]
		zCtxEntry.SigmaNumerator = append(zCtxEntry.SigmaNumerator, symbolic.Neg(stc.M[frag])) // no functions for num, denom here
		zCtxEntry.SigmaDenominator = append(zCtxEntry.SigmaDenominator, symbolic.Add(stc.Gamma, stc.T[frag]))
	}

	// Process the S columns
	for table := range stc.S {
		var (
			_, _, size = wizardutils.AsExpr(stc.S[table])
			sFilter    = symbolic.NewConstant(1)
		)

		if stc.SFilters[table] != nil {
			sFilter = symbolic.NewVariable(stc.SFilters[table])
		}

		key := [2]int{round, size}
		if zCatalog[key] == nil {
			zCatalog[key] = &zCtx{
				Size:  size,
				Round: round,
				Name:  stc.TableName,
			}
		}

		zCtxEntry := zCatalog[key]
		zCtxEntry.SigmaNumerator = append(zCtxEntry.SigmaNumerator, sFilter)
		zCtxEntry.SigmaDenominator = append(zCtxEntry.SigmaDenominator, symbolic.Add(stc.Gamma, stc.S[table]))
	}
}

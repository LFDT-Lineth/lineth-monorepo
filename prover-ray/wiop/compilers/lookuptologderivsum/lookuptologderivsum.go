// Package lookuptologderivsum compiles every unreduced [wiop.TableRelationQuery]
// into a single [wiop.LogDerivativeSum] query whose final result is asserted
// to be zero. It is the prover-ray analogue of linea/prover/protocol/compiler/
// logderivativesum's LookupIntoLogDerivativeSum pass.
//
// The reduction follows the standard log-derivative argument:
//
//   - For each lookup-table fragment T (the "including" side B), commit a
//     multiplicity column M whose value at row i counts how many times that
//     row's value appears across the union of all checked-table A's that
//     reference T.
//
//   - Sample two random extension-field coins:
//
//   - α — used only when the lookup is multi-column, to fold every row
//     into a single field element via random linear combination.
//
//   - γ — used to randomise the denominators (γ + RLC(row)).
//
//   - Emit fractions
//
//     Σ_T  ( −M(row) ) / ( γ + RLC(T(row)) )       (one per T fragment)
//     Σ_S  ( filter_S(row) ) / ( γ + RLC(S(row)) ) (one per A fragment)
//
//     into a single [wiop.LogDerivativeSum] query. The α-randomised RLC
//     binds the multi-column case via Schwartz–Zippel; the γ-randomised
//     denominator makes every Den non-zero with overwhelming probability,
//     which is what closes the zero-denominator soundness gap that the
//     LogDerivativeSum constraint system inherits.
//
//   - Register a verifier action that asserts the LogDerivativeSum's
//     Result cell is zero (the standard log-derivative identity).
//
// B-side filters (selectors on the including side) are folded into the RLC
// itself by prepending the filter to the B-side and a constant 1 to the
// A-side, mirroring linea/lookuptologderivsum's IsFilteredOnIncluding
// handling.
//
// After Compile runs, every consumed [wiop.TableRelationQuery] is marked reduced
// and a single [wiop.LogDerivativeSum] query is left in sys for the
// downstream [logderivativesum] compiler pass to consume.
//
// Scope: the compiler handles inclusion queries with any number of B
// fragments (a lookup table that is the union of fragments). Each B fragment
// gets its own multiplicity column M and its own −M/(γ+RLC) term; the union
// semantics are handled at M-assignment time by a fragment-tagged hash join
// (see [mAssignmentTask]). Multiple A fragments per query and multiple queries
// per shared B side are also supported: queries sharing a B side are bin-packed
// into independent subgroups (each with its own M columns) so that no
// subgroup's summed row count reaches the [wiop.MaxLookupRows] budget, which is
// what keeps large shared tables (e.g. the byte-range tables) provable. See
// [collectGroups]. Permutation queries are ignored.
package lookuptologderivsum

import (
	"fmt"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Compile reduces every unreduced inclusion [wiop.TableRelationQuery] in sys to a
// single [wiop.LogDerivativeSum] query plus a multiplicity column per
// lookup-table fragment, plus a verifier action that asserts the resulting
// LogDerivativeSum result equals zero.
//
// All consumed inclusion queries are marked reduced. Permutation queries and
// already-reduced queries are skipped. If sys contains no eligible queries
// the function is a no-op.
//
// Panics if any inclusion query has an empty B side.
func Compile(sys *wiop.System) {
	groups := collectGroups(sys)
	if len(groups) == 0 {
		return
	}

	// collectGroups returns the subgroups in a deterministic order (first
	// appearance of each B key, then subgroup index), so the LogDerivativeSum is
	// assembled in a map-iteration-independent order without any extra sort.

	// Determine the latest witness round across every group: this dictates
	// where the coin and result rounds live. Groups whose only contributing
	// columns are precomputed leave witnessRound nil (see
	// [lookupGroup.updateWitnessRound]); they are skipped here and patched up
	// after the loop so the compiler still emits its M / α / γ on an
	// interactive round.
	var latestWitness *wiop.Round
	for _, g := range groups {
		if g.witnessRound == nil {
			continue
		}
		if latestWitness == nil || g.witnessRound.ID > latestWitness.ID {
			latestWitness = g.witnessRound
		}
	}
	if latestWitness == nil {
		// Every group's columns were precomputed. Default to the first
		// interactive round so M (committed in each group's witness round)
		// still lives outside the PrecomputedRound.
		if len(sys.Rounds) == 0 {
			panic("wiop/compilers/lookuptologderivsum: cannot compile a fully-precomputed inclusion " +
				"against a system with no interactive rounds; call sys.NewRound() first")
		}
		latestWitness = sys.Rounds[0]
	}
	// Backfill any group whose witnessRound is still nil so compileGroup can
	// rely on a non-nil interactive round.
	for _, g := range groups {
		if g.witnessRound == nil {
			g.witnessRound = latestWitness
		}
	}

	// We need two more interactive rounds after latestWitness:
	//   - latestWitness + 1: where α and γ are sampled. M is *not* placed here:
	//     it must be committed before any coin it interacts with is drawn,
	//     otherwise a malicious prover could pick M as a function of γ and
	//     break log-derivative soundness. M therefore lives in each group's
	//     own witness round (see compileGroup), matching the layout of
	//     linea/prover/protocol/compiler/logderivativesum's lookup pass.
	//   - latestWitness + 2: where the LogDerivativeSum result cell lives.
	coinRound := ensureNextRound(sys, latestWitness)
	ensureNextRound(sys, coinRound) // result round; the LogDerivativeSum constructor finds it on its own.

	compCtx := sys.Context.Childf("lookuptologderiv")

	// γ is shared across every group: a single random extension coin is
	// enough to randomise every denominator in the aggregated query.
	gamma := coinRound.NewCoinField(compCtx.Childf("gamma"))

	// Register one row-limit check per lookup query, before the group loop
	// below registers any M-assignment task on the same witness round. The
	// check runs first (registration order), so the prover fails fast — it
	// panics on an over-limit lookup instead of walking its (billions of) rows
	// to fill M; the verifier re-runs it and rejects the proof. The check
	// validates both the query's included (A) and including (B) sides.
	registerRowLimitChecks(groups)

	var (
		fractions  []wiop.Fraction
		consumedQs []*wiop.TableRelationQuery
	)
	for _, g := range groups {
		gFractions := compileGroup(g, gamma, coinRound, compCtx)
		fractions = append(fractions, gFractions...)
		for _, inc := range g.included {
			consumedQs = append(consumedQs, inc.query)
		}
	}

	ld := sys.NewLogDerivativeSum(compCtx.Childf("aggregated"), fractions)

	// The verifier check: the aggregated log-derivative sum must be zero.
	ld.Result.Round().RegisterVerifierAction(&ResultIsZeroVerifierAction{LogDerivativeSum: ld})

	// Mark every consumed query as reduced so subsequent compiler passes skip
	// them. We deliberately wait until the LogDerivativeSum has been
	// registered so a panic during construction leaves the system unchanged.
	for _, q := range consumedQs {
		q.MarkAsReduced()
	}
}

// registerRowLimitChecks guards the per-side row budget on both the compile and
// runtime paths. It runs ahead of the group loop that places each subgroup's
// M-assignment task, so every check it registers is ordered before M is filled.
//
// Two layers of enforcement guard the budget:
//
//   - Compile time: each query is checked against the full budget via
//     [wiop.TableRelationQuery.PrecheckRowLimit], which panics immediately if a
//     single query is too big to fit into any subgroup on its own (an A or B
//     side that already reaches the budget alone). That check counts dynamic
//     (or otherwise unsized) modules as their maximum height 2^22, so it is a
//     conservative upper bound.
//
//   - Runtime: one [groupRowLimitAction] per subgroup re-checks the *exact*
//     per-run heights, summed across the whole subgroup, on both the prover
//     (panic) and verifier (error) sides. This is the check that enforces the
//     shared budget now that a subgroup — not a single query — is the unit that
//     shares a multiplicity column M and its accumulators: the subgroup's whole
//     A side (the union of all its A fragments) and its B side (the shared
//     lookup table) must each stay strictly below [wiop.MaxLookupRows].
//     Compile-time bin-packing (see collectGroups) keeps every subgroup under
//     budget using static heights, and this runtime action re-verifies it for
//     the concrete witness, so a dynamic module that grew unexpectedly cannot
//     slip an over-budget subgroup past the verifier.
//
// The subgroup action is registered on the subgroup's witness round, ahead of
// the M-assignment task that compileGroup later places on the same round, so an
// over-budget subgroup fails before M is filled.
func registerRowLimitChecks(groups []*lookupGroup) {
	for _, g := range groups {
		// A query's A fragments all live in the same subgroup (collectGroups
		// keeps them together), so each query belongs to exactly one subgroup.
		// De-duplicate the per-fragment included specs down to their queries for
		// the compile-time per-query precheck.
		seen := make(map[*wiop.TableRelationQuery]struct{})
		for _, inc := range g.included {
			if _, ok := seen[inc.query]; ok {
				continue
			}
			seen[inc.query] = struct{}{}
			inc.query.PrecheckRowLimit(wiop.MaxLookupRows)
		}

		// Runtime check on the subgroup aggregate. One instance serves both
		// roles: it panics as a prover action and returns an error as a verifier
		// action.
		rowLimit := &groupRowLimitAction{group: g}
		g.witnessRound.RegisterAction(rowLimit)
		g.witnessRound.RegisterVerifierAction(rowLimit)
	}
}

// collectGroups walks sys.TableRelations once, picks out every unreduced
// inclusion query, buckets them by the canonical identity of their B side, and
// then bin-packs each bucket into one or more subgroups.
//
// Bin-packing is the crux of the row-limit fix. Every subgroup carries its own
// multiplicity column M per B fragment and therefore drains the full
// [wiop.MaxLookupRows] budget independently — proving S ⊆ T for a subset of the
// bucket's lookups is a self-contained log-derivative identity. So instead of
// forcing every lookup that shares a table T into one group (whose shared M made
// the per-lookup budget shrink as MaxLookupRows / lookupCount, which is what
// broke lookups against large tables such as the byte-range tables), we greedily
// accumulate lookups into a subgroup and open a fresh one whenever the next
// lookup would push the subgroup's static A-side row total to the budget.
//
// Three invariants are established here and relied on by [compileGroup]:
//
//   - one multiplicity column M per B fragment *per subgroup* — this is what
//     lets each subgroup's B-side sum cancel the union of its own A-side sums
//     in the final log-derivative identity;
//
//   - a subgroup's static A-side row total (dynamic modules counted as their
//     2^22 maximum) stays strictly below [wiop.MaxLookupRows], so at runtime —
//     where module heights only ever shrink — every multiplicity M fits well
//     inside the field and the reduced accumulators cannot overflow;
//
//   - each subgroup's witnessRound is the latest round across every column it
//     touches (the shared B fragments and every A fragment of every query in
//     the subgroup), so M can be allocated on a round where all its inputs are
//     already committed and *before* the α/γ coin round.
//
// A single query is always kept whole inside one subgroup: all of its A
// fragments land together, which keeps the per-query row-limit check (see
// [registerRowLimitChecks]) meaningful. A query too big to fit any subgroup on
// its own is caught there at compile time.
//
// The returned slice is in a deterministic order — buckets in first-appearance
// order of their B key, subgroups in query order within a bucket — so [Compile]
// needs no further sorting.
func collectGroups(sys *wiop.System) []*lookupGroup {
	// bucket gathers every query that targets one canonical B side, together
	// with the canonical B-fragment descriptors taken from the first query to
	// hit the key.
	type bucket struct {
		includings []includingTable
		queries    []*wiop.TableRelationQuery
	}
	buckets := make(map[string]*bucket)
	var order []string // first-appearance order of B keys, for deterministic output

	for _, q := range sys.TableRelations {
		if q.IsReduced() {
			continue
		}
		// Permutation queries are handled by the grandproduct pass.
		if q.Kind == wiop.KindPermutation {
			continue
		}
		if len(q.B) == 0 {
			panic(fmt.Sprintf(
				"wiop/compilers/lookuptologderivsum: query %q has an empty B side",
				q.Context().Path(),
			))
		}
		// Bucket key: queries that target the same B side — the same ordered
		// union of lookup-table fragments — share the canonical includingTable
		// descriptors. Two queries with distinct B descriptors that happen to
		// reference the same underlying columns *with the same shifts and
		// selectors, in the same order* are intentionally collapsed;
		// canonicalIncludingKeyMulti encodes exactly the per-fragment (column
		// pointer, shift, selector) tuples so equality of key ⇔ equality of B
		// side as seen by the prover.
		key := canonicalIncludingKeyMulti(q.B)
		b, ok := buckets[key]
		if !ok {
			// First query to hit this key supplies the canonical
			// includingTable descriptors. Subsequent queries with the same
			// key reuse them as-is — they are guaranteed by the key to
			// describe identical fragments, so we deliberately skip
			// re-validating cols/selector/module on later hits.
			b = &bucket{includings: make([]includingTable, len(q.B))}
			for frag, bTab := range q.B {
				it := includingTable{
					cols:     bTab.Columns,
					selector: bTab.Selector,
					module:   bTab.Module(),
				}
				if !allIncludingColumnsShareModule(it) {
					panic(fmt.Sprintf(
						"wiop/compilers/lookuptologderivsum: query %q B fragment %d has "+
							"columns or a selector living on different modules",
						q.Context().Path(), frag,
					))
				}
				b.includings[frag] = it
			}
			buckets[key] = b
			order = append(order, key)
		}
		b.queries = append(b.queries, q)
	}

	// Bin-pack each bucket's queries into subgroups whose static A-side row
	// count stays strictly below the budget.
	var groups []*lookupGroup
	for _, key := range order {
		b := buckets[key]
		var (
			current  *lookupGroup
			curACost uint64
		)
		for _, q := range b.queries {
			qACost := staticTableRows(q.A)
			// Open a fresh subgroup for the first query, or whenever adding this
			// query would bring the running A-side total up to the budget. A
			// query whose own A side already reaches the budget still lands in a
			// (fresh) subgroup here; the compile-time precheck in
			// registerRowLimitChecks rejects it before any witness is assigned.
			if current == nil || curACost+qACost >= wiop.MaxLookupRows {
				// All subgroups of a bucket share the same read-only B-side
				// fragments, so they can share the bucket's slice directly.
				current = &lookupGroup{includings: b.includings}
				// Witness round must dominate every B fragment (shared across the
				// whole bucket, so identical for every subgroup).
				for _, bTab := range q.B {
					current.updateWitnessRound(bTab.Round())
				}
				groups = append(groups, current)
				curACost = 0
			}
			// Every A fragment of this query joins the current subgroup; the
			// witness round advances to dominate each one.
			for _, tabA := range q.A {
				current.updateWitnessRound(tabA.Round())
				current.addIncluded(q, tabA)
			}
			curACost += qACost
		}
	}
	return groups
}

// staticTableRows sums the compile-time row count of every fragment in tables:
// a sized module contributes its fixed height, a dynamic (or otherwise unsized)
// module its maximum possible height [wiop.ColumnSizeMaxSupported] (2^22). It
// mirrors the A-side accounting of [wiop.TableRelationQuery.PrevalidateRowLimit]
// so the bin-packing bound and the compile-time precheck agree.
func staticTableRows(tables []wiop.Table) uint64 {
	var sum uint64
	for _, tab := range tables {
		m := tab.Module()
		if m.IsSized() {
			sum += uint64(m.Size())
		} else {
			sum += uint64(wiop.ColumnSizeMaxSupported)
		}
	}
	return sum
}

// compileGroup builds the fraction list for a single B-grouped collection of
// inclusion queries. It also allocates the group's multiplicity column M on
// the group's witness round and registers the prover task that fills it
// there, so M is committed before α and γ are sampled in coinRound.
func compileGroup(
	g *lookupGroup,
	gamma *wiop.CoinField,
	coinRound *wiop.Round,
	compCtx *wiop.ContextFrame,
) []wiop.Fraction {
	gCtx := compCtx.Childf("group-%p", g)

	// All B fragments share the same column width (enforced at construction by
	// wiop.NewInclusion). Take the first fragment's width as the reference and
	// assert every A fragment matches it.
	bWidth := g.includings[0].width()
	for i, inc := range g.included {
		if len(inc.cols) != bWidth {
			panic(fmt.Sprintf(
				"wiop/compilers/lookuptologderivsum: included fragment %d in group %s has width %d "+
					"but the lookup table has width %d",
				i, gCtx.Path(), len(inc.cols), bWidth,
			))
		}
	}

	// The IsFilteredOnIncluding trick is applied group-wide: if any B fragment
	// carries a selector we prepend a head to every B fragment (its own
	// selector, or a constant 1 for the unfiltered fragments) and a constant 1
	// to every A side. This keeps the effective row width uniform across all
	// fragments and both sides, so an A row matches an active B row exactly
	// when their column values coincide.
	prependOnesToA := false
	for _, it := range g.includings {
		if it.selector != nil {
			prependOnesToA = true
			break
		}
	}
	effectiveWidth := bWidth
	if prependOnesToA {
		effectiveWidth++
	}

	// α is needed whenever we have to fold more than one column down to a
	// single field element.
	var alpha *wiop.CoinField
	if effectiveWidth > 1 {
		alpha = coinRound.NewCoinField(gCtx.Childf("alpha"))
	}

	// One −M/(γ+RLC(T)) fraction per B fragment, each with its own
	// multiplicity column M committed on the group's witness round. Placing M
	// there puts it in the same round as the witness columns it depends on and
	// crucially *before* α and γ are sampled in coinRound — without that
	// ordering a malicious prover could choose M as a function of γ and break
	// log-derivative soundness.
	fractions := make([]wiop.Fraction, 0, len(g.includings)+len(g.included))
	mCols := make([]*wiop.Column, len(g.includings))
	for frag, it := range g.includings {
		var bHead wiop.Expression
		if prependOnesToA {
			if it.selector != nil {
				bHead = it.selector
			} else {
				// Unfiltered fragment inside a filtered group: its head is the
				// constant 1, matching the A-side head so all its rows stay
				// active.
				bHead = wiop.NewConstantVector(it.module, field.One())
			}
		}
		bRLC := rlcOfViews(alpha, bHead, it.cols)

		mCol := it.module.NewColumn(
			gCtx.Childf("M-%d", frag),
			wiop.VisibilityOracle,
			g.witnessRound,
		)
		mCols[frag] = mCol

		fractions = append(fractions, wiop.Fraction{
			Numerator:   wiop.Negate(mCol.View()),
			Denominator: wiop.Add(gamma, bRLC),
		})
	}

	// The A-side fractions: one per A fragment.
	for _, inc := range g.included {
		var sHead wiop.Expression
		if prependOnesToA {
			sHead = wiop.NewConstantVector(inc.cols[0].Module(), field.One())
		}
		sRLC := rlcOfViews(alpha, sHead, inc.cols)
		sDen := wiop.Add(gamma, sRLC)
		// Numerator is the constant 1 broadcast over the A fragment's module
		// so the fraction is vector-valued on the A side.
		oneNum := wiop.NewConstantVector(inc.cols[0].Module(), field.One())
		var filter wiop.Expression
		if inc.selector != nil {
			filter = inc.selector
		}
		fractions = append(fractions, wiop.Fraction{
			Filter:      filter,
			Numerator:   oneNum,
			Denominator: sDen,
		})
	}

	// The prover task that fills every M with multiplicities once the witness
	// is in place. Registered on the group's witness round so it runs before
	// AdvanceRound samples coinRound's α and γ.
	g.witnessRound.RegisterAction(&mAssignmentTask{
		ms:              mCols,
		includings:      append([]includingTable{}, g.includings...),
		included:        append([]includedSpec{}, g.included...),
		prependOneOnAOk: prependOnesToA,
	})

	return fractions
}

// ensureNextRound returns the round immediately following r, allocating one
// via [wiop.System.NewRound] if necessary.
func ensureNextRound(sys *wiop.System, r *wiop.Round) *wiop.Round {
	if next, ok := r.Next(); ok {
		return next
	}
	return sys.NewRound()
}

// rlcOfViews builds the symbolic random linear combination
//
//	head + α·cols[0] + α²·cols[1] + …
//
// when head != nil, and
//
//	cols[0] + α·cols[1] + α²·cols[2] + …
//
// when head == nil. The effective width is len(cols) plus one when a head is
// provided; when that effective width is 1, alpha must be nil and the single
// term is returned directly.
func rlcOfViews(alpha *wiop.CoinField, head wiop.Expression, cols []*wiop.ColumnView) wiop.Expression {
	exprs := viewExprs(cols)
	if head != nil {
		exprs = append([]wiop.Expression{head}, exprs...)
	}
	return rlcExpression(alpha, exprs)
}

// rlcExpression returns exprs[0] + α·exprs[1] + α²·exprs[2] + … built as a
// Horner-form chain
//
//	((…((exprs[n-1]·α + exprs[n-2])·α + exprs[n-3])·α + …)·α + exprs[0])
//
// so the resulting symbolic tree contains no explicit α² / α³ / … sub-trees
// and uses n-1 multiplications instead of 2n-3. Matches the convention of
// linea/prover/protocol/wizardutils.RandLinCombColSymbolic
// (symbolic.NewPolyEval).
//
// When alpha is nil the slice must have exactly one element, in which case
// that element is returned directly. Requires len(exprs) >= 1.
func rlcExpression(alpha *wiop.CoinField, exprs []wiop.Expression) wiop.Expression {
	if len(exprs) == 0 {
		panic("wiop/compilers/lookuptologderivsum: rlcExpression requires at least one term")
	}
	if alpha == nil {
		if len(exprs) != 1 {
			panic("wiop/compilers/lookuptologderivsum: alpha is nil but width > 1")
		}
		return exprs[0]
	}
	alphaExpr := wiop.Expression(alpha)
	acc := exprs[len(exprs)-1]
	for i := len(exprs) - 2; i >= 0; i-- {
		acc = wiop.Add(wiop.Mul(alphaExpr, acc), exprs[i])
	}
	return acc
}

// viewExprs lifts a slice of *ColumnView into the equivalent slice of
// wiop.Expression so it can be passed to [rlcExpression].
func viewExprs(cols []*wiop.ColumnView) []wiop.Expression {
	out := make([]wiop.Expression, len(cols))
	for i, cv := range cols {
		out[i] = cv
	}
	return out
}

// groupRowLimitAction enforces, at runtime, that a single subgroup's summed row
// count stays below the budget on each side. It is the runtime counterpart to
// the compile-time bin-packing in [collectGroups]: bin-packing keeps every
// subgroup under limit using static (maximum) module heights, and this action
// re-checks the exact per-run heights so the bound is enforced for the concrete
// witness on both the prover (panic — trusted code about to build an unsound
// witness) and verifier (error — reject the proof gracefully) sides.
//
// A subgroup is the unit that shares a multiplicity column M and its
// accumulators, so it is the unit the budget applies to. The check sums,
// independently:
//
//   - the A side: the runtime height of every A fragment in the subgroup (the
//     union whose per-row multiplicities feed the subgroup's M columns), and
//   - the B side: the runtime height of every shared lookup-table fragment.
//
// and fails if either reaches limit. Summing per fragment deliberately
// double-counts two fragments that live on the same module, because each
// fragment contributes its own pass over that module's rows to the argument.
type groupRowLimitAction struct {
	group *lookupGroup
}

// Run implements [wiop.ProverAction]: it panics on an over-limit subgroup.
func (a *groupRowLimitAction) Run(rt *wiop.Runtime) {
	if err := a.validate(rt); err != nil {
		panic(err)
	}
}

// Check implements [wiop.VerifierAction]: it returns an error on an over-limit
// subgroup so the verifier rejects the proof.
func (a *groupRowLimitAction) Check(rt *wiop.Runtime) error {
	return a.validate(rt)
}

// validate sums the subgroup's per-run row counts on each side and returns an
// error if either side reaches the limit. Mirrors the accounting of
// [wiop.TableRelationQuery.ValidateRowLimit], extended from a single query to
// the whole subgroup's union of A fragments against the shared B side.
func (a *groupRowLimitAction) validate(rt *wiop.Runtime) error {
	var aRows uint64
	for _, inc := range a.group.included {
		aRows += uint64(inc.cols[0].Module().RuntimeSize(rt))
	}
	if aRows >= wiop.MaxLookupRows {
		return a.overLimitError("A", aRows)
	}

	var bRows uint64
	for _, it := range a.group.includings {
		bRows += uint64(it.module.RuntimeSize(rt))
	}
	if bRows >= wiop.MaxLookupRows {
		return a.overLimitError("B", bRows)
	}
	return nil
}

// overLimitError builds the shared over-budget message, listing the lookup
// queries folded into the offending subgroup so the failure is traceable back
// to source lookups.
func (a *groupRowLimitAction) overLimitError(side string, rows uint64) error {
	seen := make(map[*wiop.TableRelationQuery]struct{})
	var paths []string
	for _, inc := range a.group.included {
		if _, ok := seen[inc.query]; ok {
			continue
		}
		seen[inc.query] = struct{}{}
		paths = append(paths, inc.query.Context().Path())
	}
	return fmt.Errorf(
		"wiop/compilers/lookuptologderivsum: subgroup [%s] has total rows on the %s side "+
			"reaching %d, which is >= the per-subgroup row limit %d (the row budget shared by the "+
			"lookups bin-packed into one multiplicity column); the accumulators in the reduced "+
			"constraints would overflow over a small field",
		strings.Join(paths, ", "), side, rows, uint64(wiop.MaxLookupRows),
	)
}

// ResultIsZeroVerifierAction asserts that the aggregated [wiop.LogDerivativeSum]
// result cell holds the zero field element. This is the standard
// log-derivative identity: the sum of A-side fractions cancels the sum of
// T-side fractions exactly when every selected A row is in the union of
// selected B rows.
//
// Exported (with an exported field) so out-of-package consumers — notably the
// verifier-ray codegen — can recognise a lookup-reduced LogDerivativeSum and
// flag that its Result must be zero.
type ResultIsZeroVerifierAction struct {
	LogDerivativeSum *wiop.LogDerivativeSum
}

// Check implements [wiop.VerifierAction].
func (a *ResultIsZeroVerifierAction) Check(rt *wiop.Runtime) error {
	v := rt.GetCellValue(a.LogDerivativeSum.Result)
	if !v.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/lookuptologderivsum: aggregated lookup result for query %q must be zero",
			a.LogDerivativeSum.Context().Path(),
		)
	}
	return nil
}

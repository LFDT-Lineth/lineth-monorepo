// Package lookuptologderivsum compiles every unreduced [wiop.LookupQuery]
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
// After Compile runs, every consumed [wiop.LookupQuery] is marked reduced
// and a single [wiop.LogDerivativeSum] query is left in sys for the
// downstream [logderivativesum] compiler pass to consume.
//
// Scope (MVP): the compiler handles inclusion queries with len(B) == 1
// (single-fragment lookup table). Multiple A fragments per query and
// multiple queries per shared B are both supported. Permutation queries are
// ignored. Multi-fragment B queries are explicitly rejected with a panic
// at compile time so callers can rework them into multiple single-fragment
// queries before running this pass.
package lookuptologderivsum

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Compile reduces every unreduced inclusion [wiop.LookupQuery] in sys to a
// single [wiop.LogDerivativeSum] query plus a multiplicity column per
// lookup-table fragment, plus a verifier action that asserts the resulting
// LogDerivativeSum result equals zero.
//
// All consumed inclusion queries are marked reduced. Permutation queries and
// already-reduced queries are skipped. If sys contains no eligible queries
// the function is a no-op.
//
// Panics if any inclusion query has len(B) != 1 (multi-fragment lookup tables
// are out of scope for this pass).
func Compile(sys *wiop.System) {
	groups := collectGroups(sys)
	if len(groups) == 0 {
		return
	}

	// Allocate the LogDerivativeSum in a deterministic group order. We sort
	// by the witness-round ID then by canonical key so the registration order
	// is independent of map iteration.
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Determine the latest witness round across every group: this dictates
	// where the coin and result rounds live. Groups whose only contributing
	// columns are precomputed leave witnessRound nil (see
	// [lookupGroup.updateWitnessRound]); they are skipped here and patched up
	// after the loop so the compiler still emits its M / α / γ on an
	// interactive round.
	var latestWitness *wiop.Round
	for _, k := range keys {
		g := groups[k]
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
	for _, k := range keys {
		if groups[k].witnessRound == nil {
			groups[k].witnessRound = latestWitness
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

	var (
		fractions  []wiop.Fraction
		consumedQs []*wiop.LookupQuery
	)
	for _, k := range keys {
		g := groups[k]
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

// collectGroups walks sys.TableRelations once, picks out every unreduced
// inclusion query, and groups them by the canonical identity of their
// (single) B-side fragment.
//
// Two invariants are established here and relied on by [compileGroup]:
//
//   - one multiplicity column M per shared B fragment, not per query — this
//     is what lets the B-side sum cancel the union of A-side sums in the
//     final log-derivative identity;
//
//   - each group's witnessRound is the latest round across every column the
//     group touches (B and every A fragment of every query in the group), so
//     M can be allocated on a round where all its inputs are already
//     committed and *before* the α/γ coin round.
//
// The returned map is unordered; callers that need a deterministic emission
// order sort the keys (see [Compile]).
func collectGroups(sys *wiop.System) map[string]*lookupGroup {
	groups := make(map[string]*lookupGroup)
	for _, q := range sys.TableRelations {
		if q.IsReduced() {
			continue
		}
		if len(q.B) != 1 {
			panic(fmt.Sprintf(
				"wiop/compilers/lookuptologderivsum: query %q has len(B)=%d; "+
					"only single-fragment lookup tables are supported in this pass",
				q.Context().Path(), len(q.B),
			))
		}
		// Grouping key: queries that target the same lookup-table fragment
		// fold into the same lookupGroup and therefore share a single M
		// column. Two queries with distinct B descriptors that happen to
		// reference the same underlying columns *with the same shifts and
		// selector* are intentionally collapsed; canonicalIncludingKey
		// encodes exactly the (column pointer, shift, selector) tuple so
		// equality of key ⇔ equality of fragment as seen by the prover.
		// Without this collapse, the same lookup table referenced by N
		// queries would yield N independent multiplicity columns and the
		// B-side terms would no longer cancel the union of A-side terms.
		key := canonicalIncludingKey(q.B[0])
		g, ok := groups[key]
		if !ok {
			// First query to hit this key supplies the canonical
			// includingTable descriptor. Subsequent queries with the same
			// key reuse it as-is — they are guaranteed by the key to
			// describe an identical fragment, so we deliberately skip
			// re-validating cols/selector/module on later hits.
			g = &lookupGroup{
				including: includingTable{
					cols:     q.B[0].Columns,
					selector: q.B[0].Selector,
					module:   q.B[0].Module(),
				},
			}
			if !allIncludingColumnsShareModule(g.including) {
				panic(fmt.Sprintf(
					"wiop/compilers/lookuptologderivsum: query %q has a B fragment whose "+
						"columns or selector live on different modules",
					q.Context().Path(),
				))
			}
			groups[key] = g
		}
		// Witness round must dominate every column referenced by the group.
		// Including both B and every A fragment of *this* query keeps the
		// invariant under incremental merging: when a later query reuses the
		// same B but supplies an A on a later round, witnessRound advances
		// accordingly so M (allocated on witnessRound by [compileGroup]) is
		// still committed before α/γ are sampled in witnessRound + 1 —
		// the soundness ordering of the log-derivative argument.
		g.updateWitnessRound(q.B[0].Round())
		for _, tabA := range q.A {
			g.updateWitnessRound(tabA.Round())
			g.addIncluded(q, tabA)
		}
	}
	return groups
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

	// Consistency: every A fragment must have the same width as B.
	for i, inc := range g.included {
		if len(inc.cols) != g.including.width() {
			panic(fmt.Sprintf(
				"wiop/compilers/lookuptologderivsum: included fragment %d in group %s has width %d "+
					"but the lookup table has width %d",
				i, gCtx.Path(), len(inc.cols), g.including.width(),
			))
		}
	}

	// α is needed whenever we have to fold more than one column down to a
	// single field element. The "effective" B-side width is the original
	// width plus one when the IsFilteredOnIncluding trick prepends the
	// B-selector to the row (and a constant 1 to every A-side).
	prependOnesToA := g.including.selector != nil
	effectiveWidth := g.including.width()
	if prependOnesToA {
		effectiveWidth++
	}

	var alpha *wiop.CoinField
	if effectiveWidth > 1 {
		alpha = coinRound.NewCoinField(gCtx.Childf("alpha"))
	}

	// Build the symbolic random linear combination of the B-side columns.
	// When the B-side carries a selector we prepend it and prepend a
	// constant-1 to every A side, mirroring the standard
	// IsFilteredOnIncluding trick.
	var bHead wiop.Expression
	if prependOnesToA {
		bHead = g.including.selector
	}
	bRLC := rlcOfViews(alpha, bHead, g.including.cols)

	// M lives on the same module as B, in the group's witness round. This
	// places M in the same round as the witness columns it depends on, and
	// crucially *before* α and γ are sampled in coinRound — without that
	// ordering a malicious prover could choose M as a function of γ and
	// break log-derivative soundness.
	mCol := g.including.module.NewColumn(
		gCtx.Childf("M"),
		wiop.VisibilityOracle,
		g.witnessRound,
	)

	// The T-side fraction:  −M / (γ + bRLC).
	negM := wiop.Negate(mCol.View())
	bDen := wiop.Add(gamma, bRLC)
	fractions := make([]wiop.Fraction, 0, 1+len(g.included))
	fractions = append(fractions, wiop.Fraction{
		Numerator:   negM,
		Denominator: bDen,
	})

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

	// The prover task that fills M with multiplicities once the witness is
	// in place. Registered on the group's witness round so it runs before
	// AdvanceRound samples coinRound's α and γ.
	g.witnessRound.RegisterAction(&mAssignmentTask{
		m:               mCol,
		bCols:           g.including.cols,
		bSelector:       g.including.selector,
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
func (a *ResultIsZeroVerifierAction) Check(rt wiop.Runtime) error {
	v := rt.GetCellValue(a.LogDerivativeSum.Result)
	if !v.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/lookuptologderivsum: aggregated lookup result for query %q must be zero",
			a.LogDerivativeSum.Context().Path(),
		)
	}
	return nil
}

package grandproduct

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// compilePermutations reduces every unreduced permutation [wiop.TableRelationQuery]
// in sys into a single aggregated [wiop.GrandProduct] query, then registers a
// prover action that assigns the grand-product Result and a verifier action
// ([CheckResultIsOne]) asserting that Result == 1.
//
// For each permutation query a fresh β coin (and an α coin when any fragment is
// multi-column) is sampled in the round following the latest column round.
// Each A-fragment contributes a numerator factor β + RLC_α(row); each
// B-fragment a denominator factor. The RLC carries an α^w length sentinel (see
// rlcOfTable) so fragments of a single query may differ in width without a
// short row aliasing a zero-padded longer one. Distinct coins per query make it
// sound to pack factors from different queries into the same downstream Z
// column.
//
// A fragment carrying a selector contributes the conditional factor
// [SelectorFold](sel, β + RLC_α(row)) instead: selected rows contribute their
// usual factor and unselected rows the neutral factor 1, so the identity
// becomes "the multiset of selected A rows equals the multiset of selected B
// rows". The neutral-factor trick is only sound for {0,1}-valued selectors,
// but this pass emits NO binarity constraint of its own: constraining
// sel·(sel−1) = 0 is the caller's obligation (see [wiop.System.NewPermutation]).
// Selectors are in practice already constrained where they are built — most are
// shared by several queries and fragments — so emitting one here would
// duplicate an existing constraint on nearly every selector.
func compilePermutations(sys *wiop.System) {
	var (
		perms    []*wiop.TableRelationQuery
		maxRound *wiop.Round
	)
	for _, q := range sys.TableRelations {
		if q.IsReduced() || q.Kind != wiop.KindPermutation {
			continue
		}
		perms = append(perms, q)
		if r := q.Round(); r != nil && (maxRound == nil || r.ID > maxRound.ID) {
			maxRound = r
		}
	}
	if len(perms) == 0 {
		return
	}
	if maxRound == nil {
		panic("wiop/compilers/grandproduct: permutation query references no round-bearing column")
	}

	// Enforce the per-permutation row limit before the grand-product discharge
	// pass walks any row. Packing factors into one Z column shrinks the NUMBER of
	// Z columns, not the rows each one walks (a Z column still spans its module's
	// rows), so the packing arity does not enlarge the per-side row budget. The
	// accumulators are also per module, so the budget does NOT grow with the
	// number of permutations either (the aggregation into one GrandProduct only
	// merges the final Result == 1 identity). The effective per-side limit is
	// therefore MaxPermutationRows. Registered on maxRound (the witness round),
	// which precedes the Result round where the row-walking prover actions live,
	// so the prover fails fast; the verifier re-checks and rejects.
	limit := wiop.MaxPermutationRows
	for _, q := range perms {
		// Compile-time check: the same per-side row budget, but evaluated from
		// static module sizes with dynamic (or otherwise unsized) modules counted
		// as their maximum height 2^22. This surfaces an over-budget permutation
		// during compilation — before any witness is assigned — on top of the
		// exact prover/verifier runtime checks registered below. It panics,
		// matching the compile-time failure convention of this pass.
		q.PrecheckRowLimit(limit)

		// One instance serves both roles: it panics as a prover action and
		// returns an error as a verifier action.
		rowLimit := &rowLimitAction{query: q, limit: limit}
		maxRound.RegisterAction(rowLimit)
		maxRound.RegisterVerifierAction(rowLimit)
	}

	// β/α live one round after the witness columns; the GrandProduct Result one
	// round after that. Both rounds must exist before NewGrandProduct runs.
	coinRound := ensureNextRound(sys, maxRound)
	ensureNextRound(sys, coinRound)

	compCtx := sys.Context.Childf("grandproduct-perm")

	var numerators, denominators []wiop.Expression
	for qi, q := range perms {
		qCtx := compCtx.Childf("perm-%d", qi)

		// α is needed whenever any fragment is multi-column: both to combine a
		// tuple's columns and to carry the α^w length sentinel. When every
		// fragment is width-1 the query is single-width, no cross-width
		// aliasing is possible, and β alone (no sentinel, no α) suffices.
		var alpha *wiop.CoinField
		if maxFragmentWidth(q) > 1 {
			alpha = coinRound.NewCoinField(qCtx.Childf("alpha"))
		}
		beta := coinRound.NewCoinField(qCtx.Childf("beta"))

		// Selector binarity is a caller obligation, not enforced here: see the
		// doc comment on compilePermutations.
		for _, tab := range q.A {
			numerators = append(numerators, permutationFactor(alpha, beta, tab))
		}
		for _, tab := range q.B {
			denominators = append(denominators, permutationFactor(alpha, beta, tab))
		}
	}

	gp := sys.NewGrandProduct(compCtx.Childf("aggregated"), numerators, denominators)
	// The Result cell is assigned by the grandproduct discharge pass
	// (compileGrandProducts), shared by every GrandProduct. Here we only attach
	// the permutation-specific verifier predicate Result == 1.
	gp.Result.Round().RegisterVerifierAction(&CheckResultIsOne{GrandProduct: gp})

	for _, q := range perms {
		q.MarkAsReduced()
	}
}

// ensureNextRound returns the round immediately following r, allocating one via
// [wiop.System.NewRound] if necessary.
func ensureNextRound(sys *wiop.System, r *wiop.Round) *wiop.Round {
	if next, ok := r.Next(); ok {
		return next
	}
	return sys.NewRound()
}

// permutationFactor builds the grand-product factor a single permutation
// fragment contributes: β + RLC_α(row) for a plain fragment, wrapped by
// [SelectorFold] when the fragment carries a selector so unselected rows
// contribute the neutral factor 1.
func permutationFactor(alpha, beta *wiop.CoinField, tab wiop.Table) wiop.Expression {
	return SelectorFold(tab.Selector, wiop.Add(rlcOfTable(alpha, tab), beta))
}

// SelectorFold wraps a grand-product factor with a row selector:
//
//	selector·factor + (1 − selector)
//
// A selected row contributes factor and an unselected row the neutral factor
// 1, dropping out of the product. With a nil selector the factor is returned
// unchanged.
//
// The selector MUST be {0,1}-valued for the fold to be sound: the fold equals
// 1 + sel·(factor − 1), so an unconstrained sel lets a prover scale a row's
// contribution to any value it likes and match arbitrary multisets. No caller
// of this helper emits the sel·(sel−1) = 0 constraint — neither the
// permutation nor the messagebus compiler — so whoever builds the selector
// column must constrain it.
//
// Exported so the messagebus compiler (and any other grand-product-based
// pass) shares one definition of the conditional-factor trick.
func SelectorFold(selector *wiop.ColumnView, factor wiop.Expression) wiop.Expression {
	if selector == nil {
		return factor
	}
	sel := wiop.Expression(selector)
	one := wiop.NewConstantField(field.One())
	return wiop.Add(wiop.Mul(sel, factor), wiop.Sub(one, sel))
}

// rlcOfTable builds the width-binding random linear combination of a
// permutation table's row (see [RLCWithSentinel]). The table's selector, if
// any, is NOT part of the fold — it wraps the whole factor via [SelectorFold].
//
// When alpha is nil the whole query is single-width (every fragment width-1),
// no cross-width aliasing is possible, and the lone column view is returned
// with no sentinel.
func rlcOfTable(alpha *wiop.CoinField, tab wiop.Table) wiop.Expression {
	cols := tab.Columns
	if alpha == nil {
		if len(cols) != 1 {
			panic("wiop/compilers/grandproduct: alpha is nil but table width > 1")
		}
		return cols[0]
	}
	return RLCWithSentinel(alpha, cols)
}

// RLCWithSentinel builds the width-binding random linear combination
//
//	α^w + col[0] + α·col[1] + α²·col[2] + … + α^{w-1}·col[w-1]
//
// where w = len(cols). The leading α^w is a length sentinel: it makes the fold
// injective across widths, so rows of different widths can never alias — a
// short tuple never collides with a zero-padded longer one.
//
// The sentinel is folded in for free by evaluating the coefficient sequence
// [1, col[w-1], …, col[0]] at α via Horner; seeding acc = α + col[w-1]
// collapses the leading 1·α step, so the sentinel costs one extra addition and
// NO extra multiplication over the plain RLC.
//
// Requires a non-nil alpha and at least one column. Exported so the
// messagebus compiler shares one definition of the sentinel fold.
func RLCWithSentinel(alpha *wiop.CoinField, cols []*wiop.ColumnView) wiop.Expression {
	if alpha == nil {
		panic("wiop/compilers/grandproduct: RLCWithSentinel requires a non-nil alpha")
	}
	if len(cols) == 0 {
		panic("wiop/compilers/grandproduct: RLCWithSentinel requires at least one column")
	}
	alphaExpr := wiop.Expression(alpha)
	// acc = α + col[w-1] fuses the leading 1·α + col[w-1] Horner step, seeding
	// the coefficient sequence [1, col[w-1], …] that carries the α^w sentinel.
	acc := wiop.Add(alphaExpr, cols[len(cols)-1])
	for i := len(cols) - 2; i >= 0; i-- {
		acc = wiop.Add(wiop.Mul(alphaExpr, acc), cols[i])
	}
	return acc
}

// maxFragmentWidth returns the greatest column width across every A and B
// fragment of a permutation query. It drives α allocation: α is required once
// any fragment is multi-column, both to combine columns and to carry the α^w
// length sentinel that disambiguates mixed widths.
func maxFragmentWidth(q *wiop.TableRelationQuery) int {
	w := 0
	for _, tab := range q.A {
		if tab.Width() > w {
			w = tab.Width()
		}
	}
	for _, tab := range q.B {
		if tab.Width() > w {
			w = tab.Width()
		}
	}
	return w
}

// CheckResultIsOne asserts that the aggregated [wiop.GrandProduct] Result cell
// equals one — the defining identity of a permutation argument: ∏(β + RLC(A))
// equals ∏(β + RLC(B)) exactly when A and B are equal as multisets.
//
// Exported (with an exported field) so out-of-package consumers — notably the
// verifier-ray codegen — can recognise a permutation-reduced GrandProduct.
type CheckResultIsOne struct {
	GrandProduct *wiop.GrandProduct
}

// Check implements [wiop.VerifierAction].
func (c *CheckResultIsOne) Check(rt *wiop.Runtime) error {
	v := rt.GetCellValue(c.GrandProduct.Result)
	if !v.IsOne() {
		return fmt.Errorf(
			"wiop/compilers/grandproduct: permutation result for query %q must be one",
			c.GrandProduct.Context().Path(),
		)
	}
	return nil
}

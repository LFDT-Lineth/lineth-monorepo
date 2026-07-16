package grandproduct

import (
	"fmt"

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

		for _, tab := range q.A {
			numerators = append(numerators, wiop.Add(rlcOfTable(alpha, tab), beta))
		}
		for _, tab := range q.B {
			denominators = append(denominators, wiop.Add(rlcOfTable(alpha, tab), beta))
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

// rlcOfTable builds the width-binding random linear combination of a
// permutation table's row:
//
//	α^w + col[0] + α·col[1] + α²·col[2] + … + α^{w-1}·col[w-1]
//
// where w = len(cols). The leading α^w is a length sentinel: it makes the fold
// injective across widths, so fragments of a query may differ in width without
// a short row aliasing a zero-padded longer one. Permutation tables carry no
// selector, so there is no head filter term.
//
// The sentinel is folded in for free by evaluating the coefficient sequence
// [1, col[w-1], …, col[0]] at α via Horner; seeding acc = α + col[w-1]
// collapses the leading 1·α step, so the sentinel costs one extra addition and
// NO extra multiplication over the plain RLC.
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

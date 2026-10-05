// Package localvanishing implements the scalar-Vanishing compiler pass for the
// wiop protocol framework. It mirrors the legacy
// prover/protocol/compiler/localcs pass: a row-pinned predicate is lifted to
// a polynomial identity over the whole domain by multiplying with a Lagrange
// indicator that is 1 at the pinned row and 0 elsewhere.
//
// A scalar [wiop.Vanishing] is one whose [wiop.Expression] is not
// multi-valued — typically produced by [wiop.Module.NewLocalConstraint],
// which lowers a row-pinned predicate into a scalar expression by rewriting
// every [wiop.ColumnView] to a [wiop.ColumnPosition] at the chosen row.
//
// For each unreduced scalar Vanishing, this pass:
//
//  1. Walks the expression and collects every distinct
//     [wiop.ColumnPosition] leaf.
//  2. Determines the anchor row min = the smallest position seen across
//     those leaves.
//  3. Creates (or reuses, via a per-(module, anchor) cache) a
//     [wiop.LagrangeSelector] L_min on the module: the public indicator that
//     is 1 at row min and 0 elsewhere. Unlike a committed column, the selector
//     carries no assignment — it is evaluated from the module's runtime size,
//     so the lift works for dynamic-size modules as well as static ones.
//  4. Rewrites the expression: every [wiop.ColumnPosition]{c, p} leaf
//     becomes c.View().Shift(p − min). Evaluating the rewritten expression
//     at row x of the domain reads column c at row (x + p − min) mod n;
//     at the anchor x = min that is exactly c[p].
//  5. Multiplies the rewritten expression by L_min and registers the product
//     as a new multi-valued [wiop.Vanishing] on the module — to be discharged
//     by the global-quotient compiler, which evaluates the selector
//     analytically on the quotient coset and at the verifier's point.
//  6. Marks the original scalar vanishing as reduced.
//
// The rewrite preserves sharing. It visits each distinct node once (see
// [wiop.EditExpression]) and re-interns every rebuilt node through a
// per-module structural cache seeded with the module's multi-valued
// vanishings, so a lifted subtree reuses the pointer of any structurally
// identical node already in the module — from a global constraint, or from
// another lifted constraint, whatever its anchor. Without it, every lifted
// leaf and compound would be a fresh allocation and the pointer-keyed caches
// downstream (the expression compiler, the verifier-ray codegen walk) would
// see a pure tree.
//
// At row min the product evaluates to the original local predicate; at
// every other row the Lagrange factor is zero. The new vanishing therefore
// holds across the entire domain exactly when the original local predicate
// holds.
//
// Caller order: invoke localvanishing.Compile(sys) BEFORE global.Compile(sys)
// — the global compiler discharges the multi-valued vanishings this pass
// emits.
//
// Limitations:
//
//   - Vanishings whose expression has no column references at all are
//     rejected — there is no anchor row to multiply a Lagrange indicator
//     against.
//
// [wiop.Cell] and [wiop.CoinField] leaves (both base and extension) are
// fully supported: the global compiler broadcasts their runtime value as a
// constant scalar across the coset and handles the resulting base/extension
// arithmetic uniformly.
package localvanishing

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// lagrangeKey identifies a Lagrange column by (module, anchor row). Cached
// across a single Compile call so identical anchors share one precomputed
// column.
type lagrangeKey struct {
	modIdx int
	anchor int
}

// Compile reduces every unreduced scalar [wiop.Vanishing] across all modules
// of sys to an equivalent multi-valued vanishing via the Lagrange trick. See
// the package documentation for the reduction strategy and call-order
// requirements.
func Compile(sys *wiop.System) {
	type entry struct {
		mIdx int
		vIdx int
		m    *wiop.Module
		v    *wiop.Vanishing
	}
	var work []entry
	for mIdx, m := range sys.Modules {
		for vIdx, v := range m.Vanishings {
			if v.IsReduced() {
				continue
			}
			if v.Expression.IsMultiValued() {
				continue
			}
			work = append(work, entry{mIdx, vIdx, m, v})
		}
	}
	if len(work) == 0 {
		return
	}

	compCtx := sys.Context.Childf("local-vanishing")
	selectors := make(map[lagrangeKey]*wiop.LagrangeSelector)
	interners := make(map[int]*interner)

	for _, w := range work {
		in, ok := interners[w.mIdx]
		if !ok {
			in = seededInterner(w.m)
			interners[w.mIdx] = in
		}
		ctx := compCtx.Childf("m%d-v%d", w.mIdx, w.vIdx)
		reduce(ctx, w.m, w.mIdx, w.v, selectors, in)
	}
}

// seededInterner returns an [interner] for m, seeded with the nodes of every
// unreduced multi-valued vanishing already registered on it.
func seededInterner(m *wiop.Module) *interner {
	in := newInterner()
	visited := make(map[wiop.Expression]struct{})
	for _, v := range m.Vanishings {
		if v.IsReduced() || !v.Expression.IsMultiValued() {
			continue
		}
		in.seed(v.Expression, visited)
	}
	return in
}

// reduce performs the Lagrange × shifted-expression rewrite for a single
// scalar vanishing.
func reduce(
	ctx *wiop.ContextFrame,
	m *wiop.Module,
	mIdx int,
	v *wiop.Vanishing,
	cache map[lagrangeKey]*wiop.LagrangeSelector,
	in *interner,
) {
	anchor, ok := minColumnPosition(v.Expression)
	if !ok {
		panic(fmt.Sprintf(
			"wiop/compilers/localvanishing: %s: scalar vanishing has no column references; nothing to lift",
			v.Context().Path(),
		))
	}

	key := lagrangeKey{mIdx, anchor}
	lagSel, ok := cache[key]
	if !ok {
		lagSel = wiop.NewLagrangeSelector(m, anchor)
		cache[key] = lagSel
	}

	shifted := wiop.EditExpression(v.Expression, func(
		curr wiop.Expression, newChildren []wiop.Expression,
	) wiop.Expression {
		if cp, ok := curr.(*wiop.ColumnPosition); ok {
			return in.intern(cp.Column.View().Shift(cp.Position - anchor))
		}
		return in.intern(wiop.DefaultConstruct(curr, newChildren))
	})

	lifted := in.intern(wiop.Mul(shifted, in.intern(lagSel)))
	// Register with no cancelled positions: the constraint must hold on every
	// row. That is safe and intentional. The shifted sub-expression may read
	// out-of-domain (wrap-around) values at rows other than `anchor`, but the
	// Lagrange factor L_anchor is zero everywhere except at `anchor`, so the
	// product is trivially zero at every non-anchor row regardless of what
	// the shift wraps to. Letting NewVanishing auto-cancel those rows would
	// be redundant and would inflate the constraint's effective degree
	// (cancellation polynomial degree feeds into computeRatio in the global
	// compiler, growing the quotient ratio, the FFT coset, and the number of
	// committed quotient share columns).
	m.NewVanishingManual(ctx.Childf("global"), lifted)

	v.MarkAsReduced()
}

// minColumnPosition returns the smallest position appearing in a
// ColumnPosition leaf of expr, and false when there is none. *Cell and
// *CoinField leaves are allowed and pass through unchanged into the lifted
// expression: the global compiler broadcasts them (base or extension) as
// constants across the coset. Shared nodes are visited once.
func minColumnPosition(expr wiop.Expression) (int, bool) {
	var (
		anchor  int
		found   bool
		visited = make(map[wiop.Expression]struct{})
	)
	var walk func(e wiop.Expression)
	walk = func(e wiop.Expression) {
		if _, ok := visited[e]; ok {
			return
		}
		visited[e] = struct{}{}
		switch t := e.(type) {
		case *wiop.ColumnPosition:
			if !found || t.Position < anchor {
				anchor, found = t.Position, true
			}
		case *wiop.ArithmeticOperation:
			for _, op := range t.Operands {
				walk(op)
			}
			// *Cell, *CoinField, *Constant leaves carry no positional info.
		}
	}
	walk(expr)
	return anchor, found
}

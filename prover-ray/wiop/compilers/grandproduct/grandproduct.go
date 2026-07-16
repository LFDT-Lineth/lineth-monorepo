// Package grandproduct compiles permutation arguments via the grand-product
// technique. It is the prover-ray analogue of
// linea/prover/protocol/compiler/permutation.
//
// Compilation is two-phase, mirroring the linea compiler's
// CompileViaGrandProduct:
//
//   - Phase 1 (permutation → grand product): every unreduced permutation
//     [wiop.TableRelationQuery] is reduced to a single aggregated
//     [wiop.GrandProduct] query. For each query a fresh β coin (and an α coin
//     when the table is multi-column) is sampled; each A-fragment row becomes
//     a numerator factor (β + RLC_α(row)) and each B-fragment row a
//     denominator factor. Because A and B are permutations of one another the
//     product ∏num / ∏den equals one, which the verifier checks via
//     [CheckResultIsOne].
//
//   - Phase 2 (grand product → Z columns): every unreduced
//     [wiop.GrandProduct] is reduced to a set of running-product extension
//     columns Z, one per (module, packed-factor-group). Each Z column carries:
//
//   - a vanishing recurrence  Z[i]·zDen − Z[i−1]·zNum  (row 0 auto-cancelled
//     by the −1 shift on Z);
//
//   - a local constraint pinning the row-0 boundary  Z[0]·zDen − zNum;
//
//   - an opening of the column endpoint Z[n−1].
//
//     A verifier action ([FinalProductCheck]) then asserts that the product of
//     all endpoint openings equals the query's claimed Result cell.
//
// Up to packingArity numerator (resp. denominator) factors sharing a module
// are packed into a single Z column for efficiency, matching the linea
// compiler. Because each permutation query uses distinct coins, packing
// factors from different queries into one Z is sound.
//
// The constraint system does not enforce non-zero denominators; the β-coin
// randomisation makes every (β + RLC(row)) non-zero with overwhelming
// probability, which is what soundly pins down each Z.
package grandproduct

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// packingArity is the maximum number of factors packed into a single Z column.
// The value matches the linea/permutation compiler.
const packingArity = 3

// Compile runs both grand-product compilation phases on sys: it first reduces
// every permutation [wiop.TableRelationQuery] into an aggregated [wiop.GrandProduct]
// (see [compilePermutations]), then reduces every [wiop.GrandProduct] into
// running-product Z columns (see [compileGrandProducts]). Each phase is a
// no-op when its input queries are absent.
func Compile(sys *wiop.System) {
	compilePermutations(sys)
	compileGrandProducts(sys)
}

// compileGrandProducts reduces every unreduced [wiop.GrandProduct] in sys to a
// Z-column running-product recurrence plus endpoint openings, and registers a
// prover/verifier action pair tying the endpoints to the query's Result cell.
func compileGrandProducts(sys *wiop.System) {
	for _, gp := range sys.GrandProducts {
		if gp.IsReduced() {
			continue
		}
		compileGrandProduct(gp)
		gp.MarkAsReduced()
	}
}

// compileGrandProduct reduces a single GrandProduct query. Z columns live in
// the query's Result round (the round after the latest factor round), so they
// are committed once every factor input is available.
func compileGrandProduct(gp *wiop.GrandProduct) {
	round := gp.Result.Round()
	compCtx := gp.Context().Childf("gp-compile")

	numByMod, modOrder := bucketByModule(gp.Numerators, nil)
	denByMod, modOrder := bucketByModule(gp.Denominators, modOrder)

	var entries []zEntry
	for mi, m := range modOrder {
		nums := numByMod[m]
		dens := denByMod[m]
		numZs := max(
			utils.DivCeil(len(nums), packingArity),
			utils.DivCeil(len(dens), packingArity),
		)
		for k := 0; k < numZs; k++ {
			packedNum := subslice(nums, k*packingArity, (k+1)*packingArity)
			packedDen := subslice(dens, k*packingArity, (k+1)*packingArity)
			entries = append(entries, buildZ(m, packedNum, packedDen, round, compCtx, mi, k))
		}
	}

	// Assign the claimed Result (∏num/∏den) from the factor expressions, and
	// the Z columns from their prefix products. The two run in either order —
	// they are independent — and FinalProductCheck below ties them together.
	// Registering the Result assignment here (rather than in the permutation
	// phase) keeps every GrandProduct self-dischargeable, including those a
	// caller builds directly via NewGrandProduct (e.g. the message-bus pass).
	round.RegisterAction(&assignResultAction{gp: gp})
	round.RegisterAction(&proverAction{entries: entries})
	round.RegisterVerifierAction(&FinalProductCheck{GrandProduct: gp, Entries: entries})
}

// zEntry collects the per-Z artefacts shared by the prover and verifier
// actions: the Z column, its packed numerator/denominator product expressions
// (used by the prover to compute the running product), and the column endpoint
// opening Z[n-1].
type zEntry struct {
	zCol *wiop.Column
	zNum wiop.Expression
	zDen wiop.Expression
	// ZFinal is the (round, slot) coordinate of the Z[n-1] opening; carries no
	// witness value of its own. Exported for the verifier-ray codegen.
	ZFinal *wiop.Cell
}

// buildZ allocates one Z column for a packed (numerator, denominator) factor
// group, registers the running-product recurrence Vanishing, pins the row-0
// boundary with a local constraint, and opens the column endpoint. Mirrors
// [logderivativesum.buildZ] but with a multiplicative recurrence.
func buildZ(
	m *wiop.Module,
	packedNum, packedDen []wiop.Expression,
	round *wiop.Round,
	ctx *wiop.ContextFrame,
	mi, k int,
) zEntry {
	zNum := productOrOne(m, packedNum)
	zDen := productOrOne(m, packedDen)

	zCol := m.NewExtensionColumn(
		ctx.Childf("z-m%d-k%d", mi, k),
		wiop.VisibilityOracle,
		round,
	)

	// Recurrence Z[i]·zDen − Z[i−1]·zNum. The −1 shift on Z places row 0 in
	// NewVanishing's cancelled positions, so the recurrence is vacuous on row 0
	// (pinned separately by the local constraint below) and on a one-row
	// dynamic module. For a statically one-row module it is skipped outright.
	if m.IsDynamic() || m.Size() > 1 {
		zView := zCol.View()
		recurrence := wiop.Sub(
			wiop.Mul(zView, zDen),
			wiop.Mul(zView.Shift(-1), zNum),
		)
		m.NewVanishing(ctx.Childf("z-recurrence-m%d-k%d", mi, k), recurrence)
	}

	// Initial condition at row 0: Z[0]·zDen − zNum = 0, i.e. Z[0] = zNum/zDen.
	m.NewLocalConstraint(
		ctx.Childf("z-init-m%d-k%d", mi, k),
		wiop.Sub(wiop.Mul(zCol.View(), zDen), zNum),
		0,
	)

	// Endpoint opening Z[last] = ∏_row (zNum/zDen) for this packed group.
	endpointPos := -1
	if !m.IsDynamic() {
		endpointPos = m.Size() - 1
	}
	zFinal := zCol.At(endpointPos).Open(ctx.Childf("z-final-m%d-k%d", mi, k))

	return zEntry{zCol: zCol, zNum: zNum, zDen: zDen, ZFinal: zFinal}
}

// bucketByModule groups vector-valued factor expressions by their owning
// module. Modules are appended to the returned order on first appearance so the
// output is deterministic; seedOrder lets a second call extend the module
// ordering established by the first, so numerators and denominators share one
// ordering. byMod[m] is nil for any seed module not referenced by factors.
func bucketByModule(
	factors []wiop.Expression,
	seedOrder []*wiop.Module,
) (map[*wiop.Module][]wiop.Expression, []*wiop.Module) {
	byMod := make(map[*wiop.Module][]wiop.Expression)
	order := append([]*wiop.Module(nil), seedOrder...)
	seen := make(map[*wiop.Module]bool, len(order))
	for _, m := range order {
		seen[m] = true
	}
	for _, e := range factors {
		m := e.Module()
		if !seen[m] {
			seen[m] = true
			order = append(order, m)
		}
		byMod[m] = append(byMod[m], e)
	}
	return byMod, order
}

// productOrOne returns the product expression of factors, or the constant 1
// broadcast over module m when factors is empty (the identity of the empty
// product). The result is always vector-valued on m.
func productOrOne(m *wiop.Module, factors []wiop.Expression) wiop.Expression {
	if len(factors) == 0 {
		return wiop.NewConstantVector(m, field.One())
	}
	return wiop.Product(factors...)
}

// subslice returns factors[start:stop], truncated to the slice bounds and empty
// when start is past the end.
func subslice(factors []wiop.Expression, start, stop int) []wiop.Expression {
	if start >= len(factors) {
		return nil
	}
	if stop > len(factors) {
		stop = len(factors)
	}
	return factors[start:stop]
}

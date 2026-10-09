// Package logderivativesum implements the LogDerivativeSum compiler pass for
// the wiop protocol framework.
//
// Each [wiop.Fraction] carries an optional Filter expression on top of the
// (Numerator, Denominator) pair, and the contribution of a fraction at row i is
//
//	Filter[i] · Numerator[i] / Denominator[i]
//
// (with a nil Filter treated as constant 1). This is the natural target for
// conditional lookups, where rows with Filter[i] = 0 should not contribute to
// the running sum even if Denominator[i] would not be invertible on those
// rows.
//
// The compiler reduces every [wiop.LogDerivativeSum] query into:
//
//   - one or more "running-sum" extension columns Z, each absorbing up to
//     packingArity fractions whose vector-valued sides live on the same module;
//
//   - per Z column, an extension cell C, the average of its fractions over
//     the module's n rows;
//
//   - per Z column, a cyclic vanishing recurrence
//
//     Z[i] − Z[i−1] = v[i] − C   on every row, row 0 reading Z[n−1],
//
//     v[i] the row's sum of fractions;
//
//   - a verifier action that checks Σ n·C over the Z columns matches the
//     query's claimed Result cell.
//
// Summed over the rows, the left side of the recurrence telescopes to zero, so
// it holds exactly when Σ_i v[i] = n·C: the cell C carries the column's total,
// and Z needs no boundary. Compared with a running sum pinned at row 0 and
// opened at row n−1, this saves the row-0 local constraint, whose Lagrange
// selector raises the degree by one, and the endpoint opening. C is assigned
// in the same round as Z, so it is bound to the transcript before the coins
// of the global quotient that discharges the recurrence.
//
// The prover-side computation is filter-aware: rows with a zero filter are
// skipped without inverting the corresponding denominator. This is what
// allows the compiler to be used for conditional lookups where the
// denominator may be ill-defined on filtered-out rows.
//
// The constraint system itself does not enforce non-zero denominators;
// callers should ensure denominators are non-zero on every row (typically by
// binding them to a randomness coin) so that the recurrence uniquely pins
// down Z.
package logderivativesum

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// packingArity is the maximum number of fractions packed into a single Z
// column. The value matches the linea/logderivativesum compiler.
const packingArity = 3

// Compile reduces every [wiop.LogDerivativeSum] query in sys to cyclic Z-column
// recurrences and their average cells, and registers prover/verifier actions
// that tie the resulting artefacts back to the query's Result cell.
// Already-reduced queries are skipped.
func Compile(sys *wiop.System) {
	for _, ld := range sys.LogDerivativeSums {
		if ld.IsReduced() {
			continue
		}
		compileQuery(ld)
		ld.MarkAsReduced()
	}
}

// compileQuery reduces a single LogDerivativeSum query.
func compileQuery(ld *wiop.LogDerivativeSum) {
	resultRound := ld.Result.Round()
	compCtx := ld.Context().Childf("logderiv-compile")

	buckets := bucketByModule(ld.Fractions)

	var entries []ZEntry
	for bIdx, b := range buckets {
		groups := packFractions(b.fractions)
		for kIdx, packed := range groups {
			entries = append(entries,
				buildZ(b.module, packed, resultRound, compCtx, bIdx, kIdx))
		}
	}

	resultRound.RegisterAction(&proverAction{ld: ld, entries: entries})
	resultRound.RegisterVerifierAction(&VerifierAction{LogDerivativeSum: ld, Entries: entries})
}

// fractionBucket groups fractions whose vector-valued side lives on the same
// module. Z columns are committed to that module.
type fractionBucket struct {
	module    *wiop.Module
	fractions []wiop.Fraction
}

// bucketByModule groups fractions by the module that owns their vector-valued
// side. Order of first appearance is preserved so the compilation output is
// deterministic.
func bucketByModule(fractions []wiop.Fraction) []fractionBucket {
	indexByModule := make(map[*wiop.Module]int)
	var buckets []fractionBucket
	for _, f := range fractions {
		m := fractionModule(f)
		i, ok := indexByModule[m]
		if !ok {
			i = len(buckets)
			indexByModule[m] = i
			buckets = append(buckets, fractionBucket{module: m})
		}
		buckets[i].fractions = append(buckets[i].fractions, f)
	}
	return buckets
}

// packFractions splits a list of fractions into groups of at most packingArity.
func packFractions(fractions []wiop.Fraction) [][]wiop.Fraction {
	groups := make([][]wiop.Fraction, 0, utils.DivCeil(len(fractions), packingArity))
	for k := 0; k < len(fractions); k += packingArity {
		end := k + packingArity
		if end > len(fractions) {
			end = len(fractions)
		}
		groups = append(groups, fractions[k:end])
	}
	return groups
}

// fractionModule returns the module that owns the vector-valued side of f.
// The LogDerivativeSum constructor guarantees at least one of Numerator and
// Denominator is vector-valued, so the result is never nil.
func fractionModule(f wiop.Fraction) *wiop.Module {
	if m := f.Numerator.Module(); m != nil {
		return m
	}
	return f.Denominator.Module()
}

// ZEntry collects the per-Z artefacts shared by the prover and verifier
// actions: the Z column, the raw fractions (filter, num, den) used by the
// prover for filter-aware row skipping, and the average cell.
type ZEntry struct {
	zCol   *wiop.Column
	packed []wiop.Fraction // raw fractions used by the prover for filter-aware evaluation
	// Average is the cell C, the average over the module's rows of the sum of
	// the packed fractions: the column's total is n·C, n the module's runtime
	// size. It carries no witness value.
	Average *wiop.Cell
}

// Module returns the module of the Z column, whose runtime size n scales the
// average cell into the column's total.
func (e ZEntry) Module() *wiop.Module { return e.zCol.Module }

// buildZ allocates one Z column and its average cell C for a packed fraction
// group, and registers the cyclic recurrence
//
//	zNum − (Z − Z<<−1 + C)·zDen = 0
//
// on every row: Z's −1 shift wraps row 0 around to row n−1, so no row is
// cancelled and no boundary is needed. The module's size does not need to be
// known at compile time, and a one-row module needs no special case: the
// recurrence then reads C·zDen = zNum, which is what binds C.
func buildZ(
	m *wiop.Module,
	packed []wiop.Fraction,
	round *wiop.Round,
	ctx *wiop.ContextFrame,
	bIdx, kIdx int,
) ZEntry {
	zNum, zDen := buildZExpressions(packed)

	zCol := m.NewExtensionColumn(
		ctx.Childf("z-b%d-k%d", bIdx, kIdx),
		round)
	average := round.NewCell(ctx.Childf("z-average-b%d-k%d", bIdx, kIdx), true)

	zView := zCol.View()
	recurrence := wiop.Sub(
		zNum,
		wiop.Mul(
			wiop.Add(wiop.Sub(zView, zView.Shift(-1)), average),
			zDen,
		),
	)
	// NewVanishingManual with no positions: the recurrence holds on every
	// row, row 0 included (NewVanishing would cancel it because of the −1
	// shift).
	m.NewVanishingManual(
		ctx.Childf("z-recurrence-b%d-k%d", bIdx, kIdx),
		recurrence,
	)

	return ZEntry{
		zCol:    zCol,
		packed:  packed,
		Average: average,
	}
}

// buildZExpressions packs up to packingArity filter-aware fractions into a
// single Numerator/Denominator pair using the cross-product identity
//
//	Σ_j (F_j · N_j) / D_j = (Σ_j F_j · N_j · ∏_{k≠j} D_k) / (∏_k D_k).
//
// A nil Filter is treated as the constant 1, in which case the j-th term
// reduces to N_j · ∏_{k≠j} D_k as in the non-filter compiler.
func buildZExpressions(packed []wiop.Fraction) (zNum, zDen wiop.Expression) {
	zDen = packed[0].Denominator
	for i := 1; i < len(packed); i++ {
		zDen = wiop.Mul(zDen, packed[i].Denominator)
	}

	for j := range packed {
		// effectiveNum_j = Filter_j · Num_j (with nil Filter → Num_j).
		term := packed[j].Numerator
		if packed[j].Filter != nil {
			term = wiop.Mul(packed[j].Filter, term)
		}
		for k := range packed {
			if k != j {
				term = wiop.Mul(term, packed[k].Denominator)
			}
		}
		if zNum == nil {
			zNum = term
		} else {
			zNum = wiop.Add(zNum, term)
		}
	}
	return zNum, zDen
}

// proverAction computes each Z column and its average cell from its packed
// fractions, and writes the aggregated sum into ld.Result.
type proverAction struct {
	ld      *wiop.LogDerivativeSum
	entries []ZEntry
}

// Run implements [wiop.ProverAction].
func (a *proverAction) Run(rt *wiop.Runtime) {
	var total field.Ext

	for _, e := range a.entries {
		n := e.zCol.Module.RuntimeSize(rt)
		z := computeFilteredPrefixSum(rt, e.packed, n)
		colTotal := z[n-1]

		// C = total/n, and Z[i] = Σ_{k≤i} (v[k] − C) = prefix[i] − (i+1)·C,
		// so that Z[n−1] = 0 = Z[−1] closes the cycle.
		var nInv field.Element
		nInv.SetUint64(uint64(n))
		nInv.Inverse(&nInv)
		var c, ic field.Ext
		c.MulByElement(&colTotal, &nInv)
		for i := range z {
			ic.Add(&ic, &c)
			z[i].Sub(&z[i], &ic)
		}

		rt.AssignColumn(e.zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(z)})
		rt.AssignCell(e.Average, field.ElemFromExt(c))

		total.Add(&total, &colTotal)
	}

	if !rt.HasCellAssignment(a.ld.Result) {
		rt.AssignCell(a.ld.Result, field.ElemFromExt(total))
	}
}

// computeFilteredPrefixSum returns the running-sum
//
//	Z[i] = Σ_{k≤i, j} F_j[k] · N_j[k] / D_j[k]
//
// over the rows of a packed fraction group, skipping rows where the
// fraction's filter is zero. Each fraction's denominator is batch-inverted
// once; the inverse is consulted only at active rows so a zero denominator at
// a filtered-out row is benign.
//
// Panics if a fraction's denominator is zero on a row where its filter is
// non-zero, since that input is malformed.
func computeFilteredPrefixSum(rt *wiop.Runtime, packed []wiop.Fraction, n int) []field.Ext {
	type evalFrac struct {
		filter []field.Ext // nil ⇒ filter is the constant 1 on every row
		num    []field.Ext
		den    []field.Ext
		invDen []field.Ext
	}
	fracs := make([]evalFrac, len(packed))
	for j, p := range packed {
		fracs[j].num = wiop.EvaluateAsExtVec(rt, p.Numerator, n)
		fracs[j].den = wiop.EvaluateAsExtVec(rt, p.Denominator, n)
		// BatchInvertExt silently leaves zero entries as zero; safe to call on
		// vectors that contain zeros at filtered-out rows.
		fracs[j].invDen = field.BatchInvertExt(fracs[j].den)
		if p.Filter != nil {
			fracs[j].filter = wiop.EvaluateAsExtVec(rt, p.Filter, n)
		}
	}

	z := make([]field.Ext, n)
	var running, term field.Ext
	for i := 0; i < n; i++ {
		for j := range fracs {
			if fracs[j].filter != nil && fracs[j].filter[i].IsZero() {
				continue
			}
			if fracs[j].den[i].IsZero() {
				panic(fmt.Sprintf(
					"wiop/compilers/logderivativesum: zero denominator at row %d for fraction %d "+
						"with non-zero filter; the filter must mask this row",
					i, j,
				))
			}
			term.Mul(&fracs[j].num[i], &fracs[j].invDen[i])
			if fracs[j].filter != nil {
				term.Mul(&term, &fracs[j].filter[i])
			}
			running.Add(&running, &term)
		}
		z[i] = running
	}
	return z
}

// VerifierAction enforces the identity that ties the Z columns to the query:
// Σ n·C over the Z columns, n the runtime size of the column's module and C
// its average cell, equals the claimed Result cell value. The recurrences,
// which make each n·C the total of its column's fractions, are discharged by
// the global quotient; this action reads only cells and module sizes — never
// the oracle witness columns.
//
// Exported (with exported fields) so out-of-package consumers — notably the
// verifier-ray codegen — can read the average cells, the module sizes and the
// Result cell.
type VerifierAction struct {
	LogDerivativeSum *wiop.LogDerivativeSum
	Entries          []ZEntry
}

// Check implements [wiop.VerifierAction].
func (a *VerifierAction) Check(rt *wiop.Runtime) error {
	var sum field.Ext

	for _, e := range a.Entries {
		var n field.Element
		n.SetUint64(uint64(e.Module().RuntimeSize(rt)))
		c := rt.GetCellValue(e.Average).AsExt()
		c.MulByElement(&c, &n)
		sum.Add(&sum, &c)
	}

	claimed := rt.GetCellValue(a.LogDerivativeSum.Result).AsExt()
	var diff field.Ext
	diff.Sub(&sum, &claimed)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/logderivativesum: final-sum check failed for query %q",
			a.LogDerivativeSum.Context().Path(),
		)
	}
	return nil
}

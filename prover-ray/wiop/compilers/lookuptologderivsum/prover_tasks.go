package lookuptologderivsum

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// mAssignmentTask is the prover-side task that fills the multiplicity column
// M for one [lookupGroup]. It is registered in the same round as M itself
// (the coin round) and runs after the witness columns of B and every A in
// the group have been committed.
//
// The task hashes each active B row and each active A row using an internal
// random extension scalar (independent of the symbolic α used by the
// LogDerivativeSum reduction), then for every active A row increments M at
// the matching B row. If multiple B rows hash to the same value, the
// highest-index row gets the count — matching the linea/logderivativesum
// "preserve the latest occurrence" convention. Filtered-out B rows
// (selector = 0) keep M = 0 by construction.
//
// Hash collisions in the internal hash function would only mis-direct
// multiplicity counts within the prover; they cannot break soundness because
// the verifier's check is the symbolic LogDerivativeSum identity, which is
// secured by the externally-sampled γ and α coins.
type mAssignmentTask struct {
	m         *wiop.Column
	bCols     []*wiop.ColumnView
	bSelector *wiop.ColumnView
	included  []includedSpec
	// prependOneOnAOk records whether the compileGroup decided to prepend a
	// constant 1 to every A side (and therefore the B selector to the B
	// side) as part of the IsFilteredOnIncluding trick. The hashing routine
	// below incorporates the same prepend so A/B hashes match when and only
	// when their effective row values match.
	prependOneOnAOk bool
}

// Run implements [wiop.ProverAction].
func (t *mAssignmentTask) Run(rt wiop.Runtime) {
	n := t.m.Module.RuntimeSize(rt)

	// Hashing scalar — fresh per run, independent of the symbolic α used in
	// the constraint system. Collisions are tolerable: they would yield a
	// proof the verifier rejects, never a false acceptance.
	alpha := field.RandomElementExt()

	// Pre-evaluate every B-side input as a length-n extension vector. We do
	// this once and reuse the values for both the B-row hashing and the
	// active-row check.
	bColExt := evaluateVecsAsExt(rt, t.bCols, n)
	var bSelectorExt []field.Ext
	if t.bSelector != nil {
		bSelectorExt = evaluateColumnViewAsExt(rt, t.bSelector, n)
	}

	// Build the B-side hash map: hash → highest-index row that produced it.
	// For rows with bSelector == 0 we still produce a hash; the prepended
	// "head" is the selector value itself, so a filtered-out B row hashes
	// differently from any A row whose head is 1.
	bMap := make(map[field.Ext]int, n)
	for i := 0; i < n; i++ {
		var head field.Ext
		if t.prependOneOnAOk {
			// B-side head is the selector value (0 for filtered rows).
			head = bSelectorExt[i]
		}
		// (else) head is the zero element — both sides skip the prepend so
		// hashes still align.
		h := rowHash(alpha, head, t.prependOneOnAOk, bColExt, i)
		bMap[h] = i // last-wins; matches linea/logderivativesum's convention.
	}

	// Walk every A-side fragment and increment M[bRow] for every active row.
	mValues := make([]field.Element, n)
	for _, inc := range t.included {
		an := inc.cols[0].Module().RuntimeSize(rt)
		aColExt := evaluateVecsAsExt(rt, inc.cols, an)
		var aSelectorExt []field.Ext
		if inc.selector != nil {
			aSelectorExt = evaluateColumnViewAsExt(rt, inc.selector, an)
		}

		for j := 0; j < an; j++ {
			if inc.selector != nil {
				if aSelectorExt[j].IsZero() {
					continue
				}
				// The included-side filter is treated as a 0/1 selector by the
				// LogDerivativeSum reduction: M is incremented by one per
				// active row, so any other value would silently break the
				// honest-prover identity. Abort early with a clear error
				// instead of letting the verifier reject a malformed proof.
				if !aSelectorExt[j].IsOne() {
					panic(fmt.Sprintf(
						"wiop/compilers/lookuptologderivsum: included filter %q has a non-binary value at row %d: %v",
						inc.selector.Column.Context.Path(), j, aSelectorExt[j].String(),
					))
				}
			}
			var head field.Ext
			if t.prependOneOnAOk {
				// A-side head is the constant 1.
				head = field.OneExt()
			}
			h := rowHash(alpha, head, t.prependOneOnAOk, aColExt, j)
			bRow, ok := bMap[h]
			if !ok {
				panic(fmt.Sprintf(
					"wiop/compilers/lookuptologderivsum: A row %d (fragment %s) has no match in the lookup table",
					j, inc.cols[0].Column.Context.Path(),
				))
			}
			mValues[bRow].Add(&mValues[bRow], &fieldOne)
		}
	}

	rt.AssignColumn(t.m, &wiop.ConcreteVector{Plain: field.VecFromBase(mValues)})
}

// fieldOne is the multiplicative identity in the base field, kept as a
// package-level variable to avoid recomputing it per row.
var fieldOne = field.One()

// rowHash computes the extension-field hash
//
//	head + α·cols[0][i] + α²·cols[1][i] + … + α^k·cols[k-1][i]
//
// when usePrepend is true, and
//
//	cols[0][i] + α·cols[1][i] + α²·cols[2][i] + …
//
// when it is false. Both forms match [rlcExpression]'s convention so the
// prover-side hash agrees with the symbolic RLC under the same scalar.
func rowHash(alpha field.Ext, head field.Ext, usePrepend bool, cols [][]field.Ext, i int) field.Ext {
	if !usePrepend {
		// acc = cols[0][i] + α·cols[1][i] + α²·cols[2][i] + …
		acc := cols[0][i]
		pow := alpha
		for k := 1; k < len(cols); k++ {
			var term field.Ext
			term.Mul(&pow, &cols[k][i])
			acc.Add(&acc, &term)
			if k+1 < len(cols) {
				pow.Mul(&pow, &alpha)
			}
		}
		return acc
	}
	// acc = head + α·cols[0][i] + α²·cols[1][i] + …
	acc := head
	pow := alpha
	for k := 0; k < len(cols); k++ {
		var term field.Ext
		term.Mul(&pow, &cols[k][i])
		acc.Add(&acc, &term)
		if k+1 < len(cols) {
			pow.Mul(&pow, &alpha)
		}
	}
	return acc
}

// evaluateVecsAsExt evaluates each column view as a length-n extension-field
// vector, lifting base-field columns when necessary.
func evaluateVecsAsExt(rt wiop.Runtime, cvs []*wiop.ColumnView, n int) [][]field.Ext {
	out := make([][]field.Ext, len(cvs))
	for i, cv := range cvs {
		out[i] = evaluateColumnViewAsExt(rt, cv, n)
	}
	return out
}

// evaluateColumnViewAsExt is like cv.EvaluateVector but always returns a
// length-n []field.Ext. Base-field columns are lifted; extension columns
// are copied as-is.
func evaluateColumnViewAsExt(rt wiop.Runtime, cv *wiop.ColumnView, n int) []field.Ext {
	cvData := cv.EvaluateVector(rt)
	out := make([]field.Ext, n)
	plain := cvData.Plain
	if plain.IsBase() {
		base := plain.AsBase()
		copyLen := len(base)
		if copyLen > n {
			copyLen = n
		}
		for i := 0; i < copyLen; i++ {
			out[i] = field.Lift(base[i])
		}
		if copyLen < n {
			pad := field.Lift(cvData.Padding)
			for i := copyLen; i < n; i++ {
				out[i] = pad
			}
		}
		return out
	}
	ext := plain.AsExt()
	copyLen := len(ext)
	if copyLen > n {
		copyLen = n
	}
	copy(out[:copyLen], ext[:copyLen])
	if copyLen < n {
		pad := field.Lift(cvData.Padding)
		for i := copyLen; i < n; i++ {
			out[i] = pad
		}
	}
	return out
}

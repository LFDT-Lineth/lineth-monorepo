package grandproduct

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/hugepage"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
)

// RowLimitAction enforces the per-permutation row bound for a single
// permutation query on both sides of the protocol. As a prover action it panics
// (the prover is trusted code about to build an unsound witness); as a verifier
// action it returns an error so the verifier rejects the proof gracefully.
//
// Limit is the effective per-side bound for this query: [wiop.MaxPermutationRows]
// divided by the accumulator budget it shares with the permutations compiled
// alongside it (see compilePermutations).
//
// Exported (with exported fields) so out-of-package consumers — notably the
// verifier-ray codegen — can read the guarded permutation query (and, through
// its A/B sides, the module partitioning) and the limit, the same way it
// already reads [lookuptologderivsum.RowLimitVerifierAction] for lookups.
type RowLimitAction struct {
	Query *wiop.TableRelationQuery
	Limit uint64
}

// Run implements [wiop.ProverAction]: it panics on an over-limit permutation.
func (a *RowLimitAction) Run(rt *wiop.Runtime) {
	a.Query.CheckRowLimit(rt, a.Limit)
}

// Check implements [wiop.VerifierAction]: it returns an error on an over-limit
// permutation so the verifier rejects the proof.
func (a *RowLimitAction) Check(rt *wiop.Runtime) error {
	return a.Query.ValidateRowLimit(rt, a.Limit)
}

// assignResultAction assigns the grand-product Result cell to the value the
// prover computes from the committed factor expressions (one for an honest
// permutation). It is registered by the discharge pass for every GrandProduct,
// so a directly-constructed query (e.g. from the message-bus pass) is assigned
// just like a permutation-derived one.
type assignResultAction struct {
	gp *wiop.GrandProduct
}

// Run implements [wiop.ProverAction].
func (a *assignResultAction) Run(rt *wiop.Runtime) {
	if !a.gp.IsAlreadyAssigned(rt) {
		a.gp.SelfAssign(rt)
	}
}

// proverAction computes each Z column as the running product of its packed
// numerator/denominator factors and assigns it. The endpoint openings resolve
// lazily from these column assignments, so no explicit cell assignment is
// needed here.
type proverAction struct {
	entries []zEntry
}

// Run implements [wiop.ProverAction]. The Z columns are independent, so they
// are computed concurrently, largest first, each with a share of the CPUs
// proportional to its row count.
func (a *proverAction) Run(rt *wiop.Runtime) {
	sizes := make([]int, len(a.entries))
	for i, e := range a.entries {
		sizes[i] = e.zCol.Module.RuntimeSize(rt)
	}
	parallel.ExecuteLargestFirst(sizes, func(i, workers int) {
		e, n := a.entries[i], sizes[i]
		z := computePrefixProduct(rt, e.zNum, e.zDen, n, workers)
		rt.AssignColumn(e.zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(z)})
	})
}

// computePrefixProduct returns the running product
//
//	Z[i] = ∏_{k≤i} zNum[k] / zDen[k]
//
// over the n rows of a packed factor group. Panics on a zero denominator,
// since the β-randomisation is supposed to make every denominator non-zero.
//
// The row terms are computed by one [vecprog] program (one batch inversion
// per block of rows), then [field.VecPrefixProductExt] takes their running
// product. workers bounds the goroutines used.
func computePrefixProduct(rt *wiop.Runtime, zNum, zDen wiop.Expression, n, workers int) []field.Ext {
	l := wiop.NewRowLowering(rt, n)
	b := l.B
	var one field.Ext
	one.SetOne()
	den := l.Lower(zDen)
	b.Check(den, b.Scalar(one, true), func(i int) {
		panic(fmt.Sprintf("wiop/compilers/grandproduct: zero denominator at row %d", i))
	})
	z := make([]field.Ext, n)
	hugepage.Advise(z)
	b.Store(b.Op(vecprog.Div, l.Lower(zNum), den), z)
	b.Compile(n, 1, 1).Run(nil, nil, workers)

	field.VecPrefixProductExt(z, workers)
	return z
}

// FinalProductCheck asserts that the product of all Z endpoint openings equals
// the GrandProduct's claimed Result cell. Each endpoint is bound in-circuit to
// the genuine running product of its committed factors (by the recurrence and
// local constraints registered in buildZ), so this single boundary identity is
// what ties the committed Z columns back to the claimed product.
//
// Exported (with exported fields) so out-of-package consumers — notably the
// verifier-ray codegen — can read the endpoint openings and the Result cell.
type FinalProductCheck struct {
	GrandProduct *wiop.GrandProduct
	Entries      []zEntry
}

// Check implements [wiop.VerifierAction].
func (f *FinalProductCheck) Check(rt *wiop.Runtime) error {
	var prod field.Ext
	prod.SetOne()
	for _, e := range f.Entries {
		zFinal := rt.GetCellValue(e.ZFinal).AsExt()
		prod.Mul(&prod, &zFinal)
	}

	claimed := rt.GetCellValue(f.GrandProduct.Result).AsExt()
	var diff field.Ext
	diff.Sub(&prod, &claimed)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/grandproduct: final-product check failed for query %q",
			f.GrandProduct.Context().Path(),
		)
	}
	return nil
}

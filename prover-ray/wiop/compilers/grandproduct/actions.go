package grandproduct

import (
	"fmt"
	"runtime"
	"sort"

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
	order := make([]int, len(a.entries))
	total := 0
	for i, e := range a.entries {
		sizes[i] = e.zCol.Module.RuntimeSize(rt)
		order[i] = i
		total += sizes[i]
	}
	sort.SliceStable(order, func(i, j int) bool { return sizes[order[i]] > sizes[order[j]] })

	cpus := runtime.GOMAXPROCS(0)
	parallel.ExecuteDynamic(len(order), func(k int) {
		e, n := a.entries[order[k]], sizes[order[k]]
		workers := max(1, cpus*n/max(1, total))
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
// per block of rows), then the running product is a two-pass chunked scan:
// each chunk multiplies locally, then by the product of the chunks before
// it. workers bounds the goroutines used. Field multiplication is exact, so
// the result does not depend on workers.
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

	// First pass: each chunk's running product from one, and its total.
	chunk := max(prefixProductMinChunk, (n+workers-1)/workers)
	nbChunks := (n + chunk - 1) / chunk
	bounds := func(c int) (int, int) { return c * chunk, min((c+1)*chunk, n) }
	chunkTotals := make([]field.Ext, nbChunks)
	parallel.Execute(nbChunks, func(start, end int) {
		for c := start; c < end; c++ {
			lo, hi := bounds(c)
			var running field.Ext
			running.SetOne()
			for i := lo; i < hi; i++ {
				running.Mul(&running, &z[i])
				z[i] = running
			}
			chunkTotals[c] = running
		}
	}, workers)

	// Second pass: multiply every chunk but the first by the product of the
	// chunks before it.
	if nbChunks < 2 {
		return z
	}
	offsets := make([]field.Ext, nbChunks)
	offsets[0].SetOne()
	for c := 1; c < nbChunks; c++ {
		offsets[c].Mul(&offsets[c-1], &chunkTotals[c-1])
	}
	parallel.Execute(nbChunks-1, func(start, end int) {
		for c := start + 1; c < end+1; c++ {
			lo, hi := bounds(c)
			for i := lo; i < hi; i++ {
				z[i].Mul(&z[i], &offsets[c])
			}
		}
	}, workers)
	return z
}

// prefixProductMinChunk is the smallest number of rows a worker of
// [computePrefixProduct] handles: below it, splitting costs more than it
// saves.
const prefixProductMinChunk = 4096

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

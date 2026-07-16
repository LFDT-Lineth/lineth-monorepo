package grandproduct

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

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

// Run implements [wiop.ProverAction].
func (a *proverAction) Run(rt *wiop.Runtime) {
	for _, e := range a.entries {
		n := e.zCol.Module.RuntimeSize(rt)
		z := computePrefixProduct(rt, e.zNum, e.zDen, n)
		rt.AssignColumn(e.zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(z)})
	}
}

// computePrefixProduct returns the running product
//
//	Z[i] = ∏_{k≤i} zNum[k] / zDen[k]
//
// over the n rows of a packed factor group. The denominator is batch-inverted
// once. Panics on a zero denominator, since the β-randomisation is supposed to
// make every denominator non-zero.
func computePrefixProduct(rt *wiop.Runtime, zNum, zDen wiop.Expression, n int) []field.Ext {
	num := evaluateAsExtVec(rt, zNum, n)
	den := evaluateAsExtVec(rt, zDen, n)
	invDen := field.BatchInvertExt(den)

	z := make([]field.Ext, n)
	var running field.Ext
	running.SetOne()
	for i := 0; i < n; i++ {
		if den[i].IsZero() {
			panic(fmt.Sprintf("wiop/compilers/grandproduct: zero denominator at row %d", i))
		}
		var term field.Ext
		term.Mul(&num[i], &invDen[i])
		running.Mul(&running, &term)
		z[i] = running
	}
	return z
}

// evaluateAsExtVec evaluates expr against the runtime and returns a length-n
// extension-field slice. Scalar expressions are broadcast to every position.
func evaluateAsExtVec(rt *wiop.Runtime, expr wiop.Expression, n int) []field.Ext {
	out := make([]field.Ext, n)

	if !expr.IsMultiValued() {
		ext := genToExt(expr.EvaluateSingle(rt).Value)
		for i := range out {
			out[i] = ext
		}
		return out
	}

	cv := expr.EvaluateVector(rt)
	plain := cv.Plain
	if plain.IsBase() {
		base := plain.AsBase()
		copyLen := min(len(base), n)
		for i := 0; i < copyLen; i++ {
			out[i] = field.Lift(base[i])
		}
		pad := field.Lift(cv.Padding)
		for i := copyLen; i < n; i++ {
			out[i] = pad
		}
		return out
	}

	ext := plain.AsExt()
	copyLen := min(len(ext), n)
	copy(out[:copyLen], ext[:copyLen])
	pad := field.Lift(cv.Padding)
	for i := copyLen; i < n; i++ {
		out[i] = pad
	}
	return out
}

// genToExt projects a [field.Gen] onto its extension representation.
func genToExt(v field.Gen) field.Ext {
	if v.IsBase() {
		return field.Lift(v.AsBase())
	}
	return v.AsExt()
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
		zFinal := genToExt(rt.GetCellValue(e.ZFinal))
		prod.Mul(&prod, &zFinal)
	}

	claimed := genToExt(rt.GetCellValue(f.GrandProduct.Result))
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

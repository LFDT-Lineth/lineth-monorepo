package wioptest

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const multiModuleScenarioName = "MultiModule"

// ConstVec returns a ConcreteVector of length n where every element equals val.
//
// A constant column interpolates to a constant polynomial, which makes FRI's
// deep quotient (f(x) - claim)/(x - zeta) vanish identically: it is zero for
// every zeta and fold challenge, so a test that proves and verifies such a
// column through the PCS does not exercise the transcript at all. Prefer a
// non-constant column in tests that reach the PCS opening.
func ConstVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// makeVec returns a ConcreteVector from a varargs list of uint64 values.
func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

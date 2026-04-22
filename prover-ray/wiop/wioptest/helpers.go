package wioptest

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// baseVec returns a ConcreteVector of length n where every element equals val.
func baseVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}}
}

// makeVec returns a ConcreteVector from a varargs list of uint64 values.
func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}}
}

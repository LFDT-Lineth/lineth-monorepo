package query

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
)

/*
Utility function used to manually check permutation and inclusion
constraints. Will return a linear combination of i-th element of
each list.
*/
func rowLinComb(alpha field.Element, i int, list []ifaces.ColAssignment) field.Element {
	var res field.Element
	for j := range list {
		res.Mul(&res, &alpha)
		x := list[j].Get(i)
		res.Add(&res, &x)
	}
	return res
}

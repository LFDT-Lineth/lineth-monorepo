package query

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
)

/*
Utility function used to manually check permutation and inclusion
constraints. Will return a linear combination of i-th element of
each list.
*/
func rowLinComb(alpha fext.Element, i int, list []ifaces.ColAssignment) fext.Element {
	var res fext.Element
	for j := range list {
		res.Mul(&res, &alpha)
		x := list[j].GetExt(i)
		res.Add(&res, &x)
	}
	return res
}

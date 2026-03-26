package fft

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
)

// BitReverseExt applies the bit-reversal permutation to v.
// len(v) must be a power of 2
func BitReverseExt(v []fext.Element) {
	firstCoord := make([]field.Element, len(v))
	secondCoord := make([]field.Element, len(v))
	for i := 0; i < len(v); i++ {
		firstCoord[i].Set(&v[i].A0)
		secondCoord[i].Set(&v[i].A1)
	}
	BitReverse(firstCoord)
	BitReverse(secondCoord)
	for i := 0; i < len(v); i++ {
		v[i].A0.Set(&firstCoord[i])
		v[i].A1.Set(&secondCoord[i])
	}
}

package fext

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// SetUint64 sets z to v and returns z
func (z *Element) SetUint64(v uint64) *Element {
	//  sets z LSB to v (non-Montgomery form) and convert z to Montgomery form
	var x fr.Element
	x.SetUint64(v)
	y := field.Zero()
	z = &Element{x, y}
	return z // z.toMont()
}

// SetInt64 sets z to v and returns z
func (z *Element) SetInt64(v int64) *Element {

	var x fr.Element
	x.SetInt64(v)
	y := field.Zero()
	z = &Element{x, y}
	return z // z.toMont()
}

func (z *Element) Uint64() (uint64, uint64) {
	return z.A0.Bits()[0], z.A1.Bits()[0]
}

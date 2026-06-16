package hash

import (
	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

const (
	WIDTH            = 16
	SpongeWidth      = 24
	SpongeRate       = 16
	NbFullRound      = 6
	NbPartialRounds  = 21
	DigestNbElements = 8
	ExtDegree        = 6
)

// 8 to land in a space big enough to be collision resistant
type Digest [DigestNbElements]koalabear.Element

const StringChunkSize = 3

type FieldHasher interface {
	Reset()
	WriteElements(...koalabear.Element)
	WriteExt(...ext.E6)
	Sum() Digest
}

func NewElement(v uint64) koalabear.Element {
	var e koalabear.Element
	e.SetUint64(v)
	return e
}

func StringToElements(domainTag uint64, s string) []koalabear.Element {
	nbChunks := (len(s) + StringChunkSize - 1) / StringChunkSize
	res := make([]koalabear.Element, 2, 2+nbChunks)
	res[0].SetUint64(domainTag)
	res[1].SetUint64(uint64(len(s)))

	for i := 0; i < len(s); i += StringChunkSize {
		var limb uint64
		for j := 0; j < StringChunkSize && i+j < len(s); j++ {
			limb |= uint64(s[i+j]) << (8 * j)
		}
		var e koalabear.Element
		e.SetUint64(limb)
		res = append(res, e)
	}

	return res
}

func OutputToExt(out Digest) ext.E6 {
	return ElementsToExt(out[0], out[1], out[2], out[3], out[4], out[5])
}

func ElementsToExt(a0, a1, b0, b1, c0, c1 koalabear.Element) ext.E6 {
	var res ext.E6
	res.B0.A0.Set(&a0)
	res.B0.A1.Set(&a1)
	res.B1.A0.Set(&b0)
	res.B1.A1.Set(&b1)
	res.B2.A0.Set(&c0)
	res.B2.A1.Set(&c1)
	return res
}

func LiftBaseToExt(v koalabear.Element) ext.E6 {
	var res ext.E6
	res.B0.A0.Set(&v)
	return res
}

func IsBaseExt(v ext.E6) bool {
	return v.B0.A1.IsZero() && v.B1.IsZero() && v.B2.IsZero()
}

func BaseFromExt(v ext.E6) (koalabear.Element, bool) {
	return v.B0.A0, IsBaseExt(v)
}

func AppendExtElements(dst []koalabear.Element, v ext.E6) []koalabear.Element {
	return append(dst,
		v.B0.A0, v.B0.A1,
		v.B1.A0, v.B1.A1,
		v.B2.A0, v.B2.A1,
	)
}

func ExtToElements(v ext.E6) []koalabear.Element {
	return AppendExtElements(make([]koalabear.Element, 0, ExtDegree), v)
}

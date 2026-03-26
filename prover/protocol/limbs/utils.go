package limbs

import (
	"math/big"
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// FuseRows fuses two rows into a single row
func FuseRows[E Endianness](hi, lo row[E]) row[E] {
	res := make([]byte, 0, hi.NumLimbs()+lo.NumLimbs())
	res = append(res, hi.ToBytes()...)
	res = append(res, lo.ToBytes()...)
	return RowFromBytes[E](res)
}

// bytesToLimbsVec convert a vector of byteslices into a vector of limbs of form
// [E].
func bytesToLimbsVec[E Endianness](bytes [][]byte, numLimbs int) [][]field.Element {

	var (
		numRow = len(bytes)
		res    = make([][]field.Element, numLimbs)
	)

	for c := range res {
		res[c] = make([]field.Element, 0, numRow)
	}

	for r := range bytes {
		lbs := bytesToLimbs[E](bytes[r])
		for c := range res {
			res[c] = append(res[c], lbs[c])
		}
	}

	return res

}

// bigIntToLimbsVec converts a vector of big.Int into limbs of form [E]
func bigIntToLimbsVec[E Endianness](bigints []*big.Int, numLimbs, bitSize int) [][]field.Element {

	var (
		numRow = len(bigints)
		res    = make([][]field.Element, numLimbs)
	)

	for c := range res {
		res[c] = make([]field.Element, 0, numRow)
	}

	for r := range bigints {
		lbs := bigIntToLimbs[E](bigints[r], bitSize)
		for c := range res {
			res[c] = append(res[c], lbs[c])
		}
	}

	return res
}

// limbsBeToBytes converts a vector of limbs in form [E] into bytes
func limbsToBytes[E Endianness](limbs []field.Element) []byte {

	if isLittleEndian[E]() {
		limbs = slices.Clone(limbs)
		slices.Reverse(limbs)
	}

	res := make([]byte, 0, len(limbs)*limbByteWidth)
	for _, c := range limbs {
		cbytes := c.Bytes()
		res = append(res, cbytes[field.Bytes-limbByteWidth:]...)
	}
	return res
}

// bytesToLimbs converts a vector of bytes into limbs of form [E]
func bytesToLimbs[E Endianness](bytes []byte) []field.Element {

	var (
		numLimbs = len(bytes) / limbByteWidth
		limbs    = make([]field.Element, numLimbs)
		buf      = [field.Bytes]byte{}
	)

	for i := 0; i < numLimbs; i++ {
		copy(buf[field.Bytes-limbByteWidth:], bytes[i*limbByteWidth:(i+1)*limbByteWidth])
		if err := limbs[i].SetBytesCanonical(buf[:]); err != nil {
			utils.Panic("bytesToLimbs failed: %v", err)
		}
	}

	if isLittleEndian[E]() {
		slices.Reverse(limbs)
	}

	return limbs
}

// bigIntToLimbs converts a big.Int into limbs of form [E] using bitSize to
// determine the number of required limbs and corresponds to the maximal number
// or bits that can be used by bi.
func bigIntToLimbs[E Endianness](bi *big.Int, bitSize int) []field.Element {
	numLimbs := utils.DivCeil(bitSize, limbBitWidth)
	buf := make([]byte, limbByteWidth*numLimbs)
	bi.FillBytes(buf)
	return bytesToLimbs[E](buf)
}

// limbToBigInt converts a limb of form [E] into a big.Int
func limbToBigInt[E Endianness](limb []field.Element) *big.Int {
	x := limbsToBytes[E](limb)
	return new(big.Int).SetBytes(x)
}

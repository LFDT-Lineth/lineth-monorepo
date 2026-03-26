package importpad

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

const totalLimbBits = generic.TotalLimbSize * 8

func leftAlignLimb(x uint64, nBytes, nCols int) field.Element {

	nBits := 8 * nBytes
	limbBits := totalLimbBits / nCols

	if nBits > limbBits {
		utils.Panic("can't accept limbs larger than limb size %d bits: got %v bits", limbBits, nBits)
	}

	f := new(field.Element).SetUint64(x)
	m := new(big.Int).SetInt64(1)
	m.Lsh(m, uint(limbBits-nBits))
	fm := new(field.Element).SetBigInt(m)
	fm.Mul(f, fm)
	return *fm
}

// leftAlign aligns a uint64 value to the leftmost bits of a field representation,
// splitting it into specified number of columns.
func leftAlign(x uint64, nBytes, totalBytes, nCols int) []field.Element {
	if nBytes > totalBytes {
		utils.Panic("value too wide: %d > %d bytes", nBytes, totalBytes)
	}

	totalBits := 8 * totalBytes
	nBits := 8 * nBytes

	limbBits := totalBits / nCols

	bigX := new(big.Int).SetUint64(x)
	bigX.Lsh(bigX, uint(totalBits-nBits))

	mask := new(big.Int).Lsh(big.NewInt(1), uint(limbBits))
	mask.Sub(mask, big.NewInt(1))

	limbs := make([]field.Element, nCols)
	for i := 0; i < nCols; i++ {
		shift := uint(totalBits - (i+1)*limbBits)

		chunk := new(big.Int).Rsh(bigX, shift)
		chunk.And(chunk, mask)
		limbs[i] = *new(field.Element).SetBigInt(chunk)
	}

	return limbs
}

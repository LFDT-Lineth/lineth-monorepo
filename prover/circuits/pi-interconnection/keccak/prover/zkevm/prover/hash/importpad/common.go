package importpad

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

func leftAlign(x uint64, nBytes int) field.Element {

	if nBytes > 16 {
		utils.Panic("can't accept limbs larger than 16 bytes: got %v bytes", nBytes)
	}

	f := new(field.Element).SetUint64(x)
	m := new(big.Int).SetInt64(1)
	m.Lsh(m, uint(128-nBytes*8))
	fm := new(field.Element).SetBigInt(m)
	fm.Mul(f, fm)
	return *fm
}

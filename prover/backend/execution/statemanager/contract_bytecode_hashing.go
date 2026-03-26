package statemanager

import (
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// HashByteCode returns the hash of the bytecode of an Ethereum contract with
// poseidon.
func HashContractByteCode(contractCode []byte) [32]byte {
	safeByteCode := FormatContractBytecodeForHashing(contractCode)
	return [32]byte(poseidon2_koalabear.HashBytes(safeByteCode))
}

// FormatContractBytecode returns the bytecode of an Ethereum contract with
// poseidon.
func FormatContractBytecodeForHashing(contractCode []byte) []byte {
	var (
		hasOddLen       = len(contractCode)%2 > 0
		paddedSize      = utils.NextMultipleOf(len(contractCode), 2)
		limbCount       = paddedSize / 2
		res             = make([]byte, 2*paddedSize)
		offset          = 0
		numLimbPerBlock = 8
	)

	for limb := 0; limb < limbCount; limb++ {

		src := 2*limb - offset
		dst := 4 * limb
		res[dst] = 0x0
		res[dst+1] = 0x0

		if hasOddLen && limb%numLimbPerBlock == 0 && limbCount-limb <= numLimbPerBlock {
			offset = 1
			res[dst+2] = 0x00
			res[dst+3] = contractCode[src]
			continue
		}

		res[4*limb+2] = contractCode[src]
		res[4*limb+3] = contractCode[src+1]
	}

	return res
}

// UnformatContractBytecodeUnsafe returns the bytecode of an Ethereum contract
// without poseidon. It is used to test the encoding function.
func UnformatContractBytecodeUnsafe(formattedCode []byte) []byte {

	if len(formattedCode)%4 != 0 {
		panic("the contractCode length must be a multiple of 4")
	}

	var (
		res      []byte
		numLimbs = len(formattedCode) / 4
	)

	for i := range numLimbs {
		if i%8 == 0 && numLimbs-i < 8 && formattedCode[4*i+2] == 0x00 {
			res = append(res, formattedCode[4*i+3])
		} else {
			res = append(res, formattedCode[4*i+2], formattedCode[4*i+3])
		}
	}

	return res
}

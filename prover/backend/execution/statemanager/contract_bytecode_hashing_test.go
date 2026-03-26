package statemanager_test

import (
	"strconv"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

func TestFormatBackAndForthBytecode(t *testing.T) {

	contractCodeByte, err := utils.HexDecodeString("0xabcdef123455689abcdefabcdef123455689abcdefabcdef123455689abcdefabcdef123455689abcdefabcdef123455689abcdef123455689abcdefabcdef123455689abcdefabcdef123455689abcdefabcdef123455689abcdef1")
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(contractCodeByte); i++ {
		t.Run("length-"+strconv.Itoa(i), func(t *testing.T) {

			var (
				contractCodeByte     = contractCodeByte[:i]
				formattedCode        = statemanager.FormatContractBytecodeForHashing(contractCodeByte)
				contractCodeByteBack = unformatContractBytecodeUnsafe(formattedCode)
			)

			assert.Equal(
				t,
				utils.HexEncodeToString(contractCodeByte),
				utils.HexEncodeToString(contractCodeByteBack),
			)
		})
	}

}

// unformatContractBytecodeUnsafe returns the bytecode of an Ethereum contract
// without poseidon. It is used to test the encoding function and won't work
// if the contract code contains a zero at the wrong position. The encoding
// is still safe to be used for a hash because we hash explicitly the codesize
// as part of the account.
func unformatContractBytecodeUnsafe(formattedCode []byte) []byte {

	if len(formattedCode)%4 != 0 {
		panic("the contractCode length must be a multiple of 4")
	}

	var (
		res      []byte
		numLimbs = len(formattedCode) / 4
	)

	for i := range numLimbs {
		if i%8 == 0 && numLimbs-i <= 8 && formattedCode[4*i+2] == 0x00 {
			res = append(res, formattedCode[4*i+3])
		} else {
			res = append(res, formattedCode[4*i+2], formattedCode[4*i+3])
		}
	}

	return res
}

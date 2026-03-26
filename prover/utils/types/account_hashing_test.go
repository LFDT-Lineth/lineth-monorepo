package types_test

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

func TestAccountHashing(t *testing.T) {

	testData := []struct {
		Name                   string
		Nonce                  int64
		Balance                *big.Int
		StorageRoot            types.KoalaOctuplet
		Bytecode               []byte
		CodeSize               int64
		ExpectedAccountHashHex string
	}{
		{
			Name:                   "ShomeiOddLengthBytecode",
			Nonce:                  65,
			Balance:                big.NewInt(835),
			StorageRoot:            statemanager.EmptyStorageTrieHash(),
			Bytecode:               utils.HexMustDecodeString("0x495340db00ecc17b5cb435d5731f8d6635e6b3ef42507a8303a068d178a95d22"),
			CodeSize:               0,
			ExpectedAccountHashHex: "0x5b4f38da3b5579846022b90a4a9ca1096653055722e75ebc0d4f68ba22d712c9",
		},
		{
			Name:                   "ShomeiOddLengthBytecode",
			Nonce:                  65,
			Balance:                big.NewInt(835),
			StorageRoot:            statemanager.EmptyStorageTrieHash(),
			Bytecode:               utils.HexMustDecodeString("0x495340db00ecc17b5cb435d5731f8d6635e6b3ef42507a8303a068d178a95d2237"),
			CodeSize:               0,
			ExpectedAccountHashHex: "0x7b25c63e1919b7fb320df01441f1aeea316736903fcabab73138d2c3116e2ffc",
		},
	}

	for _, test := range testData {
		t.Run(test.Name, func(t *testing.T) {

			getLineaCodeHash := func(code []byte) types.KoalaOctuplet {
				hashed := statemanager.HashContractByteCode(code)
				return types.MustBytesToKoalaOctuplet(hashed[:])
			}

			getKeccakCodeHash := func(code []byte) types.FullBytes32 {
				hashed := keccak.Hash(code)
				return types.FullBytes32(hashed)
			}

			account := types.Account{
				Nonce:          test.Nonce,
				Balance:        test.Balance,
				StorageRoot:    test.StorageRoot,
				LineaCodeHash:  getLineaCodeHash(test.Bytecode),
				KeccakCodeHash: getKeccakCodeHash(test.Bytecode),
				CodeSize:       test.CodeSize,
			}

			var (
				accountHash    = poseidon2_koalabear.HashWriterTo(account)
				accountHashHex = utils.HexEncodeToString(accountHash[:])
			)

			assert.Equal(t, test.ExpectedAccountHashHex, accountHashHex)
		})
	}

}

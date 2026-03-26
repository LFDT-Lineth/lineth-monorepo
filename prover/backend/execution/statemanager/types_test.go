package statemanager

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

func TestHashAccount(t *testing.T) {

	testcases := []struct {
		Title   string
		Account types.Account
		Hash    types.KoalaOctuplet
	}{
		{
			Title:   "Empty account",
			Account: types.Account{Balance: big.NewInt(0)},
			Hash:    types.MustHexToKoalabearOctuplet("0x0be39dd910329801041c54896705cb664779584732a232276e59ce2e7ca1b5a7"),
		},
		{
			Title: "EOA account",
			Account: types.Account{
				Nonce:          65,
				Balance:        big.NewInt(5690),
				StorageRoot:    types.MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
				LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
				KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			},
			Hash: types.MustHexToKoalabearOctuplet("0x60cef4a3679fc56e66cba6c72cc9e536600ca999026ef7d22fd7595e62f9fdb8"),
		},
		{
			Title: "Another EOA account",
			Account: types.Account{
				Nonce:          65,
				Balance:        big.NewInt(835),
				StorageRoot:    types.MustHexToKoalabearOctuplet("0x1c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c39"),
				LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x7b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b2"),
				KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			},
			Hash: types.MustHexToKoalabearOctuplet("0x5e6056aa0619c47d3a01df970de314a04daf224d21e8a73b0003d7a4286a7538"),
		},
	}

	for i := range testcases {
		t.Run(testcases[i].Title, func(t *testing.T) {

			h := poseidon2_koalabear.NewMDHasher()
			_, err := testcases[i].Account.WriteTo(h)
			if err != nil {
				t.Fatalf("could not hash the account: %v", err)
			}

			res := types.MustBytesToKoalaOctuplet(h.Sum(nil))
			assert.Equal(t, testcases[i].Hash.Hex(), res.Hex(), "for case %v", i)
		})
	}
}

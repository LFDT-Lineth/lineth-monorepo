package types_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	. "github.com/consensys/linea-monorepo/prover/utils/types"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorsFullBytes32(t *testing.T) {
	fullBytes32Val := FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	expectedEncode := "0x0000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a470"
	fullBytes32Hash := "0x76f5f775402233c90584827a772ac4fa1be76ee2496d5964431708c573e2bd32"

	buf := &bytes.Buffer{}
	fullBytes32Val.WriteTo(buf)
	encoded := utils.HexEncodeToString(buf.Bytes())
	assert.Equal(t, expectedEncode, encoded)

	// Calculate the hash
	hasher := poseidon2_koalabear.NewMDHasher()
	fullBytes32Val.WriteTo(hasher)
	hash := hasher.Sum(nil)

	// Format the hash as a "0x" prefixed hex string
	hashString := fmt.Sprintf("0x%x", hash)

	assert.Equal(t, fullBytes32Hash, hashString)

}

func TestVectorsEthAddress(t *testing.T) {

	tcases := []struct {
		jsonString     string
		hexAddress     string
		ExpectedEncode string
		AddressHash    string
	}{
		{
			jsonString:     `{"address": "0x"}`,
			hexAddress:     "0x0000000000000000000000000000000000000000",
			ExpectedEncode: "00000000000000000000000000000000000000000000000000000000000000000000000000000000",
			AddressHash:    "0x225471e76dca970375f18cc4222539bf1893f40b2c28b7d62119af207704c426",
		},
		{
			jsonString:     `{"address": "0xabcd"}`,
			hexAddress:     "0x000000000000000000000000000000000000abcd",
			ExpectedEncode: "0000000000000000000000000000000000000000000000000000000000000000000000000000abcd",
			AddressHash:    "0x3e455e235bb3ee213d538e4b3696f11e2dd918bd79d502703f9f6c4c0a230b59",
		},
		{
			jsonString:     `{"address": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}`,
			hexAddress:     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			ExpectedEncode: "0000dead0000beef0000dead0000beef0000dead0000beef0000dead0000beef0000dead0000beef",
			AddressHash:    "0x4f465a0b4b9bdb1c3d379bb2200f6340153aaf124c320a3014f53550491b9446",
		},
		{
			jsonString:     `{"address": "0x1234125737deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}`,
			hexAddress:     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			ExpectedEncode: "0000dead0000beef0000dead0000beef0000dead0000beef0000dead0000beef0000dead0000beef",
			AddressHash:    "0x4f465a0b4b9bdb1c3d379bb2200f6340153aaf124c320a3014f53550491b9446",
		},
	}

	for _, c := range tcases {

		var deserialized struct {
			Address EthAddress `json:"address"`
		}

		if err := json.Unmarshal([]byte(c.jsonString), &deserialized); err != nil {
			t.Fatalf("could not deserialize")
		}
		buffer := &bytes.Buffer{}
		deserialized.Address.WriteTo(buffer)

		assert.Equal(t, c.hexAddress, deserialized.Address.Hex())

		encoded := hex.EncodeToString(buffer.Bytes())
		assert.Equal(t, c.ExpectedEncode, encoded)

		// Calculate the hash
		hasher := poseidon2_koalabear.NewMDHasher()
		deserialized.Address.WriteTo(hasher)
		hash := hasher.Sum(nil)

		// Format the hash as a "0x" prefixed hex string
		hashString := fmt.Sprintf("0x%x", hash)

		assert.Equal(t, c.AddressHash, hashString)

	}

}

func TestVectorsAccount(t *testing.T) {

	tcases := []struct {
		Account        Account
		ExpectedEncode string
		AccountHash    string
	}{
		{
			Account: Account{
				Balance: big.NewInt(0),
			},
			ExpectedEncode: "0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			AccountHash:    "0x0be39dd910329801041c54896705cb664779584732a232276e59ce2e7ca1b5a7",
		},
		{
			// EOA
			Account: Account{
				Nonce:          65,
				Balance:        big.NewInt(5690),
				StorageRoot:    MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
				LineaCodeHash:  MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
				KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			},
			ExpectedEncode: "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000410000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000163a0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e730000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a47000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			AccountHash:    "0x60cef4a3679fc56e66cba6c72cc9e536600ca999026ef7d22fd7595e62f9fdb8",
		},
		{
			// Another EOA
			Account: Account{
				Nonce:          65,
				Balance:        big.NewInt(835),
				StorageRoot:    MustHexToKoalabearOctuplet("0x1c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c39"),
				LineaCodeHash:  MustHexToKoalabearOctuplet("0x7b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b2"),
				KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			},
			ExpectedEncode: "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000041000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003431c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c397b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b20000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a47000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			AccountHash:    "0x5e6056aa0619c47d3a01df970de314a04daf224d21e8a73b0003d7a4286a7538",
		},
	}

	// Loop using index 'i' so we can modify the tcases slice
	for _, c := range tcases {
		buf := &bytes.Buffer{}
		_, err := c.Account.WriteTo(buf)

		bufHex := utils.HexEncodeToString(buf.Bytes())
		assert.Equal(t, c.ExpectedEncode, bufHex)
		require.NoError(t, err)

		// Calculate the hash
		hasher := poseidon2_koalabear.NewMDHasher()
		c.Account.WriteTo(hasher)
		hash := hasher.Sum(nil)

		// Format the hash as a "0x" prefixed hex string
		hashString := fmt.Sprintf("0x%x", hash)

		assert.Equal(t, c.AccountHash, hashString)

	}

}

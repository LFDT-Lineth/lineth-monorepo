package public_input

import (
	"fmt"
	"slices"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregation(t *testing.T) {

	testCases := []struct {
		Inputs Aggregation
		Res    string
	}{
		{
			Inputs: Aggregation{
				FinalShnarf:                             "0x3f01b1a726e6317eb05d8fe8b370b1712dc16a7fde51dd38420d9a474401291c",
				ParentAggregationFinalShnarf:            "0x0f20c85d35a21767e81d5d2396169137a3ef03f58391767a17c7016cc82edf2e",
				ParentAggregationLastBlockTimestamp:     1711742796,
				FinalTimestamp:                          1711745271,
				LastFinalizedBlockNumber:                3237969,
				FinalBlockNumber:                        3238794,
				LastFinalizedL1RollingHash:              "0xe578e270cc6ee7164d4348ac7ca9a7cfc0c8c19b94954fc85669e75c1db46178",
				L1RollingHash:                           "0x0578f8009189d67ce0378619313b946f096ca20dde9cad0af12a245500054908",
				LastFinalizedL1RollingHashMessageNumber: 549238,
				L1RollingHashMessageNumber:              549263,
				L2MsgRootHashes:                         []string{"0xfb7ce9c89be905d39bfa2f6ecdf312f127f8984cf313cbea91bca882fca340cd"},
				L2MsgMerkleTreeDepth:                    5,
				// Chain configuration
				ChainID:                59144,
				BaseFee:                7,
				CoinBase:               types.EthAddress(common.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")),
				L2MessageServiceAddr:   types.EthAddress(common.HexToAddress("0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec")),
				LastFinalizedFtxNumber: 0,
				FinalFtxNumber:         0,
			},
			Res: "0x0eeedb87d99ab9ef1850a0e4c3ad717ebb9d7cf6b7e0720dc995f333046eb0af",
		},
	}

	for i := range testCases {
		res := testCases[i].Inputs.GetPublicInputHex()
		require.Equal(t, testCases[i].Res, res)
	}
}

type testExecDataHashCircuit struct {
	NbBytes     frontend.Variable
	Words16Bit  []frontend.Variable
	ExpectedSum frontend.Variable
}

func (c *testExecDataHashCircuit) Define(api frontend.API) error {
	compressor, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		return err
	}
	res, err := checksumExecDataSnark(api, c.Words16Bit, 16, c.NbBytes, compressor)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ExpectedSum, res)
	return nil
}

func TestExecDataHash(t *testing.T) {
	var (
		dataBytes    [64]byte
		dataWordsAll [32]uint16
		dataWords    [32]frontend.Variable
	)

	for i := range dataBytes {
		dataBytes[i] = byte(i)
	}

	for i := range dataWordsAll {
		dataWordsAll[i] = uint16(i*514) + 1 // (256 * 2i) + (2i+1)
		dataWords[i] = 0
	}

	circuit := testExecDataHashCircuit{Words16Bit: make([]frontend.Variable, len(dataWords))}

	for n := 1; n <= len(dataBytes); n++ {
		if n%2 == 0 {
			dataWords[(n-1)/2] = dataWordsAll[(n-1)/2]
		} else {
			dataWords[(n-1)/2] = dataWordsAll[(n-1)/2] & 0xff00
		}

		sum, err := NewExecDataChecksum(dataBytes[:n])
		require.NoError(t, err)

		assignment := testExecDataHashCircuit{
			NbBytes:     n,
			Words16Bit:  slices.Clone(dataWords[:]),
			ExpectedSum: sum.PartialHash[:],
		}

		t.Run(fmt.Sprintf("subslice length %d", n), func(t *testing.T) {
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()), "subslice of length %d", n)
		})
	}
}

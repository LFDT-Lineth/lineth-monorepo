package public_input

import (
	"testing"

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
			},
			Res: "0x26580e4b413ca2c069e358e675d115ba940c0b0448deba809cc19da6ae493d45",
		},
	}

	for i := range testCases {
		res := testCases[i].Inputs.GetPublicInputHex()
		require.Equal(t, testCases[i].Res, res)
	}
}

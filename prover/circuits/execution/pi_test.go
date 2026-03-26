package execution

import (
	"testing"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/stretchr/testify/require"

	"github.com/consensys/gnark/frontend"
	snarkTestUtils "github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func TestPIConsistency(t *testing.T) {

	pi := public_input.Execution{
		L2MessageHashes:              make([][32]byte, 2),
		FinalBlockNumber:             4,
		FinalBlockTimestamp:          5,
		LastRollingHashUpdateNumber:  6,
		InitialBlockNumber:           1,
		InitialBlockTimestamp:        2,
		FirstRollingHashUpdateNumber: 3,
		ChainID:                      7,
		BaseFee:                      3,
	}

	pi.DataChecksum.Length = 8

	utils.FillRange(pi.DataChecksum.PartialHash[:], 9)
	utils.FillRange(pi.DataChecksum.Hash[:], 10)
	utils.FillRange(pi.L2MessageHashes[0][:], 50)
	utils.FillRange(pi.L2MessageHashes[1][:], 90)
	utils.FillRange(pi.InitialStateRootHash[:], 130)
	utils.FillRange(pi.InitialRollingHashUpdate[:], 170)
	utils.FillRange(pi.FinalStateRootHash[:], 210)
	utils.FillRange(pi.LastRollingHashUpdate[:], 250)
	utils.FillRange(pi.CoinBase[:], 20)
	utils.FillRange(pi.L2MessageServiceAddr[:], 40)

	// state root hashes are field elements
	pi.InitialStateRootHash[0] &= 0x0f
	pi.FinalStateRootHash[0] &= 0x0f

	snarkPi := FunctionalPublicInputSnark{
		FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
			L2MessageHashes: L2MessageHashes{Values: make([][32]frontend.Variable, 2)},
		},
	}
	require.NoError(t, snarkPi.Assign(&pi))
	piSum := pi.Sum()

	snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		return []frontend.Variable{snarkPi.Sum(api)}
	}, piSum)(t)
}

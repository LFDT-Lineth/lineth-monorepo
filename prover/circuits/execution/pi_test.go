package execution

import (
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	snarkTestUtils "github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func TestPIConsistency(t *testing.T) {
	pi := public_input.Execution{
		L2MessageHashes:             make([][32]byte, 2),
		FinalBlockNumber:            4,
		FinalBlockTimestamp:         5,
		FinalRollingHashMsgNumber:   6,
		InitialBlockNumber:          1,
		InitialBlockTimestamp:       2,
		InitialRollingHashMsgNumber: 3,
		ChainID:                     7,
	}

	utils.FillRange(pi.DataChecksum[:], 10)
	utils.FillRange(pi.L2MessageHashes[0][:], 50)
	utils.FillRange(pi.L2MessageHashes[1][:], 90)
	utils.FillRange(pi.InitialStateRootHash[:], 130)
	utils.FillRange(pi.InitialRollingHashUpdate[:], 170)
	utils.FillRange(pi.FinalStateRootHash[:], 210)
	utils.FillRange(pi.FinalRollingHashUpdate[:], 250)
	utils.FillRange(pi.L2MessageServiceAddr[:], 40)

	// state root hashes are field elements
	pi.InitialStateRootHash[0] &= 0x0f
	pi.FinalStateRootHash[0] &= 0x0f

	snarkPi := FunctionalPublicInputSnark{
		FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
			L2MessageHashes: L2MessageHashes{Values: make([][32]frontend.Variable, 3)},
		},
	}
	require.NoError(t, snarkPi.Assign(&pi))
	piSum := pi.Sum(nil)

	snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		hsh, err := mimc.NewMiMC(api)
		if err != nil {
			panic(err)
		}
		return []frontend.Variable{snarkPi.Sum(api, &hsh)}
	}, piSum)(t)
}

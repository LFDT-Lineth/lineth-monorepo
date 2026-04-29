package invalidity_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type invalidityHashCircuit struct {
	Inputs invalidity.FunctionalPublicInputsGnark `gnark:",secret"`
	Hash   frontend.Variable                      `gnark:",public"`
}

func (c *invalidityHashCircuit) Define(api frontend.API) error {

	api.AssertIsEqual(c.Hash, c.Inputs.Sum(api))
	return nil
}

func TestInvalidityPublicInputHashConsistency(t *testing.T) {
	gnarkutil.RegisterHintsAndGkrGates()

	var ftxRollingHash [32]byte
	for i := range ftxRollingHash {
		ftxRollingHash[i] = byte(i + 1)
	}

	pi := public_input.Invalidity{
		TxHash:              common.HexToHash("0x11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"),
		TxNumber:            7,
		FromAddress:         linTypes.EthAddress(common.HexToAddress("0x1111111111111111111111111111111111111111")),
		DeadLineBlockNumber: 42,
		StateRootHash:       linTypes.MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
		FtxRollingHash:      ftxRollingHash,
	}

	// Expected hash from native byte-stream implementation.
	var expected fr.Element
	nativeSum := pi.Sum(nil)
	expected.SetBytes(nativeSum)

	var gnarkInputs invalidity.FunctionalPublicInputsGnark
	gnarkInputs.Assign(pi)

	circuit := &invalidityHashCircuit{}
	assignment := &invalidityHashCircuit{
		Inputs: gnarkInputs,
		Hash:   expected,
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
	require.NoError(t, err)

	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	require.NoError(t, ccs.IsSolved(witness))
}

// ftxRollingHashCircuit is a test circuit that computes UpdateFtxRollingHash
// in-circuit and checks it against a native-computed expected value.
type ftxRollingHashCircuit struct {
	Inputs             invalidity.FtxRollingHashInputs `gnark:",secret"`
	PrevFtxRollingHash frontend.Variable               `gnark:",secret"`
	Expected           frontend.Variable               `gnark:",public"`
}

func (c *ftxRollingHashCircuit) Define(api frontend.API) error {
	result := invalidity.UpdateFtxRollingHashGnark(api, c.Inputs)
	api.AssertIsEqual(c.Expected, result)
	return nil
}

func TestUpdateFtxRollingHashConsistency(t *testing.T) {
	gnarkutil.RegisterHintsAndGkrGates()

	// Test inputs
	txHash := common.HexToHash("0xaabbccdd11223344556677889900eeff0011223344556677aabbccddeeff0099")
	fromAddress := linTypes.EthAddress(common.HexToAddress("0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF"))
	expectedBlockHeight := uint64(12345)

	var prevFtxRollingHash [32]byte
	for i := range prevFtxRollingHash {
		prevFtxRollingHash[i] = byte(i*3 + 7)
	}

	nativeResult := invalidity.UpdateFtxRollingHash(prevFtxRollingHash, txHash, expectedBlockHeight, fromAddress)

	var expected fr.Element
	expected.SetBytes(nativeResult[:])

	// Prepare the gnark inputs — all fields must be set for witness generation
	var in invalidity.FtxRollingHashInputs
	in.PrevFtxRollingHash = prevFtxRollingHash[:]
	in.TxHash0 = txHash[:16]
	in.TxHash1 = txHash[16:]
	in.ExpectedBlockHeight = expectedBlockHeight
	in.FromAddress = fromAddress[:]

	circuit := &ftxRollingHashCircuit{}
	assignment := &ftxRollingHashCircuit{
		Inputs:             in,
		PrevFtxRollingHash: prevFtxRollingHash[:],
		Expected:           expected,
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
	require.NoError(t, err)

	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	require.NoError(t, ccs.IsSolved(witness))
}

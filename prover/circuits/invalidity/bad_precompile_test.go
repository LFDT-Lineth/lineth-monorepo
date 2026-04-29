package invalidity_test

import (
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	invalidityPI "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// we dont need a cryptographic random number generator for testing
var rng = rand.New(rand.NewPCG(0, 0)) //nolint:gosec // G404: weak random is fine for tests

var (
	piStateRootHash    = field.RandomOctuplet()
	piTxHashBytes      = [32]byte{0x11, 0x22, 0x33, 0x44}
	piFromAddressBytes = [20]byte{0xAA, 0xBB, 0xCC, 0xDD}
	piCoinBaseBytes    = [20]byte{0x20, 0x20, 0x20, 0x20}
	piBaseFee          = uint64(1 << 62)
	piChainID          = [32]byte{0x00} // for the public input extraction we set it to zero, while it is non-zero in the circuit. test should pass.
)

var fixedInputs = invalidityPI.FixedInputs{
	TxHashLimbs:   invalidity.CreateLimbs32Bytes(piTxHashBytes),
	FromLimbs:     invalidity.CreateLimbs20Bytes(piFromAddressBytes),
	StateRootHash: piStateRootHash,
	CoinBase:      invalidity.CreateLimbs20Bytes(piCoinBaseBytes),
	BaseFee:       invalidity.Create8LimbsFromInt(piBaseFee),
	ChainID:       invalidity.CreateLimbs32Bytes(piChainID),
	ColSize:       16,
}

// testCase defines a test case
type testCase struct {
	name             string
	invalidityType   invalidity.InvalidityType
	hasBadPrecompile bool
	nbL2LogsVal      int
}

var testCases = []testCase{
	{
		name:             "BadPrecompiles",
		invalidityType:   invalidity.BadPrecompile,
		hasBadPrecompile: true,
		nbL2LogsVal:      5,
	},
	{
		name:             "TooManyLogs_BadPrecompiles",
		invalidityType:   invalidity.BadPrecompile,
		hasBadPrecompile: true, // bad precompile
		nbL2LogsVal:      23,   // > MAX_L2_LOGS (16)
	},
	{
		name:             "TooManyLogs",
		invalidityType:   invalidity.TooManyLogs,
		hasBadPrecompile: false,
		nbL2LogsVal:      20, // > MAX_L2_LOGS (16)
	},
}

// TestInvalidityCircuitWithInvalidityPI tests the invalidity circuit
// by mocking the arithmetization columns registered as public inputs
func TestBadPrecompileCircuit(t *testing.T) {
	var (
		comp  *wizard.CompiledIOP
		proof wizard.Proof
	)
	congloVK := [2]field.Octuplet{field.RandomOctuplet(), field.RandomOctuplet()}
	vkMerkleRoot := field.RandomOctuplet()
	limitlessInputs := &invalidity.LimitlessInputs{
		CongloVK:     congloVK,
		VKMerkleRoot: vkMerkleRoot,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// prepare zkevm inputs
			inputs := invalidityPI.Inputs{
				FixedInputs: fixedInputs,
				CaseInputs: invalidityPI.CaseInputs{
					HasBadPrecompile: tc.hasBadPrecompile,
					NumL2Logs:        tc.nbL2LogsVal,
				},
			}

			comp, proof = invalidity.MockZkEvmPI(rng, inputs, limitlessInputs)

			// Verify the wizard proof first
			err := wizard.Verify(comp, proof)
			require.NoError(t, err, "wizard verification failed")

			// Create a transaction with a To address
			txToAddr := common.HexToAddress("0xdeadbeefdeadb")
			tx := ethTypes.NewTx(&ethTypes.DynamicFeeTx{
				ChainID: big.NewInt(59144),
				To:      &txToAddr,
			})

			// Prepare the circuit inputs
			assi := invalidity.AssigningInputs{
				InvalidityType:   tc.invalidityType,
				ZkEvmComp:        comp,
				ZkEvmWizardProof: proof,
				Transaction:      tx,
				FuncInputs: public_input.Invalidity{
					StateRootHash: piStateRootHash,

					TxHash:      common.Hash(piTxHashBytes),
					FromAddress: types.EthAddress(piFromAddressBytes),
					ToAddress:   types.EthAddress(*tx.To()),
					CoinBase:    types.EthAddress(piCoinBaseBytes),
					BaseFee:     piBaseFee,
					ChainID:     uint64(59144), // this is  zero in the extracted public inputs, but the test should pass.
				},
			}

			execCtx := invalidity.ExecutionCtx{
				LimitlessMode: true,
				CongloVK:      congloVK,
				VKMerkleRoot:  vkMerkleRoot,
			}

			// Define the circuit
			circuit := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.BadPrecompileCircuit{
					InvalidityType: tc.invalidityType,
					ExecutionCtx:   execCtx,
				},
			}

			// Allocate the circuit
			circuit.Allocate(invalidity.Config{
				ZkEvmComp: comp,
			})

			// Compile the circuit
			cs, err := frontend.Compile(
				ecc.BLS12_377.ScalarField(),
				scs.NewBuilder,
				&circuit,
			)
			require.NoError(t, err, "circuit compilation failed")

			// Assign the circuit
			assignment := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.BadPrecompileCircuit{
					InvalidityType: tc.invalidityType,
					ExecutionCtx:   execCtx,
				},
			}
			assignment.Assign(assi)

			// Create witness
			witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
			require.NoError(t, err, "witness creation failed")

			// Verify the circuit is satisfied
			err = cs.IsSolved(witness)
			require.NoError(t, err, "circuit is not satisfied")
		})
	}
}

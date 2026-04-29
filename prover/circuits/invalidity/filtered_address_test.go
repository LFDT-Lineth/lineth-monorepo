package invalidity_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// TestFilteredAddressCircuit tests the FilteredAddressCircuit through the full
// CircuitInvalidity wrapper, for both FilteredAddressFrom and FilteredAddressTo
// invalidity types.
func TestFilteredAddressCircuit(t *testing.T) {

	const maxRlpByteSize = 1024

	toAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD50")
	fromAddr := common.HexToAddress("0x00aed6")

	testCases := []struct {
		name           string
		invalidityType invalidity.InvalidityType
		tx             *types.DynamicFeeTx
		fromAddress    common.Address
	}{
		{
			name:           "FilteredAddressFrom",
			invalidityType: invalidity.FilteredAddressFrom,
			tx: &types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     1,
				GasTipCap: big.NewInt(1000000000),
				GasFeeCap: big.NewInt(100000000000),
				Gas:       21000,
				To:        &toAddr,
				Value:     big.NewInt(1000),
			},
			fromAddress: fromAddr,
		},
		{
			name:           "FilteredAddressTo",
			invalidityType: invalidity.FilteredAddressTo,
			tx: &types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     1,
				GasTipCap: big.NewInt(1000000000),
				GasFeeCap: big.NewInt(100000000000),
				Gas:       21000,
				To:        &toAddr,
				Value:     big.NewInt(1000),
			},
			fromAddress: fromAddr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			tx := types.NewTx(tc.tx)
			encodedTx := ethereum.EncodeTxForSigning(tx)

			rlpEncodedTx := make([]byte, len(encodedTx))
			copy(rlpEncodedTx, encodedTx)

			txHash := ethereum.GetTxHash(tx) // txHash := common.Hash(crypto.Keccak256(rlpEncodedTx))
			stateRoot := field.RandomOctuplet()

			assi := invalidity.AssigningInputs{
				Transaction:    tx,
				FromAddress:    tc.fromAddress,
				MaxRlpByteSize: maxRlpByteSize,
				InvalidityType: tc.invalidityType,
				RlpEncodedTx:   rlpEncodedTx,
				FuncInputs: public_input.Invalidity{
					TxHash:                  txHash,
					FromAddress:             linTypes.EthAddress(tc.fromAddress),
					StateRootHash:           linTypes.KoalaOctuplet(stateRoot),
					FromIsFiltered:          tc.invalidityType == invalidity.FilteredAddressFrom,
					ToIsFiltered:            tc.invalidityType == invalidity.FilteredAddressTo,
					ToAddress:               linTypes.EthAddress(*tc.tx.To),
					CoinBase:                linTypes.DummyAddress(32),
					BaseFee:                 1000000000,
					ChainID:                 59144,
					L2MessageServiceAddr:    linTypes.DummyAddress(32),
					SimulatedBlockTimestamp: 1000000000,
					SimulatedBlockNumber:    1000000000,
				},
			}

			// Generate keccak proof
			kcomp, kproof := invalidity.MakeKeccakProofs(tx, maxRlpByteSize, dummy.Compile)
			assi.KeccakCompiledIOP = kcomp
			assi.KeccakProof = kproof

			// Compile the circuit
			circuit := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.FilteredAddressCircuit{},
			}
			circuit.Allocate(invalidity.Config{
				KeccakCompiledIOP: kcomp,
				MaxRlpByteSize:    maxRlpByteSize,
			})

			cs, err := frontend.Compile(
				ecc.BLS12_377.ScalarField(),
				scs.NewBuilder,
				&circuit,
			)
			require.NoError(t, err)

			// Assign and solve
			assignment := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.FilteredAddressCircuit{},
			}
			assignment.Assign(assi)

			witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
			require.NoError(t, err)

			err = cs.IsSolved(witness)
			require.NoError(t, err)
		})
	}
}

// TestFilteredAddressCircuitWithLargeRLP tests filtered address proofs with
// a transaction that triggers the long RLP list encoding (0xf8+ prefix).
func TestFilteredAddressCircuitWithLargeRLP(t *testing.T) {

	const maxRlpByteSize = 1024

	toAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD50")
	fromAddr := common.HexToAddress("0xDEADBEEF00000000000000000000000000000001")

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(59144),
		Nonce:     999,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(100000000000),
		Gas:       500000,
		To:        &toAddr,
		Value:     big.NewInt(1000000000000000000),
		Data:      make([]byte, 50),
	})

	encodedTx := ethereum.EncodeTxForSigning(tx)
	require.GreaterOrEqual(t, encodedTx[1], byte(0xf8), "expected long RLP list encoding")

	rlpEncodedTx := make([]byte, len(encodedTx))
	copy(rlpEncodedTx, encodedTx)

	txHash := ethereum.GetTxHash(tx)
	stateRoot := field.RandomOctuplet()

	assi := invalidity.AssigningInputs{
		Transaction:    tx,
		FromAddress:    fromAddr,
		MaxRlpByteSize: maxRlpByteSize,
		InvalidityType: invalidity.FilteredAddressFrom,
		RlpEncodedTx:   rlpEncodedTx,
		FuncInputs: public_input.Invalidity{
			TxHash:         txHash,
			FromAddress:    linTypes.EthAddress(fromAddr),
			StateRootHash:  linTypes.KoalaOctuplet(stateRoot),
			FromIsFiltered: true,
			ToIsFiltered:   false,
			ToAddress:      linTypes.EthAddress(toAddr),
		},
	}

	kcomp, kproof := invalidity.MakeKeccakProofs(tx, maxRlpByteSize, dummy.Compile)
	assi.KeccakCompiledIOP = kcomp
	assi.KeccakProof = kproof

	circuit := invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.FilteredAddressCircuit{},
	}
	circuit.Allocate(invalidity.Config{
		KeccakCompiledIOP: kcomp,
		MaxRlpByteSize:    maxRlpByteSize,
	})

	cs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		&circuit,
	)
	require.NoError(t, err)

	assignment := invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.FilteredAddressCircuit{},
	}
	assignment.Assign(assi)

	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = cs.IsSolved(witness)
	require.NoError(t, err)
}

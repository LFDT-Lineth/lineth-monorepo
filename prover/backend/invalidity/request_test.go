package invalidity_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	backend "github.com/consensys/linea-monorepo/prover/backend/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountTrieInputs verifies that mock AccountTrieInputs produce
// valid inputs for the BadNonceBalanceCircuit.
func TestAccountTrieInputs(t *testing.T) {
	const maxRlpByteSize = 1024

	// Create test accounts and transactions
	testCases := []struct {
		name           string
		account        types.Account
		tx             ethtypes.DynamicFeeTx
		fromAddress    common.Address
		invalidityType invalidity.InvalidityType
	}{
		{
			name:    "BadBalance - zero balance account",
			account: backend.CreateMockEOAAccount(0, big.NewInt(0)),
			tx: ethtypes.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     1,
				Value:     big.NewInt(1),
				Gas:       21000,
				GasFeeCap: big.NewInt(1000000000),
				To:        &common.Address{},
			},
			fromAddress:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
			invalidityType: invalidity.BadBalance,
		},
		{
			name:    "BadNonce - wrong nonce",
			account: backend.CreateMockEOAAccount(65, big.NewInt(1e18)),
			tx: ethtypes.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     100,
				Value:     big.NewInt(0),
				Gas:       21000,
				GasFeeCap: big.NewInt(1000000000),
				To:        &common.Address{},
			},
			fromAddress:    common.HexToAddress("0xabcdef0123456789abcdef0123456789abcdef01"),
			invalidityType: invalidity.BadNonce,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := backend.CreateMockAccountProof(tc.fromAddress, tc.account)

			require.Equal(t, types.EthAddress(tc.fromAddress), mock.Address)
			require.Equal(t, tc.account.Nonce, mock.Inputs.Account.Nonce)
			require.Equal(t, 0, tc.account.Balance.Cmp(mock.Inputs.Account.Balance))

			tx := ethtypes.NewTx(&tc.tx)
			rlpEncodedTx := ethereum.EncodeTxForSigning(tx)

			// Create AssigningInputs using the extracted AccountTrieInputs
			assi := invalidity.AssigningInputs{
				AccountTrieInputs: mock.Inputs,
				Transaction:       tx,
				FromAddress:       tc.fromAddress,
				MaxRlpByteSize:    maxRlpByteSize,
				InvalidityType:    tc.invalidityType,
				RlpEncodedTx:      rlpEncodedTx,
				FuncInputs: public_input.Invalidity{
					StateRootHash:  invalidity.ComputeTopRoot(mock.Inputs.NextFreeNode, mock.Inputs.SubRoot),
					TxHash:         ethereum.GetTxHash(tx),
					FromAddress:    types.EthAddress(tc.fromAddress),
					FromIsFiltered: false,
					ToIsFiltered:   false,
					ToAddress:      types.EthAddress(*tc.tx.To),
				},
			}

			// Generate keccak proof
			kcomp, kproof := invalidity.MakeKeccakProofs(tx, maxRlpByteSize, dummy.Compile)
			assi.KeccakCompiledIOP = kcomp
			assi.KeccakProof = kproof

			// Allocate circuit
			circuit := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.BadNonceBalanceCircuit{},
			}
			circuit.Allocate(invalidity.Config{
				Depth:             40,
				KeccakCompiledIOP: kcomp,
				MaxRlpByteSize:    maxRlpByteSize,
			})

			// Compile circuit
			ccs, err := frontend.Compile(
				ecc.BLS12_377.ScalarField(),
				scs.NewBuilder,
				&circuit,
			)
			require.NoError(t, err)

			// Assign circuit
			assignment := invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.BadNonceBalanceCircuit{},
			}
			assignment.Assign(assi)

			// Create witness and verify
			witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
			require.NoError(t, err)

			err = ccs.IsSolved(witness)
			require.NoError(t, err, "circuit should be satisfied with mock AccountTrieInputs")
		})
	}
}

// TestAccountTrieInputsNilProof verifies that AccountTrieInputs returns an error
// when the proof is nil or has no valid proof data.
func TestAccountTrieInputsNilProof(t *testing.T) {
	req := backend.Request{
		InvalidityType:     invalidity.BadNonce,
		AccountMerkleProof: nil,
	}

	_, _, _, err := req.AccountTrieInputs()
	require.Error(t, err, "should error on nil AccountMerkleProof")

	req2 := backend.Request{
		InvalidityType:     invalidity.BadNonce,
		AccountMerkleProof: &backend.ShomeiAccountProof{},
	}

	_, _, _, err = req2.AccountTrieInputs()
	require.Error(t, err, "should error on empty proof (no proof/leftProof/rightProof)")
}

func TestExtractBeaconTimestampFromShomei(t *testing.T) {
	// Create a mock trace with beacon-roots storage update
	// Timestamp 1777037095 -> slot 7836 (0x1e9c), value 0x69eb6f27
	timestamp := uint64(1777037095)
	slot := timestamp % backend.BeaconRootsRingBufferSize // 7836

	// Create key = slot (as FullBytes32)
	var key types.FullBytes32
	key[31] = byte(slot & 0xff)
	key[30] = byte((slot >> 8) & 0xff)

	// Create value = timestamp (as FullBytes32)
	var value types.FullBytes32
	value[31] = byte(timestamp & 0xff)
	value[30] = byte((timestamp >> 8) & 0xff)
	value[29] = byte((timestamp >> 16) & 0xff)
	value[28] = byte((timestamp >> 24) & 0xff)

	trace := statemanager.DecodedTrace{
		Location: backend.BeaconRootsAddress,
		Type:     statemanager.UPDATE_TRACE_CODE,
		Underlying: statemanager.UpdateTraceST{
			Key:      key,
			NewValue: value,
		},
	}

	traces := [][]statemanager.DecodedTrace{{trace}}

	extractedTimestamp, found, err := backend.ExtractBeaconTimestampFromShomei(traces)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, timestamp, extractedTimestamp)
}

func TestExtractBeaconTimestampFromShomei_NoBeaconRoots(t *testing.T) {
	// Empty traces
	traces := [][]statemanager.DecodedTrace{}
	_, found, err := backend.ExtractBeaconTimestampFromShomei(traces)
	require.NoError(t, err)
	assert.False(t, found)

	// Trace for different address
	var key, value types.FullBytes32
	trace := statemanager.DecodedTrace{
		Location: "0x1234567890123456789012345678901234567890",
		Type:     statemanager.UPDATE_TRACE_CODE,
		Underlying: statemanager.UpdateTraceST{
			Key:      key,
			NewValue: value,
		},
	}
	traces = [][]statemanager.DecodedTrace{{trace}}
	_, found, err = backend.ExtractBeaconTimestampFromShomei(traces)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestValidateShomeiTimestamp_Match(t *testing.T) {
	timestamp := uint64(1777037095)
	slot := timestamp % backend.BeaconRootsRingBufferSize

	var key types.FullBytes32
	key[31] = byte(slot & 0xff)
	key[30] = byte((slot >> 8) & 0xff)

	var value types.FullBytes32
	value[31] = byte(timestamp & 0xff)
	value[30] = byte((timestamp >> 8) & 0xff)
	value[29] = byte((timestamp >> 16) & 0xff)
	value[28] = byte((timestamp >> 24) & 0xff)

	trace := statemanager.DecodedTrace{
		Location: backend.BeaconRootsAddress,
		Type:     statemanager.UPDATE_TRACE_CODE,
		Underlying: statemanager.UpdateTraceST{
			Key:      key,
			NewValue: value,
		},
	}

	traces := [][]statemanager.DecodedTrace{{trace}}

	err := backend.ValidateShomeiTimestamp(traces, timestamp)
	assert.NoError(t, err)
}

func TestValidateShomeiTimestamp_Mismatch(t *testing.T) {
	shomeiTimestamp := uint64(1777037102) // 7 seconds later
	requestTimestamp := uint64(1777037095)
	slot := shomeiTimestamp % backend.BeaconRootsRingBufferSize

	var key types.FullBytes32
	key[31] = byte(slot & 0xff)
	key[30] = byte((slot >> 8) & 0xff)

	var value types.FullBytes32
	value[31] = byte(shomeiTimestamp & 0xff)
	value[30] = byte((shomeiTimestamp >> 8) & 0xff)
	value[29] = byte((shomeiTimestamp >> 16) & 0xff)
	value[28] = byte((shomeiTimestamp >> 24) & 0xff)

	trace := statemanager.DecodedTrace{
		Location: backend.BeaconRootsAddress,
		Type:     statemanager.UPDATE_TRACE_CODE,
		Underlying: statemanager.UpdateTraceST{
			Key:      key,
			NewValue: value,
		},
	}

	traces := [][]statemanager.DecodedTrace{{trace}}

	err := backend.ValidateShomeiTimestamp(traces, requestTimestamp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "beacon-roots timestamp mismatch")
	assert.Contains(t, err.Error(), "delta=7")
}

package invalidity

import (
	"context"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	keccakDummy "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	invalidityPI "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

var rng = rand.New(rand.NewPCG(0, 0)) //nolint:gosec // G404: weak random is fine for tests

var (
	piStateRootHash    = field.RandomOctuplet()
	piTxHashBytes      = [32]byte{0x11, 0x22, 0x33, 0x44}
	piFromAddressBytes = [20]byte{0xAA, 0xBB, 0xCC, 0xDD}
)

var fixedInputs = invalidityPI.FixedInputs{
	TxHashLimbs: invalidity.CreateLimbs32Bytes(piTxHashBytes),
	FromLimbs:   invalidity.CreateLimbs20Bytes(piFromAddressBytes),
	ColSize:     16,
}

// compileSetupProve compiles the circuit, runs PLONK setup with an unsafe SRS,
// then generates and verifies a proof from the given assignment. To make the setup generation efficient we use small tree-depth, mock zkevm and dummy compilation suite.
func compileSetupProve(t *testing.T, builder circuits.Builder, circuitID circuits.CircuitID, assignments []*invalidity.CircuitInvalidity) {
	t.Helper()

	t.Logf("Compiling circuit %s...", circuitID)
	ccs, err := builder.Compile()
	require.NoError(t, err)
	t.Logf("Compiled: %d constraints", ccs.GetNbConstraints())

	t.Log("Running MakeSetup with unsafe SRS...")
	setup, err := circuits.MakeSetup(
		context.Background(),
		circuitID,
		ccs,
		circuits.NewUnsafeSRSProvider(),
		map[string]any{},
	)
	require.NoError(t, err)

	t.Log("Generating and verifying proof...")
	for _, assignment := range assignments {
		proof, err := circuits.ProveCheck(
			&setup,
			assignment,
			emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
			emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		)
		require.NoError(t, err)
		require.NotNil(t, proof)
		t.Logf("Proof generated and verified for %s", circuitID)
	}
}

// TestProveFilteredAddress generates and verifies a proof for the
// invalidity-filtered-address circuit (FilteredAddressFrom case).
func TestProveFilteredAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping proof generation test in short mode")
	}

	const maxRlpByteSize = 1024
	toAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD50")
	fromAddr := common.HexToAddress("0x00aed6")

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(59144),
		Nonce:     1,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(100000000000),
		Gas:       21000,
		To:        &toAddr,
		Value:     big.NewInt(1000),
	})

	encodedTx := ethereum.EncodeTxForSigning(tx)
	rlpEncodedTx := make([]byte, len(encodedTx))
	copy(rlpEncodedTx, encodedTx)

	txHash := ethereum.GetTxHash(tx)
	stateRoot := field.RandomOctuplet()

	// Keccak compiled IOP for the builder (circuit compilation)
	keccakCompForBuilder := invalidity.MakeKeccakCompiledIOP(maxRlpByteSize, keccakDummy.Compile)

	// Keccak compiled IOP + proof for the assignment (witness generation)
	kcomp, kproof := invalidity.MakeKeccakProofs(tx, maxRlpByteSize, keccakDummy.Compile)

	builder := invalidity.NewBuilder(
		invalidity.Config{
			KeccakCompiledIOP: keccakCompForBuilder,
			MaxRlpByteSize:    maxRlpByteSize,
		},
		&invalidity.FilteredAddressCircuit{},
	)

	assi := invalidity.AssigningInputs{
		Transaction:       tx,
		FromAddress:       fromAddr,
		MaxRlpByteSize:    maxRlpByteSize,
		InvalidityType:    invalidity.FilteredAddressFrom,
		RlpEncodedTx:      rlpEncodedTx,
		KeccakCompiledIOP: kcomp,
		KeccakProof:       kproof,

		FuncInputs: public_input.Invalidity{
			TxHash:         txHash,
			FromAddress:    linTypes.EthAddress(fromAddr),
			StateRootHash:  linTypes.KoalaOctuplet(stateRoot),
			FromIsFiltered: true,
			ToIsFiltered:   false,
			ToAddress:      linTypes.EthAddress(*tx.To()),
		},
	}

	assignment := &invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.FilteredAddressCircuit{},
	}
	assignment.Assign(assi)

	compileSetupProve(t, builder, circuits.InvalidityFilteredAddressCircuitID, []*invalidity.CircuitInvalidity{assignment})
}

// TestProveBadPrecompile generates and verifies a proof for the
// invalidity-precompile-logs circuit (BadPrecompile case).
func TestProveBadPrecompile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping proof generation test in short mode")
	}

	// Use mockZkevmPI (from bad_precompile_test.go) which directly registers
	// limb-based public input columns. This is the same mock that the existing
	// TestBadPrecompileCircuit uses and is proven to produce wizard values
	// consistent with the circuit's checkPublicInputs extraction.
	inputs := invalidityPI.Inputs{
		FixedInputs: fixedInputs,
		CaseInputs: invalidityPI.CaseInputs{
			HasBadPrecompile: true,
			NumL2Logs:        5,
		},
	}

	comp, wizProof := invalidity.MockZkEvmPI(rng, inputs, nil)

	txToAddr := common.HexToAddress("0xdeadbeefdeadb")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: big.NewInt(59144),
		To:      &txToAddr,
	})

	builder := invalidity.NewBuilder(
		invalidity.Config{
			ZkEvmComp: comp,
		},
		&invalidity.BadPrecompileCircuit{},
	)

	assi := invalidity.AssigningInputs{
		InvalidityType:   invalidity.BadPrecompile,
		ZkEvmComp:        comp,
		ZkEvmWizardProof: wizProof,
		Transaction:      tx,
		FuncInputs: public_input.Invalidity{

			TxHash:      common.Hash(piTxHashBytes),
			FromAddress: linTypes.EthAddress(piFromAddressBytes),
			ToAddress:   linTypes.EthAddress(*tx.To()),
		},
	}

	assignment := &invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.BadPrecompileCircuit{
			InvalidityType: invalidity.BadPrecompile,
		},
	}
	assignment.Assign(assi)

	compileSetupProve(t, builder, circuits.InvalidityPrecompileLogsCircuitID, []*invalidity.CircuitInvalidity{assignment})
}
func TestProveNonceBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping proof generation test in short mode")
	}

	const (
		maxRlpByteSize = 1024
		treeDepth      = 10
	)

	toAddr := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	fromAddr := common.HexToAddress("0x00aed6")

	testCases := []struct {
		name           string
		mock           MockAccountProof
		tx             *types.Transaction
		invalidityType invalidity.InvalidityType
	}{
		{
			name: "Existing/BadBalance",
			mock: CreateMockAccountProof(fromAddr, CreateMockEOAAccount(1, big.NewInt(0)), treeDepth),
			tx: types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     1,
				Value:     big.NewInt(1),
				Gas:       1,
				GasFeeCap: big.NewInt(1),
				To:        &toAddr,
			}),
			invalidityType: invalidity.BadBalance,
		},
		{
			name: "Existing/BadNonce",
			mock: CreateMockAccountProof(fromAddr, CreateMockEOAAccount(65, big.NewInt(1e18)), treeDepth),
			tx: types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     100,
				Value:     big.NewInt(0),
				Gas:       1,
				GasFeeCap: big.NewInt(1),
				To:        &toAddr,
			}),
			invalidityType: invalidity.BadNonce,
		},
		{
			name: "NonExisting/BadBalance",
			mock: CreateMockNonExistingAccountProof(fromAddr, treeDepth),
			tx: types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     0,
				Value:     big.NewInt(1),
				Gas:       1,
				GasFeeCap: big.NewInt(1),
				To:        &toAddr,
			}),
			invalidityType: invalidity.BadBalance,
		},
		{
			name: "NonExisting/BadNonce",
			mock: CreateMockNonExistingAccountProof(fromAddr, treeDepth),
			tx: types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(59144),
				Nonce:     5,
				Value:     big.NewInt(0),
				Gas:       1,
				GasFeeCap: big.NewInt(1),
				To:        &toAddr,
			}),
			invalidityType: invalidity.BadNonce,
		},
	}

	var assignments []*invalidity.CircuitInvalidity
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := tc.mock

			assi := invalidity.AssigningInputs{
				AccountTrieInputs: mock.Inputs,
				Transaction:       tc.tx,
				FromAddress:       common.Address(mock.Address),
				MaxRlpByteSize:    maxRlpByteSize,
				InvalidityType:    tc.invalidityType,
				FuncInputs: public_input.Invalidity{
					ToAddress:     linTypes.EthAddress(*tc.tx.To()),
					StateRootHash: linTypes.KoalaOctuplet(mock.Inputs.TopRoot),
					FromAddress:   mock.Address,
				},
			}

			b := ethereum.EncodeTxForSigning(tc.tx)
			assi.RlpEncodedTx = make([]byte, len(b))
			copy(assi.RlpEncodedTx, b)
			assi.FuncInputs.TxHash = common.Hash(crypto.Keccak256(assi.RlpEncodedTx))

			kcomp, kproof := invalidity.MakeKeccakProofs(tc.tx, maxRlpByteSize, keccakDummy.Compile)
			assi.KeccakCompiledIOP = kcomp
			assi.KeccakProof = kproof

			assignment := &invalidity.CircuitInvalidity{
				SubCircuit: &invalidity.BadNonceBalanceCircuit{},
			}
			assignment.Assign(assi)
			assignments = append(assignments, assignment)
		})
	}

	keccakCompForBuilder := invalidity.MakeKeccakCompiledIOP(maxRlpByteSize, keccakDummy.Compile)
	builder := invalidity.NewBuilder(
		invalidity.Config{
			Depth:             treeDepth,
			KeccakCompiledIOP: keccakCompForBuilder,
			MaxRlpByteSize:    maxRlpByteSize,
		},
		&invalidity.BadNonceBalanceCircuit{},
	)
	compileSetupProve(t, builder, circuits.InvalidityNonceBalanceCircuitID, assignments)
}

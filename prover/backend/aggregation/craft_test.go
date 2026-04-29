package aggregation

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/invalidity"
	backendInvalidity "github.com/consensys/linea-monorepo/prover/backend/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	circInvalidity "github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiniTrees(t *testing.T) {

	cases := []struct {
		MsgHashes []string
		Res       []string
	}{
		{
			MsgHashes: []string{
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x1111111111111111111111111111111111111111111111111111111111111111",
				"0x2222222222222222222222222222222222222222222222222222222222222222",
				"0x3333333333333333333333333333333333333333333333333333333333333333",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
			},
			Res: []string{
				"0x97d2505cd0c868c753353628fbb1aacc52bba62ddebac0536256e1e8560d4f27",
			},
		},
		{
			MsgHashes: []string{
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x1111111111111111111111111111111111111111111111111111111111111111",
				"0x2222222222222222222222222222222222222222222222222222222222222222",
				"0x3333333333333333333333333333333333333333333333333333333333333333",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x5555555555555555555555555555555555555555555555555555555555555555",
				"0x6666666666666666666666666666666666666666666666666666666666666666",
			},
			Res: []string{
				"0x52b5853ebe75cdc639ba9ed15de287bb918b9a0aba00b7aba087de5ee5d0528d",
			},
		},
		{
			MsgHashes: []string{
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x1111111111111111111111111111111111111111111111111111111111111111",
				"0x2222222222222222222222222222222222222222222222222222222222222222",
				"0x3333333333333333333333333333333333333333333333333333333333333333",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x5555555555555555555555555555555555555555555555555555555555555555",
				"0x6666666666666666666666666666666666666666666666666666666666666666",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
			},
			Res: []string{
				"0x52b5853ebe75cdc639ba9ed15de287bb918b9a0aba00b7aba087de5ee5d0528d",
			},
		},
		{
			MsgHashes: []string{
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x1111111111111111111111111111111111111111111111111111111111111111",
				"0x2222222222222222222222222222222222222222222222222222222222222222",
				"0x3333333333333333333333333333333333333333333333333333333333333333",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x5555555555555555555555555555555555555555555555555555555555555555",
				"0x6666666666666666666666666666666666666666666666666666666666666666",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
				"0x4444444444444444444444444444444444444444444444444444444444444444",
			},
			Res: []string{
				"0x8b25bcdfa0bc56e9e67d3db3c513aa605c8584ac450fb14d62d46cef0fba6f7d",
				"0xcb876e4686e714c06dd52157412e91a490483e9b43e477984c615e6e5dd44b29",
			},
		},
	}

	for i, testcase := range cases {
		res := PackInMiniTrees(testcase.MsgHashes)
		assert.Equal(t, testcase.Res, res, "for case %v", i)
	}
}

func TestL1OffsetBlocks(t *testing.T) {

	testcases := []struct {
		Inps []bool
		Outs string
	}{
		{
			Inps: []bool{true, true, false, false, false},
			Outs: "0x00010002",
		},
		{
			Inps: []bool{false, true, false, false, true, true},
			Outs: "0x000200050006",
		},
	}

	for i, c := range testcases {
		o := PackOffsets(c.Inps)
		oHex := utils.HexEncodeToString(o)
		assert.Equal(t, c.Outs, oHex, "for case %v", i)
	}

}

func TestCollectFilteredAddresses(t *testing.T) {
	fromAddr := types.EthAddress(common.HexToAddress("0xaaaa"))
	toAddr := types.EthAddress(common.HexToAddress("0xbbbb"))
	otherFrom := types.EthAddress(common.HexToAddress("0xcccc"))

	tests := []struct {
		name     string
		pis      []public_input.Invalidity
		expected []types.EthAddress
	}{
		{
			name:     "no invalidity PIs",
			pis:      nil,
			expected: []types.EthAddress{},
		},
		{
			name: "no filtered addresses",
			pis: []public_input.Invalidity{
				{FromAddress: fromAddr, ToAddress: toAddr},
			},
			expected: []types.EthAddress{},
		},
		{
			name: "FilteredAddressFrom only",
			pis: []public_input.Invalidity{
				{FromAddress: fromAddr, FromIsFiltered: true, ToAddress: toAddr},
			},
			expected: []types.EthAddress{fromAddr},
		},
		{
			name: "FilteredAddressTo only",
			pis: []public_input.Invalidity{
				{FromAddress: fromAddr, ToAddress: toAddr, ToIsFiltered: true},
			},
			expected: []types.EthAddress{toAddr},
		},
		{
			name: "multiple types mixed",
			pis: []public_input.Invalidity{
				{FromAddress: fromAddr, FromIsFiltered: true, ToAddress: toAddr},
				{FromAddress: otherFrom, ToAddress: toAddr, ToIsFiltered: true},
				{FromAddress: fromAddr, ToAddress: toAddr},
			},
			expected: []types.EthAddress{fromAddr, toAddr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectFilteredAddresses(tt.pis)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// makeMockSignedTx creates a deterministic signed transaction for testing.
func makeMockSignedTx(t *testing.T, seed int64) *ethtypes.Transaction {
	t.Helper()
	hash := crypto.Keccak256(big.NewInt(seed).Bytes())
	privKey, err := crypto.ToECDSA(hash)
	require.NoError(t, err)

	toAddr := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeef12345678")
	signer := ethtypes.NewLondonSigner(big.NewInt(51))

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(51),
		Nonce:     uint64(seed),
		GasTipCap: big.NewInt(112121212),
		GasFeeCap: big.NewInt(123543135),
		Gas:       4531112,
		To:        &toAddr,
		Value:     big.NewInt(845315452),
	})

	signedTx, err := ethtypes.SignTx(tx, signer, privKey)
	require.NoError(t, err)
	return signedTx
}

func mockCircuitIDForType(invalidityType circInvalidity.InvalidityType) circuits.MockCircuitID {
	switch invalidityType {
	case circInvalidity.BadNonce, circInvalidity.BadBalance:
		return circuits.MockCircuitIDInvalidityNonceBalance
	case circInvalidity.BadPrecompile, circInvalidity.TooManyLogs:
		return circuits.MockCircuitIDInvalidityPrecompileLogs
	case circInvalidity.FilteredAddressFrom, circInvalidity.FilteredAddressTo:
		return circuits.MockCircuitIDInvalidityFilteredAddress
	default:
		panic("unknown invalidity type")
	}
}

// makeMockInvalidityResponse creates a mock invalidity Response with a real
// dummy PLONK proof, suitable for JSON serialization and consumption by
// collectInvalidityInfo.
func makeMockInvalidityResponse(
	t *testing.T,
	invalidityType circInvalidity.InvalidityType,
	ftxNumber uint64,
	prevRollingHash types.Bls12377Fr,
	signedTx *ethtypes.Transaction,
	srsProvider circuits.SRSProvider,
) backendInvalidity.Response {
	t.Helper()

	rlpBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)
	fromAddress := ethereum.GetFrom(signedTx)

	var stateRoot types.KoalaOctuplet

	req := backendInvalidity.Request{
		RlpEncodedTx:                     "0x" + common.Bytes2Hex(rlpBytes),
		ForcedTransactionNumber:          ftxNumber,
		InvalidityType:                   invalidityType,
		DeadlineBlockHeight:              1000000,
		PrevFtxRollingHash:               prevRollingHash,
		ZkParentStateRootHash:            stateRoot,
		SimulatedExecutionBlockNumber:    100,
		SimulatedExecutionBlockTimestamp: 1700000000,
	}

	funcInput := invalidity.FuncInput(&req, &config.Config{})
	mockCircuitID := mockCircuitIDForType(invalidityType)

	setup, err := dummy.MakeUnsafeSetup(srsProvider, mockCircuitID, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	proofStr := dummy.MakeProof(&setup, funcInput.SumAsField(), mockCircuitID)

	return backendInvalidity.Response{
		Signer:                           fromAddress,
		TxHash:                           utils.HexEncodeToString(funcInput.TxHash[:]),
		RLPEncodedTx:                     req.RlpEncodedTx,
		ForcedTransactionNumber:          req.ForcedTransactionNumber,
		PrevFtxRollingHash:               req.PrevFtxRollingHash,
		DeadlineBlockHeight:              req.DeadlineBlockHeight,
		InvalidityType:                   req.InvalidityType,
		ZkParentStateRootHash:            req.ZkParentStateRootHash,
		SimulatedExecutionBlockNumber:    req.SimulatedExecutionBlockNumber,
		SimulatedExecutionBlockTimestamp: req.SimulatedExecutionBlockTimestamp,
		Proof:                            proofStr,
		VerifyingKeyShaSum:               setup.VerifyingKeyDigest(),
		PublicInput:                      types.Bls12377Fr(funcInput.Sum(nil)),
		FtxRollingHash:                   funcInput.FtxRollingHash,
	}
}

// TestCollectInvalidityInfoMultipleTypes verifies that collectInvalidityInfo
// correctly processes multiple invalidity responses with different circuit types
// (and therefore different verifying keys).
func TestCollectInvalidityInfoMultipleTypes(t *testing.T) {

	srsProvider := circuits.NewUnsafeSRSProvider()

	invalidityTypes := []circInvalidity.InvalidityType{
		circInvalidity.BadNonce,
		circInvalidity.FilteredAddressFrom,
		circInvalidity.FilteredAddressTo,
	}

	// Build mock responses with chained rolling hashes
	var (
		responses       []backendInvalidity.Response
		prevRollingHash types.Bls12377Fr
	)
	for i, invType := range invalidityTypes {
		tx := makeMockSignedTx(t, int64(i+1))
		resp := makeMockInvalidityResponse(t, invType, uint64(i+1), prevRollingHash, tx, srsProvider)
		prevRollingHash = resp.FtxRollingHash
		responses = append(responses, resp)
	}

	// Write responses to temp files
	tmpDir := t.TempDir()
	respDir := filepath.Join(tmpDir, "responses")
	require.NoError(t, os.MkdirAll(respDir, 0o755))

	var filenames []string
	for _, resp := range responses {
		fname := filepath.Join(respDir, filepath.Base(t.Name())+"-"+resp.InvalidityType.String()+".json")
		f, err := os.Create(fname)
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(f).Encode(resp))
		require.NoError(t, f.Close())
		filenames = append(filenames, filepath.Base(fname))
	}

	cfg := &config.Config{
		Invalidity: config.Invalidity{
			WithRequestDir: config.WithRequestDir{
				RequestsRootDir: tmpDir,
			},
		},
	}

	aggReq := &Request{
		InvalidityProofs: filenames,
	}

	cf := &CollectedFields{
		InvalidityPI:      make([]public_input.Invalidity, 0),
		InnerCircuitTypes: make([]pi_interconnection.InnerCircuitType, 0),
	}

	err := cf.collectInvalidityInfo(cfg, aggReq)
	require.NoError(t, err)

	// All 3 responses should be collected
	assert.Len(t, cf.InnerCircuitTypes, 3)
	assert.Len(t, cf.ProofClaims, 3)
	assert.Len(t, cf.InvalidityPI, 3)

	for _, ct := range cf.InnerCircuitTypes {
		assert.Equal(t, pi_interconnection.Invalidity, ct)
	}

	// BadNonce and FilteredAddress use different MockCircuitIDs → different VK shasums
	vkNonce := cf.ProofClaims[0].VerifyingKeyShasum
	vkFilteredFrom := cf.ProofClaims[1].VerifyingKeyShasum
	vkFilteredTo := cf.ProofClaims[2].VerifyingKeyShasum
	assert.NotEqual(t, vkNonce, vkFilteredFrom, "BadNonce and FilteredAddressFrom should have different VK shasums")
	assert.Equal(t, vkFilteredFrom, vkFilteredTo, "FilteredAddressFrom and FilteredAddressTo should share the same VK")

	// FinalFtxNumber and FinalFtxRollingHash should come from the last response
	lastResp := responses[len(responses)-1]
	assert.Equal(t, uint(lastResp.ForcedTransactionNumber), cf.FinalFtxNumber)
	assert.Equal(t, lastResp.FtxRollingHash.Hex(), cf.FinalFtxRollingHash)

	// Verify the invalidity PIs carry the correct filtered-address flags
	assert.False(t, cf.InvalidityPI[0].FromIsFiltered, "BadNonce should not have FromIsFiltered")
	assert.False(t, cf.InvalidityPI[0].ToIsFiltered, "BadNonce should not have ToIsFiltered")
	assert.True(t, cf.InvalidityPI[1].FromIsFiltered, "FilteredAddressFrom should have FromIsFiltered")
	assert.True(t, cf.InvalidityPI[2].ToIsFiltered, "FilteredAddressTo should have ToIsFiltered")

	// Verify that collectFilteredAddresses extracts the right addresses
	filteredAddrs := collectFilteredAddresses(cf.InvalidityPI)
	assert.Len(t, filteredAddrs, 2, "should have 2 filtered addresses (1 from + 1 to)")
	assert.Equal(t, cf.InvalidityPI[1].FromAddress, filteredAddrs[0])
	assert.Equal(t, cf.InvalidityPI[2].ToAddress, filteredAddrs[1])
}

// TestCraftResponseWithInvalidityProofs verifies that CraftResponse correctly
// propagates invalidity fields (filtered addresses, FTX rolling hash, etc.)
// into the aggregation response.
func TestCraftResponseWithInvalidityProofs(t *testing.T) {
	fromAddr := types.EthAddress(common.HexToAddress("0xaaaa"))
	toAddr := types.EthAddress(common.HexToAddress("0xbbbb"))
	otherFrom := types.EthAddress(common.HexToAddress("0xcccc"))

	zeroHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	ftxHash := "0x1111111111111111111111111111111111111111111111111111111111111111"
	parentFtxHash := "0x2222222222222222222222222222222222222222222222222222222222222222"

	cf := &CollectedFields{
		FinalShnarf:                         zeroHash,
		ParentAggregationFinalShnarf:        zeroHash,
		DataParentHash:                      zeroHash,
		ParentStateRootHashContract:         types.FullBytes32{},
		ParentAggregationLastBlockTimestamp: 1000,
		FinalTimestamp:                      2000,
		L1RollingHash:                       zeroHash,
		L2MessagingBlocksOffsets:            "0x",
		FinalFtxRollingHash:                 ftxHash,
		FinalFtxNumber:                      3,
		LastFinalizedFtxRollingHash:         parentFtxHash,
		LastFinalizedFtxNumber:              0,
		LastFinalizedL1RollingHash:          zeroHash,

		// Simulate 3 invalidity PIs: BadNonce, FilteredAddressFrom, FilteredAddressTo
		InvalidityPI: []public_input.Invalidity{
			{FromAddress: fromAddr, ToAddress: toAddr},
			{FromAddress: otherFrom, ToAddress: toAddr, FromIsFiltered: true},
			{FromAddress: fromAddr, ToAddress: toAddr, ToIsFiltered: true},
		},
		IsProoflessJob: true,
	}

	cfg := &config.Config{}
	cfg.Layer2.ChainID = 51
	cfg.Layer2.BaseFee = 7

	resp, err := CraftResponse(cfg, cf)
	require.NoError(t, err)

	assert.Equal(t, ftxHash, resp.FinalFtxRollingHash)
	assert.Equal(t, uint(3), resp.FinalFtxNumber)
	assert.Equal(t, parentFtxHash, resp.ParentAggregationFtxRollingHash)
	assert.Equal(t, uint(0), resp.ParentAggregationFtxNumber)

	require.Len(t, resp.FilteredAddresses, 2)
	assert.Equal(t, otherFrom, resp.FilteredAddresses[0], "first filtered address should be the FromIsFiltered address")
	assert.Equal(t, toAddr, resp.FilteredAddresses[1], "second filtered address should be the ToIsFiltered address")
}

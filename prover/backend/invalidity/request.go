package invalidity

import (
	"fmt"
	"strings"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
)

// Request file for a forced transaction attempted to be included in the current aggregation.
// The forcedTransactionNumbers from request files per aggregation should create a consecutive sequence.
type Request struct {
	// RLP encoding of the forced transaction (hex encoded with 0x prefix).
	RlpEncodedTx string `json:"ftxRLP"`

	// Transaction number assigned by L1 contract (decimal encoding)
	ForcedTransactionNumber uint64 `json:"ftxNumber"`

	// Previous FTX rolling hash, i.e. the FTX stream hash of the previous forced transaction.
	PrevFtxRollingHash types.Bls12377Fr `json:"prevFtxRollingHash"`

	// The block number deadline before which one expects to see the transaction (decimal encoding)
	DeadlineBlockHeight uint64 `json:"ftxBlockNumberDeadline"`

	// The type of invalidity for the forced transaction.
	// Valid values: BadNonce, BadBalance, BadPrecompile, TooManyLogs, FilteredAddressFrom, FilteredAddressTo
	InvalidityType invalidity.InvalidityType `json:"invalidityType"`

	// ZK parent state root hash
	ZkParentStateRootHash types.KoalaOctuplet `json:"zkParentStateRootHash"`

	// Path to conflated execution traces file (required for BadPrecompile, TooManyLogs cases)
	ConflatedExecutionTracesFile string `json:"conflatedExecutionTracesFile,omitempty"`

	// Account merkle proof from Shomei linea_getProof API (with proofRelatedNodes).
	// Required for BadNonce, BadBalance cases.
	AccountMerkleProof *ShomeiAccountProof `json:"accountMerkleProof,omitempty"`

	// ZK state merkle proof (full Shomei trace)
	// Required for BadPrecompile, TooManyLogs cases
	ZkStateMerkleProof [][]statemanager.DecodedTrace `json:"zkStateMerkleProof,omitempty"`
	// case of FilteredAddressFrom/FilteredAddressTo: accountMerkleProof=null, zkStateMerkleProof=null

	// Simulated execution block number (ParentAggregationLastBlockNumber + 1)
	SimulatedExecutionBlockNumber uint64 `json:"simulatedExecutionBlockNumber,omitempty"`

	// Simulated execution block timestamp
	SimulatedExecutionBlockTimestamp uint64 `json:"simulatedExecutionBlockTimestamp,omitempty"`
}

// AccountTrieInputs decodes AccountMerkleProof into circuit assignment inputs.
func (req *Request) AccountTrieInputs() (invalidity.AccountTrieInputs, types.EthAddress, types.KoalaOctuplet, error) {
	if req.AccountMerkleProof == nil {
		return invalidity.AccountTrieInputs{}, types.EthAddress{}, types.KoalaOctuplet{}, fmt.Errorf("accountMerkleProof is nil")
	}
	return DecodeAccountTrieInputs(*req.AccountMerkleProof)
}

// Validate checks that the required fields are present based on the InvalidityType.
// Both partial and full modes require the same trace inputs.
func (req *Request) Validate(proverMode config.ProverMode) error {

	logrus.Infof("Validating invalidity request")

	if req.ZkParentStateRootHash == (types.KoalaOctuplet{}) {
		return fmt.Errorf("zkParentStateRootHash is required")
	}

	if req.SimulatedExecutionBlockNumber == 0 {
		return fmt.Errorf("simulatedExecutionBlockNumber is required")
	}
	if req.SimulatedExecutionBlockTimestamp == 0 {
		return fmt.Errorf("simulatedExecutionBlockTimestamp is required")
	}

	switch req.InvalidityType {

	case invalidity.BadNonce, invalidity.BadBalance:
		if req.AccountMerkleProof != nil || proverMode != config.ProverModeDev {
			if err := req.validateAccountMerkleProof(); err != nil {
				return err
			}
		}

	case invalidity.BadPrecompile, invalidity.TooManyLogs:
		if proverMode != config.ProverModeDev {
			if req.ConflatedExecutionTracesFile == "" {
				return fmt.Errorf("conflatedExecutionTracesFile is required for %s invalidity type in %s mode", req.InvalidityType, proverMode)
			}
			if req.ZkStateMerkleProof == nil {
				return fmt.Errorf("zkStateMerkleProof is required for %s invalidity type in %s mode", req.InvalidityType, proverMode)
			}
		}
		if req.ZkStateMerkleProof != nil {
			// Validate that beacon-roots timestamp in Shomei matches the request's simulatedExecutionBlockTimestamp, for EIP-4788
			if err := ValidateShomeiTimestamp(req.ZkStateMerkleProof, req.SimulatedExecutionBlockTimestamp); err != nil {
				return err
			}
		}
	case invalidity.FilteredAddressFrom, invalidity.FilteredAddressTo:
		// No additional fields required.

	default:
		return fmt.Errorf("unknown invalidity type: %s", req.InvalidityType)
	}

	return nil
}

// validateAccountMerkleProof performs comprehensive sanity checks on the
// decoded accountMerkleProof, covering both existing and non-existing accounts.
func (req *Request) validateAccountMerkleProof() error {
	inputs, addr, topRoot, err := DecodeAccountTrieInputs(*req.AccountMerkleProof)
	if err != nil {
		return fmt.Errorf(`accountMerkleProof decode error, accountMerkleProof should be of the form {"key": <address>, "leafIndex": <leafIndex>, "proof": <proof>} for the existing account, or {"key": <address>, "leftProof": <leftProof>, "rightProof": <rightProof>, "leftLeafIndex": <leftLeafIndex>, "rightLeafIndex": <rightLeafIndex>} for the non-existing account: %w`, err)
	}

	// topRoot must match the request's zkParentStateRootHash
	if topRoot != req.ZkParentStateRootHash {
		return fmt.Errorf("topRoot mismatch: topRoot from accountMerkleProof=%s, zkParentStateRootHash from request=%s",
			topRoot.Hex(), req.ZkParentStateRootHash.Hex())
	}

	hKey := HashAddress(addr)

	if inputs.AccountExists {
		return validateExistingAccount(inputs, hKey)
	}
	return validateNonExistingAccount(inputs, hKey)
}

func validateExistingAccount(inputs invalidity.AccountTrieInputs, hKey types.KoalaOctuplet) error {
	lo := inputs.ProofMinus

	if len(lo.Proof.Siblings) != smt_koalabear.DefaultDepth {
		return fmt.Errorf("proof siblings depth: expected %d, got %d",
			smt_koalabear.DefaultDepth, len(lo.Proof.Siblings))
	}

	if field.Octuplet(lo.LeafOpening.Hash()) != lo.Leaf {
		return fmt.Errorf("leaf hash mismatch: Hash(LeafOpening) != Leaf")
	}

	if lo.LeafOpening.HKey != hKey {
		return fmt.Errorf("hKey mismatch: leaf hKey=%s, Hash(address)=%s",
			lo.LeafOpening.HKey.Hex(), hKey.Hex())
	}

	recovered, err := smt_koalabear.RecoverRoot(&lo.Proof, lo.Leaf)
	if err != nil {
		return fmt.Errorf("recovering root from proof: %w", err)
	}
	if recovered != field.Octuplet(inputs.SubRoot) {
		return fmt.Errorf("recovered root mismatch: recovered=%v, subRoot=%v", recovered, inputs.SubRoot)
	}

	return nil
}

func validateNonExistingAccount(inputs invalidity.AccountTrieInputs, hKey types.KoalaOctuplet) error {
	minus := inputs.ProofMinus
	plus := inputs.ProofPlus

	// Both proofs must have correct depth
	if len(minus.Proof.Siblings) != smt_koalabear.DefaultDepth {
		return fmt.Errorf("minus proof siblings depth: expected %d, got %d",
			smt_koalabear.DefaultDepth, len(minus.Proof.Siblings))
	}
	if len(plus.Proof.Siblings) != smt_koalabear.DefaultDepth {
		return fmt.Errorf("plus proof siblings depth: expected %d, got %d",
			smt_koalabear.DefaultDepth, len(plus.Proof.Siblings))
	}

	// Leaf must equal Hash(LeafOpening) for both neighbors
	if field.Octuplet(minus.LeafOpening.Hash()) != minus.Leaf {
		return fmt.Errorf("minus leaf hash mismatch: Hash(LeafOpening) != Leaf")
	}
	if field.Octuplet(plus.LeafOpening.Hash()) != plus.Leaf {
		return fmt.Errorf("plus leaf hash mismatch: Hash(LeafOpening) != Leaf")
	}

	// Both proofs must recover the same subRoot
	recoveredMinus, err := smt_koalabear.RecoverRoot(&minus.Proof, minus.Leaf)
	if err != nil {
		return fmt.Errorf("recovering root from minus proof: %w", err)
	}
	if recoveredMinus != field.Octuplet(inputs.SubRoot) {
		return fmt.Errorf("minus recovered root mismatch: recovered=%v, subRoot=%v", recoveredMinus, inputs.SubRoot)
	}

	recoveredPlus, err := smt_koalabear.RecoverRoot(&plus.Proof, plus.Leaf)
	if err != nil {
		return fmt.Errorf("recovering root from plus proof: %w", err)
	}
	if recoveredPlus != field.Octuplet(inputs.SubRoot) {
		return fmt.Errorf("plus recovered root mismatch: recovered=%v, subRoot=%v", recoveredPlus, inputs.SubRoot)
	}

	// Hash(address) must be between hKey(minus) and hKey(plus)
	if minus.LeafOpening.HKey.Cmp(hKey) >= 0 {
		return fmt.Errorf("non-existing account: hKey(minus)=%s >= Hash(address)=%s",
			minus.LeafOpening.HKey.Hex(), hKey.Hex())
	}
	if hKey.Cmp(plus.LeafOpening.HKey) >= 0 {
		return fmt.Errorf("non-existing account: Hash(address)=%s >= hKey(plus)=%s",
			hKey.Hex(), plus.LeafOpening.HKey.Hex())
	}

	return nil
}

// BeaconRootsAddress is the EIP-4788 beacon roots contract address.
// This contract stores block timestamps in a ring buffer at slot = timestamp mod 8191.
const BeaconRootsAddress = "0x000f3df6d732807ef1319fb7b8bb8522d0beac02"

// BeaconRootsRingBufferSize is the ring buffer size for EIP-4788 (8191).
const BeaconRootsRingBufferSize = 8191

// ExtractBeaconTimestampFromShomei scans zkStateMerkleProof for storage operations
// on the beacon-roots contract and extracts the implied block timestamp.
// Returns (timestamp, found, error). If no beacon-roots entries are found, found=false.
func ExtractBeaconTimestampFromShomei(traces [][]statemanager.DecodedTrace) (uint64, bool, error) {
	beaconAddr := strings.ToLower(BeaconRootsAddress)

	for _, blockTraces := range traces {
		for _, trace := range blockTraces {
			// Skip world-state traces (location == "0x")
			if trace.Location == "0x" {
				continue
			}

			// Check if this is a storage trace for the beacon-roots contract
			location := strings.ToLower(trace.Location)
			if location != beaconAddr {
				continue
			}

			// Extract key and value based on trace type
			key, value, ok := extractStorageKeyValue(trace)
			if !ok {
				continue
			}

			// The timestamp slot has key = timestamp mod 8191
			// and value = timestamp itself
			// Skip the beacon root slot (key = timestamp mod 8191 + 8191)
			keyInt, _ := key.Uint64BigEndian()
			if keyInt >= BeaconRootsRingBufferSize {
				// This is the beacon root slot, not the timestamp slot
				continue
			}

			valueInt, ok := value.Uint64BigEndian()
			if !ok || valueInt == 0 {
				continue
			}

			// Verify: value mod 8191 should equal key
			if valueInt%BeaconRootsRingBufferSize != keyInt {
				// Not a valid timestamp entry
				continue
			}

			return valueInt, true, nil
		}
	}

	return 0, false, nil
}

// extractStorageKeyValue extracts the storage key and new value from a decoded trace.
// Returns (key, newValue, ok). Only handles storage trie traces (not world-state).
func extractStorageKeyValue(trace statemanager.DecodedTrace) (types.FullBytes32, types.FullBytes32, bool) {
	switch t := trace.Underlying.(type) {
	case statemanager.ReadNonZeroTraceST:
		return t.Key, t.Value, true
	case statemanager.UpdateTraceST:
		return t.Key, t.NewValue, true
	case statemanager.InsertionTraceST:
		return t.Key, t.Val, true
	default:
		return types.FullBytes32{}, types.FullBytes32{}, false
	}
}

// ValidateShomeiTimestamp checks that the beacon-roots timestamp in zkStateMerkleProof
// matches the simulatedExecutionBlockTimestamp in the request.
func ValidateShomeiTimestamp(traces [][]statemanager.DecodedTrace, expectedTimestamp uint64) error {
	shomeiTimestamp, found, err := ExtractBeaconTimestampFromShomei(traces)
	if err != nil {
		return fmt.Errorf("failed to extract beacon timestamp from zkStateMerkleProof: %w", err)
	}

	if !found {
		// No beacon-roots entries found; this might be okay for some scenarios
		return nil
	}

	if shomeiTimestamp != expectedTimestamp {
		delta := int64(shomeiTimestamp) - int64(expectedTimestamp)
		shomeiKey := shomeiTimestamp % BeaconRootsRingBufferSize
		expectedKey := expectedTimestamp % BeaconRootsRingBufferSize
		return fmt.Errorf(
			"beacon-roots timestamp mismatch: zkStateMerkleProof implies timestamp %d (slot 0x%x), "+
				"but simulatedExecutionBlockTimestamp is %d (slot 0x%x); delta=%d seconds. "+
				"The Shomei proof was likely generated for a different block than the conflation",
			shomeiTimestamp, shomeiKey,
			expectedTimestamp, expectedKey,
			delta,
		)
	}

	return nil
}

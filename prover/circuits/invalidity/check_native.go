package invalidity

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	publicInput "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	invalidityPI "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
	"github.com/ethereum/go-ethereum/crypto"
)

// CheckOnlyNativeBadPrecompile performs native verification of the outer
// constraints for BadPrecompile / TooManyLogs invalidity types.
//
// The inner wizard constraints are already validated by the dummy compiler
// during ProveInner/VerifyInner. This checks:
//   - PI values extracted from the wizard proof match funcInputs
//   - hasBadPrecompile != 0 (type 2) or NbL2Logs > MAX_L2_LOGS (type 3)
//   - The public input hash is consistent
func CheckOnlyNativeBadPrecompile(
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
	funcInputs public_input.Invalidity,
	invalidityType InvalidityType,
) error {
	invExtractor, err := getInvalidityExtractor(comp)
	if err != nil {
		return err
	}
	execExtractor, err := getExecutionExtractor(comp)
	if err != nil {
		return err
	}

	getPI := func(pi wizard.PublicInput) field.Element {
		return proof.GetPublicInput(comp, pi.Name, true).Base
	}

	if err := checkWizardPI(invExtractor, execExtractor, getPI, funcInputs); err != nil {
		return fmt.Errorf("wizard PI cross-check failed: %w", err)
	}

	hasBadPrecompile := getPI(invExtractor.HasBadPrecompile)
	nbL2Logs := getPI(invExtractor.NbL2Logs)

	switch invalidityType {
	case BadPrecompile:
		if hasBadPrecompile.IsZero() {
			return fmt.Errorf("hasBadPrecompile is zero for BadPrecompile invalidity type")
		}
	case TooManyLogs:
		var threshold field.Element
		threshold.SetUint64(MAX_L2_LOGS + 1)
		if nbL2Logs.Cmp(&threshold) < 0 {
			return fmt.Errorf("nbL2Logs=%v is not > %d for TooManyLogs invalidity type", nbL2Logs, MAX_L2_LOGS)
		}
	default:
		return fmt.Errorf("unsupported invalidity type for bad-precompile native check: %s", invalidityType)
	}

	return nil
}

// CheckOnlyNativeNonceBalance performs native verification of the outer
// constraints for BadNonce / BadBalance invalidity types.
//
// The keccak wizard constraints are already validated by the dummy compiler.
// This checks:
//   - keccak256(rlpEncodedTx) == funcInputs.TxHash
//   - RLP nonce and cost extraction match the transaction fields
//   - Account trie membership (or non-membership) via native Merkle proof
//   - Address hash: Poseidon2(fromAddress) == TargetHKey
//   - Nonce / balance invalidity assertion
//   - PI hash consistency
func CheckOnlyNativeNonceBalance(
	assi AssigningInputs,
) error {
	tx := assi.Transaction
	rlp := assi.RlpEncodedTx

	// --- Keccak hash check ---
	computedHash := crypto.Keccak256(rlp)
	expectedHash := assi.FuncInputs.TxHash
	if !bytes.Equal(computedHash, expectedHash[:]) {
		return fmt.Errorf("keccak hash mismatch: computed=%x, expected=%x", computedHash, expectedHash[:])
	}

	// --- RLP nonce consistency ---
	extractedNonce, err := ExtractNonceFromRLP(rlp)
	if err != nil {
		return fmt.Errorf("could not extract nonce from RLP: %w", err)
	}
	if extractedNonce != tx.Nonce() {
		return fmt.Errorf("RLP nonce mismatch: extracted=%d, tx=%d", extractedNonce, tx.Nonce())
	}

	// --- RLP cost consistency ---
	extractedCost, err := ExtractTxCostFromRLP(rlp)
	if err != nil {
		return fmt.Errorf("could not extract tx cost from RLP: %w", err)
	}
	txCost := tx.Cost()
	if txCost.Cmp(new(big.Int).SetUint64(extractedCost)) != 0 {
		return fmt.Errorf("RLP tx cost mismatch: extracted=%d, tx=%s", extractedCost, txCost)
	}

	// --- Account trie membership proof ---
	ati := assi.AccountTrieInputs
	if err := verifyAccountTrie(ati, types.EthAddress(assi.FromAddress)); err != nil {
		return fmt.Errorf("account trie verification failed: %w", err)
	}

	// --- Nonce / Balance invalidity assertion ---
	accountNonce := ati.Account.Nonce
	accountBalance := ati.Account.Balance
	if accountBalance == nil {
		accountBalance = new(big.Int)
	}

	switch assi.InvalidityType {
	case BadNonce:
		if tx.Nonce() == uint64(accountNonce+1) {
			return fmt.Errorf("nonce is valid (tx=%d, account+1=%d): not a bad-nonce case", tx.Nonce(), accountNonce+1)
		}
	case BadBalance:
		if txCost.Cmp(accountBalance) <= 0 {
			return fmt.Errorf("balance is sufficient (cost=%s, balance=%s): not a bad-balance case", txCost, accountBalance)
		}
	default:
		return fmt.Errorf("unsupported invalidity type for nonce/balance native check: %s", assi.InvalidityType)
	}

	if !ati.AccountExists {
		if accountBalance.Sign() != 0 {
			return fmt.Errorf("non-existing account has non-zero balance: %s", accountBalance)
		}
		if accountNonce != 0 {
			return fmt.Errorf("non-existing account has non-zero nonce: %d", accountNonce)
		}
	}

	return nil
}

// CheckOnlyNativeFilteredAddress performs native verification of the outer
// constraints for FilteredAddressFrom / FilteredAddressTo invalidity types.
//
// The keccak wizard constraints are already validated by the dummy compiler.
// This checks:
//   - keccak256(rlpEncodedTx) == funcInputs.TxHash
//   - ToAddress from RLP matches the transaction's To field
//   - Filtered address flags are valid and the flagged address is non-zero
//   - PI hash consistency
func CheckOnlyNativeFilteredAddress(
	assi AssigningInputs,
) error {
	rlp := assi.RlpEncodedTx

	// --- Keccak hash check ---
	computedHash := crypto.Keccak256(rlp)
	expectedHash := assi.FuncInputs.TxHash
	if !bytes.Equal(computedHash, expectedHash[:]) {
		return fmt.Errorf("keccak hash mismatch: computed=%x, expected=%x", computedHash, expectedHash[:])
	}

	// --- To address from RLP ---
	toBytes, err := ExtractToAddressFromRLP(rlp)
	if err != nil {
		return fmt.Errorf("could not extract to address from RLP: %w", err)
	}
	txTo := assi.Transaction.To()
	if txTo == nil {
		return fmt.Errorf("transaction has nil To address")
	}
	if !bytes.Equal(toBytes, txTo[:]) {
		return fmt.Errorf("to address mismatch: rlp=%x, tx=%x", toBytes, txTo[:])
	}

	// --- Filtered address flags ---
	fromAddr := types.EthAddress(assi.FromAddress)
	toAddr := types.EthAddress(*txTo)
	var zeroAddr types.EthAddress

	switch assi.InvalidityType {
	case FilteredAddressFrom:
		if fromAddr == zeroAddr {
			return fmt.Errorf("from address is zero for FilteredAddressFrom")
		}
	case FilteredAddressTo:
		if toAddr == zeroAddr {
			return fmt.Errorf("to address is zero for FilteredAddressTo")
		}
	default:
		return fmt.Errorf("unsupported invalidity type for filtered-address native check: %s", assi.InvalidityType)
	}

	return nil
}

// verifyAccountTrie verifies the account trie membership / non-membership
// proof natively using Poseidon2 over KoalaBear.
func verifyAccountTrie(ati AccountTrieInputs, fromAddress types.EthAddress) error {
	// Address hash: Poseidon2(fromAddress) should equal TargetHKey
	computedHKey := hashAddress(fromAddress)
	if computedHKey != ati.TargetHKey {
		return fmt.Errorf("address hash mismatch: computed=%s, expected=%s", computedHKey.Hex(), ati.TargetHKey.Hex())
	}

	// TopRoot = Poseidon2(NextFreeNode || SubTreeRoot)
	computedTopRoot := ComputeTopRoot(ati.NextFreeNode, ati.SubRoot)
	if computedTopRoot != ati.TopRoot {
		return fmt.Errorf("top root mismatch")
	}

	// Verify both Merkle proofs share the same root (SubRoot)
	leafMinusHash := ati.ProofMinus.LeafOpening.Hash()
	if leafMinusHash != ati.ProofMinus.Leaf {
		return fmt.Errorf("leaf minus hash mismatch")
	}
	if err := smt.Verify(&ati.ProofMinus.Proof, ati.ProofMinus.Leaf, ati.SubRoot); err != nil {
		return fmt.Errorf("merkle proof minus verification failed: %w", err)
	}

	leafPlusHash := ati.ProofPlus.LeafOpening.Hash()
	if leafPlusHash != ati.ProofPlus.Leaf {
		return fmt.Errorf("leaf plus hash mismatch")
	}
	if err := smt.Verify(&ati.ProofPlus.Proof, ati.ProofPlus.Leaf, ati.SubRoot); err != nil {
		return fmt.Errorf("merkle proof plus verification failed: %w", err)
	}

	if ati.AccountExists {
		// Hash(Account) == LeafMinus.HVal
		computedHVal := hashAccount(ati.Account)
		if computedHVal != ati.ProofMinus.LeafOpening.HVal {
			return fmt.Errorf("account hash mismatch (HVal): computed=%s, expected=%s", computedHVal.Hex(), ati.ProofMinus.LeafOpening.HVal.Hex())
		}
		// TargetHKey == LeafMinus.HKey
		if ati.TargetHKey != ati.ProofMinus.LeafOpening.HKey {
			return fmt.Errorf("HKey mismatch: target=%s, leafMinus=%s", ati.TargetHKey.Hex(), ati.ProofMinus.LeafOpening.HKey.Hex())
		}
	} else {
		// Non-existing: adjacency checks
		if ati.ProofMinus.LeafOpening.Next != int64(ati.ProofPlus.Proof.Path) {
			return fmt.Errorf("adjacency: leafMinus.Next=%d != proofPlus.Path=%d", ati.ProofMinus.LeafOpening.Next, ati.ProofPlus.Proof.Path)
		}
		if ati.ProofPlus.LeafOpening.Prev != int64(ati.ProofMinus.Proof.Path) {
			return fmt.Errorf("adjacency: leafPlus.Prev=%d != proofMinus.Path=%d", ati.ProofPlus.LeafOpening.Prev, ati.ProofMinus.Proof.Path)
		}
		// Wrapping: minus.HKey < TargetHKey < plus.HKey
		if !octupletLess(ati.ProofMinus.LeafOpening.HKey, ati.TargetHKey) {
			return fmt.Errorf("wrapping: leafMinus.HKey is not less than targetHKey")
		}
		if !octupletLess(ati.TargetHKey, ati.ProofPlus.LeafOpening.HKey) {
			return fmt.Errorf("wrapping: targetHKey is not less than leafPlus.HKey")
		}
	}

	return nil
}

// hashAddress computes Poseidon2(address) natively.
func hashAddress(addr types.EthAddress) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	addr.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// hashAccount computes Poseidon2(account) natively.
func hashAccount(a types.Account) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	a.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// octupletLess returns true if a < b (element-wise big-endian comparison).
func octupletLess(a, b types.KoalaOctuplet) bool {
	for i := 0; i < 8; i++ {
		ai := a[i].Uint64()
		bi := b[i].Uint64()
		if ai < bi {
			return true
		}
		if ai > bi {
			return false
		}
	}
	return false
}

// checkWizardPI cross-checks PI values extracted from the wizard proof against
// the expected funcInputs. Used by BadPrecompile/TooManyLogs native path.
func checkWizardPI(
	invExtractor *invalidityPI.InvalidityPIExtractor,
	execExtractor *publicInput.FunctionalInputExtractor,
	getPI func(wizard.PublicInput) field.Element,
	funcInputs public_input.Invalidity,
) error {
	// TxHash
	txHashHi := nativeCombine16BitLimbs(invExtractor.TxHash[:8], getPI)
	txHashLo := nativeCombine16BitLimbs(invExtractor.TxHash[8:], getPI)
	if txHashHi.Cmp(new(big.Int).SetBytes(funcInputs.TxHash[:16])) != 0 {
		return fmt.Errorf("txHash[0] mismatch")
	}
	if txHashLo.Cmp(new(big.Int).SetBytes(funcInputs.TxHash[16:])) != 0 {
		return fmt.Errorf("txHash[1] mismatch")
	}

	// FromAddress
	fromAddr := nativeCombine16BitLimbs(invExtractor.FromAddress[:], getPI)
	if fromAddr.Cmp(new(big.Int).SetBytes(funcInputs.FromAddress[:])) != 0 {
		return fmt.Errorf("fromAddress mismatch")
	}

	// StateRootHash (8 x 32-bit KoalaBear elements)
	srHi := nativeCombine32BitLimbs(execExtractor.InitialStateRootHash[:4], getPI)
	srLo := nativeCombine32BitLimbs(execExtractor.InitialStateRootHash[4:], getPI)
	expectedSR := funcInputs.StateRootHash.ToBytes()
	if srHi.Cmp(new(big.Int).SetBytes(expectedSR[:16])) != 0 {
		return fmt.Errorf("stateRootHash[0] mismatch")
	}
	if srLo.Cmp(new(big.Int).SetBytes(expectedSR[16:])) != 0 {
		return fmt.Errorf("stateRootHash[1] mismatch")
	}

	// Conditional cross-checks (only when extracted != 0)
	type piCheck struct {
		name     string
		limbs    []wizard.PublicInput
		expected *big.Int
	}
	for _, c := range []piCheck{
		{"coinBase", execExtractor.CoinBase[:], new(big.Int).SetBytes(funcInputs.CoinBase[:])},
		{"baseFee", execExtractor.BaseFee[:], new(big.Int).SetUint64(funcInputs.BaseFee)},
		{"chainID", execExtractor.ChainID[:], new(big.Int).SetUint64(funcInputs.ChainID)},
		{"l2MsgServiceAddr", execExtractor.L2MessageServiceAddr[:], new(big.Int).SetBytes(funcInputs.L2MessageServiceAddr[:])},
		{"blockTimestamp", execExtractor.InitialBlockTimestamp[:], new(big.Int).SetUint64(funcInputs.SimulatedBlockTimestamp)},
	} {
		extracted := nativeCombine16BitLimbs(c.limbs, getPI)
		if extracted.Sign() != 0 && extracted.Cmp(c.expected) != 0 {
			return fmt.Errorf("%s mismatch: wizard=%s, expected=%s", c.name, extracted, c.expected)
		}
	}

	// blockNumber
	blockNum := nativeCombine16BitLimbs(execExtractor.InitialBlockNumber[:], getPI)
	if blockNum.Cmp(new(big.Int).SetUint64(funcInputs.SimulatedBlockNumber)) != 0 {
		return fmt.Errorf("blockNumber mismatch")
	}

	return nil
}

func nativeCombine16BitLimbs(pis []wizard.PublicInput, getPI func(wizard.PublicInput) field.Element) *big.Int {
	result := new(big.Int)
	shift := new(big.Int).SetUint64(1 << 16)
	for _, pi := range pis {
		v := getPI(pi)
		var vBig big.Int
		v.BigInt(&vBig)
		result.Mul(result, shift)
		result.Add(result, &vBig)
	}
	return result
}

func nativeCombine32BitLimbs(pis []wizard.PublicInput, getPI func(wizard.PublicInput) field.Element) *big.Int {
	result := new(big.Int)
	shift := new(big.Int).SetUint64(1 << 32)
	for _, pi := range pis {
		v := getPI(pi)
		var vBig big.Int
		v.BigInt(&vBig)
		result.Mul(result, shift)
		result.Add(result, &vBig)
	}
	return result
}

func getInvalidityExtractor(comp *wizard.CompiledIOP) (*invalidityPI.InvalidityPIExtractor, error) {
	raw, found := comp.ExtraData[invalidityPI.InvalidityPIExtractorMetadata]
	if !found {
		return nil, fmt.Errorf("InvalidityPIExtractor not found in CompiledIOP ExtraData")
	}
	ext, ok := raw.(*invalidityPI.InvalidityPIExtractor)
	if !ok {
		return nil, fmt.Errorf("InvalidityPIExtractor has wrong type: %T", raw)
	}
	return ext, nil
}

func getExecutionExtractor(comp *wizard.CompiledIOP) (*publicInput.FunctionalInputExtractor, error) {
	raw, found := comp.ExtraData[publicInput.PublicInputExtractorMetadata]
	if !found {
		return nil, fmt.Errorf("FunctionalInputExtractor not found in CompiledIOP ExtraData")
	}
	ext, ok := raw.(*publicInput.FunctionalInputExtractor)
	if !ok {
		return nil, fmt.Errorf("FunctionalInputExtractor has wrong type: %T", raw)
	}
	return ext, nil
}

// compile-time check that limb constants match expected layouts
var _ [common.NbLimbU256]wizard.PublicInput
var _ [common.NbLimbEthAddress]wizard.PublicInput

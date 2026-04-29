package invalidity

import (
	"github.com/consensys/gnark/frontend"
	wizardk "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/consensys/linea-monorepo/prover/circuits/internal"
)

// FilteredAddressCircuit defines the circuit for filtered address invalidity proofs.
// It proves that a forced transaction involves a filtered (denied) address, either as
// the sender (From) or the recipient (To).
//
// The circuit verifies:
// - FromIsFiltered and ToIsFiltered are boolean and mutually exclusive
// - The flagged address is non-zero
// - The ToAddress is consistent with the RLP-encoded transaction
// - The TxHash matches the keccak hash of the RLP-encoded transaction
// - Rolling hash binds FromAddress on-chain (FromAddress is passed to the publicInputs)
type FilteredAddressCircuit struct {
	// RLP-encoded payload prefixed with the type byte: txType || rlp(tx.inner)
	RLPEncodedTx []frontend.Variable
	// Sender address
	TxFromAddress frontend.Variable
	// To address extracted from the RLP
	TxToAddress frontend.Variable
	// Hash of the transaction (split into two 16-byte chunks)
	TxHash [2]frontend.Variable
	// 1 if the from address is filtered, 0 otherwise
	FromIsFiltered frontend.Variable
	// 1 if the to address is filtered, 0 otherwise
	ToIsFiltered frontend.Variable
	// Keccak verifier circuit
	KeccakH wizardk.VerifierCircuit

	// Execution PI fields (unconstrained by subcircuit, flow into public input hash)
	StateRootHash         [2]frontend.Variable
	CoinBase              frontend.Variable
	BaseFee               frontend.Variable
	ChainID               frontend.Variable
	L2MessageServiceAddr  frontend.Variable
	InitialBlockTimestamp frontend.Variable
	InitialBlockNumber    frontend.Variable

	api frontend.API
}

// Define represents the constraints relevant to [FilteredAddressCircuit]
func (c *FilteredAddressCircuit) Define(api frontend.API) error {
	c.api = api

	// ========== FILTERED ADDRESS FLAGS ==========
	// FromIsFiltered and ToIsFiltered are boolean
	api.AssertIsBoolean(c.FromIsFiltered)
	api.AssertIsBoolean(c.ToIsFiltered)

	// At least one must be filtered (this is an invalidity proof for a filtered address)
	api.AssertIsEqual(api.Add(c.FromIsFiltered, c.ToIsFiltered), 1)

	// If FromIsFiltered is 1, then FromAddress must be non-zero
	internal.AssertEqualIf(api, c.FromIsFiltered, api.Sub(1, api.IsZero(c.TxFromAddress)), 1)

	// If ToIsFiltered is 1, then ToAddress must be non-zero
	internal.AssertEqualIf(api, c.ToIsFiltered, api.Sub(1, api.IsZero(c.TxToAddress)), 1)

	// ========== TO ADDRESS CONSISTENCY ==========
	// Verify ToAddress matches the RLP-encoded transaction's "to" field
	expectedToAddress := ExtractToAddressFromRLPZk(api, c.RLPEncodedTx)
	api.AssertIsEqual(expectedToAddress, c.TxToAddress)

	// ========== KECCAK VERIFICATION ==========
	// Verify TxHash matches the keccak hash of the RLP-encoded transaction
	CheckKeccakConsistency(api, c.RLPEncodedTx, c.TxHash, &c.KeccakH)
	c.KeccakH.Verify(api)

	return nil
}

// Allocate the circuit
func (c *FilteredAddressCircuit) Allocate(config Config) {
	c.KeccakH = *wizardk.AllocateWizardCircuit(config.KeccakCompiledIOP, 0)
	c.RLPEncodedTx = make([]frontend.Variable, config.MaxRlpByteSize)
}

// Assign the circuit from [AssigningInputs]
func (c *FilteredAddressCircuit) Assign(assi AssigningInputs) {
	txHash := crypto.Keccak256(assi.RlpEncodedTx)
	keccak := wizardk.AssignVerifierCircuit(assi.KeccakCompiledIOP, assi.KeccakProof, 0)

	c.TxFromAddress = assi.FromAddress[:]
	c.TxHash[0] = txHash[0:LIMB_SIZE]
	c.TxHash[1] = txHash[LIMB_SIZE:]

	// Extract "to" address from RLP
	toBytes, err := ExtractToAddressFromRLP(assi.RlpEncodedTx)
	if err != nil {
		utils.Panic("could not extract to address from RLP: %v", err)
	}
	c.TxToAddress = toBytes

	// Set filtered flags based on invalidity type
	switch assi.InvalidityType {
	case FilteredAddressFrom:
		c.FromIsFiltered = 1
		c.ToIsFiltered = 0
	case FilteredAddressTo:
		c.FromIsFiltered = 0
		c.ToIsFiltered = 1
	default:
		utils.Panic("expected invalidity type FilteredAddressFrom or FilteredAddressTo but received %v", assi.InvalidityType)
	}

	// Assign RLP encoding
	c.RLPEncodedTx = make([]frontend.Variable, assi.MaxRlpByteSize)

	if assi.RlpEncodedTx[0] != 0x02 {
		utils.Panic("only support typed 2 transactions, maybe the rlp is not prefixed with the type byte")
	}

	elements := internal.FromBytesToElements(assi.RlpEncodedTx)
	rlpLen := len(elements)
	if rlpLen > assi.MaxRlpByteSize {
		utils.Panic("rlp encoding is too large: got %d, max %d", rlpLen, assi.MaxRlpByteSize)
	}

	copy(c.RLPEncodedTx, elements)
	for i := len(elements); i < assi.MaxRlpByteSize; i++ {
		c.RLPEncodedTx[i] = 0
	}

	c.KeccakH = *keccak
	rootBytes := assi.FuncInputs.StateRootHash.ToBytes()
	c.StateRootHash[0] = rootBytes[:16]
	c.StateRootHash[1] = rootBytes[16:]

	c.CoinBase = assi.FuncInputs.CoinBase[:]
	c.BaseFee = assi.FuncInputs.BaseFee
	c.ChainID = assi.FuncInputs.ChainID
	c.L2MessageServiceAddr = assi.FuncInputs.L2MessageServiceAddr[:]
	c.InitialBlockTimestamp = assi.FuncInputs.SimulatedBlockTimestamp
	c.InitialBlockNumber = assi.FuncInputs.SimulatedBlockNumber
}

// FunctionalPIQGnark returns the subcircuit-derived functional public inputs
func (c *FilteredAddressCircuit) FunctionalPIQGnark() FunctionalPIQGnark {
	return FunctionalPIQGnark{
		TxHash:                  c.TxHash,
		FromAddress:             c.TxFromAddress,
		StateRootHash:           c.StateRootHash,
		ToAddress:               c.TxToAddress,
		ToIsFiltered:            c.ToIsFiltered,
		FromIsFiltered:          c.FromIsFiltered,
		CoinBase:                c.CoinBase,
		BaseFee:                 c.BaseFee,
		ChainID:                 c.ChainID,
		L2MessageServiceAddr:    c.L2MessageServiceAddr,
		SimulatedBlockTimestamp: c.InitialBlockTimestamp,
		SimulatedBlockNumber:    c.InitialBlockNumber,
	}
}

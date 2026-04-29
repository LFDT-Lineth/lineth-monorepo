package invalidity

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	zkevmcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecdsa"
)

// MockInvalidityPIInputs holds mock columns for testing InvalidityPI (from arithmetization, ECDSA, and logs module)
type MockInvalidityPIInputs struct {
	// Mock badPrecompile column
	BadPrecompileCol ifaces.Column

	// Mock Addresses columns (limb-based)
	AddressHiUntrimmed   [zkevmcommon.NbLimbU128]ifaces.Column // 8 limbs for 128-bit value
	AddressLo            [zkevmcommon.NbLimbU128]ifaces.Column // 8 limbs for 128-bit value
	IsAddressFromTxnData ifaces.Column

	// Mock TxSignature columns (limb-based)
	TxHash   [zkevmcommon.NbLimbU256]ifaces.Column // 16 limbs for 256-bit value
	IsTxHash ifaces.Column

	// Mock ExtractedData columns
	FilterFetched ifaces.Column
}

// FixedInputs are the inputs that are fixed for the test
type FixedInputs struct {
	TxHashLimbs [zkevmcommon.NbLimbU256]field.Element       // 16 limbs for 256-bit tx hash, LE order
	FromLimbs   [zkevmcommon.NbLimbEthAddress]field.Element // 10 limbs for 160-bit address, LE order
	ColSize     int

	// Execution PI fields (from FunctionalInputExtractor)
	StateRootHash         [zkevmcommon.NbElemPerHash]field.Element    // 8 x 32-bit KoalaBear elements
	CoinBase              [zkevmcommon.NbLimbEthAddress]field.Element // 10 x 16-bit limbs
	BaseFee               [zkevmcommon.NbLimbU128]field.Element       // 8 x 16-bit limbs
	ChainID               [zkevmcommon.NbLimbU256]field.Element       // 16 x 16-bit limbs
	L2MessageServiceAddr  [zkevmcommon.NbLimbEthAddress]field.Element // 10 x 16-bit limbs
	InitialBlockTimestamp [zkevmcommon.NbLimbU128]field.Element       // 8 x 16-bit limbs
	InitialBlockNumber    [zkevmcommon.NbLimbU48]field.Element        // 3 x 16-bit limbs
}

// CaseInputs are the inputs that are different for each test case
type CaseInputs struct {
	HasBadPrecompile bool
	NumL2Logs        int
}
type Inputs struct {
	FixedInputs
	CaseInputs
}

// CreateMockInputs creates mock columns for testing
func CreateMockInputs(comp *wizard.CompiledIOP, size int) *MockInvalidityPIInputs {
	var (
		addressHiUntrimmed [zkevmcommon.NbLimbU128]ifaces.Column
		addressLo          [zkevmcommon.NbLimbU128]ifaces.Column
		txHash             [zkevmcommon.NbLimbU256]ifaces.Column
	)

	for i := range zkevmcommon.NbLimbU128 {
		addressHiUntrimmed[i] = comp.InsertCommit(0, ifaces.ColIDf("MOCK_ADDRESS_HI_UNTRIMMED_%d", i), size, true)
	}

	for i := range zkevmcommon.NbLimbU128 {
		addressLo[i] = comp.InsertCommit(0, ifaces.ColIDf("MOCK_ADDRESS_LO_%d", i), size, true)
	}

	for i := range zkevmcommon.NbLimbU256 {
		txHash[i] = comp.InsertCommit(0, ifaces.ColIDf("MOCK_TX_HASH_%d", i), size, true)
	}

	return &MockInvalidityPIInputs{
		BadPrecompileCol:     comp.InsertCommit(0, "hub.PROVER_ILLEGAL_TRANSACTION_DETECTED", size/2, true),
		AddressHiUntrimmed:   addressHiUntrimmed,
		AddressLo:            addressLo,
		IsAddressFromTxnData: comp.InsertCommit(0, "MOCK_IS_ADDRESS_FROM_TXNDATA", size, true),
		TxHash:               txHash,
		IsTxHash:             comp.InsertCommit(0, "MOCK_IS_TX_HASH", size, true),
		FilterFetched:        comp.InsertCommit(0, "MOCK_FILTER_FETCHED", size*2, true),
	}
}

func AssignMockInputs(run *wizard.ProverRuntime, colSize int, mockInputs *MockInvalidityPIInputs, in Inputs) {
	// Assign mock badPrecompile column
	badPrecompileVec := make([]field.Element, colSize/2)
	if in.CaseInputs.HasBadPrecompile {
		badPrecompileVec[2] = field.One() // Set a non-zero value at index 2
		badPrecompileVec[3] = field.One()
		badPrecompileVec[4] = field.One()
	}
	run.AssignColumn(mockInputs.BadPrecompileCol.GetColID(), smartvectors.NewRegular(badPrecompileVec))

	// Assign mock address columns (limb-based)
	// The address is constructed using Addresses() which uses:
	//   FuseLimbs(AddressHiUntrimmed.SliceOnBit(96, 128), AddressLo)
	// SliceOnBit uses BE-style bit indexing, so SliceOnBit(96, 128) returns limbs 0-1 (lower 32 bits)
	// This gives 10 limbs (160 bits):
	//   - Fused limbs 0-7: AddressLo (low 128 bits of address)
	//   - Fused limbs 8-9: AddressHiUntrimmed limbs 0-1 (high 32 bits of address)
	// FromLimbs is in LE order: [0:8] = low 128 bits, [8:10] = high 32 bits
	isAddressFromTxnDataVec := make([]field.Element, colSize)
	isAddressFromTxnDataVec[3] = field.One() // Mark row 3 as having the address

	// Assign AddressLo = FromLimbs[0:8] (low 128 bits of the address)
	for i := range zkevmcommon.NbLimbU128 {
		vec := make([]field.Element, colSize)
		vec[3] = in.FromLimbs[i] // FromLimbs[0:8] maps to AddressLo[0:8]
		run.AssignColumn(mockInputs.AddressLo[i].GetColID(), smartvectors.NewRegular(vec))
	}

	// Assign AddressHiUntrimmed: limbs 0-1 = FromLimbs[8:10] (high 32 bits), limbs 2-7 are zeros
	// SliceOnBit(96, 128) extracts limbs 0-1 from AddressHiUntrimmed
	for i := range zkevmcommon.NbLimbU128 {
		vec := make([]field.Element, colSize)
		if i < 2 { // Limbs 0-1 contain the high 32 bits of address
			vec[3] = in.FromLimbs[8+i] // FromLimbs[8:10] maps to AddressHiUntrimmed[0:2]
		}
		// limbs 2-7 remain zero
		run.AssignColumn(mockInputs.AddressHiUntrimmed[i].GetColID(), smartvectors.NewRegular(vec))
	}
	run.AssignColumn(mockInputs.IsAddressFromTxnData.GetColID(), smartvectors.NewRegular(isAddressFromTxnDataVec))

	// Assign mock TxSignature columns (16 limbs for TxHash)
	isTxHashVec := make([]field.Element, colSize)
	isTxHashVec[5] = field.One() // Mark row 5 as having the tx hash

	for i := range zkevmcommon.NbLimbU256 {
		vec := make([]field.Element, colSize)
		vec[5] = in.TxHashLimbs[i]
		run.AssignColumn(mockInputs.TxHash[i].GetColID(), smartvectors.NewRegular(vec))
	}
	run.AssignColumn(mockInputs.IsTxHash.GetColID(), smartvectors.NewRegular(isTxHashVec))

	// Assign mock FilterFetched column (number of L2L1 logs) - uses different size
	filterFetchedSize := colSize * 2
	filterFetchedVec := make([]field.Element, filterFetchedSize)
	for i := 0; i < in.NumL2Logs && i < filterFetchedSize; i++ {
		filterFetchedVec[i] = field.One()
	}
	run.AssignColumn(mockInputs.FilterFetched.GetColID(), smartvectors.NewRegular(filterFetchedVec))
}

// MockZkevmArithCols creates a zkevm with just the public inputs needed for InvalidityPI
func MockZkevmArithCols(in Inputs) (*wizard.CompiledIOP, wizard.Proof) {
	var (
		mockInputs *MockInvalidityPIInputs
		pi         *InvalidityPI
	)

	define := func(b *wizard.Builder) {
		comp := b.CompiledIOP

		// Create mock input columns
		mockInputs = CreateMockInputs(comp, in.ColSize)

		// Create limbs.Uint128Le and limbs.Uint256Le from mock column arrays
		addressHiUntrimmed := limbs.FromSliceUnsafe[limbs.S128, limbs.LittleEndian]("MOCK_ADDRESS_HI_UNTRIMMED", mockInputs.AddressHiUntrimmed[:])
		addressLo := limbs.FromSliceUnsafe[limbs.S128, limbs.LittleEndian]("MOCK_ADDRESS_LO", mockInputs.AddressLo[:])
		txHash := limbs.FromSliceUnsafe[limbs.S256, limbs.LittleEndian]("MOCK_TX_HASH", mockInputs.TxHash[:])

		pi = NewInvalidityPI(comp,
			&ecdsa.EcdsaZkEvm{
				Ant: &ecdsa.Antichamber{
					Size: in.ColSize,
					TxSignature: &ecdsa.TxSignature{
						IsTxHash: mockInputs.IsTxHash,
						TxHash:   txHash,
					},
					Addresses: &ecdsa.Addresses{
						AddressHiUntrimmed:   addressHiUntrimmed,
						AddressLo:            addressLo,
						IsAddressFromTxnData: mockInputs.IsAddressFromTxnData,
					},
				},
			},
			mockInputs.FilterFetched,
		)
	}

	prove := func(run *wizard.ProverRuntime) {
		// First assign the mock input columns
		AssignMockInputs(run, in.ColSize, mockInputs, in)

		// Run the InvalidityPI Assign
		pi.Assign(run)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)

	return comp, proof
}

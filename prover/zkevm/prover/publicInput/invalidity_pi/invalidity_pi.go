package invalidity

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	zkevmcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecdsa"
)

// Public input names for invalidity proofs - these must match what BadPrecompileCircuit expects

const (
	// Metadata key for storing the extractor in CompiledIOP
	InvalidityPIExtractorMetadata = "InvalidityPIExtractor"

	TxHashName           = "TxHash" // 16 limbs in BE order (MSB first), converted from ECDSA LE
	FromName             = "From"   // 10 limbs in BE order (MSB first), converted from ECDSA LE
	HasBadPrecompileName = "HasBadPrecompile"
	NbL2LogsName         = "NbL2Logs"
)

// InvalidityPI is the module responsible for extracting public inputs
// needed for invalidity proofs (BadPrecompile and TooManyLogs circuits).
type InvalidityPI struct {
	InputColumns InputColumns

	// Output columns - these become wizard public inputs
	TxHash limbs.Uint256Be
	From   limbs.EthAddress

	HasBadPrecompile ifaces.Column // backward accumulator of the badPrecompileCol column
	NbL2Logs         ifaces.Column // backward accumulator of the filterFetched column
	// Extractor holds the LocalOpening queries for the public inputs
	// used to register public inputs via LocalOpening queries
	Extractor InvalidityPIExtractor
}

// InputColumns collects the input columns from the arithmetization, ECDSA, and logs module
type InputColumns struct {
	// Input columns from arithmetization indicate if a bad precompile was detected
	BadPrecompileCol ifaces.Column

	// Input columns from ECDSA
	Addresses   *ecdsa.Addresses   // used to bubble up FromAddress to the public input
	TxSignature *ecdsa.TxSignature // used to bubble up TxHash to the public input

	// Input columns from logs and root hash fetcher
	FilterFetchedL2L1 ifaces.Column // Input columns from logs - used to bubble up NbL2Logs to the public input
}

// InvalidityPIExtractor holds wizard.PublicInput for invalidity public inputs
type InvalidityPIExtractor struct {
	TxHash      [zkevmcommon.NbLimbU256]wizard.PublicInput
	FromAddress [zkevmcommon.NbLimbEthAddress]wizard.PublicInput

	HasBadPrecompile wizard.PublicInput // the first row of the HasBadPrecompile column (a non-zero value means a bad precompile was detected)
	NbL2Logs         wizard.PublicInput // the first row of the backward accumulator of the FilterFetched column
}

// NewInvalidityPI creates a new InvalidityPI module from the arithmetization, ECDSA, and logs module
func NewInvalidityPI(comp *wizard.CompiledIOP, ecdsa *ecdsa.EcdsaZkEvm, filteredFetchedL2L1 ifaces.Column) *InvalidityPI {

	var (
		name              = "INVALIDITY_PI"
		ecdsaSize         = ecdsa.Ant.Size
		badPrecompileCol  = comp.Columns.GetHandle("hub.PROVER_ILLEGAL_TRANSACTION_DETECTED")
		badPrecompileSize = badPrecompileCol.Size()
		filterFetchedSize = filteredFetchedL2L1.Size()
	)

	// Create output columns using limbs types for clear endianness
	txHash := limbs.NewUint256Be(comp, ifaces.ColID(name+"_TX_HASH"), ecdsaSize)
	from := limbs.NewEthAddress(comp, ifaces.ColID(name+"_FROM"), ecdsaSize)

	hashBadPrecompile := comp.InsertCommit(0, ifaces.ColIDf("%s_HAS_BAD_PRECOMPILE", name), badPrecompileSize, true)
	nbL2Logs := comp.InsertCommit(0, ifaces.ColIDf("%s_NB_L2_LOGS", name), filterFetchedSize, true)

	pi := &InvalidityPI{
		// Output columns
		TxHash:           txHash,
		From:             from,
		HasBadPrecompile: hashBadPrecompile,
		NbL2Logs:         nbL2Logs,

		// Input columns
		InputColumns: InputColumns{
			BadPrecompileCol:  badPrecompileCol,
			TxSignature:       ecdsa.Ant.TxSignature,
			Addresses:         ecdsa.Ant.Addresses,
			FilterFetchedL2L1: filteredFetchedL2L1,
		},
	}

	// Define constraints over the columns of pi.
	pi.defineConstraints(comp)

	// Create extractor and register public inputs
	pi.generateExtractor(comp, name)

	return pi
}

// defineConstraints defines constraints over the columns of pi.
func (pi *InvalidityPI) defineConstraints(comp *wizard.CompiledIOP) {

	// HasBadPrecompile must be an accumulator backward of the badPrecompileCol column
	commonconstraints.MustBeAccumulatorBackward(comp, pi.HasBadPrecompile, pi.InputColumns.BadPrecompileCol)

	// NbL2Logs is the backward accumulator of the filterFetched column
	commonconstraints.MustBeAccumulatorBackward(comp, pi.NbL2Logs, pi.InputColumns.FilterFetchedL2L1)

	// Backward propagation for TxHash:
	//   when IsTxHash=1: pi.TxHash[i] = ecdsaTxHash_BE[i]  (grab from ECDSA)
	//   when IsTxHash=0: pi.TxHash[i] = pi.TxHash[i+1]     (copy from next row)
	// This propagates the first flagged value up to row 0 for the LocalOpening.
	txHashShifted := limbs.Shift(pi.TxHash.AsDynSize(), 1)
	limbs.NewGlobal(comp, ifaces.QueryIDf("%s_TX_HASH_PROPAGATION", "INVALIDITY_PI"),
		sym.Add(
			sym.Sub(pi.TxHash.AsDynSize(), txHashShifted),
			sym.Mul(pi.InputColumns.TxSignature.IsTxHash,
				sym.Sub(txHashShifted, pi.InputColumns.TxSignature.TxHash.ToBigEndianLimbs()),
			),
		))

	// Backward propagation for From:
	//   when IsAddressFromTxnData=1: pi.From[i] = ecdsaAddr_BE[i]  (grab from ECDSA)
	//   when IsAddressFromTxnData=0: pi.From[i] = pi.From[i+1]     (copy from next row)
	fromShifted := limbs.Shift(pi.From.AsDynSize(), 1)
	limbs.NewGlobal(comp, ifaces.QueryIDf("%s_FROM_PROPAGATION", "INVALIDITY_PI"),
		sym.Add(
			sym.Sub(pi.From.AsDynSize(), fromShifted),
			sym.Mul(pi.InputColumns.Addresses.IsAddressFromTxnData,
				sym.Sub(fromShifted, pi.InputColumns.Addresses.Addresses().ToBigEndianLimbs()),
			),
		))
}

// generateExtractor registers public inputs
func (pi *InvalidityPI) generateExtractor(comp *wizard.CompiledIOP, name string) {
	// Helper: creates LocalOpening query + PublicInput for a single column
	newLoPublicInput := func(col ifaces.Column, name string) wizard.PublicInput {
		q := comp.InsertLocalOpening(0, ifaces.QueryIDf("%s_%s", "PUBLIC_INPUT_LOCAL_OPENING", name), col)
		return comp.InsertPublicInput(name, accessors.NewLocalOpeningAccessor(q, 0))
	}

	// Helper: creates LocalOpening queries + PublicInputs for an array of columns
	newLoPublicInputs := func(cols []ifaces.Column, baseName string) []wizard.PublicInput {
		pis := make([]wizard.PublicInput, len(cols))
		for i, col := range cols {
			pis[i] = newLoPublicInput(col, fmt.Sprintf("%s_%d", baseName, i))
		}
		return pis
	}

	// Register TxHash public inputs
	copy(pi.Extractor.TxHash[:], newLoPublicInputs(pi.TxHash.GetLimbs(), TxHashName))

	// Register FromAddress public inputs
	copy(pi.Extractor.FromAddress[:], newLoPublicInputs(pi.From.GetLimbs(), FromName))

	// Register scalar public inputs
	pi.Extractor.HasBadPrecompile = newLoPublicInput(pi.HasBadPrecompile, HasBadPrecompileName)
	pi.Extractor.NbL2Logs = newLoPublicInput(pi.NbL2Logs, NbL2LogsName)

	// Store extractor in CompiledIOP metadata for easy circuit access
	// This follows the same pattern as execution circuit's input_extractor.go:171
	comp.ExtraData[InvalidityPIExtractorMetadata] = &pi.Extractor
}

// Assign assigns values to the InvalidityPI columns.
func (pi *InvalidityPI) Assign(run *wizard.ProverRuntime) {

	isTxHashCol := pi.InputColumns.TxSignature.IsTxHash.GetColAssignment(run)
	ecdsaSize := isTxHashCol.Len()
	isFromCol := pi.InputColumns.Addresses.IsAddressFromTxnData.GetColAssignment(run)

	// Backward-propagate TxHash: when IsTxHash=1 grab from ECDSA, else copy next row.
	// The first flagged value propagates to row 0.
	txHashLimbValues := backwardPropagate(ecdsaSize, zkevmcommon.NbLimbU256, isTxHashCol,
		func(row int) []field.Element {
			leRow := pi.InputColumns.TxSignature.TxHash.GetRow(run, row)
			be := make([]field.Element, zkevmcommon.NbLimbU256)
			for j := range be {
				be[j] = leRow.T[zkevmcommon.NbLimbU256-1-j]
			}
			return be
		})
	for limbIdx, col := range pi.TxHash.GetLimbs() {
		run.AssignColumn(col.GetColID(), smartvectors.NewRegular(txHashLimbValues[limbIdx]))
	}

	// Backward-propagate From: when IsAddressFromTxnData=1 grab from ECDSA, else copy next row.
	fromLimbValues := backwardPropagate(ecdsaSize, zkevmcommon.NbLimbEthAddress, isFromCol,
		func(row int) []field.Element {
			leRow := pi.InputColumns.Addresses.Addresses().GetRow(run, row)
			be := make([]field.Element, zkevmcommon.NbLimbEthAddress)
			for j := range be {
				be[j] = leRow.T[zkevmcommon.NbLimbEthAddress-1-j]
			}
			return be
		})
	for limbIdx, col := range pi.From.GetLimbs() {
		run.AssignColumn(col.GetColID(), smartvectors.NewRegular(fromLimbValues[limbIdx]))
	}

	// Assign NbL2Logs (backward accumulator of filterFetched)
	filterFetched := pi.InputColumns.FilterFetchedL2L1.GetColAssignment(run)
	sizeFetched := filterFetched.Len()
	accNbL2LogsValues := make([]field.Element, sizeFetched)
	accNbL2LogsValues[sizeFetched-1] = filterFetched.Get(sizeFetched - 1)
	for i := sizeFetched - 2; i >= 0; i-- {
		val := filterFetched.Get(i)
		accNbL2LogsValues[i].Add(&val, &accNbL2LogsValues[i+1])
	}
	run.AssignColumn(pi.NbL2Logs.GetColID(), smartvectors.NewRegular(accNbL2LogsValues))

	// Assign HasBadPrecompile (backward accumulator of badPrecompileCol)
	badPrecompileCol := pi.InputColumns.BadPrecompileCol.GetColAssignment(run)
	size := badPrecompileCol.Len()
	accBadPrecompileValues := make([]field.Element, size)
	accBadPrecompileValues[size-1] = badPrecompileCol.Get(size - 1)
	for i := size - 2; i >= 0; i-- {
		val := badPrecompileCol.Get(i)
		accBadPrecompileValues[i].Add(&val, &accBadPrecompileValues[i+1])
	}
	run.AssignColumn(pi.HasBadPrecompile.GetColID(), smartvectors.NewRegular(accBadPrecompileValues))

	// Assign local openings from row 0 of the propagated columns
	assignLO := func(pi wizard.PublicInput, value field.Element) {
		q, ok := pi.Acc.(*accessors.FromLocalOpeningYAccessor)
		if !ok {
			panic("pi.Acc is not a FromLocalOpeningYAccessor")
		}
		run.AssignLocalPoint(q.Q.ID, value)
	}

	for i := range pi.Extractor.TxHash {
		assignLO(pi.Extractor.TxHash[i], txHashLimbValues[i][0])
	}
	for i := range pi.Extractor.FromAddress {
		assignLO(pi.Extractor.FromAddress[i], fromLimbValues[i][0])
	}
	assignLO(pi.Extractor.HasBadPrecompile, accBadPrecompileValues[0])
	assignLO(pi.Extractor.NbL2Logs, accNbL2LogsValues[0])
}

// backwardPropagate builds column values via backward propagation:
// when flag[i]=1, grab BE limbs from readBE(i); when flag[i]=0, copy from row i+1.
// Returns [nbLimbs][size]field.Element (one full column per limb).
func backwardPropagate(
	size, nbLimbs int,
	flag smartvectors.SmartVector,
	readBE func(row int) []field.Element,
) [][]field.Element {

	// Find the first flagged row to seed the cyclic wrap
	var seedBE []field.Element
	for i := 0; i < size; i++ {
		v := flag.Get(i)
		if v.IsOne() {
			seedBE = readBE(i)
			break
		}
	}
	if seedBE == nil {
		panic("backwardPropagate: no flagged row found")
	}

	cols := make([][]field.Element, nbLimbs)
	for j := range cols {
		cols[j] = make([]field.Element, size)
	}

	// Initialize current values with the seed (first flagged row)
	current := make([]field.Element, nbLimbs)
	copy(current, seedBE)

	// Backward pass: from the last row toward row 0
	for i := size - 1; i >= 0; i-- {
		v := flag.Get(i)
		if v.IsOne() {
			copy(current, readBE(i))
		}
		for j := 0; j < nbLimbs; j++ {
			cols[j][i] = current[j]
		}
	}

	return cols
}

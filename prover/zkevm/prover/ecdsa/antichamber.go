package ecdsa

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

const (
	ROUND_NR                 = 0
	NAME_ANTICHAMBER         = "ECDSA_ANTICHAMBER"
	NAME_ECRECOVER           = "ECDSA_ANTICHAMBER_ECRECOVER"
	NAME_ADDRESSES           = "ECDSA_ANTICHAMBER_ADDRESSES"
	NAME_TXSIGNATURE         = "ECDSA_ANTICHAMBER_TXSIGNATURE"
	NAME_UNALIGNED_GNARKDATA = "ECDSA_ANTICHAMBER_UNALIGNED_GNARK_DATA"
	NAME_GNARK_DATA          = "ECDSA_ANTICHAMBER_GNARK_DATA"
)

const (
	// keccak digest is on size 32bytes
	halfDigest = 16

	// number of public inputs gnark circuit takes as a public witness
	nbRowsPerGnarkPushing = 14

	// 10 rows for fetching from arithmetization-columns
	nbRowsPerEcRecFetching = 10
	nbRowsPerEcRec         = nbRowsPerEcRecFetching + nbRowsPerGnarkPushing

	// 1 row for fetching from RLP
	nbRowsPerTxSignFetching = 1
	nbRowsPerTxSign         = nbRowsPerTxSignFetching + nbRowsPerGnarkPushing
)

func createColFn(comp *wizard.CompiledIOP, rootName string, size int) func(name string) ifaces.Column {
	return func(name string) ifaces.Column {
		return comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", rootName, name), size)
	}
}

type antichamberInput struct {
	EcSource     *ecDataSource
	TxSource     *txnData
	RlpTxn       generic.GenDataModule
	Settings     *Settings
	PlonkOptions []query.PlonkOption
}

type antichamber struct {
	Inputs     *antichamberInput
	IsActive   ifaces.Column
	ID         ifaces.Column
	IsPushing  ifaces.Column
	IsFetching ifaces.Column
	Source     ifaces.Column // source of the data. Source=0 <> ECRecover, Source=1 <> TxSignature

	*EcRecover
	*Addresses
	*TxSignature
	*UnalignedGnarkData
	AlignedGnarkData *plonk.Alignment

	// Size of AntiChamber
	Size int

	// providers for keccak, Providers contain the inputs and outputs of keccak hash.
	Providers []generic.GenericByteModule
}

type Settings struct {
	MaxNbEcRecover     int
	MaxNbTx            int
	NbInputInstance    int
	NbCircuitInstances int
}

func (l *Settings) sizeAntichamber() int {
	return utils.NextPowerOfTwo(l.MaxNbEcRecover*nbRowsPerEcRec + l.MaxNbTx*nbRowsPerTxSign)
}

func newAntichamber(comp *wizard.CompiledIOP, inputs *antichamberInput) *antichamber {

	settings := inputs.Settings
	if settings.MaxNbEcRecover+settings.MaxNbTx > settings.NbInputInstance*settings.NbCircuitInstances {
		utils.Panic("the number of supported instances %v should be %v + %v", settings.NbInputInstance*settings.NbCircuitInstances, settings.MaxNbEcRecover, settings.MaxNbTx)
	}
	size := inputs.Settings.sizeAntichamber()
	createCol := createColFn(comp, NAME_ANTICHAMBER, size)

	// declare the native columns
	res := &antichamber{

		IsActive:   createCol("IS_ACTIVE"),
		ID:         createCol("ID"),
		IsPushing:  createCol("IS_PUSHING"),
		IsFetching: createCol("IS_FETCHING"),
		Source:     createCol("SOURCE"),
		Inputs:     inputs,

		Size: size,
	}

	// declare submodules
	txSignInputs := txSignatureInputs{
		RlpTxn: inputs.RlpTxn,
		Ac:     res,
	}
	res.TxSignature = newTxSignatures(comp, txSignInputs)
	res.EcRecover = newEcRecover(comp, inputs.Settings, inputs.EcSource)
	res.UnalignedGnarkData = newUnalignedGnarkData(comp, size, res.unalignedGnarkDataSource())
	res.Addresses = newAddress(comp, size, res.EcRecover, res, inputs.TxSource)
	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_GNARK_DATA,
		Round:              ROUND_NR,
		DataToCircuit:      res.UnalignedGnarkData.GnarkData,
		DataToCircuitMask:  res.IsPushing,
		Circuit:            newMultiEcRecoverCircuit(settings.NbInputInstance),
		PlonkOptions:       inputs.PlonkOptions,
		NbCircuitInstances: settings.NbCircuitInstances,
		InputFillerKey:     plonkInputFillerKey,
	}

	res.AlignedGnarkData = plonk.DefineAlignment(comp, toAlign)

	// root module constraints
	res.csIsActiveActivation(comp)
	res.csZeroWhenInactive(comp)
	res.csConsistentPushingFetching(comp)
	res.csIDSequential(comp)
	res.csSource(comp)
	res.csTransitions(comp)

	// consistency with submodules
	// ecrecover
	res.EcRecover.csConstrainAuxProjectionMaskConsistency(comp, res.Source, res.IsFetching)

	// assign keccak providers
	res.Providers = append([]generic.GenericByteModule{res.Addresses.Provider}, res.TxSignature.Provider)

	return res
}

// assign assings all values in the antichamber. This includes assignment of all
// the submodules:
//   - EcRecover
//   - TxSignature
//   - Addresses
//   - UnalignedGnarkData
//   - AlignedGnarkData
//
// As the initial data is copied from the EC_DATA arithmetization module, then
// it has to be provided as an input.
func (ac *antichamber) assign(run *wizard.ProverRuntime, txGet TxSignatureGetter, nbTx int) {

	var (
		ecSrc             = ac.Inputs.EcSource
		txSource          = ac.Inputs.TxSource
		nbActualEcRecover = ecSrc.nbActualInstances(run)
	)

	ac.assignAntichamber(run, nbActualEcRecover, nbTx)
	ac.EcRecover.Assign(run, ecSrc)
	ac.TxSignature.assignTxSignature(run, nbActualEcRecover)
	ac.UnalignedGnarkData.Assign(run, ac.unalignedGnarkDataSource(), txGet)
	ac.Addresses.assignAddress(run, nbActualEcRecover, ac.Size, ac, ac.EcRecover, ac.UnalignedGnarkData, txSource)
	ac.AlignedGnarkData.Assign(run)
}

// assignAntichamber assigns the values values in the main part of the antichamber, namely columns:
//   - IsActive
//   - ID
//   - IsPushing
//   - IsFetching
//   - Source
//
// The assignment depends on the number of defined EcRecover and TxSignature instances.
func (ac *antichamber) assignAntichamber(run *wizard.ProverRuntime, nbEcRecInstances, nbTxInstances int) {

	var (
		maxNbEcRecover = ac.Inputs.Settings.MaxNbEcRecover
		maxNbTx        = ac.Inputs.Settings.MaxNbTx

		// Calculate the Logical Limit (The "Contract")
		// This is the maximum row space we claimed we would need in your config.
		// The circuit is built to exactly this capacity.
		configuredLimitRows = nbRowsPerEcRec*maxNbEcRecover + nbRowsPerTxSign*maxNbTx

		// Calculate Actual Usage (The "Reality") - This is how many rows your trace actually demands.
		actualUsageRows = nbRowsPerEcRec*nbEcRecInstances + nbRowsPerTxSign*nbTxInstances
	)

	if nbRowsPerEcRec*maxNbEcRecover+nbRowsPerTxSign*maxNbTx > ac.Size {
		exit.OnLimitOverflow(
			ac.Size,
			nbRowsPerEcRec*maxNbEcRecover+nbRowsPerTxSign*maxNbTx,
			fmt.Errorf("not enough space in ECDSA antichamber to store all the data. Need %d, got %d",
				nbRowsPerEcRec*maxNbEcRecover+nbRowsPerTxSign*maxNbTx, ac.Size,
			),
		)
	}

	// prepare root module columns
	// for ecrecover case we need 10+14 rows (fetchin and pushing). For TX we need 1+14
	if nbTxInstances*nbRowsPerTxSign+nbEcRecInstances*nbRowsPerEcRec > ac.Size {
		exit.OnLimitOverflow(
			ac.Size,
			nbTxInstances*nbRowsPerTxSign+nbEcRecInstances*nbRowsPerEcRec,
			fmt.Errorf("not enough space in ECDSA antichamber to store all the data. Need %d, got %d",
				nbTxInstances*nbRowsPerTxSign+nbEcRecInstances*nbRowsPerEcRec, ac.Size,
			),
		)
	}

	// This catches the case where the data fits in the physical buffer (ac.Size)
	// but exceeds the logical capacity the circuit was built for.
	if actualUsageRows > configuredLimitRows {
		exit.OnLimitOverflow(
			configuredLimitRows,
			actualUsageRows,
			fmt.Errorf("ECDSA antichamber row limit exceeded: trace requires %d rows (EcRec:%d, Tx:%d), but config limits to %d rows (MaxEcRec:%d, MaxTx:%d)",
				actualUsageRows, nbEcRecInstances, nbTxInstances, configuredLimitRows, maxNbEcRecover, maxNbTx),
		)
	}

	// allocate the columns for preparing the assignment
	resIsActive := make([]field.Element, ac.Size)
	resID := make([]field.Element, ac.Size)
	resIsPushing := make([]field.Element, ac.Size)
	resIsFetching := make([]field.Element, ac.Size)
	resSource := make([]field.Element, ac.Size)

	var idxInstance uint64
	for i := 0; i < nbEcRecInstances; i++ {
		for j := 0; j < nbRowsPerEcRec; j++ {
			resIsActive[i*nbRowsPerEcRec+j] = field.NewElement(1)
			resID[i*nbRowsPerEcRec+j] = field.NewElement(idxInstance)
			resSource[i*nbRowsPerEcRec+j].Set(&SOURCE_ECRECOVER)
		}
		for j := 0; j < nbRowsPerEcRecFetching; j++ {
			resIsFetching[i*nbRowsPerEcRec+j] = field.NewElement(1)
		}
		for j := 0; j < nbRowsPerGnarkPushing; j++ {
			resIsPushing[i*nbRowsPerEcRec+nbRowsPerEcRecFetching+j] = field.NewElement(1)
		}
		idxInstance++
	}

	for i := 0; i < nbTxInstances; i++ {
		for j := 0; j < nbRowsPerTxSign; j++ {
			resIsActive[nbRowsPerEcRec*nbEcRecInstances+i*nbRowsPerTxSign+j] = field.NewElement(1)
			resID[nbRowsPerEcRec*nbEcRecInstances+i*nbRowsPerTxSign+j] = field.NewElement(idxInstance)
			resSource[nbRowsPerEcRec*nbEcRecInstances+i*nbRowsPerTxSign+j].Set(&SOURCE_TX)
		}
		resIsFetching[nbRowsPerEcRec*nbEcRecInstances+i*nbRowsPerTxSign] = field.NewElement(1)
		for j := 0; j < nbRowsPerGnarkPushing; j++ {
			resIsPushing[nbRowsPerEcRec*nbEcRecInstances+i*nbRowsPerTxSign+nbRowsPerTxSignFetching+j] = field.NewElement(1)
		}
		idxInstance++
	}

	run.AssignColumn(ac.IsActive.GetColID(), smartvectors.NewRegular(resIsActive))
	run.AssignColumn(ac.ID.GetColID(), smartvectors.NewRegular(resID))
	run.AssignColumn(ac.IsPushing.GetColID(), smartvectors.NewRegular(resIsPushing))
	run.AssignColumn(ac.IsFetching.GetColID(), smartvectors.NewRegular(resIsFetching))
	run.AssignColumn(ac.Source.GetColID(), smartvectors.NewRegular(resSource))
}

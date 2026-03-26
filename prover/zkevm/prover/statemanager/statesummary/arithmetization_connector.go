package statesummary

import (
	"slices"
	"sync"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// arithmetizationLink collects columns from the hub that are of interest for
// checking consistency between the StateSummary and the rest of the
// arithmetization.
type arithmetizationLink struct {
	Acp, Scp    HubColumnSet
	ScpSelector ScpSelector
}

// ConnectToHub generates all the constraints attesting that the state-summary
// and the Hub relates to the same state operations.
func (ss *Module) ConnectToHub(comp *wizard.CompiledIOP, acp, scp HubColumnSet) {

	al := &arithmetizationLink{
		Acp:         acp,
		Scp:         scp,
		ScpSelector: newScpSelector(comp, scp),
	}

	storageIntegrationDefineInitial(comp, *ss, scp, al.ScpSelector)
	storageIntegrationDefineFinal(comp, *ss, scp, al.ScpSelector)
	accountIntegrationDefineInitial(comp, *ss, acp)
	accountIntegrationDefineFinal(comp, *ss, acp)

	ss.ArithmetizationLink = al
}

func (ss *Module) assignArithmetizationLink(run *wizard.ProverRuntime) {

	storageIntegrationAssignInitial(run, *ss, ss.ArithmetizationLink.Scp)
	storageIntegrationAssignFinal(run, *ss, ss.ArithmetizationLink.Scp)
	accountIntegrationAssignInitial(run, *ss, ss.ArithmetizationLink.Acp)
	accountIntegrationAssignFinal(run, *ss, ss.ArithmetizationLink.Acp)

	// @alex: this should be commonized utility or should be simplified to not
	// use a closure because the closure is used only once.
	runConcurrent := func(pas []wizard.ProverAction) {
		wg := &sync.WaitGroup{}
		for _, pa := range pas {
			wg.Add(1)
			go func(pa wizard.ProverAction) {
				pa.Run(run)
				wg.Done()
			}(pa)
		}

		wg.Wait()
	}

	var arithActions []wizard.ProverAction
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorSTKeyDiffHi[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorSTKeyDiffLo[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorBlockNoDiff[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorMinDeplBlock[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorMaxDeplBlock[:]...)
	arithActions = append(arithActions,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorAccountAddressDiff,
	)

	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueHi[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueLo[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueNextHi[:]...)
	arithActions = append(arithActions, ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueNextLo[:]...)

	runConcurrent(arithActions)

}

/*
HubColumnSet is a struct that corresponds to the HUB columns present in the ACP (account consistency permutation)
and the SCP (storage consistency permutation)
*/
type HubColumnSet struct {
	// account data
	AddressHI                                            [common.NbLimbU32]ifaces.Column
	AddressLO                                            [common.NbLimbU128]ifaces.Column
	Nonce, NonceNew                                      [common.NbLimbU64]ifaces.Column
	CodeHashHI, CodeHashLO, CodeHashHINew, CodeHashLONew [common.NbLimbU128]ifaces.Column
	CodeSizeOld, CodeSizeNew                             [common.NbLimbU64]ifaces.Column
	BalanceOld, BalanceNew                               [common.NbLimbU256]ifaces.Column
	// storage data
	KeyHI, KeyLO                                       [common.NbLimbU128]ifaces.Column
	ValueHICurr, ValueLOCurr, ValueHINext, ValueLONext [common.NbLimbU128]ifaces.Column
	// helper numbers
	DeploymentNumber, DeploymentNumberInf [common.NbLimbU32]ifaces.Column
	BlockNumber                           [common.NbLimbU64]ifaces.Column
	// helper columns
	Exists, ExistsNew ifaces.Column
	PeekAtAccount     ifaces.Column
	PeekAtStorage     ifaces.Column
	// first and last marker columns
	FirstAOC, LastAOC ifaces.Column
	FirstKOC, LastKOC ifaces.Column
	// first and last block marker columns
	FirstAOCBlock, LastAOCBlock ifaces.Column
	FirstKOCBlock, LastKOCBlock ifaces.Column
	// block deployment
	MinDeplBlock, MaxDeplBlock [common.NbLimbU32]ifaces.Column
	// account existence information, to detect account pattern if needed
	ExistsFirstInBlock ifaces.Column
	ExistsFinalInBlock ifaces.Column
}

/*
ScpSelector contains two columns SelectorMinDeplBlock and SelectorMaxDeplBlock
These columns are 1 at indices where the deployment number is equal to MinDeplBlock/MaxDeplBlock, and 0 otherwise
*/
type ScpSelector struct {
	SelectorMinDeplBlock, SelectorMaxDeplBlock               [common.NbLimbU32]ifaces.Column
	ComputeSelectorMinDeplBlock, ComputeSelectorMaxDeplBlock [common.NbLimbU32]wizard.ProverAction
	// selectors for empty keys, current values
	SelectorEmptySTValueHi, SelectorEmptySTValueLo               [common.NbLimbU128]ifaces.Column
	ComputeSelectorEmptySTValueHi, ComputeSelectorEmptySTValueLo [common.NbLimbU128]wizard.ProverAction
	// selectors for empty keys, next values
	SelectorEmptySTValueNextHi, SelectorEmptySTValueNextLo               [common.NbLimbU128]ifaces.Column
	ComputeSelectorEmptySTValueNextHi, ComputeSelectorEmptySTValueNextLo [common.NbLimbU128]wizard.ProverAction
	// storage key difference selectors
	SelectorSTKeyDiffHi, SelectorSTKeyDiffLo               [common.NbLimbU128]ifaces.Column
	ComputeSelectorSTKeyDiffHi, ComputeSelectorSTKeyDiffLo [common.NbLimbU128]wizard.ProverAction

	// Account Address Diff
	SelectorAccountAddressDiff        ifaces.Column
	ComputeSelectorAccountAddressDiff wizard.ProverAction
	// block number key difference selectors
	SelectorBlockNoDiff        [common.NbLimbU64]ifaces.Column
	ComputeSelectorBlockNoDiff [common.NbLimbU64]wizard.ProverAction
}

// Address returns the concatenated list of the HI and Lo address columns
func (s HubColumnSet) Address() []ifaces.Column {
	return slices.Concat(s.AddressHI[:], s.AddressLO[:])
}

/*
newScpSelector creates the selector columns needed for the connector between the state summary and the HUB arithmetization
these two selectors are only defined for the arithmetization columns
*/
func newScpSelector(comp *wizard.CompiledIOP, smc HubColumnSet) ScpSelector {

	var SelectorMinDeplNoBlock [common.NbLimbU32]ifaces.Column
	var ComputeSelectorMinDeplNoBlock [common.NbLimbU32]wizard.ProverAction
	var SelectorMaxDeplNoBlock [common.NbLimbU32]ifaces.Column
	var ComputeSelectorMaxDeplNoBlock [common.NbLimbU32]wizard.ProverAction

	for i := range common.NbLimbU32 {
		SelectorMinDeplNoBlock[i], ComputeSelectorMinDeplNoBlock[i] = dedicated.IsZero(
			comp,
			sym.Sub(smc.DeploymentNumber[i], smc.MinDeplBlock[i]),
		).GetColumnAndProverAction()

		SelectorMaxDeplNoBlock[i], ComputeSelectorMaxDeplNoBlock[i] = dedicated.IsZero(
			comp,
			sym.Sub(smc.DeploymentNumber[i], smc.MaxDeplBlock[i]),
		).GetColumnAndProverAction()
	}

	// ST value selectors
	var selectorEmptySTValueHi [common.NbLimbU128]ifaces.Column
	var computeSelectorEmptySTValueHi [common.NbLimbU128]wizard.ProverAction
	var selectorEmptySTValueLo [common.NbLimbU128]ifaces.Column
	var computeSelectorEmptySTValueLo [common.NbLimbU128]wizard.ProverAction
	var selectorEmptySTValueNextHi [common.NbLimbU128]ifaces.Column
	var computeSelectorEmptySTValueNextHi [common.NbLimbU128]wizard.ProverAction
	var selectorEmptySTValueNextLo [common.NbLimbU128]ifaces.Column
	var computeSelectorEmptySTValueNextLo [common.NbLimbU128]wizard.ProverAction

	for i := range common.NbLimbU128 {
		selectorEmptySTValueHi[i], computeSelectorEmptySTValueHi[i] = dedicated.IsZero(
			comp,
			ifaces.ColumnAsVariable(smc.ValueHICurr[i]),
		).GetColumnAndProverAction()

		selectorEmptySTValueLo[i], computeSelectorEmptySTValueLo[i] = dedicated.IsZero(
			comp,
			ifaces.ColumnAsVariable(smc.ValueLOCurr[i]),
		).GetColumnAndProverAction()

		selectorEmptySTValueNextHi[i], computeSelectorEmptySTValueNextHi[i] = dedicated.IsZero(
			comp,
			ifaces.ColumnAsVariable(smc.ValueHINext[i]),
		).GetColumnAndProverAction()

		selectorEmptySTValueNextLo[i], computeSelectorEmptySTValueNextLo[i] = dedicated.IsZero(
			comp,
			ifaces.ColumnAsVariable(smc.ValueLONext[i]),
		).GetColumnAndProverAction()
	}
	// storage key diff selectors
	var selectorSTKeyDiffHi [common.NbLimbU128]ifaces.Column
	var computeSelectorSTKeyDiffHi [common.NbLimbU128]wizard.ProverAction
	var selectorSTKeyDiffLo [common.NbLimbU128]ifaces.Column
	var computeSelectorSTKeyDiffLo [common.NbLimbU128]wizard.ProverAction

	for i := range common.NbLimbU128 {
		selectorSTKeyDiffHi[i], computeSelectorSTKeyDiffHi[i] = dedicated.IsZero(
			comp,
			sym.Sub(
				smc.KeyHI[i],
				column.Shift(smc.KeyHI[i], -1),
			),
		).GetColumnAndProverAction()

		selectorSTKeyDiffLo[i], computeSelectorSTKeyDiffLo[i] = dedicated.IsZero(
			comp,
			sym.Sub(
				smc.KeyLO[i],
				column.Shift(smc.KeyLO[i], -1),
			),
		).GetColumnAndProverAction()
	}

	// compute selectors for the ethereum address difference
	SelectorAccountAddressDiff, ComputeSelectorAccountAddressDiff := dedicated.IsZero(
		comp,
		sym.Sub(
			smc.Address()[0],
			column.Shift(smc.Address()[0], -1),
		),
	).GetColumnAndProverAction()

	// compute selectors for the block number difference
	var selectorBlockNoDiff [common.NbLimbU64]ifaces.Column
	var computeSelectorBlockNoDiff [common.NbLimbU64]wizard.ProverAction

	for i := range common.NbLimbU64 {
		selectorBlockNoDiff[i], computeSelectorBlockNoDiff[i] = dedicated.IsZero(
			comp,
			sym.Sub(
				smc.BlockNumber[i],
				column.Shift(smc.BlockNumber[i], -1),
			),
		).GetColumnAndProverAction()
	}

	res := ScpSelector{
		SelectorMinDeplBlock:        SelectorMinDeplNoBlock,
		SelectorMaxDeplBlock:        SelectorMaxDeplNoBlock,
		ComputeSelectorMinDeplBlock: ComputeSelectorMinDeplNoBlock,
		ComputeSelectorMaxDeplBlock: ComputeSelectorMaxDeplNoBlock,
		// ST selectors, current
		SelectorEmptySTValueHi:        selectorEmptySTValueHi,
		SelectorEmptySTValueLo:        selectorEmptySTValueLo,
		ComputeSelectorEmptySTValueHi: computeSelectorEmptySTValueHi,
		ComputeSelectorEmptySTValueLo: computeSelectorEmptySTValueLo,
		// ST selectors, next
		SelectorEmptySTValueNextHi:        selectorEmptySTValueNextHi,
		SelectorEmptySTValueNextLo:        selectorEmptySTValueNextLo,
		ComputeSelectorEmptySTValueNextHi: computeSelectorEmptySTValueNextHi,
		ComputeSelectorEmptySTValueNextLo: computeSelectorEmptySTValueNextLo,
		// ST Key diff
		SelectorSTKeyDiffHi:        selectorSTKeyDiffHi,
		SelectorSTKeyDiffLo:        selectorSTKeyDiffLo,
		ComputeSelectorSTKeyDiffHi: computeSelectorSTKeyDiffHi,
		ComputeSelectorSTKeyDiffLo: computeSelectorSTKeyDiffLo,
		// Address Number Diff,  account address difference selectors
		SelectorAccountAddressDiff:        SelectorAccountAddressDiff,
		ComputeSelectorAccountAddressDiff: ComputeSelectorAccountAddressDiff,
		// Block Number Diff
		SelectorBlockNoDiff:        selectorBlockNoDiff,
		ComputeSelectorBlockNoDiff: computeSelectorBlockNoDiff,
	}

	return res
}

/*
accountIntegrationDefineInitial defines the bidirectional lookups used to check initial account data consistency between
a StateSummary struct corresponding to Shomei traces and a StateManagerColumns struct
(which corresponds to a permutation of the arithmetization's HUB columns, in this case an ACP—account consistency permutation)
For each block, these lookups will check the consistency of the initial account data from the Shomei traces with
the corresponding columns in the arithmetization.
*/
func accountIntegrationDefineInitial(comp *wizard.CompiledIOP, ss Module, smc HubColumnSet) {

	var (
		filterArith = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_ARITHMETIZATION",
			smc.AddressHI[0].Size(),
			true,
		)

		filterSummary = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_SUMMARY",
			ss.IsStorage.Size(),
			true,
		)

		stateSummaryTable []ifaces.Column

		arithTable []ifaces.Column
	)

	arithTable = append(arithTable, smc.Address()[:]...)
	arithTable = append(arithTable, smc.BalanceOld[:]...)
	arithTable = append(arithTable, smc.Nonce[:]...)
	arithTable = append(arithTable, smc.CodeSizeOld[:]...)
	arithTable = append(arithTable, smc.CodeHashHI[:]...)
	arithTable = append(arithTable, smc.CodeHashLO[:]...)
	arithTable = append(arithTable, smc.BlockNumber[:]...)
	arithTable = append(arithTable,
		smc.Exists,
	)

	pragmas.MarkLeftPadded(filterArith)
	// Order must match arithTable: Address, Balance, Nonce, CodeSize, CodeHashHI, CodeHashLO, BlockNumber, Exists
	stateSummaryTable = append(stateSummaryTable, ss.Account.Address[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.Balance[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.Nonce[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.CodeSize[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.ExpectedHubCodeHash.Hi[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.ExpectedHubCodeHash.Lo[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.BatchNumber[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Initial.Exists)

	comp.InsertInclusionDoubleConditional(0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_INIT_ACCOUNT",
		stateSummaryTable,
		arithTable,
		filterSummary,
		filterArith,
	)

	comp.InsertInclusionDoubleConditional(0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_INIT_ACCOUNT_REVERSED",
		arithTable,
		stateSummaryTable,
		filterArith,
		filterSummary,
	)

	// Now we define the constraints for our filters
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_ARITHMETIZATION"),
		sym.Sub(
			filterArith,
			sym.Mul(
				smc.PeekAtAccount,
				smc.FirstAOCBlock,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_SUMMARY"),
		sym.Sub(
			filterSummary,
			sym.Mul(
				ss.IsInitialDeployment,
				sym.Sub(
					1,
					ss.IsStorage,
				),
			),
		),
	)
}

/*
accountIntegrationAssignInitial assigns the columns used to check initial account
data consistency using the lookups from AccountIntegrationDefineInitial
*/
func accountIntegrationAssignInitial(run *wizard.ProverRuntime, ss Module, smc HubColumnSet) {

	svfilterArith := smartvectors.Mul(
		smc.PeekAtAccount.GetColAssignment(run),
		smc.FirstAOCBlock.GetColAssignment(run),
	)

	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_ARITHMETIZATION", svfilterArith)

	selectorNotStorage := make([]field.Element, ss.IsStorage.Size())

	for index := range selectorNotStorage {
		isStorage := ss.IsStorage.GetColAssignmentAt(run, index)
		if isStorage.IsZero() {
			selectorNotStorage[index].SetOne()
		}
	}

	svSelectorNotStorage := smartvectors.NewRegular(selectorNotStorage)
	svfilterSummary := smartvectors.Mul(svSelectorNotStorage, ss.IsInitialDeployment.GetColAssignment(run))

	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_SUMMARY", svfilterSummary)
}

/*
accountIntegrationDefineFinal defines the bidirectional lookups used to check final account data consistency between
a StateSummary struct corresponding to Shomei traces and a StateManagerColumns struct
(which corresponds to a permutation of the arithmetization's HUB columns, in this case an ACP—account consistency permutation)
For each block, these lookups will check the consistency of the final account data from the Shomei traces with
the corresponding columns in the arithmetization.
*/
func accountIntegrationDefineFinal(comp *wizard.CompiledIOP, ss Module, smc HubColumnSet) {
	filterArith := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_ARITHMETIZATION", smc.AddressHI[0].Size(), true)
	filterSummary := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_SUMMARY", ss.IsStorage.Size(), true)

	pragmas.MarkLeftPadded(filterArith)

	var stateSummaryTable []ifaces.Column
	// Order must match arithTable: Address, Balance, Nonce, CodeSize, CodeHashHI, CodeHashLO, BlockNumber, Exists
	stateSummaryTable = append(stateSummaryTable, ss.Account.Address[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.Balance[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.Nonce[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.CodeSize[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.ExpectedHubCodeHash.Hi[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.ExpectedHubCodeHash.Lo[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.BatchNumber[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.Account.Final.Exists)

	var arithTable []ifaces.Column
	arithTable = append(arithTable, smc.Address()[:]...)
	arithTable = append(arithTable, smc.BalanceNew[:]...)
	arithTable = append(arithTable, smc.NonceNew[:]...)
	arithTable = append(arithTable, smc.CodeSizeNew[:]...)
	arithTable = append(arithTable, smc.CodeHashHINew[:]...)
	arithTable = append(arithTable, smc.CodeHashLONew[:]...)
	arithTable = append(arithTable, smc.BlockNumber[:]...)
	arithTable = append(arithTable, smc.ExistsNew)

	comp.InsertInclusionDoubleConditional(0, "LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_FINAL_ACCOUNT", stateSummaryTable, arithTable, filterSummary, filterArith)
	comp.InsertInclusionDoubleConditional(0, "LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_FINAL_ACCOUNT_REVERSED", arithTable, stateSummaryTable, filterArith, filterSummary)

	// Now we define the constraints for our filters
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_ARITHMETIZATION"),
		sym.Sub(
			filterArith,
			sym.Mul(
				smc.PeekAtAccount,
				smc.LastAOCBlock,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_SUMMARY"),
		sym.Sub(
			filterSummary,
			sym.Mul(
				ss.IsFinalDeployment,
				sym.Sub(
					1,
					ss.IsStorage,
				),
			),
		),
	)
}

/*
accountIntegrationAssignFinal assigns the columns used to check initial account data consistency using the lookups from accountIntegrationAssignFinal
*/
func accountIntegrationAssignFinal(run *wizard.ProverRuntime, ss Module, smc HubColumnSet) {
	filterArith := smartvectors.Mul(
		smc.PeekAtAccount.GetColAssignment(run),
		smc.LastAOCBlock.GetColAssignment(run),
	)

	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_ARITHMETIZATION", filterArith)

	selectorNotStorage := make([]field.Element, ss.IsStorage.Size())

	for index := range selectorNotStorage {
		isStorage := ss.IsStorage.GetColAssignmentAt(run, index)
		if isStorage.IsZero() {
			selectorNotStorage[index].SetOne()
		}
	}

	svSelectorNotStorage := smartvectors.NewRegular(selectorNotStorage)
	svfilterSummary := smartvectors.Mul(svSelectorNotStorage, ss.IsFinalDeployment.GetColAssignment(run))

	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_SUMMARY", svfilterSummary)
}

/*
storageIntegrationDefineInitial defines the bidirectional lookups used to check initial storage data consistency between
a StateSummary struct corresponding to Shomei traces and a StateManagerColumns struct
(which corresponds to a permutation of the arithmetization's HUB columns, in this case an SCP—storage consistency permutation)
For each block, these lookups will check the consistency of the initial storage data from the Shomei traces with
the corresponding columns in the arithmetization.
*/
func storageIntegrationDefineInitial(comp *wizard.CompiledIOP, ss Module, smc HubColumnSet, sc ScpSelector) {
	filterSummary := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_SUMMARY", ss.Account.Address[0].Size(), true)
	filterArith := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION", smc.AddressHI[0].Size(), true)
	filterArithReversed := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION_REVERSED", smc.AddressHI[0].Size(), true)

	pragmas.MarkLeftPadded(filterArith)
	pragmas.MarkLeftPadded(filterArithReversed)

	var summaryTable []ifaces.Column
	// Order must match arithTable: Address, KeyHI, KeyLO, ValueHICurr, ValueLOCurr, BlockNumber
	summaryTable = append(summaryTable, ss.Account.Address[:]...)
	summaryTable = append(summaryTable, ss.Storage.Key.Hi[:]...)
	summaryTable = append(summaryTable, ss.Storage.Key.Lo[:]...)
	summaryTable = append(summaryTable, ss.Storage.OldValue.Hi[:]...)
	summaryTable = append(summaryTable, ss.Storage.OldValue.Lo[:]...)
	summaryTable = append(summaryTable, ss.BatchNumber[:]...)

	var arithTable []ifaces.Column
	arithTable = append(arithTable, smc.Address()[:]...)
	arithTable = append(arithTable, smc.KeyHI[:]...)
	arithTable = append(arithTable, smc.KeyLO[:]...)
	arithTable = append(arithTable, smc.ValueHICurr[:]...)
	arithTable = append(arithTable, smc.ValueLOCurr[:]...)
	arithTable = append(arithTable, smc.BlockNumber[:]...)
	comp.InsertInclusionDoubleConditional(
		0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_INIT_STORAGE",
		summaryTable,
		arithTable,
		filterSummary,
		filterArith,
	)
	comp.InsertInclusionDoubleConditional(
		0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_INIT_STORAGE_REVERSE",
		arithTable,
		summaryTable,
		filterArithReversed,
		filterSummary,
	)

	filterAccountInsert := defineInsertionFilterForFinalStorage(comp, smc, sc)
	filterEphemeralAccounts := defineEphemeralAccountFilterStorage(comp, smc, sc)
	// Now we define the constraints for our filters
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION"),
		sym.Sub(
			filterArith,
			sym.Mul(
				sc.SelectorMinDeplBlock[0],
				sc.SelectorMinDeplBlock[1],
				smc.PeekAtStorage,
				smc.FirstKOCBlock,
				filterAccountInsert,
				filterEphemeralAccounts,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION_REVERSED"),
		sym.Sub(
			filterArithReversed,
			sym.Mul(
				smc.PeekAtStorage,
				smc.FirstKOCBlock,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_SUMMARY"),
		sym.Sub(
			filterSummary,
			sym.Mul(
				ss.IsStorage,
				ss.IsInitialDeployment,
			),
		),
	)
}

/*
storageIntegrationAssignInitial assigns the columns used to check initial storage data consistency using the lookups from StorageIntegrationDefineInitial
*/
func storageIntegrationAssignInitial(run *wizard.ProverRuntime, ss Module, smc HubColumnSet) {
	filterSummary := smartvectors.Mul(ss.IsStorage.GetColAssignment(run), ss.IsInitialDeployment.GetColAssignment(run))
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_SUMMARY", filterSummary)

	selectorMinDeplBlock := make([]field.Element, smc.AddressHI[0].Size())

	for index := range selectorMinDeplBlock {
		var deplEqual [common.NbLimbU32]bool
		for i := range common.NbLimbU32 {
			minDeplBlock := smc.MinDeplBlock[i].GetColAssignmentAt(run, index)
			deplNumber := smc.DeploymentNumber[i].GetColAssignmentAt(run, index)
			deplEqual[i] = minDeplBlock.Equal(&deplNumber)
		}

		if deplEqual[0] && deplEqual[1] {
			selectorMinDeplBlock[index].SetOne()
		}
	}
	svSelectorMinDeplBlock := smartvectors.NewRegular(selectorMinDeplBlock)

	filterAccountInsert := assignInsertionFilterForStorage(run, smc)
	filterEphemeralAccounts := assignEphemeralAccountFilterStorage(run, smc)

	filterArith := smartvectors.Mul(
		svSelectorMinDeplBlock,
		smc.PeekAtStorage.GetColAssignment(run),
		smc.FirstKOCBlock.GetColAssignment(run),
		filterAccountInsert,
		filterEphemeralAccounts,
	)
	run.AssignColumn(
		"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION",
		filterArith,
	)

	/*
		When looking up with including = {arithmetization} and included = {State summary}, we remove the MinDeplBlock filter selector
		(arithmetization keys might be read after the first deployment in the block)
	*/
	filterArithReversed := smartvectors.Mul(
		smc.PeekAtStorage.GetColAssignment(run),
		smc.FirstKOCBlock.GetColAssignment(run),
	)
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION_REVERSED", filterArithReversed)
}

/*
storageIntegrationDefineFinal defines the bidirectional lookups used to check final storage data consistency between
a StateSummary struct corresponding to Shomei traces and a StateManagerColumns struct
(which corresponds to a permutation of the arithmetization's HUB columns, in this case an SCP—storage consistency permutation)
For each block, these lookups will check the consistency of the final storage data from the Shomei traces with
the corresponding columns in the arithmetization.
*/
func storageIntegrationDefineFinal(comp *wizard.CompiledIOP, ss Module, smc HubColumnSet, sc ScpSelector) {

	var (
		summaryTable []ifaces.Column

		arithTable = smc.Address()[:]

		filterArith = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION",
			smc.AddressHI[0].Size(),
			true,
		)

		filterArithReversed = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION_REVERSED",
			smc.AddressHI[0].Size(),
			true,
		)

		filterSummary = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY",
			ss.Account.Address[0].Size(),
			true,
		)

		filterSummaryReversed = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY_REVERSED",
			ss.Account.Address[0].Size(),
			true,
		)

		filterAccountInsert     = comp.Columns.GetHandle("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER")
		filterEphemeralAccounts = comp.Columns.GetHandle("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER")
		filterAccountDelete     = defineDeletionFilterForShomeiStorage(comp, ss)
	)

	pragmas.MarkLeftPadded(filterArith)
	pragmas.MarkLeftPadded(filterArithReversed)
	arithTable = append(arithTable, smc.KeyHI[:]...)
	arithTable = append(arithTable, smc.KeyLO[:]...)
	arithTable = append(arithTable, smc.ValueHINext[:]...)
	arithTable = append(arithTable, smc.ValueLONext[:]...)

	summaryTable = append(summaryTable, ss.Account.Address[:]...)
	summaryTable = append(summaryTable, ss.Storage.Key.Hi[:]...)
	summaryTable = append(summaryTable, ss.Storage.Key.Lo[:]...)
	summaryTable = append(summaryTable, ss.Storage.NewValue.Hi[:]...)
	summaryTable = append(summaryTable, ss.Storage.NewValue.Lo[:]...)

	comp.InsertInclusionDoubleConditional(0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_FINAL_STORAGE",
		summaryTable,
		arithTable,
		filterSummary,
		filterArith,
	)

	comp.InsertInclusionDoubleConditional(0,
		"LOOKUP_STATE_MGR_ARITH_TO_STATE_SUMMARY_FINAL_STORAGE_REVERSED",
		arithTable,
		summaryTable,
		filterArithReversed,
		filterSummaryReversed,
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION"),
		sym.Sub(
			filterArith,
			sym.Mul(
				sc.SelectorMaxDeplBlock[0],
				sc.SelectorMaxDeplBlock[1],
				smc.PeekAtStorage,
				smc.LastKOCBlock,
				filterAccountInsert,
				filterEphemeralAccounts,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_REVERSED_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION"),
		sym.Sub(
			filterArithReversed,
			sym.Mul(
				sc.SelectorMaxDeplBlock[0],
				sc.SelectorMaxDeplBlock[1],
				smc.PeekAtStorage,
				smc.LastKOCBlock,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY"),
		sym.Sub(
			filterSummary,
			sym.Mul(
				ss.IsStorage,
				ss.IsFinalDeployment,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("CONSTRAINT_FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY_REVERSED"),
		sym.Sub(
			filterSummaryReversed,
			sym.Mul(
				ss.IsStorage,
				ss.IsFinalDeployment,
				filterAccountDelete,
			),
		),
	)
}

/*
storageIntegrationAssignFinal assigns the columns used to check initial storage data consistency using the lookups from StorageIntegrationDefineFinal
*/
func storageIntegrationAssignFinal(run *wizard.ProverRuntime, ss Module, smc HubColumnSet) {
	selectorMaxDeplBlock := make([]field.Element, smc.AddressHI[0].Size())
	for index := range selectorMaxDeplBlock {
		var deplEqual [common.NbLimbU32]bool
		for i := range common.NbLimbU32 {
			maxDeplBlock := smc.MaxDeplBlock[i].GetColAssignmentAt(run, index)
			deplNumber := smc.DeploymentNumber[i].GetColAssignmentAt(run, index)
			deplEqual[i] = maxDeplBlock.Equal(&deplNumber)
		}
		if deplEqual[0] && deplEqual[1] {
			selectorMaxDeplBlock[index].SetOne()
		}
	}
	svSelectorMaxDeplBlock := smartvectors.NewRegular(selectorMaxDeplBlock)

	filterSummary := smartvectors.Mul(
		ss.IsStorage.GetColAssignment(run),
		ss.IsFinalDeployment.GetColAssignment(run),
	)
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY", filterSummary)

	// assign the insertion, deletion and deletion filters
	filterAccountInsert := run.GetColumn("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER")
	filterEphemeralAccounts := run.GetColumn("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER")
	filterAccountDelete := assignDeletionFilterForShomeiStorage(run, ss)

	// assign the filter on Shomei for the reverse lookup
	filterSummaryReversed := smartvectors.Mul(
		ss.IsStorage.GetColAssignment(run),
		ss.IsFinalDeployment.GetColAssignment(run),
		filterAccountDelete,
	)
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY_REVERSED", filterSummaryReversed)

	filterArith := smartvectors.Mul(
		svSelectorMaxDeplBlock,
		smc.PeekAtStorage.GetColAssignment(run),
		smc.LastKOCBlock.GetColAssignment(run),
		filterAccountInsert,
		filterEphemeralAccounts,
	)
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION", filterArith)

	filterArithReversed := smartvectors.Mul(
		svSelectorMaxDeplBlock,
		smc.PeekAtStorage.GetColAssignment(run),
		smc.LastKOCBlock.GetColAssignment(run),
	)
	run.AssignColumn("FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION_REVERSED", filterArithReversed)

}

/*
defineInsertionFilterForFinalStorage defines an insertion filter for the edge case of
missing storage keys that get created but then wiped when an account that did not exist is
not added to the state
*/
func defineInsertionFilterForFinalStorage(comp *wizard.CompiledIOP, smc HubColumnSet, sc ScpSelector) ifaces.Column {
	// create the filter
	filterAccountInsert := comp.InsertCommit(0,
		"FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER",
		smc.Address()[0].Size(),
		true,
	)

	pragmas.MarkLeftPadded(filterAccountInsert)

	// constraint the insertion selector filter
	// on storage rows, we enforce that filterAccountInsert is 0 then (existsFirstInBlock = 0 and existsFinalInBlock = 1)
	// security of the following constraint relies on the fact that the underlying marker columns are binary
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER"),
		sym.Mul(
			smc.PeekAtStorage, // when we are dealing with storage segments
			sym.Mul(
				sym.Sub(
					1,
					filterAccountInsert,
				), // if  filterAccountInsert = 0 it must be that the conditions of the filter are both satisfied
				sym.Add( // the addition must sum up to 0
					smc.ExistsFirstInBlock,
					sym.Sub(
						1,
						smc.ExistsFinalInBlock,
					),
				),
			),
		),
	)

	selectorEmptySTValue := sym.Mul(
		sc.SelectorEmptySTValueHi[0], sc.SelectorEmptySTValueHi[1],
		sc.SelectorEmptySTValueHi[2], sc.SelectorEmptySTValueHi[3],
		sc.SelectorEmptySTValueHi[4], sc.SelectorEmptySTValueHi[5],
		sc.SelectorEmptySTValueHi[6], sc.SelectorEmptySTValueHi[7],
		sc.SelectorEmptySTValueLo[0], sc.SelectorEmptySTValueLo[1],
		sc.SelectorEmptySTValueLo[2], sc.SelectorEmptySTValueLo[3],
		sc.SelectorEmptySTValueLo[4], sc.SelectorEmptySTValueLo[5],
		sc.SelectorEmptySTValueLo[6], sc.SelectorEmptySTValueLo[7],
	)
	// if the filter is set to 0, then all the emoty value selectors must be 1.
	// but this only must be true for the last values seen in the relevant segment.
	// afterwards the keys are allowed to fluctuate
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER_VALUE_ZEROIZATION"),
		sym.Mul(
			smc.PeekAtStorage,
			smc.LastKOCBlock, // Very important, only check the wiping on the last row of the storage key
			sym.Sub(
				1,
				filterAccountInsert,
			),
			sym.Sub(1, selectorEmptySTValue),
		),
	)

	stKeyIsSame := sym.Mul(
		sc.SelectorSTKeyDiffHi[0], sc.SelectorSTKeyDiffHi[1],
		sc.SelectorSTKeyDiffHi[2], sc.SelectorSTKeyDiffHi[3],
		sc.SelectorSTKeyDiffHi[4], sc.SelectorSTKeyDiffHi[5],
		sc.SelectorSTKeyDiffHi[6], sc.SelectorSTKeyDiffHi[7],
		sc.SelectorSTKeyDiffLo[0], sc.SelectorSTKeyDiffLo[1],
		sc.SelectorSTKeyDiffLo[2], sc.SelectorSTKeyDiffLo[3],
		sc.SelectorSTKeyDiffLo[4], sc.SelectorSTKeyDiffLo[5],
		sc.SelectorSTKeyDiffLo[6], sc.SelectorSTKeyDiffLo[7],
	)

	blockNumIsSame := sym.Mul(
		sc.SelectorBlockNoDiff[0], sc.SelectorBlockNoDiff[1],
		sc.SelectorBlockNoDiff[2], sc.SelectorBlockNoDiff[3],
	)

	// filter must be constant as long as the storage key does not change
	// and the address and block number also does not change
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_HUB_STATE_SUMMARY__ACCOUNT_INSERT_FILTER_CONSTANCY"),
		sym.Mul(
			stKeyIsSame,                   // 1 if ST key LO is the same as in the previous index
			sc.SelectorAccountAddressDiff, // 1 if the account address is the same, meaning that our storage segment is within the same account segment
			blockNumIsSame,                // 1 if the block number is the same, meaning that we are in the same storage key segment
			sym.Sub(
				filterAccountInsert,
				column.Shift(filterAccountInsert, -1), // the filter remains constant if the ST key is the same, account address, and block is the same
			),
		),
	)
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER_NON_ZEROIZATION"),
		sym.Mul(
			sym.Sub(
				1,
				smc.PeekAtStorage,
			), // when we are not dealing with storage segments
			sym.Sub(
				1,
				filterAccountInsert,
			), // filterAccountInsert must be 1
		),
	)
	// constrain the filter to be binary
	mustBeBinary(comp, filterAccountInsert)

	return filterAccountInsert
}

/*
assignInsertionFilterForStorage assigns the insertion filter for the edge case of
missing storage keys that get created but then wiped when an account that did not exist is
not added to the state
*/
func assignInsertionFilterForStorage(run *wizard.ProverRuntime, smc HubColumnSet) smartvectors.SmartVector {
	// compute the filter that detects account inserts in order to exclude those key reads from the
	// arithmetization to state summary lookups.
	filterAccountInsert := make([]field.Element, smc.AddressHI[0].Size())
	lastSegmentStart := 0
	for index := range filterAccountInsert {
		filterAccountInsert[index].SetOne() // always set the filter as one, unless we detect an insertion segment
		isStorage := smc.PeekAtStorage.GetColAssignmentAt(run, index)
		if isStorage.IsOne() {
			firstKOCBlock := smc.FirstKOCBlock.GetColAssignmentAt(run, index)
			lastKOCBlock := smc.LastKOCBlock.GetColAssignmentAt(run, index)
			existsAtBlockEnd := smc.ExistsFinalInBlock.GetColAssignmentAt(run, index)

			if firstKOCBlock.IsOne() {
				// remember when the segment starts
				lastSegmentStart = index
			}
			if lastKOCBlock.IsOne() && existsAtBlockEnd.IsOne() {
				existsAtBlockStart := smc.ExistsFirstInBlock.GetColAssignmentAt(run, lastSegmentStart)
				if existsAtBlockStart.IsZero() {
					// we are indeed dealing with an insertion segment
					// now check if indeed all the storage values on the last row are 0
					valueNextHi := smc.ValueHINext[0].GetColAssignmentAt(run, index)
					valueNextLo := smc.ValueLONext[0].GetColAssignmentAt(run, index)
					if valueNextHi.IsZero() && valueNextLo.IsZero() {
						// we are indeed dealing with an insertion segment, check if indeed all the storage values are 0
						allStorageIsZero := true
						for j := lastSegmentStart; j <= index; j++ {
							for i := range common.NbLimbU128 {
								valueCurrentHi := smc.ValueHICurr[i].GetColAssignmentAt(run, j)
								valueCurrentLo := smc.ValueLOCurr[i].GetColAssignmentAt(run, j)
								valueNextHi := smc.ValueHINext[i].GetColAssignmentAt(run, j)
								valueNextLo := smc.ValueLONext[i].GetColAssignmentAt(run, j)

								if !valueCurrentHi.IsZero() || !valueCurrentLo.IsZero() || !valueNextHi.IsZero() || !valueNextLo.IsZero() {
									allStorageIsZero = false
									break
								}
							}
						}

						if allStorageIsZero {
							// indeed we are dealing with a zeroed insertion segment
							for j := lastSegmentStart; j <= index; j++ {
								// set the filter to zeros on the insertion segment
								filterAccountInsert[j].SetZero()
							}
						}

					}
				}
			}

		}
	}
	svfilterAccountInsert := smartvectors.NewRegular(filterAccountInsert)
	run.AssignColumn("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER", svfilterAccountInsert)
	return svfilterAccountInsert

}

/*
defineEphemeralAccountFilterStorage defines an ephemeral filter for the edge case of
missing storage keys that belogn to accounts which do not exist at the beginning/end of
a block, but exist in-between
*/
func defineEphemeralAccountFilterStorage(comp *wizard.CompiledIOP, smc HubColumnSet, sc ScpSelector) ifaces.Column {
	// create the filter
	filterEphemeralAccounts := comp.InsertCommit(0,
		"FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER",
		smc.AddressHI[0].Size(),
		true,
	)

	pragmas.MarkLeftPadded(filterEphemeralAccounts)

	// constraint the ephemeral selector filter
	// on storage rows, we enforce that filterEphemeralAccounts is 0 then (existsFirstInBlock = 0 and existsFinalInBlock = 0)
	// security of the following constraint relies on the fact that the underlying marker columns are binary
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER"),
		sym.Mul(
			smc.PeekAtStorage, // when we are dealing with storage segments
			sym.Mul(
				sym.Sub(
					1,
					filterEphemeralAccounts,
				), // if  filterEphemeralAccounts = 0 it must be that the conditions of the filter are both satisfied
				sym.Add(
					smc.ExistsFirstInBlock,
					smc.ExistsFinalInBlock,
				),
			),
		),
	)

	stKeyIsSame := sym.Mul(
		sc.SelectorSTKeyDiffHi[0], sc.SelectorSTKeyDiffHi[1],
		sc.SelectorSTKeyDiffHi[2], sc.SelectorSTKeyDiffHi[3],
		sc.SelectorSTKeyDiffHi[4], sc.SelectorSTKeyDiffHi[5],
		sc.SelectorSTKeyDiffHi[6], sc.SelectorSTKeyDiffHi[7],
		sc.SelectorSTKeyDiffLo[0], sc.SelectorSTKeyDiffLo[1],
		sc.SelectorSTKeyDiffLo[2], sc.SelectorSTKeyDiffLo[3],
		sc.SelectorSTKeyDiffLo[4], sc.SelectorSTKeyDiffLo[5],
		sc.SelectorSTKeyDiffLo[6], sc.SelectorSTKeyDiffLo[7],
	)
	blockNumIsSame := sym.Mul(
		sc.SelectorBlockNoDiff[0], sc.SelectorBlockNoDiff[1],
		sc.SelectorBlockNoDiff[2], sc.SelectorBlockNoDiff[3],
	)
	// filter must be constant as long as the storage key does not change
	// and the address and block number also does not change
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_HUB_STATE_SUMMARY__ACCOUNT_EPHEMERAL_FILTER_CONSTANCY"),
		sym.Mul(
			stKeyIsSame,                   // 1 if ST key LO is the same as in the previous index
			sc.SelectorAccountAddressDiff, // 1 if the account address is the same, meaning that our storage segment is within the same account segment
			blockNumIsSame,                // 1 if the block number is the same, meaning that we are in the same storage key segment
			sym.Sub(
				filterEphemeralAccounts,
				column.Shift(filterEphemeralAccounts, -1), // the filter remains constant if the ST key is the same, account address, and block is the same
			),
		),
	)
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER_NON_ZEROIZATION"),
		sym.Mul(
			sym.Sub(
				1,
				smc.PeekAtStorage,
			), // when we are not dealing with storage segments
			sym.Sub(
				1,
				filterEphemeralAccounts,
			), // filterEphemeralAccounts must be 1
		),
	)

	// constrain the filter to be binary
	mustBeBinary(comp, filterEphemeralAccounts)

	return filterEphemeralAccounts
}

/*
assignEphemeralAccountFilterStorage assigns a filter that removes the storage from ephemeral accounts
that do not exist at the beginning&end of a block, but are deployed inside.
The filter will be 0 when the keys must be removed, and 1 elsewhere
*/
func assignEphemeralAccountFilterStorage(run *wizard.ProverRuntime, smc HubColumnSet) smartvectors.SmartVector {
	// compute the filter that detects storage keys of ephemeral accounts to exclude those key reads from the
	// arithmetization to state summary lookups.
	filterEphemeralAccounts := make([]field.Element, smc.AddressHI[0].Size())
	lastSegmentStart := 0
	for index := range filterEphemeralAccounts {
		filterEphemeralAccounts[index].SetOne() // always set the filter as one, unless we detect an insertion segment
		isStorage := smc.PeekAtStorage.GetColAssignmentAt(run, index)
		if isStorage.IsOne() {
			firstKOCBlock := smc.FirstKOCBlock.GetColAssignmentAt(run, index)
			lastKOCBlock := smc.LastKOCBlock.GetColAssignmentAt(run, index)
			existsAtBlockEnd := smc.ExistsFinalInBlock.GetColAssignmentAt(run, index)

			if firstKOCBlock.IsOne() {
				// remember when the segment starts
				lastSegmentStart = index
			}
			if lastKOCBlock.IsOne() && existsAtBlockEnd.IsZero() {
				existsAtBlockStart := smc.ExistsFirstInBlock.GetColAssignmentAt(run, lastSegmentStart)
				if existsAtBlockStart.IsZero() {
					// we are indeed dealing with an ephemeral account, that does not exist
					// at the beginning of the block, nor at the end
					// indeed we are dealing with a zeroed ephemeral segment
					for j := lastSegmentStart; j <= index; j++ {
						// set the filter to zeros on the ephemeral segment
						filterEphemeralAccounts[j].SetZero()
					}
				}

			}
		}
	}
	svFilterEphemeralAccounts := smartvectors.NewRegular(filterEphemeralAccounts)
	run.AssignColumn("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER", svFilterEphemeralAccounts)
	return svFilterEphemeralAccounts
}

// defineDeletionFilterForShomeiStorage covers a very specific edge case: normally, deleting an account
// with storage causes no problems on the Shomei vs arithmetization integration
// sometimes, there is an extra account access on the arithmetization side (not a storage access)
// this extra access (can be a balance query for instance) will correspond to an incremented deployment number, so then
// the final check cannot find the proper storage keys on the arithmetization side (since there is this trivial access for
// non-existing account, the maxDeploymentNumber = storageKeyDeploymentNumber selector will not find anything).
// The fix is to just exclude these storage keys from the check when we have a deletion filter.
func defineDeletionFilterForShomeiStorage(comp *wizard.CompiledIOP, ss Module) ifaces.Column {
	// create the filter
	filterDeletionInsert := comp.InsertCommit(0,
		"FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_DELETION_FILTER",
		ss.Account.Address[0].Size(),
		true,
	)

	// if the filter is set to 0, then all the emoty value selectors must be 1.
	// but this only must be true for the last values seen in the relevant segment.
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_DELETION_FILTER_VALUE_ZEROIZATION"),
		sym.Sub(
			1,
			filterDeletionInsert,
			ss.IsDeleteSegment,
		),
	)
	// constrain the filter to be binary
	mustBeBinary(comp, filterDeletionInsert)

	return filterDeletionInsert
}

// assignDeletionFilterForShomeiStorage computes and assigns the deletion filter
func assignDeletionFilterForShomeiStorage(run *wizard.ProverRuntime, ss Module) smartvectors.SmartVector {
	// compute the filter that detects account deletions in order to exclude those storage key accesses from the
	// state summary to arithmetization lookups.
	filterAccountDelete := make([]field.Element, ss.Account.Address[0].Size())
	for index := range filterAccountDelete {
		filterAccountDelete[index].SetOne() // always set the filter as one, unless we detect a deletion segment
		isDeleteSegment := ss.IsDeleteSegment.GetColAssignmentAt(run, index)
		if isDeleteSegment.IsOne() {
			// exclude this cell from the lookup
			filterAccountDelete[index].SetZero()
		} else {
			// otherwise, include the cell in the lookup
			filterAccountDelete[index].SetOne()
		}

	}
	svfilterAccountDelete := smartvectors.NewRegular(filterAccountDelete)
	run.AssignColumn("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_DELETION_FILTER", svfilterAccountDelete)
	return svfilterAccountDelete
}

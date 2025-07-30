package statesummary

import (
	"sync"

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

	runConcurrent([]wizard.ProverAction{
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorMinDeplBlock,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorMaxDeplBlock,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueHi,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueLo,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueNextHi,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorEmptySTValueNextLo,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorSTKeyDiffHi,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorSTKeyDiffLo,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorAccountAddressDiff,
		ss.ArithmetizationLink.ScpSelector.ComputeSelectorBlockNoDiff,
	})

}

/*
HubColumnSet is a struct that corresponds to the HUB columns present in the ACP (account consistency permutation)
and the SCP (storage consistency permutation)
*/
type HubColumnSet struct {
	// helper column
	Address ifaces.Column
	// account data
	AddressHI, AddressLO                                 ifaces.Column
	Nonce, NonceNew                                      ifaces.Column
	CodeHashHI, CodeHashLO, CodeHashHINew, CodeHashLONew ifaces.Column
	CodeSizeOld, CodeSizeNew                             ifaces.Column
	BalanceOld, BalanceNew                               ifaces.Column
	// storage data
	KeyHI, KeyLO                                       ifaces.Column
	ValueHICurr, ValueLOCurr, ValueHINext, ValueLONext ifaces.Column
	// helper numbers
	DeploymentNumber, DeploymentNumberInf ifaces.Column
	BlockNumber                           ifaces.Column
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
	MinDeplBlock, MaxDeplBlock ifaces.Column
	// account existence information, to detect account pattern if needed
	ExistsFirstInBlock ifaces.Column
	ExistsFinalInBlock ifaces.Column
}

/*
ScpSelector contains two columns SelectorMinDeplBlock and SelectorMaxDeplBlock
These columns are 1 at indices where the deployment number is equal to MinDeplBlock/MaxDeplBlock, and 0 otherwise
*/
type ScpSelector struct {
	SelectorMinDeplBlock, SelectorMaxDeplBlock               ifaces.Column
	ComputeSelectorMinDeplBlock, ComputeSelectorMaxDeplBlock wizard.ProverAction
	// selectors for empty keys, current values
	SelectorEmptySTValueHi, SelectorEmptySTValueLo               ifaces.Column
	ComputeSelectorEmptySTValueHi, ComputeSelectorEmptySTValueLo wizard.ProverAction
	// selectors for empty keys, next values
	SelectorEmptySTValueNextHi, SelectorEmptySTValueNextLo               ifaces.Column
	ComputeSelectorEmptySTValueNextHi, ComputeSelectorEmptySTValueNextLo wizard.ProverAction
	// storage key difference selectors
	SelectorSTKeyDiffHi, SelectorSTKeyDiffLo               ifaces.Column
	ComputeSelectorSTKeyDiffHi, ComputeSelectorSTKeyDiffLo wizard.ProverAction
	// Account Address Diff
	SelectorAccountAddressDiff        ifaces.Column
	ComputeSelectorAccountAddressDiff wizard.ProverAction
	// block number key difference selectors
	SelectorBlockNoDiff        ifaces.Column
	ComputeSelectorBlockNoDiff wizard.ProverAction
}

/*
newScpSelector creates the selector columns needed for the connector between the state summary and the HUB arithmetization
these two selectors are only defined for the arithmetization columns
*/
func newScpSelector(comp *wizard.CompiledIOP, smc HubColumnSet) ScpSelector {

	SelectorMinDeplNoBlock, ComputeSelectorMinDeplNoBlock := dedicated.IsZero(
		comp,
		sym.Sub(smc.DeploymentNumber, smc.MinDeplBlock),
	)

	SelectorMaxDeplNoBlock, ComputeSelectorMaxDeplNoBlock := dedicated.IsZero(
		comp,
		sym.Sub(smc.DeploymentNumber, smc.MaxDeplBlock),
	)

	// ST value selectors
	SelectorEmptySTValueHi, ComputeSelectorEmptySTValueHi := dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(smc.ValueHICurr),
	)

	SelectorEmptySTValueLo, ComputeSelectorEmptySTValueLo := dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(smc.ValueLOCurr),
	)
	SelectorEmptySTValueNextHi, ComputeSelectorEmptySTValueNextHi := dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(smc.ValueHINext),
	)

	SelectorEmptySTValueNextLo, ComputeSelectorEmptySTValueNextLo := dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(smc.ValueLONext),
	)
	// storage key diff selectors
	SelectorSTKeyDiffHi, ComputeSelectorSTKeyDiffHi := dedicated.IsZero(
		comp,
		sym.Sub(
			smc.KeyHI,
			column.Shift(smc.KeyHI, -1),
		),
	)
	SelectorSTKeyDiffLo, ComputeSelectorSTKeyDiffLo := dedicated.IsZero(
		comp,
		sym.Sub(
			smc.KeyLO,
			column.Shift(smc.KeyLO, -1),
		),
	)

	// compute selectors for the ethereum address difference
	SelectorAccountAddressDiff, ComputeSelectorAccountAddressDiff := dedicated.IsZero(
		comp,
		sym.Sub(
			smc.Address,
			column.Shift(smc.Address, -1),
		),
	)

	// compute selectors for the block number difference
	SelectorBlockNoDiff, ComputeSelectorBlockNoDiff := dedicated.IsZero(
		comp,
		sym.Sub(
			smc.BlockNumber,
			column.Shift(smc.BlockNumber, -1),
		),
	)

	res := ScpSelector{
		SelectorMinDeplBlock:        SelectorMinDeplNoBlock,
		SelectorMaxDeplBlock:        SelectorMaxDeplNoBlock,
		ComputeSelectorMinDeplBlock: ComputeSelectorMinDeplNoBlock,
		ComputeSelectorMaxDeplBlock: ComputeSelectorMaxDeplNoBlock,
		// ST selectors, current
		SelectorEmptySTValueHi:        SelectorEmptySTValueHi,
		SelectorEmptySTValueLo:        SelectorEmptySTValueLo,
		ComputeSelectorEmptySTValueHi: ComputeSelectorEmptySTValueHi,
		ComputeSelectorEmptySTValueLo: ComputeSelectorEmptySTValueLo,
		// ST selectors, next
		SelectorEmptySTValueNextHi:        SelectorEmptySTValueNextHi,
		SelectorEmptySTValueNextLo:        SelectorEmptySTValueNextLo,
		ComputeSelectorEmptySTValueNextHi: ComputeSelectorEmptySTValueNextHi,
		ComputeSelectorEmptySTValueNextLo: ComputeSelectorEmptySTValueNextLo,
		// ST Key diff
		SelectorSTKeyDiffHi:        SelectorSTKeyDiffHi,
		SelectorSTKeyDiffLo:        SelectorSTKeyDiffLo,
		ComputeSelectorSTKeyDiffHi: ComputeSelectorSTKeyDiffHi,
		ComputeSelectorSTKeyDiffLo: ComputeSelectorSTKeyDiffLo,
		// Address Number Diff,  account address difference selectors
		SelectorAccountAddressDiff:        SelectorAccountAddressDiff,
		ComputeSelectorAccountAddressDiff: ComputeSelectorAccountAddressDiff,
		// Block Number Diff
		SelectorBlockNoDiff:        SelectorBlockNoDiff,
		ComputeSelectorBlockNoDiff: ComputeSelectorBlockNoDiff,
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
			smc.AddressHI.Size(),
		)

		filterSummary = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_INITIAL_SUMMARY",
			ss.IsStorage.Size(),
		)

		stateSummaryTable = []ifaces.Column{ss.Account.Address,
			ss.Account.Initial.Balance,
			ss.Account.Initial.Nonce,
			ss.Account.Initial.CodeSize,
			ss.Account.Initial.ExpectedHubCodeHash.Hi,
			ss.Account.Initial.ExpectedHubCodeHash.Lo,
			ss.BatchNumber,
			ss.Account.Initial.Exists,
		}

		arithTable = []ifaces.Column{smc.Address,
			smc.BalanceOld,
			smc.Nonce,
			smc.CodeSizeOld,
			smc.CodeHashHI,
			smc.CodeHashLO,
			smc.BlockNumber,
			smc.Exists,
		}
	)

	pragmas.MarkLeftPadded(filterArith)

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
	filterArith := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_ARITHMETIZATION", smc.AddressHI.Size())
	filterSummary := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_ACCOUNT_FINAL_SUMMARY", ss.IsStorage.Size())

	pragmas.MarkLeftPadded(filterArith)

	stateSummaryTable := []ifaces.Column{
		ss.Account.Address,
		ss.Account.Final.Balance,
		ss.Account.Final.Nonce,
		ss.Account.Final.CodeSize,
		ss.Account.Final.ExpectedHubCodeHash.Hi,
		ss.Account.Final.ExpectedHubCodeHash.Lo,
		ss.BatchNumber,
		ss.Account.Final.Exists,
	}
	arithTable := []ifaces.Column{
		smc.Address,
		smc.BalanceNew,
		smc.NonceNew,
		smc.CodeSizeNew,
		smc.CodeHashHINew,
		smc.CodeHashLONew,
		smc.BlockNumber,
		smc.ExistsNew,
	}

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
	filterSummary := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_SUMMARY", ss.Account.Address.Size())
	filterArith := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION", smc.AddressHI.Size())
	filterArithReversed := comp.InsertCommit(0, "FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_INITIAL_ARITHMETIZATION_REVERSED", smc.AddressHI.Size())

	pragmas.MarkLeftPadded(filterArith)
	pragmas.MarkLeftPadded(filterArithReversed)

	summaryTable := []ifaces.Column{
		ss.Account.Address,
		ss.Storage.Key.Hi,
		ss.Storage.Key.Lo,
		ss.Storage.OldValue.Hi,
		ss.Storage.OldValue.Lo,
		ss.BatchNumber,
	}
	arithTable := []ifaces.Column{
		smc.Address,
		smc.KeyHI,
		smc.KeyLO,
		smc.ValueHICurr,
		smc.ValueLOCurr,
		smc.BlockNumber,
	}
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
				sc.SelectorMinDeplBlock,
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

	selectorMinDeplBlock := make([]field.Element, smc.AddressHI.Size())

	for index := range selectorMinDeplBlock {
		minDeplBlock := smc.MinDeplBlock.GetColAssignmentAt(run, index)
		deplNumber := smc.DeploymentNumber.GetColAssignmentAt(run, index)
		if minDeplBlock.Equal(&deplNumber) {
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
		summaryTable = []ifaces.Column{
			ss.Account.Address,
			ss.Storage.Key.Hi,
			ss.Storage.Key.Lo,
			ss.Storage.NewValue.Hi,
			ss.Storage.NewValue.Lo,
			ss.BatchNumber,
		}

		arithTable = []ifaces.Column{smc.Address,
			smc.KeyHI,
			smc.KeyLO,
			smc.ValueHINext,
			smc.ValueLONext,
			smc.BlockNumber,
		}

		filterArith = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION",
			smc.AddressHI.Size(),
		)

		filterArithReversed = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_ARITHMETIZATION_REVERSED",
			smc.AddressHI.Size(),
		)

		filterSummary = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY",
			ss.Account.Address.Size(),
		)

		filterSummaryReversed = comp.InsertCommit(0,
			"FILTER_CONNECTOR_SUMMARY_ARITHMETIZATION_STORAGE_FINAL_SUMMARY_REVERSED",
			ss.Account.Address.Size(),
		)

		filterAccountInsert     = comp.Columns.GetHandle("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_INSERT_FILTER")
		filterEphemeralAccounts = comp.Columns.GetHandle("FILTER_CONNECTOR_HUB_STATE_SUMMARY_ACCOUNT_EPHEMERAL_FILTER")
		filterAccountDelete     = defineDeletionFilterForShomeiStorage(comp, ss)
	)

	pragmas.MarkLeftPadded(filterArith)
	pragmas.MarkLeftPadded(filterArithReversed)

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
				sc.SelectorMaxDeplBlock,
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
				sc.SelectorMaxDeplBlock,
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
	selectorMaxDeplBlock := make([]field.Element, smc.AddressHI.Size())
	for index := range selectorMaxDeplBlock {
		maxDeplBlock := smc.MaxDeplBlock.GetColAssignmentAt(run, index)
		deplNumber := smc.DeploymentNumber.GetColAssignmentAt(run, index)
		if maxDeplBlock.Equal(&deplNumber) {
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
		smc.AddressHI.Size(),
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
			sym.Sub(
				1,
				sym.Mul(
					sc.SelectorEmptySTValueNextHi,
					sc.SelectorEmptySTValueNextLo,
				),
			),
		),
	)
	// filter must be constant as long as the storage key does not change
	// and the address and block number also does not change
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_HUB_STATE_SUMMARY__ACCOUNT_INSERT_FILTER_CONSTANCY"),
		sym.Mul(
			sc.SelectorSTKeyDiffHi,        // 1 if ST key HI is the same as in the previous index
			sc.SelectorSTKeyDiffLo,        // 1 if ST key LO is the same as in the previous index
			sc.SelectorAccountAddressDiff, // 1 if the account address is the same, meaning that our storage segment is within the same account segment
			sc.SelectorBlockNoDiff,        // 1 if the block number is the same, meaning that we are in the same storage key segment
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
	filterAccountInsert := make([]field.Element, smc.AddressHI.Size())
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
					valueNextHi := smc.ValueHINext.GetColAssignmentAt(run, index)
					valueNextLo := smc.ValueLONext.GetColAssignmentAt(run, index)
					if valueNextHi.IsZero() && valueNextLo.IsZero() {
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
		smc.AddressHI.Size(),
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

	// filter must be constant as long as the storage key does not change
	// and the address and block number also does not change
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("GLOBAL_CONSTRAINT_HUB_STATE_SUMMARY__ACCOUNT_EPHEMERAL_FILTER_CONSTANCY"),
		sym.Mul(
			sc.SelectorSTKeyDiffHi,        // 1 if ST key HI is the same as in the previous index
			sc.SelectorSTKeyDiffLo,        // 1 if ST key LO is the same as in the previous index
			sc.SelectorAccountAddressDiff, // 1 if the account address is the same, meaning that our storage segment is within the same account segment
			sc.SelectorBlockNoDiff,        // 1 if the block number is the same, meaning that we are in the same storage key segment
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
	filterEphemeralAccounts := make([]field.Element, smc.AddressHI.Size())
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
		ss.Account.Address.Size(),
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
	filterAccountDelete := make([]field.Element, ss.Account.Address.Size())
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

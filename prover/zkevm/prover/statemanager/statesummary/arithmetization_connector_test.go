package statesummary

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mock"
)

// TestIntegrationConnector checks the connector between the StateSummary
// and the HUB arithmetization (account/storage consistency permutations—ACP/SCP)
func TestIntegrationConnector(t *testing.T) {

	t.Skip("skipping the test connector test as they fail currently, but we wait for the integration to fix")

	initialBlockNo := 0
	tContext := common.InitializeContext(initialBlockNo)
	var ss Module

	for i, tCase := range tContext.TestCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			t.Logf("Test case explainer: %v", tCase.Explainer)

			var (
				initState    = tContext.State
				shomeiState  = mock.InitShomeiState(initState)
				stateLogs    = tCase.StateLogsGens(initState)
				shomeiTraces = mock.StateLogsToShomeiTraces(shomeiState, stateLogs)
			)

			mock.AssertShomeiAgree(t, tContext.State, stateLogs)

			var stitcher mock.Stitcher
			stitcher.Initialize(initialBlockNo, initState)
			for index := range stateLogs {
				for _, frame := range stateLogs[index] {
					stitcher.AddFrame(frame)
				}
			}
			acpVectors := stitcher.Finalize(mock.GENERATE_ACP_SAMPLE)
			scpVectors := stitcher.Finalize(mock.GENERATE_SCP_SAMPLE)
			var acp, scp HubColumnSet

			define := func(b *wizard.Builder) {
				acp = defineStateManagerColumns(b.CompiledIOP, mock.GENERATE_ACP_SAMPLE, acpVectors.Size())
				scp = defineStateManagerColumns(b.CompiledIOP, mock.GENERATE_SCP_SAMPLE, scpVectors.Size())
				ss = NewModule(b.CompiledIOP, 1<<6)
				ss.ConnectToHub(b.CompiledIOP, acp, scp)
			}

			prove := func(run *wizard.ProverRuntime) {
				acp.assignForTest(run, acpVectors)
				scp.assignForTest(run, scpVectors)
				ss.Assign(run, shomeiTraces)
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prove)
			err := wizard.Verify(comp, proof)

			if err != nil {
				t.Fatalf("verification failed: %v", err)
			}

		})
	}

}

/*
defineStateManagerColumns is a function used for testing which assigns data from the arithmetization mock
in order to create a test sample of HUB columns, either ACP or SCP
*/
func defineStateManagerColumns(comp *wizard.CompiledIOP, sampleType int, size int) HubColumnSet {
	if !utils.IsPowerOfTwo(size) {
		utils.Panic("size must be power of two, got %v", size)
	}
	// createCol is function to quickly create a column
	createCol := func(name string) ifaces.Column {
		sampleTypeString := mock.SampleTypeToString(sampleType)
		return comp.InsertCommit(0, ifaces.ColIDf("ARITHM_%s_COL_%v", sampleTypeString, name), size)
	}

	res := HubColumnSet{
		Address:             createCol("Address"),
		AddressHI:           createCol("ADDRESS_HI"),
		AddressLO:           createCol("ADDRESS_LO"),
		Nonce:               createCol("NONCE"),
		NonceNew:            createCol("NONCE_NEW"),
		CodeHashHI:          createCol("CodeHashHI"),
		CodeHashLO:          createCol("CodeHashLO"),
		CodeHashHINew:       createCol("CodeHashHINew"),
		CodeHashLONew:       createCol("CodeHashLONew"),
		CodeSizeOld:         createCol("CodeSizeOld"),
		CodeSizeNew:         createCol("CodeSizeNew"),
		BalanceOld:          createCol("BalanceOld"),
		BalanceNew:          createCol("BalanceNew"),
		KeyHI:               createCol("KeyHI"),
		KeyLO:               createCol("KeyLO"),
		ValueHICurr:         createCol("ValueHICurr"),
		ValueLOCurr:         createCol("ValueLOCurr"),
		ValueHINext:         createCol("ValueHINext"),
		ValueLONext:         createCol("ValueLONext"),
		DeploymentNumber:    createCol("DeploymentNumber"),
		DeploymentNumberInf: createCol("DeploymentNumberInf"),
		BlockNumber:         createCol("BlockNumber"),
		Exists:              createCol("Exists"),
		ExistsNew:           createCol("ExistsNew"),
		PeekAtAccount:       createCol("PeekAtAccount"),
		PeekAtStorage:       createCol("PeekAtStorage"),
		FirstAOC:            createCol("FirstAOC"),
		LastAOC:             createCol("LastAOC"),
		FirstKOC:            createCol("FirstKOC"),
		LastKOC:             createCol("LastKOC"),
		FirstAOCBlock:       createCol("FirstAOCBlock"),
		LastAOCBlock:        createCol("LastAOCBlock"),
		FirstKOCBlock:       createCol("FirstKOCBlock"),
		LastKOCBlock:        createCol("LastKOCBlock"),
		MinDeplBlock:        createCol("MinDeplBlock"),
		MaxDeplBlock:        createCol("MaxDeplBlock"),
	}
	return res
}

/*
assignForTest is used to assign testing data from the arithmetization mock to a StateManagerColumns struct, corresponding
to a test sample of either the SCP or ACP.
*/
func (smc *HubColumnSet) assignForTest(run *wizard.ProverRuntime, smVectors *mock.StateManagerVectors) {
	assign := func(column ifaces.Column, vector []field.Element) {
		run.AssignColumn(column.GetColID(), smartvectors.NewRegular(vector))
	}

	addressVect := make([]field.Element, len(smVectors.Address))
	for index := range smVectors.Address {
		addressVect[index].SetBytes(smVectors.Address[index][:])
	}

	assign(smc.Address, addressVect)
	assign(smc.AddressHI, smVectors.AddressHI)
	assign(smc.AddressLO, smVectors.AddressLO)
	assign(smc.Nonce, smVectors.Nonce)
	assign(smc.NonceNew, smVectors.NonceNew)
	assign(smc.CodeHashHI, smVectors.CodeHashHI)
	assign(smc.CodeHashLO, smVectors.CodeHashLO)
	assign(smc.CodeHashHINew, smVectors.CodeHashHINew)
	assign(smc.CodeHashLONew, smVectors.CodeHashLONew)
	assign(smc.CodeSizeOld, smVectors.CodeSizeOld)
	assign(smc.CodeSizeNew, smVectors.CodeSizeNew)
	assign(smc.BalanceOld, smVectors.BalanceOld)
	assign(smc.BalanceNew, smVectors.BalanceNew)
	assign(smc.KeyHI, smVectors.KeyHI)
	assign(smc.KeyLO, smVectors.KeyLO)
	assign(smc.ValueHICurr, smVectors.ValueHICurr)
	assign(smc.ValueLOCurr, smVectors.ValueLOCurr)
	assign(smc.ValueHINext, smVectors.ValueHINext)
	assign(smc.ValueLONext, smVectors.ValueLONext)
	assign(smc.DeploymentNumber, smVectors.DeploymentNumber)
	assign(smc.DeploymentNumberInf, smVectors.DeploymentNumberInf)
	assign(smc.BlockNumber, smVectors.BlockNumber)
	assign(smc.Exists, smVectors.Exists)
	assign(smc.ExistsNew, smVectors.ExistsNew)
	assign(smc.PeekAtAccount, smVectors.PeekAtAccount)
	assign(smc.PeekAtStorage, smVectors.PeekAtStorage)
	assign(smc.FirstAOC, smVectors.FirstAOC)
	assign(smc.LastAOC, smVectors.LastAOC)
	assign(smc.FirstKOC, smVectors.FirstKOC)
	assign(smc.LastKOC, smVectors.LastKOC)
	assign(smc.FirstAOCBlock, smVectors.FirstAOCBlock)
	assign(smc.LastAOCBlock, smVectors.LastAOCBlock)
	assign(smc.FirstKOCBlock, smVectors.FirstKOCBlock)
	assign(smc.LastKOCBlock, smVectors.LastKOCBlock)
	assign(smc.MinDeplBlock, smVectors.MinDeploymentBlock)
	assign(smc.MaxDeplBlock, smVectors.MaxDeploymentBlock)
}

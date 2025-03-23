package statemanager

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mimccodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

const (
	ACP             = "acp"
	SCP             = "scp"
	ADDR_MULTIPLIER = "340282366920938463463374607431768211456" // 2^{16*8}
)

// romLex returns the columns of the arithmetization.RomLex module of interest
// to justify the consistency between them and the MiMCCodeHash module
func romLex(comp *wizard.CompiledIOP) *mimccodehash.RomLexInput {
	return &mimccodehash.RomLexInput{
		CFIRomLex:  comp.Columns.GetHandle("romlex.CODE_FRAGMENT_INDEX"),
		CodeHashHi: comp.Columns.GetHandle("romlex.CODE_HASH_HI"),
		CodeHashLo: comp.Columns.GetHandle("romlex.CODE_HASH_LO"),
	}
}

// rom returns the columns of the arithmetization corresponding to the Rom module
// that are of interest to justify consistency with the MiMCCodeHash module
func rom(comp *wizard.CompiledIOP) *mimccodehash.RomInput {
	res := &mimccodehash.RomInput{
		CFI:      comp.Columns.GetHandle("rom.CODE_FRAGMENT_INDEX"),
		Acc:      comp.Columns.GetHandle("rom.ACC"),
		NBytes:   comp.Columns.GetHandle("rom.nBYTES"),
		Counter:  comp.Columns.GetHandle("rom.COUNTER"),
		CodeSize: comp.Columns.GetHandle("rom.CODE_SIZE"),
	}
	return res
}

// acp returns the columns of the arithmetization corresponding to the ACP
// perspective of the Hub that are of interest for checking consistency with
// the stateSummary
func acp(comp *wizard.CompiledIOP) statesummary.HubColumnSet {
	size := comp.Columns.GetHandle("hub.acp_ADDRESS_HI").Size()

	// the prover-side state manager uses a single field element for 20-bytes addresses
	// and we need to create this column ourselves
	if !comp.Columns.Exists("HUB_acp_PROVER_SIDE_ADDRESS_IDENTIFIER") {
		combinedAddr := comp.InsertCommit(0,
			"HUB_acp_PROVER_SIDE_ADDRESS_IDENTIFIER",
			size,
		)

		// constrain the processed HUB addresses
		addrHI := comp.Columns.GetHandle("hub.acp_ADDRESS_HI")
		addrLO := comp.Columns.GetHandle("hub.acp_ADDRESS_LO")
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_MANAGER_ACP_HUB_PROCESSED_ADDRESSES_GLOBAL_CONSTRAINT"),
			sym.Sub(
				combinedAddr,
				sym.Mul(
					addrHI,
					field.NewFromString(ADDR_MULTIPLIER),
				),
				addrLO,
			),
		)
	}

	constantZero := verifiercol.NewConstantCol(field.Zero(), size)

	res := statesummary.HubColumnSet{
		Address:             comp.Columns.GetHandle("HUB_acp_PROVER_SIDE_ADDRESS_IDENTIFIER"),
		AddressHI:           comp.Columns.GetHandle("hub.acp_ADDRESS_HI"),
		AddressLO:           comp.Columns.GetHandle("hub.acp_ADDRESS_LO"),
		Nonce:               comp.Columns.GetHandle("hub.acp_NONCE"),
		NonceNew:            comp.Columns.GetHandle("hub.acp_NONCE_NEW"),
		CodeHashHI:          comp.Columns.GetHandle("hub.acp_CODE_HASH_HI"),
		CodeHashLO:          comp.Columns.GetHandle("hub.acp_CODE_HASH_LO"),
		CodeHashHINew:       comp.Columns.GetHandle("hub.acp_CODE_HASH_HI_NEW"),
		CodeHashLONew:       comp.Columns.GetHandle("hub.acp_CODE_HASH_LO_NEW"),
		CodeSizeOld:         comp.Columns.GetHandle("hub.acp_CODE_SIZE"),
		CodeSizeNew:         comp.Columns.GetHandle("hub.acp_CODE_SIZE_NEW"),
		BalanceOld:          comp.Columns.GetHandle("hub.acp_BALANCE"),
		BalanceNew:          comp.Columns.GetHandle("hub.acp_BALANCE_NEW"),
		KeyHI:               constantZero,
		KeyLO:               constantZero,
		ValueHICurr:         constantZero,
		ValueLOCurr:         constantZero,
		ValueHINext:         constantZero,
		ValueLONext:         constantZero,
		DeploymentNumber:    comp.Columns.GetHandle("hub.acp_DEPLOYMENT_NUMBER"),
		DeploymentNumberInf: comp.Columns.GetHandle("hub.acp_DEPLOYMENT_NUMBER"),
		BlockNumber:         comp.Columns.GetHandle("hub.acp_REL_BLK_NUM"),
		Exists:              comp.Columns.GetHandle("hub.acp_EXISTS"),
		ExistsNew:           comp.Columns.GetHandle("hub.acp_EXISTS_NEW"),
		PeekAtAccount:       comp.Columns.GetHandle("hub.acp_PEEK_AT_ACCOUNT"),
		PeekAtStorage:       constantZero,
		FirstAOC:            comp.Columns.GetHandle("hub.acp_FIRST_IN_CNF"),
		LastAOC:             comp.Columns.GetHandle("hub.acp_FINAL_IN_CNF"),
		FirstKOC:            constantZero,
		LastKOC:             constantZero,
		FirstAOCBlock:       comp.Columns.GetHandle("hub.acp_FIRST_IN_BLK"),
		LastAOCBlock:        comp.Columns.GetHandle("hub.acp_FINAL_IN_BLK"),
		FirstKOCBlock:       constantZero,
		LastKOCBlock:        constantZero,
		MinDeplBlock:        comp.Columns.GetHandle("hub.acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK"),
		MaxDeplBlock:        comp.Columns.GetHandle("hub.acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK"),
	}
	return res
}

// scp returns the columns of the arithmetization correspoanding to the SCP
// perspective of the Hub that are of interest for checking consistency with
// the stateSummary
func scp(comp *wizard.CompiledIOP) statesummary.HubColumnSet {
	size := comp.Columns.GetHandle("hub.scp_ADDRESS_HI").Size()

	// the prover-side state manager uses a single field element for 20-bytes addresses
	// and we need to create this column ourselves
	if !comp.Columns.Exists("HUB_scp_PROVER_SIDE_ADDRESS_IDENTIFIER") {
		combinedAddr := comp.InsertCommit(0,
			"HUB_scp_PROVER_SIDE_ADDRESS_IDENTIFIER",
			size,
		)

		// constrain the processed HUB addresses
		addrHI := comp.Columns.GetHandle("hub.scp_ADDRESS_HI")
		addrLO := comp.Columns.GetHandle("hub.scp_ADDRESS_LO")
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_MANAGER_SCP_HUB_PROCESSED_ADDRESSES_GLOBAL_CONSTRAINT"),
			sym.Sub(
				combinedAddr,
				sym.Mul(
					addrHI,
					field.NewFromString(ADDR_MULTIPLIER),
				),
				addrLO,
			),
		)
	}

	constantZero := verifiercol.NewConstantCol(field.Zero(), size)

	res := statesummary.HubColumnSet{
		Address:             comp.Columns.GetHandle("HUB_scp_PROVER_SIDE_ADDRESS_IDENTIFIER"),
		AddressHI:           comp.Columns.GetHandle("hub.scp_ADDRESS_HI"),
		AddressLO:           comp.Columns.GetHandle("hub.scp_ADDRESS_LO"),
		Nonce:               constantZero,
		NonceNew:            constantZero,
		CodeHashHI:          constantZero,
		CodeHashLO:          constantZero,
		CodeHashHINew:       constantZero,
		CodeHashLONew:       constantZero,
		CodeSizeOld:         constantZero,
		CodeSizeNew:         constantZero,
		BalanceOld:          constantZero,
		BalanceNew:          constantZero,
		KeyHI:               comp.Columns.GetHandle("hub.scp_STORAGE_KEY_HI"),
		KeyLO:               comp.Columns.GetHandle("hub.scp_STORAGE_KEY_LO"),
		ValueHICurr:         comp.Columns.GetHandle("hub.scp_VALUE_CURR_HI"),
		ValueLOCurr:         comp.Columns.GetHandle("hub.scp_VALUE_CURR_LO"),
		ValueHINext:         comp.Columns.GetHandle("hub.scp_VALUE_NEXT_HI"),
		ValueLONext:         comp.Columns.GetHandle("hub.scp_VALUE_NEXT_LO"),
		DeploymentNumber:    comp.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER"),
		DeploymentNumberInf: comp.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER"),
		BlockNumber:         comp.Columns.GetHandle("hub.scp_REL_BLK_NUM"),
		Exists:              constantZero,
		ExistsNew:           constantZero,
		PeekAtAccount:       constantZero,
		PeekAtStorage:       comp.Columns.GetHandle("hub.scp_PEEK_AT_STORAGE"),
		FirstAOC:            constantZero,
		LastAOC:             constantZero,
		FirstKOC:            comp.Columns.GetHandle("hub.scp_FIRST_IN_CNF"),
		LastKOC:             comp.Columns.GetHandle("hub.scp_FINAL_IN_CNF"),
		FirstAOCBlock:       constantZero,
		LastAOCBlock:        constantZero,
		FirstKOCBlock:       comp.Columns.GetHandle("hub.scp_FIRST_IN_BLK"),
		LastKOCBlock:        comp.Columns.GetHandle("hub.scp_FINAL_IN_BLK"),
		MinDeplBlock:        comp.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK"),
		MaxDeplBlock:        comp.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK"),
	}
	return res
}

/*
assignHubAddresses is a function that combines addressHI and addressLO from
the arithmetization columns into a single column.
*/
func assignHubAddresses(run *wizard.ProverRuntime) {
	assignHubAddressesSubdomain := func(domainName string) {
		addressHI := run.GetColumn(ifaces.ColID(fmt.Sprintf("hub.%s_ADDRESS_HI", domainName)))
		addressLO := run.GetColumn(ifaces.ColID(fmt.Sprintf("hub.%s_ADDRESS_LO", domainName)))

		size := addressHI.Len()
		newVect := make([]field.Element, size)
		for i := range newVect {
			elemHi := addressHI.Get(i)
			bytesHi := elemHi.Bytes()

			elemLo := addressLO.Get(i)
			bytesLo := elemLo.Bytes()
			newBytes := make([]byte, field.Bytes)
			// set the high part
			for j := 0; j < 4; j++ {
				newBytes[12+j] = bytesHi[32-(4-j)]
			}
			// set the low part
			for j := 4; j < 20; j++ {
				newBytes[12+j] = bytesLo[16+(j-4)]
			}
			newVect[i].SetBytes(newBytes)
		}
		run.AssignColumn(
			ifaces.ColID(fmt.Sprintf("HUB_%s_PROVER_SIDE_ADDRESS_IDENTIFIER", domainName)),
			smartvectors.NewRegular(newVect),
		)
	}
	// assign the addresses column in each of the submodules
	assignHubAddressesSubdomain(ACP)
	assignHubAddressesSubdomain(SCP)
}

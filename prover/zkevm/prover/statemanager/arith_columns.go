package statemanager

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/lineacodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

const (
	ACP             = "acp"
	SCP             = "scp"
	ADDR_MULTIPLIER = "340282366920938463463374607431768211456" // 2^{16*8}
)

// romLex returns the columns of the arithmetization.RomLex module of interest
// to justify the consistency between them and the MiMCCodeHash module
func romLex(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *lineacodehash.RomLexInput {
	return &lineacodehash.RomLexInput{
		CFIRomLex: arith.GetLimbsOfU32Be(comp, "romlex", "CODE_FRAGMENT_INDEX").LimbsArr2(),
		CodeHash: limbs.FuseLimbs(
			arith.GetLimbsOfU128Be(comp, "romlex", "CODE_HASH_HI").AsDynSize(),
			arith.GetLimbsOfU128Be(comp, "romlex", "CODE_HASH_LO").AsDynSize(),
		).LimbsArr16(),
	}
}

// rom returns the columns of the arithmetization corresponding to the Rom module
// that are of interest to justify consistency with the MiMCCodeHash module
func rom(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *lineacodehash.RomInput {
	res := &lineacodehash.RomInput{
		NBytes:   comp.Columns.GetHandle("rom.nBYTES"),
		Counter:  comp.Columns.GetHandle("rom.COUNTER"),
		Acc:      arith.GetLimbsOfU128Be(comp, "rom", "ACC").LimbsArr8(),
		CFI:      arith.GetLimbsOfU32Be(comp, "rom", "CODE_FRAGMENT_INDEX").LimbsArr2(),
		CodeSize: arith.GetLimbsOfU32Be(comp, "rom", "CODE_SIZE").LimbsArr2(),
	}

	return res
}

// acp returns the columns of the arithmetization corresponding to the ACP
// perspective of the Hub that are of interest for checking consistency with
// the stateSummary
func acp(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) statesummary.HubColumnSet {
	size := arith.GetLimbsOfU32Be(comp, "hub", "acp_ADDRESS_HI").NumLimbs()

	constantZero := verifiercol.NewConstantCol(field.Zero(), size, "hub.acp_connection")
	constantZero8 := [8]ifaces.Column{constantZero, constantZero, constantZero, constantZero, constantZero, constantZero, constantZero, constantZero}

	res := statesummary.HubColumnSet{
		AddressHI:     arith.GetLimbsOfU32Be(comp, "hub", "acp_ADDRESS_HI").LimbsArr2(),
		AddressLO:     arith.GetLimbsOfU128Be(comp, "hub", "acp_ADDRESS_LO").LimbsArr8(),
		Nonce:         arith.GetLimbsOfU64Be(comp, "hub", "acp_NONCE").LimbsArr4(),
		NonceNew:      arith.GetLimbsOfU64Be(comp, "hub", "acp_NONCE_NEW").LimbsArr4(),
		CodeHashHI:    arith.GetLimbsOfU128Be(comp, "hub", "acp_CODE_HASH_HI").LimbsArr8(),
		CodeHashLO:    arith.GetLimbsOfU128Be(comp, "hub", "acp_CODE_HASH_LO").LimbsArr8(),
		CodeHashHINew: arith.GetLimbsOfU128Be(comp, "hub", "acp_CODE_HASH_HI_NEW").LimbsArr8(),
		CodeHashLONew: arith.GetLimbsOfU128Be(comp, "hub", "acp_CODE_HASH_LO_NEW").LimbsArr8(),
		// @alex: the glue has been implemented assuming CodeSizeOld|New are
		// stored on u64 but they are actually provided as u32. Hence, we do
		// some patching here.
		CodeSizeOld:         arith.GetLimbsOfU32Be(comp, "hub", "acp_CODE_SIZE").ZeroExtendToSize(4).LimbsArr4(),
		CodeSizeNew:         arith.GetLimbsOfU32Be(comp, "hub", "acp_CODE_SIZE_NEW").ZeroExtendToSize(4).LimbsArr4(),
		BalanceOld:          arith.GetLimbsOfU128Be(comp, "hub", "acp_BALANCE").ZeroExtendToSize(16).LimbsArr16(),
		BalanceNew:          arith.GetLimbsOfU128Be(comp, "hub", "acp_BALANCE_NEW").ZeroExtendToSize(16).LimbsArr16(),
		KeyHI:               constantZero8,
		KeyLO:               constantZero8,
		ValueHICurr:         constantZero8,
		ValueLOCurr:         constantZero8,
		ValueHINext:         constantZero8,
		ValueLONext:         constantZero8,
		DeploymentNumber:    arith.GetLimbsOfU32Be(comp, "hub", "acp_DEPLOYMENT_NUMBER").LimbsArr2(),
		DeploymentNumberInf: arith.GetLimbsOfU32Be(comp, "hub", "acp_DEPLOYMENT_NUMBER").LimbsArr2(),
		BlockNumber:         arith.GetLimbsOfU16Be(comp, "hub", "acp_BLK_NUMBER").ZeroExtendToSize(4).LimbsArr4(),
		Exists:              arith.ColumnOf(comp, "hub", "acp_EXISTS"),
		ExistsNew:           arith.ColumnOf(comp, "hub", "acp_EXISTS_NEW"),
		PeekAtAccount:       arith.ColumnOf(comp, "hub", "acp_PEEK_AT_ACCOUNT"),
		PeekAtStorage:       constantZero,
		FirstAOC:            arith.ColumnOf(comp, "hub", "acp_FIRST_IN_CNF"),
		LastAOC:             arith.ColumnOf(comp, "hub", "acp_FINAL_IN_CNF"),
		FirstKOC:            constantZero,
		LastKOC:             constantZero,
		FirstAOCBlock:       arith.ColumnOf(comp, "hub", "acp_FIRST_IN_BLK"),
		LastAOCBlock:        arith.ColumnOf(comp, "hub", "acp_FINAL_IN_BLK"),
		FirstKOCBlock:       constantZero,
		LastKOCBlock:        constantZero,
		MinDeplBlock:        arith.GetLimbsOfU32Be(comp, "hub", "acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK").LimbsArr2(),
		MaxDeplBlock:        arith.GetLimbsOfU32Be(comp, "hub", "acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK").LimbsArr2(),
		ExistsFirstInBlock:  constantZero,
		ExistsFinalInBlock:  constantZero,
	}

	return res
}

// scp returns the columns of the arithmetization correspoanding to the SCP
// perspective of the Hub that are of interest for checking consistency with
// the stateSummary
func scp(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) statesummary.HubColumnSet {
	size := arith.GetLimbsOfU32Be(comp, "hub", "scp_ADDRESS_HI").NumRow()

	var (
		constantZero   = verifiercol.NewConstantCol(field.Zero(), size, "hub.scp_connection")
		czs            = []ifaces.Column{constantZero}
		constantZero4  = [4]ifaces.Column(slices.Repeat(czs, 4))
		constantZero8  = [8]ifaces.Column(slices.Repeat(czs, 8))
		constantZero16 = [16]ifaces.Column(slices.Repeat(czs, 16))
	)

	res := statesummary.HubColumnSet{
		AddressHI:           arith.GetLimbsOfU32Be(comp, "hub", "scp_ADDRESS_HI").LimbsArr2(),
		AddressLO:           arith.GetLimbsOfU128Be(comp, "hub", "scp_ADDRESS_LO").LimbsArr8(),
		Nonce:               constantZero4,
		NonceNew:            constantZero4,
		CodeHashHI:          constantZero8,
		CodeHashLO:          constantZero8,
		CodeHashHINew:       constantZero8,
		CodeHashLONew:       constantZero8,
		CodeSizeOld:         constantZero4,
		CodeSizeNew:         constantZero4,
		BalanceOld:          constantZero16,
		BalanceNew:          constantZero16,
		KeyHI:               arith.GetLimbsOfU128Be(comp, "hub", "scp_STORAGE_KEY_HI").LimbsArr8(),
		KeyLO:               arith.GetLimbsOfU128Be(comp, "hub", "scp_STORAGE_KEY_LO").LimbsArr8(),
		ValueHICurr:         arith.GetLimbsOfU128Be(comp, "hub", "scp_VALUE_CURR_HI").LimbsArr8(),
		ValueLOCurr:         arith.GetLimbsOfU128Be(comp, "hub", "scp_VALUE_CURR_LO").LimbsArr8(),
		ValueHINext:         arith.GetLimbsOfU128Be(comp, "hub", "scp_VALUE_NEXT_HI").LimbsArr8(),
		ValueLONext:         arith.GetLimbsOfU128Be(comp, "hub", "scp_VALUE_NEXT_LO").LimbsArr8(),
		DeploymentNumber:    arith.GetLimbsOfU16Be(comp, "hub", "scp_DEPLOYMENT_NUMBER").ZeroExtendToSize(2).LimbsArr2(),
		DeploymentNumberInf: arith.GetLimbsOfU16Be(comp, "hub", "scp_DEPLOYMENT_NUMBER").ZeroExtendToSize(2).LimbsArr2(),
		BlockNumber:         arith.GetLimbsOfU16Be(comp, "hub", "scp_BLK_NUMBER").ZeroExtendToSize(4).LimbsArr4(),
		Exists:              constantZero,
		ExistsNew:           constantZero,
		PeekAtAccount:       constantZero,
		PeekAtStorage:       arith.ColumnOf(comp, "hub", "scp_PEEK_AT_STORAGE"),
		FirstAOC:            constantZero,
		LastAOC:             constantZero,
		FirstKOC:            arith.ColumnOf(comp, "hub", "scp_FIRST_IN_CNF"),
		LastKOC:             arith.ColumnOf(comp, "hub", "scp_FINAL_IN_CNF"),
		FirstAOCBlock:       constantZero,
		LastAOCBlock:        constantZero,
		FirstKOCBlock:       arith.ColumnOf(comp, "hub", "scp_FIRST_IN_BLK"),
		LastKOCBlock:        arith.ColumnOf(comp, "hub", "scp_FINAL_IN_BLK"),
		MinDeplBlock:        arith.GetLimbsOfU32Be(comp, "hub", "scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK").LimbsArr2(),
		MaxDeplBlock:        arith.GetLimbsOfU32Be(comp, "hub", "scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK").LimbsArr2(),
		ExistsFirstInBlock:  arith.ColumnOf(comp, "hub", "scp_EXISTS_FIRST_IN_BLOCK"),
		ExistsFinalInBlock:  arith.ColumnOf(comp, "hub", "scp_EXISTS_FINAL_IN_BLOCK"),
	}

	return res
}

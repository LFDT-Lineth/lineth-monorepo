package modexp

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// input collects references to the columns of the arithmetization containing
// the Modexp statements. These columns are constrained via a projection query
// to describe the same statement as what is being stated in the antichamber
// module. They are also used as a data source to assign the columns of the
// antichamber module.
//
// The columns provided here are columns from the BLK_MDXP module.
type Input struct {
	Settings Settings
	// Binary column indicating if we have base limbs
	IsModExpBase ifaces.Column
	// Binary column indicating if we have exponent limbs
	IsModExpExponent ifaces.Column
	// Binary column indicating if we have modulus limbs
	IsModExpModulus ifaces.Column
	// Binary column indicating if we have result limbs
	IsModExpResult ifaces.Column
	// IsModexp is a constructed column constrained to be equal to the sum of the
	// 4 above columns.
	IsModExp ifaces.Column
	// Multiplexed column containing limbs for base, exponent, modulus, and result
	Limbs ifaces.Column
}

type Settings struct {
	MaxNbInstance256, MaxNbInstanceLarge int
	NbInstancesPerCircuitModexp256       int
	NbInstancesPerCircuitModexpLarge     int
}

func newZkEVMInput(comp *wizard.CompiledIOP, settings Settings) Input {
	return Input{
		Settings:         settings,
		IsModExpBase:     comp.Columns.GetHandle("blake2fmodexpdata.IS_MODEXP_BASE"),
		IsModExpExponent: comp.Columns.GetHandle("blake2fmodexpdata.IS_MODEXP_EXPONENT"),
		IsModExpModulus:  comp.Columns.GetHandle("blake2fmodexpdata.IS_MODEXP_MODULUS"),
		IsModExpResult:   comp.Columns.GetHandle("blake2fmodexpdata.IS_MODEXP_RESULT"),
		Limbs:            comp.Columns.GetHandle("blake2fmodexpdata.LIMB"),
	}
}

// setIsModexp constructs, constraints and set the [isModexpColumn]
func (i *Input) setIsModexp(comp *wizard.CompiledIOP) {

	i.IsModExp = comp.InsertCommit(0, "MODEXP_INPUT_IS_MODEXP", i.IsModExpBase.Size())

	comp.InsertGlobal(
		0,
		"MODEXP_IS_MODEXP_WELL_CONSTRUCTED",
		sym.Sub(
			i.IsModExp,
			i.IsModExpBase,
			i.IsModExpExponent,
			i.IsModExpModulus,
			i.IsModExpResult,
		),
	)
}

// assignIsModexp evaluates and assigns the IsModexp column
func (i *Input) assignIsModexp(run *wizard.ProverRuntime) {

	var (
		isBase     = i.IsModExpBase.GetColAssignment(run)
		isExponent = i.IsModExpExponent.GetColAssignment(run)
		isModulus  = i.IsModExpModulus.GetColAssignment(run)
		isResult   = i.IsModExpResult.GetColAssignment(run)
		isModexp   = smartvectors.Add(isBase, isExponent, isModulus, isResult)
	)

	run.AssignColumn(i.IsModExp.GetColID(), isModexp)
}

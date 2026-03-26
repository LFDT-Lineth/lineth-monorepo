package modexp

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commoncs "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

const (
	// smallModexpSize is the bit-size bound for small modexp instances
	smallModExpSize = 256
	// largeModexpSize is the bit-size bound for large modexp instances
	largeModExpSize = 8192

	// nbBitPerInputRow is the number of bits per row in the input module.
	nbBitPerInputRow = 128

	// nbRowSmallInInput is the number of rows in the input module to represent
	// a single 256 bit number in the inputs.
	nbSmallModexpLimbs = smallModExpSize / nbBitPerInputRow

	// nbRowLargeInInput is the number of rows in the input module to represent
	// a single 8192 bit number in the inputs.
	nbLargeModexpLimbs = largeModExpSize / nbBitPerInputRow

	// modexpNumRows corresponds to the number of rows present in the MODEXP
	// module to represent a single instance. Each instance has 4 operands
	// dispatched in limbs of [limbSizeBits] bits.
	modexpNumRowsPerInstance = nbLargeModexpLimbs * 4 // 4 operands: base, exponent, modulus, result

	// roundNr is the round number where the modexp module runs. It is based on
	// arithmetization, so we start from round 0.
	roundNr = 0
)

type Settings struct {
	// MaxNbInstance256 is the maximum number of small (256 bits) modexp
	// instances to be handled by the module.
	//
	// NB: This number must be > 0.
	MaxNbInstance256 int
	// MaxNbInstanceLarge is the maximum number of large (8192 bits) modexp
	// instances to be handled by the module. All large instances (over 256 bits)
	// are handled by the same large modexp circuit.
	//
	// NB: This number must be > 0.
	MaxNbInstanceLarge int
}

type Input struct {
	*Settings
	// Binary column indicating if we have base limbs
	IsModExpBase ifaces.Column
	// Binary column indicating if we have exponent limbs
	IsModExpExponent ifaces.Column
	// Binary column indicating if we have modulus limbs
	IsModExpModulus ifaces.Column
	// Binary column indicating if we have result limbs
	IsModExpResult ifaces.Column
	// Multiplexed column containing limbs for base, exponent, modulus, and result
	Limbs limbs.Uint128Le
}

func newZkEVMInput(comp *wizard.CompiledIOP, settings Settings, arith *arithmetization.Arithmetization) *Input {
	return &Input{
		Settings:         &settings,
		IsModExpBase:     arith.ColumnOf(comp, "blake2fmodexpdata", "IS_MODEXP_BASE"),
		IsModExpExponent: arith.ColumnOf(comp, "blake2fmodexpdata", "IS_MODEXP_EXPONENT"),
		IsModExpModulus:  arith.ColumnOf(comp, "blake2fmodexpdata", "IS_MODEXP_MODULUS"),
		IsModExpResult:   arith.ColumnOf(comp, "blake2fmodexpdata", "IS_MODEXP_RESULT"),
		Limbs:            arith.GetLimbsOfU128Le(comp, "blake2fmodexpdata", "LIMB"),
	}
}

// NewModuleZkEvm constructs an instance of the modexp module. It should be called
// only once during zkEVM prover lifecycle.
func NewModuleZkEvm(comp *wizard.CompiledIOP, settings Settings, arith *arithmetization.Arithmetization) *Module {
	return newModule(comp, newZkEVMInput(comp, settings, arith))
}

// Module implements the wizard part responsible for checking the MODEXP
// claims coming from the BLKMDXP module of the arithmetization.
//
// It handles both small (256 bits) and large (8192 bits) modexp instances.
type Module struct {
	// Input stores the columns used as a source for the antichamber.
	*Input

	// IsModExp is a binary indicator column marking with a 1, the rows of the
	// antichamber modules corresponding "active" rows: e.g. NOT padding rows.
	IsModExp ifaces.Column
	// IsSmall, IsLarge are indicator columns that are constant per modexp
	// instances. They are mutually exclusive and activated by IsModExp.
	IsSmall, IsLarge ifaces.Column
	// ToSmall is indicator column marking with a 1 the
	// positions of limbs corresponding to public inputs of (respectely) the
	// small circuit. It is implicit for large circuit as ToLarge = IsLarge
	ToSmall ifaces.Column

	// Small and Large are the submodule doing the actual modular exponentiation
	// checks using field emulation.
	Small, Large *Modexp
}

func newModule(comp *wizard.CompiledIOP, input *Input) *Module {
	var (
		settings = input.Settings
		mod      = &Module{
			IsModExp: comp.InsertCommit(0, "MODEXP_INPUT_IS_MODEXP", input.Limbs.Size(), true),
			IsSmall:  comp.InsertCommit(0, "MODEXP_IS_SMALL", input.Limbs.Size(), true),
			IsLarge:  comp.InsertCommit(0, "MODEXP_IS_LARGE", input.Limbs.Size(), true),
			ToSmall:  comp.InsertCommit(0, "MODEXP_TO_SMALL", input.Limbs.Size(), true),
			Input:    input,
		}
	)
	comp.RegisterProverAction(roundNr, mod)
	// run later to ensure we have the assignments already done
	mod.Small = newModexp(comp, "MODEXP_SMALL", mod, mod.IsSmall, mod.ToSmall, smallModExpSize, settings.MaxNbInstance256)
	mod.Large = newModexp(comp, "MODEXP_LARGE", mod, mod.IsLarge, mod.IsLarge, largeModExpSize, settings.MaxNbInstanceLarge)

	// check that isModexp is well constructed
	mod.csIsModExp(comp)
	// check that IsSmall and IsLarge are well constructed
	mod.csIsSmallAndLarge(comp)
	// check that the concrete mask for IS_SMALL is well constructed
	mod.csToCirc(comp)

	return mod
}

// csIsModExp constructs, constraints and set the [isModexpColumn]
func (m *Module) csIsModExp(comp *wizard.CompiledIOP) {
	comp.InsertGlobal(
		roundNr,
		"MODEXP_IS_MODEXP_WELL_CONSTRUCTED",
		sym.Sub(
			m.IsModExp,
			m.IsModExpBase,
			m.IsModExpExponent,
			m.IsModExpModulus,
			m.IsModExpResult,
		),
	)
}

// csIsSmallAndLarge constrains IsSmall and IsLarge
func (mod *Module) csIsSmallAndLarge(comp *wizard.CompiledIOP) {
	commoncs.MustBeMutuallyExclusiveBinaryFlags(comp, mod.IsModExp, []ifaces.Column{mod.IsSmall, mod.IsLarge})

	//
	// NB: The facts that
	// * IsSmall and IsLarge are mutually exclusive columns
	// * that IsActive implictly only switches at the end of the last modexp
	// instance (see the comment in the [csIsActive] function).
	// * The constraint [MODEXP_IS_SMALL_CONSTANT_BY_SEGMENT] is here
	//
	// Imply already an equivalent [MODEXP_IS_LARGE_CONSTANT_BY_SEGMENT]
	// constraint. So we do not need to declare it.
	//

	comp.InsertGlobal(
		0,
		"MODEXP_IS_SMALL_CONSTANT_BY_SEGMENT",
		sym.Mul(
			sym.Sub(1, variables.NewPeriodicSample(modexpNumRowsPerInstance, 0)),
			sym.Sub(mod.IsSmall, column.Shift(mod.IsSmall, -1)),
		),
	)

	//
	// The constraint below ensures that if the IS_SMALL flag is set, then the
	// limbs 2..64 of the operands of the corresponding modexp must be zero
	// (otherwise, they would represent numbers larger than 256 bits).
	//
	// The converse constraint does not exists in the large (8192) case because it would
	// not be wrong to supply
	//

	masks := make([]any, nbSmallModexpLimbs)
	for i := range nbSmallModexpLimbs {
		masks[i] = variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-i-1)
	}

	limbs.NewGlobal(
		comp,
		"MODEXP_IS_SMALL_IMPLIES_SMALL_OPERANDS",
		sym.Mul(
			mod.Limbs,
			mod.IsSmall,
			sym.Sub(1, masks...),
		),
	)
}

// csToCirc ensures the well-construction of ant.ToSmallCirc
func (mod *Module) csToCirc(comp *wizard.CompiledIOP) {
	masks := make([]any, nbSmallModexpLimbs)
	for i := range nbSmallModexpLimbs {
		masks[i] = variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-i-1)
	}
	comp.InsertGlobal(
		0,
		"MODEXP_TO_SMALL_CIRC_VAL",
		sym.Sub(
			mod.ToSmall,
			sym.Mul(
				mod.IsSmall,
				sym.Add(masks...),
			),
		),
	)

	//
	// NB: We set ToLargeCirc = IsLarge as these to indicator coincidates
	// so there is no need to add extra constraints.
	//
}

func (m *Module) Run(run *wizard.ProverRuntime) {
	m.assignIsActive(run)
	m.assignIsSmallOrLarge(run)
}

// assignIsActive evaluates and assigns the IsActive column
func (m *Module) assignIsActive(run *wizard.ProverRuntime) {

	var (
		isBase     = m.IsModExpBase.GetColAssignment(run)
		isExponent = m.IsModExpExponent.GetColAssignment(run)
		isModulus  = m.IsModExpModulus.GetColAssignment(run)
		isResult   = m.IsModExpResult.GetColAssignment(run)
		isActive   = smartvectors.Add(isBase, isExponent, isModulus, isResult)
	)

	run.AssignColumn(m.IsModExp.GetColID(), isActive)
}

func (m *Module) assignIsSmallOrLarge(run *wizard.ProverRuntime) {

	var (
		srcIsModExp = m.IsModExp.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcLimbs    = m.Limbs.GetAssignmentAsBigInt(run)
		dstIsSmall  = common.NewVectorBuilder(m.IsSmall)
		dstIsLarge  = common.NewVectorBuilder(m.IsLarge)
		dstToSmall  = common.NewVectorBuilder(m.ToSmall)
	)

	checkSmall := func(ptr int) bool {
		for k := range nbLargeModexpLimbs - nbSmallModexpLimbs {
			for j := range 4 {
				if srcLimbs[ptr+k+j*nbLargeModexpLimbs].Sign() != 0 {
					return false
				}
			}
		}
		return true
	}

	for ptr := 0; ptr < len(srcLimbs); {
		if srcIsModExp[ptr].IsZero() {
			dstIsSmall.PushZero()
			dstIsLarge.PushZero()
			dstToSmall.PushZero()
			ptr++
			continue
		}
		// found a modexp instance. We read the modexp inputs from here.
		isSmall := checkSmall(ptr)
		for r := range modexpNumRowsPerInstance {
			if isSmall {
				dstIsSmall.PushOne()
				dstIsLarge.PushZero()
				if (r % nbLargeModexpLimbs) > (nbLargeModexpLimbs-nbSmallModexpLimbs)-1 {
					dstToSmall.PushOne()
				} else {
					dstToSmall.PushZero()
				}
			} else {
				dstIsSmall.PushZero()
				dstIsLarge.PushOne()
				dstToSmall.PushZero()
			}
		}
		ptr += modexpNumRowsPerInstance
	}
	dstIsLarge.PadAndAssign(run)
	dstIsSmall.PadAndAssign(run)
	dstToSmall.PadAndAssign(run)
}

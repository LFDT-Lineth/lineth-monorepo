package modexp

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/emulated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

// emulatedLimbSize is the size of an emulated limb.
const emulatedLimbSizeBit = 16

// Modexp is the concrete instance of the modexp module for zkEVM prover.
type Modexp struct {
	// Module references to the main modexp module.
	*Module
	// InstanceSize indicates the size of the modexp instances handled by this module.
	// It can be either [smallModExpSize] or [largeModExpSize], and panics if set
	// to anything else.
	InstanceSize int
	// NbLimbs indicates the number of limbs used to represent the modexp operands.
	// It is computed as instanceSize/emulatedLimbSizeBit.
	NbLimbs int
	// Name of the module for creating column and query names
	Name string

	// ToEval indicates the limbs which go to the circuit when IsActive is set.
	// For small instance this corresponds to the nbSmallModexpLimbs limbs, for
	// large instance this corresponds to all nbLargeModexpLimbs limbs (the same as IsActive).
	ToEval ifaces.Column
	// IsActiveFromInput indicates from input which rows are part of this modexp instance
	IsActiveFromInput ifaces.Column // the active flag for this module
	// IsBase is IsModExpBase masked by IsActive
	IsBase ifaces.Column
	// IsExponent is IsModExpExponent masked by IsActive
	IsExponent ifaces.Column
	// IsModulus is IsModExpModulus masked by IsActive
	IsModulus ifaces.Column
	// IsResult is IsModExpResult masked by IsActive
	IsResult ifaces.Column

	// IsFirstLineOfInstance indicates the first line of every modexp instance in this module
	IsFirstLineOfInstance ifaces.Column
	// IsLastLineOfInstance indicates the last line of every modexp instance in this module
	IsLastLineOfInstance ifaces.Column
	// IsActive indicates that any of the modexp operands are present in this row
	IsActive ifaces.Column

	// PrevAccumulator is the previous accumulator limbs. First row corresponds
	// to initial value 1. It is copied from the current accumulator from
	// previous row.
	PrevAccumulator limbs.Limbs[limbs.LittleEndian]
	// CurrAccumulator is the current accumulator limbs. Defines the result of
	// exponentiation step.
	CurrAccumulator limbs.Limbs[limbs.LittleEndian]
	// Modulus is the modulus limbs
	Modulus limbs.Limbs[limbs.LittleEndian]
	// Exponent is the exponent limbs. Not used in modexp directly (only its
	// bits are used), but we use it to check correctness of exponent bits
	// decomposition.
	Exponent limbs.Limbs[limbs.LittleEndian]
	// ExponentBits is the exponent bits limbs. Used to check correctness of
	// exponent bits decomposition. We use [limbs.Limbs] as it is input
	// to the emulated field operations.
	ExponentBits limbs.Limbs[limbs.LittleEndian]
	// Base is the modexp base. It is fixed over all rows of the instance.
	Base limbs.Limbs[limbs.LittleEndian]
}

// newModexp constructs a modexp instance for either small or large modexp. The
// returned value can be used to reference the columns. All the assignments are
// automatically done, thus there is no `Assign` method.
func newModexp(comp *wizard.CompiledIOP, name string, module *Module, isActiveFromInput ifaces.Column, toEval ifaces.Column, instanceSize int, nbInstances int) *Modexp {
	// we omit the Modexp module when there is no instance to process
	if nbInstances == 0 {
		return nil
	}
	var nbLimbs, nbRows int
	switch instanceSize {
	case smallModExpSize:
		nbLimbs = nbSmallModexpLimbs * limbs.NbLimbU128
		nbRows = utils.NextPowerOfTwo(nbInstances * smallModExpSize)
	case largeModExpSize:
		nbLimbs = nbLargeModexpLimbs * limbs.NbLimbU128
		nbRows = utils.NextPowerOfTwo(nbInstances * largeModExpSize)
	default:
		utils.Panic("unsupported modexp instance size: %d", instanceSize)
	}
	// we assume that the limb widths are aligned between arithmetization and emulated field ops
	prevAcc := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_PREV_ACC", name), nbLimbs, nbRows)
	currAcc := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_CURR_ACC", name), nbLimbs, nbRows)
	modulus := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_MODULUS", name), nbLimbs, nbRows)
	exponent := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EXPONENT", name), nbLimbs, nbRows)
	exponentBits := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EXPONENT_BITS", name), 1, nbRows)
	base := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_BASE", name), nbLimbs, nbRows)

	me := &Modexp{
		InstanceSize:          instanceSize,
		NbLimbs:               nbLimbs,
		Name:                  name,
		PrevAccumulator:       prevAcc,
		CurrAccumulator:       currAcc,
		Modulus:               modulus,
		Exponent:              exponent,
		ExponentBits:          exponentBits,
		Base:                  base,
		IsActiveFromInput:     isActiveFromInput,
		ToEval:                toEval,
		Module:                module,
		IsBase:                comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_BASE", name), module.Input.IsModExpBase.Size(), true),
		IsExponent:            comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_EXPONENT", name), module.Input.IsModExpExponent.Size(), true),
		IsModulus:             comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_MODULUS", name), module.Input.IsModExpModulus.Size(), true),
		IsResult:              comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_RESULT", name), module.Input.IsModExpResult.Size(), true),
		IsFirstLineOfInstance: comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_FIRST_LINE_OF_INSTANCE", name), nbRows, true),
		IsLastLineOfInstance:  comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_LAST_LINE_OF_INSTANCE", name), nbRows, true),
		IsActive:              comp.InsertCommit(roundNr, ifaces.ColIDf("%s_IS_ACTIVE", name), nbRows, true),
	}
	// Pragma from wizard distribution
	pragmas.AddModuleRef(me.IsActive, name+"_EMULATED_MODEXP")
	// register prover action before emulated evaluation so that the limb assignements are available
	comp.RegisterProverAction(roundNr, me)
	emulated.NewEval(comp, name+"_EVAL", emulatedLimbSizeBit, modulus, [][]limbs.Limbs[limbs.LittleEndian]{
		// R_i = R_{i-1}^2 + R_{i-1}^2*e_i*base - R_{i-1}^2*e_i - currAcc
		{prevAcc, prevAcc}, {prevAcc, prevAcc, exponentBits, base}, {prevAcc, prevAcc, exponentBits}, {currAcc},
	}, []int{1, 1, -1, -1})

	// the values are only set when IsActive is set
	me.csIsActive(comp)
	// first line of instance in this module has flag 1 set. We use it for
	// defining projection correctness of base and modulus
	me.csFirstAndLastRowOfInstance(comp)
	// projection that the base, modulus and result are correctly projected
	me.csModexpDataProjection(comp)
	// projection that the MSB decomposition of exponent is correct
	me.csModexpExponentProjection(comp)
	// query that the base and modulus are constant over all rows of the
	// instance
	me.csConstantBaseAndModulus(comp)
	// query that the accumulator transitions are correct (prev_i ==
	// current_{i-1}) and that accumulator is set to 1 in the first row
	me.csAccumulatorTransition(comp)

	return me
}

func (m *Modexp) csIsActive(comp *wizard.CompiledIOP) {
	var cols []ifaces.Column
	cols = append(cols, m.Base.GetLimbs()...)
	cols = append(cols, m.Modulus.GetLimbs()...)
	cols = append(cols, m.ExponentBits.GetLimbs()...)
	cols = append(cols, m.CurrAccumulator.GetLimbs()...)
	cols = append(cols, m.PrevAccumulator.GetLimbs()...)
	commonconstraints.MustZeroWhenInactive(comp, m.IsActive, cols...)
}

func (m *Modexp) csFirstAndLastRowOfInstance(comp *wizard.CompiledIOP) {
	// we need to ensure that the first and last lines are exactly nbRows apart.
	// We don't need to do more trickery (with active rows) as we already use projection
	// from the trace, which ensures that the number of first lines matches the number of last lines.
	// and if everything in between is well formed, then also the whole segment is well formed.
	comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_FIRST_LAST_ROW_WELL_FORMED", m.Name),
		sym.Sub(
			m.IsFirstLineOfInstance,
			column.Shift(m.IsLastLineOfInstance, m.InstanceSize-1),
		),
	)
}

func (m *Modexp) csModexpDataProjection(comp *wizard.CompiledIOP) {
	// * projection query that the following values are correctly projected to the first row of each instance:
	// - base limbs
	// - modulus limbs
	// - result limbs (i.e. final accumulator)
	// - exponent limbs

	// base
	columnsB := make([][]ifaces.Column, m.Base.NumLimbs())
	for i, l := range m.Base.GetLimbs() {
		columnsB[len(columnsB)-1-i] = []ifaces.Column{l}
	}
	filtersB := make([]ifaces.Column, len(columnsB))
	for i := range filtersB {
		filtersB[i] = m.IsFirstLineOfInstance
	}

	var (
		inputLimbsProjectionTable          = make([][]ifaces.Column, m.Input.Limbs.NumLimbs())
		inputLimbsBaseProjectionFilter     = make([]ifaces.Column, m.Input.Limbs.NumLimbs())
		inputLimbsExponentProjectionFilter = make([]ifaces.Column, m.Input.Limbs.NumLimbs())
		inputLimbsModulusProjectionFilter  = make([]ifaces.Column, m.Input.Limbs.NumLimbs())
		inputLimbsResultProjectionFilter   = make([]ifaces.Column, m.Input.Limbs.NumLimbs())
	)

	for i, l := range m.Input.Limbs.ToBigEndianLimbs().GetLimbs() {
		inputLimbsProjectionTable[i] = []ifaces.Column{l}
		inputLimbsBaseProjectionFilter[i] = m.IsBase
		inputLimbsExponentProjectionFilter[i] = m.IsExponent
		inputLimbsModulusProjectionFilter[i] = m.IsModulus
		inputLimbsResultProjectionFilter[i] = m.IsResult
	}

	q := query.ProjectionMultiAryInput{
		ColumnsA: inputLimbsProjectionTable,
		FiltersA: inputLimbsBaseProjectionFilter,
		ColumnsB: columnsB,
		FiltersB: filtersB,
	}

	comp.InsertProjection(ifaces.QueryIDf("%s_PROJ_BASE", m.Name), q)

	// modulus
	columnsB = make([][]ifaces.Column, m.Modulus.NumLimbs())
	for i, l := range m.Modulus.GetLimbs() {
		columnsB[len(columnsB)-1-i] = []ifaces.Column{l}
	}
	filtersB = make([]ifaces.Column, len(columnsB))
	for i := range filtersB {
		filtersB[i] = m.IsFirstLineOfInstance
	}

	q = query.ProjectionMultiAryInput{
		ColumnsA: inputLimbsProjectionTable,
		FiltersA: inputLimbsModulusProjectionFilter,
		ColumnsB: columnsB,
		FiltersB: filtersB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJ_MODULUS", m.Name), q)

	// result
	columnsB = make([][]ifaces.Column, m.CurrAccumulator.NumLimbs())
	for i, l := range m.CurrAccumulator.GetLimbs() {
		columnsB[len(columnsB)-1-i] = []ifaces.Column{l}
	}
	filtersB = make([]ifaces.Column, len(columnsB))
	for i := range filtersB {
		filtersB[i] = m.IsLastLineOfInstance
	}

	q = query.ProjectionMultiAryInput{
		ColumnsA: inputLimbsProjectionTable,
		FiltersA: inputLimbsResultProjectionFilter,
		ColumnsB: columnsB,
		FiltersB: filtersB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJ_RESULT", m.Name), q)

	// exponent
	columnsB = make([][]ifaces.Column, m.Exponent.NumLimbs())
	for i, l := range m.Exponent.GetLimbs() {
		columnsB[len(columnsB)-1-i] = []ifaces.Column{l}
	}
	filtersB = make([]ifaces.Column, len(columnsB))
	for i := range filtersB {
		filtersB[i] = m.IsLastLineOfInstance
	}

	q = query.ProjectionMultiAryInput{
		ColumnsA: inputLimbsProjectionTable,
		FiltersA: inputLimbsExponentProjectionFilter,
		ColumnsB: columnsB,
		FiltersB: filtersB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJ_EXPONENT", m.Name), q)
}

func (m *Modexp) csModexpExponentProjection(comp *wizard.CompiledIOP) {
	// * query that the bits are binary
	for _, l := range m.ExponentBits.GetLimbs() {
		commonconstraints.MustBeBinary(comp, l)
	}
	// at every `limbSize` row the exponent accumulator corresponds to MSB bit. This corresponds
	// to the case where we RSH the exponent limbs by `limbSize` bits.
	comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_EXPONENT_MSB_DECOMP_LIMB_0", m.Name),
		sym.Mul(
			m.IsActive,
			variables.NewPeriodicSample(emulatedLimbSizeBit, 0),
			sym.Sub(
				m.Exponent.GetLimbs()[0],
				m.ExponentBits.GetLimbs()[0],
			),
		),
	)
	// also, we want to ensure that limb shifts are correctly done at every `limbSize` row
	for limbIdx := 1; limbIdx < len(m.Exponent.GetLimbs()); limbIdx++ {
		comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_EXPONENT_MSB_DECOMP_LIMB_%d", m.Name, limbIdx),
			sym.Mul(
				m.IsActive,
				sym.Sub(1, m.IsFirstLineOfInstance),
				variables.NewPeriodicSample(emulatedLimbSizeBit, 0),
				sym.Sub(
					m.Exponent.GetLimbs()[limbIdx],
					column.Shift(m.Exponent.GetLimbs()[limbIdx-1], -1),
				),
			),
		)
	}
	// finally, we ensure that the smallest limb correctly accumulates the exponent bit
	comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_EXPONENT_MSB_DECOMP_ACCUM", m.Name),
		sym.Mul(
			m.IsActive,
			sym.Sub(1, m.IsFirstLineOfInstance),
			sym.Sub(1, variables.NewPeriodicSample(emulatedLimbSizeBit, 0)),
			sym.Sub(
				m.Exponent.GetLimbs()[0],
				sym.Add(
					sym.Mul(2, column.Shift(m.Exponent.GetLimbs()[0], -1)),
					m.ExponentBits.GetLimbs()[0],
				),
			),
		),
	)
}

func (m *Modexp) csConstantBaseAndModulus(comp *wizard.CompiledIOP) {
	// * the base is fixed over all rows of the instance
	for i, l := range m.Base.GetLimbs() {
		comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_BASE_FIXED_LIMB_%d", m.Name, i),
			sym.Mul(
				m.IsActive,
				sym.Sub(1, m.IsFirstLineOfInstance),
				sym.Sub(l, column.Shift(l, -1)),
			),
		)
	}
	// * the modulus is fixed over all rows of the instance
	for i, l := range m.Modulus.GetLimbs() {
		comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_MODULUS_FIXED_LIMB_%d", m.Name, i),
			sym.Mul(
				m.IsActive,
				sym.Sub(1, m.IsFirstLineOfInstance),
				sym.Sub(l, column.Shift(l, -1)),
			),
		)
	}
}

func (m *Modexp) csAccumulatorTransition(comp *wizard.CompiledIOP) {
	// * query that the accumulator is copied correctly from previous to current row
	for i, l := range m.PrevAccumulator.GetLimbs() {
		comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_ACC_TRANSITION_LIMB_%d", m.Name, i),
			sym.Mul(
				m.IsActive,
				sym.Sub(1, m.IsFirstLineOfInstance),
				sym.Sub(l, column.Shift(m.CurrAccumulator.GetLimbs()[i], -1)),
			),
		)
	}
	// * query that the accumulator is initialised to 1 at the first row of the instance
	// LSB limb is 1
	comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_ACC_INIT_LIMB_0", m.Name),
		sym.Mul(
			m.IsActive,
			m.IsFirstLineOfInstance,
			sym.Sub(m.PrevAccumulator.GetLimbs()[0], 1),
		),
	)
	// other limbs are 0
	for i := 1; i < m.PrevAccumulator.NumLimbs(); i++ {
		comp.InsertGlobal(roundNr, ifaces.QueryIDf("%s_ACC_INIT_LIMB_%d", m.Name, i),
			sym.Mul(
				m.IsActive,
				m.IsFirstLineOfInstance,
				m.PrevAccumulator.GetLimbs()[i],
			),
		)
	}
}

func (m *Modexp) Run(run *wizard.ProverRuntime) {
	m.assignMasks(run)
	m.assignLimbs(run)
}

func (m *Modexp) assignMasks(run *wizard.ProverRuntime) {
	var (
		srcIsBase     = m.Module.Input.IsModExpBase.GetColAssignment(run)
		srcIsExponent = m.Module.Input.IsModExpExponent.GetColAssignment(run)
		srcIsModulus  = m.Module.Input.IsModExpModulus.GetColAssignment(run)
		srcIsResult   = m.Module.Input.IsModExpResult.GetColAssignment(run)
		srcToEval     = m.ToEval.GetColAssignment(run)
	)

	var (
		dstIsBase     = smartvectors.Mul(srcToEval, srcIsBase)
		dstIsExponent = smartvectors.Mul(srcToEval, srcIsExponent)
		dstIsModulus  = smartvectors.Mul(srcToEval, srcIsModulus)
		dstIsResult   = smartvectors.Mul(srcToEval, srcIsResult)
	)
	run.AssignColumn(m.IsBase.GetColID(), dstIsBase)
	run.AssignColumn(m.IsExponent.GetColID(), dstIsExponent)
	run.AssignColumn(m.IsModulus.GetColID(), dstIsModulus)
	run.AssignColumn(m.IsResult.GetColID(), dstIsResult)
}

func (m *Modexp) assignLimbs(run *wizard.ProverRuntime) {
	// XXX(ivokub): can parallelize assignments over instances. But seems fast
	// enough not worth it now as adds complexity and synchronization.
	var (
		srcLimbs         = m.Input.Limbs.GetAssignment(run)
		srcIsActiveInput = m.IsActiveFromInput.GetColAssignment(run).IntoRegVecSaveAlloc()
		nbRows           = m.IsActiveFromInput.Size()
	)
	var (
		dstIsFirstLine      = common.NewVectorBuilder(m.IsFirstLineOfInstance)
		dstIsLastLine       = common.NewVectorBuilder(m.IsLastLineOfInstance)
		dstIsActive         = common.NewVectorBuilder(m.IsActive)
		dstPrevAccLimbs     = make([]*common.VectorBuilder, m.PrevAccumulator.NumLimbs())
		dstCurrAccLimbs     = make([]*common.VectorBuilder, m.CurrAccumulator.NumLimbs())
		dstModulusLimbs     = make([]*common.VectorBuilder, m.Modulus.NumLimbs())
		dstExponentBitLimbs = make([]*common.VectorBuilder, m.ExponentBits.NumLimbs())
		dstExponentLimbs    = make([]*common.VectorBuilder, m.Exponent.NumLimbs())
		dstBaseLimbs        = make([]*common.VectorBuilder, m.Base.NumLimbs())
	)
	for i := range dstPrevAccLimbs {
		dstPrevAccLimbs[i] = common.NewVectorBuilder(m.PrevAccumulator.GetLimbs()[i])
	}
	for i := range dstCurrAccLimbs {
		dstCurrAccLimbs[i] = common.NewVectorBuilder(m.CurrAccumulator.GetLimbs()[i])
	}
	for i := range dstModulusLimbs {
		dstModulusLimbs[i] = common.NewVectorBuilder(m.Modulus.GetLimbs()[i])
	}
	for i := range dstExponentBitLimbs {
		dstExponentBitLimbs[i] = common.NewVectorBuilder(m.ExponentBits.GetLimbs()[i])
	}
	for i := range dstExponentLimbs {
		dstExponentLimbs[i] = common.NewVectorBuilder(m.Exponent.GetLimbs()[i])
	}
	for i := range dstBaseLimbs {
		dstBaseLimbs[i] = common.NewVectorBuilder(m.Base.GetLimbs()[i])
	}
	buf := make([]uint64, m.NbLimbs)
	baseBi := new(big.Int)
	exponentBi := new(big.Int)
	modulusBi := new(big.Int)
	expectedBi := new(big.Int)
	exponentBits := make([]*big.Int, m.InstanceSize) // in reversed order MSB first
	for i := range exponentBits {
		exponentBits[i] = new(big.Int)
	}
	currExponentBi := new(big.Int)
	currExponentBiHigh := new(big.Int)

	prevAccumulatorBi := new(big.Int)
	currAccumulatorBi := new(big.Int)

	instMod := make([]field.Element, m.NbLimbs)
	instBase := make([]field.Element, m.NbLimbs)

	expectedZeroPadding := nbLargeModexpLimbs - (m.NbLimbs / limbs.NbLimbU128)

	// scan through all the rows to find the modexp instances
	for ptr := 0; ptr < nbRows; {
		// we didn't find anything, move on
		if srcIsActiveInput[ptr].IsZero() {
			ptr++
			continue
		}
		// found a modexp instance. We read the modexp inputs from here.
		// first we do a sanity-check to see that we have expected number of inputs everywhere:
		if len(srcLimbs[ptr:]) < modexpNumRowsPerInstance {
			utils.Panic("A new modexp is starting but there is not enough rows (ptr=%v len(srcLimbs)=%v)", ptr, len(srcLimbs))
		}
		// we also sanity check that the inputs are consequtively marked as modexp
		for k := range modexpNumRowsPerInstance {
			if srcIsActiveInput[ptr+k].IsZero() {
				utils.Panic("A modexp instance is missing the modexp selector at row %v", ptr+k)
			}
		}
		// regardless if we are on small or large modexp, the arithmetization
		// sends us inputs on nbLargeModexpLimbs rows. So for small modexp we
		// only read the first nbSmallModexpLimbs rows and ignore the rest.
		base := srcLimbs[ptr+expectedZeroPadding : ptr+nbLargeModexpLimbs]
		exponent := srcLimbs[ptr+nbLargeModexpLimbs+expectedZeroPadding : ptr+2*nbLargeModexpLimbs]
		modulus := srcLimbs[ptr+2*nbLargeModexpLimbs+expectedZeroPadding : ptr+3*nbLargeModexpLimbs]
		expected := srcLimbs[ptr+3*nbLargeModexpLimbs+expectedZeroPadding : ptr+4*nbLargeModexpLimbs]

		// assign the big-int values for intermediate computation
		for i := range base {
			baseRow := base[len(base)-1-i]
			copy(buf[limbs.NbLimbU128*i:], baseRow.ToIntegerLimbs())
			copy(instBase[limbs.NbLimbU128*i:], baseRow.ToRawUnsafe())
		}

		if err := emulated.IntLimbRecompose(buf, emulatedLimbSizeBit, baseBi); err != nil {
			utils.Panic("could not convert base limbs to big.Int: %v", err)
		}
		for i := range exponent {
			copy(buf[limbs.NbLimbU128*i:], exponent[len(exponent)-1-i].ToIntegerLimbs())
		}
		if err := emulated.IntLimbRecompose(buf, emulatedLimbSizeBit, exponentBi); err != nil {
			utils.Panic("could not convert exponent limbs to big.Int: %v", err)
		}
		for i := range modulus {
			modRow := modulus[len(modulus)-1-i]
			copy(buf[limbs.NbLimbU128*i:], modRow.ToIntegerLimbs())
			copy(instMod[limbs.NbLimbU128*i:], modRow.ToRawUnsafe())
		}
		if err := emulated.IntLimbRecompose(buf, emulatedLimbSizeBit, modulusBi); err != nil {
			utils.Panic("could not convert modulus limbs to big.Int: %v", err)
		}
		if modulusBi.Sign() == 0 {
			utils.Panic("modulus is zero, buffer: %v, instMod: %v", buf, vector.Prettify(instMod))
		}
		for i := range expected {
			copy(buf[limbs.NbLimbU128*i:], expected[len(expected)-1-i].ToIntegerLimbs())
		}
		if err := emulated.IntLimbRecompose(buf, emulatedLimbSizeBit, expectedBi); err != nil {
			utils.Panic("could not convert result limbs to big.Int: %v", err)
		}
		// extract exponent bits
		for i := 0; i < m.InstanceSize; i++ {
			exponentBits[m.InstanceSize-1-i].SetUint64(uint64(exponentBi.Bit(i)))
		}
		// initialize all intermediate values and assign them
		prevAccumulatorBi.SetInt64(1)
		currExponentBi.SetInt64(0)
		currExponentBiHigh.SetInt64(0)
		for i := range exponentBits {
			switch i {
			case 0:
				// first line of the instance
				dstIsFirstLine.PushOne()
				dstIsLastLine.PushZero()
			case m.InstanceSize - 1:
				// last line of the instance
				dstIsLastLine.PushOne()
				dstIsFirstLine.PushZero()
			default:
				dstIsFirstLine.PushZero()
				dstIsLastLine.PushZero()
			}
			dstIsActive.PushOne()
			currAccumulatorBi.Mul(prevAccumulatorBi, prevAccumulatorBi)
			if exponentBits[i].Sign() == 1 {
				currAccumulatorBi.Mul(currAccumulatorBi, baseBi)
			}
			currAccumulatorBi.Mod(currAccumulatorBi, modulusBi)
			if err := emulated.IntLimbDecompose(prevAccumulatorBi, emulatedLimbSizeBit, buf); err != nil {
				utils.Panic("could not decompose prevAccumulatorBi into limbs: %v", err)
			}
			for j := range m.PrevAccumulator.GetLimbs() {
				var f field.Element
				f.SetUint64(buf[j])
				dstPrevAccLimbs[j].PushField(f)
			}
			if err := emulated.IntLimbDecompose(currAccumulatorBi, emulatedLimbSizeBit, buf); err != nil {
				utils.Panic("could not decompose currAccumulatorBi into limbs: %v", err)
			}
			for j := range m.CurrAccumulator.GetLimbs() {
				var f field.Element
				f.SetUint64(buf[j])
				dstCurrAccLimbs[j].PushField(f)
			}
			// swap
			prevAccumulatorBi.Set(currAccumulatorBi)

			// set the exponent bit
			dstExponentBitLimbs[0].PushField(field.NewElement(exponentBits[i].Uint64()))
			// we should only have one limb anyway, but leave for completeness
			for j := range dstExponentBitLimbs[1:] {
				dstExponentBitLimbs[j+1].PushInt(0)
			}

			// compute the exponent accumulator
			currExponentBi.Lsh(currExponentBi, 1)
			currExponentBi.Add(currExponentBi, exponentBits[i])
			// we need to do the decomposition in two parts:
			//  - lower part for the smallest limb. This we use to show that the bits
			//    are correctly forming the exponent
			//  - upper part for the remaining limbs. This we just use to show that
			//    we copy the exponent limbs correctly
			if err := emulated.IntLimbDecompose(currExponentBi, emulatedLimbSizeBit, buf[:1]); err != nil {
				utils.Panic("could not decompose currExponentBi into limbs: %v", err)
			}
			if err := emulated.IntLimbDecompose(currExponentBiHigh, emulatedLimbSizeBit, buf[1:]); err != nil {
				utils.Panic("could not decompose currExponentBiHigh into limbs: %v", err)
			}
			for j := range m.Exponent.GetLimbs() {
				var f field.Element
				f.SetUint64(buf[j])
				dstExponentLimbs[j].PushField(f)
			}
			// update the high part of the exponent if needed
			if (i+1)%emulatedLimbSizeBit == 0 {
				currExponentBiHigh.Lsh(currExponentBiHigh, emulatedLimbSizeBit)
				currExponentBiHigh.Add(currExponentBiHigh, currExponentBi)
				currExponentBi.SetInt64(0)
			}

			// and also set the constant per MODEXP-instance values
			for j := range instBase {
				dstBaseLimbs[j].PushField(instBase[j])
			}
			for j := range instMod {
				dstModulusLimbs[j].PushField(instMod[j])
			}
		}
		ptr += modexpNumRowsPerInstance
	}
	// commit all built vectors
	dstIsFirstLine.PadAndAssign(run)
	dstIsLastLine.PadAndAssign(run)
	dstIsActive.PadAndAssign(run)
	for i := range dstPrevAccLimbs {
		dstPrevAccLimbs[i].PadAndAssign(run)
	}
	for i := range dstCurrAccLimbs {
		dstCurrAccLimbs[i].PadAndAssign(run)
	}
	for i := range dstModulusLimbs {
		dstModulusLimbs[i].PadAndAssign(run)
	}
	for i := range dstExponentBitLimbs {
		dstExponentBitLimbs[i].PadAndAssign(run)
	}
	for i := range dstExponentLimbs {
		dstExponentLimbs[i].PadAndAssign(run)
	}
	for i := range dstBaseLimbs {
		dstBaseLimbs[i].PadAndAssign(run)
	}
}

package keccakfkoalabear

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

type (
	state        = [5][5][8]ifaces.Column
	stateIn4Bits = [5][5][16]ifaces.Column
	lane         = [8]ifaces.Column
)

type lookupTables struct {
	ColBaseChi   ifaces.Column // dirty BaseChi
	ColBaseTheta ifaces.Column // clean BaseTheta
	ColBase2     ifaces.Column // base 2
}

// BackToThetaOrOutput module, responsible for converting the state from base dirty BaseChi to BaseTheta or to base 2 (output step).
type BackToThetaOrOutput struct {
	// state before applying the base conversion step, in base dirty BaseChi.
	StateCurr state
	// flag used to indicated where to use base theta conversion (continues permutation) or base 2 conversion (output step).
	IsFirstBlock ifaces.Column
	IsActive     ifaces.Column // it indicate active part of the module
	// state after applying the base conversion step, in base clean BaseTheta.
	StateNext state
	// state in the middle of base conversion each lane is divided into two limbs of 4 bits each. This step is to reduce the size of the lookup table.
	StateInternalChi, StateInternalTheta stateIn4Bits
	// it is 1 if the base conversion is to base 2
	IsBase2 ifaces.Column
	// it is 1 if the base conversion is to base theta.
	IsBaseTheta ifaces.Column
	// lookup tables for base conversion
	LookupTable lookupTables
}

// newBackToThetaOrOutput creates a new base conversion module, declares the columns and constraints and returns its pointer
func newBackToThetaOrOutput(comp *wizard.CompiledIOP, stateCurr [5][5]lane, isActive, isFirstBlock ifaces.Column) *BackToThetaOrOutput {

	var (
		bc = &BackToThetaOrOutput{
			StateCurr:    stateCurr,
			IsFirstBlock: isFirstBlock,
			IsActive:     isActive,
		}
		size = stateCurr[0][0][0].Size()
	)
	// declare the lookup table columns
	dirtyBaseChi, cleanBaseTheta, cleanBase2 := createLookupTablesChiToTheta()
	bc.LookupTable.ColBaseChi = comp.InsertPrecomputed(ifaces.ColID("BC_LOOKUP_DIRTY_BASECHI"), dirtyBaseChi)
	bc.LookupTable.ColBase2 = comp.InsertPrecomputed(ifaces.ColID("BC_LOOKUP_BASE2"), cleanBase2)
	bc.LookupTable.ColBaseTheta = comp.InsertPrecomputed(ifaces.ColID("BC_LOOKUP_CLEAN_BASE_THETA"), cleanBaseTheta)

	// declare the flags for base conversion
	bc.IsBase2 = comp.InsertCommit(0, ifaces.ColID("BC_IS_BASE2"), size, true)
	bc.IsBaseTheta = comp.InsertCommit(0, ifaces.ColID("BC_IS_BASETHETA"), size, true)

	// declare the columns for the new and internal state
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 16; z++ {

				if z < 8 {
					bc.StateNext[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("BC_STATE_NEXT_%v_%v_%v", x, y, z), size, true)
				}

				bc.StateInternalChi[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("BC_STATE_INTERNAL_CHI_%v_%v_%v", x, y, z), size, true)

				bc.StateInternalTheta[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("BC_STATE_INTERNAL_THETA_OR_BASE2_%v_%v_%v", x, y, z), size, true)

				// attest the relation between stateInternalChi and stateInternalTheta using the lookup table
				comp.InsertInclusionConditionalOnIncluded(0,
					ifaces.QueryIDf("BC_LOOKUP_INCLUSION_%v_%v_%v", x, y, z),
					[]ifaces.Column{bc.LookupTable.ColBaseTheta, bc.LookupTable.ColBaseChi},
					[]ifaces.Column{bc.StateInternalTheta[x][y][z], bc.StateInternalChi[x][y][z]},
					bc.IsBaseTheta)

				comp.InsertInclusionConditionalOnIncluded(0,
					ifaces.QueryIDf("BC_LOOKUP_INCLUSION_BASE2_%v_%v_%v", x, y, z),
					[]ifaces.Column{bc.LookupTable.ColBaseChi, bc.LookupTable.ColBase2},
					[]ifaces.Column{bc.StateInternalChi[x][y][z], bc.StateInternalTheta[x][y][z]},
					bc.IsBase2)

			}

			for z := 0; z < 8; z++ {
				// assert that stateCurr is decomposed correctly into two slices of stateInternalChi
				comp.InsertGlobal(0, ifaces.QueryIDf("BC_RECOMPOSE_CHI_%v_%v_%v", x, y, z),
					sym.Sub(bc.StateCurr[x][y][z],
						sym.Add(bc.StateInternalChi[x][y][2*z],
							sym.Mul(bc.StateInternalChi[x][y][2*z+1], kcommon.BaseChi4),
						),
					),
				)

				baseThetaOrOutPut := sym.Add(
					sym.Mul(bc.IsBaseTheta, kcommon.BaseTheta4),
					sym.Mul(bc.IsBase2, 16),
				)

				comp.InsertGlobal(0, ifaces.QueryIDf("BC_RECOMPOSE_THETA_%v_%v_%v", x, y, z),
					sym.Sub(bc.StateNext[x][y][z],
						sym.Add(bc.StateInternalTheta[x][y][2*z],
							sym.Mul(bc.StateInternalTheta[x][y][2*z+1], baseThetaOrOutPut),
						),
					),
				)
			}
		}
	}
	commonconstraints.MustBeMutuallyExclusiveBinaryFlags(comp,
		bc.IsActive,
		[]ifaces.Column{bc.IsBase2, bc.IsBaseTheta},
	)

	// the previous row before a newHash or the last active row indicates the output step (base 2 conversion)
	// if  isFirstBlock[i] ==0 && isFirstBlock[i+1] == 1 --> isBase2[i] == 1
	// if isActive[i] == 1 and isActive[i+1] == 0 --> isBase2[i] == 1
	// if isActive[last-row] == 1 --> isBase2[last-row] == 1
	comp.InsertGlobal(
		0,
		ifaces.QueryID("BC_ISBASE2_BEFORE_NEWHASH"),
		sym.Mul(
			sym.Sub(1, bc.IsFirstBlock),
			column.Shift(bc.IsFirstBlock, 1),
			sym.Sub(1, bc.IsBase2),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryID("BC_ISBASE2_AT_LASTACTIVE"),
		sym.Mul(
			bc.IsActive,
			sym.Sub(1, column.Shift(bc.IsActive, 1)),
			sym.Sub(1, bc.IsBase2),
		),
	)

	comp.InsertLocal(
		0,
		ifaces.QueryID("BC_ISBASE2_AT_END"),
		sym.Mul(
			column.Shift(bc.IsActive, -1),
			sym.Sub(1, column.Shift(bc.IsBase2, -1)),
		),
	)

	return bc
}

// assignBaseConversion assigns the values to the columns of base conversion step.
func (bc *BackToThetaOrOutput) Run(run *wizard.ProverRuntime) {
	// decompose each bytes of the lane into 4 bits (base 12)
	var (
		size                  = bc.StateCurr[0][0][0].Size()
		isBase2               = common.NewVectorBuilder(bc.IsBase2)
		isBaseT               = common.NewVectorBuilder(bc.IsBaseTheta)
		isActive              = run.GetColumn(bc.IsActive.GetColID()).IntoRegVecSaveAlloc()
		isFirstBlock          = run.GetColumn(bc.IsFirstBlock.GetColID()).IntoRegVecSaveAlloc()
		baseThetaOr2, temp    = make([]field.Element, size), make([]field.Element, size)
		baseTheta4or16, temp4 = make([]field.Element, size), make([]field.Element, size)
	)

	// assign isBase2 and isBaseTheta
	for i := 0; i < size; i++ {
		if i+1 < size && isActive[i+1].IsOne() {
			if isFirstBlock[i].IsZero() && isFirstBlock[i+1].IsOne() {
				isBase2.PushOne()
				isBaseT.PushZero()
			} else {
				isBase2.PushZero()
				isBaseT.PushOne()
			}
		} else {
			if isActive[i].IsOne() {
				isBase2.PushOne()
				isBaseT.PushZero()
			} else {
				isBase2.PushZero()
				isBaseT.PushZero()
			}
		}
	}
	isBase2.PadAndAssign(run)
	isBaseT.PadAndAssign(run)

	// compute baseThetaOr2 = isBaseTheta * BaseTheta + isBase2 * 2
	vector.ScalarMul(baseThetaOr2, isBaseT.Slice(), kcommon.BaseThetaFr)
	vector.ScalarMul(temp, isBase2.Slice(), kcommon.Base2Fr)
	vector.Add(baseThetaOr2, baseThetaOr2, temp)

	// compute baseTheta4or16 = isBaseTheta * BaseTheta4 + isBase2 * 16
	vector.ScalarMul(baseTheta4or16, isBaseT.Slice(), kcommon.BaseTheta4Fr)
	vector.ScalarMul(temp4, isBase2.Slice(), field.NewElement(16))
	vector.Add(baseTheta4or16, baseTheta4or16, temp4)

	// assign  the internal states
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				col := bc.StateCurr[x][y][z].GetColAssignment(run).IntoRegVecSaveAlloc()

				for i := range col {
					if col[i].Uint64() >= kcommon.BaseChi4*kcommon.BaseChi4 {
						panic("base conversion: input is out of range")
					}
				}
				// decompose in base BaseChi4 (2 chunks)
				v := kcommon.DecomposeCol(col, kcommon.BaseChi4, 2)
				q := v[1] // high limb
				r := v[0] // low limb

				// set the low limb (4 digits)
				run.AssignColumn(bc.StateInternalChi[x][y][2*z].GetColID(), smartvectors.NewRegular(r))
				// set the high limb (4 digits)
				run.AssignColumn(bc.StateInternalChi[x][y][2*z+1].GetColID(), smartvectors.NewRegular(q))
				// decompose in base BaseChi and convert to bits
				bitsR := kcommon.DecomposeAndCleanCol(r, kcommon.BaseChi, 4)
				bitsQ := kcommon.DecomposeAndCleanCol(q, kcommon.BaseChi, 4)
				// recompose in base BaseTheta if isBaseTheta == 1 and in base 2 if isBase2 == 1
				lowLimb := kcommon.RecomposeCols(bitsR, baseThetaOr2)
				highLimb := kcommon.RecomposeCols(bitsQ, baseThetaOr2)
				// set stateInternalTheta
				run.AssignColumn(bc.StateInternalTheta[x][y][2*z].GetColID(), smartvectors.NewRegular(lowLimb))
				run.AssignColumn(bc.StateInternalTheta[x][y][2*z+1].GetColID(), smartvectors.NewRegular(highLimb))

				// set StateNext (8 digits)
				var recomposed = make([]field.Element, size)
				vector.MulElementWise(recomposed, highLimb, baseTheta4or16)
				vector.Add(recomposed, recomposed, lowLimb)

				run.AssignColumn(bc.StateNext[x][y][z].GetColID(), smartvectors.NewRegular(recomposed))
			}
		}
	}
}

func createLookupTablesChiToTheta() (dirtyChi, cleanTheta, cleanBase2 smartvectors.SmartVector) {
	var (
		lookupDirtyBaseChi               []field.Element
		lookupCleanBaseTheta             []field.Element
		lookupCleanBase2                 []field.Element
		cleanValueTheta, cleanValueBase2 field.Element
		targetSize                       = utils.NextPowerOfTwo(kcommon.BaseChi4)
	)

	// for each value in base dirty BaseChi (0 to 14640), compute its equivalent in base clean BaseTheta
	for i := 0; i < kcommon.BaseChi4; i++ {
		// decompose in base BaseChi (4 digits) and clean it
		v := kcommon.DecomposeAndCleanFr(field.NewElement(uint64(i)), kcommon.BaseChi, 4)
		// recompose in base BaseTheta
		cleanValueTheta = kcommon.RecomposeRow(v, &kcommon.BaseThetaFr)
		// recompose in base 2.
		cleanValueBase2 = kcommon.RecomposeRow(v, &kcommon.Base2Fr)

		lookupDirtyBaseChi = append(lookupDirtyBaseChi, field.NewElement(uint64(i)))
		lookupCleanBaseTheta = append(lookupCleanBaseTheta, cleanValueTheta)
		lookupCleanBase2 = append(lookupCleanBase2, cleanValueBase2)
	}
	dirtyChi = smartvectors.RightPadded(lookupDirtyBaseChi, field.NewElement(kcommon.BaseChi4-1), targetSize)
	cleanTheta = smartvectors.RightPadded(lookupCleanBaseTheta, cleanValueTheta, targetSize)
	cleanBase2 = smartvectors.RightPadded(lookupCleanBase2, cleanValueBase2, targetSize)
	return dirtyChi, cleanTheta, cleanBase2
}

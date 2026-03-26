package packing

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing/dedicated"
)

// laneRepackingInputs collects the inputs of the [newLane] function.
type laneRepackingInputs struct {
	Spaghetti spaghettiCtx
	PckInp    PackingInput
}

// laneRepacking stores all the intermediate columns required to constraint the repacking.
type laneRepacking struct {
	Inputs *laneRepackingInputs
	// the set of lanes, they all have the same size
	Lanes ifaces.Column
	// the ProverAction for accumulateUpToMax
	PAAccUpToMax wizard.ProverAction
	// the result of ProverAction for accumulateUpToMax
	// It is one if the lane is complete
	IsLaneComplete ifaces.Column
	// the coefficient  for computing lanes from decomposedLimbs
	Coeff ifaces.Column
	// it is 1 on the effective part of lane module.
	IsLaneActive ifaces.Column
	// It indicate where on the lane-column the new hash begins. 1 if it is the beginning of a new hash.
	IsBeginningOfNewHash ifaces.Column
	// the size of the columns in this submodule.
	Size int
	// As MAXNBYTE=2 we can't use one row for each lane. So this value
	// is equal to number of rows per one lane.
	RowsPerLane int
}

// It imposes all the constraints for correct repacking of decompsedLimbs into lanes.
func newLane(comp *wizard.CompiledIOP, spaghetti spaghettiCtx, pckInp PackingInput) laneRepacking {
	var (
		rowsPerLane           = (pckInp.PackingParam.LaneSizeBytes() + MAXNBYTE - 1) / MAXNBYTE
		size                  = utils.NextPowerOfTwo(pckInp.PackingParam.NbOfLanesPerBlock() * pckInp.MaxNumBlocks * rowsPerLane)
		createCol             = common.CreateColFn(comp, LANE+"_"+pckInp.Name, size, pragmas.RightPadded)
		isFirstSliceOfNewHash = spaghetti.NewHashSp
		decomposedLenSp       = spaghetti.DecLenSp
		spaghettiSize         = spaghetti.SpaghettiSize
		pa                    = dedicated.AccumulateUpToMax(comp, MAXNBYTE, decomposedLenSp, spaghetti.FilterSpaghetti)
	)

	l := laneRepacking{
		Inputs: &laneRepackingInputs{
			PckInp:    pckInp,
			Spaghetti: spaghetti,
		},

		Lanes:                createCol("Lane"),
		IsBeginningOfNewHash: createCol("IsFirstLaneOfNewHash"),
		IsLaneActive:         createCol("IsLaneActive"),
		Coeff:                comp.InsertCommit(0, ifaces.ColID("Coefficient_"+pckInp.Name), spaghettiSize, true),

		PAAccUpToMax:   pa,
		IsLaneComplete: pa.IsMax,
		Size:           size,
		RowsPerLane:    rowsPerLane,
	}

	// Declare the constraints
	commonconstraints.MustBeActivationColumns(comp, l.IsLaneActive)

	// Constraints on Coeff, it is a accumulator of decomposedLenPowers over isLaneComplete.
	// coef[last_active_row] := 1
	// coeff[i] := coeff[i+1] * decomposedLenPowers[i+1] * (1-isLaneComplete[i]) + isLaneComplete[i]
	l.csCoeff(comp, spaghetti)

	// Constraints on the Recomposition of slices into the lanes
	l.csRecomposeToLanes(comp, spaghetti)

	// constraints over isFirstLaneOfNewHash
	// Project the isFirstLaneOfNewHash from isFirstSliceOfNewHash
	comp.InsertProjection(ifaces.QueryID("Project_IsFirstLaneOfHash_"+pckInp.Name),
		query.ProjectionInput{ColumnA: []ifaces.Column{isFirstSliceOfNewHash},
			ColumnB: []ifaces.Column{l.IsBeginningOfNewHash},
			FilterA: l.IsLaneComplete,
			FilterB: l.IsLaneActive})
	return l
}

// it declares the constraints over coefficients.
func (l *laneRepacking) csCoeff(comp *wizard.CompiledIOP, s spaghettiCtx) {
	var (
		partialCoeff = s.DecLenPowerSp // decomposedLenPowers in spaghetti form.
		isActive     = s.FilterSpaghetti
	)

	// coeff[last-active-row] = 1
	comp.InsertGlobal(
		0, ifaces.QueryIDf("%v_Coeff_In_Last_Active_Row", l.Inputs.PckInp.Name),
		sym.Mul(isActive,
			sym.Sub(column.Shift(isActive, 1), 1),
			sym.Sub(l.Coeff,
				1)),
	)

	// coeff[last] = 1 // to cover the case where; last-active-row ==  last-row
	comp.InsertLocal(
		0, ifaces.QueryIDf("%v_Coeff-In_Last_Row", l.Inputs.PckInp.Name),
		sym.Sub(column.Shift(l.Coeff, -1), column.Shift(isActive, -1)),
	)

	// coeff[i] := coeff[i+1] * partialCoeff[i+1] * (1-isLaneComplete[i+1]) + isLaneComplete[i+1]
	res := sym.Mul(
		column.Shift(l.Coeff, 1),
		column.Shift(partialCoeff, 1),
		sym.Sub(1, column.Shift(l.IsLaneComplete, 1)),
	)
	res = sym.Add(res, column.Shift(l.IsLaneComplete, 1))
	expr := sym.Mul(sym.Sub(l.Coeff, res), column.Shift(isActive, 1))
	comp.InsertGlobal(0, ifaces.QueryIDf("%v_Coefficient_Glob", l.Inputs.PckInp.Name), expr)
}

// It declares the constraints over the lanes
// Lanes are the recomposition of slices.
func (l *laneRepacking) csRecomposeToLanes(comp *wizard.CompiledIOP, s spaghettiCtx) {
	// compute the partitioned inner product
	//ipTaker[i] = (decomposedLimbs[i] * coeff[i]) + ipTracker[i+1]* (1- isLaneComplete[i+1])
	// Constraints on the Partitioned Inner-Products
	ipTracker := dedicated.InsertPartitionedIP(comp, l.Inputs.PckInp.Name+"_PIP_For_LaneRePacking",
		s.CleanLimbSp,
		l.Coeff,
		l.IsLaneComplete,
	)

	// Project the lanes from ipTracker over the Lane column.
	comp.InsertProjection(ifaces.QueryIDf("%v_ProjectOverLanes", l.Inputs.PckInp.Name),
		query.ProjectionInput{ColumnA: []ifaces.Column{ipTracker},
			ColumnB: []ifaces.Column{l.Lanes},
			FilterA: l.IsLaneComplete,
			FilterB: l.IsLaneActive})
}

// It assigns the columns specific to the submodule
func (l *laneRepacking) Assign(run *wizard.ProverRuntime) {
	// assign the spaghetti forms
	l.Inputs.Spaghetti.PA.Run(run)
	// assign the IsMax column from  the ProverAction (AccumulateUpToMax).
	l.PAAccUpToMax.Run(run)
	// assign coeff
	l.assignCoeff(run)
	// assign the Lanes, isFirstLaneofNewHash , IsLaneActive
	l.assignLane(run)
}

// It assigns column coeff
func (l *laneRepacking) assignCoeff(
	run *wizard.ProverRuntime) {

	var (
		isLaneComplete       = l.IsLaneComplete.GetColAssignment(run).IntoRegVecSaveAlloc()
		size                 = len(isLaneComplete)
		decomposedLenPowerSp = l.Inputs.Spaghetti.DecLenPowerSp
		partialCoeff         = decomposedLenPowerSp.GetColAssignment(run).IntoRegVecSaveAlloc()
		one                  = field.One()
		isActive             = l.Inputs.Spaghetti.FilterSpaghetti.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	//partialCoeff := decomposedLenPowers
	// partialCoeff = 0 over s.isActive = 0
	// coeff[i] := coeff[i+1] * partialCoeff[i+1] * (1-isLaneComplete[i+1]) + isLaneComplete[i+1]
	coeff := make([]field.Element, size)

	coeff[size-1] = isActive[size-1]

	var res, notComplete field.Element
	for i := size - 2; i >= 0; i-- {
		notComplete.Sub(&one, &isLaneComplete[i+1])
		res.Mul(&notComplete, &partialCoeff[i+1])
		res.Mul(&res, &coeff[i+1])
		coeff[i].Add(&res, &isLaneComplete[i+1])

		if isActive[i].IsOne() && isActive[i+1].IsZero() {
			coeff[i] = field.One()
		}
	}

	// assign the columns
	run.AssignColumn(l.Coeff.GetColID(), smartvectors.RightZeroPadded(coeff, size))
}

// it assigns the lanes
func (l *laneRepacking) assignLane(run *wizard.ProverRuntime) {
	var (
		lane                 = common.NewVectorBuilder(l.Lanes)
		param                = l.Inputs.PckInp.PackingParam
		isFirstLaneofNewHash = common.NewVectorBuilder(l.IsBeginningOfNewHash)
		isActive             = common.NewVectorBuilder(l.IsLaneActive)
		nbLaneBytes          = param.LaneSizeBytes()
		blocks, flag         = l.getBlocks(run, l.Inputs.PckInp)
	)
	var f field.Element

	if len(flag) != len(blocks) {
		utils.Panic("should have one flag per block numFlags=%v numBlocks=%v", len(flag), len(blocks))
	}

	for k, block := range blocks {
		if len(block) != param.BlockSizeBytes() {
			utils.Panic("blocks should be of size %v, but it is of size %v", param.BlockSizeBytes(), len(block))
		}
		for j := 0; j < param.NbOfLanesPerBlock(); j++ {
			laneBytes := block[j*nbLaneBytes : j*nbLaneBytes+nbLaneBytes]

			leftLaneBytes := nbLaneBytes
			for i := range l.RowsPerLane {
				offset := min(leftLaneBytes, MAXNBYTE)

				laneBytesPerRow := laneBytes[i*MAXNBYTE : i*MAXNBYTE+offset]
				f.SetBytes(laneBytesPerRow)
				leftLaneBytes -= offset

				lane.PushField(f)
				if flag[k] == 1 && j == 0 && i == 0 {
					isFirstLaneofNewHash.PushInt(1)
				} else {
					isFirstLaneofNewHash.PushInt(0)
				}
				isActive.PushInt(1)
			}
		}

	}
	lane.PadAndAssign(run, field.Zero())
	isFirstLaneofNewHash.PadAndAssign(run, field.Zero())
	isActive.PadAndAssign(run, field.Zero())
}

//	It outputs the expected blocks.
//
// The expected blocks are used for the assignments of lanes.
func (l *laneRepacking) getBlocks(run *wizard.ProverRuntime, inp PackingInput) ([][]byte, []int) {

	var (
		s = 0
		// counter for the number of blocks
		ctr              = 0
		blockSize        = inp.PackingParam.BlockSizeBytes()
		imported         = inp.Imported
		limbs            = make([][]field.Element, len(imported.Limb))
		nBytes           = smartvectors.Window(imported.NByte.GetColAssignment(run))
		decomposedNBytes = decomposeNByte(nBytes)
		isNewHash        = smartvectors.Window(imported.IsNewHash.GetColAssignment(run))
	)
	for i := range common.NbLimbU128 {
		limbs[i] = smartvectors.Window(imported.Limb[i].GetColAssignment(run))
	}
	nbRows := len(limbs[0])

	var stream []byte
	var block [][]byte
	var isFirstBlockOfHash []int
	isFirstBlockOfHash = append(isFirstBlockOfHash, 1)
	for pos := 0; pos < nbRows; pos++ {
		nbyte := field.ToInt(&nBytes[pos])
		s = s + nbyte

		// Serialize the limb value from 8 left-aligned 2-byte values into one "nbyte" byte array
		var usefulByte []byte
		for i := range limbs {
			limbNByte := decomposedNBytes[i][pos]
			limbBytes := limbs[i][pos].Bytes()
			usefulByte = append(usefulByte, limbBytes[LEFT_ALIGNMENT:LEFT_ALIGNMENT+limbNByte]...)
		}

		// SANITY CHECK
		utils.Require(len(usefulByte) == nbyte, "invalid length of usefulByte %d != %d", len(usefulByte), nbyte)

		if s > blockSize || s == blockSize {
			// extra part that should be moved to the next block
			s = s - blockSize
			res := usefulByte[:(nbyte - s)]
			newBlock := append(stream, res...)
			if len(newBlock) != blockSize {
				utils.Panic("could not extract the new Block")
			}
			block = append(block, newBlock)
			if pos+1 != nbRows && isNewHash[pos+1].IsOne() {
				// the next block is the first block of hash
				isFirstBlockOfHash = append(isFirstBlockOfHash, 1)
			} else if pos+1 != nbRows {
				isFirstBlockOfHash = append(isFirstBlockOfHash, 0)
			}
			stream = make([]byte, s)
			copy(stream, usefulByte[(nbyte-s):])
			ctr++
		} else {
			stream = append(stream, usefulByte[:]...)
		}
		// If we are on the last limb or if it is a new hash
		// the steam should be empty,
		// unless \sum_i NByte_i does not divide the blockSize (for i from the same hash)
		if pos+1 == nbRows || isNewHash[pos+1].Uint64() == 1 {
			if len(stream) != 0 {
				utils.Panic("The stream-length should be zero before launching a new hash/batch len(stream) = %v", len(stream))
			}
		}
		if ctr > inp.MaxNumBlocks {
			exit.OnLimitOverflow(
				inp.MaxNumBlocks,
				ctr,
				fmt.Errorf("too many block keccack - the number of blocks %v passes the limit %v", ctr, inp.MaxNumBlocks),
			)
		}
	}

	// This corresponds to the edge-case were no blocks are being processed. In
	// that situation we can simply return empty lists. This addressment is
	// necessary because by default, [isFirstBlockOfHash] is initialized with
	// one value while the blocks are not.
	if len(block) == 0 {
		return [][]byte{}, []int{}
	}

	return block, isFirstBlockOfHash
}

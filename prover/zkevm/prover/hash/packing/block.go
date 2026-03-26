package packing

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// blockInput stores the inputs for [newBlock] function.
type blockInput struct {
	Lanes laneRepacking
	Param generic.HashingUsecase
}

type block struct {
	Inputs *blockInput
	// it is 1 iff the block is complete
	IsBlockComplete ifaces.Column
	// accumulator for the number of lanes
	AccNumLane ifaces.Column
	// Size of the submodule
	Size int
	// the ProverAction for Iszero()
	PA wizard.ProverAction
}

// newBlock imposes the constraints  that the total sum of nBytes, given via imported.NBytes,
// indeed divides the blockSize.
func newBlock(comp *wizard.CompiledIOP, inp blockInput) block {

	var (
		name           = inp.Lanes.Inputs.PckInp.Name
		size           = inp.Lanes.Size
		createCol      = common.CreateColFn(comp, BLOCK+"_"+name, size, pragmas.RightPadded)
		isLaneActive   = inp.Lanes.IsLaneActive
		nbRowsPerBlock = inp.Param.NbOfLanesPerBlock() * inp.Lanes.RowsPerLane
	)

	b := block{
		Size:   inp.Lanes.Size,
		Inputs: &inp,

		AccNumLane: createCol("AccNumLane"),
	}

	b.IsBlockComplete, b.PA = dedicated.IsZero(comp, sym.Sub(b.AccNumLane, nbRowsPerBlock)).GetColumnAndProverAction()

	// constraints over accNumLanes (accumulate backward)
	comp.InsertLocal(0, ifaces.QueryID(name+"_AccNumLane_Last"),
		sym.Sub(column.Shift(b.AccNumLane, -1),
			column.Shift(isLaneActive, -1)),
	)

	// accNumLanes[i] = accNumLane[i+1]*(1-isBlockComplete[i+1]) + isLaneActive[i]
	res := sym.Sub(1, column.Shift(b.IsBlockComplete, 1)) // 1-isBlockComplete[i+1]

	expr :=
		sym.Sub(
			sym.Add(
				sym.Mul(
					column.Shift(b.AccNumLane, 1), res),
				isLaneActive),
			b.AccNumLane,
		)

	comp.InsertGlobal(0, ifaces.QueryID(name+"_AccNumLane_Glob"), expr)

	// isBlockComplete[0] = 1
	// NB: this guarantees that the total sum of  nybtes ,given via imported.Nbytes,
	// indeed divides the blockSize.
	// This fact can be used to guarantee that enough zeroes where padded during padding.
	comp.InsertLocal(
		0, ifaces.QueryID(name+"_IsBlockComplete"),
		sym.Mul(
			isLaneActive,
			sym.Sub(1, b.IsBlockComplete),
		),
	)

	// if isFirstLaneOfNewHash = 1 then isBlockComplete = 1.
	comp.InsertGlobal(0, ifaces.QueryID(name+"_EACH_HASH_HAS_COMPLETE_BLOCKS"),
		sym.Mul(
			inp.Lanes.IsBeginningOfNewHash,
			sym.Sub(1, b.IsBlockComplete),
		),
	)

	return b
}

func (b *block) Assign(run *wizard.ProverRuntime) {

	var (
		size             = b.Size
		accNumLane       = make([]field.Element, size)
		isActive         = b.Inputs.Lanes.IsLaneActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		nbOfRowsPerBlock = b.Inputs.Param.NbOfLanesPerBlock() * b.Inputs.Lanes.RowsPerLane
	)
	accNumLane[size-1] = isActive[size-1]
	// accNumLanes[i] = accNumLane[i+1]*(1-isBlockComplete[i+1]) + isLaneActive[i]
	for row := size - 2; row >= 0; row-- {
		if field.ToInt(&accNumLane[row+1]) == nbOfRowsPerBlock {
			accNumLane[row] = field.One()
		} else {
			accNumLane[row].Add(&isActive[row], &accNumLane[row+1])
		}
	}
	run.AssignColumn(b.AccNumLane.GetColID(),
		smartvectors.RightZeroPadded(accNumLane, size))

	b.PA.Run(run)
}

// it creates a blockInputs object.
func getBlockInputs(lane laneRepacking, param generic.HashingUsecase) blockInput {
	return blockInput{
		Lanes: lane,
		Param: param,
	}
}

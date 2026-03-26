package baseconversion

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/assert"
)

func TestKeccakFBlockPreparation(t *testing.T) {
	var (
		b    = &ToBaseX{}
		size = 8
	)

	define := func(build *wizard.Builder) {
		var (
			comp      = build.CompiledIOP
			createCol = common.CreateColFn(comp, "BASE_CONVERSION_TEST", size, pragmas.RightPadded)
		)

		inp := ToBaseXInputs{
			Lane:           createCol("LANE"),
			IsLaneActive:   createCol("IS_ACTIVE"),
			BaseX:          []int{3, 5, 7},
			NbBitsPerBaseX: 4,
			IsBaseX:        []ifaces.Column{},
		}
		for i := range inp.BaseX {
			inp.IsBaseX = append(inp.IsBaseX, createCol("IS_BASEX_%v", i))
		}

		b = NewToBaseX(comp, inp)

	}
	prover := func(run *wizard.ProverRuntime) {

		var (
			lane          = common.NewVectorBuilder(b.Inputs.Lane)
			isActive      = common.NewVectorBuilder(b.Inputs.IsLaneActive)
			effectiveSize = size - 3 // leave some padding at the end
			isBaseX       = make([]*common.VectorBuilder, len(b.Inputs.BaseX))
			isBaseXFr     = make([][]field.Element, len(b.Inputs.BaseX))
		)

		for i := range b.Inputs.BaseX {
			isBaseX[i] = common.NewVectorBuilder(b.Inputs.IsBaseX[i])
		}

		for row := 0; row < effectiveSize; row++ {
			f := field.NewElement(0x35ca) // bytes = [0xca, 0x35]
			lane.PushField(f)
			isActive.PushInt(1)
			if row%3 == 0 {
				isBaseX[0].PushInt(1)
				isBaseX[1].PushInt(0)
				isBaseX[2].PushInt(0)
			}
			if row%3 == 1 {
				isBaseX[0].PushInt(0)
				isBaseX[1].PushInt(1)
				isBaseX[2].PushInt(0)
			}
			if row%3 == 2 {
				isBaseX[0].PushInt(0)
				isBaseX[1].PushInt(0)
				isBaseX[2].PushInt(1)
			}

		}

		lane.PadAndAssign(run, field.Zero())
		isActive.PadAndAssign(run, field.Zero())
		for i := range b.Inputs.BaseX {
			isBaseX[i].PadAndAssign(run, field.Zero())
		}

		b.Run(run)

		// verify output blocks here, see the example below
		expected0 := []uint64{4, 10, 36, 30}
		expected1 := []uint64{6, 26, 150, 130}
		expected3 := []uint64{8, 50, 392, 350}

		laneX := make([][]field.Element, len(b.LaneX))
		for i := range b.LaneX {
			laneX[i] = run.GetColumn(b.LaneX[i].GetColID()).IntoRegVecSaveAlloc()
		}
		for i := range b.Inputs.BaseX {
			isBaseXFr[i] = isBaseX[i].Slice()
		}

		for row := 0; row < size; row++ {
			for i := 0; i < len(b.LaneX); i++ {
				val := laneX[i][row]
				if row%3 == 0 && row < effectiveSize {
					assert.Equalf(t, fmt.Sprintf("%d", expected0[i]),
						fmt.Sprintf("%d", val.Uint64()), "invalid base conversion at laneX[%d] row %d is %d", i, 0, val.Uint64())
				}
				if row%3 == 1 && row < effectiveSize {
					assert.Equalf(t, fmt.Sprintf("%d", expected1[i]),
						fmt.Sprintf("%d", val.Uint64()), "invalid base conversion at laneX[%d] row %d is %d", i, 0, val.Uint64())
				}
				if row%3 == 2 && row < effectiveSize {
					assert.Equalf(t, fmt.Sprintf("%d", expected3[i]),
						fmt.Sprintf("%d", val.Uint64()), "invalid base conversion at laneX[%d] row %d is %d", i, 0, val.Uint64())
				}
				if row >= effectiveSize {
					assert.Equalf(t, "0",
						fmt.Sprintf("%d", val.Uint64()), "invalid base conversion at laneX[%d] row %d is %d", i, 0, val.Uint64())
				}
			}
		}

	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")

}

//  Example:
//  input: 0x35ca
//	inputBytes := []byte{0b11001010, 0b00110101}
//	bitsPerChunk := 4
//	nbSlices := 4
//	base := uint64(3)
//
// Combined bitstream (LSB-first):
//
//	[0 1 0 1 0 0 1 1 1 0 1 0 1 1 0 0]
//
// Grouped into 4-bit chunks (msb first):
//
//	chunk 3 → [0 1 0 1]
//	chunk 2 → [0 0 1 1]
//	chunk 1 → [1 0 1 0]
//	chunk 0 → [1 1 0 0]
//
// Interpretation in base 3 (each bit contributes bit_j * 3^j):
//
//	val[3] = 0*3^0 + 1*3^1 + 0*3^2 + 1*3^3 = 30
//	val[2] = 0*3^0 + 0*3^1 + 1*3^2 + 1*3^3 = 36
//	val[1] = 1*3^0 + 0*3^1 + 1*3^2 + 0*3^3 = 10
//	val[0] = 1*3^0 + 1*3^1 + 0*3^2 + 0*3^3 = 4
//
// So the function returns:
//	[]uint64{4,10,36,30}
//
// and in base 5:
//  []uint64{6, 26, 130, 130}
//
// and in base 7:
//  []uint64{8, 50, 392, 350}
//

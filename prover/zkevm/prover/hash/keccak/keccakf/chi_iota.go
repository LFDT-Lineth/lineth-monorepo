package keccakfkoalabear

import (
	"strconv"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

type block [common.NumLanesInBlock][common.NumSlices]ifaces.Column

type chiIota struct {
	Inputs *chiInputs
	// internal state  recomposing each [common.NumSlices] bits into a base clean BaseChi.
	// it is a placeholder for the linear combination expressions, to facilitate the assignment.
	StateInternal [5][5][common.NumSlices]*sym.Expression
	// state after applying the chi step.
	// It is in the expression form since it will be combined with Iota step
	// to get the standard state later. This avoid declaring extra columns.
	StateNext [5][5][common.NumSlices]ifaces.Column
	// prover actions for linear combinations
	// paLinearCombinations [5][5][common.NumSlices]*protocols.LinearCombination
	// the round constant
	Rc [common.NumSlices]*dedicated.RepeatedPattern
}

type chiInputs struct {
	// state before applying the chi step
	StateCurr common.StateInBits
	// it contains the first block of the message at position 0 mod 24,
	// and any other block at position 0 mod 23.
	Blocks block
	// flag for blocks other than the first one
	IsBlockOther ifaces.Column
	// max number of keccakf permutations that module can support
	KeccakfSize int
}

// newChi define the chi step of the keccak-f permutation.
// It creates the necessary columns and constraints to enforce the chi step.
// the state is updated as follows:
// A[x][y] = A[x][y] + ( (not A[x+1][y]) and A[x+2][y] )  and
// A[0,0] = A[0,0] + RC and then A[x][y] = A[x][y] + block[x+5*y] for all message blocks except the first one.
// the blocks are added to the end of the current Iota step to avoid creating extra columns and facilitating the theta step.
func newChi(comp *wizard.CompiledIOP, in chiInputs) *chiIota {

	chi := &chiIota{
		StateInternal: [5][5][common.NumSlices]*sym.Expression{},
		Inputs:        &in,
		StateNext:     [5][5][common.NumSlices]ifaces.Column{},
	}

	var (
		stateNext   [5][5][common.NumSlices]*sym.Expression
		keccakfSize = in.KeccakfSize
	)

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < common.NumSlices; z++ {
				// set the internal state column to the result of the linear combination
				chi.StateInternal[x][y][z] = wizardutils.LinCombExpr(common.BaseChi, in.StateCurr[x][y][z*common.NumSlices:z*common.NumSlices+common.NumSlices])
				// define the state next columns
				chi.StateNext[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("CHI_STATE_NEXT_%v_%v_%v", x, y, z), keccakfSize, true)
			}
		}
	}

	// define the round constant columns
	rcCols := ValRCBase2Pattern()
	for i := 0; i < common.NumSlices; i++ {
		chi.Rc[i] = dedicated.NewRepeatedPattern(
			comp,
			0,
			rcCols[i],
			verifiercol.NewConstantCol(field.One(), keccakfSize, "keccak-rc-pattern"),
			"VAL_RC_BASE2_PATTERN_"+strconv.Itoa(i),
		)
	}

	// apply complex binary. i.e., A[x][y] = A[x][y] + ( (not A[x+1][y]) and A[x+2][y] ) + block[x+5*Y]  and A[0,0] = A[0,0] + RC
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < common.NumSlices; z++ {
				stateNext[x][y][z] = sym.Add(
					sym.Mul(2, chi.StateInternal[x][y][z]),
					chi.StateInternal[(x+1)%5][y][z],
					sym.Mul(3, chi.StateInternal[(x+2)%5][y][z]),
				)
				if x+5*y < common.NumLanesInBlock {
					// add the message block for all blocks except the first one
					stateNext[x][y][z] = sym.Add(
						stateNext[x][y][z],
						sym.Mul(2, in.Blocks[x+5*y][z], in.IsBlockOther),
					)
				}

				// add the round constant for position (0,0)
				if x == 0 && y == 0 {
					stateNext[x][y][z] = sym.Add(
						stateNext[x][y][z],
						sym.Mul(2, chi.Rc[z].Natural),
					)
				}

				comp.InsertGlobal(0, ifaces.QueryIDf("CHI_STATE_NEXT_%v_%v_%v", x, y, z),
					sym.Sub(stateNext[x][y][z], chi.StateNext[x][y][z]),
				)
			}

		}
	}
	return chi
}

// assignChi assigns the values to the columns of chi step.
func (chi *chiIota) assignChi(run *wizard.ProverRuntime, stateCurr common.StateInBits) {
	var (
		u, v          []field.Element
		stateInternal [5][5][common.NumSlices][]field.Element
		size          = stateCurr[0][0][0].Size()
		rcCols        [common.NumSlices][]field.Element
		isBlockOther  = chi.Inputs.IsBlockOther.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	// assign the linear combinations for each lane in the state
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < common.NumSlices; z++ {
				// get the assignment of the internal state
				stateInternal[x][y][z] = column.EvalExprColumn(run, chi.StateInternal[x][y][z].Board()).IntoRegVecSaveAlloc()

			}
		}
	}

	// assign the state after chi step
	two := field.NewElement(2)
	for i := 0; i < common.NumSlices; i++ {
		chi.Rc[i].Assign(run)
		rcCols[i] = chi.Rc[i].Natural.GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < common.NumSlices; z++ {
				// A[x][y] = A[x][y] + ( (not A[x+1][y]) and A[x+2][y])
				u = make([]field.Element, size)
				v = make([]field.Element, size)
				vector.ScalarMul(u, stateInternal[x][y][z], two)
				vector.ScalarMul(v, stateInternal[(x+2)%5][y][z], field.NewElement(3))
				vector.Add(u, u, v, stateInternal[(x+1)%5][y][z])

				// add the round constant for position (0,0)
				if x == 0 && y == 0 {
					var tt = make([]field.Element, size)
					vector.ScalarMul(tt, rcCols[z], two)
					vector.Add(u, u, tt)
				}
				// add the message block for all blocks
				if x+5*y < common.NumLanesInBlock {
					blocks := chi.Inputs.Blocks[x+5*y][z].GetColAssignment(run).IntoRegVecSaveAlloc()
					var tt = make([]field.Element, size)
					vector.ScalarMul(tt, blocks, two)
					vector.MulElementWise(tt, tt, isBlockOther)
					vector.Add(u, u, tt)
				}
				// assign the result to the state next column
				run.AssignColumn(chi.StateNext[x][y][z].GetColID(), smartvectors.NewRegular(u))

			}
		}
	}

}

// ValRCBase2Pattern returns the round constants in base BaseChi (11) and sliced form.
func ValRCBase2Pattern() [common.NumSlices][]field.Element {

	var (
		res = [common.NumSlices][]field.Element{}
	)

	for j := range keccak.RC {
		var out [common.NumSlices]uint64
		for i := 0; i < common.NumSlices; i++ {
			out[i] = (keccak.RC[j] >> (common.NumSlices * i)) & 0xFF // take each byte, LSB first
			a := common.U64ToBaseX(out[i], &common.BaseChiFr)
			res[i] = append(res[i], a)
		}

	}
	return res
}

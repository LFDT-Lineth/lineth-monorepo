package keccakf

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// Wizard submodule responsible for handling the piChiIota step of keccakf. This step
// also encompasses the pi and the iota part.
type piChiIota struct {
	// Complex binary replaced by arithmetics 2a+b+3c+2d
	AIotaBaseB [5][5]ifaces.Column
	// The two following columns are used for the base conversion from B to A.
	// The decomposition of aIotaBaseB in slices of 4 bits
	// it is exported since it build up the hash output.
	AIotaBaseBSliced [5][5][numSlice]ifaces.Column
	// The decompositio of aIotaBaseA in slices of 4 bits
	AIotaBaseASliced [5][5][numSlice]ifaces.Column
}

// Run the chi part of the wizard
func newPiChiIota(
	comp *wizard.CompiledIOP,
	round int,
	maxNumKeccakf int,
	mod Module,
) piChiIota {
	chi := piChiIota{}
	lu := mod.Lookups
	chi.declareColumns(comp, round, maxNumKeccakf)
	chi.csAIota(comp, round, lu, mod)
	chi.csBaseBToBaseA(comp, round, lu)
	return chi
}

// Declares the columns for the chi step
func (c *piChiIota) declareColumns(comp *wizard.CompiledIOP, round, maxNumKeccakF int) {

	colSize := numRows(maxNumKeccakF)

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {

			// declares a_chi_base_B
			c.AIotaBaseB[x][y] = comp.InsertCommit(
				round,
				deriveName("AIOTA_BASE2", x, y),
				colSize,
			)

			// declares the subslices in base B and base A
			for k := 0; k < numSlice; k++ {
				// base B
				c.AIotaBaseBSliced[x][y][k] = comp.InsertCommit(
					round,
					deriveName("AIOTA_BASE2_SLICED", x, y, k),
					colSize,
				)
				// base A
				c.AIotaBaseASliced[x][y][k] = comp.InsertCommit(
					round,
					deriveName("AIOTA_BASE1_SLICED", x, y, k),
					colSize,
				)
			}
		}
	}
}

// it unifies all the steps of Api, AChi and AIota: AIota[i][j] = 2*APi[i][j] +
// APi[i+1][j] +3* APi[i+2][j]+2*RC+2*Block (RC is from AIota, APi is obtained
// from ARho by shifting the selected columns). By extension, the step is also
// responsible for handling the XORIN of the block of the next permutation if
// there is. The incoming block of data is given in base B and in clean form.
// When not needed, (for instance, when we are in the middle of the 24 sponge
// rounds), the block is assumed to be zeroed.
func (c *piChiIota) csAIota(
	comp *wizard.CompiledIOP,
	round int,
	l lookUpTables,
	mod Module,
) {
	aRho := mod.Rho.ARho
	blockBaseB := mod.Blocks

	two := symbolic.NewConstant(2)
	three := symbolic.NewConstant(3)
	d := ifaces.ColumnAsVariable(l.RC.Natural).
		Mul(two).
		Mul(ifaces.ColumnAsVariable(mod.IsActive))

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {

			aIotaPart := ifaces.ColumnAsVariable(c.AIotaBaseB[y][(2*x+3*y)%5])
			aPiPart := ifaces.ColumnAsVariable(aRho[x][y]).Mul(two).
				Add(ifaces.ColumnAsVariable(aRho[(x+1)%5][(y+1)%5])).
				Add(ifaces.ColumnAsVariable(aRho[(x+2)%5][(y+2)%5]).Mul(three))

			// Applies the IOTA part but only on the 0x0 entry
			if x == 0 && y == 0 {
				aPiPart = aPiPart.Add(d)
			}

			// XorIn the next block if the current lane is concerned. m will
			// correspond to the position in the block if it is smaller than
			// LanesInBlock. Since Aiota is shifted by the (x, y) -> (y, 2x+3y)
			// map, it must be accounted for in the constraint.
			xiota, yiota := y, (2*x+3*y)%5
			m := 5*yiota + xiota
			if m < numLanesInBlock { // 17 is the number of keccak lanes in a block
				aPiPart = symbolic.Add(aPiPart, symbolic.Mul(
					symbolic.Mul(blockBaseB[m], mod.IO.IsBlockBaseB), 2))
			}

			comp.InsertGlobal(
				round,
				ifaces.QueryIDf("KECCAKF_API_TO_AIOTA_%v_%v", x, y),
				aIotaPart.Sub(aPiPart),
			)
		}
	}
}

// it backs to (clean) BaseA (original form of state), slice by slice
func (c *piChiIota) csBaseBToBaseA(comp *wizard.CompiledIOP, round int, l lookUpTables) {
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {

			// Enforces that the decomposition in slice of 4 bits of aIota is correctly done.
			expr := ifaces.ColumnAsVariable(c.AIotaBaseB[x][y]).
				Sub(BaseRecomposeSliceHandles(c.AIotaBaseBSliced[x][y][:], BaseB))
			name := ifaces.QueryIDf("AIOTA_DECOMPOSITION_%v_%v", x, y)
			comp.InsertGlobal(round, name, expr)

			// Enforces the correctness of the base conversion slice by slice
			for k := 0; k < numSlice; k++ {
				name := ifaces.QueryIDf("AIOTA_BASE_CONVERSION_%v_%v_%v", x, y, k)
				comp.InsertInclusion(
					round,
					name,
					[]ifaces.Column{l.BaseAClean, l.BaseBDirty},
					[]ifaces.Column{c.AIotaBaseASliced[x][y][k], c.AIotaBaseBSliced[x][y][k]},
				)
			}
		}
	}
}

// Generic function that permutes the array in the same way the keccakf function
// permutes the lanes during the pi step
func pi[T any](a [5][5]T) (b [5][5]T) {
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			b[y][(2*x+3*y)%5] = a[x][y]
		}
	}
	return b
}

// Performs the assignment of the submodule
func (c *piChiIota) assign(
	run *wizard.ProverRuntime,
	numKeccakf int,
	lookups lookUpTables,
	aRho [5][5]ifaces.Column,
	blockBaseB [numLanesInBlock]ifaces.Column,
	isBlockBaseB ifaces.Column,
) {

	lookups.RC.Assign(run)

	// effNumRows is the number of rows that are effectively not padded
	effNumRows := numKeccakf * keccak.NumRound
	colSize := c.AIotaBaseB[0][0].Size()

	// Fetch the the values of aPiOut with the assignment for aRho
	aPiOut := [5][5][]field.Element{}
	rc := lookups.RC.Natural.GetColAssignment(run)
	base1Clean := lookups.BaseAClean.GetColAssignment(run)
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for k := 0; k < numSlice; k++ {
				aPiOut[x][y] = aRho[x][y].GetColAssignment(run). // #nosec G602 -- the slice is an array and not a slice
											SubVector(0, effNumRows).
											IntoRegVecSaveAlloc()
			}
		}
	}

	// Fetch the assignment for the block of data
	blockBaseBVal := [numLanesInBlock][]field.Element{}
	for m := 0; m < numLanesInBlock; m++ {
		blockBaseBVal[m] = blockBaseB[m].GetColAssignment(run).
			SubVector(0, effNumRows).
			IntoRegVecSaveAlloc()
	}

	isBlockBaseBVal := isBlockBaseB.GetColAssignment(run).
		SubVector(0, effNumRows).
		IntoRegVecSaveAlloc()

	// Then permute the columns to apply the effects of the Pi permutation.
	aPiOut = pi(aPiOut)

	// Allocate the columns that are assigned during this prover phase.
	aIotaBaseB := [5][5][]field.Element{}
	aIotaBaseASliced := [5][5][numSlice][]field.Element{}
	aIotaBaseBSliced := [5][5][numSlice][]field.Element{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			aIotaBaseB[x][y] = make([]field.Element, effNumRows)
			for k := 0; k < numSlice; k++ {
				aIotaBaseASliced[x][y][k] = make([]field.Element, effNumRows)
				aIotaBaseBSliced[x][y][k] = make([]field.Element, effNumRows)
			}
		}
	}

	parallel.Execute(effNumRows, func(start, stop int) {

		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {

				// Computes a thread local version of aIota
				aIota := make([]field.Element, stop-start)
				copy(aIota, aPiOut[x][y][start:stop]) // Api

				// Optionally adds rc. (Only once since we are going to double
				// later anyway)
				rc := rc.SubVector(start, stop).IntoRegVecSaveAlloc()
				if x == 0 && y == 0 {
					vector.Add(aIota, aIota, rc) // aPi + Rc
				}

				// Optionally adds the fresh block data. Here the permutation of
				// pi is already taken into account so we don't need to reflect
				// it in the coordinates when xoring in the block data. Again,
				// it will doubled later on, so there is no need to do that just
				// now.
				m := 5*y + x
				if m < numLanesInBlock {
					res := make([]field.Element, stop-start)
					vector.MulElementWise(res, blockBaseBVal[m][start:stop], isBlockBaseBVal[start:stop])
					vector.Add(aIota, aIota, res)
				}

				vector.Add(aIota, aIota, aIota) // 2 (aPi + Rc + block)
				apiPlus1 := aPiOut[(x+1)%5][y][start:stop]
				vector.Add(aIota, aIota, apiPlus1) // 2 (aPi + Rc) + aPi1
				apiPlus2 := aPiOut[(x+2)%5][y][start:stop]
				// 2 (aPi + Rc) + aPi1 + 3 aPi2
				vector.Add(aIota, aIota, apiPlus2, apiPlus2, apiPlus2)

				// Saves the value for aIota
				copy(aIotaBaseB[x][y][start:stop], aIota)

				// Slice aIota
				aIotaB2Slices := DecomposeFrInSlice(aIota, BaseB)
				for k := 0; k < numSlice; k++ {
					// Save the result in base 2
					copy(
						aIotaBaseBSliced[x][y][k][start:stop],
						aIotaB2Slices[k],
					)

					// And also maps the corresponding base 1 value using the
					// lookup table.
					for r := start; r < stop; r++ {
						pos := aIotaBaseBSliced[x][y][k][r].Uint64()
						// Coincidentally, the values of base2Dirty are 0, 1, 3
						// so we can us the bear value to perform the lookup.
						lookedUp := base1Clean.Get(utils.ToInt(pos))
						// Sanity-check : are we getting the same value with the
						// conversion
						{
							expectedLookup := aIotaBaseBSliced[x][y][k][r]
							u := BaseXToU64(expectedLookup, &BaseBFr, 1)
							expectedLookup = U64ToBaseX(u, &BaseAFr)
							if expectedLookup != lookedUp {
								utils.Panic("Unexpected lookup %v != %v", lookedUp.String(), expectedLookup.String())
							}
						}
						aIotaBaseASliced[x][y][k][r] = lookedUp
					}
				}

			}
		}
	})

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			run.AssignColumn(
				c.AIotaBaseB[x][y].GetColID(),
				smartvectors.RightZeroPadded(aIotaBaseB[x][y], colSize),
			)
			for k := 0; k < numSlice; k++ {
				run.AssignColumn(
					c.AIotaBaseBSliced[x][y][k].GetColID(),
					smartvectors.RightZeroPadded(aIotaBaseBSliced[x][y][k], colSize),
				)
				run.AssignColumn(
					c.AIotaBaseASliced[x][y][k].GetColID(),
					smartvectors.RightZeroPadded(aIotaBaseASliced[x][y][k], colSize),
				)
			}
		}
	}

}

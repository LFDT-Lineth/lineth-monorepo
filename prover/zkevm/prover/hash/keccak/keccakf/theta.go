package keccakfkoalabear

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

const (
	thetaBase = 4 // lane is decomposed into 8 slices in base 4.
	// base 4 allows three additions without overflow.
	// it also allows acceptable column size for lookup arguments.
	thetaBase8 = 65536 // thetaBase^8
)

// theta module, responsible for updating the state in the theta step of keccakf
type theta struct {
	// state before applying the theta step, in base clean 4.
	StateCurr kcommon.State
	// state after applying the theta step, in base clean 4.
	StateInternalDirty, StateInternalClean kcommon.State
	// state after applying the theta step, in bits.
	StateNext kcommon.StateInBits
	// Intermediate columns, after each 3 additions
	CMiddleDirty, CFinalDirty, CMiddleClean, CFinalClean, CcDirty, CcCleaned [5][8]ifaces.Column
	// Msb of cFinal used for the rotation and computing cc.
	Msb [5][8]ifaces.Column
	// lookup tables to attest the correctness of base conversion from dirty to clean base. The first column is in dirty base and the second in clean base.
	LookupTable [2]ifaces.Column
}

// newTheta creates a new theta module, declares the columns and constraints and returns its pointer
func newTheta(comp *wizard.CompiledIOP,
	keccakfSize int,
	stateCurr state) *theta {
	theta := &theta{}
	theta.StateCurr = stateCurr

	// declare the new state and intermediate columns
	theta.declareColumnsTheta(comp, keccakfSize)

	// declare the constraints
	theta.computationStepConstraints(comp)
	theta.lookupConstraints(comp)

	return theta
}

// declareColumnsTheta declares the intermediate columns generated during theta step, including the new state.
func (theta *theta) declareColumnsTheta(comp *wizard.CompiledIOP, keccakfSize int) {
	// declare the new state
	theta.StateInternalClean = kcommon.State{}
	theta.StateNext = kcommon.StateInBits{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				theta.StateInternalDirty[x][y][z] = comp.InsertCommit(
					0,
					ifaces.ColIDf("STATE_THETA_DIRTY_%v_%v_%v", x, y, z),
					keccakfSize,
					true,
				)
				theta.StateInternalClean[x][y][z] = comp.InsertCommit(
					0,
					ifaces.ColIDf("STATE_THETA_CLEAN_%v_%v_%v", x, y, z),
					keccakfSize,
					true,
				)

				// declare the new state in bits
				for i := 0; i < 8; i++ {
					theta.StateNext[x][y][z*8+i] = comp.InsertCommit(
						0,
						ifaces.ColIDf("STATE_THETA_BIT_%v_%v_%v", x, y, z*8+i),
						keccakfSize,
						true,
					)
				}
			}
		}
	}

	// declare the lookup table columns
	dirtyBaseTheta, cleanBaseTheta := createLookupTablesTheta()
	theta.LookupTable[0] = comp.InsertPrecomputed(ifaces.ColID("BC_LOOKUP_DIRTY_BASETHETA"), dirtyBaseTheta)
	theta.LookupTable[1] = comp.InsertPrecomputed(ifaces.ColID("BC_LOOKUP_CLEAN_BASETHETA"), cleanBaseTheta)

	// declare cm ,cf, cmCleaned, cfCleaned, cc, ccCleaned, and msb columns
	for x := 0; x < 5; x++ {
		for z := 0; z < 8; z++ {
			theta.CMiddleDirty[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("C_MIDDLE_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.CFinalDirty[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("C_FINAL_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.CMiddleClean[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("C_MIDDLE_CLEAN_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.CFinalClean[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("C_FINAL_CLEAN_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.CcDirty[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("CC_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.CcCleaned[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("CC_CLEAN_%v_%v", x, z),
				keccakfSize,
				true,
			)
			theta.Msb[x][z] = comp.InsertCommit(
				0,
				ifaces.ColIDf("THETA_MSB_%v_%v", x, z),
				keccakfSize,
				true,
			)
		}
	}
}

// computationStepConstraints declares the constraints for the computation steps of theta
// (step by step)
func (theta *theta) computationStepConstraints(comp *wizard.CompiledIOP) {
	for x := 0; x < 5; x++ {
		for z := 0; z < 8; z++ {
			// cmDirty[x][z] =  A[x][0][z] + A[x][1][z] + A[x][2][z]
			exprCm := sym.Sub(theta.CMiddleDirty[x][z],
				theta.StateCurr[x][0][z],
				theta.StateCurr[x][1][z],
				theta.StateCurr[x][2][z],
			)
			comp.InsertGlobal(0, ifaces.QueryIDf("CM_DIRTY_THETA_%v_%v", x, z),
				exprCm,
			)
			// cfDirty[x][z] = cmClean[x][z] + A[x][3][z] + A[x][4][z]
			exprCf := sym.Sub(theta.CFinalDirty[x][z],
				theta.CMiddleClean[x][z],
				theta.StateCurr[x][3][z],
				theta.StateCurr[x][4][z],
			)
			comp.InsertGlobal(0, ifaces.QueryIDf("CF_DIRTY_THETA_%v_%v", x, z),
				exprCf,
			)

			// booleanity of msb[x][z]
			exprMsbBool := sym.Mul(theta.Msb[x][z],
				sym.Sub(field.One(), theta.Msb[x][z]),
			)
			comp.InsertGlobal(0, ifaces.QueryIDf("MSB_BOOL_THETA_%v_%v", x, z),
				exprMsbBool,
			)

			// ccDirty[x][z] = cfClean[x][z] * thetaBase - msb[x][z] * thetaBase^8 + msb of previous slice
			exprCc := sym.Sub(theta.CcDirty[x][z],
				sym.Mul(theta.CFinalClean[x][z], thetaBase),
				sym.Mul(theta.Msb[x][z], -1*thetaBase8),
				theta.Msb[x][(z-1+8)%8],
			)
			comp.InsertGlobal(0, ifaces.QueryIDf("CC_DIRTY_THETA_%v_%v", x, z),
				exprCc,
			)
		}
	}
	// constraint on the stateCurr, stateInternalDirty, cfClean, ccCleaned
	// stateInternalDirty[x][y][z] = stateCurr[x][y][z] + cFinalClean[(x-1)%5][z] + ccCleaned[(x+1)%5][z]
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				eqTheta := sym.Sub(theta.StateInternalDirty[x][y][z],
					theta.StateCurr[x][y][z],
					theta.CFinalClean[(x-1+5)%5][z],
					theta.CcCleaned[(x+1)%5][z],
				)
				qName := ifaces.QueryIDf("EQ_THETA_STATE_%v_%v_%v", x, y, z)
				comp.InsertGlobal(0, qName, eqTheta)
			}
		}
	}
}

// lookupConstraints use inclusion query to validate the dirty and clean versions
func (theta *theta) lookupConstraints(comp *wizard.CompiledIOP) {
	for x := 0; x < 5; x++ {
		for z := 0; z < 8; z++ {
			// lookup: (cMiddleDirty, cMiddleClean)
			comp.InsertInclusion(0,
				ifaces.QueryIDf("LOOKUP_THETA_C_MIDDLE_%v_%v", x, z),
				theta.LookupTable[:],
				[]ifaces.Column{
					theta.CMiddleDirty[x][z],
					theta.CMiddleClean[x][z],
				},
			)
			// lookup: (cFinalDirty, cFinalClean)
			comp.InsertInclusion(0,
				ifaces.QueryIDf("LOOKUP_THETA_C_FINAL_%v_%v", x, z),
				theta.LookupTable[:],
				[]ifaces.Column{
					theta.CFinalDirty[x][z],
					theta.CFinalClean[x][z],
				},
			)
			// lookup: (ccDirty, ccCleaned)
			comp.InsertInclusion(0,
				ifaces.QueryIDf("LOOKUP_THETA_CC_%v_%v", x, z),
				theta.LookupTable[:],
				[]ifaces.Column{
					theta.CcDirty[x][z],
					theta.CcCleaned[x][z],
				},
			)
		}
	}
	// lookup: (stateInternalDirty, stateInternalClean)
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				comp.InsertInclusion(0,
					ifaces.QueryIDf("LOOKUP_THETA_STATE_%v_%v_%v", x, y, z),
					theta.LookupTable[:],
					[]ifaces.Column{
						theta.StateInternalDirty[x][y][z],
						theta.StateInternalClean[x][y][z],
					},
				)
			}
		}
	}
}

// assignTheta assigns the values to the columns of theta step.
func (theta *theta) assignTheta(run *wizard.ProverRuntime, stateCurr state) {

	var (
		stateCurrWit                                                   [5][5][8][]field.Element
		cmDirtyFr, cmCleanFr, cfDirtyFr, cfCleanFr, ccCleanedFr, msbFr [5][8][]field.Element
		cmDirty, cfDirty, cmClean, cfClean, ccDirty, ccCleaned, msb    [5][8]*common.VectorBuilder
		col                                                            []field.Element
		stateInternalClean                                             [5][5][8]*common.VectorBuilder
		stateInternalDirty                                             [5][5][8]*common.VectorBuilder
		stateBinary                                                    [5][5][64]*common.VectorBuilder
	)
	// get the current state
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				stateCurrWit[x][y][z] = stateCurr[x][y][z].GetColAssignment(run).IntoRegVecSaveAlloc()
			}
		}
	}
	size := len(stateCurrWit[0][0][0])
	// compute c and cc, cleanBaseC, msb
	for x := 0; x < 5; x++ {
		for z := 0; z < 8; z++ {
			cmDirty[x][z] = common.NewVectorBuilder(theta.CMiddleDirty[x][z])
			cfDirty[x][z] = common.NewVectorBuilder(theta.CFinalDirty[x][z])
			cmClean[x][z] = common.NewVectorBuilder(theta.CMiddleClean[x][z])
			cfClean[x][z] = common.NewVectorBuilder(theta.CFinalClean[x][z])
			ccDirty[x][z] = common.NewVectorBuilder(theta.CcDirty[x][z])
			ccCleaned[x][z] = common.NewVectorBuilder(theta.CcCleaned[x][z])
			msb[x][z] = common.NewVectorBuilder(theta.Msb[x][z])
			cmDirtyFr[x][z] = make([]field.Element, size)
			cmCleanFr[x][z] = make([]field.Element, size)
			cfDirtyFr[x][z] = make([]field.Element, size)
			cfCleanFr[x][z] = make([]field.Element, size)
			ccCleanedFr[x][z] = make([]field.Element, size)
			msbFr[x][z] = make([]field.Element, size)

			// cmDirty[x][z] = A[x][0][z] + A[x][1][z] + A[x][2][z]
			vector.Add(cmDirtyFr[x][z],
				stateCurrWit[x][0][z],
				stateCurrWit[x][1][z],
				stateCurrWit[x][2][z],
			)
			for i := 0; i < size; i++ {
				cmDirty[x][z].PushField(cmDirtyFr[x][z][i])
			}

			// clean the cm by decomposing each element into base thetaBase and recomposing
			for i := 0; i < len(cmDirtyFr[x][z]); i++ {
				v := cmDirtyFr[x][z][i].Uint64()
				resc := clean(kcommon.Decompose(v, thetaBase, 8, true))

				cleaned := 0
				for k := len(resc) - 1; k >= 0; k-- {
					cleaned = cleaned*thetaBase + resc[k]
				}
				cmCleanFr[x][z][i] = field.NewElement(uint64(cleaned))
				cmClean[x][z].PushInt(cleaned)

			}
			// Add the rest of the state to get the cFinal
			vector.Add(cfDirtyFr[x][z],
				cmCleanFr[x][z],
				stateCurrWit[x][3][z],
				stateCurrWit[x][4][z],
			)
			for i := 0; i < size; i++ {
				cfDirty[x][z].PushField(cfDirtyFr[x][z][i])
			}
		}
		// First pass: compute msb of the next slice for all z
		for z := 0; z < 8; z++ {
			for i := 0; i < len(cfDirtyFr[x][z]); i++ {
				v := cfDirtyFr[x][z][i].Uint64()
				resf := clean(kcommon.Decompose(v, thetaBase, 8, true))

				msbFr[x][z][i] = field.NewElement(uint64(resf[len(resf)-1]))

				cfCleaned := 0
				for k := len(resf) - 1; k >= 0; k-- {
					cfCleaned = cfCleaned*thetaBase + resf[k]
				}
				cfCleanFr[x][z][i] = field.NewElement(uint64(cfCleaned))
				cfClean[x][z].PushInt(cfCleaned)
				msb[x][z].PushField(msbFr[x][z][i])
			}
		}

		// Second pass: compute cc using previous slice lsb (wrap-around)
		for z := 0; z < 8; z++ {
			prev := (z - 1 + 8) % 8 // safe wrap-around
			for i := 0; i < len(cfCleanFr[x][z]); i++ {
				// cc is the 1 bit left shifted version of cf
				// to obtain cc we do:
				// - left shift cf by 1 position (multiply by thetaBase)
				// - subtract msb * thetaBase^8 (removing the overflowed bit)
				// - add msb of previous slice (adding the bit shifted from previous slice)
				// it is the previous slice because z slices are stored in little endian order
				// cc[x][z] = cf[x][z] * thetaBase - msb[x][z] * thetaBase^8 + msb of previous slice
				a := int(cfCleanFr[x][z][i].Uint64())*thetaBase - int(msbFr[x][z][i].Uint64())*thetaBase8 + int(msbFr[x][prev][i].Uint64())
				ccDirty[x][z].PushInt(a)
				// clean a
				res := clean(kcommon.Decompose(uint64(a), thetaBase, 8, true))
				cleanedA := 0
				for k := len(res) - 1; k >= 0; k-- {
					cleanedA = cleanedA*thetaBase + res[k]
				}
				ccCleanedFr[x][z][i] = field.NewElement(uint64(cleanedA))
				ccCleaned[x][z].PushInt(cleanedA)
			}
			// Assign all the vector builders
			cmDirty[x][z].PadAndAssign(run)
			cfDirty[x][z].PadAndAssign(run)
			cmClean[x][z].PadAndAssign(run)
			cfClean[x][z].PadAndAssign(run)
			ccDirty[x][z].PadAndAssign(run)
			ccCleaned[x][z].PadAndAssign(run)
			msb[x][z].PadAndAssign(run)
		}

	}

	// assign internal and final state.
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 8; z++ {
				stateInternalClean[x][y][z] = common.NewVectorBuilder(theta.StateInternalClean[x][y][z])
				stateInternalDirty[x][y][z] = common.NewVectorBuilder(theta.StateInternalDirty[x][y][z])
				// assign the binary state
				for j := 0; j < 8; j++ {
					stateBinary[x][y][z*8+j] = common.NewVectorBuilder(theta.StateNext[x][y][z*8+j])
				}

				col = make([]field.Element, size)

				// A'[x][y][z] = A[x][y][z] + c[(x-1+5)%5][z] + cc[(x+1)%5][z]
				vector.Add(col, stateCurrWit[x][y][z], cfCleanFr[(x-1+5)%5][z], ccCleanedFr[(x+1)%5][z])

				// assign dirty and clean the state
				for i := 0; i < len(col); i++ {
					stateInternalDirty[x][y][z].PushField(col[i])
					res := clean(kcommon.Decompose(col[i].Uint64(), thetaBase, 8, true))
					// recompose to get clean state
					stateCleaned := 0
					for i := len(res) - 1; i >= 0; i-- {
						stateCleaned = stateCleaned*thetaBase + res[i]
					}
					stateInternalClean[x][y][z].PushInt(stateCleaned)
					// assign the binary representation
					for j := 0; j < 8; j++ {
						stateBinary[x][y][z*8+j].PushInt(res[j])
					}
				}
				// assign the internal state
				stateInternalDirty[x][y][z].PadAndAssign(run)
				stateInternalClean[x][y][z].PadAndAssign(run)
				// assign the binary state
				for i := 0; i < 8; i++ {
					stateBinary[x][y][z*8+i].PadAndAssign(run)
				}
			}
		}
	}

}

// clean converts a slice of uint64 values into a new slice of ints of the
// same length where each element is 1 if the corresponding input value is
// odd and 0 if it is even. The input slice is not modified.
func clean(in []uint64) (out []int) {
	out = make([]int, 0, len(in))
	for _, element := range in {
		if element%2 == 0 {
			out = append(out, 0)
		} else {
			out = append(out, 1)
		}
	}
	return out
}

// createLookupTablesTheta creates the lookup tables for validating th
// cleaning from dirty to clean version for theta step.
func createLookupTablesTheta() (dirtyBaseTheta, cleanBaseTheta smartvectors.SmartVector) {
	var (
		lookupDirtyBaseTheta []field.Element
		lookupCleanBaseTheta []field.Element
		cleanValueTheta      int
		targetSize           = utils.NextPowerOfTwo(thetaBase8)
	)

	// for each value in base dirty BaseTheta (0 to 65535), compute its equivalent in base clean BaseTheta
	for i := 0; i < thetaBase8; i++ {
		// decompose in base thetaBase (8 digits) and clean it
		v := clean(kcommon.Decompose(uint64(i), thetaBase, 8, true))
		cleanValueTheta = 0
		for k := len(v) - 1; k >= 0; k-- {
			cleanValueTheta = cleanValueTheta*thetaBase + int(v[k])
		}

		lookupDirtyBaseTheta = append(lookupDirtyBaseTheta, field.NewElement(uint64(i)))
		lookupCleanBaseTheta = append(lookupCleanBaseTheta, field.NewElement(uint64(cleanValueTheta)))
	}

	// pad to target size
	dirtyBaseTheta = smartvectors.RightPadded(lookupDirtyBaseTheta, field.NewElement(thetaBase8-1), targetSize)
	cleanBaseTheta = smartvectors.RightPadded(lookupCleanBaseTheta, field.NewElement(uint64(cleanValueTheta)), targetSize)
	return dirtyBaseTheta, cleanBaseTheta
}

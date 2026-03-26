package keccakf

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// SEGMENT size is how many elements we compute in parallel during the proving
// phase for an atomic job. (e.g. what's the minimal amount of rows that can be
// computed in parallel. Too little and the parallelization overhead is larger,
// too large and we wait longer for the slower threads. A good rule of thumb is
// that an atomic job should no less than 1 ms.
const ATOMIC_PROVER_JOB_SIZE = 1 << 10

// Theta defines the theta part of the keccakf permutation in the wizard
type theta struct {
	// Keccakf state after theta step, the results holds over 65 bits. Namely,
	// the first and the last bits of AThetaBaseA are incompletely computed. In
	// order to get the proper result we should cancel it
	AThetaBaseA    [5][5]ifaces.Column
	AThetaBaseAMsb [5][5]ifaces.Column
	// slices of ATheta in (Dirty) BaseA, each slice holds 4 bits
	AThetaSlicedBaseA [5][5][numSlice]ifaces.Column
	// slices of ATheta in (clean) BaseB, each slice holds 4 bits
	AThetaSlicedBaseB [5][5][numSlice]ifaces.Column
}

// Constructs a new theta object and registers the colums into the context
func newTheta(
	comp *wizard.CompiledIOP,
	round, numKeccakf int,
	a [5][5]ifaces.Column,
	l lookUpTables,
) theta {

	// Initialize the context
	res := theta{}

	// Declare the columns
	res.declareColumn(comp, round, numKeccakf)

	// Declare the constraints
	res.csEqThetaBaseA(comp, round, a)
	res.csAThetaDecomposition(comp, round)
	res.csAThetaFromBaseAToBaseB(comp, round, l)

	return res
}

// declare the columns in the Wizard. It only registers the columns (i.e. no
// constraints are registered)
func (t *theta) declareColumn(comp *wizard.CompiledIOP, round, numKeccakf int) {
	// size of the columns to declare
	colSize := numRows(numKeccakf)
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			//
			t.AThetaBaseA[x][y] = comp.InsertCommit(
				round, deriveName("A_THETA_BASE1", x, y), colSize,
			)
			//
			t.AThetaBaseAMsb[x][y] = comp.InsertCommit(
				round, deriveName("A_THETA_BASE1_MBS", x, y), colSize,
			)
			//
			for k := 0; k < numSlice; k++ {
				//
				t.AThetaSlicedBaseA[x][y][k] = comp.InsertCommit(
					round, deriveName("A_THETA_BASE1_SLICED", x, y, k), colSize,
				)
				//
				t.AThetaSlicedBaseB[x][y][k] = comp.InsertCommit(
					round, deriveName("A_THETA_BASE2_SLICED", x, y, k), colSize,
				)
			}
		}
	}
}

// Declare the constraints to justify the construction of eqThetaBaseA
func (t *theta) csEqThetaBaseA(
	comp *wizard.CompiledIOP,
	round int,
	a [5][5]ifaces.Column,
) {
	// cc is the bitshifted version of c. Unlike what is specified by in the
	// spec of keccak, the shifting here is not cyclic. Thus, cc uses 65 bits.
	var c, cc [5]*symbolic.Expression
	for x := 0; x < 5; x++ {
		c[x] = ifaces.ColumnAsVariable(a[x][0]).
			Add(ifaces.ColumnAsVariable(a[x][1])).
			Add(ifaces.ColumnAsVariable(a[x][2])).
			Add(ifaces.ColumnAsVariable(a[x][3])).
			Add(ifaces.ColumnAsVariable(a[x][4]))
		cc[x] = c[x].Mul(symbolic.NewConstant(BaseA))
	}

	// Since cc is not actually a cyclic rotation, the result for eqTheta still
	// requires adding the MSbit to the LSbit to derive the actual result.
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			eqTheta := ifaces.ColumnAsVariable(t.AThetaBaseA[x][y]).
				Sub(ifaces.ColumnAsVariable(a[x][y]).
					Add(c[(x-1+5)%5]).
					Add(cc[(x+1)%5]))
			qName := ifaces.QueryIDf("EQ_THETA_%v_%v", x, y)
			comp.InsertGlobal(round, qName, eqTheta)
		}
	}
}

// Proves the link between (aThetaBaseA, aThetaBaseAMsb) with the sliced
// decomposition of aThetaBaseA.
func (t *theta) csAThetaDecomposition(comp *wizard.CompiledIOP, round int) {

	// shf64 = BaseA^U64, it is used to left-shift the MSB to lay on the 64 bits
	// and cancel the MSB of aThetaBaseA
	var shf64 big.Int
	shf64.Exp(big.NewInt(BaseA), big.NewInt(64), nil)

	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			// aThetaFirst is 65 bits, first come back to 64 bits using the
			// committed msb
			aTheta64BaseA := ifaces.ColumnAsVariable(t.AThetaBaseA[i][j]).
				Sub(ifaces.ColumnAsVariable(t.AThetaBaseAMsb[i][j]).
					Mul(symbolic.NewConstant(shf64))).
				Add(ifaces.ColumnAsVariable(t.AThetaBaseAMsb[i][j]))

			// On the other hand, recompose the sliced version of aTheta the
			// result should equals the values of aThetaFirstU64. Note that the
			// fact that the decomposition holds over 16 * 4 bits forces the
			// correctness of the MSB.
			aThetaRecomposedBaseA := BaseRecomposeSliceHandles(
				t.AThetaSlicedBaseA[i][j][:],
				BaseA,
			)

			expr := aTheta64BaseA.Sub(aThetaRecomposedBaseA)
			name := ifaces.QueryIDf(
				"ATHETA_DECOMPOSITION_%v_%v", i, j,
			)
			comp.InsertGlobal(round, name, expr)
		}
	}
}

// Move from AThetaFirst to AThetaSecond (slice by slice)
func (t *theta) csAThetaFromBaseAToBaseB(
	comp *wizard.CompiledIOP,
	round int,
	l lookUpTables,
) {

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for s := 0; s < numSlice; s++ {
				comp.InsertInclusion(
					round,
					ifaces.QueryIDf("A_THETA_BASE1_TO_BASE2_%v_%v_%v", x, y, s),
					[]ifaces.Column{
						l.BaseADirty,
						l.BaseBClean,
					},
					[]ifaces.Column{
						t.AThetaSlicedBaseA[x][y][s],
						t.AThetaSlicedBaseB[x][y][s],
					},
				)
			}
		}
	}
}

// Assigns the columns specified by theta. a is the column used to denotes the
// inputs of the keccakf round function.
func (t *theta) assign(
	run *wizard.ProverRuntime,
	a [5][5]ifaces.Column,
	lookups lookUpTables,
	numKeccakf int,
) {

	// effNumRows is the number of rows that are effectively not padded
	effNumRows := numKeccakf * keccak.NumRound
	// Collect the witness for a.
	aWit := [5][5]smartvectors.SmartVector{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			aWit[x][y] = run.GetColumn(a[x][y].GetColID())
		}
	}

	baseAF := field.NewElement(BaseA)

	// Assumedly, all columns of aWits have the same size. However, it
	// differs from effNumRows in the sense that it includes the padding and
	// the unused rows for keccak while effNumRows does not.
	colSize := aWit[0][0].Len()

	// Before running the jobs, preallocate all the slices that we are going
	// to compute during the theta phase. This does not include the
	// intermediate values as they are locally allocated in chunks in the
	// threads.
	var aThetaBaseA, aThetaBaseAMsb [5][5][]field.Element
	var aThetaBaseASliced, aThetaBaseBSliced [5][5][numSlice][]field.Element
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			aThetaBaseA[x][y] = make([]field.Element, effNumRows)
			aThetaBaseAMsb[x][y] = make([]field.Element, effNumRows)
			for k := 0; k < numSlice; k++ {
				aThetaBaseASliced[x][y][k] = make([]field.Element, effNumRows)
				aThetaBaseBSliced[x][y][k] = make([]field.Element, effNumRows)
			}
		}
	}

	// Also extract the unpadded part of the lookup table. We will use it
	// to compute aThetaBaseBSliced from aThetaBaseASliced. The assumption
	// here is that the cost of looking up is lower than the cost of
	// explicitly computing it.
	baseBTableId := lookups.BaseBClean.GetColID()
	baseBTable := run.GetColumn(baseBTableId).
		SubVector(0, int(BaseAPow4)).
		IntoRegVecSaveAlloc()

	parallel.Execute(effNumRows, func(start, stop int) {

		// Local values holding C and the shifted version of C.
		var cloc, ccloc [5][]field.Element

		// Computes c and cc
		for x := 0; x < 5; x++ {
			// Thread local pointer to c and cc
			cloc[x] = make([]field.Element, stop-start)
			ccloc[x] = make([]field.Element, stop-start)

			vector.Add(
				cloc[x],
				aWit[x][0].SubVector(start, stop).IntoRegVecSaveAlloc(),
				aWit[x][1].SubVector(start, stop).IntoRegVecSaveAlloc(),
				aWit[x][2].SubVector(start, stop).IntoRegVecSaveAlloc(),
				aWit[x][3].SubVector(start, stop).IntoRegVecSaveAlloc(),
				aWit[x][4].SubVector(start, stop).IntoRegVecSaveAlloc(),
			)

			vector.ScalarMul(ccloc[x], cloc[x], baseAF)
		}

		// Then computes the local slices for aTheta in base A and then break
		// it down into slices and perform the conversion from base A to base B.
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {

				// Reminder, aThetaBaseA is on 65 bits
				vector.Add(
					aThetaBaseA[x][y][start:stop],
					aWit[x][y].SubVector(start, stop).IntoRegVecSaveAlloc(),
					cloc[(x-1+5)%5],
					ccloc[(x+1)%5],
				)

				// The 17th slice contains the MSB. The first one needs to
				// completed before we assign it to the witness.
				slices := DecomposeFrInSlice(
					aThetaBaseA[x][y][start:stop],
					BaseA,
				)

				// Slices aTheta in base A, set it back to 64 bits along
				// the way.
				for k := 0; k < numSlice+1; k++ {
					switch {
					// Complete the first limb with the MSB.
					case k == 0:
						vector.Add(
							aThetaBaseASliced[x][y][k][start:stop],
							slices[k],
							slices[numSlice],
						)

						// Also lookup the corresponding value for the
						// corresponding base B value. Coincidentally the
						// position to lookup corresponds to the position
						// in the table directly.
						for r := start; r < stop; r++ {
							pos := aThetaBaseASliced[x][y][k][r].Uint64()
							aThetaBaseBSliced[x][y][k][r] = baseBTable[pos]
						}
					// Copy the slice as is, in the result
					case k > 0 && k < numSlice:
						copy(aThetaBaseASliced[x][y][k][start:stop], slices[k])

						// Also lookup the corresponding value for the
						// corresponding base B value.
						for r := start; r < stop; r++ {
							pos := aThetaBaseASliced[x][y][k][r].Uint64()
							aThetaBaseBSliced[x][y][k][r] = baseBTable[pos]
						}
					// It's the MSB
					case k == numSlice:
						copy(aThetaBaseAMsb[x][y][start:stop], slices[k])
					}
				}

			}
		}
	})

	// Now perform the assignment of aTheta as a smart vector and we trim
	// the full vector later on.
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			// aThetaBaseA
			run.AssignColumn(
				t.AThetaBaseA[x][y].GetColID(),
				smartvectors.RightZeroPadded(aThetaBaseA[x][y], colSize),
			)
			// aThetaBaseAMsb
			run.AssignColumn(
				t.AThetaBaseAMsb[x][y].GetColID(),
				smartvectors.RightZeroPadded(aThetaBaseAMsb[x][y], colSize),
			)

			for k := 0; k < numSlice; k++ {

				// aThetaBaseASliced
				topad := aThetaBaseASliced[x][y][k]
				run.AssignColumn(
					t.AThetaSlicedBaseA[x][y][k].GetColID(),
					smartvectors.RightZeroPadded(topad, colSize),
				)

				// aThetaBaseBSliced
				topad = aThetaBaseBSliced[x][y][k]
				run.AssignColumn(
					t.AThetaSlicedBaseB[x][y][k].GetColID(),
					smartvectors.RightZeroPadded(topad, colSize),
				)
			}
		}
	}
}

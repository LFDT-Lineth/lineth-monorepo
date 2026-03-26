package keccakf

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// rho is a wizard gadget responsible for the rho and pi steps of the keccakf
// permutation.
type rho struct {
	// State after the rotation by LR in base B
	ARho [5][5]ifaces.Column
	// The  decomposition of slice cut by the rotation LR
	TargetSliceDecompose [5][5][numChunkBaseX]ifaces.Column
}

// Constructs a rho permutation object and take care of declaring all the
// columns and the constraints.
func newRho(
	comp *wizard.CompiledIOP,
	round int,
	numKeccakf int,
	aThetaSlicedBaseB [5][5][numSlice]ifaces.Column,
) rho {

	res := rho{}

	res.declareColumns(comp, round, numKeccakf)
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			res.checkConvertAndRotateLr(comp, round, aThetaSlicedBaseB, x, y)
			res.checkTargetSliceDecompose(comp, round, aThetaSlicedBaseB, x, y)
		}
	}
	return res
}

// Declares the columns for the rhopi module
func (r *rho) declareColumns(comp *wizard.CompiledIOP, round, numKeccakf int) {
	colSize := numRows(numKeccakf)
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			r.ARho[x][y] = comp.InsertCommit(
				round,
				deriveName("A_RHO", x, y),
				colSize,
			)

			// Commit to target slice decompose only if the LR[x, y] is not a
			// multiple of 4
			if keccak.LR[x][y]%4 == 0 {
				continue
			}

			for s := 0; s < numChunkBaseX; s++ {
				r.TargetSliceDecompose[x][y][s] = comp.InsertCommit(
					round,
					deriveName("TARGET_SLICE_DECOMPOSE", x, y, s),
					colSize,
				)
			}
		}
	}
}

// Declares the rotation in the case where LR divides 4. In this case, the
// slices do not need to be cut and we can just get away with a linear
// combination of the rotated slices.
func (r *rho) checkConvertAndRotateLr(
	comp *wizard.CompiledIOP, round int,
	aThetaSlicedBaseB [5][5][numSlice]ifaces.Column,
	x, y int,
) {

	lr := keccak.LR[x][y]

	// Convert the slice as an expression
	aThetaSlicedBaseBExprs := make([]*symbolic.Expression, numSlice)
	for i := range aThetaSlicedBaseB[x][y] {
		aThetaSlicedBaseBExprs[i] = ifaces.ColumnAsVariable(
			aThetaSlicedBaseB[x][y][i],
		)
	}

	// Rotate the slices, this amounts to perform a rotation by a multiple of 4.
	rotateBy := lr / 4
	rotated := RotateRight(aThetaSlicedBaseBExprs[:], rotateBy)

	// Perform the last bit of the decomposition. This part is not needed if
	// LR[x][y] is divisible by 4 because in that case, the above slice rotation
	// was sufficient. The process is to linearly shift every limb by
	// multiplying them by (BASE2 ^ (LR % 4)). Thereafter, we need to surgically
	// fix the first and the last limbs of the decompositions.
	if lr%4 > 0 {

		// The offset shifting
		offset := symbolic.NewConstant(IntExp(BaseB, lr%4))
		for i := range rotated {
			rotated[i] = rotated[i].Mul(offset)
		}

		// Now, we need TargetDecomposition to be applied to perform the fix.
		// Each entry in target decompose corresponds to one bit that is
		// overflowing. We use them to reconstruct the whole overflow in base B.
		overflow := BaseRecomposeHandles(
			r.TargetSliceDecompose[x][y][4-(lr%4):],
			BaseB,
		)

		// Shave the overflow out of the last rotated slice
		baseBPow4 := symbolic.NewConstant(IntExp(BaseB, 4)) // BaseB ^ 4
		rotated[numSlice-1] = rotated[numSlice-1].Sub(
			overflow.Mul(baseBPow4),
		)

		// Add it back into the empty first bits of the first slice
		rotated[0] = rotated[0].Add(overflow)
	}

	// And enforces the consistency of the rotated version with a_rho
	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("KECCAKF_CONVERT_ROTATE_LAZY_%v_%v", x, y),
		BaseRecomposeSliceExpr(rotated, BaseB).
			Sub(ifaces.ColumnAsVariable(r.ARho[x][y])),
	)
}

// check the target slice is correctly decomposed to bits
func (r *rho) checkTargetSliceDecompose(
	comp *wizard.CompiledIOP,
	round int,
	aThetaSlicedBaseB [5][5][numSlice]ifaces.Column,
	x, y int,
) {

	// Skip the target if lr is a multiple of 4
	if keccak.LR[x][y]%4 == 0 {
		return
	}

	// Enforces the booleanity of each entry
	for bitPos := 0; bitPos < numChunkBaseX; bitPos++ {
		curr := ifaces.ColumnAsVariable(r.TargetSliceDecompose[x][y][bitPos])
		comp.InsertGlobal(
			round,
			ifaces.QueryIDf(
				"KECCAKF_TARGET_SLICE_DECOMPOSE_%v_%v_%v",
				x, y, bitPos,
			),
			curr.Mul(curr).Sub(curr),
		)
	}

	// Enforces that when composed, these bits gives the relevant slice of
	// aTheta.
	targetPos := (64 - keccak.LR[x][y]) / numChunkBaseX
	relevantSlice := ifaces.ColumnAsVariable(aThetaSlicedBaseB[x][y][targetPos])
	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("KECCAKF_TARGET_SLICE_DECOMPOSITION_%v_%v", x, y),
		BaseRecomposeHandles(r.TargetSliceDecompose[x][y][:], BaseB).
			Sub(relevantSlice),
	)

}

// Assigns the rho submodule given a column representing aTheta in base B in a
// sliced way.
func (r *rho) assign(
	run *wizard.ProverRuntime,
	aThetaBaseBSliced [5][5][16]ifaces.Column,
	numKeccakf int,
) {

	// effNumRows is the number of rows that are effectively not padded
	effNumRows := numKeccakf * keccak.NumRound

	aThetaOut := [5][5][16]smartvectors.SmartVector{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for k := 0; k < numSlice; k++ {
				aThetaOut[x][y][k] = run.GetColumn(
					aThetaBaseBSliced[x][y][k].GetColID(),
				)
			}
		}
	}

	// Preallocate the slices containing the results
	aRho := [5][5][]field.Element{}
	// Target slice decomposition
	targSliceDec := [5][5][numChunkBaseX][]field.Element{}

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			aRho[x][y] = make([]field.Element, effNumRows)

			if keccak.LR[x][y]%numChunkBaseX == 0 {
				// Skip the allocation of the target slice
				continue
			}

			for b := 0; b < numChunkBaseX; b++ {
				targSliceDec[x][y][b] = make([]field.Element, effNumRows)
			}
		}
	}

	parallel.Execute(effNumRows, func(start, stop int) {

		slice := [numSlice]field.Element{}

		// Multiplying a field in base B by shfBy64 is equivalent to bitshift by
		// 64 to the left.
		var shfBy64 field.Element
		shfBy64.Exp(BaseBFr, big.NewInt(64))

		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {

				// Position of the target slice in aThetaBaseB
				targetPos := (64 - keccak.LR[x][y]) / numChunkBaseX
				rotateBy := keccak.LR[x][y] / 4

				for r := start; r < stop; r++ {

					// Permutate the slices to perform a rotation of LR floored
					// to the lower multiple of 4.
					for k := 0; k < numSlice; k++ {
						slice[(k+rotateBy)%numSlice] = aThetaOut[x][y][k].Get(r)
					}

					// Then, recompose the slice to regroup all the 64 bits of
					// the slice. Now, what remains is to shift by LR % 4.
					z := BaseRecompose(slice[:], &BaseBPow4Fr)

					// The shifting by LR % 4 is done by multiplying the grouped
					// point by BaseB ^ shfBy. Then, we need to manually shave
					// the shfBy extra bits from z.
					shfBy := keccak.LR[x][y] % 4

					if shfBy != 0 {
						var shfFactor field.Element
						shfFactor.Exp(BaseBFr, big.NewInt(int64(shfBy)))
						z.Mul(&z, &shfFactor)

						// The trick here, is that slice decompose is returns the
						// extra limb as the last entry.
						extraLimb := DecomposeFr(z, int(BaseBPow4), numSlice+1)[numSlice]
						// readd the missing limb at the start
						z.Add(&z, &extraLimb)
						extraLimb.Mul(&extraLimb, &shfBy64)
						// remove the missing limb at the end
						z.Sub(&z, &extraLimb)
					}

					// Save the value of z
					aRho[x][y][r] = z

					// Assigns the target decomposition slice. It is used to
					// justify that the extra limb is well constructed w.r.t.
					// aTheta. This is only required if there exists an extra
					// limbs in the first place however.
					if shfBy != 0 {
						targetSlice := aThetaOut[x][y][targetPos].Get(r)
						decomposed := DecomposeFr(targetSlice, BaseB, 4)
						for b := 0; b < numChunkBaseX; b++ {
							targSliceDec[x][y][b][r] = decomposed[b]
						}
					}

				}
			}
		}

	})

	// Assigns the columns
	colSize := r.ARho[0][0].Size()

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			run.AssignColumn(
				r.ARho[x][y].GetColID(),
				smartvectors.RightZeroPadded(aRho[x][y], colSize),
			)

			if keccak.LR[x][y]%numChunkBaseX == 0 {
				// Skip the allocation of the target slice
				continue
			}

			for b := 0; b < numChunkBaseX; b++ {
				run.AssignColumn(
					r.TargetSliceDecompose[x][y][b].GetColID(),
					smartvectors.RightZeroPadded(targSliceDec[x][y][b], colSize),
				)
			}
		}
	}
}

package emulated

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// AssignEmulatedColumnsAction is the serializable replacement for the closure.
// type AssignEmulatedColumnsAction struct {
// 	Terms         [][]limbs.Limbs[limbs.LittleEndian]
// 	Modulus       limbs.Limbs[limbs.LittleEndian]
// 	Quotient      limbs.Limbs[limbs.LittleEndian]
// 	Carry         limbs.Limbs[limbs.LittleEndian]
// 	NbBitsPerLimb int
// 	NbLimbs       int
// }

// assignEmulatedColumns assigns the values to the columns used in the emulated evaluation
func (a *AssignEmulatedColumnsProverAction) Run(run *wizard.ProverRuntime) {
	nbRows := a.Modulus.NumRow()

	var (
		srcTerms   = make([][]limbs.VecRow[limbs.LittleEndian], len(a.Terms))
		srcModulus = a.Modulus.GetAssignment(run)
	)
	for i := range a.Terms {
		srcTerms[i] = make([]limbs.VecRow[limbs.LittleEndian], len(a.Terms[i]))
		for j := range a.Terms[i] {
			srcTerms[i][j] = a.Terms[i][j].GetAssignment(run)
		}
	}

	var (
		dstQuoLimbs = make([][]field.Element, a.Quotient.NumLimbs())
		dstCarry    = make([][]field.Element, a.Carry.NumLimbs())
	)
	for i := range dstQuoLimbs {
		dstQuoLimbs[i] = make([]field.Element, nbRows)
	}
	for i := range dstCarry {
		dstCarry[i] = make([]field.Element, nbRows)
	}

	parallel.Execute(nbRows, func(start, end int) {
		// initialize all buffers to avoid reallocations
		bufWit := make([]uint64, a.NbLimbs)
		// we allocate LHS twice as we set the result value and we modify the buffer
		bufTermProd1 := make([]uint64, len(dstCarry))
		bufTermProd2 := make([]uint64, len(dstCarry))
		bufLhs := make([]*big.Int, len(dstCarry))
		for i := range bufLhs {
			bufLhs[i] = new(big.Int)
		}

		bufMod := make([]uint64, a.Modulus.NumLimbs())
		bufQuo := make([]uint64, a.Quotient.NumLimbs())
		bufRhs := make([]uint64, nbMultiplicationResLimbs(len(bufQuo), len(bufMod)))

		wit := new(big.Int)
		witModulus := new(big.Int)

		tmpEval := new(big.Int)
		tmpQuotient := new(big.Int)
		carry := new(big.Int)
		tmpTermProduct := new(big.Int)
		tmpRemainder := new(big.Int) // only for assigning and checking that the eval result is 0
		tmpUTBi := new(big.Int)

		for i := start; i < end; i++ {
			// clear all buffers
			clearBuffer(bufMod)
			clearBuffer(bufQuo)
			clearBuffer(bufRhs)
			clearBuffer(bufLhs)
			clearBuffer(bufWit)
			tmpEval.SetInt64(0)
			tmpQuotient.SetInt64(0)
			// recompose all terms
			for j := range a.Terms {
				clearBuffer(bufTermProd1)
				bufTermProd1[0] = 1 // multiplication identity
				clearBuffer(bufTermProd2)
				tmpTermProduct.SetInt64(1)
				termNbLimbs := 1
				for k := range a.Terms[j] {
					nbLimbs := a.Terms[j][k].NumLimbs()
					// recompose the term value as integer
					if err := limbsToBigInt(wit, bufWit[:nbLimbs], srcTerms[j][k], i, a.NbBitsPerLimb); err != nil {
						utils.Panic("failed to convert witness term [%d][%d]: %v", j, k, err)
					}
					if wit.Sign() != 0 {
						// perform integer multiplication
						tmpTermProduct.Mul(tmpTermProduct, wit)
						// perform limb multiplication
						if err := limbMul(bufTermProd2[:nbMultiplicationResLimbs(termNbLimbs, nbLimbs)], bufTermProd1[:termNbLimbs], bufWit[:nbLimbs]); err != nil {
							utils.Panic("failed to multiply LHS2 and LHS1: %v", err)
						}
					} else {
						// when one term is zero, then the whole term is zero
						tmpTermProduct.SetInt64(0)
						clearBuffer(bufTermProd2)
					}
					termNbLimbs = nbMultiplicationResLimbs(termNbLimbs, nbLimbs)
					bufTermProd2, bufTermProd1 = bufTermProd1, bufTermProd2
				}
				// accumulate the term into the evaluation as integer and as limbs
				if a.Coeffs[j] == 1 {
					tmpEval.Add(tmpEval, tmpTermProduct)
					for i := range bufLhs {
						bufLhs[i].Add(bufLhs[i], tmpUTBi.SetUint64(bufTermProd1[i]))
					}
				} else {
					tmpEval.Sub(tmpEval, tmpTermProduct)
					for i := range bufLhs {
						bufLhs[i].Sub(bufLhs[i], tmpUTBi.SetUint64(bufTermProd1[i]))
					}
				}
			}
			// recompose the modulus as integer
			if err := limbsToBigInt(witModulus, bufMod, srcModulus, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert witness modulus: %v", err)
			}
			// compute the quotient from the accumulated evaluation. NB! We require the evaluation
			// value to be zero.
			switch {
			case witModulus.Sign() != 0 && tmpEval.Sign() != 0:
				// we have both nonzero modulus and eval.
				tmpQuotient.QuoRem(tmpEval, witModulus, tmpRemainder)
				if tmpRemainder.Sign() != 0 {
					utils.Panic("emulated evaluation at row %d: evaluation not divisible by modulus: tmpEval=%v tmpModulus=%v", i, tmpEval.Text(16), witModulus.Text(16))
				}
				if tmpQuotient.Sign() < 0 {
					utils.Panic("emulated evaluation at row %d: quotient is negative (tmpEval=%v tmpModulus=%v); negative quotients are not supported", i, tmpEval.Text(16), witModulus.Text(16))
				}
			case witModulus.Sign() == 0 && tmpEval.Sign() != 0:
				// modulus is zero, eval non zero => invalid
				utils.Panic("modulus cannot be zero when evaluation is non zero")
			default:
				// eval is zero, quotient and remainder are zero. We don't
				// need to reset as the values are zeroed already
			}
			// assign the quotient limbs from the computed quotient
			if err := bigIntToLimbs(tmpQuotient, bufQuo, a.Quotient, dstQuoLimbs, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert quotient to limbs: %v", err)
			}
			// compute the carry limbs from the difference
			if tmpQuotient.Sign() != 0 && witModulus.Sign() != 0 {
				if err := limbMul(bufRhs, bufQuo, bufMod); err != nil {
					utils.Panic("failed to compute quotient * modulus: %v", err)
				}
			}
			carry.SetInt64(0)
			for j := range dstCarry {
				if j < len(bufLhs) {
					carry.Add(carry, bufLhs[j])
				}
				if j < len(bufRhs) {
					carry.Sub(carry, tmpUTBi.SetUint64(bufRhs[j]))
				}
				carry.Rsh(carry, uint(a.NbBitsPerLimb))
				dstCarry[j][i].SetBigInt(carry)
			}
		}
	})

	for i, l := range a.Quotient.GetLimbs() {
		run.AssignColumn(l.GetColID(), smartvectors.NewRegular(dstQuoLimbs[i]))
	}
	for i, l := range a.Carry.GetLimbs() {
		run.AssignColumn(l.GetColID(), smartvectors.NewRegular(dstCarry[i]))
	}
}

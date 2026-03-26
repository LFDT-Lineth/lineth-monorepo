package emulated

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

func (a *MultiplicationAssignmentProverAction) Run(run *wizard.ProverRuntime) {
	nbRows := a.TermL.NumRow()
	var (
		srcTermL   = a.TermL.GetAssignment(run)
		srcTermR   = a.TermR.GetAssignment(run)
		srcModulus = a.Modulus.GetAssignment(run)
	)
	var (
		dstQuoLimbs = make([][]field.Element, a.Quotient.NumLimbs())
		dstRemLimbs = make([][]field.Element, a.Result.NumLimbs())
		dstCarry    = make([][]field.Element, a.Carry.NumLimbs())
	)
	for i := range dstQuoLimbs {
		dstQuoLimbs[i] = make([]field.Element, nbRows)
	}
	for i := range dstRemLimbs {
		dstRemLimbs[i] = make([]field.Element, nbRows)
	}
	for i := range dstCarry {
		dstCarry[i] = make([]field.Element, nbRows)
	}

	parallel.Execute(nbRows, func(start, end int) {
		bufL := make([]uint64, a.TermL.NumLimbs())
		bufR := make([]uint64, a.TermR.NumLimbs())
		bufMod := make([]uint64, a.Modulus.NumLimbs())
		bufQuo := make([]uint64, a.Quotient.NumLimbs())
		bufRem := make([]uint64, a.Result.NumLimbs())
		// to compute the carries, we need to perform multiplication on limbs
		bufLhs := make([]uint64, nbMultiplicationResLimbs(len(bufL), len(bufR)))
		bufRhs := make([]uint64, nbMultiplicationResLimbs(len(bufQuo), len(bufMod)))

		witTermL := new(big.Int)
		witTermR := new(big.Int)
		witModulus := new(big.Int)

		tmpProduct := new(big.Int)
		tmpQuotient := new(big.Int)
		tmpRemainder := new(big.Int)
		tmpU64ToBig := new(big.Int)
		carry := new(big.Int)
		for i := start; i < end; i++ {
			// we can reuse all the big ints here
			if err := limbsToBigInt(witTermL, bufL, srcTermL, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert witness term L: %v", err)
			}
			if err := limbsToBigInt(witTermR, bufR, srcTermR, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert witness term R: %v", err)
			}
			if err := limbsToBigInt(witModulus, bufMod, srcModulus, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert witness modulus: %v", err)
			}
			tmpProduct.Mul(witTermL, witTermR)
			switch {
			case witModulus.Sign() != 0 && tmpProduct.Sign() != 0:
				// we have both nonzero modulus and product.
				tmpQuotient.QuoRem(tmpProduct, witModulus, tmpRemainder)
			case witModulus.Sign() == 0 && tmpProduct.Sign() != 0:
				// modulus is zero, product non zero => invalid
				utils.Panic("modulus cannot be zero when product is non zero")
			default:
				// product is zero, quotient and remainder are zero. We don't
				// need to reset as the values are zeroed already
			}
			if err := bigIntToLimbs(tmpQuotient, bufQuo, a.Quotient, dstQuoLimbs, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert quotient to limbs: %v", err)
			}
			if err := bigIntToLimbs(tmpRemainder, bufRem, a.Result, dstRemLimbs, i, a.NbBitsPerLimb); err != nil {
				utils.Panic("failed to convert remainder to limbs: %v", err)
			}
			if err := limbMul(bufLhs, bufL, bufR); err != nil {
				utils.Panic("failed to multiply lhs limbs: %v", err)
			}
			if err := limbMul(bufRhs, bufQuo, bufMod); err != nil {
				utils.Panic("failed to multiply rhs limbs: %v", err)
			}
			for j := range bufRem {
				bufRhs[j] += bufRem[j]
			}

			for j := range dstCarry {
				if j < len(bufLhs) {
					carry.Add(carry, tmpU64ToBig.SetUint64(bufLhs[j]))
				}
				if j < len(bufRhs) {
					carry.Sub(carry, tmpU64ToBig.SetUint64(bufRhs[j]))
				}
				carry.Rsh(carry, uint(a.NbBitsPerLimb))
				dstCarry[j][i].SetBigInt(carry)
			}

			clearBuffer(bufL)
			clearBuffer(bufR)
			clearBuffer(bufMod)
			clearBuffer(bufQuo)
			clearBuffer(bufRem)
			clearBuffer(bufLhs)
			clearBuffer(bufRhs)
			tmpProduct.SetUint64(0)
			tmpQuotient.SetUint64(0)
			tmpRemainder.SetUint64(0)
			carry.SetUint64(0)
		}
	})
	for i, l := range a.Quotient.GetLimbs() {
		run.AssignColumn(l.GetColID(), smartvectors.NewRegular(dstQuoLimbs[i]))
	}
	for i, l := range a.Result.GetLimbs() {
		run.AssignColumn(l.GetColID(), smartvectors.NewRegular(dstRemLimbs[i]))
	}
	for i, l := range a.Carry.GetLimbs() {
		run.AssignColumn(l.GetColID(), smartvectors.NewRegular(dstCarry[i]))
	}
}

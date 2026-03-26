package packing

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type lookUpTables struct {
	//colNumber:=(0,1,2) and colPowers:=(1, 2^(8*1), 2^(8*2))
	ColNumber ifaces.Column
	ColPowers ifaces.Column
}

// It commits to the lookUp tables used by dataTransfer module.
func NewLookupTables(comp *wizard.CompiledIOP) lookUpTables {
	res := lookUpTables{}
	// table for powers of numbers (used for decomposition of clean limbs)
	colNum, colPower2 := numToPower2(MAXNBYTE)
	res.ColNumber = comp.InsertPrecomputed(ifaces.ColIDf("LookUp_Num"), colNum)
	res.ColPowers = comp.InsertPrecomputed(ifaces.ColIDf("LookUp_Powers"), colPower2)

	return res
}

func numToPower2(n int) (colNum, colPower2 smartvectors.SmartVector) {
	var num, power2 []field.Element
	var res field.Element
	for i := 0; i <= n; i++ {
		num = append(num, field.NewElement(uint64(i)))
		res.Exp(field.NewElement(POWER8), big.NewInt(int64(i)))
		power2 = append(power2, res)
	}
	size := utils.NextPowerOfTwo(n + 1)
	return smartvectors.RightZeroPadded(num, size),
		smartvectors.RightPadded(power2, field.One(), size)
}

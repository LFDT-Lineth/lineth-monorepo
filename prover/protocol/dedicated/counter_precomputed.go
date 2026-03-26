package dedicated

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// CounterPrecomputed generates a precomputed column counting "from" ... "to",
// (excluding "to"). If the columns is already declared, the column is reused.
// The function expects a "power of two"-sized range, so that "to - from = 2**N"
func CounterPrecomputed(comp *wizard.CompiledIOP, from, to int) ifaces.Column {
	name := ifaces.ColIDf("COUNTER_%v_%v_%v_%v", from, to, comp.SelfRecursionCount, comp.Columns.NumEntriesTotal())

	if from >= to || !utils.IsPowerOfTwo(to-from) {
		utils.Panic("invalid range from=%v to=%v", from, to)
	}

	value := make([]field.Element, to-from)
	for i := from; i < to; i++ {
		value[i-from] = field.NewElement(uint64(i))
	}

	return comp.InsertPrecomputed(name, smartvectors.NewRegular(value))
}

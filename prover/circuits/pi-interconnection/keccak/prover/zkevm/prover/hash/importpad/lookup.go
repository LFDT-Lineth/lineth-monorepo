package importpad

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// newLookup creates a range-checking lookup table to ensure that the padding
// is properly done. The returned lookup table stores all the value in the
// range 1..size and is padded with ones to the next power of two.
//
// The function also caches the tables so it can be safely called multiple time
// , it won't recreate the same lookup table several times.
func getLookupForSize(comp *wizard.CompiledIOP, size int) ifaces.Column {

	var (
		res  = make([]field.Element, size)
		name = ifaces.ColIDf("LOOKUP_TABLE_RANGE_1_%v", size)
	)

	if comp.Columns.Exists(name) {
		return comp.Columns.GetHandle(name)
	}

	for i := range res {
		res[i].SetInt64(int64(i) + 1)
	}

	return comp.InsertPrecomputed(
		name,
		smartvectors.RightPadded(res, field.One(), utils.NextPowerOfTwo(size)),
	)
}

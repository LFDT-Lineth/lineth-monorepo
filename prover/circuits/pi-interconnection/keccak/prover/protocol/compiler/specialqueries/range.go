package specialqueries

import (
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

const (
	COMPILER_RANGE string = "COMPILER_RANGE"
	IN_POLY_RANGE  string = "IN_POLY_RANGE"
)

/*
Reduce all range proofs
*/
func RangeProof(comp *wizard.CompiledIOP) {

	ctx := rangeCtx{
		rangePoly: map[int]ifaces.Column{},
		ranges:    []int{},
	}
	numRounds := comp.NumRounds()

	for roundID := 0; roundID < numRounds; roundID++ {
		queries := comp.QueriesNoParams.AllKeysAt(roundID)
		for _, qName := range queries {

			q, ok := comp.QueriesNoParams.Data(qName).(query.Range)

			// Not a range, don't care
			if !ok {
				continue
			}

			// Skip if it was already compiled, else mark it as compiled
			if comp.QueriesNoParams.MarkAsIgnored(qName) {
				continue
			}

			/*
				Get the range poly
			*/
			rangePoly, ok := ctx.rangePoly[q.B]
			if !ok {
				/*
					Basically, the polynomial that will contain 0..q.B
					Always registered at round zero. We do not check anything
					regarding it, but we will put it as a precomputed.
				*/
				rangePoly = comp.InsertPrecomputed(rangePolyName(comp.SelfRecursionCount, q.B), ZeroToB(q.B))
				ctx.rangePoly[q.B] = rangePoly
				ctx.ranges = append(ctx.ranges, q.B)
			}

			comp.InsertInclusion(
				roundID,
				deriveName[ifaces.QueryID]("RANGE", q.ID, IN_POLY_RANGE),
				[]ifaces.Column{rangePoly},
				[]ifaces.Column{q.Handle},
			)

		}
	}

}

/*
The range context holds informations regarding a batch of
range proofs.
*/
type rangeCtx struct {
	// The range polys for each requested range
	rangePoly map[int]ifaces.Column
	// List of the ranges tha
	ranges []int
}

func rangePolyName(selfRecursionCnt int, i int) ifaces.ColID {
	return ifaces.ColIDf("%v_%v_%v", COMPILER_RANGE, selfRecursionCnt, i)
}

// returns a smart-vector containing [0, b)
func ZeroToB(b int) sv.SmartVector {

	// If b is not a power of two, we pad with zeroes
	lenWit := utils.NextPowerOfTwo(b)
	res := make([]field.Element, lenWit)

	for i := 0; i < b; i++ {
		res[i].SetUint64(uint64(i))
	}

	return sv.NewRegular(res)
}

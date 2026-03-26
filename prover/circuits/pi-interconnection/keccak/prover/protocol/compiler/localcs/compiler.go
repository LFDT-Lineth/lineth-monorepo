package localcs

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

/*
Derive a name for a a coin created during the compilation process
*/
func deriveName[R ~string](context string, q ifaces.QueryID, name string) R {
	var res string
	if q == "" {
		res = fmt.Sprintf("%v_%v", context, name)
	} else {
		res = fmt.Sprintf("%v_%v_%v", q, context, name)
	}
	return R(res)
}

/*
Compiles the local constraints
*/
func Compile(comp *wizard.CompiledIOP) {

	compileOpeningsToConstraints(comp)

	numRounds := comp.NumRounds()

	/*
		First compile all local constraints
	*/
	for i := 0; i < numRounds; i++ {
		queries := comp.QueriesNoParams.AllKeysAt(i)

		for _, qName := range queries {

			// Skip if it was already compiled
			if comp.QueriesNoParams.IsIgnored(qName) {
				continue
			}

			if q_, ok := comp.QueriesNoParams.Data(qName).(query.LocalConstraint); ok {
				ReduceLocalConstraint(comp, q_, i)
			}
		}
	}
}

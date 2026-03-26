package univariates

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

/*
Derive a name for a a coin created during the compilation process
*/
func deriveName[R ~string](comp *wizard.CompiledIOP, context string, q ifaces.QueryID, name string) R {
	var res string
	if q == "" {
		res = fmt.Sprintf("%v_%v_%v", context, comp.SelfRecursionCount, name)
	} else {
		res = fmt.Sprintf("%v_%v_%v_%v", q, context, comp.SelfRecursionCount, name)
	}
	return R(res)
}

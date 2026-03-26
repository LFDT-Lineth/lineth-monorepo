package plonkinternal

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
)

// Returns the complete Name of a gate columns
func (ctx *compilationCtx) colIDf(Name string, args ...any) ifaces.ColID {
	fmtted := fmt.Sprintf(Name, args...)
	return ifaces.ColIDf("%v_%v", ctx.Name, fmtted)
}

// Returns a queryID
func (ctx *compilationCtx) queryIDf(Name string, args ...any) ifaces.QueryID {
	fmtted := fmt.Sprintf(Name, args...)
	return ifaces.QueryIDf("%v_%v", ctx.Name, fmtted)
}

// Returns a labeled string
func (ctx *compilationCtx) Sprintf(Name string, args ...any) string {
	fmtted := fmt.Sprintf(Name, args...)
	return fmt.Sprintf("%v_%v", ctx.Name, fmtted)
}

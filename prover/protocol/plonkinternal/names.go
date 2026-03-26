package plonkinternal

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
)

// Returns the complete name of a gate columns
func (ctx *CompilationCtx) colIDf(name string, args ...any) ifaces.ColID {
	fmtted := fmt.Sprintf(name, args...)
	return ifaces.ColIDf("%v_%v", ctx.Name, fmtted)
}

// Returns a queryID
func (ctx *CompilationCtx) queryIDf(name string, args ...any) ifaces.QueryID {
	fmtted := fmt.Sprintf(name, args...)
	return ifaces.QueryIDf("%v_%v", ctx.Name, fmtted)
}

// Returns a labeled string
func (ctx *CompilationCtx) Sprintf(name string, args ...any) string {
	fmtted := fmt.Sprintf(name, args...)
	return fmt.Sprintf("%v_%v", ctx.Name, fmtted)
}

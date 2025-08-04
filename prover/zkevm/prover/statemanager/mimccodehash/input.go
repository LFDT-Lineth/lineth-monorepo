package mimccodehash

import (
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// The first four elements are to be populated from the ROM module,
// the last element is to be computed by the IsZero query
type RomInput struct {
	CFI                             ifaces.Column
	Acc                             ifaces.Column
	NBytes                          ifaces.Column
	Counter                         ifaces.Column
	CodeSize                        ifaces.Column
	CounterIsEqualToNBytesMinusOne  ifaces.Column
	CptCounterEqualToNBytesMinusOne wizard.ProverAction
}

type RomLexInput struct {
	CFIRomLex  ifaces.Column
	CodeHashHi ifaces.Column
	CodeHashLo ifaces.Column
}

// complete constructs the IsZero columns "CounterIsEqualToNBytesMinusOne" and
// the corresponding prover action "CptCounterEqualToNBytesMinusOne". It is
// run at the beginning of the "ConnectToRom" module.
func (inp *RomInput) complete(comp *wizard.CompiledIOP) *RomInput {
	inp.CounterIsEqualToNBytesMinusOne, inp.CptCounterEqualToNBytesMinusOne = dedicated.IsZero(comp, symbolic.Sub(inp.NBytes, inp.Counter, 1)).GetColumnAndProverAction()
	return inp
}

func (inp *RomInput) completeAssign(run *wizard.ProverRuntime) {
	inp.CptCounterEqualToNBytesMinusOne.Run(run)
}

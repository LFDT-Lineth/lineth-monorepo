package byte32cmp

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

type Bytes32CmpProverAction struct {
	Bcp *BytesCmpCtx
}

func (a *Bytes32CmpProverAction) Run(run *wizard.ProverRuntime) {
	colA := a.Bcp.ColumnA.GetColAssignment(run)
	colB := a.Bcp.ColumnB.GetColAssignment(run)
	a.Bcp.assign(run, colA, colB)
}

func Bytes32Cmp(
	// compiled IOP
	comp *wizard.CompiledIOP,
	// number of limbs and bit per limbs to represent a byte32 value
	numLimbs, bitPerLimbs int,
	// name of the byte32 comparison instance
	name string,
	// ColumnA, ColumnB, allegedly the column with larger and smaller values respectively
	columnA, columnB ifaces.Column,
	// activeRows works as a filter on the checking
	activeRow *symbolic.Expression,
) {
	bcp := BytesCmpCtx{}
	round := column.MaxRound(columnA, columnB)
	bcp.Round = round
	bcp.ColumnA = columnA
	bcp.ColumnB = columnB
	bcp.ActiveRow = activeRow
	comp.RegisterProverAction(round, &Bytes32CmpProverAction{
		// Must pass pointer here
		Bcp: &bcp,
	})
	// We do call the assign function before define to avoid the race condition with
	// the bigrange module
	bcp.Define(comp, numLimbs, bitPerLimbs, name)
}

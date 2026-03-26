package cleanup

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// Simple compilation steps which frees ignored items
func CleanUp(comp *wizard.CompiledIOP) {
	// Gets the last round of the comp
	lastRound := comp.NumRounds() - 1
	// Get the list of all ignored columns
	colToRemove := comp.Columns.AllKeysIgnored()

	// Register the prover action to remove unrequired data
	comp.RegisterProverAction(lastRound, &CleanupProverAction{
		ColumnsToRemove: colToRemove,
	})
}

// CleanupProverAction is the action to remove ignored columns.
// It implements the [wizard.ProverAction] interface.
type CleanupProverAction struct {
	ColumnsToRemove []ifaces.ColID
}

// Run executes the cleanup by removing ignored columns from the runtime.
func (a *CleanupProverAction) Run(run *wizard.ProverRuntime) {
	// Remove all the ignored columns
	for _, col := range a.ColumnsToRemove {
		run.Columns.TryDel(col)
	}
}

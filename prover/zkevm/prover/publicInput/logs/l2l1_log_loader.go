package logs

import (
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// NewL2L1LogLoader returns a new LogHasher with initialized columns that are not constrained.
func NewL2L1LogLoader(comp *wizard.CompiledIOP, size int, name string, fetched ExtractedData) *dedicated.Compactification {
	return dedicated.Compactify(comp, fetched.Data[:], fetched.FilterFetched, name)
}

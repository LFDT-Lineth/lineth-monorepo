package mimc

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

// CompileMiMC compiles the MiMC queries by checking each query individually in a single loop
func CompileMiMC(comp *wizard.CompiledIOP) {
	hasMiMCQueries := false

	for _, id := range comp.QueriesNoParams.AllUnignoredKeys() {
		// Fetch the query
		q := comp.QueriesNoParams.Data(id)
		qMiMC, ok := q.(query.MiMC)
		if !ok {
			// Not a MiMC query, skip it
			continue
		}

		// Mark it as ignored
		comp.QueriesNoParams.MarkAsIgnored(id)
		hasMiMCQueries = true

		// Apply manual check to the query immediately
		logrus.Debugf("MiMC compiler: checking query %v individually", qMiMC.ID)
		manualCheckMiMCBlock(comp, qMiMC.Blocks, qMiMC.OldState, qMiMC.NewState)
	}

	if !hasMiMCQueries {
		logrus.Debug("MiMC compiler exited: no MiMC queries to compile")
	}
}

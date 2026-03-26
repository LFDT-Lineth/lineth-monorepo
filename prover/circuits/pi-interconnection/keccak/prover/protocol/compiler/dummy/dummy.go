package dummy

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

/*
Transforms all oracle commitments into explicit messages and instructs the verifier
to manually check all associated queries against them.

This utility is primarily intended for testing and debugging, allowing inspection of commitment-query
consistency without automated verification.
*/
func Compile(comp *wizard.CompiledIOP) {

	comp.DummyCompiled = true

	/*
		Registers all declared commitments and query parameters
		as messages in the same round. This steps is only relevant
		for the compiledIOP. We elaborate on how to update the provers
		and verifiers to account for this. Additionally, we take the queries
		as we compile them from the `CompiledIOP`.
	*/
	numRounds := comp.NumRounds()

	/*
		The filter returns true, as long as the query has not been marked as
		already compiled. This is to avoid them being compiled a second time.
	*/
	queriesParamsToCompile := comp.QueriesParams.AllUnignoredKeys()
	queriesNoParamsToCompile := comp.QueriesNoParams.AllUnignoredKeys()

	for i := 0; i < numRounds; i++ {
		// Mark all the commitments as messages
		coms := comp.Columns.AllKeysAt(i)
		for _, com := range coms {
			// Check the status of the commitment
			status := comp.Columns.Status(com)

			if status == column.Ignored {
				// If the column is ignored, we can just skip it
				continue
			}

			if status.IsPublic() {
				// Nothing specific to do on the prover side
				continue
			}

			// Mark them as "public" to the verifier
			switch status {
			case column.Precomputed:
				// send it to the verifier directly as part of the verifying key
				comp.Columns.SetStatus(com, column.VerifyingKey)
			case column.Committed:
				// send it to the verifier directly as part of the proof
				comp.Columns.SetStatus(com, column.Proof)
			default:
				utils.Panic("Unknown status : %v", status.String())
			}
		}
	}

	/*
		And mark the queries as already compiled
	*/
	for _, q := range queriesNoParamsToCompile {
		comp.QueriesNoParams.MarkAsIgnored(q)
	}

	for _, q := range queriesParamsToCompile {
		comp.QueriesParams.MarkAsIgnored(q)
	}

	/*
		One step to be run at the end, by verifying every constraint
		"a la mano"
	*/
	comp.RegisterVerifierAction(numRounds-1, &DummyVerifierAction{
		Comp:                     comp,
		QueriesParamsToCompile:   queriesParamsToCompile,
		QueriesNoParamsToCompile: queriesNoParamsToCompile,
	})

	logrus.Debugf("NB: The gnark circuit does not check the verifier of the dummy reduction\n")
}

// DummyVerifierAction is the action to verify queries in the dummy compiler.
// It implements the [wizard.VerifierAction] interface.
type DummyVerifierAction struct {
	Comp                     *wizard.CompiledIOP
	QueriesParamsToCompile   []ifaces.QueryID
	QueriesNoParamsToCompile []ifaces.QueryID
}

// Run executes the verifier action, checking all queries in parallel.
func (a *DummyVerifierAction) Run(run wizard.Runtime) error {
	var finalErr error
	lock := sync.Mutex{}

	/*
		Test all the query with parameters
	*/
	parallel.Execute(len(a.QueriesParamsToCompile), func(start, stop int) {
		for i := start; i < stop; i++ {
			name := a.QueriesParamsToCompile[i]
			lock.Lock()
			q := a.Comp.QueriesParams.Data(name)
			lock.Unlock()
			if err := q.Check(run); err != nil {
				lock.Lock()
				finalErr = fmt.Errorf("%v\nfailed %v - %v", finalErr, name, err)
				lock.Unlock()
				logrus.Debugf("query %v failed\n", name)
			} else {
				logrus.Debugf("query %v passed\n", name)
			}
		}
	})

	/*
		Test the queries without parameters
	*/
	parallel.Execute(len(a.QueriesNoParamsToCompile), func(start, stop int) {
		for i := start; i < stop; i++ {
			name := a.QueriesNoParamsToCompile[i]
			lock.Lock()
			q := a.Comp.QueriesNoParams.Data(name)
			lock.Unlock()
			if err := q.Check(run); err != nil {
				lock.Lock()
				finalErr = fmt.Errorf("%v\nfailed %v - %v", finalErr, name, err)
				lock.Unlock()
			} else {
				logrus.Debugf("query %v passed\n", name)
			}
		}
	})

	/*
		Nil to indicate all checks passed
	*/
	return finalErr
}

// RunGnark executes the verifier action in a Gnark circuit.
// In this dummy implementation, no constraints are enforced.
func (a *DummyVerifierAction) RunGnark(api frontend.API, gnarkRun wizard.GnarkRuntime) {
	// No constraints are enforced in the dummy reduction, as per the original empty function
}

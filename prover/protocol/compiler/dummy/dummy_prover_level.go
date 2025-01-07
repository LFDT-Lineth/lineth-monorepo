package dummy

import (
	"fmt"
	"sync"

	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

// CompileAtProverLvl instantiate the oracle as the prover. Meaning that the
// prover is responsible for checking all the queries and the verifier does not
// see any compiled IOP.
//
// This is useful for quick "manual" testing and debugging. One perk is that
// unlike [CompileAtVerifierLvl] the FS is not pressured as we don't push many
// column in plain-text to the verifier. The drawback is that since it happens
// at prover level, the "errors" result in panics. This makes it not very
// suitable for established unit-tests where we want to analyze the errors.
func CompileAtProverLvl(comp *wizard.CompiledIOP) {

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

	/*
		One step to be run at the end, by verifying every constraint
		"a la mano"
	*/
	verifier := func(run *wizard.ProverRuntime) {

		logrus.Infof("started to run the dummy verifier")

		var finalErr error
		lock := sync.Mutex{}

		/*
			Test all the query with parameters
		*/
		parallel.Execute(len(queriesParamsToCompile), func(start, stop int) {
			for i := start; i < stop; i++ {
				name := queriesParamsToCompile[i]
				lock.Lock()
				q := comp.QueriesParams.Data(name)
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
		parallel.Execute(len(queriesNoParamsToCompile), func(start, stop int) {
			for i := start; i < stop; i++ {
				name := queriesNoParamsToCompile[i]
				lock.Lock()
				q := comp.QueriesNoParams.Data(name)
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

		if finalErr != nil {
			utils.Panic("dummy.Compile brought errors: %v", finalErr.Error())
		}
	}

	logrus.Debugf("NB: The gnark circuit does not check the verifier of the dummy reduction\n")
	comp.SubProvers.AppendToInner(numRounds-1, verifier)

}

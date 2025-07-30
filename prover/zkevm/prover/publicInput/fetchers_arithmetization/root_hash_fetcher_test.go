package fetchers_arithmetization

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	stmCommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mock"
	stmgr "github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
	"github.com/stretchr/testify/assert"
)

// Constructs a state summary struct based on predefined test data and uses it to test
// the RootHashFetcher (which is supposed to extract the first/final root hashes from the state summary)
func TestRootHashFetcher(t *testing.T) {

	var (
		tContext = stmCommon.InitializeContext(100)
		ss       stmgr.Module
		fetcher  *RootHashFetcher
	)

	for i, tCase := range tContext.TestCases {

		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			t.Logf("Test case explainer: %v", tCase.Explainer)

			define := func(b *wizard.Builder) {
				ss = stmgr.NewModule(b.CompiledIOP, 1<<6)
				fetcher = NewRootHashFetcher(b.CompiledIOP, "ROOT_HASH_FETCHER_FROM_STATE_SUMMARY", ss.IsActive.Size())
				DefineRootHashFetcher(b.CompiledIOP, fetcher, "ROOT_HASH_FETCHER_FROM_STATE_SUMMARY", ss)
			}

			prove := func(run *wizard.ProverRuntime) {

				var (
					initState      = tContext.State
					shomeiState    = mock.InitShomeiState(initState)
					initRootBytes  = shomeiState.AccountTrie.TopRoot()
					stateLogs      = tCase.StateLogsGens(initState)
					shomeiTraces   = mock.StateLogsToShomeiTraces(shomeiState, stateLogs)
					finalRootBytes = shomeiState.AccountTrie.TopRoot()
					initRoot       field.Element
					finalRoot      field.Element
				)
				// nil for the scp because we do not need to ensure consistency with SCP values for this test
				ss.Assign(run, shomeiTraces)
				// assign the RootHashFetcher
				AssignRootHashFetcher(run, fetcher, ss)
				// compute two field elements that correspond to the Shomei initial and final root hash in the account tries
				initRoot.SetBytes(initRootBytes[:])
				finalRoot.SetBytes(finalRootBytes[:])
				// check that the fetcher works properly
				assert.Equal(t, initRoot, fetcher.First.GetColAssignmentAt(run, 0), "Initial root value is incorrect")
				assert.Equal(t, finalRoot, fetcher.Last.GetColAssignmentAt(run, 0), "Final root value is incorrect")
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prove)
			err := wizard.Verify(comp, proof)

			if err != nil {
				t.Fatalf("verification failed: %v", err)
			}
		})

	}
}

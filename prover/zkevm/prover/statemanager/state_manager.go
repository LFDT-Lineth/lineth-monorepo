package statemanager

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/accumulator"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/accumulatorsummary"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/codehashconsistency"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mimccodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// StateManager is a collection of modules responsible for attesting the
// correctness of the state-transitions occuring in Linea w.r.t. to the
// arithmetization.
type StateManager struct {
	accumulator                 accumulator.Module
	accumulatorSummaryConnector accumulatorsummary.Module
	StateSummary                statesummary.Module // exported because needed by the public input module
	mimcCodeHash                mimccodehash.Module
	codeHashConsistency         codehashconsistency.Module
}

// Settings stores all the setting to construct a StateManager and is passed to
// the [NewStateManager] function. All the settings of the submodules are
// constructed based on this structure
type Settings struct {
	AccSettings      accumulator.Settings
	MiMCCodeHashSize int
}

// NewStateManager instantiate the [StateManager] module
func NewStateManager(comp *wizard.CompiledIOP, settings Settings) *StateManager {

	sm := &StateManager{
		StateSummary: statesummary.NewModule(comp, settings.stateSummarySize()),
		accumulator:  accumulator.NewModule(comp, settings.AccSettings),
		mimcCodeHash: mimccodehash.NewModule(comp, mimccodehash.Inputs{
			Name: "MiMCCodeHash",
			Size: settings.MiMCCodeHashSize,
		}),
	}

	sm.accumulatorSummaryConnector = *accumulatorsummary.NewModule(
		comp,
		accumulatorsummary.Inputs{
			Name:        "ACCUMULATOR_SUMMARY",
			Accumulator: sm.accumulator,
		},
	)

	sm.accumulatorSummaryConnector.ConnectToStateSummary(comp, &sm.StateSummary)
	sm.mimcCodeHash.ConnectToRom(comp, rom(comp), romLex(comp))
	sm.StateSummary.ConnectToHub(comp, acp(comp), scp(comp))
	sm.codeHashConsistency = codehashconsistency.NewModule(comp, "CODEHASHCONSISTENCY", &sm.StateSummary, &sm.mimcCodeHash)

	return sm
}

// Assign assignes the submodules of the state-manager. It requires the
// arithmetization columns to be assigned first.
func (sm *StateManager) Assign(run *wizard.ProverRuntime, shomeiTraces [][]statemanager.DecodedTrace) {
	assignHubAddresses(run)
	addSkipFlags(&shomeiTraces)
	sm.StateSummary.Assign(run, shomeiTraces)
	sm.accumulator.Assign(run, utils.Join(shomeiTraces...))
	sm.accumulatorSummaryConnector.Assign(run)
	sm.mimcCodeHash.Assign(run)
	sm.codeHashConsistency.Assign(run)
}

// stateSummarySize returns the number of rows to give to the state-summary
// module.
func (s *Settings) stateSummarySize() int {
	return utils.NextPowerOfTwo(s.AccSettings.MaxNumProofs)
}

// addSkipFlags adds skip flags to redundant shomei traces
func addSkipFlags(shomeiTraces *[][]statemanager.DecodedTrace) {
	// AddressAndKey is a struct used as a key in order to identify skippable traces
	// in our maps
	type AddressAndKey struct {
		address    types.Bytes32
		storageKey types.Bytes32
	}
	// iterate over all the Shomei blocks
	for blockNo, vec := range *shomeiTraces {
		var (
			curAddress = types.EthAddress{}
			err        error
		)

		// instantiate the map for the current block
		traceMap := make(map[AddressAndKey]int)
		// now we process the traces themselves
		for i, trace := range vec {
			// compute the current address account
			curAddress, err = trace.GetRelatedAccount()
			if err != nil {
				panic(err)
			}
			x := *(&field.Element{}).SetBytes(curAddress[:])

			if trace.Location != statemanager.WS_LOCATION {
				// we have a STORAGE trace
				// prepare the search key
				searchKey := AddressAndKey{
					address:    x.Bytes(),
					storageKey: trace.Underlying.HKey(statemanager.MIMC_CONFIG),
				}
				previousIndex, isFound := traceMap[searchKey]
				if isFound {
					// set the previous trace as a skippable trace
					(*shomeiTraces)[blockNo][previousIndex].IsSkipped = true
				} else {
					// when not found, add its index to the map (if a duplicate is found later)
					// this stored index will be then used to make the current trace skippable
					traceMap[searchKey] = i
				}
			}
		}
	}
}

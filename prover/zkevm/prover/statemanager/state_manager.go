package statemanager

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/accumulator"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/accumulatorsummary"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/codehashconsistency"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/lineacodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// StateManager is a collection of modules responsible for attesting the
// correctness of the state-transitions occuring in Linea w.r.t. to the
// arithmetization.
type StateManager struct {
	Accumulator                 accumulator.Module
	AccumulatorSummaryConnector accumulatorsummary.Module
	StateSummary                statesummary.Module // exported because needed by the public input module
	LineaCodeHash               lineacodehash.Module
	CodeHashConsistency         codehashconsistency.Module
}

// Settings stores all the setting to construct a StateManager and is passed to
// the [NewStateManager] function. All the settings of the submodules are
// constructed based on this structure
type Settings struct {
	AccSettings       accumulator.Settings
	LineaCodeHashSize int
}

// NewStateManager instantiate the [StateManager] module
func NewStateManager(comp *wizard.CompiledIOP, settings Settings, arith *arithmetization.Arithmetization) *StateManager {

	sm := &StateManager{
		StateSummary: statesummary.NewModule(comp, settings.stateSummarySize()),
		Accumulator:  accumulator.NewModule(comp, settings.AccSettings),
		LineaCodeHash: lineacodehash.NewModule(comp, lineacodehash.Inputs{
			Name: "LineaCodeHash",
			Size: settings.LineaCodeHashSize,
		}),
	}

	sm.AccumulatorSummaryConnector = *accumulatorsummary.NewModule(
		comp,
		accumulatorsummary.Inputs{
			Name:        "ACCUMULATOR_SUMMARY",
			Accumulator: sm.Accumulator,
		},
	)

	sm.AccumulatorSummaryConnector.ConnectToStateSummary(comp, &sm.StateSummary)
	sm.LineaCodeHash.ConnectToRom(comp, rom(comp, arith), romLex(comp, arith))
	sm.StateSummary.ConnectToHub(comp, acp(comp, arith), scp(comp, arith))
	sm.CodeHashConsistency = codehashconsistency.NewModule(comp, "CODEHASHCONSISTENCY", &sm.StateSummary, &sm.LineaCodeHash)

	return sm
}

// Assign assignes the submodules of the state-manager. It requires the
// arithmetization columns to be assigned first.
func (sm *StateManager) Assign(run *wizard.ProverRuntime, arith *arithmetization.Arithmetization, shomeiTraces [][]statemanager.DecodedTrace) {

	addSkipFlags(&shomeiTraces)
	shomeiTraces = removeSystemTransactions(shomeiTraces)
	sm.StateSummary.Assign(run, shomeiTraces)
	sm.Accumulator.Assign(run, utils.Join(shomeiTraces...))
	sm.AccumulatorSummaryConnector.Assign(run)
	sm.LineaCodeHash.Assign(run)
	sm.CodeHashConsistency.Assign(run)
}

// stateSummarySize returns the number of rows to give to the state-summary
// module.
func (s *Settings) stateSummarySize() int {
	return utils.NextPowerOfTwo(s.AccSettings.MaxNumProofs)
}

func removeSystemTransactions(shomeiTraces [][]statemanager.DecodedTrace) [][]statemanager.DecodedTrace {

	cleanedTraces := [][]statemanager.DecodedTrace{}
	systemAddreses, _ := types.AddressFromHex("0xfffffffffffffffffffffffffffffffffffffffe")

	for _, blockTraces := range shomeiTraces {
		cleanedBlockTraces := []statemanager.DecodedTrace{}
		for _, trace := range blockTraces {
			address, err := trace.GetRelatedAccount()
			if err != nil {
				utils.Panic("could not get related account while removing system transactions: %v", err)
			}
			// check if address is fffffffffffffffffffffffffffffffffffffffe
			if address != systemAddreses {
				cleanedBlockTraces = append(cleanedBlockTraces, trace)
			}
		}
		cleanedTraces = append(cleanedTraces, cleanedBlockTraces)
	}
	return cleanedTraces
}

// addSkipFlags adds skip flags to redundant shomei traces
func addSkipFlags(shomeiTraces *[][]statemanager.DecodedTrace) {
	// AddressAndKey is a struct used as a key in order to identify skippable traces
	// in our maps
	type AddressAndKey struct {
		address    types.FullBytes32
		storageKey types.FullBytes32
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
			b32 := types.LeftPadToFullBytes32(curAddress[:])

			if trace.Location != statemanager.WS_LOCATION {
				// we have a STORAGE trace
				// prepare the search key
				searchKey := AddressAndKey{
					address:    b32,
					storageKey: trace.Underlying.HKey().ToBytes32(),
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

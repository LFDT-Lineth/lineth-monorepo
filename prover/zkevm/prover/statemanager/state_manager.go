package statemanager

import (
	"fmt"
	"os"

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
	printAllShomeiTraces(&shomeiTraces)
	addSkipFlags(&shomeiTraces)
	sm.StateSummary.Assign(run, shomeiTraces)
	sm.accumulator.Assign(run, utils.Join(shomeiTraces...))
	sm.accumulatorSummaryConnector.Assign(run)
	sm.mimcCodeHash.Assign(run)
	sm.codeHashConsistency.Assign(run)

	// csvtraces.FmtCsv(
	// 	files.MustOverwrite("./alex-csv/arith.csv"),
	// 	run,
	// 	[]ifaces.Column{
	// 		run.Spec.Columns.GetHandle("HUB_acp_PROVER_SIDE_ADDRESS_IDENTIFIER"),
	// 		run.Spec.Columns.GetHandle("hub.acp_ADDRESS_HI"),
	// 		run.Spec.Columns.GetHandle("hub.acp_ADDRESS_LO"),
	// 		run.Spec.Columns.GetHandle("hub.acp_BALANCE"),
	// 		run.Spec.Columns.GetHandle("hub.acp_NONCE"),
	// 		run.Spec.Columns.GetHandle("hub.acp_CODE_SIZE"),
	// 		run.Spec.Columns.GetHandle("hub.acp_CODE_HASH_HI"),
	// 		run.Spec.Columns.GetHandle("hub.acp_CODE_HASH_LO"),
	// 		run.Spec.Columns.GetHandle("hub.acp_REL_BLK_NUM"),
	// 		run.Spec.Columns.GetHandle("hub.acp_EXISTS"),
	// 		run.Spec.Columns.GetHandle("hub.acp_EXISTS_NEW"),
	// 		run.Spec.Columns.GetHandle("hub.acp_PEEK_AT_ACCOUNT"),
	// 		run.Spec.Columns.GetHandle("hub.acp_FIRST_IN_BLK"),
	// 		run.Spec.Columns.GetHandle("hub.acp_IS_PRECOMPILE"),
	// 	},
	// 	[]csvtraces.Option{},
	// )
	// csvtraces.FmtCsv(
	// 	files.MustOverwrite("./alex-csv/ss.csv"),
	// 	run,
	// 	[]ifaces.Column{
	// 		sm.StateSummary.Account.Address,
	// 		sm.StateSummary.Account.Initial.Balance,
	// 		sm.StateSummary.Account.Initial.Nonce,
	// 		sm.StateSummary.Account.Initial.CodeSize,
	// 		sm.StateSummary.Account.Initial.KeccakCodeHash.Hi,
	// 		sm.StateSummary.Account.Initial.KeccakCodeHash.Lo,
	// 		sm.StateSummary.BatchNumber,
	// 		sm.StateSummary.Account.Initial.Exists,
	// 		sm.StateSummary.Account.Final.Exists,
	// 		sm.StateSummary.IsInitialDeployment,
	// 		sm.StateSummary.IsStorage,
	// 	},
	// 	[]csvtraces.Option{},
	// )
	// csvtraces.FmtCsv(
	// 	files.MustOverwrite("./alex-csv/hub.csv"),
	// 	run,
	// 	[]ifaces.Column{
	// 		run.Spec.Columns.GetHandle("hub.RELATIVE_BLOCK_NUMBER"),
	// 	},
	// 	[]csvtraces.Option{},
	// )

	// csvtraces.FmtCsv(
	// 	files.MustOverwrite("./alex-csv/scparith.csv"),
	// 	run,
	// 	[]ifaces.Column{
	// 		run.Spec.Columns.GetHandle("HUB_scp_PROVER_SIDE_ADDRESS_IDENTIFIER"),
	// 		run.Spec.Columns.GetHandle("hub.scp_ADDRESS_HI"),
	// 		run.Spec.Columns.GetHandle("hub.scp_ADDRESS_LO"),
	// 		run.Spec.Columns.GetHandle("hub.scp_STORAGE_KEY_HI"),
	// 		run.Spec.Columns.GetHandle("hub.scp_STORAGE_KEY_LO"),
	// 		run.Spec.Columns.GetHandle("hub.scp_VALUE_CURR_HI"),
	// 		run.Spec.Columns.GetHandle("hub.scp_VALUE_CURR_LO"),
	// 		run.Spec.Columns.GetHandle("hub.scp_VALUE_NEXT_HI"),
	// 		run.Spec.Columns.GetHandle("hub.scp_VALUE_NEXT_LO"),
	// 		run.Spec.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER"),
	// 		run.Spec.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER"),
	// 		run.Spec.Columns.GetHandle("hub.scp_REL_BLK_NUM"),
	// 		run.Spec.Columns.GetHandle("hub.scp_PEEK_AT_STORAGE"),
	// 		run.Spec.Columns.GetHandle("hub.scp_FIRST_IN_CNF"),
	// 		run.Spec.Columns.GetHandle("hub.scp_FINAL_IN_CNF"),
	// 		run.Spec.Columns.GetHandle("hub.scp_FIRST_IN_BLK"),
	// 		run.Spec.Columns.GetHandle("hub.scp_FINAL_IN_BLK"),
	// 		run.Spec.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK"),
	// 		run.Spec.Columns.GetHandle("hub.scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK"),
	// 		run.Spec.Columns.GetHandle("hub.scp_EXISTS_FIRST_IN_BLOCK"),
	// 		run.Spec.Columns.GetHandle("hub.scp_EXISTS_FINAL_IN_BLOCK"),
	// 		//run.Spec.Columns.GetHandle("hub.scp_TX_EXEC"),
	// 	},
	// 	[]csvtraces.Option{},
	// )

	// csvtraces.FmtCsv(
	// 	files.MustOverwrite("./alex-csv/scpss.csv"),
	// 	run,
	// 	[]ifaces.Column{
	// 		sm.StateSummary.Account.Address,
	// 		sm.StateSummary.Storage.Key.Hi,
	// 		sm.StateSummary.Storage.Key.Lo,
	// 		sm.StateSummary.Storage.OldValue.Hi,
	// 		sm.StateSummary.Storage.OldValue.Lo,
	// 		sm.StateSummary.Storage.NewValue.Hi,
	// 		sm.StateSummary.Storage.NewValue.Lo,
	// 		sm.StateSummary.BatchNumber,
	// 		sm.StateSummary.IsFinalDeployment,
	// 		sm.StateSummary.Account.Final.Exists,
	// 		sm.StateSummary.IsStorage,
	// 	},
	// 	[]csvtraces.Option{},
	// )
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

func printAllShomeiTraces(shomeiTraces *[][]statemanager.DecodedTrace) {
	// AddressAndKey is a struct used as a key in order to identify skippable traces
	// in our maps
	type AddressAndKey struct {
		address    types.Bytes32
		storageKey types.Bytes32
	}
	file, err := os.OpenFile("shomeifull.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	// iterate over all the Shomei blocks
	for blockNo, vec := range *shomeiTraces {
		batchNumber := blockNo + 1
		for _, trace := range vec {
			curAddress, err := trace.GetRelatedAccount()

			if err != nil {
				panic(err)
			}

			accountAddress := curAddress
			switch t := trace.Underlying.(type) {
			case statemanager.ReadZeroTraceST:
				// BEGIN LOGGING
				// Open the file in append mode, create it if it doesn't exist
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("READZEROST") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " STORAGE KEY "+t.Key.Hex()+" %d", batchNumber) +
						fmt.Sprintln("IS SKIPPED ", trace.IsSkipped),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING

			case statemanager.ReadNonZeroTraceST:
				// BEGIN LOGGING
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("READNONZEROST") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " STORAGE KEY "+t.Key.Hex()+" %d"+" STORAGE VALUE "+t.Value.Hex(), batchNumber) +
						fmt.Sprintln("IS SKIPPED ", trace.IsSkipped),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING
			case statemanager.InsertionTraceST:
				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("INSERTST") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " STORAGE KEY "+t.Key.Hex()+" %d"+" STORAGE VALUE "+t.Val.Hex(), batchNumber) +
						fmt.Sprintln("IS SKIPPED ", trace.IsSkipped),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
			//  END LOGGING
			case statemanager.UpdateTraceST:
				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("UPDATEST") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " STORAGE KEY "+t.Key.Hex()+" %d"+" STORAGE VALUE + "+t.OldValue.Hex()+" "+t.NewValue.Hex(), batchNumber) +
						fmt.Sprintln("IS SKIPPED ", trace.IsSkipped),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
			// END LOGGING

			case statemanager.DeletionTraceST:
				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("DELETEST") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " STORAGE KEY "+t.Key.Hex()+" %d"+" STORAGE VALUE + "+t.DeletedValue.Hex(), batchNumber) +
						fmt.Sprintln("IS SKIPPED ", trace.IsSkipped),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
			// END LOGGING
			case statemanager.ReadZeroTraceWS:
				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("READZEROWS") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " %d", batchNumber),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING
			case statemanager.ReadNonZeroTraceWS:

				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}

				// Write the text to the file
				if _, err := file.WriteString(
					fmt.Sprintln("READNONZEROWS") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " %d", batchNumber),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING
			case statemanager.InsertionTraceWS:

				// BEGIN LOGGING
				// Write the text to the file
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				//x := *(&field.Element{}).SetBytes(accountAddress[:])
				if _, err := file.WriteString(
					fmt.Sprintln("INSERTWS") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " %d", batchNumber),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING

			case statemanager.UpdateTraceWS:
				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				//x := *(&field.Element{}).SetBytes(accountAddress[:])
				if _, err := file.WriteString(
					fmt.Sprintln("UPDATEWS") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " %d", batchNumber),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING
			case statemanager.DeletionTraceWS:

				// BEGIN LOGGING
				if err != nil {
					fmt.Println("Error opening file:", err)
				}
				//x := *(&field.Element{}).SetBytes(accountAddress[:])
				if _, err := file.WriteString(
					fmt.Sprintln("DELETEWS") +
						fmt.Sprintln("ADDRESS ", accountAddress.Hex(), " %d", batchNumber),
				); err != nil {
					fmt.Println("Error writing to file:", err)
				}
				// END LOGGING
			default:
				panic("unknown trace type")

			}

		}
	}
	file.Close()
}

package logs

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

// test the fetcher of message data from L2L1/RollingHash logs
func TestLogsDataFetcher(t *testing.T) {

	var testGenVect = [...]func() ([]LogInfo, types.EthAddress, string){
		GenerateSimpleL2L1Test,
		GenerateSimpleRollingTest,
		GenerateTestWithoutRelevantLogs,
		GenerateLargeTest,
	}

	for i, GenerateTest := range testGenVect {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {
			testLogs, bridgeAddress, testName := GenerateTest()
			t.Logf("Test case explainer: %v", testName)
			// test log parsing
			for _, logInfo := range testLogs {
				if logInfo.LogType == LOG4 {
					log := logInfo.ConvertToL2L1Log()
					bridge.ParseL2L1Log(log)
				}
			}

			// compute the cumulative size of the LogColumns we will compute next
			fullSize := ComputeSize(testLogs[:])
			colSize := utils.NextPowerOfTwo(fullSize)

			// LogColumns and selectors on which we will run our tests
			var (
				logCols                                            LogColumns
				selectors                                          Selectors
				fetchedL2L1, fetchedRollingMsg, fetchedRollingHash ExtractedData
				hasherL2l1                                         LogHasher
				rollingSelector                                    *RollingSelector
			)

			define := func(b *wizard.Builder) {
				// define mock log columns
				logCols = NewLogColumns(b.CompiledIOP, colSize, "MOCK")
				// initialize extracted data to be fetched from logs
				fetchedL2L1 = NewExtractedData(b.CompiledIOP, colSize, "L2L1LOGS")
				fetchedRollingMsg = NewExtractedData(b.CompiledIOP, colSize, "ROLLING_MSG")
				fetchedRollingHash = NewExtractedData(b.CompiledIOP, colSize, "ROLLING_HASH")
				// initialize selectors
				selectors = NewSelectorColumns(b.CompiledIOP, logCols)
				// initialize hashers
				hasherL2l1 = NewLogHasher(b.CompiledIOP, colSize, "L2L1LOGS")
				// initialize rolling selector
				rollingSelector = NewRollingSelector(b.CompiledIOP, "ROLLING_SEL", fetchedRollingHash.Hi.Size(), fetchedRollingMsg.Hi.Size())
				// define extracted data from logs and associated filters
				DefineExtractedData(b.CompiledIOP, logCols, selectors, fetchedL2L1, L2L1)
				DefineExtractedData(b.CompiledIOP, logCols, selectors, fetchedRollingMsg, RollingMsgNo)
				DefineExtractedData(b.CompiledIOP, logCols, selectors, fetchedRollingHash, RollingHash)
				// define the L2L1 hasher
				DefineHasher(b.CompiledIOP, hasherL2l1, "L2L1LOGS", fetchedL2L1)
				// define the Rolling selector
				DefineRollingSelector(b.CompiledIOP, rollingSelector, "ROLLING_SEL", fetchedRollingHash, fetchedRollingMsg)
			}

			prove := func(run *wizard.ProverRuntime) {
				// assign the mock log columns
				LogColumnsAssign(run, &logCols, testLogs[:])
				// assign the selectors
				selectors.Assign(run, common.Address(bridgeAddress))
				// assign the data extracted from logs
				AssignExtractedData(run, logCols, selectors, fetchedL2L1, L2L1)
				AssignExtractedData(run, logCols, selectors, fetchedRollingMsg, RollingMsgNo)
				AssignExtractedData(run, logCols, selectors, fetchedRollingHash, RollingHash)
				// assign the L2L1 hasher
				AssignHasher(run, hasherL2l1, fetchedL2L1)
				// assign the Rolling selector
				AssignRollingSelector(run, rollingSelector, fetchedRollingHash, fetchedRollingMsg)
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

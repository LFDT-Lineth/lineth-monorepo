//go:build !fuzzlight

package ecpair

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

func TestPairingDataCircuit(t *testing.T) {
	for _, tc := range pairingDataTestCases {
		testModule(t, tc, true, false, true, false)
	}
}

func TestMembershipCircuit(t *testing.T) {
	for _, tc := range membershipTestCases {
		testModule(t, tc, false, true, false, true)
	}
}

func TestGeneratedData(t *testing.T) {
	// in order to run the test, you need to have generated the different test
	// cases. Run the test [TestGenerateECPairTestCases] at
	// prover/zkevm/prover/ecpair/testdata/testdata_generator_test.go to
	// generate all inputs.
	t.Skip("long test, run manually when needed")
	var generatedData []pairingDataTestCase
	filepath.Walk("testdata/generated", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "input.csv") {
			tc := pairingDataTestCase{
				InputFName:       path,
				NbMillerLoops:    5,
				NbFinalExps:      1,
				NbSubgroupChecks: 5,
			}
			generatedData = append(generatedData, tc)
		}
		return nil
	})
	parallel.Execute(len(generatedData), func(start, end int) {
		for i := start; i < end; i++ {
			testModule(t, generatedData[i], true, true, true, true)
		}
	})
}

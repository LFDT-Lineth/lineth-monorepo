package zkcdriver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/zkc/pkg/util/file"
	"github.com/sirupsen/logrus"
)

const (
	acceptExtension = ".accepts"
	zkcExtension    = ".zkc"
	// testdataGlob is the glob pattern used to find all the testdata files for
	// the synced integration tests. Currently we only match unit tests, but we
	// can extend this to include other types of tests (mixed/bench) in the
	// future.
	testdataGlob = "testdata/synced/unit/*.zkc"
	// known failing tests
	knownFailuresFiles = "testdata/known_failures.json"
)

func TestZkcIntegrationTestSynced(t *testing.T) {
	// logs are chatty. Set the output to the test's output so that we can see
	// it in case of failures (but not in the normal case, where we want to keep
	// the test output clean)
	logrus.SetOutput(t.Output())

	// set the proverCompilePipeline to the full prover pipeline, unless we're
	// in short mode, in which case we skip the full prover pipeline.
	sysPipeline := proverCompilePipeline
	if testing.Short() {
		t.Log("short mode, skipping full prover pipeline")
		sysPipeline = func(_ *wiop.System) {}
	}

	// glob the testdata files, and run each one as a sub-test. Each test-case is
	// a line in the .accepts file, and we run each test-case as a sub-test of the
	// test-case's unit test.
	testFiles, err := filepath.Glob(testdataGlob)
	if err != nil {
		t.Fatalf("error globbing testdata: %v", err)
	}
	if len(testFiles) == 0 {
		t.Fatalf("no testdata found. Have you run `make download-zkc-testdata`?")
	}

	// read the known failures file. It is fine if we cannot read it, then we
	// consider all failures as new.
	failures := make(knownFailures)
	failuresF, err := os.Open(knownFailuresFiles)
	if err != nil {
		t.Errorf("failed to open known failures file: %v", err)
	}
	if err := json.NewDecoder(failuresF).Decode(&failures); err != nil {
		t.Errorf("failed to decode known failures file: %v", err)
	}
	failuresF.Close()
	for _, f := range testFiles {
		splitName := strings.Split(f, string(filepath.Separator))
		baseName := filepath.Join(splitName[len(splitName)-2:]...)
		t.Run(baseName, func(t *testing.T) {
			basePath := strings.TrimSuffix(f, zkcExtension)
			acceptPath := basePath + acceptExtension
			if files.CheckFilePath(acceptPath) != nil {
				fatalIfNotKnown(t, failures, baseName, -1, failReasonNoTestData, "accept file %s does not exist for test-case %s", acceptPath, f)
				return
			}
			binF, err := compileBinaryConstraints(f)
			if err != nil {
				fatalIfNotKnown(t, failures, baseName, -2, failReasonCompileZkc, "failed to compile binary constraints: %v", err)
				return
			}
			lineNr := 0
			acceptCases, ok := file.ReadInputFileAsLines(acceptPath)
			if !ok {
				fatalIfNotKnown(t, failures, baseName, -3, failReasonNoTestData, "failed to read accept file %s for test-case %s", acceptPath, f)
				return
			}
			for _, line := range acceptCases {
				// check that we're not in a comment line. I.e. we only want lines starting with `{` to be considered as test-cases.
				if !strings.HasPrefix(line, "{") {
					continue
				}
				l := lineNr
				t.Run(fmt.Sprintf("case=%d", lineNr), func(t *testing.T) {
					t.Parallel()
					zkcInput, zkcOutputs, err := parseTestCase(zkcTestCase{ZkcFilePath: f, InputStr: line}, binF)
					if err != nil {
						fatalIfNotKnown(t, failures, baseName, l, failReasonParse, "failed to parse test case: %v", err)
						return
					}
					for outputName, expectedOutput := range zkcOutputs {
						if !bytes.Equal(expectedOutput, zkcInput.Inputs[outputName]) {
							fatalIfNotKnown(t, failures, baseName, l, failReasonOutputMismatch, "output mismatch for %s: expected %x, got %x", outputName, expectedOutput, zkcInput.Inputs[outputName])
							return
						}
					}
					if err = runProveVerify(zkcInput, binF, sysPipeline); err != nil {
						fatalIfNotKnown(t, failures, baseName, l, failReasonProve, "failed to run test case: %v", err)
						return
					}
				})
				lineNr++
			}
		})
	}
}

var failuresMapMutex = new(sync.RWMutex)

func fatalIfNotKnown(t *testing.T, knownFailures knownFailures, unitName string, caseNr int, reason zkcFailReason, msg string, args ...any) {
	t.Helper()
	// if it is already in the map then we have already seen this failure, so we can just log it and continue. Otherwise, we fail the test.
	failuresMapMutex.RLock()
	if unitFailures, ok := knownFailures[unitName]; ok {
		if knownReason, ok := unitFailures[caseNr]; ok {
			if knownReason == reason {
				t.Logf(msg, args...)
				failuresMapMutex.RUnlock()
				return
			}
		}
	}
	failuresMapMutex.RUnlock()
	failuresMapMutex.Lock()
	if _, ok := knownFailures[unitName]; !ok {
		knownFailures[unitName] = make(map[int]zkcFailReason)
	}
	knownFailures[unitName][caseNr] = reason
	failuresMapMutex.Unlock()
	// unknown error. We fail the test and log the error. The user can then add
	// this failure to the known failures map if it is expected.
	//
	// the negative case number indicate input file reading and compiling. We're still not in subtest cases
	t.Fatalf(msg, args...)
}

type zkcFailReason string

const (
	failReasonNoTestData     zkcFailReason = "no test data"
	failReasonCompileZkc     zkcFailReason = "zkc compile failed"
	failReasonParse          zkcFailReason = "zkc trace/check failed"
	failReasonOutputMismatch zkcFailReason = "output mismatch"
	failReasonProve          zkcFailReason = "prover failed"
)

type knownFailures map[string]map[int]zkcFailReason

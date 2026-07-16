package zkcdriver_test

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
)

const (
	acceptExtension = ".accepts"
	zkcExtension    = ".zkc"
	// testdataGlob is the glob pattern used to find all the testdata files for
	// the synced integration tests. Currently we only match unit tests, but we
	// can extend this to include other types of tests (mixed/bench) in the
	// future.
	testdataGlob = "testdata/synced/unit/*.zkc"
)

func TestZkcIntegrationTestSynced(t *testing.T) {
	testFiles, err := filepath.Glob(testdataGlob)
	if err != nil {
		t.Fatalf("error globbing testdata: %v", err)
	}
	if len(testFiles) == 0 {
		t.Fatalf("no testdata found. Have you run `make download-zkc-testdata`?")
	}
	for _, f := range testFiles {
		splitName := strings.Split(f, string(filepath.Separator))
		baseName := filepath.Join(splitName[len(splitName)-2:]...)
		t.Run(baseName, func(t *testing.T) {
			basePath := strings.TrimSuffix(f, zkcExtension)
			acceptPath := basePath + acceptExtension
			if files.CheckFilePath(acceptPath) != nil {
				t.Fatalf("accept file %s does not exist for test-case %s", acceptPath, f)
			}
			binF, err := compileBinaryConstraints(f)
			if err != nil {
				t.Fatalf("failed to compile binary constraints: %v", err)
			}
			inputF := files.MustRead(acceptPath)
			defer inputF.Close()
			inputFBuf := bufio.NewScanner(inputF)
			lineNr := 0
			for inputFBuf.Scan() {
				line := inputFBuf.Text()
				// check that we're not in a comment line. I.e. we only want lines starting with `{` to be considered as test-cases.
				if !strings.HasPrefix(line, "{") {
					continue
				}
				t.Run(fmt.Sprintf("case=%d", lineNr), func(t *testing.T) {
					sys, zkcInput, zkcOutputs, err := parseTestCase(zkcTestCase{ZkcFilePath: f, InputStr: line}, binF)
					if err != nil {
						t.Fatalf("failed to parse test case: %v", err)
					}
					if err = runProveVerify(sys, zkcInput, binF); err != nil {
						t.Errorf("failed to run test case: %v", err)
					}
					for outputName, expectedOutput := range zkcOutputs {
						if !bytes.Equal(expectedOutput, zkcInput.Inputs[outputName]) {
							t.Errorf("output mismatch for %s: expected %x, got %x", outputName, expectedOutput, zkcInput.Inputs[outputName])
						}
					}
				})
				lineNr++
			}

		})

	}
}

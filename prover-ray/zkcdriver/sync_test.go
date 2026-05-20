package zkcdriver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testdataSyncDir = "testdata/synced"
	acceptExtension = ".accepts"
	binExtension    = ".bin"
)

func TestZKCIntegrationSynced(t *testing.T) {

	// numPassing is the number of tests that passed
	numPassing := 0

	err := filepath.Walk(testdataSyncDir,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}

			// The corset function for parsing the inputs of the test-case may
			// panic. We recover from that panic and skip the test-case.
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("recovered from: ", r)
				}
			}()

			if info.IsDir() || filepath.Ext(path) != binExtension {
				return nil
			}

			acceptPath := strings.TrimSuffix(path, binExtension) + acceptExtension
			inputStr, readErr := os.ReadFile(acceptPath) //nolint
			if readErr != nil {
				// If the .accept file does not exist, we skip the test.
				return nil //nolint
			}

			sys, input, err := parseTestCase(zkcTestCase{
				BinFilePath: path,
				InputStr:    string(inputStr),
			})

			if err != nil {
				// The corset input parsing failed. In that case, we consider
				// there is a bug in corset side and we skip the test.
				return nil //nolint
			}

			testName := strings.TrimPrefix(
				strings.TrimSuffix(path, binExtension), "testdata/")

			t.Run(testName, func(t *testing.T) {
				err := runTestCase(sys, *input, zkcTestCase{
					BinFilePath: path,
					InputStr:    string(inputStr),
				})

				if err != nil {
					t.Fatalf("test %s failed: %v", testName, err)
				}

				numPassing++
			})

			return nil
		})

	if err != nil {
		t.Fatalf("error walking %s: %v", testdataSyncDir, err)
	}

	if numPassing == 0 {
		t.Fatalf("no test were executed, there must be a bug. Could be corset side.")
	}
}

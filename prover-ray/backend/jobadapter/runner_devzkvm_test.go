package jobadapter

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// devZkVMCommitment stands in for the guest output. The runner does not read
// it; it only compares it byte for byte with the native runner's output.
var devZkVMCommitment = []byte("guest output")

// devZkVMRunnerJSON is the native runner's --json output for the single-block
// fixture: the start block number and the public inputs.
const devZkVMRunnerJSON = `{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0xee93f90395f9384466a618018575f933dca8fa630afbcc6c048b2381a7d72af9","endBlockHash":"0xb6107b07672ca66bd702a53c5105925002037e593f46c8cecba5af3425496d64","endBlockNumber":1,"endBlockTimestamp":12,"l2L1Messages":[],"endL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endL1L2BridgeRollingHashMessageNumber":0,"dynamicChainConfigHash":"0x4c87eca77cb88c24f2de035ddac21297991d5a1946b9e7d6a2b25375037715fa","parentFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentFtxNumber":0,"endFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endProcessedFtxNumber":0,"filteredAddresses":[],"txFromsHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","blockCount":1,"l2MessagingBlocksOffsets":[]}}`

// fakeRunnerJSONSSZ builds a native runner stub that answers --json with the
// given response shape and --ssz with the given raw wire bytes, so dev-zkvm can
// exercise both runner calls without the real binary.
func fakeRunnerJSONSSZ(t *testing.T, jsonOut string, sszBytes []byte) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell runner is POSIX-only")
	}
	var octal strings.Builder
	for _, b := range sszBytes {
		fmt.Fprintf(&octal, "\\%03o", b)
	}
	script := "#!/bin/sh\n" +
		"if [ \"$2\" = \"--ssz\" ]; then\n" +
		"  printf '" + octal.String() + "'\n" +
		"else\n" +
		"  cat <<'JSON'\n" + jsonOut + "\nJSON\n" +
		"fi\n"
	bin := filepath.Join(t.TempDir(), "fake-runner")
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700)) //nolint:gosec // G306: test stub must be executable
	return bin
}

func devZkVMProver(guestOutput []byte) *mockProver {
	return &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{
			JobID:      job.ID,
			Status:     backend.ResultStatusOK,
			ProofBytes: guestOutput, // dev-zkvm's proof output is the guest commitment
		}
	}}
}

// dev-zkvm fills the response from the native runner and passes when the guest's
// ZkC commitment matches the native runner's --ssz commitment.
func TestRunner_DevZkVM_CrossCheckPasses(t *testing.T) {
	commitment := devZkVMCommitment
	mock := devZkVMProver(commitment)
	runner, err := NewRunner(mock, testProverVersion,
		WithMode(backend.ProverModeDevZkVM),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, commitment)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-1", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1, "dev-zkvm must run the guest under ZkC Execute")

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-dev-zkvm", resp.ProverVersion)
	assert.Equal(t, "0xee93f90395f9384466a618018575f933dca8fa630afbcc6c048b2381a7d72af9", resp.PublicInputs.ParentBlockHash)
	assert.Equal(t, guestProgramID, resp.ProgramVk)
	assert.Equal(t, "0x"+hex.EncodeToString(backend.DevMarkerProof(backend.ProverModeDevZkVM)), resp.ProofHex,
		"proof is a placeholder marker; the commitment is an internal cross-check artifact")
}

// A guest commitment that disagrees with the native runner is refused.
func TestRunner_DevZkVM_CrossCheckFails(t *testing.T) {
	commitment := devZkVMCommitment
	wrong := append([]byte(nil), commitment...)
	wrong[2] ^= 0xff
	mock := devZkVMProver(wrong)
	runner, err := NewRunner(mock, testProverVersion,
		WithMode(backend.ProverModeDevZkVM),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, commitment)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-2", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "cross-check failed")
}

func TestRunner_DevZkVM_RequiresBinary(t *testing.T) {
	runner, err := NewRunner(&mockProver{}, testProverVersion, WithMode(backend.ProverModeDevZkVM))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-3", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "native runner binary")
}

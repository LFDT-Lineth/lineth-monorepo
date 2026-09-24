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

// devZkVMCommitment is the guest's 0x0003 wire output for the single-block
// fixture (from the Stage 0 spike): schema id 0x0003 + keccak256(SSZ(pi)).
var devZkVMCommitment = decodeHexOrPanic("0003bd5489092594781eb7a96aee925e008c3d581f7aa8da36a7712bd982a45edf5e")

// devZkVMRunnerJSON is the native runner's --json output for the single-block
// fixture: the 16 real public inputs plus the revealed preimage arrays.
const devZkVMRunnerJSON = `{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972","endBlockHash":"0x74560e963ebefb718fb712ec6a9b8e37672c85d8e6120898e6018734bc72f842","endBlockNumber":1,"endBlockTimestamp":1000,"l2L1MessagesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","parentL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentL1L2BridgeRollingHashMessageNumber":0,"endL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endL1L2BridgeRollingHashMessageNumber":0,"dynamicChainConfigHash":"0x0bf5347e375f112c245569da023a5abf696fa9e2af3dff33b0a085ab2757dd28","parentFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentFtxNumber":0,"endFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endProcessedFtxNumber":0,"filteredAddressesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","txFromsHash":"0x463172c05a4d389f66ba68c6bc3beab68b6d5d6ce143bb6aa536b9ac49469f1b"},"l2L1Messages":[],"txFroms":["0xd66f224cbd2fabb21b17bb53c56118f32f62ebd6"],"filteredAddresses":[]}`

func decodeHexOrPanic(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

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
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700))
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
	mock := devZkVMProver(devZkVMCommitment)
	runner, err := NewRunner(mock, testProverVersion,
		WithMode(backend.ProverModeDevZkVM),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, devZkVMCommitment)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-1", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1, "dev-zkvm must run the guest under ZkC Execute")

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-dev-zkvm", resp.ProverVersion)
	assert.Equal(t, "0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972", resp.PublicInputs.ParentBlockHash)
	assert.Equal(t, programVk, resp.ProgramVk)
	assert.Equal(t, "0x"+hex.EncodeToString(backend.DevMarkerProof(backend.ProverModeDevZkVM)), resp.ProofHex,
		"proof is a placeholder marker; the commitment is an internal cross-check artifact")
}

// A guest commitment that disagrees with the native runner is refused.
func TestRunner_DevZkVM_CrossCheckFails(t *testing.T) {
	wrong := append([]byte(nil), devZkVMCommitment...)
	wrong[2] ^= 0xff
	mock := devZkVMProver(wrong)
	runner, err := NewRunner(mock, testProverVersion,
		WithMode(backend.ProverModeDevZkVM),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, devZkVMCommitment)))
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

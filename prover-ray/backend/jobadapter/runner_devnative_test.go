package jobadapter

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const devNativeRunnerJSON = `{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972","endBlockHash":"0x74560e963ebefb718fb712ec6a9b8e37672c85d8e6120898e6018734bc72f842","endBlockNumber":1,"endBlockTimestamp":1000,"l2L1MessagesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","parentL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentL1L2BridgeRollingHashMessageNumber":0,"endL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endL1L2BridgeRollingHashMessageNumber":0,"dynamicChainConfigHash":"0x0bf5347e375f112c245569da023a5abf696fa9e2af3dff33b0a085ab2757dd28","parentFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentFtxNumber":0,"endFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endProcessedFtxNumber":0,"filteredAddressesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","txFromsHash":"0x463172c05a4d389f66ba68c6bc3beab68b6d5d6ce143bb6aa536b9ac49469f1b"},"l2L1Messages":[],"txFroms":["0xd66f224cbd2fabb21b17bb53c56118f32f62ebd6"],"filteredAddresses":[]}`

func fakeRunner(t *testing.T, output string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell runner is POSIX-only")
	}
	bin := filepath.Join(t.TempDir(), "fake-runner")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\ncat <<'JSON'\n"+output+"\nJSON\n"), 0o700))
	return bin
}

// dev-native builds the extended input, runs the (fake) native runner, and fills
// the response with its real public inputs and arrays, without calling the ZkC prover.
func TestRunner_DevNative_UsesRunnerOutput(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion,
		WithMode(backend.ProverModeDevNative), WithNativeRunnerBin(fakeRunner(t, devNativeRunnerJSON)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dn-1", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	assert.Empty(t, mock.jobs, "dev-native must not call the ZkC prover")

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-dev-native", resp.ProverVersion)
	assert.Equal(t, uint64(1), resp.StartBlockNumber)
	assert.Equal(t, "0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972", resp.PublicInputs.ParentBlockHash)
	assert.Equal(t, programVk, resp.ProgramVk)
	require.Len(t, resp.TxFroms, 1)
	assert.Equal(t, "0xd66f224cbd2fabb21b17bb53c56118f32f62ebd6", resp.TxFroms[0])
	assert.Empty(t, resp.L2L1Messages)
	assert.Contains(t, resp.ProofHex, "0x")
}

func TestRunner_DevNative_RequiresBinary(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion, WithMode(backend.ProverModeDevNative)) // no binary
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dn-2", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "native runner binary")
}

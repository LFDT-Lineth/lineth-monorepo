package nativerunner

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realOutput is a verbatim capture of `l2-execution-runner --json` on
// test/testdata/stateless_input.ssz (after the parentFtxNumber name fix).
const realOutput = `{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972","endBlockHash":"0x74560e963ebefb718fb712ec6a9b8e37672c85d8e6120898e6018734bc72f842","endBlockNumber":1,"endBlockTimestamp":1000,"l2L1MessagesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","parentL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentL1L2BridgeRollingHashMessageNumber":0,"endL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endL1L2BridgeRollingHashMessageNumber":0,"dynamicChainConfigHash":"0x0bf5347e375f112c245569da023a5abf696fa9e2af3dff33b0a085ab2757dd28","parentFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentFtxNumber":0,"endFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endProcessedFtxNumber":0,"filteredAddressesHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","txFromsHash":"0x463172c05a4d389f66ba68c6bc3beab68b6d5d6ce143bb6aa536b9ac49469f1b"},"l2L1Messages":[],"txFroms":["0xd66f224cbd2fabb21b17bb53c56118f32f62ebd6"],"filteredAddresses":[]}`

func hexTo(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	require.NoError(t, err)
	return b
}

func TestParse_RealOutput(t *testing.T) {
	got, err := Parse([]byte(realOutput))
	require.NoError(t, err)

	assert.Equal(t, uint64(1), got.StartBlockNumber)
	assert.Equal(t, uint64(1), got.PublicInputs.EndBlockNumber)
	assert.Equal(t, uint64(1000), got.PublicInputs.EndBlockTimestamp)
	assert.Equal(t, uint64(0), got.PublicInputs.ParentFtxNumber)

	assert.Equal(t, hexTo(t, "0x3e92984e1569f7296de4776ac738531c3a602df8cf2b1734a56f3adb3e111972"),
		got.PublicInputs.ParentBlockHash[:])
	assert.Equal(t, hexTo(t, "0x463172c05a4d389f66ba68c6bc3beab68b6d5d6ce143bb6aa536b9ac49469f1b"),
		got.PublicInputs.TxFromsHash[:])

	assert.Empty(t, got.L2L1Messages)
	assert.Empty(t, got.FilteredAddresses)
	require.Len(t, got.TxFroms, 1)
	assert.Equal(t, hexTo(t, "0xd66f224cbd2fabb21b17bb53c56118f32f62ebd6"), got.TxFroms[0][:])
}

func TestParse_Errors(t *testing.T) {
	_, err := Parse([]byte("not json"))
	require.Error(t, err)

	_, err = Parse([]byte(`{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0x1234"},"l2L1Messages":[],"txFroms":[],"filteredAddresses":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parentBlockHash")
}

// TestRun_FakeBinary exercises the temp-file + exec + parse pipeline with a stub
// binary that echoes a known output, so it needs neither zig nor the real runner.
func TestRun_FakeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary is POSIX-only")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-runner")
	script := "#!/bin/sh\ncat <<'JSON'\n" + realOutput + "\nJSON\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700)) //nolint:gosec // G306: test stub must be executable

	got, err := Run(context.Background(), bin, []byte{0x00, 0x02, 0xAA})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), got.StartBlockNumber)
	require.Len(t, got.TxFroms, 1)
}

func TestRun_BinaryError(t *testing.T) {
	_, err := Run(context.Background(), "/nonexistent/runner", []byte{0x00, 0x02})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running native runner")
}

// TestRun_RealBinary runs the actual native runner if its path is provided via
// L2_EXECUTION_RUNNER_BIN and a real 0x0002 input via L2_EXECUTION_RUNNER_INPUT.
// Skipped otherwise (e.g. CI without zig).
func TestRun_RealBinary(t *testing.T) {
	bin := os.Getenv("L2_EXECUTION_RUNNER_BIN")
	input := os.Getenv("L2_EXECUTION_RUNNER_INPUT")
	if bin == "" || input == "" {
		t.Skip("set L2_EXECUTION_RUNNER_BIN and L2_EXECUTION_RUNNER_INPUT to run against the real runner")
	}
	data, err := os.ReadFile(input) //nolint:gosec // G703: test-controlled path from env
	require.NoError(t, err)
	got, err := Run(context.Background(), bin, data)
	require.NoError(t, err)
	assert.NotZero(t, got.StartBlockNumber)
}

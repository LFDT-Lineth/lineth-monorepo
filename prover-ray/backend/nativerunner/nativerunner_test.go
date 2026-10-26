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
// riscv-guests/l2-execution/test/testdata/stateless_input.ssz; re-capture it when
// the runner's output changes.
const realOutput = `{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0xee93f90395f9384466a618018575f933dca8fa630afbcc6c048b2381a7d72af9","endBlockHash":"0xb6107b07672ca66bd702a53c5105925002037e593f46c8cecba5af3425496d64","endBlockNumber":1,"endBlockTimestamp":12,"l2L1Messages":[],"parentL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentL1L2BridgeRollingHashMessageNumber":0,"endL1L2BridgeRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endL1L2BridgeRollingHashMessageNumber":0,"dynamicChainConfigHash":"0x4c87eca77cb88c24f2de035ddac21297991d5a1946b9e7d6a2b25375037715fa","parentFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","parentFtxNumber":0,"endFtxRollingHash":"0x0000000000000000000000000000000000000000000000000000000000000000","endProcessedFtxNumber":0,"filteredAddresses":[],"txFromsHash":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470","blockCount":1,"l2MessagingBlocksOffsets":[]}}`

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
	assert.Equal(t, uint64(12), got.PublicInputs.EndBlockTimestamp)
	assert.Equal(t, uint64(0), got.PublicInputs.ParentFtxNumber)

	assert.Equal(t, hexTo(t, "0xee93f90395f9384466a618018575f933dca8fa630afbcc6c048b2381a7d72af9"),
		got.PublicInputs.ParentBlockHash[:])
	assert.Equal(t, hexTo(t, "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
		got.PublicInputs.TxFromsHash[:])
	assert.Equal(t, uint64(1), got.PublicInputs.BlockCount)
	assert.Equal(t, []uint16{}, got.PublicInputs.L2MessagingBlocksOffsets)

	assert.Empty(t, got.PublicInputs.L2L1Messages)
	assert.Empty(t, got.PublicInputs.FilteredAddresses)
}

func TestParse_Errors(t *testing.T) {
	_, err := Parse([]byte("not json"))
	require.Error(t, err)

	_, err = Parse([]byte(`{"startBlockNumber":1,"publicInputs":{"parentBlockHash":"0x1234"}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parentBlockHash")
}

// TestParse_RejectsUnknownField locks in strict decoding: a field outside the
// spec shape must fail parsing, not be silently ignored.
func TestParse_RejectsUnknownField(t *testing.T) {
	withExtra := strings.Replace(realOutput,
		`"endBlockTimestamp":12,`,
		`"endBlockTimestamp":12,"l2L1MessagesHash":"0x00",`, 1)
	_, err := Parse([]byte(withExtra))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "l2L1MessagesHash")
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

package nativerunner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunSSZ_FakeBinary checks the temp file and exec steps with a stub binary:
// it must be called with the input file and --ssz, and its stdout returned as
// is. It needs neither zig nor the real runner.
func TestRunSSZ_FakeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary is POSIX-only")
	}
	bin := filepath.Join(t.TempDir(), "fake-runner")
	// Echo the flag, then the input file, so the test sees both.
	script := "#!/bin/sh\nprintf '%s:' \"$2\"\ncat \"$1\"\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700)) //nolint:gosec // G306: test stub must be executable

	got, err := RunSSZ(context.Background(), bin, []byte{0x00, 0x02, 0xAA})
	require.NoError(t, err)
	assert.Equal(t, append([]byte("--ssz:"), 0x00, 0x02, 0xAA), got)
}

func TestRunSSZ_BinaryError(t *testing.T) {
	_, err := RunSSZ(context.Background(), "/nonexistent/runner", []byte{0x00, 0x02})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running native runner")
}

// TestRunSSZ_RealBinary runs the actual native runner if its path is provided
// via L2_EXECUTION_RUNNER_BIN and a real 0x0002 input via
// L2_EXECUTION_RUNNER_INPUT. Skipped otherwise (e.g. CI without zig).
func TestRunSSZ_RealBinary(t *testing.T) {
	bin := os.Getenv("L2_EXECUTION_RUNNER_BIN")
	input := os.Getenv("L2_EXECUTION_RUNNER_INPUT")
	if bin == "" || input == "" {
		t.Skip("set L2_EXECUTION_RUNNER_BIN and L2_EXECUTION_RUNNER_INPUT to run against the real runner")
	}
	data, err := os.ReadFile(input) //nolint:gosec // G703: test-controlled path from env
	require.NoError(t, err)
	got, err := RunSSZ(context.Background(), bin, data)
	require.NoError(t, err)
	// The hash, the schema id, then the public inputs.
	assert.Greater(t, len(got), 34)
}

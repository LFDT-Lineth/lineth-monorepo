package subprocess

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func script(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake is POSIX-only")
	}
	path := filepath.Join(t.TempDir(), "fake")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o700))
	return path
}

func TestProver_ExitCode(t *testing.T) {
	bin := script(t, "#!/bin/sh\nexit 3\n")
	code, err := Prover{BinPath: bin, ConfigPath: "x"}.Prove(context.Background(), "in", "out")
	require.NoError(t, err)
	assert.Equal(t, 3, code)
}

func TestProver_Success(t *testing.T) {
	bin := script(t, "#!/bin/sh\nexit 0\n")
	code, err := Prover{BinPath: bin, ConfigPath: "x"}.Prove(context.Background(), "in", "out")
	require.NoError(t, err)
	assert.Equal(t, 0, code)
}

func TestProver_CannotRun(t *testing.T) {
	_, err := Prover{BinPath: "/does/not/exist", ConfigPath: "x"}.Prove(context.Background(), "in", "out")
	require.Error(t, err)
}

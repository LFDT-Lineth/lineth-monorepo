package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestNewConfigFromFile_Valid(t *testing.T) {
	cfg, err := NewConfigFromFile(writeConfig(t, `
version = "0.0.0-riscv"
log_level = 4

[execution]
prover_mode = "dev-zkvm"
requests_root_dir = "/opt/linea/prover-ray/requests"
native_runner_bin = "/opt/linea/prover-ray/l2-execution-runner"
guest_elf = "/opt/linea/prover-ray/evm_execution_guest"
`))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0-riscv", cfg.Version)
	assert.Equal(t, 4, cfg.LogLevel)
	assert.Equal(t, "dev-zkvm", cfg.Execution.ProverMode)
	assert.Equal(t, "/opt/linea/prover-ray/requests", cfg.Execution.RequestsRootDir)
	assert.Equal(t, "/opt/linea/prover-ray/l2-execution-runner", cfg.Execution.NativeRunnerBin)
}

func TestNewConfigFromFile_UnknownKeyRejected(t *testing.T) {
	_, err := NewConfigFromFile(writeConfig(t, `
version = "t"
typo_key = "oops"

[execution]
prover_mode = "dev-mock"
requests_root_dir = "/tmp"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config")
}

func TestNewConfigFromFile_MissingRequired(t *testing.T) {
	_, err := NewConfigFromFile(writeConfig(t, `
version = "t"

[execution]
prover_mode = "dev-mock"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requests_root_dir")
}

func TestNewConfigFromFile_MissingFile(t *testing.T) {
	_, err := NewConfigFromFile("/does/not/exist.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

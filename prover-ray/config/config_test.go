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
guests_dir = "/data/guests"

[execution]
prover_mode = "dev-zkvm"
requests_root_dir = "/opt/linea/prover-ray/requests"
native_runner_bin = "/opt/linea/prover-ray/l2_execution_runner"
`))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0-riscv", cfg.Version)
	assert.Equal(t, 4, cfg.LogLevel)
	assert.Equal(t, "dev-zkvm", cfg.Execution.ProverMode)
	assert.Equal(t, "/opt/linea/prover-ray/requests", cfg.Execution.RequestsRootDir)
	assert.Equal(t, "/opt/linea/prover-ray/l2_execution_runner", cfg.Execution.NativeRunnerBin)
	assert.Equal(t, "/data/guests", cfg.GuestsDir)
}

func TestNewConfigFromFile_GuestELFRejected(t *testing.T) {
	_, err := NewConfigFromFile(writeConfig(t, `
version = "t"

[execution]
prover_mode = "dev-zkvm"
guest_elf = "/opt/linea/prover-ray/evm_execution_guest"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guest_elf", "guest_elf is replaced by guests_dir and must not be silently ignored")
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

func TestNewConfigFromFile_MissingMode(t *testing.T) {
	_, err := NewConfigFromFile(writeConfig(t, `
version = "t"

[execution]
requests_root_dir = "/tmp"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution.prover_mode")
}

func TestNewConfigFromFile_NoQueueFolderForProve(t *testing.T) {
	cfg, err := NewConfigFromFile(writeConfig(t, `
version = "t"

[execution]
prover_mode = "dev-mock"

[rollup]
prover_mode = "dev-mock"
`))
	require.NoError(t, err)
	assert.True(t, cfg.Rollup.Configured())
}

func TestNewConfigFromFile_PipelineWithoutMode(t *testing.T) {
	_, err := NewConfigFromFile(writeConfig(t, `
version = "t"

[execution]
prover_mode = "dev-mock"

[rollup]
requests_root_dir = "/tmp/rollup"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollup.prover_mode")
}

func TestValidateQueues(t *testing.T) {
	const devMock = "dev-mock"
	tests := map[string]struct {
		cfg     Config
		wantErr string
	}{
		"execution only": {
			cfg: Config{Execution: Execution{ProverMode: devMock, RequestsRootDir: "/q"}},
		},
		"missing execution folder": {
			cfg:     Config{Execution: Execution{ProverMode: devMock}},
			wantErr: "execution.requests_root_dir",
		},
		"configured rollup without folder": {
			cfg: Config{
				Execution: Execution{ProverMode: devMock, RequestsRootDir: "/q"},
				Rollup:    Pipeline{ProverMode: devMock},
			},
			wantErr: "rollup.requests_root_dir",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.ValidateQueues()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewConfigFromFile_MissingFile(t *testing.T) {
	_, err := NewConfigFromFile("/does/not/exist.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

// guestsMount is where every deployment mounts the guests folder, like
// /opt/linea/prover/prover-assets for the legacy prover.
const guestsMount = "/opt/linea/prover-ray/guests"

// shippedConfigs lists every prover-ray config shipped in the repository.
func shippedConfigs(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range []string{
		"*.toml",
		"../../docker/config/prover/riscv/prover-ray-config.*.toml",
		"../../docker/config/prover-ray/prover-ray-config.toml",
	} {
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		require.NotEmpty(t, matches, "pattern %s matches no config", pattern)
		paths = append(paths, matches...)
	}
	return paths
}

// TestShippedConfigs loads every config shipped in the repository, so a config
// key change cannot leave one of them behind. A config that runs a guest must
// read it from the guests mount.
func TestShippedConfigs(t *testing.T) {
	for _, path := range shippedConfigs(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			cfg, err := NewConfigFromFile(path)
			require.NoError(t, err, "shipped config %s must load", path)
			if cfg.GuestsDir != "" {
				assert.Equal(t, guestsMount, cfg.GuestsDir, "every deployment mounts the guests folder at the same path")
			}
		})
	}
}

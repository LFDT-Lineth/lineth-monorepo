// Package config loads the prover-ray TOML configuration: viper reads the file
// and UnmarshalExact rejects any unknown key.
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config is the top-level prover-ray configuration: top-level metadata plus a
// section per proof pipeline. [execution] is required; [rollup] and
// [aggregation] are optional (watched only when configured).
type Config struct {
	// Version is echoed to the coordinator as proverVersion.
	Version string `mapstructure:"version"`
	// LogLevel is a logrus level (0=panic … 6=trace); 4 is info.
	LogLevel int `mapstructure:"log_level"`
	// Execution configures the L2-execution pipeline.
	Execution Execution `mapstructure:"execution"`
	// Rollup configures the rollup (compression) pipeline.
	Rollup Pipeline `mapstructure:"rollup"`
	// Aggregation configures the aggregation pipeline.
	Aggregation Pipeline `mapstructure:"aggregation"`
}

// Execution holds the L2-execution pipeline settings.
type Execution struct {
	// ProverMode is one of backend.ProverMode (dev-mock, dev-zkvm, partial, full).
	ProverMode string `mapstructure:"prover_mode"`
	// RequestsRootDir contains the requests/, responses/, requests-done/ queue.
	RequestsRootDir string `mapstructure:"requests_root_dir"`
	// NativeRunnerBin is the native l2-execution-runner binary, required by dev-zkvm.
	NativeRunnerBin string `mapstructure:"native_runner_bin"`
	// GuestELF is the l2-execution guest ELF, required by dev-zkvm.
	GuestELF string `mapstructure:"guest_elf"`
}

// Pipeline holds the rollup and aggregation pipeline settings (no guest, so no
// native runner or ELF).
type Pipeline struct {
	ProverMode      string `mapstructure:"prover_mode"`
	RequestsRootDir string `mapstructure:"requests_root_dir"`
}

// Configured reports whether the pipeline is set (either field is set). validate
// then requires both fields when one is present.
func (p Pipeline) Configured() bool {
	return p.ProverMode != "" || p.RequestsRootDir != ""
}

// NewConfigFromFile reads and validates a TOML config. It uses viper with
// UnmarshalExact, so an unknown key is an error rather than being silently
// ignored.
func NewConfigFromFile(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}
	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Version == "" {
		return fmt.Errorf("version must be set")
	}
	if c.Execution.ProverMode == "" {
		return fmt.Errorf("execution.prover_mode must be set")
	}
	if c.Execution.RequestsRootDir == "" {
		return fmt.Errorf("execution.requests_root_dir must be set")
	}
	for name, p := range map[string]Pipeline{"rollup": c.Rollup, "aggregation": c.Aggregation} {
		if p.Configured() && (p.ProverMode == "" || p.RequestsRootDir == "") {
			return fmt.Errorf("%s: prover_mode and requests_root_dir must be set together", name)
		}
	}
	return nil
}

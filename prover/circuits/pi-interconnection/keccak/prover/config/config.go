package config

import (
	"time"
)

type PerformanceMonitor struct {
	Active         bool          `mapstructure:"active"`
	SampleDuration time.Duration `mapstructure:"sample_duration"`
	ProfileDir     string        `mapstructure:"profile_dir"`
	Profile        string        `mapstructure:"profile" validate:"oneof=prover-steps prover-rounds all"`
}

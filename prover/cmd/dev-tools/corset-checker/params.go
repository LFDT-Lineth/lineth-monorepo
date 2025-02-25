package main

import (
	"flag"
	"fmt"

	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/linea-monorepo/prover/config"
)

var (
	configFPathCLI       string
	traceFPathCLI        string
	optimisationLevelCLI uint
)

func init() {
	flag.StringVar(&configFPathCLI, "config", "", "path to the config file. Only the trace limits are read")
	flag.StringVar(&traceFPathCLI, "trace-file", "", "path to the `.lt` trace file")
	flag.UintVar(&optimisationLevelCLI, "opt", 1, "set go-corset optimisation level to apply")
	flag.Parse()
}

func getParamsFromCLI() (cfg *config.Config, optConfig *mir.OptimisationConfig, traceFPath string, err error) {

	flag.Parse()

	if len(configFPathCLI) == 0 {
		return nil, nil, "", fmt.Errorf("could not find the config path, got %++v", configFPathCLI)
	}

	if len(traceFPathCLI) == 0 {
		return nil, nil, "", fmt.Errorf("could not find the trace file path, got %++v", traceFPathCLI)
	}

	if cfg, err = config.NewConfigFromFile(configFPathCLI); err != nil {
		return nil, nil, "", fmt.Errorf("could not parse the config: %w", err)
	}
	// Sanity check specified optimisation level makes sense.
	if optimisationLevelCLI >= uint(len(mir.OPTIMISATION_LEVELS)) {
		return nil, nil, "", fmt.Errorf("invalid optimisation level: %d", optimisationLevelCLI)
	}
	// Set optimisation config
	optConfig = &mir.OPTIMISATION_LEVELS[optimisationLevelCLI]
	// Done
	return cfg, optConfig, traceFPathCLI, nil
}

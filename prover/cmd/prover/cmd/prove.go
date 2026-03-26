package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/backend/dataavailability"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/execution/limitless"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils/signal"
)

type ProverArgs struct {
	Input      string
	Output     string
	Large      bool
	ConfigFile string
}

// Prove orchestrates the proving process based on the job type
func Prove(args ProverArgs) error {

	// This allows the user to dump stacktraces by sending a SIGUSR1 to the
	// current process.
	signal.RegisterStackTraceDumpHandler()

	// TODO @gbotrel with a specific flag, we could compile the circuit and compare with the checksum of the
	// asset we deserialize, to make sure we are using the circuit associated with the compiled
	// binary and the setup.
	const cmdName = "prove"

	// Read config
	cfg, err := config.NewConfigFromFile(args.ConfigFile)
	if err != nil {
		return fmt.Errorf("%s failed to read config file at %v: %w", cmdName, args.ConfigFile, err)
	}

	// Determine job type from input file name
	var (
		jobExecution        = strings.Contains(args.Input, "getZkProof")
		jobDataAvailability = strings.Contains(args.Input, "getZkBlobCompressionProof")
		jobAggregation      = strings.Contains(args.Input, "getZkAggregatedProof")
	)

	// Handle job type
	switch {
	case jobExecution:
		return handleExecutionJob(cfg, args)
	case jobDataAvailability:
		return handleDataAvailabilityJob(cfg, args)
	case jobAggregation:
		return handleAggregationJob(cfg, args)
	default:
		return errors.New("unknown job type")
	}
}

// handleExecutionJob processes an execution job
func handleExecutionJob(cfg *config.Config, args ProverArgs) error {
	req := &execution.Request{}
	if err := readRequest(args.Input, req); err != nil {
		return fmt.Errorf("could not read the input file (%v): %w", args.Input, err)
	}

	var resp *execution.Response
	var err error

	if cfg.Execution.ProverMode == config.ProverModeLimitless {
		// Limitless execution mode
		resp, err = limitless.Prove(cfg, req)
		if err != nil {
			return fmt.Errorf("could not prove the execution in limitless mode: %w", err)
		}
	} else {
		// Standard execution mode
		large := args.Large || (strings.Contains(args.Input, "large") && cfg.Execution.CanRunFullLarge)
		resp, err = execution.Prove(cfg, req, large)
		if err != nil {
			return fmt.Errorf("could not prove the execution: %w", err)
		}
	}

	return writeResponse(args.Output, resp)
}

// handleDataAvailabilityJob processes a data availability proof job
func handleDataAvailabilityJob(cfg *config.Config, args ProverArgs) error {
	req := &dataavailability.Request{}
	if err := readRequest(args.Input, req); err != nil {
		return fmt.Errorf("could not read the input file (%v): %w", args.Input, err)
	}

	resp, err := dataavailability.Prove(cfg, req)
	if err != nil {
		return fmt.Errorf("could not prove the data availability: %w", err)
	}

	return writeResponse(args.Output, resp)
}

// handleAggregationJob processes an aggregation job
func handleAggregationJob(cfg *config.Config, args ProverArgs) error {
	req := &aggregation.Request{}
	if err := readRequest(args.Input, req); err != nil {
		return fmt.Errorf("could not read the input file (%v): %w", args.Input, err)
	}

	resp, err := aggregation.Prove(cfg, req)
	if err != nil {
		return fmt.Errorf("could not prove the aggregation: %w", err)
	}

	return writeResponse(args.Output, resp)
}

// readRequest reads and decodes a request from a file
func readRequest(path string, into any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(into); err != nil {
		return fmt.Errorf("could not decode input file: %w", err)
	}

	return nil
}

// writeResponse encodes and writes a response to a file
func writeResponse(path string, from any) error {
	f := files.MustOverwrite(path)
	defer f.Close()

	if err := json.NewEncoder(f).Encode(from); err != nil {
		return fmt.Errorf("could not encode output file: %w", err)
	}

	return nil
}

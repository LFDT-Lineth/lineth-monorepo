// Command prover runs either the queue adapter or a one-shot prover:
//
//	prover --config <toml>                                 watch the queues, prove each request
//	prover prove --config <toml> --in <req> --out <resp>   prove one request file, exit with a code
//
// Config is a TOML file (--config or CONFIG_FILE). The one-shot form also serves
// to prove a single request by hand.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/filesystem"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/config"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		logrus.Fatalf("prover: %v", err)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "prove" {
		return runProve(args[1:])
	}
	return runAdapter(args)
}

// loadConfig loads the config (--config or CONFIG_FILE) and sets the log level.
// It returns the resolved path, which the adapter passes to the provers it spawns.
func loadConfig(configPath string) (*config.Config, string, error) {
	path := configPath
	if path == "" {
		path = os.Getenv("CONFIG_FILE")
	}
	if path == "" {
		return nil, "", fmt.Errorf("--config (or CONFIG_FILE) is required")
	}
	cfg, err := config.NewConfigFromFile(path)
	if err != nil {
		return nil, "", err
	}
	if cfg.LogLevel >= int(logrus.PanicLevel) && cfg.LogLevel <= int(logrus.TraceLevel) {
		logrus.SetLevel(logrus.Level(cfg.LogLevel))
	}
	return cfg, path, nil
}

// pipeline is one proof type's resolved config. priority is the queue tiebreaker
// (execution 0, rollup 1, aggregation 2).
type pipeline struct {
	mode            backend.ProverMode
	requestsRootDir string
	nativeRunnerBin string
	guestELF        string
	priority        int
}

// proofTypes lists the proof types in priority order (lower priority value first).
var proofTypes = []backend.ProofType{
	backend.ProofTypeL2Execution,
	backend.ProofTypeRollup,
	backend.ProofTypeRollupAggregation,
}

// pipelineFor resolves the config for a proof type. ok is false when the pipeline
// is not configured (rollup and aggregation are optional).
func pipelineFor(cfg *config.Config, t backend.ProofType) (p pipeline, ok bool) {
	switch t {
	case backend.ProofTypeL2Execution:
		return pipeline{
			mode:            backend.ProverMode(cfg.Execution.ProverMode),
			requestsRootDir: cfg.Execution.RequestsRootDir,
			nativeRunnerBin: cfg.Execution.NativeRunnerBin,
			guestELF:        cfg.Execution.GuestELF,
			priority:        0,
		}, true
	case backend.ProofTypeRollup:
		if !cfg.Rollup.Configured() {
			return pipeline{}, false
		}
		return pipeline{mode: backend.ProverMode(cfg.Rollup.ProverMode), requestsRootDir: cfg.Rollup.RequestsRootDir, priority: 1}, true
	case backend.ProofTypeRollupAggregation:
		if !cfg.Aggregation.Configured() {
			return pipeline{}, false
		}
		return pipeline{mode: backend.ProverMode(cfg.Aggregation.ProverMode), requestsRootDir: cfg.Aggregation.RequestsRootDir, priority: 2}, true
	}
	return pipeline{}, false
}

// buildRunner builds the request-to-response runner for one pipeline, backed by an
// in-process Core.
func buildRunner(version string, p pipeline) (*jobadapter.Runner, error) {
	core, err := backend.New(backend.Config{Mode: p.mode, GuestELFPath: p.guestELF})
	if err != nil {
		return nil, fmt.Errorf("building prover core: %w", err)
	}
	opts := []jobadapter.RunnerOption{jobadapter.WithMode(p.mode)}
	if p.nativeRunnerBin != "" {
		opts = append(opts, jobadapter.WithNativeRunnerBin(p.nativeRunnerBin))
	}
	return jobadapter.NewRunner(core, version, opts...)
}

// runAdapter watches one queue per configured pipeline and spawns a "prover
// prove" child for each request.
func runAdapter(args []string) error {
	fs := flag.NewFlagSet("prover", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the TOML config file (or set CONFIG_FILE)")
	localID := fs.String("local-id", "", "worker id for job claiming and crash recovery (or set WORKER_ID)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, path, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	var queues []filesystem.Queue
	for _, t := range proofTypes {
		p, ok := pipelineFor(cfg, t)
		if !ok {
			continue
		}
		if !p.mode.Valid() {
			return fmt.Errorf("invalid prover_mode %q for %s", p.mode, t)
		}
		queues = append(queues, filesystem.Queue{RequestsRootDir: p.requestsRootDir, Priority: p.priority})
		if p.mode.IsDev() {
			logrus.Warnf("prover-ray %s in DEV mode %q against %s: responses are NOT real proofs",
				t, p.mode, p.requestsRootDir)
		}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding prover binary: %w", err)
	}
	adapter, err := filesystem.New(
		filesystem.Config{Queues: queues, WorkerID: resolveWorkerID(*localID)},
		spawnProver(self, path))
	if err != nil {
		return fmt.Errorf("building filesystem adapter: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return adapter.Run(ctx)
}

// resolveWorkerID picks the worker id from the flag, then WORKER_ID, then the
// hostname. A stable id lets a restarted worker requeue its own crashed jobs.
func resolveWorkerID(flagID string) string {
	if flagID != "" {
		return flagID
	}
	if env := os.Getenv("WORKER_ID"); env != "" {
		return env
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "prover-ray"
}

// spawnProver returns the adapter's default RunProver: run "prover prove" as a
// child process and report its exit code.
func spawnProver(bin, configPath string) filesystem.RunProver {
	return func(ctx context.Context, reqPath, respPath string) (int, error) {
		cmd := exec.CommandContext(ctx, bin, "prove",
			"--config", configPath, "--in", reqPath, "--out", respPath)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err := cmd.Run()
		if err == nil {
			return 0, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
}

// runProve is the prover: prove one request file, write the response, exit with
// a code.
func runProve(args []string) error {
	fs := flag.NewFlagSet("prover prove", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the TOML config file (or set CONFIG_FILE)")
	inPath := fs.String("in", "", "request file to prove (required)")
	outPath := fs.String("out", "", "file to write the response to (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inPath == "" || *outPath == "" {
		return fmt.Errorf("--in and --out are required")
	}
	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	// Proof type comes from the request filename; it selects the pipeline config.
	name := filepath.Base(*inPath)
	proofType := jobadapter.ProofTypeForName(name)
	p, ok := pipelineFor(cfg, proofType)
	if !ok {
		return fmt.Errorf("proof type %q is not configured", proofType)
	}
	if !p.mode.Valid() {
		return fmt.Errorf("invalid prover_mode %q for %s", p.mode, proofType)
	}
	runner, err := buildRunner(cfg.Version, p)
	if err != nil {
		return err
	}

	body, err := os.ReadFile(*inPath)
	if err != nil {
		return fmt.Errorf("reading request file: %w", err)
	}
	id := strings.TrimSuffix(name, filepath.Ext(name))

	result := runner.Run(context.Background(),
		jobadapter.RunRequest{ID: id, Type: proofType, Body: body})

	data, err := json.MarshalIndent(result.ResponseBody, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding response: %w", err)
	}
	if err := os.WriteFile(*outPath, data, 0o600); err != nil {
		return fmt.Errorf("writing response file: %w", err)
	}

	// Response is always written; the exit code signals success or failure.
	if result.Status != jobadapter.RunStatusSuccess {
		if result.Err != nil {
			return fmt.Errorf("prove failed: %w", result.Err)
		}
		return fmt.Errorf("prove failed with status %s", result.Status)
	}
	return nil
}

// Command prover runs either the queue adapter or a one-shot prove worker:
//
//	prover --config <toml>                                 watch the queue, prove each request
//	prover prove --config <toml> --in <req> --out <resp>   prove one request file, exit with a code
//
// Config is a TOML file (--config or CONFIG_FILE). The worker also serves to
// prove a single request by hand.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/filesystem"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/subprocess"
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

// loadConfig loads the config (--config or CONFIG_FILE), sets the log level, and
// validates the mode. It also returns the resolved path, which the adapter passes
// to the worker it spawns.
func loadConfig(configPath string) (*config.Config, backend.ProverMode, string, error) {
	path := configPath
	if path == "" {
		path = os.Getenv("CONFIG_FILE")
	}
	if path == "" {
		return nil, "", "", fmt.Errorf("--config (or CONFIG_FILE) is required")
	}
	cfg, err := config.NewConfigFromFile(path)
	if err != nil {
		return nil, "", "", err
	}
	if cfg.LogLevel >= int(logrus.PanicLevel) && cfg.LogLevel <= int(logrus.TraceLevel) {
		logrus.SetLevel(logrus.Level(cfg.LogLevel))
	}
	mode := backend.ProverMode(cfg.Execution.ProverMode)
	if !mode.Valid() {
		return nil, "", "", fmt.Errorf("invalid execution.prover_mode %q", cfg.Execution.ProverMode)
	}
	return cfg, mode, path, nil
}

// buildRunner builds the request-to-response runner backed by an in-process Core.
func buildRunner(cfg *config.Config, mode backend.ProverMode) (*jobadapter.Runner, error) {
	core, err := backend.New(backend.Config{Mode: mode, GuestELFPath: cfg.Execution.GuestELF})
	if err != nil {
		return nil, fmt.Errorf("building prover core: %w", err)
	}
	opts := []jobadapter.RunnerOption{jobadapter.WithMode(mode)}
	if cfg.Execution.NativeRunnerBin != "" {
		opts = append(opts, jobadapter.WithNativeRunnerBin(cfg.Execution.NativeRunnerBin))
	}
	return jobadapter.NewRunner(core, cfg.Version, opts...)
}

// runAdapter watches the request queue and spawns a "prover prove" worker for
// each request.
func runAdapter(args []string) error {
	fs := flag.NewFlagSet("prover", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the TOML config file (or set CONFIG_FILE)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, mode, path, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding prover binary: %w", err)
	}

	adapter, err := filesystem.New(
		filesystem.Config{RequestsRootDir: cfg.Execution.RequestsRootDir},
		subprocess.Prover{BinPath: self, ConfigPath: path},
	)
	if err != nil {
		return fmt.Errorf("building filesystem adapter: %w", err)
	}

	if mode.IsDev() {
		logrus.Warnf("prover-ray running in DEV mode %q against %s: responses are NOT real proofs",
			mode, cfg.Execution.RequestsRootDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return adapter.Run(ctx)
}

// runProve is the worker: prove one request file, write the response, exit with
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
	cfg, mode, _, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	runner, err := buildRunner(cfg, mode)
	if err != nil {
		return err
	}

	body, err := os.ReadFile(*inPath)
	if err != nil {
		return fmt.Errorf("reading request file: %w", err)
	}
	// Proof type comes from the request filename.
	name := filepath.Base(*inPath)
	id := strings.TrimSuffix(name, filepath.Ext(name))

	result := runner.Run(context.Background(),
		jobadapter.RunRequest{ID: id, Type: jobadapter.ProofTypeForName(name), Body: body})

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

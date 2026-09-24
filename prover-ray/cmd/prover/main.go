// Command prover runs the prover-ray backend against a filesystem request queue.
// Configuration is a TOML file, like the legacy prover: pass --config (or set
// CONFIG_FILE). dev-mock and dev-zkvm are runnable today; partial and full still
// return their blocker errors.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
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
	fs := flag.NewFlagSet("prover", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the TOML config file (or set CONFIG_FILE)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := *configPath
	if path == "" {
		path = os.Getenv("CONFIG_FILE")
	}
	if path == "" {
		return fmt.Errorf("--config (or CONFIG_FILE) is required")
	}

	cfg, err := config.NewConfigFromFile(path)
	if err != nil {
		return err
	}
	if cfg.LogLevel >= int(logrus.PanicLevel) && cfg.LogLevel <= int(logrus.TraceLevel) {
		logrus.SetLevel(logrus.Level(cfg.LogLevel))
	}

	mode := backend.ProverMode(cfg.Execution.ProverMode)
	if !mode.Valid() {
		return fmt.Errorf("invalid execution.prover_mode %q", cfg.Execution.ProverMode)
	}

	core, err := backend.New(backend.Config{Mode: mode, GuestELFPath: cfg.Execution.GuestELF})
	if err != nil {
		return fmt.Errorf("building prover core: %w", err)
	}

	adapter, err := filesystem.New(filesystem.Config{
		RequestsRootDir:     cfg.Execution.RequestsRootDir,
		ProverVersion:       cfg.Version,
		Mode:                mode,
		NativeRunnerBinPath: cfg.Execution.NativeRunnerBin,
	}, core)
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

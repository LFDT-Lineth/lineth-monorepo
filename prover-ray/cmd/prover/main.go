// Command prover runs the prover-ray backend against a filesystem request queue.
// dev-mock and dev-zkvm are runnable today; partial and full still return their
// blocker errors. dev-zkvm needs --native-runner-bin and --guest-elf.
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
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		logrus.Fatalf("prover: %v", err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("prover", flag.ContinueOnError)
	requestsDir := fs.String("requests-dir", "",
		"directory holding requests/, responses/, requests-done/ (required)")
	proverVersion := fs.String("prover-version", "0.0.0-riscv",
		"prover version echoed to the coordinator")
	modeStr := fs.String("mode", string(backend.ProverModeDevMock),
		"prover mode (dev-mock or dev-zkvm are runnable today)")
	nativeRunnerBin := fs.String("native-runner-bin", "",
		"path to the native l2-execution-runner binary (required for dev-zkvm)")
	guestELF := fs.String("guest-elf", "",
		"path to the l2-execution guest ELF (required for dev-zkvm)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *requestsDir == "" {
		return fmt.Errorf("--requests-dir is required")
	}
	mode := backend.ProverMode(*modeStr)
	if !mode.Valid() {
		return fmt.Errorf("invalid --mode %q", *modeStr)
	}

	core, err := backend.New(backend.Config{Mode: mode, GuestELFPath: *guestELF})
	if err != nil {
		return fmt.Errorf("building prover core: %w", err)
	}

	adapter, err := filesystem.New(filesystem.Config{
		RequestsRootDir:     *requestsDir,
		ProverVersion:       *proverVersion,
		Mode:                mode,
		NativeRunnerBinPath: *nativeRunnerBin,
	}, core)
	if err != nil {
		return fmt.Errorf("building filesystem adapter: %w", err)
	}

	if mode.IsDev() {
		logrus.Warnf("prover-ray running in DEV mode %q against %s: responses are NOT real proofs",
			mode, *requestsDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return adapter.Run(ctx)
}

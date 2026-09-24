// Package subprocess runs "prover prove" as a child process, one per request, so
// a proof that runs out of memory kills only its worker and returns an exit code.
package subprocess

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// Prover proves a request file by spawning the prove worker.
type Prover struct {
	// BinPath is the prover binary to spawn (usually os.Executable()).
	BinPath string
	// ConfigPath is passed to the worker as --config.
	ConfigPath string
}

// Prove runs "<bin> prove --config <cfg> --in <req> --out <resp>" and returns the
// worker's exit code (0 = success). A non-nil error means the worker could not be
// run at all (as opposed to running and failing, which is a non-zero exit code).
func (p Prover) Prove(ctx context.Context, reqPath, respPath string) (int, error) {
	cmd := exec.CommandContext(ctx, p.BinPath, "prove",
		"--config", p.ConfigPath, "--in", reqPath, "--out", respPath)
	// Worker logs flow to the adapter's stdout/stderr, and on to the container log.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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

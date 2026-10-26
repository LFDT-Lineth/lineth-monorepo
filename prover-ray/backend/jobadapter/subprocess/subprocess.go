// Package subprocess runs one proof request in a "prover prove" child process.
// The request is written to the child's stdin and the response is read from its
// stdout. A crash or OOM kill terminates only the child, so the caller can
// report the failure.
package subprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
)

// Exit codes of the child.
const (
	ExitSuccess = 0
	ExitFailure = 1
	// ExitOOM is the exit code of a child killed by SIGKILL, as done by the
	// kernel OOM killer.
	ExitOOM = 128 + int(syscall.SIGKILL)
)

// Command describes how to start the prover child.
type Command struct {
	Bin        string
	ConfigPath string
}

// Output holds the child's stdout and exit code. A child killed by a signal has
// exit code 128 + signal number.
type Output struct {
	Response []byte
	ExitCode int
}

// Spawn runs one request in a child and waits for it. It returns an error only
// when the child could not start or ctx was cancelled; a failed run is a
// non-zero ExitCode.
func (c Command) Spawn(ctx context.Context, req jobadapter.RunRequest) (Output, error) {
	//nolint:gosec // G204: Bin and ConfigPath are operator configuration, not user input
	cmd := exec.CommandContext(ctx, c.Bin, "prove",
		"--config", c.ConfigPath,
		"--type", string(req.Type),
		"--id", req.ID,
		"--in", "-",
		"--out", "-",
	)
	var stdout bytes.Buffer
	cmd.Stdin = bytes.NewReader(req.Body)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return Output{Response: stdout.Bytes(), ExitCode: ExitSuccess}, nil
	}
	// Cancellation kills the child with SIGKILL; report it as an error, not as
	// ExitOOM.
	if ctx.Err() != nil {
		return Output{}, fmt.Errorf("prover child for %s cancelled: %w", req.ID, ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return Output{Response: stdout.Bytes(), ExitCode: exitCode(exitErr)}, nil
	}
	return Output{}, fmt.Errorf("starting prover child for %s: %w", req.ID, err)
}

// exitCode returns the exit status, using 128 + signal number for a child killed
// by a signal, for which os.ProcessState.ExitCode returns -1.
func exitCode(exitErr *exec.ExitError) int {
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exitErr.ExitCode()
}

// Run runs one request like [Command.Spawn] and returns the result in the form
// of [jobadapter.Runner.Run].
func (c Command) Run(ctx context.Context, req jobadapter.RunRequest) jobadapter.RunResult {
	out, err := c.Spawn(ctx, req)
	if err != nil {
		return failed(jobadapter.FailureCodeInternalError, err, nil)
	}
	if out.ExitCode == ExitSuccess {
		if !json.Valid(out.Response) {
			return failed(jobadapter.FailureCodeInternalError,
				fmt.Errorf("prover child for %s exited 0 without a valid JSON response", req.ID), nil)
		}
		return jobadapter.RunResult{
			ResponseBody: json.RawMessage(out.Response),
			Status:       jobadapter.RunStatusSuccess,
		}
	}
	code, msg := FailureFor(out)
	err = fmt.Errorf("prover child for %s exited with code %d", req.ID, out.ExitCode)
	if msg != "" {
		err = fmt.Errorf("%w: %s", err, msg)
	}
	var body json.RawMessage
	if json.Valid(out.Response) {
		body = json.RawMessage(out.Response)
	}
	return failed(code, err, body)
}

// FailureFor returns the failure code and message of a failed run. A failure
// code in the child's response takes precedence; otherwise [ExitOOM] maps to
// oom and any other exit code to internal_error.
func FailureFor(out Output) (jobadapter.FailureCode, string) {
	var body struct {
		FailureCode jobadapter.FailureCode `json:"failureCode"`
		Error       string                 `json:"error"`
	}
	if json.Unmarshal(out.Response, &body) == nil && body.FailureCode != "" {
		return body.FailureCode, body.Error
	}
	if out.ExitCode == ExitOOM {
		return jobadapter.FailureCodeOOM, ""
	}
	return jobadapter.FailureCodeInternalError, ""
}

func failed(code jobadapter.FailureCode, err error, body json.RawMessage) jobadapter.RunResult {
	r := jobadapter.RunResult{Status: jobadapter.RunStatusFailed, FailureCode: code, Err: err}
	if body != nil {
		r.ResponseBody = body
	}
	return r
}

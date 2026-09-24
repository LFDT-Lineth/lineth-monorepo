// Package filesystem is the queue supervisor. It finds request files, claims each
// with an atomic rename, hands it to a Prover (which writes the response), and
// archives the request tagged with the prover's exit code.
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	requestsSubDir  = "requests"
	responsesSubDir = "responses"
	doneSubDir      = "requests-done"

	inProgressSuffix = ".inprogress"
	successSuffix    = ".success"

	defaultPollInterval = time.Second

	dirPerm = 0o750
)

// Prover proves one request file into a response file. Prove returns the worker's
// exit code (0 = success); a non-nil error means the worker could not be run at
// all, which is different from running and failing (a non-zero exit code).
type Prover interface {
	Prove(ctx context.Context, requestPath, responsePath string) (exitCode int, err error)
}

// Config holds the queue layout and poll cadence.
type Config struct {
	// RequestsRootDir contains the requests/, responses/, and requests-done/
	// subdirectories; [New] creates them if missing.
	RequestsRootDir string
	// PollInterval is how often [Adapter.Run] rescans requests/. Defaults to one
	// second when unset.
	PollInterval time.Duration
}

// Adapter polls the request queue and spawns a Prover for each request.
type Adapter struct {
	cfg    Config
	prover Prover
}

// New creates the requests/, responses/, and requests-done/ subdirectories under
// cfg.RequestsRootDir and returns an [Adapter] ready to run.
func New(cfg Config, prover Prover) (*Adapter, error) {
	if cfg.RequestsRootDir == "" {
		return nil, fmt.Errorf("jobadapter/filesystem.New: RequestsRootDir must be set")
	}
	if prover == nil {
		return nil, fmt.Errorf("jobadapter/filesystem.New: prover must not be nil")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}

	a := &Adapter{cfg: cfg, prover: prover}
	for _, dir := range []string{a.requestsDir(), a.responsesDir(), a.doneDir()} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("jobadapter/filesystem.New: creating %s: %w", dir, err)
		}
	}
	return a, nil
}

func (a *Adapter) requestsDir() string {
	return filepath.Join(a.cfg.RequestsRootDir, requestsSubDir)
}
func (a *Adapter) responsesDir() string {
	return filepath.Join(a.cfg.RequestsRootDir, responsesSubDir)
}
func (a *Adapter) doneDir() string { return filepath.Join(a.cfg.RequestsRootDir, doneSubDir) }

// Run polls requests/ every cfg.PollInterval until ctx is cancelled, draining the
// request it is processing before returning nil.
func (a *Adapter) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := a.processOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// processOnce scans requests/ once and processes every pending request file (those
// ending in .json), one at a time. Already-claimed files (.inprogress) no longer
// end in .json and are skipped. It stops early if ctx is cancelled.
func (a *Adapter) processOnce(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(a.requestsDir())
	if err != nil {
		return 0, fmt.Errorf("jobadapter: reading requests dir: %w", err)
	}

	processed := 0
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return processed, nil // cancellation is graceful, not an error
		default:
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		handled, err := a.processRequest(ctx, entry.Name())
		if err != nil {
			return processed, err
		}
		if handled {
			processed++
		}
	}
	return processed, nil
}

// processRequest claims one request, spawns the prover on it, publishes the
// response, and archives the request tagged with the exit code. It returns false
// without error when the claim is lost or the worker could not be run (the request
// is left for the next scan). A returned error is a filesystem failure.
func (a *Adapter) processRequest(ctx context.Context, name string) (bool, error) {
	src := filepath.Join(a.requestsDir(), name)
	claimed := src + inProgressSuffix
	if err := os.Rename(src, claimed); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil // another worker claimed it first
		}
		return false, fmt.Errorf("jobadapter: claiming %s: %w", name, err)
	}

	respTmp := filepath.Join(a.responsesDir(), name+inProgressSuffix)
	exitCode, err := a.prover.Prove(ctx, claimed, respTmp)
	if err != nil {
		// The worker could not be run; leave the request for the next scan.
		_ = os.Remove(respTmp)
		_ = os.Rename(claimed, src)
		return false, nil
	}

	if err := a.publishResponse(respTmp, name); err != nil {
		_ = os.Rename(claimed, src)
		return false, err
	}
	if err := a.archive(claimed, name, exitCode); err != nil {
		_ = os.Rename(claimed, src)
		return false, err
	}
	return true, nil
}

// publishResponse moves the worker's response into place atomically. A worker that
// crashed before writing leaves no temp file, which is not an error here.
func (a *Adapter) publishResponse(respTmp, name string) error {
	if _, err := os.Stat(respTmp); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("jobadapter: response for %s: %w", name, err)
	}
	if err := os.Rename(respTmp, filepath.Join(a.responsesDir(), name)); err != nil {
		return fmt.Errorf("jobadapter: publishing response for %s: %w", name, err)
	}
	return nil
}

// archive moves the claimed request into requests-done/, tagging the outcome with
// the exit code (.success for 0, .failure.<code> otherwise) so it can be monitored.
func (a *Adapter) archive(claimed, name string, exitCode int) error {
	suffix := successSuffix
	if exitCode != 0 {
		suffix = fmt.Sprintf(".failure.%d", exitCode)
	}
	dst := filepath.Join(a.doneDir(), name+suffix)
	if err := os.Rename(claimed, dst); err != nil {
		return fmt.Errorf("jobadapter: archiving %s: %w", name, err)
	}
	return nil
}

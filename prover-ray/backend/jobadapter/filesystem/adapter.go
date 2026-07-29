// Package filesystem provides a file queue around jobadapter.Runner.
//
// Adapter finds request files, claims them with an atomic rename, passes their
// contents to the runner, writes response files, and archives completed
// requests.
package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
)

const (
	requestsSubDir  = "requests"
	responsesSubDir = "responses"
	doneSubDir      = "requests-done"

	inProgressSuffix = ".inprogress"
	successSuffix    = ".success"
	failureSuffix    = ".failure"

	defaultPollInterval = time.Second

	dirPerm  = 0o750
	filePerm = 0o600
)

// Config holds the filesystem queue layout and poll cadence.
type Config struct {
	// RequestsRootDir contains the requests/, responses/, and requests-done/
	// subdirectories; [New] creates them if missing.
	RequestsRootDir string
	// PollInterval is how often [Adapter.Run] rescans requests/ for new work.
	// Defaults to one second when unset.
	PollInterval time.Duration
	// ProverVersion is emitted on successful getZkL2ExecutionProofV1-shaped
	// responses and must be set by process config.
	ProverVersion string
}

const statusFailed = "failed"

type failureResponseBody struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Adapter polls a filesystem request queue and sends each request to
// jobadapter.Runner.
type Adapter struct {
	cfg    Config
	runner *jobadapter.Runner
}

// New creates the requests/, responses/, and requests-done/ subdirectories
// under cfg.RequestsRootDir and returns an [Adapter] ready to run.
func New(cfg Config, prover jobadapter.Prover) (*Adapter, error) {
	if cfg.RequestsRootDir == "" {
		return nil, fmt.Errorf("jobadapter/filesystem.New: RequestsRootDir must be set")
	}
	if prover == nil {
		return nil, fmt.Errorf("jobadapter/filesystem.New: prover must not be nil")
	}
	if cfg.ProverVersion == "" {
		return nil, fmt.Errorf("jobadapter/filesystem.New: ProverVersion must be set")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	runner, err := jobadapter.NewRunner(prover, cfg.ProverVersion)
	if err != nil {
		return nil, err
	}

	a := &Adapter{cfg: cfg, runner: runner}
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

// Run polls requests/ every cfg.PollInterval until ctx is cancelled, draining
// the request it is processing before returning nil.
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

// processOnce scans requests/ once, processes every pending request file
// (those ending in .json), and returns how many it handled. Already-claimed
// files such as req.json.inprogress are skipped because they no longer end in
// .json. It stops early if ctx is cancelled, leaving the remaining files for
// the next scan.
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

// processRequest claims one request (atomic rename to .inprogress), runs it,
// writes its response, and archives it. It returns false without error when the
// claim is lost to another worker. A returned error is an infrastructure
// failure (filesystem), not a proof failure; those are recorded in the
// response.
func (a *Adapter) processRequest(ctx context.Context, name string) (bool, error) {
	src := filepath.Join(a.requestsDir(), name)
	claimed := src + inProgressSuffix
	if err := os.Rename(src, claimed); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil // another worker claimed it first
		}
		return false, fmt.Errorf("jobadapter: claiming %s: %w", name, err)
	}

	result := a.run(ctx, name, claimed)

	if err := a.writeResponse(name, result.ResponseBody); err != nil {
		_ = os.Rename(claimed, src) // best-effort: avoid stranding the claimed request
		return false, err
	}
	if err := a.archive(claimed, name, result.Success); err != nil {
		_ = os.Rename(claimed, src) // best-effort: allow retry after archive infrastructure failures
		return false, err
	}
	return true, nil
}

// run reads the claimed request and hands it to Runner. Read failures map to
// failure responses so the request can still be archived and the loop can
// continue.
func (a *Adapter) run(ctx context.Context, name, claimed string) jobadapter.RunResult {
	id := strings.TrimSuffix(name, ".json")

	data, err := os.ReadFile(claimed) //nolint:gosec // claimed is a scanned entry under RequestsRootDir/requests
	if err != nil {
		return jobadapter.RunResult{ResponseBody: failureResponse(id, err)}
	}
	return a.runner.Run(ctx, id, data)
}

func failureResponse(id string, err error) failureResponseBody {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return failureResponseBody{JobID: id, Status: statusFailed, Error: msg}
}

func (a *Adapter) writeResponse(name string, resp any) error {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("jobadapter: encoding response for %s: %w", name, err)
	}
	if err := writeFileAtomic(filepath.Join(a.responsesDir(), name), data, filePerm); err != nil {
		return fmt.Errorf("jobadapter: writing response for %s: %w", name, err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	keepTemp = true
	return nil
}

// archive moves the claimed request into requests-done/, tagging failures so a
// human can tell them apart.
func (a *Adapter) archive(claimed, name string, success bool) error {
	suffix := failureSuffix
	if success {
		suffix = successSuffix
	}
	dst := filepath.Join(a.doneDir(), name+suffix)
	if err := os.Rename(claimed, dst); err != nil {
		return fmt.Errorf("jobadapter: archiving %s: %w", name, err)
	}
	return nil
}

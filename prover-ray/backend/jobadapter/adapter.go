package jobadapter

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

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

const (
	requestsSubDir  = "requests"
	responsesSubDir = "responses"
	doneSubDir      = "requests-done"

	// inProgressSuffix marks a request claimed by this worker; a file bearing
	// it is skipped by the scan (another worker owns it, or it is mid-flight).
	inProgressSuffix = ".inprogress"
	// failedSuffix distinguishes archived requests that did not prove.
	failedSuffix = ".failed"

	defaultPollInterval  = time.Second
	defaultProverVersion = "4.0.0-riscv"

	dirPerm  = 0o750
	filePerm = 0o600
)

// Prover is the subset of [backend.Core] the adapter needs. It is an interface
// so tests can drive the adapter with a mock and never build a circuit.
type Prover interface {
	Prove(ctx context.Context, job backend.Job) backend.Result
}

// Config holds the adapter's filesystem layout and poll cadence.
type Config struct {
	// RootDir contains the requests/, responses/, and requests-done/
	// subdirectories; [New] creates them if missing.
	RootDir string
	// PollInterval is how often [Adapter.Run] rescans requests/ for new work.
	// Defaults to one second when unset.
	PollInterval time.Duration
	// ProverVersion is emitted on successful getZkL2ExecutionProofV1-shaped
	// responses. Defaults to defaultProverVersion when unset.
	ProverVersion string
}

const statusFailed = "failed"

// Adapter polls a filesystem request queue and drives each request through a
// [Prover].
type Adapter struct {
	cfg    Config
	prover Prover
}

// New creates the requests/, responses/, and requests-done/ subdirectories
// under cfg.RootDir and returns an [Adapter] ready to run.
func New(cfg Config, prover Prover) (*Adapter, error) {
	if cfg.RootDir == "" {
		return nil, fmt.Errorf("jobadapter.New: RootDir must be set")
	}
	if prover == nil {
		return nil, fmt.Errorf("jobadapter.New: prover must not be nil")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.ProverVersion == "" {
		cfg.ProverVersion = defaultProverVersion
	}

	a := &Adapter{cfg: cfg, prover: prover}
	for _, dir := range []string{a.requestsDir(), a.responsesDir(), a.doneDir()} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("jobadapter.New: creating %s: %w", dir, err)
		}
	}
	return a, nil
}

func (a *Adapter) requestsDir() string  { return filepath.Join(a.cfg.RootDir, requestsSubDir) }
func (a *Adapter) responsesDir() string { return filepath.Join(a.cfg.RootDir, responsesSubDir) }
func (a *Adapter) doneDir() string      { return filepath.Join(a.cfg.RootDir, doneSubDir) }

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
// (those ending in .json, skipping already-claimed .inprogress files), and
// returns how many it handled. It stops early if ctx is cancelled, leaving the
// remaining files for the next scan.
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
// failure (filesystem), not a proof failure — those are recorded in the
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

	resp, success := a.run(ctx, name, claimed)

	if err := a.writeResponse(name, resp); err != nil {
		return false, err
	}
	if err := a.archive(claimed, name, success); err != nil {
		return false, err
	}
	return true, nil
}

// run decodes the claimed request and proves it, returning the response body
// and whether it succeeded. Decode, validation, and proof failures all map to a
// failure response rather than an error.
func (a *Adapter) run(ctx context.Context, name, claimed string) (any, bool) {
	id := strings.TrimSuffix(name, ".json")

	data, err := os.ReadFile(claimed) //nolint:gosec // claimed is a scanned entry under RootDir/requests
	if err != nil {
		return failureResponse(id, err), false
	}

	req, err := DecodeRequest(data)
	if err != nil {
		return failureResponse(id, err), false
	}
	if len(req.Payloads) != 1 {
		return failureResponse(id, fmt.Errorf(
			"multi-block requests are not supported (got %d payloads): %w",
			len(req.Payloads), backend.ErrNotImplemented)), false
	}

	payload := req.Payloads[0]
	if len(payload.ForcedTransactions) != 0 {
		return failureResponse(id, fmt.Errorf(
			"forced transactions are not supported (got %d): %w",
			len(payload.ForcedTransactions), backend.ErrNotImplemented)), false
	}

	result := a.prover.Prove(ctx, backend.Job{
		ID:         id,
		Type:       backend.ProofTypeL2Execution,
		StartBlock: payload.BlockNumber,
		EndBlock:   payload.BlockNumber,
		Payload:    payload.FramedSSZ,
	})
	if result.Status != backend.ResultStatusOK {
		err := result.Err
		if err == nil {
			err = fmt.Errorf("prover returned status %s", result.Status)
		}
		return failureResponse(id, err), false
	}
	return newExecutionResponse(result, payload.BlockNumber, a.cfg.ProverVersion), true
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
	dst := filepath.Join(a.doneDir(), name)
	if !success {
		dst += failedSuffix
	}
	if err := os.Rename(claimed, dst); err != nil {
		return fmt.Errorf("jobadapter: archiving %s: %w", name, err)
	}
	return nil
}

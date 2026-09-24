// Package filesystem is the queue supervisor. It watches one request folder per
// pipeline, picks the next request by end block (earliest first), claims it with
// an atomic rename, runs a prover on it (which writes the response), and archives
// the request tagged with the prover's exit code.
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

	// scorePriorityBase keeps the per-queue priority a tiebreaker below the
	// end-block term, so end block always dominates the order.
	scorePriorityBase = 100
)

// RunProver runs a prover for one request file, writing the response file, and
// returns its exit code (0 = success). A non-nil error means the prover could not
// be run at all, which is different from running and failing (a non-zero code).
type RunProver func(ctx context.Context, requestPath, responsePath string) (exitCode int, err error)

// Queue is one pipeline's request folder plus its priority. Priority is only a
// tiebreaker for requests with the same end block (lower runs first).
type Queue struct {
	RequestsRootDir string
	Priority        int
}

// Config holds the queues to watch and the poll cadence.
type Config struct {
	// Queues is one entry per pipeline (execution, rollup, aggregation).
	Queues []Queue
	// PollInterval is how often [Adapter.Run] rescans when the queues are empty.
	// Defaults to one second when unset.
	PollInterval time.Duration
}

// Adapter polls the request queues and runs a prover for each request, earliest
// end block first.
type Adapter struct {
	cfg   Config
	spawn RunProver
}

// New creates the requests/, responses/, and requests-done/ subdirectories under
// each queue's root and returns an [Adapter] ready to run.
func New(cfg Config, spawn RunProver) (*Adapter, error) {
	if len(cfg.Queues) == 0 {
		return nil, fmt.Errorf("jobadapter/filesystem.New: at least one queue must be set")
	}
	if spawn == nil {
		return nil, fmt.Errorf("jobadapter/filesystem.New: spawn function must not be nil")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}

	a := &Adapter{cfg: cfg, spawn: spawn}
	for _, q := range cfg.Queues {
		for _, dir := range []string{requestsDir(q.RequestsRootDir), responsesDir(q.RequestsRootDir), doneDir(q.RequestsRootDir)} {
			if err := os.MkdirAll(dir, dirPerm); err != nil {
				return nil, fmt.Errorf("jobadapter/filesystem.New: creating %s: %w", dir, err)
			}
		}
	}
	return a, nil
}

func requestsDir(root string) string  { return filepath.Join(root, requestsSubDir) }
func responsesDir(root string) string { return filepath.Join(root, responsesSubDir) }
func doneDir(root string) string      { return filepath.Join(root, doneSubDir) }

// Run picks the highest-priority claimable request, runs it, and rescans; when the
// queues are empty it waits PollInterval. It returns nil when ctx is cancelled.
func (a *Adapter) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()
	for {
		processed, err := a.processNext(ctx)
		if err != nil {
			return err
		}
		if processed {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// pending is one claimable request across the queues.
type pending struct {
	root  string
	name  string
	score int
}

// processNext scans every queue, orders the pending requests by score (end block
// first, priority as tiebreaker), and processes the best claimable one. It reports
// whether it processed a request.
func (a *Adapter) processNext(ctx context.Context) (bool, error) {
	var jobs []pending
	for _, q := range a.cfg.Queues {
		entries, err := os.ReadDir(requestsDir(q.RequestsRootDir))
		if err != nil {
			return false, fmt.Errorf("jobadapter: reading requests dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			jobs = append(jobs, pending{
				root:  q.RequestsRootDir,
				name:  entry.Name(),
				score: scorePriorityBase*endBlock(entry.Name()) + q.Priority,
			})
		}
	}
	if len(jobs) == 0 {
		return false, nil
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].score < jobs[j].score })

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return false, nil
		default:
		}
		claimed, err := a.processRequest(ctx, job.root, job.name)
		if err != nil {
			return false, err
		}
		if claimed {
			return true, nil
		}
	}
	return false, nil
}

// endBlock reads the end block from a "<start>-<end>-getZk...json" filename. An
// unparseable name scores 0 (processed first, then fails and is archived).
func endBlock(name string) int {
	parts := strings.SplitN(name, "-", 3)
	if len(parts) < 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}

// processRequest claims one request, runs the prover, publishes the response, and
// archives the request tagged with the exit code. It returns false without error
// when the claim is lost or the prover could not be run. A returned error is a
// filesystem failure.
func (a *Adapter) processRequest(ctx context.Context, root, name string) (bool, error) {
	src := filepath.Join(requestsDir(root), name)
	claimed := src + inProgressSuffix
	if err := os.Rename(src, claimed); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil // another supervisor claimed it first
		}
		return false, fmt.Errorf("jobadapter: claiming %s: %w", name, err)
	}

	respTmp := filepath.Join(responsesDir(root), name+inProgressSuffix)
	exitCode, err := a.spawn(ctx, claimed, respTmp)
	if err != nil {
		// The prover could not be run; leave the request for the next scan.
		_ = os.Remove(respTmp)
		_ = os.Rename(claimed, src)
		return false, nil
	}

	if err := publishResponse(respTmp, filepath.Join(responsesDir(root), name)); err != nil {
		_ = os.Rename(claimed, src)
		return false, err
	}
	if err := archive(claimed, filepath.Join(doneDir(root), name), exitCode); err != nil {
		_ = os.Rename(claimed, src)
		return false, err
	}
	return true, nil
}

// publishResponse moves the prover's response into place atomically. A prover that
// crashed before writing leaves no temp file, which is not an error here.
func publishResponse(respTmp, respFinal string) error {
	if _, err := os.Stat(respTmp); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("jobadapter: response %s: %w", respFinal, err)
	}
	if err := os.Rename(respTmp, respFinal); err != nil {
		return fmt.Errorf("jobadapter: publishing response %s: %w", respFinal, err)
	}
	return nil
}

// archive moves the claimed request into requests-done/, tagging the outcome with
// the exit code (.success for 0, .failure.<code> otherwise) so it can be monitored.
func archive(claimed, doneBase string, exitCode int) error {
	dst := doneBase + successSuffix
	if exitCode != 0 {
		dst = fmt.Sprintf("%s.failure.%d", doneBase, exitCode)
	}
	if err := os.Rename(claimed, dst); err != nil {
		return fmt.Errorf("jobadapter: archiving %s: %w", filepath.Base(claimed), err)
	}
	return nil
}

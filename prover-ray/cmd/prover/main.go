// Command prover runs either the queue adapter or a one-shot prover:
//
//	prover --config <toml>                                 watch the queues, prove each request
//	prover prove --config <toml> --in <req> --out <resp>   prove one request, exit with a code
//
// Config is a TOML file (--config or CONFIG_FILE). The queue adapter runs each
// request in a "prover prove" child. --in and --out also accept "-" for stdin
// and stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
		return pipeline{
			mode:            backend.ProverMode(cfg.Rollup.ProverMode),
			requestsRootDir: cfg.Rollup.RequestsRootDir,
			priority:        1,
		}, true
	case backend.ProofTypeRollupAggregation:
		if !cfg.Aggregation.Configured() {
			return pipeline{}, false
		}
		return pipeline{
			mode:            backend.ProverMode(cfg.Aggregation.ProverMode),
			requestsRootDir: cfg.Aggregation.RequestsRootDir,
			priority:        2,
		}, true
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
	if err := cfg.ValidateQueues(); err != nil {
		return fmt.Errorf("invalid config %q: %w", path, err)
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
		spawnProver(subprocess.Command{Bin: self, ConfigPath: path}))
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

// spawnProver returns the adapter's RunProver. The proof type and job id come
// from the request file name.
func spawnProver(cmd subprocess.Command) filesystem.RunProver {
	return func(ctx context.Context, name string, request []byte) ([]byte, int, error) {
		out, err := cmd.Spawn(ctx, jobadapter.RunRequest{
			ID:   strings.TrimSuffix(name, filepath.Ext(name)),
			Type: jobadapter.ProofTypeForName(name),
			Body: request,
		})
		if err != nil {
			return nil, 0, err
		}
		return out.Response, out.ExitCode, nil
	}
}

// stdStream is the --in / --out value for stdin / stdout.
const stdStream = "-"

// validProofTypes are the values accepted by --type.
var validProofTypes = map[backend.ProofType]bool{
	backend.ProofTypeL2Execution:       true,
	backend.ProofTypeRollup:            true,
	backend.ProofTypeRollupAggregation: true,
}

// runProve proves one request, writes the response, and exits with a code.
func runProve(args []string) error {
	fs := flag.NewFlagSet("prover prove", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the TOML config file (or set CONFIG_FILE)")
	inPath := fs.String("in", "", `request file to prove, or "-" for stdin (required)`)
	outPath := fs.String("out", "", `file to write the response to, or "-" for stdout (required)`)
	typeFlag := fs.String("type", "", `proof type (l2-execution, rollup, rollup-aggregation); `+
		`required with --in -, otherwise read from the request file name`)
	idFlag := fs.String("id", "", "job id used in logs and failure bodies; defaults to the request file name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inPath == "" || *outPath == "" {
		return fmt.Errorf("--in and --out are required")
	}

	// Reserve stdout for the response: the ZkC DEBUG instruction writes to
	// os.Stdout.
	respOut := os.Stdout
	if *outPath == stdStream {
		os.Stdout = os.Stderr
		defer func() { os.Stdout = respOut }()
	}

	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	proofType, id, err := requestIdentity(*inPath, *typeFlag, *idFlag)
	if err != nil {
		return err
	}
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

	body, err := readRequest(*inPath)
	if err != nil {
		return err
	}

	result := runner.Run(context.Background(),
		jobadapter.RunRequest{ID: id, Type: proofType, Body: body})

	data, err := json.MarshalIndent(result.ResponseBody, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding response: %w", err)
	}
	if err := writeResponse(*outPath, respOut, data); err != nil {
		return err
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

// requestIdentity returns the proof type and job id from the flags, or from the
// request file name.
func requestIdentity(inPath, typeFlag, idFlag string) (backend.ProofType, string, error) {
	var proofType backend.ProofType
	switch {
	case typeFlag != "":
		proofType = backend.ProofType(typeFlag)
		if !validProofTypes[proofType] {
			return "", "", fmt.Errorf("invalid --type %q", typeFlag)
		}
	case inPath == stdStream:
		return "", "", fmt.Errorf("--type is required when the request is read from stdin")
	default:
		proofType = jobadapter.ProofTypeForName(filepath.Base(inPath))
	}

	id := idFlag
	if id == "" && inPath != stdStream {
		name := filepath.Base(inPath)
		id = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if id == "" {
		id = "stdin"
	}
	return proofType, id, nil
}

func readRequest(inPath string) ([]byte, error) {
	if inPath == stdStream {
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading request from stdin: %w", err)
		}
		return body, nil
	}
	body, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("reading request file: %w", err)
	}
	return body, nil
}

func writeResponse(outPath string, stdout io.Writer, data []byte) error {
	if outPath == stdStream {
		if _, err := stdout.Write(data); err != nil {
			return fmt.Errorf("writing response to stdout: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("writing response file: %w", err)
	}
	return nil
}

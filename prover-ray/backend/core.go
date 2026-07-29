package backend

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// ErrNotImplemented is returned by stubs that are not yet wired up.
var ErrNotImplemented = errors.New("not yet implemented")

// wiopSystemName names the wiop constraint system built in [New].
const wiopSystemName = "lineth-riscv"

// Core is the shared proving kernel. Initialize once via [New]; it is
// safe for concurrent use after that; each [Prove] call gets its own
// wiop.Runtime.
type Core struct {
	cfg    Config
	sys    *wiop.System
	driver *zkcdriver.ZkCDriver
	elf    elfInputs // guest ELF sections + entry point, extracted once in New; reused per job
}

// New loads the circuit binary and the guest ELF, calls [zkcdriver.NewZkCDriver]
// to define all columns and constraints, and returns a [Core] ready to prove.
//
// Compiler passes (rangecheck → lookup → logderiv → localvanishing → global)
// and wiop.Materialize are not yet wired. They must be added before the
// system can produce sound proofs (see wiki backend-overview.md §4).
func New(cfg Config) (*Core, error) {
	binFile, err := os.Open(cfg.CircuitBinPath)
	if err != nil {
		return nil, fmt.Errorf("opening circuit bin %q: %w", cfg.CircuitBinPath, err)
	}
	defer binFile.Close()

	elfFile, err := os.Open(cfg.GuestELFPath)
	if err != nil {
		return nil, fmt.Errorf("opening guest ELF %q: %w", cfg.GuestELFPath, err)
	}
	defer elfFile.Close()

	parsedELF, err := loadELFInputs(elfFile)
	if err != nil {
		return nil, fmt.Errorf("extracting ELF blobs from %q: %w", cfg.GuestELFPath, err)
	}

	sys := wiop.NewSystemf(wiopSystemName)
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, binFile)

	// Compiler passes go here once the real RISC-V .bin is fully supported:
	//   compilers.RangeCheck(sys)
	//   compilers.LookupToLogDerivSum(sys)
	//   compilers.LogDerivativeSum(sys)
	//   compilers.LocalVanishing(sys)
	//   compilers.Global(sys)
	//   wiop.Materialize(sys)

	return &Core{
		cfg:    cfg,
		sys:    sys,
		driver: driver,
		elf:    parsedELF,
	}, nil
}

// Prove runs a single [Job] end-to-end and returns its [Result].
func (c *Core) Prove(ctx context.Context, job Job) Result {
	inputs, err := c.buildInputs(job)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("building inputs: %w", err))
	}

	proof, pub, err := c.runProve(ctx, &zkcdriver.PreReadInputs{Inputs: inputs})
	if err != nil {
		return failResult(job.ID, err)
	}

	proofBytes, err := SerializeProof(proof, pub)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("serializing proof: %w", err))
	}

	return Result{
		JobID:      job.ID,
		Status:     ResultStatusOK,
		ProofBytes: proofBytes,
	}
}

// buildInputs converts a Job's Payload into the three ZkC pub-input values,
// keyed by name (see [encodeInputs]). ELF memory blobs are pre-extracted in
// [New] and reused across calls; only the per-job StatelessInput blobs
// (schema id + SSZ body) differ.
func (c *Core) buildInputs(job Job) (map[string][]byte, error) {
	if err := sanityCheckJobs(job); err != nil {
		return nil, err
	}

	sszMemBlobs, err := sszBlobs(c.cfg.inOrigin(), decodePayload(job))
	if err != nil {
		return nil, err
	}
	memBlobs := make([]memoryBlob, 0, len(c.elf.blobs)+2)
	memBlobs = append(memBlobs, c.elf.blobs...)
	memBlobs = append(memBlobs, sszMemBlobs...)
	return encodeInputs(memBlobs, c.elf.entry), nil
}

func sanityCheckJobs(job Job) error {
	// Inverted ranges are malformed input.
	if job.EndBlock < job.StartBlock {
		return fmt.Errorf("invalid block range [%d, %d]: EndBlock < StartBlock",
			job.StartBlock, job.EndBlock)
	}

	// Multi-block SSZ conflation is not implemented yet.
	if job.EndBlock > job.StartBlock {
		return fmt.Errorf("multi-block job [%d, %d]: %w",
			job.StartBlock, job.EndBlock, ErrNotImplemented)
	}

	return nil
}

// runProve calls AssignWithPreRead, sys.Prove, and sys.Verify.
func (c *Core) runProve(
	ctx context.Context,
	preRead *zkcdriver.PreReadInputs,
) (wiop.Proof, wiop.PublicInput, error) {
	_ = ctx // cancellation not yet propagated into the prover internals

	proof, pub := c.sys.Prove(func(rt *wiop.Runtime) {
		c.driver.AssignWithPreRead(rt, preRead)
	})

	if err := c.sys.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("proof verification: %w", err)
	}

	return proof, pub, nil
}

// SerializeProof encodes a wiop.Proof into the wire bytes the coordinator
// expects in the "proof" field of the response.
//
// Wire format not yet decided (wiki backend-overview.md §6).
func SerializeProof(_ wiop.Proof, _ wiop.PublicInput) ([]byte, error) {
	return nil, fmt.Errorf("SerializeProof: %w", ErrNotImplemented)
}

// decodePayload extracts the raw SSZ bytes from a Job's Payload.
// Today it is a pass-through; once the coordinator API encoding is finalized
// this will handle any wrapping (JSON envelope, multi-block conflation, etc.).
func decodePayload(job Job) []byte {
	return job.Payload
}

func failResult(jobID string, err error) Result {
	return Result{JobID: jobID, Status: ResultStatusFailed, Err: err}
}

package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// ErrNotImplemented is returned by stubs that are not yet wired up.
var ErrNotImplemented = errors.New("not yet implemented")

// Core is the shared proving kernel. Initialize once via [New]; it is
// safe for concurrent use after that — each [Prove] call gets its own
// wiop.Runtime.
type Core struct {
	cfg    Config
	sys    *wiop.System
	driver *zkcdriver.ZkCDriver
	elf    []byte
}

// New loads the circuit binary and the guest ELF, calls [zkcdriver.NewZkCDriver]
// to define all columns and constraints, and returns a [Core] ready to prove.
//
// Compiler passes (rangecheck → lookup → logderiv → localvanishing → global)
// and wiop.Materialize are not yet wired — they must be added before the
// system can produce cryptographically sound proofs (see backend-overview.md §4).
func New(cfg Config) (*Core, error) {
	binBytes, err := os.ReadFile(cfg.CircuitBinPath)
	if err != nil {
		return nil, fmt.Errorf("reading circuit bin %q: %w", cfg.CircuitBinPath, err)
	}

	elfBytes, err := os.ReadFile(cfg.GuestELFPath)
	if err != nil {
		return nil, fmt.Errorf("reading guest ELF %q: %w", cfg.GuestELFPath, err)
	}

	sys := wiop.NewSystemf("linea-riscv")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(binBytes))

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
		elf:    elfBytes,
	}, nil
}

// Prove runs a single [Job] end-to-end and returns its [Result].
func (c *Core) Prove(ctx context.Context, job Job) Result {
	inputs, err := c.buildInputs(job)
	if err != nil {
		return failResult(job.ID, err)
	}

	proof, pub, err := c.runProve(ctx, zkcdriver.PreReadInputs{Inputs: inputs})
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

// buildInputs converts a Job's Payload into the three ZkC pub-input blobs.
func (c *Core) buildInputs(job Job) (map[string][]byte, error) {
	return BuildZkcInputs(c.elf, decodePayload(job), c.cfg.inOrigin())
}

// runProve calls AssignWithPreRead, sys.Prove, and sys.Verify.
func (c *Core) runProve(
	ctx context.Context,
	preRead zkcdriver.PreReadInputs,
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

// EncodeStatelessInput SSZ-encodes the coordinator's per-block payload into
// the byte slice the guest reads at _in_start. The [u64 LE len] frame is
// added later by [BuildZkcInputs]; this returns raw SSZ only.
//
// Not yet implemented. Reference codec: arithmetization/proof_io_v1.py.
func EncodeStatelessInput(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("EncodeStatelessInput: %w", ErrNotImplemented)
}

// SerializeProof encodes a wiop.Proof into the wire bytes the coordinator
// expects in the "proof" field of the response.
//
// Wire format not yet decided (backend-overview.md §6).
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

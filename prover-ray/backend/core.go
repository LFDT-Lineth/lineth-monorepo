package backend

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/sirupsen/logrus"
)

// ErrNotImplemented is returned by stubs that are not yet wired up.
var ErrNotImplemented = errors.New("not yet implemented")

// wiopSystemName names the wiop constraint system built in [New].
const wiopSystemName = "lineth-riscv"

// guestOutputMemory is the ZkC Execute output-map key carrying the guest's
// wire output.
const guestOutputMemory = "guest_output"

// guestOutputSize is the fixed 0x0003 wire-output length: a 2-byte schema id
// plus a 32-byte keccak256(SSZ(public inputs)).
const guestOutputSize = 34

// Core is the shared proving kernel. Initialize once via [New]; it is
// safe for concurrent use after that; each [Prove] call gets its own
// wiop.Runtime.
type Core struct {
	cfg     Config
	mode    ProverMode
	sys     *wiop.System
	driver  *zkcdriver.ZkCDriver
	program elfmapping.Program
	decoded predecoding.DecodedProgram

	// binaryFile is the compiled R5 arithmetization, used by dev-zkvm to run the
	// guest under ZkC Execute (the no-trace path). nil outside dev-zkvm.
	binaryFile *zkcdriver.BinaryFile
}

// New loads the circuit binary and the guest ELF, calls [zkcdriver.NewZkCDriver]
// to define all columns and constraints, and returns a [Core] ready to prove.
//
// Compiler passes (rangecheck → lookup → logderiv → localvanishing → global)
// and wiop.Materialize are not yet wired. They must be added before the
// system can produce sound proofs.
func New(cfg Config) (*Core, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = ProverModeFull
	}
	if !mode.Valid() {
		return nil, fmt.Errorf("invalid prover mode %q", cfg.Mode)
	}

	// dev-mock runs no guest, so it loads no circuit bin or guest ELF.
	if !mode.needsArtifacts() {
		return &Core{cfg: cfg, mode: mode}, nil
	}

	// dev-zkvm runs the guest under ZkC Execute (no trace, no AIR), so it needs
	// the compiled arithmetization and the guest ELF but not the wiop system,
	// driver, or public-output columns the proving modes build.
	if mode == ProverModeDevZkVM {
		return newDevZkVM(cfg, mode)
	}

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

	program, err := elfmapping.Load(elfFile)
	if err != nil {
		return nil, fmt.Errorf("extracting ELF blobs from %q: %w", cfg.GuestELFPath, err)
	}
	decoded, err := predecoding.Predecode(program)
	if err != nil {
		return nil, fmt.Errorf("predecoding guest ELF %q: %w", cfg.GuestELFPath, err)
	}

	sys := wiop.NewSystemf(wiopSystemName)
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, binFile)

	// Must run after the arithmetization is defined, so the guest_output columns
	// exist, and before the compiler passes, which discharge the openings it
	// registers.
	risc5.RegisterGuestPublicOutputs(sys)

	// Compiler passes go here once the real RISC-V .bin is fully supported:
	//   compilers.RangeCheck(sys)
	//   compilers.LookupToLogDerivSum(sys)
	//   compilers.LogDerivativeSum(sys)
	//   compilers.LocalVanishing(sys)
	//   compilers.Global(sys)
	//   wiop.Materialize(sys)

	return &Core{
		cfg:     cfg,
		mode:    mode,
		sys:     sys,
		driver:  driver,
		program: program,
		decoded: decoded,
	}, nil
}

// newDevZkVM builds an Execute-only Core: the compiled R5 arithmetization plus
// the predecoded guest ELF, without the wiop system, driver, or AIR the proving
// modes construct. dev-zkvm uses [zkcdriver.BinaryFile.Execute], the no-trace
// path, so those are unnecessary.
func newDevZkVM(cfg Config, mode ProverMode) (*Core, error) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		return nil, fmt.Errorf("compiling R5 arithmetization: %w", err)
	}

	elfFile, err := os.Open(cfg.GuestELFPath)
	if err != nil {
		return nil, fmt.Errorf("opening guest ELF %q: %w", cfg.GuestELFPath, err)
	}
	defer elfFile.Close()

	program, err := elfmapping.Load(elfFile)
	if err != nil {
		return nil, fmt.Errorf("extracting ELF blobs from %q: %w", cfg.GuestELFPath, err)
	}
	decoded, err := predecoding.Predecode(program)
	if err != nil {
		return nil, fmt.Errorf("predecoding guest ELF %q: %w", cfg.GuestELFPath, err)
	}

	return &Core{
		cfg:        cfg,
		mode:       mode,
		program:    program,
		decoded:    decoded,
		binaryFile: binf,
	}, nil
}

// Prove runs a single [Job] and returns its [Result]. Each mode is dispatched
// here; modes that cannot run yet return their blocker error.
func (c *Core) Prove(ctx context.Context, job Job) Result {
	switch c.mode {
	case ProverModeDevMock:
		return c.proveDevMock(job)
	case ProverModeDevZkVM:
		return c.proveDevZkVM(job)
	case ProverModePartial:
		return failResult(job.ID, fmt.Errorf("partial mode not runnable yet, memory-gated: %w", ErrNotImplemented))
	case ProverModeFull:
		return c.proveFull(ctx, job)
	default:
		return failResult(job.ID, fmt.Errorf("unknown prover mode %q: %w", c.mode, ErrNotImplemented))
	}
}

// proveDevMock returns a placeholder result: success, a marker proof, zero
// public inputs. It runs no guest and ignores Payload.
func (c *Core) proveDevMock(job Job) Result {
	return Result{
		JobID:      job.ID,
		Status:     ResultStatusOK,
		ProofBytes: DevMarkerProof(ProverModeDevMock),
	}
}

// proveDevZkVM runs the guest under ZkC Execute and returns its 0x0003 wire
// output as [Result.ProofBytes], so the runner can cross-check it against the
// native runner. It runs no real proof; Payload is the whole extended (0x0002)
// input, the same bytes the native runner consumes.
func (c *Core) proveDevZkVM(job Job) Result {
	inputs, err := c.encodeGuestInputs(job)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("building inputs: %w", err))
	}

	guestOutput, err := executeGuest(c.binaryFile, inputs)
	if err != nil {
		return failResult(job.ID, err)
	}

	return Result{
		JobID:      job.ID,
		Status:     ResultStatusOK,
		ProofBytes: guestOutput,
	}
}

// executeGuest runs the guest under ZkC Execute (the no-trace path) and returns
// its validated 0x0003 wire output.
func executeGuest(binf *zkcdriver.BinaryFile, inputs map[string][]byte) ([]byte, error) {
	output, errs := binf.Execute(inputs)
	return classifyGuestOutput(output, errs)
}

// classifyGuestOutput turns a ZkC Execute result into either the guest's
// validated 0x0003 output or an error that distinguishes a guest block rejection
// (a recognized [vm.Failure], the guest's own exit on an invalid block) from an
// internal VM failure (a prover-side bug). A guest that rejects the block writes
// no output, so a missing or malformed guest_output is also a rejection.
func classifyGuestOutput(output map[string][]byte, errs []error) ([]byte, error) {
	if len(errs) > 0 {
		var failure *vm.Failure
		if errors.As(errors.Join(errs...), &failure) {
			return nil, fmt.Errorf("guest rejected the block (invalid): %s", failure.Message)
		}
		return nil, fmt.Errorf("guest VM execution failed: %w", errors.Join(errs...))
	}

	out, ok := output[guestOutputMemory]
	if !ok {
		return nil, fmt.Errorf("guest wrote no %s output (block likely invalid)", guestOutputMemory)
	}
	if len(out) != guestOutputSize {
		return nil, fmt.Errorf("guest output is %d bytes, want %d", len(out), guestOutputSize)
	}
	if out[0] != 0x00 || out[1] != 0x03 {
		return nil, fmt.Errorf("guest output schema id is 0x%04x, want 0x0003", uint16(out[0])<<8|uint16(out[1]))
	}
	return out, nil
}

// proveFull is the real proving path; it is blocked at SerializeProof today.
func (c *Core) proveFull(ctx context.Context, job Job) Result {
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

// buildInputs converts a Job's Payload into the guest and memory inputs ZkC
// expects. ELF mapping and predecoding are cached by [New]; only the per-job
// guest input data differs.
func (c *Core) buildInputs(job Job) (map[string][]byte, error) {
	if err := sanityCheckJobs(job); err != nil {
		return nil, err
	}
	return c.encodeGuestInputs(job)
}

// encodeGuestInputs converts a Job's Payload into the guest and memory inputs
// ZkC expects, without the proving-path single-block check. dev-zkvm passes the
// whole extended (0x0002) input and lets the guest conflate the range; a
// conflation disagreement surfaces as a cross-check failure, not silent
// corruption.
func (c *Core) encodeGuestInputs(job Job) (map[string][]byte, error) {
	dataBlobs, err := elfmapping.NewData(
		elfmapping.DefaultInputOrigin,
		decodePayload(job),
		elfmapping.WithLengthPrefix(),
	)
	if err != nil {
		return nil, fmt.Errorf("building data section: %w", err)
	}
	inputs, err := elfmapping.EncodeInputs(c.program, dataBlobs)
	if err != nil {
		return nil, err
	}
	maps.Copy(inputs, c.decoded.EncodeInputs())
	return inputs, nil
}

func sanityCheckJobs(job Job) error {
	// Inverted ranges are malformed input.
	if job.EndBlock < job.StartBlock {
		return fmt.Errorf("invalid block range [%d, %d]: EndBlock < StartBlock",
			job.StartBlock, job.EndBlock)
	}

	// Multi-block guest-input conflation is not implemented yet.
	if job.EndBlock > job.StartBlock {
		return fmt.Errorf("multi-block job [%d, %d]: %w",
			job.StartBlock, job.EndBlock, ErrNotImplemented)
	}

	return nil
}

// runProve traces the inputs, assigns the shard, then runs sys.Prove and sys.Verify.
func (c *Core) runProve(
	ctx context.Context,
	preRead *zkcdriver.PreReadInputs,
) (wiop.Proof, wiop.PublicInput, error) {
	_ = ctx // cancellation not yet propagated into the prover internals

	traces := c.driver.TraceZkcInputs(preRead)
	if len(traces) > 1 {
		logrus.Fatalf("expected a single public input")
	}

	proof, pub := c.sys.Prove(func(rt *wiop.Runtime) {
		c.driver.AssignTraceShard(rt, traces[0], field.Octuplet{})
	})

	if err := c.sys.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("proof verification: %w", err)
	}

	return proof, pub, nil
}

// SerializeProof encodes a wiop.Proof into the wire bytes the coordinator
// expects in the "proof" field of the response.
//
// Wire format not yet decided.
func SerializeProof(_ wiop.Proof, _ wiop.PublicInput) ([]byte, error) {
	return nil, fmt.Errorf("SerializeProof: %w", ErrNotImplemented)
}

// decodePayload extracts the guest input bytes from a Job's Payload.
// Today it is a pass-through; request adapters already do protocol-specific
// encoding before constructing Jobs.
func decodePayload(job Job) []byte {
	return job.Payload
}

func failResult(jobID string, err error) Result {
	return Result{JobID: jobID, Status: ResultStatusFailed, Err: err}
}

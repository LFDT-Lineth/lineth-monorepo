package backend

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// ErrNotImplemented is returned by stubs that are not yet wired up.
var ErrNotImplemented = errors.New("not yet implemented")

// wiopSystemName names the wiop constraint system built in [New].
const wiopSystemName = "lineth-riscv"

// guestOutputMemory is the ZkC Execute output-map key carrying the guest's
// wire output.
const guestOutputMemory = "guest_output"

// Spec frame of the l2-execution guest output.
const (
	guestOutputSchemaID     = 0x0003
	guestOutputSchemaIDSize = 2
	guestOutputHashSize     = 32
)

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

	// round 0 is reserved for the idential data, variable data per shard goes to round 1
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{TraceRound: 1}, binFile)

	// Must run after the arithmetization is defined, so the guest_output columns
	// and the program round exist, and before the compiler passes, which
	// discharge the guest-output openings and commit the program round that the
	// program VK cells read.
	risc5.RegisterPublicInputs(sys)

	// Shared randomness is off: no preflight computes γ yet.
	if err := compilers.CompileFull(sys,
		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
	); err != nil {
		return nil, fmt.Errorf("compiling the wiop system: %w", err)
	}

	risc5.CheckPublicInputs(sys)

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

// proveDevMock returns a placeholder result: success, a marker proof, and zero
// public inputs except EndBlockNumber, which the coordinator needs to identify
// the proven batch. It runs no guest and ignores Payload.
func (c *Core) proveDevMock(job Job) Result {
	return Result{
		JobID:        job.ID,
		Status:       ResultStatusOK,
		ProofBytes:   DevMarkerProof(ProverModeDevMock),
		PublicInputs: PublicInputs{EndBlockNumber: job.EndBlock},
	}
}

// proveDevZkVM runs the guest under ZkC Execute and returns its 0x0003 output
// as [Result.ProofBytes], so the runner can cross-check it against the native
// runner. It runs no real proof; Payload is the whole extended (0x0002)
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
		ProofBytes: guestOutput.Bytes(),
	}
}

// executeGuest runs the guest under ZkC Execute (the no-trace path) and returns
// its validated 0x0003 output.
func executeGuest(binf *zkcdriver.BinaryFile, inputs map[string]bit.Array) (bit.Array, error) {
	output, errs := binf.Execute(inputs)
	return classifyGuestOutput(output, errs)
}

// classifyGuestOutput turns a ZkC Execute result into either the guest's
// validated 0x0003 output or an error that distinguishes a guest block rejection
// (a recognized [vm.Failure], the guest's own exit on an invalid block) from an
// internal VM failure (a prover-side bug). A guest that rejects the block writes
// no output, so a missing guest_output is also a rejection.
func classifyGuestOutput(output map[string]bit.Array, errs []error) (res bit.Array, err error) {
	if len(errs) > 0 {
		var failure *vm.Failure
		if errors.As(errors.Join(errs...), &failure) {
			return res, fmt.Errorf("guest rejected the block (invalid): %s", failure.Message)
		}
		return res, fmt.Errorf("guest VM execution failed: %w", errors.Join(errs...))
	}

	out, ok := output[guestOutputMemory]
	if !ok {
		return res, fmt.Errorf("guest wrote no %s output (block likely invalid)", guestOutputMemory)
	}
	if err = checkGuestOutputFrame(out); err != nil {
		return res, fmt.Errorf("%s does not follow the spec: %w", guestOutputMemory, err)
	}
	return out, nil
}

// checkGuestOutputFrame checks the spec frame of the guest output,
// keccak256(0x0003 || public inputs) || 0x0003 || public inputs. The public
// inputs themselves are checked by the byte-for-byte comparison with the
// native runner.
func checkGuestOutputFrame(out bit.Array) error {
	var (
		outBytes = out.Bytes()
	)
	if len(outBytes) >= guestOutputHashSize+guestOutputSchemaIDSize {
		framed := outBytes[guestOutputHashSize:]
		if bytes.Equal(outBytes[:guestOutputHashSize], keccak256(framed)) {
			if got := binary.BigEndian.Uint16(framed); got != guestOutputSchemaID {
				return fmt.Errorf("schema id is 0x%04x, want 0x%04x", got, guestOutputSchemaID)
			}
			return nil
		}
	}
	if hasPreSpecGuestOutputLayout(outBytes) {
		return errors.New("got 0x0003 || public inputs || keccak256(public inputs), " +
			"the spec expects keccak256(0x0003 || public inputs) || 0x0003 || public inputs")
	}
	if len(outBytes) < guestOutputHashSize+guestOutputSchemaIDSize {
		return fmt.Errorf("%d bytes, shorter than the hash and schema id", len(outBytes))
	}
	return errors.New("hash does not match keccak256(0x0003 || public inputs)")
}

// hasPreSpecGuestOutputLayout reports whether out is 0x0003 || x || keccak256(x),
// the layout the guest wrote before the spec put the hash first.
func hasPreSpecGuestOutputLayout(out []byte) bool {
	if len(out) < guestOutputSchemaIDSize+guestOutputHashSize ||
		binary.BigEndian.Uint16(out) != guestOutputSchemaID {
		return false
	}
	body := out[guestOutputSchemaIDSize : len(out)-guestOutputHashSize]
	return bytes.Equal(out[len(out)-guestOutputHashSize:], keccak256(body))
}

func keccak256(b []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)
	return h.Sum(nil)
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
	//nolint (until SerializeProof implemented)
	proofBytes, err := SerializeProof(proof, pub)
	//nolint (whilst SerializeProof unimplemented)
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
func (c *Core) buildInputs(job Job) (map[string]bit.Array, error) {
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
func (c *Core) encodeGuestInputs(job Job) (map[string]bit.Array, error) {
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

	lazyTrace := c.driver.TraceZkcInputs(preRead)
	// Assert exactly one shard (for now)
	if lazyTrace.Len() != 1 {
		logrus.Fatalf("expected a single public input")
	}
	// Materalise first shard
	shard, errs := lazyTrace.Get(0)
	// Sanity check for tracing errors
	if len(errs) > 0 {
		logrus.Fatalf("tracing failure: %v", errors.Join(errs...))
	}
	// Generate proof
	proof, pub := c.sys.Prove(func(rt *wiop.Runtime) {
		c.driver.AssignTraceShard(rt, shard.Unwrap(), field.Octuplet{})
	})
	// Verify proof
	if err := c.sys.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("proof verification: %w", err)
	}
	// Done
	return proof, pub, nil
}

// SerializeProof encodes a wiop.Proof into the wire bytes the coordinator
// expects in the "proof" field of the response.
//
// Wire format not yet decided.
// nolint
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

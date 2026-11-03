package backend

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

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

// guestOutputMemory and guestLogMemory are the ZkC Execute output-map keys
// carrying the guest's wire output: the hash in the former, the schema id and
// public inputs in the latter.
const (
	guestOutputMemory = "guest_output"
	guestLogMemory    = "guest_log"
)

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
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{TraceRound: 1, ShardSize: cfg.ShardSize}, binFile)

	// Must run after the arithmetization is defined, so the guest_output columns
	// and the program round exist, and before the compiler passes, which
	// discharge the guest-output openings and commit the program round that the
	// program VK cells read.
	risc5.RegisterPublicInputs(sys)

	// Shared randomness is off: no preflight computes γ yet. Unsound: the bus
	// coins then depend on round 0 only, not on the trace.
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

// checkShardIndexes returns a sorted copy of the shard indexes, rejecting
// duplicates and, without sharding (shardSize zero), any shard other than 0.
func checkShardIndexes(indexes []uint, shardSize uint64) ([]uint, error) {
	indexes = slices.Sorted(slices.Values(indexes))
	for k, i := range indexes {
		if k > 0 && indexes[k-1] == i {
			return nil, fmt.Errorf("duplicate shard index %d", i)
		}
		if shardSize == 0 && i != 0 {
			return nil, fmt.Errorf("shard index %d requires a non-zero shard size", i)
		}
	}
	return indexes, nil
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
// no output, so a missing guest_output is also a rejection. The guest_output
// must be exactly the hash, the only bytes the proof binds, and the guest_log
// the schema id and public inputs it commits to.
func classifyGuestOutput(output map[string]bit.Array, errs []error) (res bit.Array, err error) {
	if len(errs) > 0 {
		return res, guestError(errors.Join(errs...))
	}

	out, ok := output[guestOutputMemory]
	if !ok {
		return res, fmt.Errorf("guest wrote no %s output (block likely invalid)", guestOutputMemory)
	}
	if n := len(out.Bytes()); n != guestOutputHashSize {
		return res, fmt.Errorf("guest wrote %d bytes to %s, want the %d-byte hash", n, guestOutputMemory, guestOutputHashSize)
	}
	log, ok := output[guestLogMemory]
	if !ok {
		return res, fmt.Errorf("guest wrote the hash but no %s (schema id and public inputs)", guestLogMemory)
	}
	if err = checkGuestOutputFrame(out.Bytes(), log.Bytes()); err != nil {
		return res, fmt.Errorf("%s does not follow the spec: %w", guestOutputMemory, err)
	}
	frame := slices.Concat(out.Bytes(), log.Bytes())
	return bit.NewArray(frame, uint(len(frame)*8), bit.BigEndian), nil
}

// guestError distinguishes a guest block rejection (a recognized [vm.Failure],
// the guest's own exit on an invalid block) from an internal VM failure (a
// prover-side bug) in the errors of a ZkC execution or tracing.
func guestError(err error) error {
	var failure *vm.Failure
	if errors.As(err, &failure) {
		return fmt.Errorf("guest rejected the block (invalid): %s", failure.Message)
	}
	return fmt.Errorf("guest VM execution failed: %w", err)
}

// checkGuestOutputFrame checks the spec frame of the guest output: hash is
// keccak256(0x0003 || public inputs) and framed is 0x0003 || public inputs. The
// public inputs themselves are checked by the byte-for-byte comparison with the
// native runner.
func checkGuestOutputFrame(hash, framed []byte) error {
	if len(framed) < guestOutputSchemaIDSize {
		return fmt.Errorf("%d bytes after the hash, shorter than the schema id", len(framed))
	}
	if !bytes.Equal(hash, keccak256(framed)) {
		return errors.New("hash does not match keccak256(0x0003 || public inputs)")
	}
	if got := binary.BigEndian.Uint16(framed); got != guestOutputSchemaID {
		return fmt.Errorf("schema id is 0x%04x, want 0x%04x", got, guestOutputSchemaID)
	}
	return nil
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

	nodes, numShards, err := c.runProve(ctx, &zkcdriver.PreReadInputs{Inputs: inputs}, job.ShardIndexes)
	if err != nil {
		return failResult(job.ID, err)
	}
	// Conglomerating shard proofs into one proof is not implemented yet. Only
	// an unsharded trace, whose single shard proof is the final proof, is
	// supported: with sharding the in-shard message-bus check is off, so even a
	// one-shard trace is not proved until conglomerated.
	if c.cfg.ShardSize > 0 {
		return failResult(job.ID, fmt.Errorf("conglomerating %d of %d shard proofs: %w", len(nodes), numShards, ErrNotImplemented))
	}
	//nolint (until SerializeProof implemented)
	proofBytes, err := SerializeProof(nodes[0].Proof, nodes[0].Pub)
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
	if err := sanityCheckJobs(job, c.cfg.ShardSize); err != nil {
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

func sanityCheckJobs(job Job, shardSize uint64) error {
	if _, err := checkShardIndexes(job.ShardIndexes, shardSize); err != nil {
		return fmt.Errorf("shard indexes: %w", err)
	}

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

// Node is a proof over a set of trace shards: a shard proof, whose Coverage is
// its single shard index. Conglomeration will aggregate nodes into nodes
// covering the union of their children's, with the same public-input layout.
type Node struct {
	Coverage []uint
	Proof    wiop.Proof
	Pub      wiop.PublicInput
}

// runProve lazily traces the inputs, then proves and verifies the shards of
// shardIndexes (all of them if empty) one at a time, so that a single shard is
// materialised at any moment rather than the whole trace. It returns the
// shard proofs, in shard order, and the trace's shard count. With sharding,
// the in-shard message-bus check is off and the bus balances only across
// shards: the shard proofs are then not a proof of the execution until
// conglomerated.
func (c *Core) runProve(
	ctx context.Context,
	preRead *zkcdriver.PreReadInputs,
	shardIndexes []uint,
) ([]Node, uint, error) {

	// With sharding, this only runs the fast-mode pass recording one checkpoint
	// per shard.
	lazyTrace, err := c.driver.TryTraceZkcInputs(preRead)
	if err != nil {
		return nil, 0, fmt.Errorf("tracing: %w", guestError(err))
	}
	numShards := lazyTrace.Len()
	indexes := slices.Sorted(slices.Values(shardIndexes))
	if len(indexes) == 0 {
		indexes = make([]uint, numShards)
		for i := range indexes {
			indexes[i] = uint(i)
		}
	}
	if len(indexes) == 0 {
		return nil, 0, errors.New("the trace has no shards")
	}
	if last := indexes[len(indexes)-1]; last >= numShards {
		return nil, 0, fmt.Errorf("shard index %d out of range, the trace has %d shards", last, numShards)
	}
	if len(indexes) != int(numShards) {
		logrus.Infof("Proving %d of %d shards", len(indexes), numShards)
	}

	nodes := make([]Node, 0, len(indexes))
	for k, i := range indexes {
		if err := ctx.Err(); err != nil {
			return nil, 0, fmt.Errorf("before shard %d: %w", i, err)
		}
		shard, err := traceShard(lazyTrace, i)
		if err != nil {
			return nil, 0, err
		}
		// Once the last shard is traced, drop the lazy trace so that its
		// checkpoints can be garbage collected before that shard is proved.
		if k == len(indexes)-1 {
			lazyTrace = zkcdriver.LazyTrace{}
		}
		proof, pub, err := c.proveShard(shard)
		if err != nil {
			return nil, 0, fmt.Errorf("shard %d: %w", i, err)
		}
		nodes = append(nodes, Node{Coverage: []uint{i}, Proof: proof, Pub: pub})
	}
	return nodes, numShards, nil
}

// traceShard traces and expands the ith shard of the lazy trace.
func traceShard(lazyTrace zkcdriver.LazyTrace, i uint) (zkcdriver.Shard, error) {
	shard, errs := lazyTrace.Get(i)
	if len(errs) > 0 {
		return zkcdriver.Shard{}, fmt.Errorf("tracing shard %d: %w", i, errors.Join(errs...))
	}
	if shard.IsEmpty() {
		return zkcdriver.Shard{}, fmt.Errorf("tracing shard %d: no shard produced", i)
	}
	return shard.Unwrap(), nil
}

// proveShard proves the given shard, then verifies the proof.
func (c *Core) proveShard(shard zkcdriver.Shard) (wiop.Proof, wiop.PublicInput, error) {
	proof, pub := c.sys.Prove(func(rt *wiop.Runtime) {
		c.driver.AssignTraceShard(rt, shard, field.Octuplet{})
	})
	// Verify proof
	if err := c.sys.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("proof verification: %w", err)
	}
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

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
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/guests"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/conglomeration"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
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
	cfg    Config
	mode   ProverMode
	sys    *wiop.System
	driver *zkcdriver.ZkCDriver
	guests guests.Store

	// merger runs the conglomeration steps over the shard proofs. nil in
	// full until the recursion machine lands; tests set it, and dev-partial uses
	// [conglomeration.MockMerger].
	merger conglomeration.Merger

	// binaryFile is the compiled R5 arithmetization, used by dev-zkvm to run the
	// guest under ZkC Execute (the no-trace path). nil outside dev-zkvm.
	binaryFile *zkcdriver.BinaryFile

	// devSys is the devmode system whose proof dev-partial returns. nil outside
	// dev-partial.
	devSys *wiop.System
}

// guestProgram is a guest ELF mapped and predecoded for ZkC.
type guestProgram struct {
	program elfmapping.Program
	decoded predecoding.DecodedProgram
}

// parseGuest maps and predecodes a guest ELF.
func parseGuest(elf []byte) (guestProgram, error) {
	program, err := elfmapping.Load(bytes.NewReader(elf))
	if err != nil {
		return guestProgram{}, fmt.Errorf("extracting ELF blobs: %w", err)
	}
	decoded, err := predecoding.Predecode(program)
	if err != nil {
		return guestProgram{}, fmt.Errorf("predecoding guest ELF: %w", err)
	}
	return guestProgram{program: program, decoded: decoded}, nil
}

// loadGuest loads the guest ELF whose Program ID is id from the guests folder.
func (c *Core) loadGuest(id [32]byte) (guestProgram, error) {
	elf, err := c.guests.Load(id)
	if err != nil {
		return guestProgram{}, err
	}
	g, err := parseGuest(elf)
	if err != nil {
		return guestProgram{}, fmt.Errorf("guest 0x%x: %w", id[:], err)
	}
	return g, nil
}

// New loads the circuit binary, calls [zkcdriver.NewZkCDriver] to define all
// columns and constraints, and returns a [Core] ready to prove. The guest ELF is
// loaded per job, from [Job.GuestProgramID].
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
	if cfg.GuestsDir == "" {
		return nil, fmt.Errorf("guests folder must be set for prover mode %q", mode)
	}

	// dev-zkvm runs the guest under ZkC Execute (no trace, no AIR), so it needs
	// the compiled arithmetization and the guest ELF but not the wiop system,
	// driver, or public-output columns the proving modes build.
	if mode == ProverModeDevZkVM {
		return newDevZkVM(cfg, mode)
	}
	if mode == ProverModeDevPartial {
		return newDevPartial(cfg, mode)
	}

	binFile, err := os.Open(cfg.CircuitBinPath)
	if err != nil {
		return nil, fmt.Errorf("opening circuit bin %q: %w", cfg.CircuitBinPath, err)
	}
	defer binFile.Close()

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
		cfg:    cfg,
		mode:   mode,
		sys:    sys,
		driver: driver,
		guests: guests.Store{Dir: cfg.GuestsDir},
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

// newDevZkVM builds an Execute-only Core: the compiled R5 arithmetization,
// without the wiop system, driver, or AIR the proving modes construct.
// dev-zkvm uses [zkcdriver.BinaryFile.Execute], the no-trace path, so those are
// unnecessary.
func newDevZkVM(cfg Config, mode ProverMode) (*Core, error) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		return nil, fmt.Errorf("compiling R5 arithmetization: %w", err)
	}

	return &Core{
		cfg:        cfg,
		mode:       mode,
		guests:     guests.Store{Dir: cfg.GuestsDir},
		binaryFile: binf,
	}, nil
}

// newDevPartial builds a check-only Core: the R5 arithmetization and its public
// inputs, without a PCS, so that proving a trace commits to nothing and checks
// it against every constraint. Its proofs are devmode proofs.
func newDevPartial(cfg Config, mode ProverMode) (*Core, error) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		return nil, fmt.Errorf("compiling R5 arithmetization: %w", err)
	}
	bin, err := binf.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encoding R5 arithmetization: %w", err)
	}

	sys := wiop.NewSystemf(wiopSystemName)
	sys.NewRound()
	if cfg.ShardSize == 0 {
		logrus.Warnf("dev-partial without shard_size: a real block does not fit in one shard")
	}
	settings := zkcdriver.Settings{TraceRound: 1, ShardSize: cfg.ShardSize}
	driver := zkcdriver.NewZkCDriver(sys, settings, bytes.NewReader(bin))
	risc5.RegisterPublicInputs(sys)
	if err := compilers.Configure(sys,
		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
		compilers.WithoutPCS(),
	); err != nil {
		return nil, fmt.Errorf("configuring the wiop system: %w", err)
	}
	// A message bus has no check of its own: reduce it to a grand product,
	// which has one. Every other query is checked as is. Without a PCS no
	// column data enters the transcript, so the bus coins are fixed and the
	// same on every shard: fine to check our own trace, not a proof.
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
	risc5.CheckPublicInputs(sys)

	devSys, err := devmode.NewSystem()
	if err != nil {
		return nil, err
	}
	return &Core{
		cfg:    cfg,
		mode:   mode,
		sys:    sys,
		driver: driver,
		guests: guests.Store{Dir: cfg.GuestsDir},
		// Without sharding the single shard checks its own message bus; with
		// it, the mock merger checks the bus across the shards.
		merger: conglomeration.NewMockMerger(sys),
		devSys: devSys,
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
	case ProverModeDevPartial:
		return c.proveDevPartial(ctx, job)
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
	g, err := c.loadGuest(job.GuestProgramID)
	if err != nil {
		return failResult(job.ID, err)
	}
	inputs, err := c.encodeGuestInputs(job, g)
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

// proveDevPartial runs the guest, checks its trace against the constraints, and
// returns the public inputs decoded from its output, with a devmode proof of
// the output hash in place of a real proof. Payload is the whole extended
// (0x0002) input, as in dev-zkvm.
func (c *Core) proveDevPartial(ctx context.Context, job Job) Result {
	g, err := c.loadGuest(job.GuestProgramID)
	if err != nil {
		return failResult(job.ID, err)
	}
	inputs, err := c.encodeGuestInputs(job, g)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("building inputs: %w", err))
	}

	// The lazy trace keeps no output, so Execute gives it. It also fails fast
	// on a rejected block, before the trace.
	guestOutput, err := executeGuest(c.driver.BinaryFile, inputs)
	if err != nil {
		return failResult(job.ID, err)
	}
	out := guestOutput.Bytes()
	pi, err := decodeGuestPublicInputs(out[guestOutputHashSize:])
	if err != nil {
		return failResult(job.ID, fmt.Errorf("decoding the guest public inputs: %w", err))
	}

	// Every shard, so that the result is the root.
	root, _, err := c.runProve(ctx, &zkcdriver.PreReadInputs{Inputs: inputs}, nil)
	if err != nil {
		return failResult(job.ID, err)
	}
	pub, err := c.devmodePublicInputs(root.Pub, out[:guestOutputHashSize])
	if err != nil {
		return failResult(job.ID, err)
	}
	proof, pub, err := devmode.Prove(c.devSys, pub)
	if err != nil {
		return failResult(job.ID, err)
	}
	proofBytes, err := devmode.Encode(c.devSys, proof, pub, devmode.GuestBase)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("encoding the devmode proof: %w", err))
	}

	return Result{
		JobID:        job.ID,
		Status:       ResultStatusOK,
		ProofBytes:   proofBytes,
		PublicInputs: pi,
		GuestOutput:  out,
	}
}

// checkAssignment proves the assignment, which commits to nothing but checks
// every query, then verifies the proof, which runs the verifier actions, such
// as the message-bus balance. It returns the proof and its public inputs.
func (c *Core) checkAssignment(assign func(*wiop.Runtime)) (proof wiop.Proof, pub wiop.PublicInput, err error) {
	// A failed query check panics.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("checking the trace: %v", r)
		}
	}()
	proof, pub = c.sys.Prove(assign, wiop.ProveOptions{CheckUnreducedQueries: true})
	if err := c.sys.Verify(proof, pub); err != nil {
		return wiop.Proof{}, nil, fmt.Errorf("checking the trace: %w", err)
	}
	return proof, pub, nil
}

// devmodePublicInputs returns the public inputs of the devmode proof from those
// of the trace: the guest output hash, which must be hash, then the program
// VK, zero as nothing is committed.
func (c *Core) devmodePublicInputs(pub wiop.PublicInput, hash []byte) (wiop.PublicInput, error) {
	out := make(wiop.PublicInput, 0, devmode.NumPublicInputs)
	for i, b := range hash {
		var e field.Element
		e.SetUint64(uint64(b))
		_, pos := c.sys.LookupPublicInputByTag(risc5.GuestPublicOutputsPI, i)
		if diff := pub[pos].Sub(field.ElemFromBase(e)); !diff.IsZero() {
			return nil, fmt.Errorf("traced guest output byte %d is %v, the executed one %d", i, pub[pos], b)
		}
		out = append(out, pub[pos])
	}
	for i := range risc5.NumProgramVK {
		_, pos := c.sys.LookupPublicInputByTag(risc5.ProgramVKPI, i)
		out = append(out, pub[pos])
	}
	return out, nil
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
// public inputs are bound by the hash, which dev-partial checks against the
// trace; the native runner check, when set, compares the whole output.
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
	g, err := c.loadGuest(job.GuestProgramID)
	if err != nil {
		return failResult(job.ID, err)
	}
	inputs, err := c.buildInputs(job, g)
	if err != nil {
		return failResult(job.ID, fmt.Errorf("building inputs: %w", err))
	}

	node, numShards, err := c.runProve(ctx, &zkcdriver.PreReadInputs{Inputs: inputs}, job.ShardIndexes)
	if err != nil {
		return failResult(job.ID, err)
	}
	// A strict subset of the shards is a conglomeration node for the
	// orchestrator to merge further, never a final proof.
	if uint(len(node.Coverage)) != numShards {
		return Result{JobID: job.ID, Status: ResultStatusNode, Node: &node, NumShards: numShards}
	}
	//nolint (until SerializeProof implemented)
	proofBytes, err := SerializeProof(node.Proof, node.Pub)
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
// expects for guest g.
func (c *Core) buildInputs(job Job, g guestProgram) (map[string]bit.Array, error) {
	if err := sanityCheckJobs(job, c.cfg.ShardSize); err != nil {
		return nil, err
	}
	return c.encodeGuestInputs(job, g)
}

// encodeGuestInputs converts a Job's Payload into the guest and memory inputs
// ZkC expects, without the proving-path single-block check. dev-zkvm passes the
// whole extended (0x0002) input and lets the guest conflate the range; a
// conflation disagreement surfaces as a cross-check failure, not silent
// corruption.
func (c *Core) encodeGuestInputs(job Job, g guestProgram) (map[string]bit.Array, error) {
	dataBlobs, err := elfmapping.NewData(
		elfmapping.DefaultInputOrigin,
		decodePayload(job),
		elfmapping.WithLengthPrefix(),
	)
	if err != nil {
		return nil, fmt.Errorf("building data section: %w", err)
	}
	inputs, err := elfmapping.EncodeInputs(g.program, dataBlobs)
	if err != nil {
		return nil, err
	}
	maps.Copy(inputs, g.decoded.EncodeInputs())
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

// runProve lazily traces the inputs, proves and verifies the shards of
// shardIndexes (all of them if empty) one at a time, so that a single shard is
// materialised at any moment rather than the whole trace, and conglomerates
// the shard proofs as they arrive. It returns the resulting node and the
// trace's shard count; the node is the root, and has passed the root checks,
// iff it covers every shard. Without sharding, the single shard's proof is
// returned as is: the in-shard message-bus check already applies.
func (c *Core) runProve(
	ctx context.Context,
	preRead *zkcdriver.PreReadInputs,
	shardIndexes []uint,
) (conglomeration.Node, uint, error) {

	// With sharding, this only runs the fast-mode pass recording one checkpoint
	// per shard.
	lazyTrace, err := c.driver.TryTraceZkcInputs(preRead)
	if err != nil {
		return conglomeration.Node{}, 0, fmt.Errorf("tracing: %w", guestError(err))
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
		return conglomeration.Node{}, 0, errors.New("the trace has no shards")
	}
	if last := indexes[len(indexes)-1]; last >= numShards {
		return conglomeration.Node{}, 0, fmt.Errorf("shard index %d out of range, the trace has %d shards", last, numShards)
	}

	// With sharding the in-shard message-bus check is off, so even a single
	// shard needs the root checks of the merger.
	isRoot := len(indexes) == int(numShards)
	needsMerger := len(indexes) > 1 || (isRoot && c.cfg.ShardSize > 0)
	if needsMerger && c.merger == nil {
		return conglomeration.Node{}, 0, fmt.Errorf("conglomerating %d of %d shards needs the recursion machine: %w",
			len(indexes), numShards, ErrNotImplemented)
	}
	if needsMerger {
		logrus.Warnf("Conglomerating %d of %d shards with %T", len(indexes), numShards, c.merger)
	}
	if !isRoot {
		logrus.Infof("Proving %d of %d shards: the result is a conglomeration node, not a final proof", len(indexes), numShards)
	}

	// Without a merger there is a single shard, whose proof is the result as
	// is: no reducer, which would lift it through the merger.
	var reducer *conglomeration.Reducer
	if needsMerger {
		reducer = conglomeration.NewReducer(c.merger, conglomeration.DefaultMaxInFlightMerges)
	}
	for k, i := range indexes {
		if err := ctx.Err(); err != nil {
			return conglomeration.Node{}, 0, fmt.Errorf("before shard %d: %w", i, err)
		}
		shard, err := traceShard(lazyTrace, i)
		if err != nil {
			return conglomeration.Node{}, 0, err
		}
		// Once the last shard is traced, drop the lazy trace so that its
		// checkpoints can be garbage collected before that shard is proved.
		if k == len(indexes)-1 {
			lazyTrace = zkcdriver.LazyTrace{}
		}
		proof, pub, err := c.proveShard(shard)
		if err != nil {
			return conglomeration.Node{}, 0, fmt.Errorf("shard %d: %w", i, err)
		}
		node := conglomeration.Node{Coverage: []uint{i}, Proof: proof, Pub: pub}
		if reducer == nil {
			return node, numShards, nil
		}
		// Add reports a merge that failed meanwhile: stop rather than prove
		// shards whose result is already lost.
		if err := reducer.Add(node); err != nil {
			return conglomeration.Node{}, 0, fmt.Errorf("merging after shard %d: %w", i, err)
		}
	}
	// The last merge is the root merge when every shard is covered.
	var last func([]conglomeration.Node) (conglomeration.Node, error)
	if isRoot && c.cfg.ShardSize > 0 {
		last = func(children []conglomeration.Node) (conglomeration.Node, error) {
			return c.merger.Finalize(children, numShards)
		}
	}
	root, err := reducer.Close(last)
	if err != nil {
		return conglomeration.Node{}, 0, fmt.Errorf("merging: %w", err)
	}
	return root, numShards, nil
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

// proveShard proves the given shard, then verifies the proof. In dev-partial it
// checks the shard instead.
func (c *Core) proveShard(shard zkcdriver.Shard) (wiop.Proof, wiop.PublicInput, error) {
	assign := func(rt *wiop.Runtime) { c.driver.AssignTraceShard(rt, shard, field.Octuplet{}) }
	if c.mode == ProverModeDevPartial {
		return c.checkAssignment(assign)
	}
	proof, pub := c.sys.Prove(assign)
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

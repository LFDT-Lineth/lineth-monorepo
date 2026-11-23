package backend

// Config holds startup parameters for [New]. All paths must be absolute or
// relative to the working directory of the process.
type Config struct {
	// CircuitBinPath is the path to RISCV-ZKC.bin, the compiled RISC-V VM
	// circuit. Loaded once at startup and reused across all proof requests.
	CircuitBinPath string

	// GuestsDir holds the guest ELFs, each found by [Job.GuestProgramID]
	// (see [guests.Store]). Required by every mode that runs a guest.
	GuestsDir string

	// Mode selects how a Job is proved; empty defaults to [ProverModeFull].
	Mode ProverMode

	// ShardSize is the number of executed RISC-V instructions per trace shard
	// (see zkcdriver.Settings.ShardSize). Zero disables sharding: the whole
	// execution is a single shard.
	//
	// Sharding is not production-ready. With a non-zero ShardSize the in-shard
	// message-bus check is off and the bus balances only across shards, which
	// a [conglomeration.Merger] must check. dev-partial checks it with
	// [conglomeration.MockMerger]; full mode has no merger until
	// [conglomeration.RecursionMerger] has its in-machine verifier, so every
	// sharded job fails with [ErrNotImplemented]. Leave it zero in production.
	ShardSize uint64

	// ShardWorkers is the number of shards traced and proved at the same time.
	// Zero or one proves them one at a time. Each shard in progress needs its
	// own memory: about 30 to 35 GiB, on top of the trace checkpoints,
	// measured on the blocks listed in docs/prover-modes.md.
	ShardWorkers int

	// GoGen runs ZkC Execute with gogen: zkc generates Go code for the circuit
	// and builds it with the Go toolchain on PATH, instead of interpreting it.
	// It is much faster (a 2M-gas block runs in about 1s instead of 78s) and
	// gives the same output. dev-zkvm and dev-partial run Execute.
	GoGen bool
}

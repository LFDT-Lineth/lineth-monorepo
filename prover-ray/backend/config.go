package backend

// Config holds startup parameters for [New]. All paths must be absolute or
// relative to the working directory of the process.
type Config struct {
	// CircuitBinPath is the path to RISCV-ZKC.bin, the compiled RISC-V VM
	// circuit. Loaded once at startup and reused across all proof requests.
	CircuitBinPath string

	// GuestELFPath is the path to the guest ELF binary. For l2-execution
	// proofs this is riscv-guests/l2_execution/zig-out/bin/evm_execution_guest.
	// Different proof types will eventually point to different ELFs; that
	// dispatch is not yet implemented.
	GuestELFPath string

	// Mode selects how a Job is proved; empty defaults to [ProverModeFull].
	Mode ProverMode

	// ShardSize is the number of executed RISC-V instructions per trace shard
	// (see zkcdriver.Settings.ShardSize). Zero disables sharding: the whole
	// execution is a single shard.
	//
	// Sharding is not production-ready. With a non-zero ShardSize the in-shard
	// message-bus check is off and the bus balances only across shards, which
	// only conglomeration can check; in full mode, until it lands, every
	// sharded job fails with [ErrNotImplemented]. Leave it zero in production.
	ShardSize uint64
}

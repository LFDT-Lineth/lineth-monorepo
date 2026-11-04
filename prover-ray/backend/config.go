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
	// only conglomeration can check; in full mode, until it lands, every
	// sharded job fails with [ErrNotImplemented]. Leave it zero in production.
	ShardSize uint64
}

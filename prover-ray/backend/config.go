package backend

// DefaultINOrigin is the canonical _in_start RAM address for all Linea guest
// programs: the address where the framed SSZ StatelessInput is mapped before
// the VM starts executing.
//
// Source: riscv-guests/build_common/Makefile, IN_ORIGIN = 0x08800000.
const DefaultINOrigin uint64 = 0x08800000

// Config holds startup parameters for [New]. All paths must be absolute or
// relative to the working directory of the process.
type Config struct {
	// CircuitBinPath is the path to RISCV-ZKC.bin, the compiled RISC-V VM
	// circuit. Loaded once at startup and reused across all proof requests.
	CircuitBinPath string

	// GuestELFPath is the path to the guest ELF binary. For l2-execution
	// proofs this is riscv-guests/l2-execution/zig-out/bin/evm_execution_guest.
	// Different proof types will eventually point to different ELFs; that
	// dispatch is not yet implemented.
	GuestELFPath string

	// INOrigin is the guest RAM address where the SSZ StatelessInput is
	// placed. Defaults to [DefaultINOrigin] when zero.
	INOrigin uint64
}

func (c *Config) inOrigin() uint64 {
	if c.INOrigin == 0 {
		return DefaultINOrigin
	}
	return c.INOrigin
}

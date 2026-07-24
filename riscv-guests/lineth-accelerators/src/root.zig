// Public surface of the zkVM wrapper package.
//
// The individual wrapper modules are kept private; their methods and types are
// re-exported flat here, so consumers call e.g. `lib.zkvm_keccak256` rather
// than `lib.keccak.zkvm_keccak256`.

const lineth_std = @import("std.zig");
const zkvm_types = @import("zkvm_types.zig");
const keccak = @import("keccak.zig");
const poseidon2 = @import("poseidon2.zig");
const io = @import("io.zig");

// ── zkVM standard runtime (include/zkvm_std.h) ──────────────────────────────
pub const zkvm_exit = lineth_std.zkvm_exit;
pub const panic = lineth_std.panic;

// ── Shared accelerator types (include/zkvm_accelerators.h) ──────────────────
pub const zkvm_status = zkvm_types.zkvm_status;
pub const zkvm_bytes_16 = zkvm_types.zkvm_bytes_16;
pub const zkvm_bytes_32 = zkvm_types.zkvm_bytes_32;
pub const zkvm_bytes_48 = zkvm_types.zkvm_bytes_48;
pub const zkvm_bytes_64 = zkvm_types.zkvm_bytes_64;
pub const zkvm_bytes_96 = zkvm_types.zkvm_bytes_96;
pub const zkvm_bytes_128 = zkvm_types.zkvm_bytes_128;
pub const zkvm_bytes_192 = zkvm_types.zkvm_bytes_192;

// ── Keccak accelerator (include/zkvm_accelerators.h) ────────────────────────
pub const zkvm_keccak256_hash = keccak.zkvm_keccak256_hash;
pub const zkvm_keccak256 = keccak.zkvm_keccak256;

// ── io accelerator (zkvm-standards io-interface, include/zkvm_io.h) ──────────
pub const write_output = io.write_output;

// ── Poseidon2 accelerator (include/lineth_accelerators.h) ───────────────────
pub const lineth_zkvm_poseidon2_permutation = poseidon2.lineth_zkvm_poseidon2_permutation;

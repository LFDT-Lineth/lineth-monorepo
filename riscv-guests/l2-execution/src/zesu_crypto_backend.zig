//! Re-exports the functions this guest borrows directly from zesu's own native crypto backend
//! (`crypto/backends/*.zig`) — the ones with no C-library dependency, so they cross-compile to
//! riscv64 freestanding just like `zesu_zkvm_stdlibs`. Used by zkvm_provide.zig as the software
//! implementation for precompiles `zesu_zkvm_stdlibs` leaves stubbed (modexp, RIPEMD-160), until a
//! real Lineth accelerator wrapper exists for them.

const modexp_impl = @import("zesu_modexp_impl");
const ripemd160_impl = @import("zesu_ripemd160_impl");

pub const modexp = modexp_impl.modexp;
pub const ripemd160 = ripemd160_impl.ripemd160;

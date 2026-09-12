//! Re-exports Zesu's native crypto backend functions that cross-compile to freestanding riscv64.

const modexp_impl = @import("zesu_modexp_impl");
const ripemd160_impl = @import("zesu_ripemd160_impl");
const blake2f_impl = @import("zesu_blake2f_impl");

pub const modexp = modexp_impl.modexp;
pub const ripemd160 = ripemd160_impl.ripemd160;
pub const blake2f = blake2f_impl.compress;

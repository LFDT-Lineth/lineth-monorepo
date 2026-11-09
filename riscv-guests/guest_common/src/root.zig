//! Generic primitives shared by riscv-guests programs: the SSZ codec and the canonical forms of the
//! lists the guests commit to. Freestanding-safe.

pub const ssz = @import("ssz.zig");
pub const lists = @import("lists.zig");

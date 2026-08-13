//! Vendored from https://github.com/jedisct1/zig-lz4, commit
//! 037e966ab9de0f1eff62344092154e6d737236c7 (2026-06-12, HEAD of `main` at vendoring time).
//! Unmodified except for this file, which trims root.zig to the two symbols
//! bench_lz4 uses.
//!
//! Vendored rather than depended on: upstream's build.zig calls
//! `Run.addPassthruArgs()`, removed from std.Build.Step.Run in this project's
//! pinned Zig (0.16.0), so `b.dependency("lz4", ...)` fails before we ever
//! reach the module we want. lz4.zig/lz4hc.zig import nothing but `std`, so
//! referencing them directly sidesteps upstream's build.zig entirely without
//! touching the decompression logic itself, which is unmodified upstream code.

const lz4 = @import("lz4.zig");
const lz4hc = @import("lz4hc.zig");

pub const Error = lz4.Error;
pub const decompressSafeUsingDict = lz4.decompressSafeUsingDict;
pub const decompressSafe = lz4.decompressSafe;

pub const hc = lz4hc;

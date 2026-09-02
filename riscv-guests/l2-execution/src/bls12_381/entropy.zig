// SPDX-License-Identifier: MIT
//! Freestanding stub for the zig-libs `entropy` module (the only declared
//! dependency of the vendored bls12_381 module). Upstream, `entropy` backs
//! `Fr.random`'s rejection-sampling draw; the EVM precompile surface never
//! calls `Fr.random` (all inputs are attacker-supplied bytes decoded through
//! the rejecting parsers), so this stub is unreachable in the guest.

const std = @import("std");

/// Fills `buf` with cryptographically secure random bytes. Unavailable in the
/// freestanding riscv64 guest: the zkVM has no randomness source, and every
/// guest code path needing "randomness" (e.g. KZG batch verification's Fiat-
/// Shamir challenge) derives it deterministically from a transcript hash.
pub fn fill(io: std.Io, buf: []u8) void {
    _ = io;
    _ = buf;
    @panic("entropy.fill called in freestanding zkVM guest: no randomness source available");
}

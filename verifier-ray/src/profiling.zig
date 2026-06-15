//! Lightweight, build-time-gated profiling counters.
//!
//! When the verifier is built without `-Dverifier-profiling`, every helper in
//! this module is a no-op guarded by a `comptime` check on `enabled`, so the
//! counter state and increment code are eliminated entirely by the optimizer
//! and add zero runtime cost. When profiling is enabled the counters track how
//! often hot primitives run.

const config = @import("profiling_config");

/// Whether profiling was enabled at build time via `-Dverifier-profiling`.
pub const enabled: bool = config.is_enabled;

/// Snapshot of all tracked counters.
pub const Counters = struct {
    poseidon2_compress: u64 = 0,
};

/// Global counter state. Only ever touched when `enabled` is true.
var counters: Counters = .{};

/// Record a single Poseidon2 compression call.
pub inline fn poseidon2Compress() void {
    if (comptime enabled) counters.poseidon2_compress += 1;
}

/// Return a copy of the current counter values.
pub fn snapshot() Counters {
    return counters;
}

/// Reset all counters back to zero.
pub fn reset() void {
    if (comptime enabled) counters = .{};
}

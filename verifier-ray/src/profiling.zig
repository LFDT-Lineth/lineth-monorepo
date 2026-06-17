//! Lightweight, build-time-gated profiling counters.
//!
//! When the verifier is built without `-Dverifier-profiling`, every helper in
//! this module is a no-op guarded by a `comptime` check on `enabled`, so the
//! counter state and increment code are eliminated entirely by the optimizer
//! and add zero runtime cost. When profiling is enabled the counters track how
//! often hot primitives run.
//!
//! Additionally, when `-Dr5-marks` is enabled, the `markR5Value` function emits
//! specially-formatted markers on the zkc write channel that are parsed and
//! printed in the interpreter trace, so you can see when certain phases start and
//! end in the zkVM execution. These are gated separately from the main profiling
//! flag so you can enable the R5 markers without the counters if you just want
//! to visualize the phases in the trace.
const config = @import("profiling_config");

/// Whether profiling was enabled at build time via `-Dverifier-profiling`.
pub const enabled: bool = config.is_enabled;

/// Whether R5 phase markers are enabled at build time via `-Dr5-marks`.
pub const r5_marks: bool = config.is_r5_marks;

/// Snapshot of all tracked counters.
pub const Counters = struct {
    poseidon2_compress: u64 = 0,
};

/// Stable marker IDs emitted by `markR5Value`.
///
/// The zkc profiling parser uses these IDs to recover verifier phase boundaries
/// from the generic RISC-V interpreter trace.
pub const Mark = struct {
    pub const verify_start: u64 = 1;
    pub const transcript_done: u64 = 2;
    pub const vanishing_start: u64 = 3;
    pub const vanishing_done: u64 = 4;
    pub const verify_done: u64 = 5;
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

/// Emit an ASCII `"VERIFIER-MARK\t<phase>\t<value>"` marker on the zkc write channel so
/// entry/exit points are visible in the interpreter trace. The `phase` prefix
/// is formatted at comptime; only the runtime `value` is rendered on the fly.
///
/// This goes through the Linux RISC-V `write(2)` syscall (a7=64), which the zkc
/// interpreter parses (see arithmetization i_type.zkc `WRITE_SYSCALL`): it reads
/// `a2` bytes from the buffer at `a1` and prints each as an ASCII char, followed
/// by its own newline — so we omit a trailing newline here.
///
/// The whole body is behind a comptime `r5_marks` gate, so on native /
/// test builds (and when profiling is off) it compiles to nothing and the
/// RISC-V assembly is never analyzed.
pub fn markR5Value(comptime phase: u64, value: u64) void {
    if (comptime r5_marks) {
        const prefix = "VERIFIER-MARK\t" ++ comptime decimalString(phase) ++ "\t";
        // prefix + up to 20 decimal digits for a u64.
        var buf: [prefix.len + 20]u8 = undefined;
        @memcpy(buf[0..prefix.len], prefix);
        const len = prefix.len + writeDecimal(buf[prefix.len..], value);
        writeR5(buf[0..len]);
    }
}

/// Comptime decimal rendering of `value` to a string literal.
fn decimalString(comptime value: u64) []const u8 {
    if (value == 0) return "0";
    comptime var buf: [20]u8 = undefined;
    comptime var i: usize = buf.len;
    comptime var n = value;
    inline while (n != 0) : (n /= 10) {
        i -= 1;
        buf[i] = '0' + @as(u8, @intCast(n % 10));
    }
    return buf[i..];
}

/// Render `value` as decimal ASCII into the start of `buf`, returning the number
/// of bytes written. `buf` must hold at least 20 bytes (max u64 digit count).
fn writeDecimal(buf: []u8, value: u64) usize {
    var tmp: [20]u8 = undefined;
    var n = value;
    var i: usize = tmp.len;
    if (n == 0) {
        i -= 1;
        tmp[i] = '0';
    } else while (n != 0) {
        i -= 1;
        tmp[i] = '0' + @as(u8, @intCast(n % 10));
        n /= 10;
    }
    const digits = tmp[i..];
    @memcpy(buf[0..digits.len], digits);
    return digits.len;
}

/// Issue the RISC-V `write(fd=1, buf, len)` syscall. Only referenced from the
/// R5-gated branch of `markR5Value`, so the assembly is never analyzed natively.
fn writeR5(bytes: []const u8) void {
    asm volatile (
        \\li a0, 1
        \\mv a1, %[ptr]
        \\mv a2, %[len]
        \\li a7, 64
        \\ecall
        :
        : [ptr] "r" (@intFromPtr(bytes.ptr)),
          [len] "r" (bytes.len),
        : .{ .a0 = true, .a1 = true, .a2 = true, .a7 = true, .memory = true });
}

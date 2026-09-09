const std = @import("std");
const builtin = @import("builtin");

const rollup_ssz = @import("rollup_ssz");
const rollup = @import("rollup.zig");

/// Exit-code taxonomy for guest failures; `pub` for this guest's own test suite.
pub const guest_errors = @import("guest_errors.zig");

// Heap starts at the address defined by the linker script (canonical Lineth layout: `_heap_start`
// = 0x48800000, grows up). The linker script does not actually cap the heap at this size; it is a
// reasonable upper bound for this guest's small, bounded inputs.
extern var _heap_start: u8;
const GUEST_HEAP_SIZE: usize = 256 * 1024 * 1024;

// This is the rollup zkVM guest stub: it decodes `RollupProofPrivateInput`, maps it to
// `RollupOutput` entirely by echo/sentinel (`rollup.run`), and emits the SSZ output. No proof
// verification and no chunk/conflation folding happen here; those are the real rollup guest's
// concern.

/// zkVM guest entry. Reads the framed input via `read_input`, runs `runRollupGuest`, and emits the
/// SSZ output via `write_output`. Exits 0 on success; on failure, exits with
/// `guest_errors.exitCode(err)` — a deterministic, category-stable nonzero code.
fn guestMain() callconv(.c) noreturn {
    const zkvm_io = @import("lineth_accelerators");

    const heap = @as([*]u8, @ptrCast(&_heap_start))[0..GUEST_HEAP_SIZE];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();

    var buf_ptr: [*]const u8 = undefined;
    var buf_size: usize = undefined;
    zkvm_io.read_input(&buf_ptr, &buf_size);
    const raw_input = buf_ptr[0..buf_size];

    const out = runRollupGuest(allocator, raw_input) catch |err| {
        exit(guest_errors.exitCode(err));
    };
    zkvm_io.write_output(out.ptr, out.len);
    exit(0);
}

/// Decode -> map -> encode, factored out of `guestMain` so the whole pipeline is one `catch` away
/// from a clean, categorized guest rejection, and so it can be driven directly from a native host
/// test (it touches no riscv asm and no zkVM io — those are `guestMain`'s own concern).
pub fn runRollupGuest(allocator: std.mem.Allocator, raw_input: []const u8) ![]const u8 {
    const decoded = try rollup_ssz.decodeInput(allocator, raw_input);
    const result = try rollup.run(allocator, decoded);
    return rollup_ssz.encodeOutput(allocator, result) catch return error.OutputEncodeFailed;
}

comptime {
    // Export `main` only for the freestanding RISC-V guest, which owns its entry point. Native
    // builds import this module as a library (the host test calling `runRollupGuest` directly) and
    // get `main` from std.start — exporting it here too would be a symbol collision.
    if (builtin.cpu.arch == .riscv64) {
        @export(&guestMain, .{ .name = "main" });
    }
}

fn exit(code: u64) noreturn {
    if (builtin.cpu.arch == .riscv64) {
        asm volatile (
            \\mv a0, %[code]
            \\li a7, 93
            \\ecall
            :
            : [code] "r" (code),
            : .{ .x10 = true, .x17 = true });
        unreachable;
    }

    std.debug.panic("guest exit({d})", .{code});
}

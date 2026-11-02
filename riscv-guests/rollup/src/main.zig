const std = @import("std");
const builtin = @import("builtin");

const package = @import("rollup");
const rollup = package.rollup;
const guest_errors = package.guest_errors;
const zkvm_provide = @import("zkvm_provide");

// Heap bounds come from the linker script (canonical Lineth layout: `_heap_start` = 0x48800000,
// grows up to `_heap_end`). build_common/Makefile sets the size via `HEAP_LENGTH` and asserts that
// the heap ends below the 4 GiB guest address space.
extern var _heap_start: u8;
extern var _heap_end: u8;

// The rollup zkVM guest: it decodes `RollupProofPrivateInput`, runs the rollup statement over it
// (`rollup.runFramed`) and emits the framed public inputs. The crypto accelerators (zkvm_*) the data
// availability and block decoding declare as externs are defined in-guest by zkvm_provide, so the
// statically-linked guest ELF has no unresolved zkvm_* externals.

/// zkVM guest entry. Reads the framed input via `read_input`, runs `rollup.runFramed`, and emits
/// the framed output via `write_output`. Exits 0 on success; on failure, exits with
/// `guest_errors.exitCode(err)` — a deterministic, category-stable nonzero code.
fn guestMain() callconv(.c) noreturn {
    const zkvm_io = @import("lineth_accelerators");

    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();
    zkvm_provide.setAllocator(allocator);
    if (!zkvm_provide.initKzg()) exit(guest_errors.exitCode(error.OutOfMemory));

    var buf_ptr: [*]const u8 = undefined;
    var buf_size: usize = undefined;
    zkvm_io.read_input(&buf_ptr, &buf_size);
    const raw_input = buf_ptr[0..buf_size];

    const out = rollup.runFramed(allocator, raw_input) catch |err| {
        exit(guest_errors.exitCode(err));
    };
    zkvm_io.write_output(out.ptr, out.len);
    exit(0);
}

comptime {
    // Only the freestanding RISC-V guest owns its entry point; every other target takes `main`
    // from std.start.
    if (builtin.cpu.arch == .riscv64) {
        @export(&guestMain, .{ .name = "main" });
        // Pull in the precompile providers for the freestanding guest.
        _ = zkvm_provide;
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

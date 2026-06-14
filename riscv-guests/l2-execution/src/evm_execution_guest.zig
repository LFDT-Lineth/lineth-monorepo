const std = @import("std");
const builtin = @import("builtin");

const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const ssz_output = @import("zesu_ssz_output");
const zesu_allocator = @import("zesu_allocator");

// Heap start from the linker script (canonical Linea layout: `_heap_start` = 0x48800000, grows up).
extern var _heap_start: u8;
// Linker script does not actually constraint the heap to 256 MiB, but this is a reasonable upper bound
const GUEST_HEAP_SIZE: usize = 256 * 1024 * 1024;

// This guest is a thin wrapper over zesu's vanilla stateless execution: it decodes an SSZ-encoded
// StatelessInput, executes the block, and serializes the SSZ validation result — the same pipeline
// as zesu's `runner.runStateless` / `zkevm-blockchain-test-runner`.
//
// The crypto accelerators (zkvm_*) that zesu declares as externs are DEFINED in-guest by
// zkvm_provide.zig (pulled in below for the riscv64 build), so the statically-linked guest ELF has
// no unresolved zkvm_* externals. The native host build doesn't reference them — it uses zesu's
// C-backed crypto instead.

/// Result of running one SSZ-encoded StatelessInput:
///   `out`     — the 105-byte SSZ SszStatelessValidationResult
///   `success` — successful_validation: execution succeeded AND the computed post-state and
///               receipts roots match the values claimed in the payload.
pub const Result = struct {
    out: [105]u8,
    success: bool,
};

/// Vanilla zesu stateless block execution. Fed an explicit byte slice so it runs identically on the
/// native host (tests) and from the zkVM guest entry below.
pub fn runStateless(allocator: std.mem.Allocator, ssz_input: []const u8) !Result {
    zesu_allocator.set(allocator);

    const si = try ssz_decode.decode(allocator, ssz_input);
    const ep = &si.new_payload_request.execution_payload;

    const success = blk: {
        const proof = executor.executeStatelessInput(allocator, si, si.chain_config.fork_name) catch break :blk false;
        if (!std.mem.eql(u8, &proof.post_state_root, &ep.state_root)) break :blk false;
        if (!std.mem.eql(u8, &proof.receipts_root, &ep.receipts_root)) break :blk false;
        break :blk true;
    };

    const out = try ssz_output.serialize(allocator, si.new_payload_request, si.chain_config.chain_id, success);
    return .{ .out = out, .success = success };
}

/// zkVM guest entry. Reads the SSZ StatelessInput via the zkvm-standards `read_input` — the same ABI
/// Zesu uses — then executes it and exits 0 on successful_validation, 1 otherwise. WHERE the input
/// lives is the proving system's concern, NOT the guest's: for Linea, `read_input` is satisfied by
/// zesu-zkvm's `linea/src/zkvm_io.zig` (imported as `linea_zkvm_io`), which reads the memory-mapped
/// `_in_start` (framed `[u64 LE len][SSZ]`). The guest never names a memory slot.
fn guestMain() callconv(.c) noreturn {
    const zkvm_io = @import("linea_zkvm_io");

    const heap = @as([*]u8, @ptrCast(&_heap_start))[0..GUEST_HEAP_SIZE];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();

    var buf_ptr: [*]const u8 = undefined;
    var buf_size: usize = undefined;
    zkvm_io.read_input(&buf_ptr, &buf_size);
    const ssz_input = buf_ptr[0..buf_size];

    const result = runStateless(allocator, ssz_input) catch exit(1);
    exit(if (result.success) 0 else 1);
}

comptime {
    // Export `main` only for the freestanding RISC-V guest, which owns its entry point. Native
    // builds import this as a library (the unit test and the spec runner exe) and get `main` from
    // std.start — exporting it here too would be a symbol collision.
    if (builtin.cpu.arch == .riscv64) {
        @export(&guestMain, .{ .name = "main" });
        // Pull in the precompile providers (zkvm_provide.zig): it DEFINES every zkvm_* symbol zesu
        // references — keccak from the Linea wrapper, the rest from zesu-zkvm's stdlibs_accel.
        // Freestanding only — the native build uses Zesu's C backend and never references zkvm_*.
        _ = @import("zkvm_provide.zig");
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

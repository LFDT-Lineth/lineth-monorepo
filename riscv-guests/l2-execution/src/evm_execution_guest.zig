const std = @import("std");
const builtin = @import("builtin");

const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const ssz_output = @import("zesu_ssz_output");
const zesu_allocator = @import("zesu_allocator");

// Heap starts at the address defined by the linker script (canonical Lineth layout: `_heap_start` = 0x48800000, grows up).
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

const ExitReason = enum {
    valid,
    execution_error,
    post_state_root_mismatch,
    receipts_root_mismatch,
    state_and_receipts_roots_mismatch,
};

const RunResult = struct {
    result: Result,
    exit_reason: ExitReason,
};

const GuestRunOutcome = union(enum) {
    result: RunResult,
    decode_error,
    serialize_error,
};

/// Vanilla zesu stateless block execution. Fed an explicit byte slice so it runs identically on the
/// native host (tests) and from the zkVM guest entry below.
pub fn runStateless(allocator: std.mem.Allocator, ssz_input: []const u8) !Result {
    return switch (runStatelessWithExitReason(allocator, ssz_input)) {
        .result => |run_result| run_result.result,
        .decode_error => error.DecodeError,
        .serialize_error => error.SerializeError,
    };
}

fn runStatelessWithExitReason(allocator: std.mem.Allocator, ssz_input: []const u8) GuestRunOutcome {
    zesu_allocator.set(allocator);

    const si = ssz_decode.decode(allocator, ssz_input) catch return .decode_error;
    const ep = &si.new_payload_request.execution_payload;

    const exit_reason: ExitReason = blk: {
        const proof = executor.executeStatelessInput(allocator, si, si.chain_config.fork_name) catch break :blk .execution_error;
        const state_root_matches = std.mem.eql(u8, &proof.post_state_root, &ep.state_root);
        const receipts_root_matches = std.mem.eql(u8, &proof.receipts_root, &ep.receipts_root);
        if (state_root_matches and receipts_root_matches) break :blk .valid;
        if (!state_root_matches and !receipts_root_matches) break :blk .state_and_receipts_roots_mismatch;
        if (!state_root_matches) break :blk .post_state_root_mismatch;
        break :blk .receipts_root_mismatch;
    };

    const out = ssz_output.serialize(allocator, si.new_payload_request, si.chain_config.chain_id, exit_reason == .valid) catch return .serialize_error;
    return .{ .result = .{ .result = .{ .out = out, .success = exit_reason == .valid }, .exit_reason = exit_reason } };
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

    switch (runStatelessWithExitReason(allocator, ssz_input)) {
        .decode_error => exitWithMarker("L2_EXEC_EXIT:SSZ_DECODE_ERROR\n", 1),
        .serialize_error => exitWithMarker("L2_EXEC_EXIT:SSZ_SERIALIZE_ERROR\n", 1),
        .result => |run_result| exitWithMarker(exitMarker(run_result.exit_reason), if (run_result.result.success) 0 else 1),
    }
}

// Emits a stable marker through the Linux write syscall before every controlled guest exit.
// Panics or traps before this function is reached cannot emit a marker.
fn exitWithMarker(marker: []const u8, code: u64) noreturn {
    writeExitMarker(marker);
    exit(code);
}

// Writes a debug marker directly through Linux RISC-V write(1, marker, len): a7 = 64.
// The arithmetization interpreter handles this in WRITE_SYSCALL and relays the bytes through
// its printf channel.
fn writeExitMarker(marker: []const u8) void {
    if (builtin.cpu.arch == .riscv64) {
        _ = asm volatile ("ecall"
            : [ret] "={a0}" (-> usize),
            : [fd] "{a0}" (@as(usize, 1)),
              [buf] "{a1}" (@intFromPtr(marker.ptr)),
              [count] "{a2}" (marker.len),
              [syscall] "{a7}" (@as(usize, 64)),
            : .{ .memory = true });
    }
}

fn exitMarker(reason: ExitReason) []const u8 {
    return switch (reason) {
        .valid => "L2_EXEC_EXIT:VALID\n",
        .execution_error => "L2_EXEC_EXIT:EXECUTOR_ERROR\n",
        .post_state_root_mismatch => "L2_EXEC_EXIT:POST_STATE_ROOT_MISMATCH\n",
        .receipts_root_mismatch => "L2_EXEC_EXIT:RECEIPTS_ROOT_MISMATCH\n",
        .state_and_receipts_roots_mismatch => "L2_EXEC_EXIT:STATE_AND_RECEIPTS_ROOTS_MISMATCH\n",
    };
}

comptime {
    // Export `main` only for the freestanding RISC-V guest, which owns its entry point. Native
    // builds import this as a library (the unit test and the spec runner exe) and get `main` from
    // std.start — exporting it here too would be a symbol collision.
    if (builtin.cpu.arch == .riscv64) {
        @export(&guestMain, .{ .name = "main" });
        // Pull in the precompile providers (zkvm_provide.zig): it DEFINES every zkvm_* symbol zesu
        // references — keccak from the Lineth wrapper, the rest from zesu-zkvm's stdlibs_accel.
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

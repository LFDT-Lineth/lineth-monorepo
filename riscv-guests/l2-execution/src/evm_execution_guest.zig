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

const EXIT_VALID: u64 = 0;
const EXIT_SSZ_DECODE_ERROR: u64 = 1;
const EXIT_EXECUTOR_ERROR: u64 = 2;
const EXIT_POST_STATE_ROOT_MISMATCH: u64 = 3;
const EXIT_RECEIPTS_ROOT_MISMATCH: u64 = 4;
const EXIT_STATE_AND_RECEIPTS_ROOTS_MISMATCH: u64 = 5;
const EXIT_SSZ_SERIALIZE_ERROR: u64 = 6;

// This guest is a thin wrapper over zesu's vanilla stateless execution: it decodes an SSZ-encoded
// StatelessInput, executes the block, and serializes the SSZ validation result — the same pipeline
// as zesu's `runner.runStateless` / `zkevm-blockchain-test-runner`.
//
// The crypto accelerators (zkvm_*) that zesu declares as externs are DEFINED in-guest by
// zkvm_provide.zig (pulled in below for the riscv64 build), so the statically-linked guest ELF has
// no unresolved zkvm_* externals. The native host build doesn't reference them — it uses zesu's
// C-backed crypto instead.

/// Result of running one SSZ-encoded StatelessInput:
///   `out`     — the 69-byte SSZ SszStatelessValidationResult
///   `success` — successful_validation: execution succeeded AND the computed post-state and
///               receipts roots match the values claimed in the payload.
pub const Result = struct {
    out: [69]u8,
    success: bool,
};

const ExitReason = enum {
    valid,
    executor_error,
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
    zesu_allocator.set(allocator);

    const si = try ssz_decode.decode(allocator, ssz_input);
    const ep = &si.new_payload_request.execution_payload;

    const success = blk: {
        const proof = executor.executeStatelessInput(allocator, si, si.chain_config.fork_name) catch break :blk false;
        if (!std.mem.eql(u8, &proof.post_state_root, &ep.state_root)) break :blk false;
        if (!std.mem.eql(u8, &proof.receipts_root, &ep.receipts_root)) break :blk false;
        break :blk true;
    };

    const out = try ssz_output.serialize(allocator, si.chain_config, si.new_payload_request, success);
    return .{ .out = out, .success = success };
}

fn runStatelessWithExitReason(allocator: std.mem.Allocator, ssz_input: []const u8) GuestRunOutcome {
    zesu_allocator.set(allocator);

    const si = ssz_decode.decode(allocator, ssz_input) catch return .decode_error;
    const ep = &si.new_payload_request.execution_payload;

    const exit_reason: ExitReason = blk: {
        const proof = executor.executeStatelessInput(allocator, si, si.chain_config.fork_name) catch break :blk .executor_error;
        const state_root_matches = std.mem.eql(u8, &proof.post_state_root, &ep.state_root);
        const receipts_root_matches = std.mem.eql(u8, &proof.receipts_root, &ep.receipts_root);
        if (state_root_matches and receipts_root_matches) break :blk .valid;
        if (!state_root_matches and !receipts_root_matches) break :blk .state_and_receipts_roots_mismatch;
        if (!state_root_matches) break :blk .post_state_root_mismatch;
        break :blk .receipts_root_mismatch;
    };

    const out = ssz_output.serialize(allocator, si.chain_config, si.new_payload_request, exit_reason == .valid) catch return .serialize_error;
    return .{ .result = .{ .result = .{ .out = out, .success = exit_reason == .valid }, .exit_reason = exit_reason } };
}

/// zkVM guest entry. Reads the SSZ StatelessInput via the zkvm-standards `read_input` — the same ABI
/// Zesu uses — then executes it and exits with a detailed code for the terminal execution path. WHERE
/// the input lives is the proving system's concern, NOT the guest's: for Linea, `read_input` is
/// satisfied by zesu-zkvm's `linea/src/zkvm_io.zig` (imported as `linea_zkvm_io`), which reads the
/// memory-mapped `_in_start` (framed `[u64 LE len][SSZ]`). The guest never names a memory slot.
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
        .decode_error => exit(EXIT_SSZ_DECODE_ERROR),
        .serialize_error => exit(EXIT_SSZ_SERIALIZE_ERROR),
        .result => |run_result| exit(exitCode(run_result.exit_reason)),
    }
}

fn exitCode(reason: ExitReason) u64 {
    return switch (reason) {
        .valid => EXIT_VALID,
        .executor_error => EXIT_EXECUTOR_ERROR,
        .post_state_root_mismatch => EXIT_POST_STATE_ROOT_MISMATCH,
        .receipts_root_mismatch => EXIT_RECEIPTS_ROOT_MISMATCH,
        .state_and_receipts_roots_mismatch => EXIT_STATE_AND_RECEIPTS_ROOTS_MISMATCH,
    };
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

const std = @import("std");
const builtin = @import("builtin");

const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const ssz_output = @import("zesu_ssz_output");
const zesu_allocator = @import("zesu_allocator");

/// Log-preserving stateless-execution seam (full event logs, unlike `runStateless`'s bloom-only
/// output). `pub` for `evm_execution_guest_test.zig`'s parity check; the guest itself never calls
/// this directly — it reaches the seam through `l2_execution.runL2Execution`.
pub const execution = @import("execution.zig");
const l2_execution = @import("l2_execution.zig");
const l2_execution_ssz = @import("l2_execution_ssz");

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

/// zkVM guest entry. Reads the extended `L2ExecutionProofPrivateInput` via `read_input`, runs
/// `l2_execution.runL2Execution`, and emits the SSZ output via `write_output`. Exits 0 on success,
/// 1 on any error. `read_input`/`write_output` are satisfied by zesu-zkvm's `linea_zkvm_io` — where
/// the input lives and how the output surfaces is the proving system's concern, not the guest's.
///
/// This frozen riscv64 binary has no argv, so output format is fixed at build time (always SSZ);
/// the `--json`/`--ssz` toggle lives on the native `l2-execution-runner` tool instead.
fn guestMain() callconv(.c) noreturn {
    const zkvm_io = @import("linea_zkvm_io");

    const heap = @as([*]u8, @ptrCast(&_heap_start))[0..GUEST_HEAP_SIZE];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();

    var buf_ptr: [*]const u8 = undefined;
    var buf_size: usize = undefined;
    zkvm_io.read_input(&buf_ptr, &buf_size);
    const raw_input = buf_ptr[0..buf_size];

    const encoded_output = runL2ExecutionGuest(allocator, raw_input) catch exit(1);
    zkvm_io.write_output(encoded_output);
    exit(0);
}

/// Decode -> execute -> encode, factored out of `guestMain` so the whole pipeline is one
/// `catch exit(1)` away from a clean guest rejection.
fn runL2ExecutionGuest(allocator: std.mem.Allocator, raw_input: []const u8) ![]u8 {
    const decoded = try l2_execution_ssz.decodeInput(allocator, raw_input);
    const result = try l2_execution.runL2Execution(allocator, decoded);

    // Debug visibility for the plain, SSZ-encoded 16-field public-input tuple: `encodeOutput`
    // below commits only its keccak256 (see `l2_execution_ssz.hashPublicInputs`), so this is the
    // only place the plain tuple is still observable. `zkvm_log` (zesu's real logging ABI — see
    // zesu/src/zkvm/root.zig — DEFINED as a no-op in `zkvm_provide.zig`, see its doc comment for
    // why) is the standard sink for this; level 0 mirrors zesu's own `std.log`/panic usage.
    const public_inputs_bytes = l2_execution_ssz.encodePublicInputsBytes(result.public_inputs);
    zkvm_log(0, &public_inputs_bytes, public_inputs_bytes.len);

    return l2_execution_ssz.encodeOutput(allocator, result.public_inputs);
}

/// zesu's own logging ABI (see zesu/src/zkvm/root.zig's doc comment) — DEFINED (as a no-op, for
/// now) in `zkvm_provide.zig` alongside every other `zkvm_*` symbol this statically-linked ELF must
/// satisfy locally.
extern fn zkvm_log(level: u8, msg_ptr: [*]const u8, msg_len: usize) void;

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

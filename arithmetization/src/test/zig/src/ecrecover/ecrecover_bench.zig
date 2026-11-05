//! secp256k1 ecrecover through the guest's `zkvm_secp256k1_ecrecover` provider: a correctness check,
//! then K more calls to measure the per-call cost (GRANULAR.md §7, B1).
//!
//! The provider is built like the l2-execution guest, so the same flags select it:
//!   make exec TEST=ecrecover/ecrecover_bench.zig IN_BYTES=0x0000000000000010 RELEASE=true \
//!     SECP256K1_BACKEND=constantine CTT_FIELD_ACCEL=all \
//!     ZKC_MAIN=$PWD/../main/riscv/main_fieldaccel.zkc
//! Any CTT_FIELD_ACCEL other than off needs an entry point that compiles those ops in.
//!
//! Input: K as a u64 at the IN origin (inline hex is big-endian: IN_BYTES=0x<16 hex digits>).
//! Exit code: 0 on success, 10 + i when vector i is wrong, 2 when a timed call disagrees.
//! Per-call cost = (X(K) - X(0)) / K, for X = steps, cells or time.
const std = @import("std");
const lineth_accel = @import("lineth_zkvm_accel"); // zkvm_exit
const provide = @import("zkvm_provide"); // defines (exports) the zkvm_* symbols

// Force zkvm_provide's comptime @export block to emit zkvm_secp256k1_ecrecover into this ELF.
comptime {
    _ = provide;
}

extern fn zkvm_secp256k1_ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) callconv(.c) i32;

extern var _in_start: u64;
extern var _heap_start: u8;

// The Constantine archive's C allocator shim allocates from this, as in evm_execution_guest.zig.
export var guest_allocator: extern struct { ptr: *anyopaque, vtable: *const anyopaque } = undefined;
const GUEST_HEAP_SIZE: usize = 256 * 1024 * 1024;

const OK: i32 = 0;

const Vector = struct { input: []const u8, address: []const u8 };

// Precompile 0x01 rows (input = hash ‖ v ‖ r ‖ s, address empty when rejected), copied from
// riscv-guests/l2_execution/test/testdata/ecrecover.csv.
const vectors = [_]Vector{
    .{ .input = "18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549", .address = "a94f5374fce5edbc8e2a8697c15331677e6ebf0b" },
    .{ .input = "18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001b73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75f1146bf4e2fc4de1c94f1b81868960f68bc842490f1b6bc98387ba6b57599fbf8", .address = "a94f5374fce5edbc8e2a8697c15331677e6ebf0b" },
    .{ .input = "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001cf6e1bbf53d6059837eccc554c3a878184d60b6ba5b4671520a4f2d33677569d38a468e05e7666cf9531ea6f69f10fdee576da1551bd82be2e44ce9c211faf703", .address = "7e5f4552091a69125d5dfcb7b8c2659029395bdf" },
    .{ .input = "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141000000000000000000000000000000000000000000000000000000000000001cf6e1bbf53d6059837eccc554c3a878184d60b6ba5b4671520a4f2d33677569d38a468e05e7666cf9531ea6f69f10fdee576da1551bd82be2e44ce9c211faf703", .address = "7e5f4552091a69125d5dfcb7b8c2659029395bdf" },
    .{ .input = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff000000000000000000000000000000000000000000000000000000000000001cf6e1bbf53d6059837eccc554c3a878184d60b6ba5b4671520a4f2d33677569d3c04442e30d92c94e3b76834862f1a87b472678236b86774dcd18638f685a51a8", .address = "7e5f4552091a69125d5dfcb7b8c2659029395bdf" },
    .{ .input = "a8b53bdf3306a35a7103ab5504a0c9b492295564b6202b1942a84ef300107281000000000000000000000000000000000000000000000000000000000000001b30783565316530336635336365313862373732636362303039336666373166336635336635633735623734646362333161383561613862383839326234653862", .address = "" }, // r is not an x-coordinate on the curve
    .{ .input = "18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001cfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141eeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549", .address = "" }, // r = n
    .{ .input = "18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75ffffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", .address = "" }, // s = n
    .{ .input = "0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000001b79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f817980000000000000000000000000000000000000000000000000000000000000001", .address = "" }, // recovered key is the point at infinity
};

const Call = struct { msg: [32]u8, sig: [64]u8, recid: u8, address: ?[20]u8 };

const calls = blk: {
    @setEvalBranchQuota(100_000);
    var out: [vectors.len]Call = undefined;
    for (vectors, &out) |v, *c| {
        var input: [128]u8 = undefined;
        _ = std.fmt.hexToBytes(&input, v.input) catch unreachable;
        c.msg = input[0..32].*;
        c.recid = input[63] - 27;
        c.sig = input[64..128].*;
        c.address = if (v.address.len == 0) null else hex20(v.address);
    }
    break :blk out;
};

fn hex20(comptime s: []const u8) [20]u8 {
    var a: [20]u8 = undefined;
    _ = std.fmt.hexToBytes(&a, s) catch unreachable;
    return a;
}

/// Valid vectors only: the timed loop cycles through these.
const n_valid = blk: {
    var n = 0;
    for (calls) |c| n += @intFromBool(c.address != null);
    break :blk n;
};

export fn main() noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0..GUEST_HEAP_SIZE];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();
    guest_allocator = .{ .ptr = allocator.ptr, .vtable = @ptrCast(allocator.vtable) };

    const k: u64 = _in_start;

    // Correctness: every vector, plus a recovery id the backends must reject.
    var pubkeys: [n_valid][64]u8 = undefined;
    var n: usize = 0;
    for (calls, 0..) |c, i| {
        var pubkey: [64]u8 = undefined;
        const ok = zkvm_secp256k1_ecrecover(&c.msg, &c.sig, c.recid, &pubkey) == OK;
        if (c.address) |want| {
            var hash: [32]u8 = undefined;
            std.crypto.hash.sha3.Keccak256.hash(&pubkey, &hash, .{});
            if (!ok or !std.mem.eql(u8, hash[12..], &want)) lineth_accel.zkvm_exit(@intCast(10 + i));
            pubkeys[n] = pubkey;
            n += 1;
        } else if (ok) lineth_accel.zkvm_exit(@intCast(10 + i));
    }
    var out: [64]u8 = undefined;
    if (zkvm_secp256k1_ecrecover(&calls[0].msg, &calls[0].sig, 2, &out) == OK) lineth_accel.zkvm_exit(10 + vectors.len);

    // Benchmark: K more recoveries over the valid vectors.
    var i: u64 = 0;
    while (i < k) : (i += 1) {
        const j: usize = @intCast(i % n_valid);
        const c = valid_calls[j];
        if (zkvm_secp256k1_ecrecover(&c.msg, &c.sig, c.recid, &out) != OK or !std.mem.eql(u8, &out, &pubkeys[j])) {
            lineth_accel.zkvm_exit(2);
        }
    }

    lineth_accel.zkvm_exit(0);
}

const valid_calls = blk: {
    var out: [n_valid]Call = undefined;
    var n = 0;
    for (calls) |c| {
        if (c.address != null) {
            out[n] = c;
            n += 1;
        }
    }
    break :blk out;
};

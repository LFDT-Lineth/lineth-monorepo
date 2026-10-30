//! Freestanding rv64im smoke guest: verifies the EIP-4844 proof of the blob in the input and
//! publishes the blob's versioned hash. The input is the 48-byte commitment, the 48-byte proof and
//! the 131072-byte blob. Exit codes: 0 verified; 2 input of the wrong size; 3 proof rejected.

const std = @import("std");
const builtin = @import("builtin");

const lineth_da = @import("lineth_da");
const accelerators = @import("lineth_accelerators");
const zkvm_provide = @import("zkvm_provide");

const blob_kzg = lineth_da.encoding.blob_kzg;

extern var _heap_start: u8;
extern var _heap_end: u8;

fn smokeMain() callconv(.c) noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    zkvm_provide.setAllocator(fba.allocator());

    var input_ptr: [*]const u8 = undefined;
    var input_len: usize = undefined;
    accelerators.read_input(&input_ptr, &input_len);
    if (input_len != blob_kzg.commitment_len + blob_kzg.proof_len + blob_kzg.blob_len) accelerators.zkvm_exit(2);
    const commitment = input_ptr[0..blob_kzg.commitment_len];
    const proof = input_ptr[blob_kzg.commitment_len..][0..blob_kzg.proof_len];
    const blob = input_ptr[blob_kzg.commitment_len + blob_kzg.proof_len ..][0..blob_kzg.blob_len];

    if (!blob_kzg.verify(blob, commitment, proof)) accelerators.zkvm_exit(3);

    const versioned_hash = blob_kzg.versionedHash(commitment);
    accelerators.write_output(&versioned_hash, versioned_hash.len);
    accelerators.zkvm_exit(0);
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&smokeMain, .{ .name = "main" });
        _ = zkvm_provide;
    }
}

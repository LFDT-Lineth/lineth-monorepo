// Micro-benchmark: RISC-V cycle cost of decoding our own compressor's format,
// in both its deployed (A0) and unmerged Huffman-on-lengths forms, from ONE
// comptime-parameterized implementation (lzss.zig). Both variants run in this
// single guest binary, each under its own marker pair -- lzss.decompress's
// `comptime huffman: bool` parameter monomorphizes into two specialized,
// non-branching functions at compile time, so there is no runtime cost to
// having both in the same build.
//
// Inputs are real: the first 780,000 bytes of
// ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin, compressed on
// the host with the actual Go implementations -- prover's vendored
// github.com/consensys/compress v0.3.0 for a0_compressed.bin, and
// github.com/consensys/compress @ c509f05 (feat/huffman-on-lengths) with our
// trained table (docs/blob-compression/results, copied here as
// huffman-table.bin) for huffman_compressed.bin. Both round-tripped correctly
// on the host before being embedded (a0: 218,921 B, matching the earlier
// blob-chunks measurement exactly; huffman: 210,040 B).
//
// Produce-mode throughout, per the bench_memory result (IN and RAM cost the
// same, so verify-mode buys nothing here either).
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start A0 decode, 11 = end A0 decode      (value = decoded length)
//   12 = FNV-1a of the A0 output
//   20 = start huffman decode, 21 = end huffman decode (value = decoded length)
//   22 = FNV-1a of the huffman output
//
// The two hashes are computed after the timed regions and checked by run.go
// against the corpus plaintext. A decoder that produced wrong bytes could
// otherwise report a cycle count that looks like an improvement, which matters
// here because the backref copy path depends on how the compiler lowers
// misaligned 64-bit accesses on this target.

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const lzss = @import("lzss");
const profiling = verifier_ray.profiling;

const decompressed_len = 780_000;

const dict: []const u8 = @embedFile("dict.bin");
const a0_compressed: []const u8 = @embedFile("a0_compressed.bin");
const huffman_compressed: []const u8 = @embedFile("huffman_compressed.bin");
const huffman_table_bytes: []const u8 = @embedFile("huffman-table.bin");

const huffman_lengths: [512]u8 = blk: {
    var lengths: [512]u8 = undefined;
    @memcpy(&lengths, huffman_table_bytes);
    break :blk lengths;
};
const huffman_table = lzss.HuffmanTable.build(huffman_lengths);

var out_a0: [decompressed_len]u8 = @splat(0);
var out_huffman: [decompressed_len]u8 = @splat(0);

/// FNV-1a, 64-bit. Chosen over a CRC because it needs no lookup table, so the
/// check itself adds nothing to the guest's read-only data.
fn fnv1a(data: []const u8) u64 {
    var h: u64 = 0xcbf29ce484222325;
    for (data) |b| {
        h ^= b;
        h = h *% 0x100000001b3;
    }
    return h;
}

pub export fn main() noreturn {
    profiling.markR5Value(0, 0);
    var i: u64 = 0;
    while (i < 1) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    profiling.markR5Value(10, 0);
    const a0_len = lzss.decompress(false, {}, a0_compressed, &out_a0, dict, decompressed_len);
    profiling.markR5Value(11, a0_len);
    profiling.markR5Value(12, fnv1a(&out_a0));

    profiling.markR5Value(20, 0);
    const huffman_len = lzss.decompress(true, huffman_table, huffman_compressed, &out_huffman, dict, decompressed_len);
    profiling.markR5Value(21, huffman_len);
    profiling.markR5Value(22, fnv1a(&out_huffman));

    accel.zkvm_exit(0);
}

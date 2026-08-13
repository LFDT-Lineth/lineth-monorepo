// Micro-benchmark: RISC-V cycle cost of decoding a zstd frame, using Zig's
// own std.compress.zstd rather than a vendored or hand-written decoder.
//
// That choice is part of what is being measured. std's decoder takes a
// caller-supplied buffer and no allocator, so it runs in the heapless guest
// unmodified, and it is maintained upstream -- the property that makes an
// off-the-shelf format attractive in the first place.
//
// Input is real: the first 262,144 bytes (two maximum-size zstd blocks) of
// ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin compressed with
// `zstd -19 -q`, the same setting docs/blob-compression/candidate_comparison.py
// uses, giving 69,767 bytes (ratio 3.7574).
//
// The sample is two blocks rather than a full 780,000-byte payload because the
// full decode ran over 25 minutes under zkc without finishing. cycles/byte is
// intensive so a shorter sample measures the same quantity; the residual bias
// is that per-block entropy-table construction amortizes over less data, which
// overstates cost slightly. The ratio here (3.7574) is NOT comparable to the
// full-payload figure (3.9760) -- less history compresses worse -- so only the
// cycle figure should be taken from this run.
//
// NO DICTIONARY: std.compress.zstd rejects dictionary frames outright
// (DictionaryIdFlagUnsupported when dictionary_id_flag != 0), so this arm is
// dictionary-free. For the same corpus window a dictionary is worth 1.44% of
// ratio (4.1388 vs 4.0801), so comparisons against dictionaried arms are not
// like-for-like and should use the no-dictionary rows.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start decode, 11 = end decode (value = decoded length)
//   12 = FNV-1a of the output, checked by run.go against the corpus plaintext

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const profiling = verifier_ray.profiling;

const decompressed_len = 262_144;

const compressed: []const u8 = @embedFile("zstd_compressed.bin");

/// The frame declares a 780,000-byte window (`zstd -lv`), and std asserts the
/// buffer holds window_len + one maximum block.
const window_len = 1 << 20;
var window: [window_len + std.compress.zstd.block_size_max]u8 = @splat(0);
var out: [decompressed_len]u8 = @splat(0);

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
    var input: std.Io.Reader = .fixed(compressed);
    var d: std.compress.zstd.Decompress = .init(&input, &window, .{
        .window_len = window_len,
        .verify_checksum = false,
    });
    // readSliceAll fills the whole slice or fails; a short or failed decode is
    // reported as length 0 so run.go rejects it rather than timing a no-op.
    var decoded: u64 = decompressed_len;
    d.reader.readSliceAll(&out) catch {
        decoded = 0;
    };
    profiling.markR5Value(11, decoded);
    profiling.markR5Value(12, fnv1a(&out));

    accel.zkvm_exit(0);
}

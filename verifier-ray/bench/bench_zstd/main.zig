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

/// Upper bound accepted for the frame's declared window; the fixture's frame
/// declares its content size (262,144), which is well under this.
const window_len = 1 << 20;
var out: [decompressed_len]u8 = @splat(0);

/// Word-at-a-time `memcpy`, overriding the byte loop compiler-rt supplies for
/// this freestanding rv64im target. std's zstd decoder routes literal runs,
/// match copies and block buffer management through `memcpy`, which the PC
/// profile attributed 89.5% of the run to, so the byte loop dominated a
/// measurement that is supposed to be about the format.
///
/// `memcpy` is defined as non-overlapping, so whole words move unconditionally
/// -- no minimum-distance test is needed, unlike the LZSS self-copy.
export fn memcpy(noalias dest: [*]u8, noalias src: [*]const u8, len: usize) callconv(.c) [*]u8 {
    var i: usize = 0;
    while (i + 8 <= len) : (i += 8) {
        const word = std.mem.readInt(u64, src[i..][0..8], .little);
        std.mem.writeInt(u64, dest[i..][0..8], word, .little);
    }
    while (i < len) : (i += 1) dest[i] = src[i];
    return dest;
}

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
    // Direct mode: empty decoder buffer, stream straight into a fixed Writer
    // over the flat output. The written output doubles as the match-copy
    // window, which is exactly the shape of the DA use case (whole payload
    // decoded once into one buffer).
    //
    // Mode makes almost no difference here (indirect measured 716.16
    // cycles/byte, direct 710.16): the PC profile (run.go -pcprof) attributes
    // 89.5% of the run to `memcpy`, which compiler-rt lowers to a
    // byte-at-a-time loop on this freestanding rv64im target, and std's
    // decoder routes literal runs, match copies and block buffer management
    // through it -- about 109 bytes moved per output byte. That cost is a
    // property of the implementation on this target, not of the zstd format;
    // the remaining ~10% (entropy decode, sequence execution, bit reading)
    // comes to ~69 cycles/byte.
    var input: std.Io.Reader = .fixed(compressed);
    var d: std.compress.zstd.Decompress = .init(&input, &.{}, .{
        .window_len = window_len,
        .verify_checksum = false,
    });
    var w: std.Io.Writer = .fixed(&out);
    // A short or failed decode reports length 0 so run.go rejects it rather
    // than timing a no-op.
    const decoded: u64 = d.reader.streamRemaining(&w) catch 0;
    profiling.markR5Value(11, decoded);
    profiling.markR5Value(12, fnv1a(&out));

    accel.zkvm_exit(0);
}

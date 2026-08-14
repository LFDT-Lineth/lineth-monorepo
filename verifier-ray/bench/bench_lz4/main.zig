// Micro-benchmark: RISC-V cycle cost of decompressing a real blob-sized chunk
// with LZ4 (arm A1 of the evaluation plan). A1 is already dead on ratio --
// consistently below the deployed LZSS, see docs/blob-compression/README.md --
// so this exists to validate the measurement harness end to end on a genuine
// decoder before the more consequential arms (LZSS, zstd), not to make a case
// for LZ4 itself.
//
// Decoder: vendored jedisct1/zig-lz4 (vendor/lz4/, see that directory's
// root.zig for exact provenance and why it is vendored rather than a normal
// package dependency). Ported, not written: this benchmark calls the
// upstream decompression logic unmodified.
//
// Input (compressed_block.bin): the first 780,000 bytes of a real corpus
// payload (~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin),
// compressed with LZ4-HC level 9 (matching the deployed dictionary, dict.bin,
// and the `-9` level used throughout candidate_comparison.py) via the same
// vendored library on the host. Round-trip verified there: decompresses back
// to the original 780,000 bytes exactly. Compressed size 242,614 B (ratio
// 3.215 -- a few percent below the reference CLI's 3.453 on this window,
// because this is a different match-finder implementing the same HC level;
// the wire format and decode logic are unaffected).
//
// Produce-mode (decode into a RAM buffer) rather than verify-mode, per the
// bench_memory result: reading IN and read/writing RAM cost the same, so
// verify-mode would buy nothing for the decoder-surgery cost it requires.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start decode,     11 = end decode (value = decompressed length, so
//                                            the call cannot be optimized away)

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const lz4 = @import("lz4");
const profiling = verifier_ray.profiling;

const dict: []const u8 = @embedFile("dict.bin");
const compressed: []const u8 = @embedFile("compressed_block.bin");

var out_buf: [780_000]u8 = @splat(0);

/// FNV-1a, 64-bit. Checked by run.go against the corpus plaintext so a decode
/// that is fast because it is wrong cannot be reported as a result. Computed
/// after the timed region.
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
    const d_len = lz4.decompressSafeUsingDict(compressed, &out_buf, dict) catch unreachable;
    profiling.markR5Value(11, d_len);
    profiling.markR5Value(12, fnv1a(&out_buf));

    accel.zkvm_exit(0);
}

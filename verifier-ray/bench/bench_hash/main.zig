// Micro-benchmark: RISC-V cycle cost of Poseidon2-hashing a full uncompressed
// blob payload.
//
// This is the leading candidate for kappa_fix (see the evaluation plan, §5.4):
// the scheme-independent cost every decoder pays regardless of which
// compression arm wins. If this dominates the decoder costs measured
// elsewhere, decoder choice stops mattering and the answer collapses to
// "maximise ratio" -- so this number is worth having before investing further
// in any one decoder.
//
// Scope, deliberately narrowed. This hashes a payload buffer only. It does NOT
// implement 254-bit field-packing/unpacking or blob-consistency checks --
// those belong to the real data-availability guest, which is separate,
// non-trivial scope (EIP-4844 point evaluation, header parsing, batch-sum
// checks) better done as its own branch. Payload hashing is measured in
// isolation because it is the one component whose cost was previously
// unverified end-to-end: bench_compress measures a single Poseidon2 compress
// call, not accumulated cost over a realistic payload.
//
// Size chosen: MaxUncompressedBytes (780,000 bytes), the deployed circuit's
// current uncompressed-payload cap and the largest realistic single-blob
// payload (prover/lib/compressor/blob/v1/blob_maker.go).
//
// Content: synthetic, not real corpus data. verifier_ray.crypto.poseidon2's
// MDHasher.writeBytes requires every 4-byte big-endian word to already be a
// canonical KoalaBear field element (< 2_130_706_433); raw payload bytes are
// not guaranteed to satisfy that without the packing step this benchmark
// deliberately excludes. Poseidon2's cost is independent of the field values
// being permuted, so synthetic canonical words give the same cycle count a
// real packed payload would.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start hash,       11 = end hash (value = digest limb, so the hash
//                                          cannot be optimized away)
//
// Built with accelerators ON by default (unlike bench_compress): this
// benchmark measures representative production cost, not a software/hardware
// comparison. Pass -Ddisable-accelerators to measure the software path.

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;
const profiling = verifier_ray.profiling;

const payload_bytes: u64 = 780_000;
const n_words: u64 = payload_bytes / field.bytes; // 195,000; payload_bytes is a multiple of 4

var buf: [n_words * field.bytes]u8 = @splat(0);

pub export fn main() noreturn {
    // Fill with synthetic canonical field elements. Volatile seed so the fill
    // loop cannot be constant-folded or hoisted.
    var seed: u32 = 0x12345678;
    var i: u64 = 0;
    while (i < n_words) : (i += 1) {
        const s = (@as(*volatile u32, &seed)).*;
        const value = (s +% @as(u32, @truncate(i))) % field.modulus;
        std_mem_writeIntBig(buf[i * field.bytes ..][0..field.bytes], value);
        seed = s +% 1;
    }

    profiling.markR5Value(0, 0);
    var j: u64 = 0;
    while (j < 1) : (j += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    profiling.markR5Value(10, 0);
    var hasher = poseidon2.MDHasher.init();
    hasher.writeBytes(&buf) catch unreachable;
    const digest = hasher.sumDigest();
    profiling.markR5Value(11, digest[0].value);

    accel.zkvm_exit(0);
}

fn std_mem_writeIntBig(dst: *[field.bytes]u8, value: u32) void {
    dst[0] = @truncate(value >> 24);
    dst[1] = @truncate(value >> 16);
    dst[2] = @truncate(value >> 8);
    dst[3] = @truncate(value);
}

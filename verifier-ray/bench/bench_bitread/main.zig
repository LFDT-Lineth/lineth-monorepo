// Cycle cost of reading bit fields from a compressed stream, the operation the
// LZSS decoder performs most: one 8-bit read per literal, and literals are
// ~90% of the symbols in our corpus.
//
// Two readers are measured against each other. `Legacy` is the bit-at-a-time
// reader bench_lzss shipped first; `lzss.BitReader` is the 64-bit-container
// reader that replaced it. The comparison is the justification for that
// change, so both stay here.
//
// Reference points from the other micro-benchmarks: a single byte copy -- the
// per-byte cost of the backref path -- is ~3.0 cycles (bench_memory), and
// bench_lz4 decodes at 7.26 cycles/byte end to end.
//
// Reads of 8 bits (a literal, and the plain format's length field) and 21 bits
// (the widest field either format reads: a far-backref address) are timed
// separately. The container reader is timed as it is actually used, one
// `refill` per symbol, so its refill cost is charged to it and not hidden.
//
// Marker IDs:
//    0 = start baseline,       1 = end baseline
//   10 = start legacy 8-bit,  11 = end
//   20 = start legacy 21-bit, 21 = end
//   30 = start container 8-bit,  31 = end
//   40 = start container 21-bit, 41 = end

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const lzss = @import("lzss");
const profiling = verifier_ray.profiling;

const N: u64 = 4096;

/// The reader bench_lzss used before the container reader: one loop iteration
/// per bit, no buffering.
const Legacy = struct {
    buf: []const u8,
    pos: usize = 0,

    fn bits(self: *Legacy, n: u6) u64 {
        var v: u64 = 0;
        var i: u6 = 0;
        while (i < n) : (i += 1) {
            const byte = self.buf[self.pos >> 3];
            const bit = (byte >> @intCast(7 - (self.pos & 7))) & 1;
            v = (v << 1) | bit;
            self.pos += 1;
        }
        return v;
    }
};

// Enough backing bytes for N reads of up to 21 bits each, with room to spare.
var backing: [(N * 21 / 8) + 16]u8 = @splat(0xA5);

pub export fn main() noreturn {
    profiling.markR5Value(0, 0);
    var i: u64 = 0;
    while (i < 1) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    var checksum: u64 = 0;

    profiling.markR5Value(10, 0);
    var legacy8 = Legacy{ .buf = &backing };
    i = 0;
    while (i < N) : (i += 1) {
        checksum +%= legacy8.bits(8);
    }
    profiling.markR5Value(11, checksum);

    profiling.markR5Value(20, 0);
    var legacy21 = Legacy{ .buf = &backing };
    i = 0;
    while (i < N) : (i += 1) {
        checksum +%= legacy21.bits(21);
    }
    profiling.markR5Value(21, checksum);

    profiling.markR5Value(30, 0);
    var fast8 = lzss.BitReader.init(&backing, 0);
    i = 0;
    while (i < N) : (i += 1) {
        fast8.refill();
        checksum +%= fast8.take(8);
    }
    profiling.markR5Value(31, checksum);

    profiling.markR5Value(40, 0);
    var fast21 = lzss.BitReader.init(&backing, 0);
    i = 0;
    while (i < N) : (i += 1) {
        fast21.refill();
        checksum +%= fast21.take(21);
    }
    profiling.markR5Value(41, checksum);

    accel.zkvm_exit(0);
}

// Micro-benchmark: is the bit-by-bit BitReader in bench_lzss/lzss.zig actually
// the bottleneck in the LZSS guest run, or just structurally suspicious?
//
// bench_lzss's full run (two 780,000-byte decodes) ran 20+ minutes and 350M+
// trace lines before being killed -- roughly 20x bench_lz4's single decode
// (56s, 17M lines) for double the output. The bit reader (one loop iteration
// PER BIT, always) was the obvious suspect by comparison to LZ4's byte-aligned
// format and to icza/bitio's actual cached-bits design, which reads whole
// bytes when possible. But "structurally suspicious" is not "measured", so
// this isolates exactly that one operation.
//
// Copied verbatim from lzss.zig (not reimplemented) so this measures the
// actual code under test, not a stand-in for it.
//
// Compared directly against bench_memory's already-measured ram_copy result
// (~3.0 cycles/iteration for a single self-referencing byte copy) -- the
// backref-copy path's per-byte cost -- since literals (the dominant symbol,
// ~90% of the stream per docs/blob-compression's symbol-frequency analysis)
// go through an 8-bit bits() call, while backref bytes go through the copy
// loop already measured. If bits(8) costs much more than ~3 cycles, the
// literal path -- not the copy path -- is where the time is going.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start bits(8) x N, 11 = end
//   20 = start bits(21) x N, 21 = end (the widest field LZSS reads: far-backref address)

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const profiling = verifier_ray.profiling;

const N: u64 = 4096;

// Verbatim copy of lzss.zig's BitReader, unchanged.
const BitReader = struct {
    buf: []const u8,
    pos: usize = 0,

    fn bits(self: *BitReader, n: u6) u64 {
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
    var r8 = BitReader{ .buf = &backing };
    i = 0;
    while (i < N) : (i += 1) {
        checksum +%= r8.bits(8);
    }
    profiling.markR5Value(11, checksum);

    profiling.markR5Value(20, 0);
    var r21 = BitReader{ .buf = &backing };
    i = 0;
    while (i < N) : (i += 1) {
        checksum +%= r21.bits(21);
    }
    profiling.markR5Value(21, checksum);

    accel.zkvm_exit(0);
}

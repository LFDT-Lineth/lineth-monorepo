// Cycle cost of reading bit fields from a compressed stream, the operation the
// LZSS decoder performs most: one 8-bit read per literal, and literals are
// ~90% of the symbols in our corpus.
//
// Four reader designs are compared. Each is driven the way `lzss.decompress`
// would drive it -- one refill/ensure per symbol, sized for the widest symbol
// either format reads (19-bit code + 1-bit flag + 21-bit address = 41 bits) --
// so these are per-symbol costs for the real access pattern, not idealized
// throughput.
//
//   Legacy    one loop iteration per bit, no buffering.
//   Accum     lzss.BitReader as shipped: 64-bit container, byte-loop refill,
//             MSB-first packing (matches icza/bitio, which consensys/compress
//             uses).
//   MsbWide   absolute bit position, container recomputed per reload from one
//             64-bit big-endian load. On a little-endian target that lowers to
//             a native load plus @byteSwap, which without the Zbb `rev8`
//             instruction becomes a 3-level masked shift-exchange.
//   LsbWide   same, but LSB-first packing: the load is native-endian, so no
//             permutation is needed at all. This is the convention DEFLATE and
//             zstd use.
//
// The comparison answers two questions at once: whether this target executes
// wide misaligned loads at all (if not, MsbWide and LsbWide collapse toward
// Accum), and what the MSB-first packing convention costs when it does.
//
// Reference points from the other micro-benchmarks: a single byte copy is
// ~3.0 cycles (bench_memory), and bench_lz4 decodes at 7.26 cycles/byte.
//
// Marker IDs: baseline 0/1, then start/end per reader and width --
//   Legacy 10/11 (8-bit) 20/21 (21-bit)
//   Accum  30/31         40/41
//   MsbWide 50/51        60/61
//   LsbWide 70/71        80/81

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const lzss = @import("lzss");
const profiling = verifier_ray.profiling;

const N: u64 = 4096;

/// Widest sequence a symbol can require, matching `lzss.decompress`.
const symbol_bits: u32 = 41;

/// One loop iteration per bit, no buffering.
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

/// MSB-first, container recomputed per reload from one big-endian load.
const MsbWide = struct {
    buf: []const u8,
    bit_pos: usize = 0,
    /// Pending bits, left-aligned: bit 63 is the next bit out.
    acc: u64 = 0,
    valid: u32 = 0,

    fn reload(self: *MsbWide) void {
        const byte_i = self.bit_pos >> 3;
        const shift: u6 = @intCast(self.bit_pos & 7);
        const chunk = std.mem.readInt(u64, self.buf[byte_i..][0..8], .big);
        self.acc = chunk << shift;
        self.valid = 64 - @as(u32, shift);
    }

    fn ensure(self: *MsbWide, n: u32) void {
        if (self.valid < n) self.reload();
    }

    fn take(self: *MsbWide, n: u6) u64 {
        const v = self.acc >> @intCast(64 - @as(u32, n));
        self.acc <<= n;
        self.valid -= n;
        self.bit_pos += n;
        return v;
    }
};

/// LSB-first, container recomputed per reload from one native-endian load.
const LsbWide = struct {
    buf: []const u8,
    bit_pos: usize = 0,
    /// Pending bits, right-aligned: bit 0 is the next bit out.
    acc: u64 = 0,
    valid: u32 = 0,

    fn reload(self: *LsbWide) void {
        const byte_i = self.bit_pos >> 3;
        const shift: u6 = @intCast(self.bit_pos & 7);
        const chunk = std.mem.readInt(u64, self.buf[byte_i..][0..8], .little);
        self.acc = chunk >> shift;
        self.valid = 64 - @as(u32, shift);
    }

    fn ensure(self: *LsbWide, n: u32) void {
        if (self.valid < n) self.reload();
    }

    fn take(self: *LsbWide, n: u6) u64 {
        const v = self.acc & ((@as(u64, 1) << n) - 1);
        self.acc >>= n;
        self.valid -= n;
        self.bit_pos += n;
        return v;
    }
};

// Enough backing bytes for N reads of up to 21 bits each, plus the 8-byte
// lookahead a wide reload performs at the last bit position.
var backing: [(N * 21 / 8) + 16]u8 = @splat(0xA5);

pub export fn main() noreturn {
    profiling.markR5Value(0, 0);
    var i: u64 = 0;
    while (i < 1) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    var checksum: u64 = 0;

    inline for (.{ 8, 21 }, .{ 10, 20 }) |width, mark| {
        profiling.markR5Value(mark, 0);
        var r = Legacy{ .buf = &backing };
        i = 0;
        while (i < N) : (i += 1) {
            checksum +%= r.bits(width);
        }
        profiling.markR5Value(mark + 1, checksum);
    }

    inline for (.{ 8, 21 }, .{ 30, 40 }) |width, mark| {
        profiling.markR5Value(mark, 0);
        var r = lzss.BitReader.init(&backing, 0);
        i = 0;
        while (i < N) : (i += 1) {
            r.refill();
            checksum +%= r.take(width);
        }
        profiling.markR5Value(mark + 1, checksum);
    }

    inline for (.{ 8, 21 }, .{ 50, 60 }) |width, mark| {
        profiling.markR5Value(mark, 0);
        var r = MsbWide{ .buf = &backing };
        r.reload();
        i = 0;
        while (i < N) : (i += 1) {
            r.ensure(symbol_bits);
            checksum +%= r.take(width);
        }
        profiling.markR5Value(mark + 1, checksum);
    }

    inline for (.{ 8, 21 }, .{ 70, 80 }) |width, mark| {
        profiling.markR5Value(mark, 0);
        var r = LsbWide{ .buf = &backing };
        r.reload();
        i = 0;
        while (i < N) : (i += 1) {
            r.ensure(symbol_bits);
            checksum +%= r.take(width);
        }
        profiling.markR5Value(mark + 1, checksum);
    }

    accel.zkvm_exit(0);
}

//! Decoder for consensys/compress's v3 LZSS wire format. The three-byte header
//! remains big-endian and the compressed payload is packed least-significant
//! bit first. Literals and back-reference lengths share a 512-symbol canonical
//! Huffman alphabet; a back-reference is followed by one near/far bit and a
//! 14- or 21-bit address.

const std = @import("std");

pub const near_addr_bits: u6 = 14;
pub const far_addr_bits: u6 = 21;
const header_size = 3; // version (u16 BE) + noCompression flag

/// Bit reservoir for the v3 Huffman payload. Bytes and fields enter at the
/// low end, matching Go's bitReader: the next `n` bits are `acc & mask(n)`.
/// On rv64 this avoids the byte reversal and variable left shifts required by
/// the old MSB-first stream, and permits raw type/address fields to be taken
/// several bits at a time.
pub const BitReader = struct {
    buf: []const u8,
    pos: usize,
    acc: u64 = 0,
    cnt: u32 = 0,

    pub fn init(buf: []const u8, byte_offset: usize) BitReader {
        return .{ .buf = buf, .pos = byte_offset };
    }

    pub fn refill(self: *BitReader) void {
        while (self.cnt <= 56) {
            const byte: u64 = if (self.pos < self.buf.len) self.buf[self.pos] else 0;
            self.acc |= byte << @intCast(self.cnt);
            self.pos += 1;
            self.cnt += 8;
        }
    }

    pub fn peek(self: *const BitReader, n: u6) u64 {
        return self.acc & ((@as(u64, 1) << n) - 1);
    }

    pub fn skip(self: *BitReader, n: u6) void {
        self.acc >>= n;
        self.cnt -= n;
    }

    pub fn take(self: *BitReader, n: u6) u64 {
        const v = self.peek(n);
        self.skip(n);
        return v;
    }
};

/// Canonical Huffman decode tables, built once at comptime from a fixed code-
/// length array. Port of lzss/huffman.go's `newHuffmanTable` (the canonical
/// code assignment loop) and `readHuffmanSymbol` (the bit-by-bit walk), at
/// commit c509f05. Because the table is comptime-known, table CONSTRUCTION
/// costs nothing at guest runtime -- only the per-symbol decode loop runs.
pub const HuffmanTable = struct {
    const alphabet = 512;
    const max_len = 19; // measured max from hufftable training ("code lengths 8..19, Kraft 0.984375000")

    /// Width of the direct-lookup table. The trained table's code lengths are
    /// distributed 8:213, 9:54, 10:20, 11:21, 12:33, 13:35, 14:47, 15:43,
    /// 16:32, 17:11, 18:1, 19:2, so codes of length <= 12 carry Kraft weight
    /// 0.9753 -- for an optimal code that is also, to a good approximation, the
    /// probability mass. So ~97.5% of symbols resolve in a single lookup and
    /// only ~2.5% reach the canonical walk below. Covering all 19 bits would
    /// need 2^19 entries for essentially the same hit rate; 12 bits needs 4096,
    /// or 12 KiB of read-only data.
    const lookup_bits: u6 = 12;
    const lookup_size = 1 << lookup_bits;
    const secondary_bits = max_len - lookup_bits;
    const secondary_size = 1 << secondary_bits;
    const long_prefix_count = 37;

    /// Each entry packs `(code_length << 9) | symbol`; zero is the invalid
    /// sentinel because every real code is at least eight bits long.
    lookup: [lookup_size]u16,
    /// One-based secondary-table number for a long-code prefix, or zero when
    /// the primary entry resolves directly.
    long_group: [lookup_size]u8,
    /// Only 37 of the 4096 primary prefixes contain long codes. Giving each a
    /// dense seven-bit table resolves every 13..19-bit code with one more
    /// indexed load instead of a canonical walk or per-symbol bit reversal.
    secondary: [long_prefix_count * secondary_size]u16,

    pub fn build(comptime lengths: [alphabet]u8) HuffmanTable {
        comptime {
            // Insertion sort over 512 entries, plus the lookup-table fills.
            @setEvalBranchQuota(1 << 20);
            var table: HuffmanTable = .{
                .lookup = @splat(0),
                .long_group = @splat(0),
                .secondary = @splat(0),
            };
            var count: [max_len + 1]usize = @splat(0);
            for (lengths) |l| {
                if (l > max_len) @compileError("HuffmanTable.max_len is too small for this table");
                count[l] += 1;
            }

            // Stable sort symbols by (length, symbol), matching Go's
            // `sort.Slice` comparator in newHuffmanTable.
            var symbols: [alphabet]u16 = undefined;
            for (0..alphabet) |i| symbols[i] = i;
            std.sort.insertion(u16, &symbols, lengths, struct {
                fn lessThan(ls: [alphabet]u8, a: u16, b: u16) bool {
                    return ls[a] < ls[b] or (ls[a] == ls[b] and a < b);
                }
            }.lessThan);

            var code: u64 = 0;
            var index: usize = 0;
            var reversed_code: [alphabet]u64 = @splat(0);
            var sorted_symbol: [alphabet]u16 = @splat(0);
            var length: usize = 1;
            while (length <= max_len) : (length += 1) {
                code <<= 1;
                if (code + count[length] > @as(u64, 1) << @intCast(length)) {
                    @compileError("Huffman table is oversubscribed");
                }
                var i: usize = 0;
                while (i < count[length]) : (i += 1) {
                    const symbol = symbols[index + i];
                    sorted_symbol[index + i] = symbol;
                    reversed_code[symbol] = @bitReverse(code) >> @intCast(64 - length);
                    code += 1;
                }
                index += count[length];
            }
            if (index != alphabet) @compileError("not every symbol was assigned a code");

            // Expand every short, reversed code into the direct table. Since
            // the next wire bit is bit zero, unconstrained following bits are
            // the high bits of the lookup index.
            index = 0;
            for (1..lookup_bits + 1) |l| {
                for (0..count[l]) |i| {
                    const symbol = sorted_symbol[index + i];
                    const span = 1 << (lookup_bits - l);
                    for (0..span) |k| {
                        const lookup_index = reversed_code[symbol] | (k << l);
                        table.lookup[lookup_index] = (@as(u16, l) << 9) | symbol;
                    }
                }
                index += count[l];
            }

            var groups: usize = 0;
            for (lookup_bits + 1..max_len + 1) |l| {
                for (0..count[l]) |i| {
                    const symbol = sorted_symbol[index + i];
                    const primary: usize = @intCast(reversed_code[symbol] & (lookup_size - 1));
                    if (table.long_group[primary] == 0) {
                        groups += 1;
                        if (groups > long_prefix_count) @compileError("increase HuffmanTable.long_prefix_count");
                        table.long_group[primary] = groups;
                    }
                    const group = table.long_group[primary] - 1;
                    const significant = l - lookup_bits;
                    const suffix = reversed_code[symbol] >> lookup_bits;
                    const span = 1 << (secondary_bits - significant);
                    for (0..span) |k| {
                        const secondary_index = @as(usize, group) * secondary_size + suffix + (k << significant);
                        table.secondary[secondary_index] = (@as(u16, l) << 9) | symbol;
                    }
                }
                index += count[l];
            }
            if (groups != long_prefix_count) @compileError("update HuffmanTable.long_prefix_count for this table");
            return table;
        }
    }

    /// Decodes the symbol lzss/huffman.go's `readHuffmanSymbol` decodes.
    /// Codes of length <= `lookup_bits` resolve in one table lookup; the ~2.5%
    /// that are longer resolve in one dense secondary lookup.
    ///
    /// Returns null at a clean end-of-stream (mirrors the `io.EOF` case for a
    /// partial final byte); this benchmark instead stops once the expected
    /// output length is reached, so null is not expected in practice here.
    ///
    /// The caller must have refilled `r` this iteration.
    fn decode(self: *const HuffmanTable, r: *BitReader) ?u16 {
        const index: usize = @intCast(r.peek(lookup_bits));
        var entry = self.lookup[index];
        if (entry == 0) {
            const group = self.long_group[index];
            if (group == 0) return null;
            const suffix: usize = @intCast((r.acc >> lookup_bits) & (secondary_size - 1));
            entry = self.secondary[(@as(usize, group) - 1) * secondary_size + suffix];
            if (entry == 0) return null;
        }
        r.skip(@intCast(entry >> 9));
        return entry & 0x1ff;
    }
};

/// Decompress `src` into `dst[0..expected_len]`, returning `expected_len`.
///
/// `expected_len` is known up front here because the encoder already told us
/// (blob_maker.go's batch-sum check plays the same role in production), so
/// the loop runs until output is satisfied rather than replicating the Go
/// decoder's until-EOF-on-a-partial-final-byte termination -- a
/// simplification available specifically because the output length is a
/// trusted input, not something this benchmark needs to discover.
pub fn decompress(
    comptime table: HuffmanTable,
    src: []const u8,
    dst: []u8,
    dict: []const u8,
    expected_len: usize,
) usize {
    var r = BitReader.init(src, header_size);
    var out_i: usize = 0;

    while (out_i < expected_len) {
        // One refill per symbol covers every field read below: the widest
        // sequence is a 19-bit code, a 1-bit near/far flag and a 21-bit
        // address, and `refill` guarantees at least 57 bits.
        r.refill();

        const symbol = table.decode(&r).?;
        if (symbol < 256) {
            dst[out_i] = @truncate(symbol);
            out_i += 1;
            continue;
        }
        const length = symbol - 256 + 1;
        const far = r.take(1) == 1;

        const addr_bits: u6 = if (far) far_addr_bits else near_addr_bits;
        const address = @as(usize, @intCast(r.take(addr_bits))) + 1;

        // A "far" backref whose address reaches further back than produced
        // so far is served from the dictionary tail; every other backref
        // (near, or far within the output already produced) self-copies
        // byte-by-byte so overlapping runs extend correctly.
        if (far and address > out_i) {
            const dict_start = dict.len - (address - out_i);
            @memcpy(dst[out_i..][0..length], dict[dict_start..][0..length]);
            out_i += length;
        } else {
            // Sequential so that overlapping runs (address < length) extend
            // correctly. When the source is at least 8 bytes behind the
            // destination no 8-byte window overlaps itself, so whole words can
            // move at once; on the measured corpus 77.7% of output bytes and a
            // mean backref length of 40 take this path.
            var i: usize = 0;
            if (address >= 8) {
                while (i + 8 <= length) : (i += 8) {
                    const word = std.mem.readInt(u64, dst[out_i + i - address ..][0..8], .little);
                    std.mem.writeInt(u64, dst[out_i + i ..][0..8], word, .little);
                }
            }
            while (i < length) : (i += 1) {
                dst[out_i + i] = dst[out_i + i - address];
            }
            out_i += length;
        }
    }
    return out_i;
}

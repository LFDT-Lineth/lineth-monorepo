//! A single, comptime-parameterized decoder for two wire formats produced by
//! consensys/compress's lzss package:
//!
//!   decompress(false, ...)  -- the deployed format (module v0.3.0, arm A0):
//!     literal = raw byte; 0xFE/0xFF are backref delimiters; length = raw 8
//!     bits; address = raw 14 (short) or 21 (dynamic) bits.
//!
//!   decompress(true, ...)   -- the unmerged Huffman-on-lengths format
//!     (github.com/consensys/compress @ c509f05, branch feat/huffman-on-lengths,
//!     arm "lzss+huffman"): one canonical-Huffman symbol over a combined
//!     512-entry alphabet (0..255 = literal byte, 256..511 = backref length
//!     1..256), then ONE raw flag bit for near/far, then the same raw 14/21
//!     address bits as above. No delimiter bytes; the Huffman code carries
//!     that information instead.
//!
//! This is a new implementation -- no existing Zig port of this format
//! exists to vendor, unlike bench_lz4 -- but it is a direct transliteration
//! of the Go reference, not an independent design: every branch below is
//! commented with the exact Go source (file, and commit for the Huffman
//! variant) it reproduces. Validated by round-tripping real corpus data
//! through both the Go implementation and this one; see main.zig.
//!
//! Both variants share the backref copy logic and the 14/21-bit address
//! encoding (backref.go's `readFrom` in both the v0.3.0 module and the
//! Huffman branch: `address = raw_bits + 1`, identical between the two).
//! They differ in how the next symbol is obtained. The plain format also
//! depends on the dictionary being pre-augmented against its delimiter bytes
//! where needed (see the comment on `decompress` below) -- the guest treats
//! that as a property of the trusted, embedded dictionary, not something it
//! re-derives.

const std = @import("std");

pub const near_addr_bits: u6 = 14;
pub const far_addr_bits: u6 = 21;
const sym_short: u8 = 0xFE;
const sym_dynamic: u8 = 0xFF;
const header_size = 3; // version (u16 BE) + noCompression flag, both formats

/// MSB-first bit reader over a byte slice, yielding the same bit sequence as
/// icza/bitio (the reader consensys/compress uses). Structured as zstd's
/// `BIT_DStream_t`: `acc` buffers up to 64 bits left-aligned, so one refill
/// serves a whole symbol and each field costs a shift and a mask.
///
///   peek(n) = acc >> (64 - n)      skip(n) = acc <<= n
///
/// `refill` is an explicit byte loop because the guest target is
/// rv64im_zicclsm: with no Zbb extension there is no `rev8`, so a big-endian
/// `readInt(u64, ...)` lowers to this same loop, and writing it out keeps the
/// cost visible and needs no misaligned wide load.
pub const BitReader = struct {
    buf: []const u8,
    /// Index of the next byte to pull into `acc` (NOT a bit position).
    pos: usize,
    /// Pending bits, left-aligned: bit 63 is the next bit to be consumed.
    acc: u64 = 0,
    /// Number of valid bits in `acc`, counted down from the MSB.
    cnt: u32 = 0,

    pub fn init(buf: []const u8, byte_offset: usize) BitReader {
        return .{ .buf = buf, .pos = byte_offset };
    }

    /// Top up `acc` to at least 57 valid bits. Reading past the end of `buf`
    /// yields zero bytes: the caller stops on output length, not on EOF, so
    /// those padding bits are never part of a real symbol -- they only keep
    /// `cnt` from underflowing while the final genuine symbol is decoded.
    pub fn refill(self: *BitReader) void {
        while (self.cnt <= 56) {
            const byte: u64 = if (self.pos < self.buf.len) self.buf[self.pos] else 0;
            self.acc |= byte << @intCast(56 - self.cnt);
            self.pos += 1;
            self.cnt += 8;
        }
    }

    /// Next `n` bits without consuming them. Requires 1 <= n <= cnt.
    pub fn peek(self: *const BitReader, n: u6) u64 {
        return self.acc >> @intCast(64 - @as(u32, n));
    }

    pub fn skip(self: *BitReader, n: u6) void {
        self.acc <<= n;
        self.cnt -= n;
    }

    /// Consume `n` bits. The caller must have called `refill` this iteration;
    /// one refill covers a whole symbol, since the widest sequence either
    /// format reads between refills is 19 + 1 + 21 = 41 bits < 57.
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

    /// `lookup_len[i] == 0` marks an index whose code is longer than
    /// `lookup_bits`; every real code length here is 8..19, so 0 is free as a
    /// sentinel.
    lookup_symbol: [lookup_size]u16,
    lookup_len: [lookup_size]u8,

    first_code: [max_len + 1]u64,
    first_index: [max_len + 1]usize,
    count: [max_len + 1]usize,
    sorted_symbol: [alphabet]u16,

    pub fn build(comptime lengths: [alphabet]u8) HuffmanTable {
        comptime {
            // Insertion sort over 512 entries, plus ~4096 lookup-table fills.
            @setEvalBranchQuota(1 << 20);
            var table: HuffmanTable = .{
                .lookup_symbol = @splat(0),
                .lookup_len = @splat(0),
                .first_code = @splat(0),
                .first_index = @splat(0),
                .count = @splat(0),
                .sorted_symbol = @splat(0),
            };
            for (lengths) |l| {
                if (l > max_len) @compileError("HuffmanTable.max_len is too small for this table");
                table.count[l] += 1;
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
            var length: usize = 1;
            while (length <= max_len) : (length += 1) {
                code <<= 1;
                if (code + table.count[length] > @as(u64, 1) << @intCast(length)) {
                    @compileError("Huffman table is oversubscribed");
                }
                table.first_code[length] = code;
                table.first_index[length] = index;
                var i: usize = 0;
                while (i < table.count[length]) : (i += 1) {
                    table.sorted_symbol[index + i] = symbols[index + i];
                    code += 1;
                }
                index += table.count[length];
            }
            if (index != alphabet) @compileError("not every symbol was assigned a code");

            // Expand every code of length <= lookup_bits into the direct table.
            // A code of length l occupies the 2^(lookup_bits - l) indices that
            // share it as a prefix, so peeking a fixed `lookup_bits` bits lands
            // on the right entry whatever follows the code.
            for (1..lookup_bits + 1) |l| {
                for (0..table.count[l]) |i| {
                    const symbol = table.sorted_symbol[table.first_index[l] + i];
                    const span = 1 << (lookup_bits - l);
                    const base = (table.first_code[l] + i) << (lookup_bits - l);
                    for (0..span) |k| {
                        table.lookup_symbol[base + k] = symbol;
                        table.lookup_len[base + k] = l;
                    }
                }
            }
            return table;
        }
    }

    /// Decodes the symbol lzss/huffman.go's `readHuffmanSymbol` decodes.
    /// Codes of length <= `lookup_bits` resolve in one table lookup; the ~2.5%
    /// that are longer fall to the canonical tables, peeking all `max_len` bits
    /// once and comparing whole codes per candidate length.
    ///
    /// Returns null at a clean end-of-stream (mirrors the `io.EOF` case for a
    /// partial final byte); this benchmark instead stops once the expected
    /// output length is reached, so null is not expected in practice here.
    ///
    /// The caller must have refilled `r` this iteration.
    fn decode(self: *const HuffmanTable, r: *BitReader) ?u16 {
        const index: usize = @intCast(r.peek(lookup_bits));
        const length = self.lookup_len[index];
        if (length != 0) {
            r.skip(@intCast(length));
            return self.lookup_symbol[index];
        }

        const wide = r.peek(max_len);
        var l: u6 = lookup_bits + 1;
        while (l <= max_len) : (l += 1) {
            const code = wide >> @intCast(max_len - @as(u32, l));
            const first = self.first_code[l];
            if (code < first) continue;
            const offset = code - first;
            if (offset < self.count[l]) {
                r.skip(l);
                return self.sorted_symbol[self.first_index[l] + @as(usize, @intCast(offset))];
            }
        }
        return null;
    }
};

/// The plain format's far-backref addressing depends on the dictionary's
/// exact byte length, which for the real compressor is whatever
/// lzss/compress.go's `AugmentDict` (v0.3.0) decided offline -- it appends
/// its two delimiter bytes (0xFE/0xFF) unless they already occur in the
/// dictionary, which they do for ours (checked directly: 151 and 597
/// occurrences respectively), making it a no-op here. The decoder is handed
/// the dictionary as a fixed, trusted input and does not re-derive whether
/// augmentation was needed; that is a property of the embedded data, decided
/// once when it was prepared, not logic the guest carries.
///
/// Decompress `src` into `dst[0..expected_len]`, returning `expected_len`.
///
/// `expected_len` is known up front here because the encoder already told us
/// (blob_maker.go's batch-sum check plays the same role in production), so
/// the loop runs until output is satisfied rather than replicating the Go
/// decoder's until-EOF-on-a-partial-final-byte termination -- a
/// simplification available specifically because the output length is a
/// trusted input, not something this benchmark needs to discover.
pub fn decompress(
    comptime huffman: bool,
    comptime table: if (huffman) HuffmanTable else void,
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

        var length: usize = undefined;
        var far: bool = undefined;

        if (huffman) {
            // huffman.go's Decompress main loop, c509f05.
            const symbol = table.decode(&r).?;
            if (symbol < 256) {
                dst[out_i] = @truncate(symbol);
                out_i += 1;
                continue;
            }
            length = symbol - 256 + 1;
            far = r.take(1) == 1;
        } else {
            // decompress.go's Decompress main loop, v0.3.0. The cursor is
            // generally not byte-aligned here, since 14- and 21-bit address
            // reads leave it mid-byte; `take(8)` reads the 8 bits at the
            // cursor, matching readUnalignedByte in icza/bitio.
            const sym: u8 = @truncate(r.take(8));
            if (sym != sym_short and sym != sym_dynamic) {
                dst[out_i] = sym;
                out_i += 1;
                continue;
            }
            far = sym == sym_dynamic;
            length = @as(usize, @intCast(r.take(8))) + 1;
        }

        const addr_bits: u6 = if (far) far_addr_bits else near_addr_bits;
        const address = @as(usize, @intCast(r.take(addr_bits))) + 1;

        // Shared copy logic, identical in both Go source files: a "far"
        // backref whose address reaches further back than we have produced
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

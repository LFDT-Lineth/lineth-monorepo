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

/// MSB-first bit reader over a byte slice, matching icza/bitio as used by
/// consensys/compress.
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

/// Canonical Huffman decode tables, built once at comptime from a fixed code-
/// length array. Port of lzss/huffman.go's `newHuffmanTable` (the canonical
/// code assignment loop) and `readHuffmanSymbol` (the bit-by-bit walk), at
/// commit c509f05. Because the table is comptime-known, table CONSTRUCTION
/// costs nothing at guest runtime -- only the per-symbol decode loop runs.
pub const HuffmanTable = struct {
    const alphabet = 512;
    const max_len = 19; // measured max from hufftable training ("code lengths 8..19, Kraft 0.984375000")

    first_code: [max_len + 1]u64,
    first_index: [max_len + 1]usize,
    count: [max_len + 1]usize,
    sorted_symbol: [alphabet]u16,

    pub fn build(comptime lengths: [alphabet]u8) HuffmanTable {
        comptime {
            @setEvalBranchQuota(alphabet * alphabet); // insertion sort over 512 entries
            var table: HuffmanTable = .{
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
            return table;
        }
    }

    /// Port of lzss/huffman.go's `readHuffmanSymbol`. Returns null at a clean
    /// end-of-stream (mirrors the `io.EOF` case for a partial final byte);
    /// this benchmark instead stops once the expected output length is
    /// reached, so null is not expected in practice here.
    fn decode(self: *const HuffmanTable, r: *BitReader) ?u16 {
        var code: u64 = 0;
        var length: u6 = 1;
        while (length <= max_len) : (length += 1) {
            code = (code << 1) | r.bits(1);
            const first = self.first_code[length];
            if (code < first) continue;
            const offset = code - first;
            if (offset < self.count[length]) {
                return self.sorted_symbol[self.first_index[length] + @as(usize, @intCast(offset))];
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
    var r = BitReader{ .buf = src, .pos = header_size * 8 };
    var out_i: usize = 0;

    while (out_i < expected_len) {
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
            far = r.bits(1) == 1;
        } else {
            // decompress.go's Decompress main loop, v0.3.0. bitio's
            // TryReadByte handles a bit cursor that is not byte-aligned
            // (readUnalignedByte in icza/bitio), which it always is here
            // after a 14- or 21-bit address read; `bits(8)` does the same.
            const sym: u8 = @truncate(r.bits(8));
            if (sym != sym_short and sym != sym_dynamic) {
                dst[out_i] = sym;
                out_i += 1;
                continue;
            }
            far = sym == sym_dynamic;
            length = @as(usize, @intCast(r.bits(8))) + 1;
        }

        const addr_bits: u6 = if (far) far_addr_bits else near_addr_bits;
        const address = @as(usize, @intCast(r.bits(addr_bits))) + 1;

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
            var i: usize = 0;
            while (i < length) : (i += 1) {
                dst[out_i] = dst[out_i - address];
                out_i += 1;
            }
        }
    }
    return out_i;
}

//! LZ4 HC: high compression mode of LZ4.
//! Based on the C reference implementation by Yann Collet.
//! Copyright (c) Yann Collet. All rights reserved. (BSD 2-Clause License)

const std = @import("std");
const mem = std.mem;
const Allocator = std.mem.Allocator;
const lz4 = @import("lz4.zig");

pub const MINMATCH = lz4.MINMATCH;
pub const WILDCOPYLENGTH = lz4.WILDCOPYLENGTH;
pub const LASTLITERALS = lz4.LASTLITERALS;
pub const MFLIMIT = lz4.MFLIMIT;
pub const ML_BITS = lz4.ML_BITS;
pub const ML_MASK = lz4.ML_MASK;
pub const RUN_BITS = lz4.RUN_BITS;
pub const RUN_MASK = lz4.RUN_MASK;
pub const LZ4_MAX_INPUT_SIZE = lz4.LZ4_MAX_INPUT_SIZE;
pub const LZ4_DISTANCE_MAX = lz4.LZ4_DISTANCE_MAX;
pub const LZ4_DISTANCE_ABSOLUTE_MAX = lz4.LZ4_DISTANCE_ABSOLUTE_MAX;

pub const LZ4HC_CLEVEL_MIN = 2;
pub const LZ4HC_CLEVEL_DEFAULT = 9;
pub const LZ4HC_CLEVEL_OPT_MIN = 10;
pub const LZ4HC_CLEVEL_MAX = 12;

pub const LZ4HC_DICTIONARY_LOGSIZE = 16;
pub const LZ4HC_MAXD = 1 << LZ4HC_DICTIONARY_LOGSIZE;
pub const LZ4HC_MAXD_MASK = LZ4HC_MAXD - 1;

pub const LZ4HC_HASH_LOG = 15;
pub const LZ4HC_HASHTABLESIZE = 1 << LZ4HC_HASH_LOG;
pub const LZ4HC_HASH_MASK = LZ4HC_HASHTABLESIZE - 1;

pub const OPTIMAL_ML = (ML_MASK - 1) + MINMATCH;
pub const LZ4_OPT_NUM = 1 << 12;

pub const LZ4MID_HASHLOG = LZ4HC_HASH_LOG - 1;
pub const LZ4MID_HASHTABLESIZE = 1 << LZ4MID_HASHLOG;
pub const LZ4MID_HASHSIZE = 8;

/// Golden ratio constants used by the match-finder hashes.
const hash_multiplier: u32 = 2654435761;
const hash_multiplier_64: u64 = 58295818150454627;

pub const Error = lz4.Error;

pub const Strategy = enum {
    /// Level 2: dual hash tables, 2 attempts.
    lz4mid,
    /// Levels 3-9: hash chains, 4-256 attempts.
    lz4hc,
    /// Levels 10-12: optimal parser, 96-16384 attempts.
    lz4opt,
};

pub const CompressionParams = struct {
    strat: Strategy,
    nbSearches: i32,
    targetLength: u32,
};

const clevel_table = [_]CompressionParams{
    .{ .strat = .lz4mid, .nbSearches = 2, .targetLength = 16 }, // 0, unused
    .{ .strat = .lz4mid, .nbSearches = 2, .targetLength = 16 }, // 1, unused
    .{ .strat = .lz4mid, .nbSearches = 2, .targetLength = 16 }, // 2
    .{ .strat = .lz4hc, .nbSearches = 4, .targetLength = 16 }, // 3
    .{ .strat = .lz4hc, .nbSearches = 8, .targetLength = 16 }, // 4
    .{ .strat = .lz4hc, .nbSearches = 16, .targetLength = 16 }, // 5
    .{ .strat = .lz4hc, .nbSearches = 32, .targetLength = 16 }, // 6
    .{ .strat = .lz4hc, .nbSearches = 64, .targetLength = 16 }, // 7
    .{ .strat = .lz4hc, .nbSearches = 128, .targetLength = 16 }, // 8
    .{ .strat = .lz4hc, .nbSearches = 256, .targetLength = 16 }, // 9
    .{ .strat = .lz4opt, .nbSearches = 96, .targetLength = 64 }, // 10
    .{ .strat = .lz4opt, .nbSearches = 512, .targetLength = 128 }, // 11
    .{ .strat = .lz4opt, .nbSearches = 16384, .targetLength = LZ4_OPT_NUM }, // 12
};

inline fn getCLevelParams(clevel: i32) CompressionParams {
    const level = if (clevel < 1) LZ4HC_CLEVEL_DEFAULT else @min(clevel, LZ4HC_CLEVEL_MAX);
    return clevel_table[@intCast(level)];
}

inline fn readU32(ptr: [*]const u8) u32 {
    return mem.readInt(u32, ptr[0..4], .little);
}

inline fn readU64(ptr: [*]const u8) u64 {
    return mem.readInt(u64, ptr[0..8], .little);
}

inline fn hashHC(sequence: u32) u32 {
    return (sequence *% hash_multiplier) >> ((MINMATCH * 8) - LZ4HC_HASH_LOG);
}

inline fn hashPtr(ptr: [*]const u8) u32 {
    return hashHC(readU32(ptr));
}

inline fn hashMid4Ptr(ptr: [*]const u8) u32 {
    return (readU32(ptr) *% hash_multiplier) >> (32 - LZ4MID_HASHLOG);
}

/// Hash the lower 56 bits of an 8-byte sequence.
inline fn hashMid8Ptr(ptr: [*]const u8) u32 {
    return @truncate(((readU64(ptr) << 8) *% hash_multiplier_64) >> (64 - LZ4MID_HASHLOG));
}

/// Count how many bytes starting at `ip` keep repeating `pattern`.
/// `pattern` must be a sample of a repetitive pattern of length 1, 2 or 4 bytes.
inline fn countPattern(ip: [*]const u8, iend: [*]const u8, pattern32: u32) usize {
    const istart = ip;
    var ptr = ip;

    const pattern64: u64 = @as(u64, pattern32) | (@as(u64, pattern32) << 32);

    while (@intFromPtr(ptr) + 7 < @intFromPtr(iend)) {
        const diff = readU64(ptr) ^ pattern64;
        if (diff != 0) {
            return @intFromPtr(ptr) + (@ctz(diff) >> 3) - @intFromPtr(istart);
        }
        ptr += 8;
    }

    var pattern_byte = pattern32;
    while (@intFromPtr(ptr) < @intFromPtr(iend)) {
        if (ptr[0] != @as(u8, @truncate(pattern_byte))) break;
        ptr += 1;
        pattern_byte >>= 8;
        if (pattern_byte == 0) pattern_byte = pattern32;
    }

    return @intFromPtr(ptr) - @intFromPtr(istart);
}

/// Count how many bytes before `ip` keep repeating `pattern`, down to `ilow`.
inline fn reverseCountPattern(ip: [*]const u8, ilow: [*]const u8, pattern: u32) usize {
    const istart = ip;
    var ptr = ip;

    while (@intFromPtr(ptr) >= @intFromPtr(ilow) + 4) {
        if (readU32(ptr - 4) != pattern) break;
        ptr -= 4;
    }

    const pattern_bytes = mem.asBytes(&pattern);
    var byte_idx: usize = 3;
    while (@intFromPtr(ptr) > @intFromPtr(ilow)) {
        if ((ptr - 1)[0] != pattern_bytes[byte_idx]) break;
        ptr -= 1;
        byte_idx = if (byte_idx == 0) 3 else byte_idx - 1;
    }

    return @intFromPtr(istart) - @intFromPtr(ptr);
}

/// A pattern is repetitive when it repeats with a period of 1, 2 or 4 bytes.
inline fn isRepetitivePattern(pattern: u32) bool {
    return ((pattern & 0xFFFF) == (pattern >> 16)) and ((pattern & 0xFF) == (pattern >> 24));
}

/// Count the number of identical bytes between two sequences.
inline fn lz4Count(p_in: [*]const u8, p_match: [*]const u8, p_in_limit: [*]const u8) usize {
    const len = @intFromPtr(p_in_limit) - @intFromPtr(p_in);
    return mem.indexOfDiff(u8, p_in[0..len], p_match[0..len]) orelse len;
}

/// Count identical bytes before `ip`/`match`, limited by `i_min`/`m_min`.
/// Returns a negative value (the backward extension).
inline fn countBack(ip: [*]const u8, match: [*]const u8, i_min: [*]const u8, m_min: [*]const u8) i32 {
    const min_dist = @min(
        @intFromPtr(ip) - @intFromPtr(i_min),
        @intFromPtr(match) - @intFromPtr(m_min),
    );
    var back: usize = 0;
    while (back < min_dist and (ip - back - 1)[0] == (match - back - 1)[0]) back += 1;
    return -@as(i32, @intCast(back));
}

/// Encode a literal run plus a match into the output buffer.
/// Advances `ip` past the match and resets `anchor`.
fn encodeSequence(
    ip: *[*]const u8,
    op: *[*]u8,
    anchor: *[*]const u8,
    match_len: i32,
    offset: i32,
    oend: [*]u8,
) Error!void {
    const lit_len: usize = @intFromPtr(ip.*) - @intFromPtr(anchor.*);

    const needed = (lit_len / 255) + lit_len + (2 + 1 + LASTLITERALS);
    if (@intFromPtr(op.*) + needed > @intFromPtr(oend)) return error.OutputTooSmall;

    const token = op.*;
    op.* += 1;

    if (lit_len >= RUN_MASK) {
        var len = lit_len - RUN_MASK;
        token[0] = RUN_MASK << ML_BITS;
        while (len >= 255) {
            op.*[0] = 255;
            op.* += 1;
            len -= 255;
        }
        op.*[0] = @truncate(len);
        op.* += 1;
    } else {
        token[0] = @as(u8, @truncate(lit_len)) << ML_BITS;
    }

    @memcpy(op.*[0..lit_len], anchor.*[0..lit_len]);
    op.* += lit_len;

    mem.writeInt(u16, op.*[0..2], @intCast(offset), .little);
    op.* += 2;

    const ml_code: usize = @intCast(match_len - MINMATCH);
    if (@intFromPtr(op.*) + (ml_code / 255) + (1 + LASTLITERALS) > @intFromPtr(oend)) {
        return error.OutputTooSmall;
    }

    if (ml_code >= ML_MASK) {
        token[0] += ML_MASK;
        var remaining = ml_code - ML_MASK;
        while (remaining >= 510) {
            op.*[0] = 255;
            op.*[1] = 255;
            op.* += 2;
            remaining -= 510;
        }
        if (remaining >= 255) {
            op.*[0] = 255;
            op.* += 1;
            remaining -= 255;
        }
        op.*[0] = @truncate(remaining);
        op.* += 1;
    } else {
        token[0] += @truncate(ml_code);
    }

    ip.* += @intCast(match_len);
    anchor.* = ip.*;
}

/// Compression context for LZ4 HC.
pub const Context = struct {
    hashTable: [LZ4HC_HASHTABLESIZE]u32,
    chainTable: [LZ4HC_MAXD]u16,
    /// Next block here to continue on current prefix.
    end: [*]const u8,
    /// Indexes are relative to this position.
    prefixStart: [*]const u8,
    /// Alternate reference for extDict.
    dictStart: [*]const u8,
    /// Below that point, need extDict.
    dictLimit: u32,
    /// Below that point, no more history.
    lowLimit: u32,
    /// Index from which to continue dictionary update.
    nextToUpdate: u32,
    compressionLevel: i16,
    /// Favor decompression speed if this flag is set.
    favorDecSpeed: i8,
    /// Stream has to be fully reset if this flag is set.
    dirty: i8,
    dictCtx: ?*const Context,

    pub fn init() Context {
        return .{
            .hashTable = @splat(0),
            .chainTable = @splat(0),
            .end = undefined,
            .prefixStart = undefined,
            .dictStart = undefined,
            .dictLimit = 0,
            .lowLimit = 0,
            .nextToUpdate = 0,
            .compressionLevel = LZ4HC_CLEVEL_DEFAULT,
            .favorDecSpeed = 0,
            .dirty = 0,
            .dictCtx = null,
        };
    }

    pub fn clearTables(self: *Context) void {
        @memset(&self.hashTable, 0);
        @memset(&self.chainTable, 0xFF);
    }

    pub fn initContext(self: *Context, start: [*]const u8) void {
        initInternal(self, start);
    }
};

pub const Match = struct {
    off: i32,
    len: i32,
    /// Backward extension (negative value).
    back: i32,
};

/// Optimal parser table entry.
const OptimalMatch = struct {
    /// Cost in bytes to reach this position.
    price: i32,
    /// Match offset (0 for a literal).
    off: i32,
    /// Match length (1 for a literal).
    mlen: i32,
    /// Total literal length.
    litlen: i32,
};

/// Byte cost of encoding a literal run.
inline fn literalsPrice(litlen: i32) i32 {
    var price: i32 = litlen;
    if (litlen >= @as(i32, RUN_MASK)) {
        price += 1 + @divTrunc(litlen - @as(i32, RUN_MASK), 255);
    }
    return price;
}

/// Byte cost of a sequence (literals + match). Requires `mlen >= MINMATCH`.
inline fn sequencePrice(litlen: i32, mlen: i32) i32 {
    var price: i32 = 1 + 2; // token + 16-bit offset
    price += literalsPrice(litlen);
    if (mlen >= @as(i32, ML_MASK + MINMATCH)) {
        price += 1 + @divTrunc(mlen - @as(i32, ML_MASK + MINMATCH), 255);
    }
    return price;
}

/// Update hash and chain tables up to `ip` (excluded).
fn insertHC(ctx: *Context, ip: [*]const u8) void {
    const prefix_ptr = ctx.prefixStart;
    const prefix_idx = ctx.dictLimit;
    const target: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);
    var idx = ctx.nextToUpdate;

    while (idx < target) : (idx += 1) {
        const offset: usize = @intCast(idx - prefix_idx);
        const h = hashPtr(prefix_ptr + offset);
        const prev_idx = ctx.hashTable[h];
        const delta: u32 = if (prev_idx > idx) LZ4_DISTANCE_MAX + 1 else idx - prev_idx;
        ctx.chainTable[idx & LZ4HC_MAXD_MASK] = @intCast(@min(delta, LZ4_DISTANCE_MAX));
        ctx.hashTable[h] = idx;
    }

    ctx.nextToUpdate = target;
}

/// Insert positions up to `ip` and return the longest match found there.
fn insertAndFindBestMatch(
    ctx: *Context,
    ip: [*]const u8,
    i_limit: [*]const u8,
    max_nb_attempts: i32,
    pattern_analysis: bool,
) Match {
    insertHC(ctx, ip);
    return insertAndGetWiderMatch(
        ctx,
        ip,
        ip, // no backward extension
        i_limit,
        MINMATCH - 1,
        max_nb_attempts,
        pattern_analysis,
    );
}

/// Core match finder with backward extension support.
fn insertAndGetWiderMatch(
    ctx: *Context,
    ip: [*]const u8,
    i_low_limit: [*]const u8,
    i_high_limit: [*]const u8,
    longest: i32,
    max_nb_attempts: i32,
    pattern_analysis: bool,
) Match {
    const prefix_ptr = ctx.prefixStart;
    const prefix_idx = ctx.dictLimit;
    const ip_index: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);
    const within_start_distance = ctx.lowLimit + (LZ4_DISTANCE_MAX + 1) > ip_index;
    const lowest_match_index: u32 = if (within_start_distance) ctx.lowLimit else ip_index - LZ4_DISTANCE_MAX;
    const dict_start = ctx.dictStart;
    const dict_idx = ctx.lowLimit;
    var nb_attempts = max_nb_attempts;
    const pattern = readU32(ip);

    var result: Match = .{ .off = 0, .len = longest, .back = 0 };

    var match_index = ctx.hashTable[hashPtr(ip)];
    // 0 means the hash table entry is uninitialized.
    if (match_index == 0) return result;

    while (match_index > 0 and nb_attempts > 0) {
        if (match_index > ip_index or (ip_index - match_index) > LZ4_DISTANCE_MAX) break;

        nb_attempts -= 1;

        if (match_index >= lowest_match_index) {
            const match_ptr: [*]const u8 = if (match_index >= dict_idx)
                prefix_ptr + (match_index - prefix_idx)
            else
                dict_start + (match_index - ctx.lowLimit);

            if (readU32(match_ptr) == pattern) {
                const mlt: i32 = @intCast(MINMATCH + lz4Count(
                    ip + MINMATCH,
                    match_ptr + MINMATCH,
                    i_high_limit,
                ));

                var back: i32 = 0;
                if (@intFromPtr(ip) > @intFromPtr(i_low_limit)) {
                    const m_min = if (match_index >= dict_idx) prefix_ptr else dict_start;
                    back = countBack(ip, match_ptr, i_low_limit, m_min);
                }

                const total_length = mlt - back;
                if (total_length > result.len) {
                    result = .{
                        .len = total_length,
                        .off = @intCast(ip_index - match_index),
                        .back = back,
                    };
                    // Early exit on a very good match.
                    if (total_length > max_nb_attempts) break;
                }
            }
        }

        const delta = ctx.chainTable[match_index & LZ4HC_MAXD_MASK];
        if (delta == 0 or delta > match_index) break;
        match_index -= delta;
    }

    // Pattern analysis, only for levels with large search depth. A chain
    // delta of 1 indicates a possibly repeated pattern.
    if (pattern_analysis and result.len > 0) {
        const delta = ctx.chainTable[match_index & LZ4HC_MAXD_MASK];
        if (delta == 1 and isRepetitivePattern(pattern)) {
            const src_pattern_length = countPattern(ip + 4, i_high_limit, pattern) + 4;

            const match_candidate_idx = match_index - 1;
            if (match_candidate_idx >= lowest_match_index and match_candidate_idx >= dict_idx) {
                const match_ptr = prefix_ptr + (match_candidate_idx - prefix_idx);

                if (readU32(match_ptr) == pattern) {
                    const forward_pattern_length = countPattern(match_ptr + 4, i_high_limit, pattern) + 4;
                    const back_length = reverseCountPattern(match_ptr, prefix_ptr, pattern);

                    // Do not extend backward beyond lowest_match_index.
                    const limited_back_length = match_candidate_idx -
                        @max(match_candidate_idx - @as(u32, @intCast(back_length)), lowest_match_index);
                    const current_segment_length = limited_back_length + forward_pattern_length;

                    const max_ml: i32 = @intCast(@min(current_segment_length, src_pattern_length));

                    const new_match_index = if (current_segment_length >= src_pattern_length and
                        forward_pattern_length <= src_pattern_length)
                        // Position at the end of the pattern.
                        match_candidate_idx + @as(u32, @intCast(forward_pattern_length)) - @as(u32, @intCast(src_pattern_length))
                    else
                        // Position at the beginning of the pattern.
                        match_candidate_idx - @as(u32, @intCast(limited_back_length));

                    if (max_ml > result.len and (ip_index - new_match_index) <= LZ4_DISTANCE_MAX) {
                        result = .{
                            .len = max_ml,
                            .off = @intCast(ip_index - new_match_index),
                            .back = 0,
                        };
                    }
                }
            }
        }
    }

    return result;
}

/// Record the positions around the start of a match in the MID hash tables.
fn midFillTablesBeforeMatch(
    hash4_table: []u32,
    hash8_table: []u32,
    ip: [*]const u8,
    ilimit: [*]const u8,
    ip_index: u32,
) void {
    if (@intFromPtr(ip) + 1 <= @intFromPtr(ilimit)) {
        hash8_table[hashMid8Ptr(ip + 1)] = ip_index + 1;
        hash4_table[hashMid4Ptr(ip + 1)] = ip_index + 1;
    }
    if (@intFromPtr(ip) + 2 <= @intFromPtr(ilimit)) {
        hash8_table[hashMid8Ptr(ip + 2)] = ip_index + 2;
    }
}

/// Record the positions around the end of a match in the MID hash tables.
/// `ip` points just past the match.
fn midFillTablesAfterMatch(
    hash4_table: []u32,
    hash8_table: []u32,
    ip: [*]const u8,
    ilimit: [*]const u8,
    prefix_ptr: [*]const u8,
    end_match_idx: u32,
    ilimit_idx: u32,
) void {
    if (end_match_idx - 2 >= ilimit_idx) return;

    if (@intFromPtr(ip) - @intFromPtr(prefix_ptr) > 5) {
        const ip5 = ip - 5;
        if (@intFromPtr(ip5) <= @intFromPtr(ilimit)) {
            hash8_table[hashMid8Ptr(ip5)] = end_match_idx - 5;
        }
    }
    if (@intFromPtr(ip) >= 3) {
        const ip3 = ip - 3;
        if (@intFromPtr(ip3) <= @intFromPtr(ilimit)) {
            hash8_table[hashMid8Ptr(ip3)] = end_match_idx - 3;
        }
    }
    if (@intFromPtr(ip) >= 2) {
        const ip2 = ip - 2;
        if (@intFromPtr(ip2) <= @intFromPtr(ilimit)) {
            hash8_table[hashMid8Ptr(ip2)] = end_match_idx - 2;
            hash4_table[hashMid4Ptr(ip2)] = end_match_idx - 2;
        }
    }
    if (@intFromPtr(ip) >= 1) {
        const ip1 = ip - 1;
        if (@intFromPtr(ip1) <= @intFromPtr(ilimit)) {
            hash4_table[hashMid4Ptr(ip1)] = end_match_idx - 1;
        }
    }
}

/// LZ4MID (level 2): dual 4-byte/8-byte hash tables, two lookups per position.
fn compressMID(ctx: *Context, src: []const u8, dst: []u8) Error!usize {
    if (src.len < MFLIMIT + 1) return emitFinalLiterals(src, dst);

    var ip: [*]const u8 = src.ptr;
    var anchor: [*]const u8 = ip;
    const iend: [*]const u8 = ip + src.len;
    const mflimit: [*]const u8 = iend - MFLIMIT;
    const matchlimit: [*]const u8 = iend - LASTLITERALS;
    const ilimit: [*]const u8 = iend - LZ4MID_HASHSIZE;

    var op: [*]u8 = dst.ptr;
    const oend: [*]u8 = op + dst.len;

    ctx.nextToUpdate = 0;
    ctx.prefixStart = src.ptr;
    ctx.end = src.ptr + src.len;
    ctx.dictStart = src.ptr;
    ctx.dictLimit = 0;
    ctx.lowLimit = 0;

    // The context's hash table is split into a 4-byte and an 8-byte half.
    const hash4_table = ctx.hashTable[0..LZ4MID_HASHTABLESIZE];
    const hash8_table = ctx.hashTable[LZ4MID_HASHTABLESIZE .. 2 * LZ4MID_HASHTABLESIZE];
    @memset(hash4_table, 0);
    @memset(hash8_table, 0);

    const prefix_ptr = ctx.prefixStart;
    const prefix_idx = ctx.dictLimit;
    const ilimit_idx: u32 = @intCast((@intFromPtr(ilimit) - @intFromPtr(prefix_ptr)) + prefix_idx);

    while (@intFromPtr(ip) <= @intFromPtr(mflimit)) {
        const ip_index: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);

        // Long match first (8-byte hash).
        {
            const h8 = hashMid8Ptr(ip);
            const pos8 = hash8_table[h8];
            hash8_table[h8] = ip_index;

            if (pos8 > 0 and ip_index - pos8 <= LZ4_DISTANCE_MAX and pos8 >= prefix_idx) {
                const match_ptr = prefix_ptr + (pos8 - prefix_idx);
                if (@intFromPtr(match_ptr) < @intFromPtr(ip)) {
                    const mlt = lz4Count(ip, match_ptr, matchlimit);
                    if (mlt >= MINMATCH) {
                        const match_distance = ip_index - pos8;
                        midFillTablesBeforeMatch(hash4_table, hash8_table, ip, ilimit, ip_index);
                        try encodeSequence(&ip, &op, &anchor, @intCast(mlt), @intCast(match_distance), oend);
                        const end_match_idx: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);
                        midFillTablesAfterMatch(hash4_table, hash8_table, ip, ilimit, prefix_ptr, end_match_idx, ilimit_idx);
                        continue;
                    }
                }
            }
        }

        // Short match (4-byte hash).
        {
            const h4 = hashMid4Ptr(ip);
            const pos4 = hash4_table[h4];
            hash4_table[h4] = ip_index;

            if (pos4 > 0 and ip_index - pos4 <= LZ4_DISTANCE_MAX and pos4 >= prefix_idx) {
                const match_ptr = prefix_ptr + (pos4 - prefix_idx);
                if (@intFromPtr(match_ptr) < @intFromPtr(ip)) {
                    var match_len: u32 = @intCast(lz4Count(ip, match_ptr, matchlimit));
                    if (match_len >= MINMATCH) {
                        var match_distance = ip_index - pos4;

                        // Check ip+1 against the 8-byte table for a longer match.
                        if (@intFromPtr(ip) < @intFromPtr(mflimit)) {
                            const h8_next = hashMid8Ptr(ip + 1);
                            const pos8_next = hash8_table[h8_next];
                            const m2_distance = ip_index + 1 - pos8_next;

                            if (m2_distance <= LZ4_DISTANCE_MAX and pos8_next >= prefix_idx and pos8_next > 0) {
                                const m2_ptr = prefix_ptr + (pos8_next - prefix_idx);
                                if (@intFromPtr(m2_ptr) < @intFromPtr(ip) + 1) {
                                    const ml2 = lz4Count(ip + 1, m2_ptr, matchlimit);
                                    if (ml2 > match_len) {
                                        hash8_table[h8_next] = ip_index + 1;
                                        ip += 1;
                                        match_len = @intCast(ml2);
                                        match_distance = m2_distance;
                                    }
                                }
                            }
                        }

                        const final_ip_index: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);
                        midFillTablesBeforeMatch(hash4_table, hash8_table, ip, ilimit, final_ip_index);
                        try encodeSequence(&ip, &op, &anchor, @intCast(match_len), @intCast(match_distance), oend);
                        const end_match_idx: u32 = @intCast((@intFromPtr(ip) - @intFromPtr(prefix_ptr)) + prefix_idx);
                        midFillTablesAfterMatch(hash4_table, hash8_table, ip, ilimit, prefix_ptr, end_match_idx, ilimit_idx);
                        continue;
                    }
                }
            }
        }

        // No match found; accelerate over incompressible data.
        ip += 1 + ((@intFromPtr(ip) - @intFromPtr(anchor)) >> 9);
    }

    op = try emitFinalLiteralsAt(anchor, iend, op, oend);
    return @intFromPtr(op) - @intFromPtr(dst.ptr);
}

/// LZ4HC (levels 3-9): hash chain based compression.
fn compressHashChain(
    ctx: *Context,
    src: []const u8,
    dst: []u8,
    max_nb_attempts: i32,
) Error!usize {
    if (src.len < MFLIMIT + 1) return emitFinalLiterals(src, dst);

    const pattern_analysis = max_nb_attempts > 128; // levels 9+

    var ip: [*]const u8 = src.ptr;
    var anchor: [*]const u8 = ip;
    const iend: [*]const u8 = ip + src.len;
    const mflimit: [*]const u8 = iend - MFLIMIT;
    const matchlimit: [*]const u8 = iend - LASTLITERALS;

    var op: [*]u8 = dst.ptr;
    const oend: [*]u8 = op + dst.len;

    ctx.nextToUpdate = 0;
    ctx.prefixStart = src.ptr;
    ctx.end = src.ptr + src.len;
    ctx.dictStart = src.ptr;
    ctx.dictLimit = 0;
    ctx.lowLimit = 0;

    while (@intFromPtr(ip) <= @intFromPtr(mflimit)) {
        const match = insertAndFindBestMatch(ctx, ip, matchlimit, max_nb_attempts, pattern_analysis);

        if (match.len < MINMATCH or match.off == 0) {
            ip += 1;
            continue;
        }

        try encodeSequence(&ip, &op, &anchor, match.len, match.off, oend);
    }

    op = try emitFinalLiteralsAt(anchor, iend, op, oend);
    return @intFromPtr(op) - @intFromPtr(dst.ptr);
}

/// LZ4OPT (levels 10-12): optimal parser using dynamic programming.
fn compressOptimal(
    ctx: *Context,
    src: []const u8,
    dst: []u8,
    nb_searches: i32,
    sufficient_length: usize,
) Error!usize {
    const trailing_literals = 3;

    if (src.len < MFLIMIT + 1) return emitFinalLiterals(src, dst);

    // About 64 KiB of stack.
    var opt: [LZ4_OPT_NUM + trailing_literals]OptimalMatch = undefined;

    var ip: [*]const u8 = src.ptr;
    var anchor: [*]const u8 = ip;
    const iend: [*]const u8 = ip + src.len;
    const mflimit: [*]const u8 = iend - MFLIMIT;
    const matchlimit: [*]const u8 = iend - LASTLITERALS;

    var op: [*]u8 = dst.ptr;
    const oend: [*]u8 = op + dst.len;

    ctx.nextToUpdate = 0;
    ctx.prefixStart = src.ptr;
    ctx.end = src.ptr + src.len;
    ctx.dictStart = src.ptr;
    ctx.dictLimit = 0;
    ctx.lowLimit = 0;

    const sufficient_len = @min(sufficient_length, LZ4_OPT_NUM - 1);

    outer: while (@intFromPtr(ip) <= @intFromPtr(mflimit)) {
        const llen: i32 = @intCast(@intFromPtr(ip) - @intFromPtr(anchor));

        insertHC(ctx, ip);
        const first_match = insertAndGetWiderMatch(
            ctx,
            ip,
            ip,
            matchlimit,
            MINMATCH - 1,
            nb_searches,
            true,
        );

        if (first_match.len == 0) {
            ip += 1;
            continue;
        }

        if (@as(usize, @intCast(first_match.len)) > sufficient_len) {
            try encodeSequence(&ip, &op, &anchor, first_match.len, first_match.off, oend);
            continue;
        }

        // Prices for the first positions (literals).
        var r_pos: usize = 0;
        while (r_pos < MINMATCH) : (r_pos += 1) {
            opt[r_pos] = .{
                .mlen = 1,
                .off = 0,
                .litlen = llen + @as(i32, @intCast(r_pos)),
                .price = literalsPrice(llen + @as(i32, @intCast(r_pos))),
            };
        }

        // Prices using the initial match.
        const match_ml: usize = @intCast(first_match.len);
        var mlen: usize = MINMATCH;
        while (mlen <= match_ml) : (mlen += 1) {
            opt[mlen] = .{
                .mlen = @intCast(mlen),
                .off = first_match.off,
                .litlen = llen,
                .price = sequencePrice(llen, @intCast(mlen)),
            };
        }

        var last_match_pos: usize = match_ml;

        var add_lit: usize = 1;
        while (add_lit <= trailing_literals) : (add_lit += 1) {
            opt[last_match_pos + add_lit] = .{
                .mlen = 1,
                .off = 0,
                .litlen = @intCast(add_lit),
                .price = opt[last_match_pos].price + literalsPrice(@intCast(add_lit)),
            };
        }

        var cur: usize = 1;
        while (cur < last_match_pos) : (cur += 1) {
            const cur_ptr: [*]const u8 = ip + cur;

            if (@intFromPtr(cur_ptr) > @intFromPtr(mflimit)) break;

            // Skip if the next position has the same or a lower cost.
            if (opt[cur + 1].price <= opt[cur].price) continue;

            insertHC(ctx, cur_ptr);
            const new_match = insertAndGetWiderMatch(
                ctx,
                cur_ptr,
                cur_ptr,
                matchlimit,
                MINMATCH - 1,
                nb_searches,
                true,
            );

            if (new_match.len == 0) continue;

            if (@as(usize, @intCast(new_match.len)) > sufficient_len or
                new_match.len + @as(i32, @intCast(cur)) >= @as(i32, LZ4_OPT_NUM))
            {
                // Good enough: encode the recorded path up to cur, then this match.
                var rp: usize = 0;
                while (rp < cur) {
                    if (opt[rp].mlen == 1) {
                        ip += 1;
                        rp += 1;
                        continue;
                    }
                    const ml = opt[rp].mlen;
                    const off = opt[rp].off;
                    rp += @intCast(ml);
                    try encodeSequence(&ip, &op, &anchor, ml, off, oend);
                }

                try encodeSequence(&ip, &op, &anchor, new_match.len, new_match.off, oend);
                continue :outer;
            }

            // Prices with literals before the match.
            const base_litlen = opt[cur].litlen;
            var litlen: usize = 1;
            while (litlen < MINMATCH) : (litlen += 1) {
                const price = opt[cur].price - literalsPrice(base_litlen) +
                    literalsPrice(base_litlen + @as(i32, @intCast(litlen)));
                const pos = cur + litlen;
                if (price < opt[pos].price) {
                    opt[pos] = .{
                        .mlen = 1,
                        .off = 0,
                        .litlen = base_litlen + @as(i32, @intCast(litlen)),
                        .price = price,
                    };
                }
            }

            // Prices using the match found at cur.
            const new_match_ml: usize = @intCast(new_match.len);
            var ml: usize = MINMATCH;
            while (ml <= new_match_ml) : (ml += 1) {
                const pos = cur + ml;
                var price: i32 = undefined;
                var ll: i32 = undefined;

                if (opt[cur].mlen == 1) {
                    ll = opt[cur].litlen;
                    price = if (cur > @as(usize, @intCast(ll)))
                        opt[cur - @as(usize, @intCast(ll))].price
                    else
                        0;
                    price += sequencePrice(ll, @intCast(ml));
                } else {
                    ll = 0;
                    price = opt[cur].price + sequencePrice(0, @intCast(ml));
                }

                if (pos > last_match_pos + trailing_literals or price <= opt[pos].price) {
                    if (ml == new_match_ml and last_match_pos < pos) {
                        last_match_pos = pos;
                    }
                    opt[pos] = .{
                        .mlen = @intCast(ml),
                        .off = new_match.off,
                        .litlen = ll,
                        .price = price,
                    };
                }
            }

            add_lit = 1;
            while (add_lit <= trailing_literals) : (add_lit += 1) {
                opt[last_match_pos + add_lit] = .{
                    .mlen = 1,
                    .off = 0,
                    .litlen = @intCast(add_lit),
                    .price = opt[last_match_pos].price + literalsPrice(@intCast(add_lit)),
                };
            }
        }

        // Backtrack from the last match position to reconstruct the best path.
        const best_mlen = opt[last_match_pos].mlen;
        const best_off = opt[last_match_pos].off;
        cur = last_match_pos - @as(usize, @intCast(best_mlen));

        var candidate_pos = cur;
        var selected_match_length = best_mlen;
        var selected_offset = best_off;
        while (true) {
            const next_match_length = opt[candidate_pos].mlen;
            const next_offset = opt[candidate_pos].off;
            opt[candidate_pos].mlen = selected_match_length;
            opt[candidate_pos].off = selected_offset;
            selected_match_length = next_match_length;
            selected_offset = next_offset;
            if (next_match_length > @as(i32, @intCast(candidate_pos))) break;
            candidate_pos -= @as(usize, @intCast(next_match_length));
        }

        // Encode all recorded sequences in order.
        r_pos = 0;
        while (r_pos < last_match_pos) {
            if (opt[r_pos].mlen == 1) {
                ip += 1;
                r_pos += 1;
                continue;
            }
            const ml = opt[r_pos].mlen;
            const off = opt[r_pos].off;
            r_pos += @intCast(ml);
            try encodeSequence(&ip, &op, &anchor, ml, off, oend);
        }
    }

    op = try emitFinalLiteralsAt(anchor, iend, op, oend);
    return @intFromPtr(op) - @intFromPtr(dst.ptr);
}

/// Encode `anchor[0 .. iend - anchor]` as a final literal-only sequence.
/// Returns the advanced output pointer.
fn emitFinalLiteralsAt(anchor: [*]const u8, iend: [*]const u8, op: [*]u8, oend: [*]u8) Error![*]u8 {
    const lit_len: usize = @intFromPtr(iend) - @intFromPtr(anchor);
    if (lit_len == 0) return op;

    const ext_bytes = if (lit_len >= RUN_MASK) (lit_len - RUN_MASK) / 255 + 1 else 0;
    if (@intFromPtr(op) + 1 + ext_bytes + lit_len > @intFromPtr(oend)) {
        return error.OutputTooSmall;
    }

    var out = op;
    if (lit_len >= RUN_MASK) {
        var len = lit_len - RUN_MASK;
        out[0] = RUN_MASK << ML_BITS;
        out += 1;
        while (len >= 255) {
            out[0] = 255;
            out += 1;
            len -= 255;
        }
        out[0] = @truncate(len);
        out += 1;
    } else {
        out[0] = @as(u8, @truncate(lit_len)) << ML_BITS;
        out += 1;
    }

    @memcpy(out[0..lit_len], anchor[0..lit_len]);
    return out + lit_len;
}

/// Encode the whole input as literals (used for tiny inputs).
fn emitFinalLiterals(src: []const u8, dst: []u8) Error!usize {
    const out = try emitFinalLiteralsAt(src.ptr, src.ptr + src.len, dst.ptr, dst.ptr + dst.len);
    return @intFromPtr(out) - @intFromPtr(dst.ptr);
}

/// Maximum compressed size in the worst case.
pub fn compressBound(input_size: usize) usize {
    return lz4.compressBound(input_size);
}

/// Compress with HC mode at the given compression level (2-12); higher levels
/// compress better but are slower. Level 2 uses LZ4MID, levels 3-9 LZ4HC with
/// progressive search depth, and levels 10-12 the optimal parser.
/// Returns the number of bytes written to `dst`.
pub fn compressHC(src: []const u8, dst: []u8, compression_level: i32) Error!usize {
    if (src.len > LZ4_MAX_INPUT_SIZE) return Error.InputTooLarge;
    if (src.len == 0) return 0;

    const level = if (compression_level < LZ4HC_CLEVEL_MIN)
        LZ4HC_CLEVEL_DEFAULT
    else
        @min(compression_level, LZ4HC_CLEVEL_MAX);

    // The context is about 256 KiB; consider compressHCExtState with
    // heap-allocated state when stack space is tight.
    var ctx = Context.init();
    return compressHCExtState(&ctx, src, dst, level);
}

/// Compress with HC mode using caller-managed state.
pub fn compressHCExtState(ctx: *Context, src: []const u8, dst: []u8, compression_level: i32) Error!usize {
    if (src.len > LZ4_MAX_INPUT_SIZE) return Error.InputTooLarge;
    if (src.len == 0) return 0;
    if (dst.len == 0) return Error.OutputTooSmall;

    const level = if (compression_level < 1)
        LZ4HC_CLEVEL_DEFAULT
    else
        @min(compression_level, LZ4HC_CLEVEL_MAX);

    const params = getCLevelParams(level);
    ctx.compressionLevel = @intCast(level);

    return switch (params.strat) {
        .lz4hc => compressHashChain(ctx, src, dst, params.nbSearches),
        .lz4mid => compressMID(ctx, src, dst),
        .lz4opt => compressOptimal(ctx, src, dst, params.nbSearches, params.targetLength),
    };
}

/// Size of the HC compression context.
pub fn sizeofStateHC() usize {
    return @sizeOf(Context);
}

/// Insert every position in `[base, end)` into the hash chain.
fn insertRange(ctx: *Context, base: [*]const u8, end: [*]const u8) void {
    var ip = base;
    while (@intFromPtr(ip) < @intFromPtr(end)) : (ip += 1) {
        insertHC(ctx, ip);
    }
}

/// Promote the current prefix to an external dictionary before switching to a
/// new, non-contiguous block.
fn setExternalDict(ctx: *Context, new_block: [*]const u8) void {
    if (@intFromPtr(ctx.end) >= @intFromPtr(ctx.prefixStart) + 4) {
        const params = getCLevelParams(@intCast(ctx.compressionLevel));
        if (params.strat != .lz4mid) {
            // Insert the last 3 positions to keep the chain continuous.
            insertRange(ctx, ctx.end - 3, ctx.end);
        }
    }

    // There is only one extDict segment; any previous one is lost.
    ctx.lowLimit = ctx.dictLimit;
    ctx.dictStart = ctx.prefixStart;
    ctx.dictLimit += @as(u32, @intCast(@intFromPtr(ctx.end) - @intFromPtr(ctx.prefixStart)));
    ctx.prefixStart = new_block;
    ctx.end = new_block;
    ctx.nextToUpdate = ctx.dictLimit;

    // An extDict and a dictCtx cannot be referenced at the same time.
    ctx.dictCtx = null;
}

fn initInternal(ctx: *Context, start: [*]const u8) void {
    const buffer_size: usize = @intFromPtr(ctx.end) -% @intFromPtr(ctx.prefixStart);
    var new_starting_offset: usize = buffer_size +% ctx.dictLimit;

    if (new_starting_offset > 1024 * 1024 * 1024) {
        ctx.clearTables();
        new_starting_offset = 0;
    }

    new_starting_offset += 64 * 1024;

    ctx.nextToUpdate = @intCast(new_starting_offset);
    ctx.prefixStart = start;
    ctx.end = start;
    ctx.dictStart = start;
    ctx.dictLimit = @intCast(new_starting_offset);
    ctx.lowLimit = @intCast(new_starting_offset);
}

/// Streaming HC compression context. Compresses data in multiple chunks while
/// maintaining compression history across blocks.
pub const StreamHC = struct {
    ctx: Context,
    allocator: Allocator,
    initialized: bool,

    pub fn create(allocator: Allocator) Allocator.Error!*StreamHC {
        const stream = try allocator.create(StreamHC);
        stream.* = .{
            .ctx = Context.init(),
            .allocator = allocator,
            .initialized = false,
        };
        stream.ctx.compressionLevel = LZ4HC_CLEVEL_DEFAULT;
        return stream;
    }

    pub fn destroy(self: *StreamHC) void {
        self.allocator.destroy(self);
    }

    /// Reset the stream state to start a fresh compression.
    pub fn reset(self: *StreamHC, compression_level: i32) void {
        self.ctx.clearTables();

        const level = if (compression_level < 1)
            LZ4HC_CLEVEL_DEFAULT
        else
            @min(compression_level, LZ4HC_CLEVEL_MAX);
        self.ctx.compressionLevel = @intCast(level);

        self.ctx.dictLimit = 0;
        self.ctx.lowLimit = 0;
        self.ctx.nextToUpdate = 0;
        self.ctx.dictCtx = null;
        self.initialized = false;
    }

    /// Compress the next block, sharing compression history with prior blocks.
    pub fn compressContinue(self: *StreamHC, src: []const u8, dst: []u8) Error!usize {
        if (src.len > LZ4_MAX_INPUT_SIZE) return Error.InputTooLarge;
        if (src.len == 0) return 0;
        if (dst.len == 0) return Error.OutputTooSmall;

        if (!self.initialized) {
            initInternal(&self.ctx, src.ptr);
            self.initialized = true;
        }

        // After 2 GiB of history, save the last 64 KiB as dictionary and reset.
        const current_offset = (@intFromPtr(self.ctx.end) -% @intFromPtr(self.ctx.prefixStart)) +% self.ctx.dictLimit;
        if (current_offset > 2 * 1024 * 1024 * 1024) {
            const dict_size = @min(@intFromPtr(self.ctx.end) - @intFromPtr(self.ctx.prefixStart), 64 * 1024);
            if (dict_size > 0) {
                const dict_start = self.ctx.end - dict_size;
                _ = try self.loadDict(dict_start[0..dict_size]);
            }
        }

        // A non-contiguous block turns the previous one into an external dictionary.
        if (src.ptr != self.ctx.end) {
            setExternalDict(&self.ctx, src.ptr);
        }

        // Shrink the dictionary if the input overlaps it.
        const source_end = src.ptr + src.len;
        const dict_begin = self.ctx.dictStart;
        const dict_end = self.ctx.dictStart + (self.ctx.dictLimit - self.ctx.lowLimit);

        if (@intFromPtr(source_end) > @intFromPtr(dict_begin) and @intFromPtr(src.ptr) < @intFromPtr(dict_end)) {
            const adjusted_source_end = if (@intFromPtr(source_end) > @intFromPtr(dict_end))
                dict_end
            else
                source_end;

            const overlap: u32 = @intCast(@intFromPtr(adjusted_source_end) - @intFromPtr(self.ctx.dictStart));
            self.ctx.lowLimit += overlap;
            self.ctx.dictStart += overlap;

            // Drop the dictionary entirely once it is too small to be useful.
            const min_dict_size = 4;
            if (self.ctx.dictLimit - self.ctx.lowLimit < min_dict_size) {
                self.ctx.lowLimit = self.ctx.dictLimit;
                self.ctx.dictStart = self.ctx.prefixStart;
            }
        }

        return compressHCExtState(&self.ctx, src, dst, self.ctx.compressionLevel);
    }

    /// Load an external dictionary (at most the last 64 KiB is used) to
    /// improve compression of subsequent blocks.
    /// Returns the number of dictionary bytes retained.
    pub fn loadDict(self: *StreamHC, dict: []const u8) Error!usize {
        var dictionary = dict;
        if (dictionary.len > 64 * 1024) {
            dictionary = dictionary[dictionary.len - 64 * 1024 ..];
        }
        if (dictionary.len == 0) return 0;

        const level = self.ctx.compressionLevel;
        const params = getCLevelParams(level);

        self.reset(level);
        initInternal(&self.ctx, dictionary.ptr);
        self.ctx.end = dictionary.ptr + dictionary.len;

        // LZ4MID fills its dual hash tables lazily during compression; hash
        // chain strategies need the last positions inserted now.
        if (params.strat != .lz4mid and dictionary.len >= 4) {
            insertRange(&self.ctx, self.ctx.end - 3, self.ctx.end);
        }

        return dictionary.len;
    }

    /// Copy the last bytes of compression history into `safe_buffer` so
    /// compression can continue after the input buffers are reused.
    /// Returns the number of bytes saved.
    pub fn saveDict(self: *StreamHC, safe_buffer: []u8) usize {
        const prefix_size: usize = @intFromPtr(self.ctx.end) - @intFromPtr(self.ctx.prefixStart);

        var dict_size: usize = @min(safe_buffer.len, 64 * 1024);
        if (dict_size < 4) dict_size = 0;
        if (dict_size > prefix_size) dict_size = prefix_size;

        if (dict_size > 0) {
            const src_start = self.ctx.end - dict_size;
            @memcpy(safe_buffer[0..dict_size], src_start[0..dict_size]);
        }

        const end_index: u32 = @intCast((@intFromPtr(self.ctx.end) - @intFromPtr(self.ctx.prefixStart)) + self.ctx.dictLimit);

        if (dict_size > 0) {
            self.ctx.end = safe_buffer.ptr + dict_size;
            self.ctx.prefixStart = safe_buffer.ptr;
        } else {
            self.ctx.end = undefined;
            self.ctx.prefixStart = undefined;
        }

        self.ctx.dictLimit = end_index - @as(u32, @intCast(dict_size));
        self.ctx.lowLimit = end_index - @as(u32, @intCast(dict_size));
        self.ctx.dictStart = self.ctx.prefixStart;

        if (self.ctx.nextToUpdate < self.ctx.dictLimit) {
            self.ctx.nextToUpdate = self.ctx.dictLimit;
        }

        return dict_size;
    }
};

//! LZ4 block format: fast LZ compression algorithm.
//! Based on the C reference implementation by Yann Collet.

const std = @import("std");
const mem = std.mem;
const Allocator = std.mem.Allocator;

pub const VERSION_MAJOR = 1;
pub const VERSION_MINOR = 10;
pub const VERSION_RELEASE = 0;
pub const VERSION_NUMBER = VERSION_MAJOR * 100 * 100 + VERSION_MINOR * 100 + VERSION_RELEASE;
pub const VERSION_STRING = std.fmt.comptimePrint("{d}.{d}.{d}", .{
    VERSION_MAJOR, VERSION_MINOR, VERSION_RELEASE,
});

pub fn versionNumber() u32 {
    return VERSION_NUMBER;
}

pub fn versionString() []const u8 {
    return VERSION_STRING;
}

pub const MINMATCH = 4;
pub const WILDCOPYLENGTH = 8;
pub const LASTLITERALS = 5;
pub const MFLIMIT = 12;
pub const MATCH_SAFEGUARD_DISTANCE = (2 * WILDCOPYLENGTH) - MINMATCH;

pub const ML_BITS = 4;
pub const ML_MASK = (1 << ML_BITS) - 1;
pub const RUN_BITS = 8 - ML_BITS;
pub const RUN_MASK = (1 << RUN_BITS) - 1;

pub const LZ4_MAX_INPUT_SIZE = 0x7E000000;
pub const LZ4_DISTANCE_ABSOLUTE_MAX = 65535;
pub const LZ4_DISTANCE_MAX = 65535;

pub const LZ4_MEMORY_USAGE_MIN = 10;
pub const LZ4_MEMORY_USAGE_DEFAULT = 14;
pub const LZ4_MEMORY_USAGE_MAX = 20;
pub const LZ4_MEMORY_USAGE = 14;
pub const LZ4_HASHLOG = LZ4_MEMORY_USAGE - 2;
pub const LZ4_HASHTABLESIZE = 1 << LZ4_MEMORY_USAGE;
pub const LZ4_HASH_SIZE_U32 = 1 << (LZ4_MEMORY_USAGE - 2);

pub const ACCELERATION_DEFAULT = 1;
pub const ACCELERATION_MAX = 65537;

pub const HASH_UNIT = 4;

pub const LZ4_STREAM_MINSIZE = (1 << LZ4_MEMORY_USAGE) + 32;
pub const LZ4_STREAMDECODE_MINSIZE = 32;

/// Golden ratio constant used by the match-finder hash.
const hash_multiplier: u32 = 2654435761;

pub const Error = error{
    OutputTooSmall,
    InputTooLarge,
    CorruptedData,
    DecompressionFailed,
    InvalidState,
    AllocationFailed,
};

inline fn hash4(sequence: u32) u32 {
    return (sequence *% hash_multiplier) >> ((MINMATCH * 8) - LZ4_HASHLOG);
}

inline fn readU32(bytes: []const u8) u32 {
    return mem.readInt(u32, bytes[0..4], .little);
}

/// Maximum compressed size in the worst case, or 0 if the input is too large.
pub fn compressBound(input_size: usize) usize {
    if (input_size > LZ4_MAX_INPUT_SIZE) return 0;
    return input_size + (input_size / 255) + 16;
}

/// Core decompression engine used by all public decompression functions.
/// `target_output_size` enables partial decompression; pass `dst.len` for full.
/// When `partial` is set, decoding stops once `target_output_size` bytes have
/// been produced, even in the middle of a sequence.
/// `low_prefix_ptr` points to the prefix start when streaming; null otherwise.
/// `dict` is an optional external dictionary.
fn decompressGeneric(
    src: []const u8,
    dst: []u8,
    target_output_size: usize,
    partial: bool,
    low_prefix_ptr: ?[*]const u8,
    dict: ?[]const u8,
) Error!usize {
    if (src.len == 0) return 0;
    if (dst.len == 0) return 0;
    if (target_output_size > dst.len) return error.OutputTooSmall;

    const low_prefix = low_prefix_ptr orelse dst.ptr;
    const dict_end: ?[*]const u8 = if (dict) |d| d.ptr + d.len else null;
    const dict_size: usize = if (dict) |d| d.len else 0;

    var ip: usize = 0;
    var op: usize = 0;
    const iend = src.len;
    const oend = target_output_size;

    while (ip < iend) {
        const token = src[ip];
        ip += 1;

        var literal_len: usize = token >> ML_BITS;
        if (literal_len == RUN_MASK) {
            while (true) {
                if (ip >= iend) return error.CorruptedData;
                const s = src[ip];
                ip += 1;
                literal_len += s;
                if (s != 255) break;
            }
        }

        if (literal_len > 0) {
            if (partial and op + literal_len > oend) {
                const copy_len = oend - op;
                if (ip + copy_len > iend) return error.CorruptedData;
                @memcpy(dst[op..][0..copy_len], src[ip..][0..copy_len]);
                return oend;
            }
            if (ip + literal_len > iend) return error.CorruptedData;
            if (op + literal_len > oend) return error.OutputTooSmall;
            @memcpy(dst[op..][0..literal_len], src[ip..][0..literal_len]);
            ip += literal_len;
            op += literal_len;
        }

        // The last sequence carries no match.
        if (ip >= iend) break;

        if (ip + 2 > iend) return error.CorruptedData;
        const offset = mem.readInt(u16, src[ip..][0..2], .little);
        ip += 2;
        if (offset == 0) return error.CorruptedData;

        var match_len: usize = token & ML_MASK;
        if (match_len == ML_MASK) {
            while (true) {
                if (ip >= iend) return error.CorruptedData;
                const s = src[ip];
                ip += 1;
                match_len += s;
                if (s != 255) break;
            }
        }
        match_len += MINMATCH;

        var finished = false;
        if (op + match_len > oend) {
            if (!partial) return error.OutputTooSmall;
            match_len = oend - op;
            finished = true;
        }

        const match_ptr = dst.ptr + op - offset;
        if (@intFromPtr(match_ptr) < @intFromPtr(low_prefix)) {
            // Match starts in the external dictionary.
            if (dict_end == null) return error.CorruptedData;

            const prefix_offset = @intFromPtr(dst.ptr + op) - @intFromPtr(low_prefix);
            if (offset > prefix_offset + dict_size) return error.CorruptedData;

            const low_prefix_offset = @intFromPtr(low_prefix) - @intFromPtr(match_ptr);
            const dict_match_ptr = dict_end.? - low_prefix_offset;

            if (match_len <= low_prefix_offset) {
                @memcpy(dst[op..][0..match_len], dict_match_ptr[0..match_len]);
                op += match_len;
            } else {
                // Match spans the dictionary and the current block.
                const copy_size = low_prefix_offset;
                const rest_size = match_len - copy_size;

                @memcpy(dst[op..][0..copy_size], dict_match_ptr[0..copy_size]);
                op += copy_size;

                const rest_start = @intFromPtr(low_prefix) - @intFromPtr(dst.ptr);
                if (rest_size > op - rest_start) {
                    mem.copyForwards(u8, dst[op..][0..rest_size], dst[rest_start..][0..rest_size]);
                } else {
                    @memcpy(dst[op..][0..rest_size], dst[rest_start..][0..rest_size]);
                }
                op += rest_size;
            }
        } else {
            if (offset > op) return error.CorruptedData;
            const match_pos = op - offset;
            if (offset < match_len) {
                mem.copyForwards(u8, dst[op..][0..match_len], dst[match_pos..][0..match_len]);
            } else {
                @memcpy(dst[op..][0..match_len], dst[match_pos..][0..match_len]);
            }
            op += match_len;
        }

        if (finished) return oend;
    }

    return op;
}

/// Decompress LZ4 block data into a pre-allocated buffer, with bounds checking.
/// Returns the number of bytes written to `dst`.
pub fn decompressSafe(src: []const u8, dst: []u8) Error!usize {
    return decompressGeneric(src, dst, dst.len, false, null, null);
}

pub const HashTable = struct {
    table: [LZ4_HASH_SIZE_U32]u32,

    pub fn init() HashTable {
        return .{ .table = @splat(0) };
    }

    fn get(self: *const HashTable, hash: u32) u32 {
        return self.table[hash];
    }

    fn put(self: *HashTable, hash: u32, pos: u32) void {
        self.table[hash] = pos;
    }
};

/// Compress with default settings (acceleration = 1).
/// `dst` must be at least `compressBound(src.len)` bytes for guaranteed success.
/// Returns the number of bytes written to `dst`.
pub fn compressDefault(src: []const u8, dst: []u8) Error!usize {
    return compressFast(src, dst, ACCELERATION_DEFAULT);
}

/// Compress with the given acceleration (higher = faster, less compression).
/// `dst` must be at least `compressBound(src.len)` bytes for guaranteed success.
/// Returns the number of bytes written to `dst`.
pub fn compressFast(src: []const u8, dst: []u8, acceleration: u32) Error!usize {
    if (src.len > LZ4_MAX_INPUT_SIZE) return error.InputTooLarge;
    if (src.len == 0) return 0;
    if (src.len < MFLIMIT + 1) return emitLastLiterals(src, dst, 0, 0);

    var hash_table = HashTable.init();
    return compressFastWithHashTable(src, dst, acceleration, &hash_table);
}

/// Write the extension bytes of a length >= RUN_MASK/ML_MASK (a run of 255s
/// followed by the remainder). Returns the new output position.
fn putExtendedLength(dst: []u8, pos: usize, length: usize) Error!usize {
    var op = pos;
    var len = length;
    while (len >= 255) {
        if (op >= dst.len) return error.OutputTooSmall;
        dst[op] = 255;
        op += 1;
        len -= 255;
    }
    if (op >= dst.len) return error.OutputTooSmall;
    dst[op] = @intCast(len);
    return op + 1;
}

/// Encode `src[anchor..]` as a final literal-only sequence starting at `op`.
/// Returns the total compressed size.
fn emitLastLiterals(src: []const u8, dst: []u8, anchor: usize, op: usize) Error!usize {
    const literal_len = src.len - anchor;
    var out_pos = op;

    if (literal_len == 0) return out_pos;

    if (out_pos >= dst.len) return error.OutputTooSmall;
    if (literal_len >= RUN_MASK) {
        dst[out_pos] = RUN_MASK << ML_BITS;
        out_pos = try putExtendedLength(dst, out_pos + 1, literal_len - RUN_MASK);
    } else {
        dst[out_pos] = @as(u8, @intCast(literal_len)) << ML_BITS;
        out_pos += 1;
    }

    if (out_pos + literal_len > dst.len) return error.OutputTooSmall;
    @memcpy(dst[out_pos..][0..literal_len], src[anchor..][0..literal_len]);
    return out_pos + literal_len;
}

fn compressFastWithHashTable(src: []const u8, dst: []u8, acceleration: u32, hash_table: *HashTable) Error!usize {
    var ip: usize = 0;
    var op: usize = 0;
    var anchor: usize = 0;

    const mflimit_plus_one = src.len - MFLIMIT;
    const match_limit = src.len - LASTLITERALS;
    const accel = std.math.clamp(acceleration, 1, ACCELERATION_MAX);

    ip += 1;

    while (ip < mflimit_plus_one) {
        var step: usize = accel;
        var search_match_nb: usize = accel;

        var match: usize = undefined;
        var forward_ip = ip;

        while (true) {
            ip = forward_ip;
            forward_ip += step;
            step = search_match_nb >> 6;
            search_match_nb += 1;

            if (forward_ip > mflimit_plus_one) {
                return emitLastLiterals(src, dst, anchor, op);
            }

            const h = hash4(readU32(src[ip..]));
            match = hash_table.get(h);

            // Validate before inserting the current position, to avoid
            // matching ourselves.
            const is_valid_match = match > 0 and
                match < ip and
                match + LZ4_DISTANCE_ABSOLUTE_MAX >= ip and
                readU32(src[match..]) == readU32(src[ip..]);

            hash_table.put(h, @intCast(ip));

            if (is_valid_match) break;
        }

        const literal_len = ip - anchor;
        const token_pos = op;
        op += 1;
        if (op >= dst.len) return error.OutputTooSmall;

        if (literal_len >= RUN_MASK) {
            dst[token_pos] = RUN_MASK << ML_BITS;
            op = try putExtendedLength(dst, op, literal_len - RUN_MASK);
        } else {
            dst[token_pos] = @as(u8, @intCast(literal_len)) << ML_BITS;
        }

        if (op + literal_len > dst.len) return error.OutputTooSmall;
        if (literal_len > 0) {
            @memcpy(dst[op..][0..literal_len], src[anchor..][0..literal_len]);
            op += literal_len;
        }

        const offset: u16 = @intCast(ip - match);
        if (op + 2 > dst.len) return error.OutputTooSmall;
        mem.writeInt(u16, dst[op..][0..2], offset, .little);
        op += 2;

        ip += MINMATCH;
        match += MINMATCH;
        var match_len: usize = 0;
        while (ip < match_limit and src[ip] == src[match]) {
            ip += 1;
            match += 1;
            match_len += 1;
        }

        if (match_len >= ML_MASK) {
            dst[token_pos] |= ML_MASK;
            op = try putExtendedLength(dst, op, match_len - ML_MASK);
        } else {
            dst[token_pos] |= @intCast(match_len);
        }

        anchor = ip;

        if (ip < mflimit_plus_one) {
            const h = hash4(readU32(src[ip..]));
            hash_table.put(h, @intCast(ip));
            ip += 1;
        }
    }

    return emitLastLiterals(src, dst, anchor, op);
}

/// Size needed for an external compression state buffer.
pub fn sizeofState() usize {
    return @sizeOf(HashTable);
}

/// Compress using a caller-provided state buffer of at least `sizeofState()`
/// bytes, avoiding internal allocation.
pub fn compressFastExtState(state: []align(@alignOf(HashTable)) u8, src: []const u8, dst: []u8, acceleration: u32) Error!usize {
    if (state.len < sizeofState()) return error.InvalidState;

    if (src.len > LZ4_MAX_INPUT_SIZE) return error.InputTooLarge;
    if (src.len == 0) return 0;
    if (src.len < MFLIMIT + 1) return emitLastLiterals(src, dst, 0, 0);

    const hash_table: *HashTable = @ptrCast(@alignCast(state.ptr));
    hash_table.* = HashTable.init();

    return compressFastWithHashTable(src, dst, acceleration, hash_table);
}

/// Compress as much of `src` as fits into `dst`.
/// `src_size_ptr` holds the input size on entry and the consumed bytes on return.
/// Returns the compressed size.
pub fn compressDestSize(src: []const u8, dst: []u8, src_size_ptr: *usize) Error!usize {
    const max_src_size = src_size_ptr.*;
    if (max_src_size == 0) {
        src_size_ptr.* = 0;
        return 0;
    }

    // If the destination can hold the worst case, compress everything.
    if (dst.len >= compressBound(max_src_size)) {
        const result = try compressDefault(src[0..max_src_size], dst);
        src_size_ptr.* = max_src_size;
        return result;
    }

    var low: usize = 1;
    var high: usize = max_src_size;
    var best_size: usize = 0;
    var best_compressed_size: usize = 0;

    // Quick estimate first: assume a roughly 1:1 compression ratio.
    if (dst.len <= max_src_size) {
        const estimate = @min(dst.len, max_src_size);
        if (compressDefault(src[0..estimate], dst)) |size| {
            if (size <= dst.len) {
                best_size = estimate;
                best_compressed_size = size;
                low = estimate + 1;
            } else {
                high = estimate - 1;
            }
        } else |_| {
            high = estimate - 1;
        }
    }

    // Binary search for the largest input size that fits.
    while (low <= high) {
        const mid = low + (high - low) / 2;
        if (mid == 0 or mid > max_src_size) break;

        if (compressDefault(src[0..mid], dst)) |size| {
            if (size <= dst.len) {
                best_size = mid;
                best_compressed_size = size;
                if (mid == max_src_size) break;
                low = mid + 1;
            } else {
                high = mid - 1;
            }
        } else |_| {
            high = mid - 1;
        }

        if (low > max_src_size) break;
    }

    src_size_ptr.* = best_size;
    return best_compressed_size;
}

/// Decompress only the first `target_output_size` bytes.
pub fn decompressSafePartial(src: []const u8, dst: []u8, target_output_size: usize) Error!usize {
    return decompressGeneric(src, dst, target_output_size, true, null, null);
}

const TableType = enum(u32) {
    byU32 = 1,
    byU16 = 2,
    byPtr = 3,
};

/// Streaming compression context.
pub const Stream = struct {
    hashTable: [LZ4_HASH_SIZE_U32]u32,
    dictionary: ?[]const u8,
    dictCtx: ?*const Stream,
    currentOffset: u32,
    tableType: TableType,
    dictSize: u32,
    allocator: ?Allocator,

    pub fn create(allocator: Allocator) Error!*Stream {
        const stream = allocator.create(Stream) catch return error.AllocationFailed;
        stream.* = init();
        stream.allocator = allocator;
        return stream;
    }

    pub fn destroy(self: *Stream) void {
        if (self.allocator) |allocator| {
            allocator.destroy(self);
        }
    }

    /// Initialize a stream in place (for stack-allocated streams).
    pub fn init() Stream {
        return .{
            .hashTable = @splat(0),
            .dictionary = null,
            .dictCtx = null,
            .currentOffset = 0,
            .tableType = .byU32,
            .dictSize = 0,
            .allocator = null,
        };
    }

    /// Reset the stream for a new compression.
    pub fn resetFast(self: *Stream) void {
        @memset(&self.hashTable, 0);
        self.dictionary = null;
        self.dictCtx = null;
        self.currentOffset = 0;
        self.dictSize = 0;
    }

    /// Load a dictionary; only the last 64 KiB are kept.
    /// Returns the number of dictionary bytes retained.
    pub fn loadDict(self: *Stream, dict: []const u8) usize {
        self.resetFast();

        if (dict.len == 0) return 0;

        const dict_size = @min(dict.len, 64 * 1024);
        const dict_start = dict.len - dict_size;
        self.dictionary = dict[dict_start..];
        self.dictSize = @intCast(dict_size);

        if (dict_size >= MINMATCH) {
            for (0..dict_size - MINMATCH) |i| {
                const h = hash4(readU32(dict[dict_start + i ..]));
                self.hashTable[h] = @intCast(i);
            }
        }

        return dict_size;
    }

    /// Compress the next block in the stream.
    pub fn compressFastContinue(self: *Stream, src: []const u8, dst: []u8, acceleration: u32) Error!usize {
        if (src.len > LZ4_MAX_INPUT_SIZE) return error.InputTooLarge;
        if (src.len == 0) return 0;
        if (src.len < MFLIMIT + 1) return emitLastLiterals(src, dst, 0, 0);

        var hash_table = HashTable{ .table = self.hashTable };
        const result = try compressFastWithHashTable(src, dst, acceleration, &hash_table);
        self.hashTable = hash_table.table;
        self.currentOffset +|= @intCast(src.len);

        return result;
    }

    /// Save the dictionary into a caller-owned buffer.
    /// Returns the number of bytes saved.
    pub fn saveDict(self: *Stream, safe_buffer: []u8, max_dict_size: usize) usize {
        if (max_dict_size == 0) return 0;
        const dict = self.dictionary orelse return 0;

        const dict_size = @min(@min(dict.len, max_dict_size), 64 * 1024);
        const copy_size = @min(dict_size, safe_buffer.len);
        @memcpy(safe_buffer[0..copy_size], dict[dict.len - copy_size ..]);
        return copy_size;
    }
};

/// Create a streaming compression context.
pub fn createStream(allocator: Allocator) Error!*Stream {
    return Stream.create(allocator);
}

/// Free a streaming compression context.
pub fn freeStream(stream: *Stream) void {
    stream.destroy();
}

/// Streaming decompression context.
pub const StreamDecode = struct {
    externalDict: ?[]const u8,
    prefixEnd: ?[]const u8,
    extDictSize: usize,
    prefixSize: usize,
    allocator: ?Allocator,

    pub fn create(allocator: Allocator) Error!*StreamDecode {
        const stream = allocator.create(StreamDecode) catch return error.AllocationFailed;
        stream.* = init();
        stream.allocator = allocator;
        return stream;
    }

    pub fn destroy(self: *StreamDecode) void {
        if (self.allocator) |allocator| {
            allocator.destroy(self);
        }
    }

    /// Initialize a decode stream in place.
    pub fn init() StreamDecode {
        return .{
            .externalDict = null,
            .prefixEnd = null,
            .extDictSize = 0,
            .prefixSize = 0,
            .allocator = null,
        };
    }

    /// Set the dictionary for subsequent decompression.
    pub fn setStreamDecode(self: *StreamDecode, dict: ?[]const u8) void {
        self.externalDict = dict;
        self.prefixEnd = null;
        self.extDictSize = if (dict) |d| d.len else 0;
        self.prefixSize = 0;
    }

    /// Decompress the next block in the stream.
    pub fn decompressSafeContinue(self: *StreamDecode, src: []const u8, dst: []u8) Error!usize {
        if (self.prefixSize == 0 and self.extDictSize == 0) {
            const result = try decompressSafe(src, dst);
            self.prefixEnd = dst[0..result];
            self.prefixSize = result;
            return result;
        }

        const low_prefix: [*]const u8 = if (self.prefixEnd) |prefix| prefix.ptr else dst.ptr;
        const dict: ?[]const u8 = if (self.extDictSize > 0) self.externalDict else null;

        const result = try decompressGeneric(src, dst, dst.len, false, low_prefix, dict);

        self.prefixEnd = dst[0..result];
        self.prefixSize = result;
        // The dictionary is only used once.
        self.externalDict = null;
        self.extDictSize = 0;

        return result;
    }
};

/// Create a streaming decompression context.
pub fn createStreamDecode(allocator: Allocator) Error!*StreamDecode {
    return StreamDecode.create(allocator);
}

/// Free a streaming decompression context.
pub fn freeStreamDecode(stream: *StreamDecode) void {
    stream.destroy();
}

/// Size of the ring buffer needed for streaming decompression.
pub fn decoderRingBufferSize(max_block_size: usize) usize {
    if (max_block_size == 0) return 0;
    return 65536 + 14 + max_block_size;
}

/// Decompress with an external dictionary (stateless).
pub fn decompressSafeUsingDict(src: []const u8, dst: []u8, dict: []const u8) Error!usize {
    return decompressGeneric(src, dst, dst.len, false, dst.ptr, dict);
}

/// Partially decompress with an external dictionary (stateless).
pub fn decompressSafePartialUsingDict(src: []const u8, dst: []u8, target_output_size: usize, dict: []const u8) Error!usize {
    return decompressGeneric(src, dst, target_output_size, true, dst.ptr, dict);
}

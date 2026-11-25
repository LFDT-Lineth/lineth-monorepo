//! The conflation compressor: full block RLPs truncated, serialized and compressed into the zstd
//! frame of a DA segment. The compressor decodes what it compresses; block numbers, parent hashes
//! and chain IDs are the caller's to check.

const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");

const truncation = lineth_da.encoding.truncation;
const Allocator = std.mem.Allocator;

pub const Error = error{
    OutOfMemory,
    /// The dictionary is not a well-formed zstd dictionary that declares an ID.
    InvalidDictionary,
    /// No blocks to compress.
    NoBlocks,
    /// A block cannot be truncated; the `Rejection` says which and why.
    InvalidBlockRlp,
    /// libzstd failed in a way other than an exhausted allocator.
    CompressionFailed,
};

/// A block that truncation rejected.
pub const Rejection = struct {
    block_index: usize,
    reason: error{ MalformedBlock, SenderRecoveryFailed },
};

pub const Compressor = struct {
    allocator: Allocator,
    chain_id: u64,
    dictionary: []u8,
    encoder: zstd_encoder.Encoder,

    /// Copies `dictionary`.
    pub fn init(allocator: Allocator, dictionary: []const u8, chain_id: u64) Error!Compressor {
        const copy = try allocator.dupe(u8, dictionary);
        errdefer allocator.free(copy);
        const encoder = try zstd_encoder.Encoder.init(allocator, copy);
        return .{ .allocator = allocator, .chain_id = chain_id, .dictionary = copy, .encoder = encoder };
    }

    pub fn deinit(self: Compressor) void {
        self.encoder.deinit();
        self.allocator.free(self.dictionary);
    }

    /// Compresses the conflation of `block_rlps` into one frame, allocated from the compressor's
    /// allocator, which the caller owns. A rejected block is reported in `rejection` along with
    /// `error.InvalidBlockRlp`.
    /// Any number of threads may call it at once: the dictionary is read-only, and every call has
    /// its own arenas and libzstd context. The compressor's allocator serves them all.
    pub fn compress(self: *const Compressor, block_rlps: []const []const u8, rejection: ?*Rejection) Error![]u8 {
        if (block_rlps.len == 0) return error.NoBlocks;

        var output_arena = std.heap.ArenaAllocator.init(self.allocator);
        defer output_arena.deinit();
        var scratch_arena = std.heap.ArenaAllocator.init(self.allocator);
        defer scratch_arena.deinit();
        const output = output_arena.allocator();
        const scratch = scratch_arena.allocator();

        const blocks = try output.alloc(truncation.TruncatedBlock, block_rlps.len);
        for (block_rlps, blocks, 0..) |block_rlp, *block, index| {
            block.* = truncation.truncateBlockRlp(output, scratch, block_rlp, self.chain_id) catch |err| switch (err) {
                error.OutOfMemory => return error.OutOfMemory,
                error.MalformedBlock, error.SenderRecoveryFailed => |reason| {
                    if (rejection) |out| out.* = .{ .block_index = index, .reason = reason };
                    return error.InvalidBlockRlp;
                },
            };
            _ = scratch_arena.reset(.retain_capacity);
        }
        const payload = try truncation.encodeBlocks(output, scratch, blocks);
        return self.encoder.compress(self.allocator, payload);
    }
};

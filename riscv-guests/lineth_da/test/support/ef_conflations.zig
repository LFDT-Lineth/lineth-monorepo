//! Conflations of real blocks: the valid blocks of the EF zkevm fixtures without a blob transaction,
//! in the order of the corpus, each truncated by `encoding.truncation` and the truncated blocks of a
//! conflation encoded by `encodeBlocks` into the canonical payload its zstd frame decompresses to.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;

const Allocator = std.mem.Allocator;

/// The consecutive truncated blocks of a corpus directory.
pub const Blocks = struct {
    corpus: ef_blocks.Corpus,
    case: ?ef_blocks.Case = null,
    next_block: usize = 0,

    /// `corpus_dir` is a directory of fixture files, absolute or relative to the working directory.
    pub fn open(gpa: Allocator, io: std.Io, corpus_dir: []const u8) !Blocks {
        const absolute = try std.Io.Dir.cwd().realPathFileAlloc(io, corpus_dir, gpa);
        defer gpa.free(absolute);
        return .{ .corpus = try ef_blocks.Corpus.open(gpa, io, absolute) };
    }

    pub fn deinit(self: *Blocks) void {
        self.corpus.deinit();
    }

    /// The truncation of the next valid block, allocated from `output`; `scratch` holds what the
    /// truncation needs. `error.CorpusExhausted` follows the last block of the corpus.
    pub fn next(self: *Blocks, output: Allocator, scratch: Allocator) !truncation.TruncatedBlock {
        while (true) {
            if (self.case) |case| {
                while (self.next_block < case.blocks.len) {
                    const fact = case.blocks[self.next_block] orelse {
                        self.next_block += 1;
                        continue;
                    };
                    self.next_block += 1;
                    if (fact.blob_transaction) continue;
                    return truncation.truncateBlockRlp(output, scratch, fact.rlp, case.chain_id);
                }
            }
            self.case = try self.corpus.next() orelse return error.CorpusExhausted;
            self.next_block = 0;
        }
    }

    /// The canonical payload of a conflation of the next `count` blocks; the caller owns it.
    pub fn conflationPayload(self: *Blocks, allocator: Allocator, count: usize) ![]u8 {
        var truncated_arena = std.heap.ArenaAllocator.init(allocator);
        defer truncated_arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(allocator);
        defer scratch.deinit();

        const truncated = try truncated_arena.allocator().alloc(truncation.TruncatedBlock, count);
        for (truncated) |*block| {
            block.* = try self.next(truncated_arena.allocator(), scratch.allocator());
            _ = scratch.reset(.retain_capacity);
        }
        return truncation.encodeBlocks(allocator, scratch.allocator(), truncated);
    }
};

/// The payloads of conflations of three blocks, concatenated until they hold `len` bytes, read once
/// per process and kept: the same bytes for every caller, the first `len` of them.
pub fn stream(io: std.Io, corpus_dir: []const u8, len: usize) ![]const u8 {
    if (cached.len >= len) return cached[0..len];
    const allocator = std.heap.page_allocator;
    var blocks = try Blocks.open(allocator, io, corpus_dir);
    defer blocks.deinit();

    var bytes: std.ArrayList(u8) = .empty;
    errdefer bytes.deinit(allocator);
    while (bytes.items.len < len) {
        const payload = try blocks.conflationPayload(allocator, 3);
        defer allocator.free(payload);
        try bytes.appendSlice(allocator, payload);
    }
    allocator.free(cached);
    cached = try bytes.toOwnedSlice(allocator);
    return cached[0..len];
}

var cached: []u8 = &.{};

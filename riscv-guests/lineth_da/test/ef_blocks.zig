//! The blocks of the Ethereum Foundation's zkevm fixtures: the cases of a corpus directory, and
//! for every block a case accepts the facts the fixture lists for it, hex-decoded.

const std = @import("std");

const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;
pub const Address = [20]u8;

/// A block a fixture accepts, as the fixture lists it.
pub const Fact = struct {
    /// The block's RLP.
    rlp: []const u8,
    hash: Hash,
    parent_hash: Hash,
    prev_randao: Hash,
    number: u64,
    timestamp: u64,
    /// The sender of every transaction, in order.
    senders: []const Address,
};

/// One case of a fixture file.
pub const Case = struct {
    /// The fixture file, relative to the corpus directory.
    path: []const u8,
    name: []const u8,
    chain_id: u64,
    /// One entry per block of the case: the facts of a block the fixture accepts, null for one it
    /// rejects or lists without RLP.
    blocks: []const ?Fact,
};

const Header = struct {
    hash: []const u8,
    parentHash: []const u8,
    mixHash: []const u8,
    timestamp: []const u8,
    number: []const u8,
};

const Transaction = struct {
    sender: []const u8,
};

const Block = struct {
    rlp: ?[]const u8 = null,
    blockHeader: ?Header = null,
    transactions: ?[]const Transaction = null,
    expectException: ?[]const u8 = null,
};

const RawCase = struct {
    config: struct { chainid: []const u8 } = .{ .chainid = "0x01" },
    blocks: []const Block = &.{},
};

/// The cases of every fixture file under a directory, one file at a time.
pub const Corpus = struct {
    io: std.Io,
    dir: std.Io.Dir,
    walker: std.Io.Dir.Walker,
    /// Holds the file being read, its parsed cases and the cases handed out.
    arena: std.heap.ArenaAllocator,
    path: []const u8 = "",
    cases: std.json.ArrayHashMap(RawCase) = .{},
    next_case: usize = 0,

    pub fn open(gpa: Allocator, io: std.Io, path: []const u8) !Corpus {
        var dir = try std.Io.Dir.openDirAbsolute(io, path, .{ .iterate = true });
        errdefer dir.close(io);
        return .{ .io = io, .dir = dir, .walker = try dir.walk(gpa), .arena = std.heap.ArenaAllocator.init(gpa) };
    }

    pub fn deinit(self: *Corpus) void {
        self.walker.deinit();
        self.dir.close(self.io);
        self.arena.deinit();
    }

    /// The next case, or null behind the last one. A block the fixture accepts without listing its
    /// header is `error.FixtureWithoutHeader`. The case is valid until the call that reads the next
    /// fixture file.
    pub fn next(self: *Corpus) !?Case {
        while (self.next_case == self.cases.map.count()) {
            if (!try self.readNextFile()) return null;
        }
        const arena = self.arena.allocator();
        const name = self.cases.map.keys()[self.next_case];
        const raw = self.cases.map.values()[self.next_case];
        self.next_case += 1;

        const blocks = try arena.alloc(?Fact, raw.blocks.len);
        for (raw.blocks, blocks) |block, *fact| fact.* = try factOf(arena, block);
        return .{
            .path = self.path,
            .name = name,
            .chain_id = try std.fmt.parseInt(u64, raw.config.chainid, 0),
            .blocks = blocks,
        };
    }

    /// Replaces the file being read with the next fixture file; false when there is none.
    fn readNextFile(self: *Corpus) !bool {
        while (try self.walker.next(self.io)) |entry| {
            if (entry.kind != .file or !std.mem.endsWith(u8, entry.basename, ".json")) continue;
            _ = self.arena.reset(.retain_capacity);
            const arena = self.arena.allocator();
            self.path = try arena.dupe(u8, entry.path);
            const text = try self.dir.readFileAlloc(self.io, entry.path, arena, .limited(1 << 30));
            self.cases = try std.json.parseFromSliceLeaky(std.json.ArrayHashMap(RawCase), arena, text, .{ .ignore_unknown_fields = true });
            self.next_case = 0;
            return true;
        }
        return false;
    }
};

fn factOf(arena: Allocator, block: Block) !?Fact {
    if (block.expectException != null) return null;
    const rlp_text = block.rlp orelse return null;
    const header = block.blockHeader orelse return error.FixtureWithoutHeader;
    const transactions = block.transactions orelse &.{};
    const senders = try arena.alloc(Address, transactions.len);
    for (transactions, senders) |transaction, *sender| sender.* = try decodeHex(20, transaction.sender);
    return .{
        .rlp = try decodeHexAlloc(arena, rlp_text),
        .hash = try decodeHex(32, header.hash),
        .parent_hash = try decodeHex(32, header.parentHash),
        .prev_randao = try decodeHex(32, header.mixHash),
        .number = try std.fmt.parseInt(u64, header.number, 0),
        .timestamp = try std.fmt.parseInt(u64, header.timestamp, 0),
        .senders = senders,
    };
}

fn digits(text: []const u8) []const u8 {
    return if (std.mem.startsWith(u8, text, "0x")) text[2..] else text;
}

fn decodeHex(comptime len: usize, text: []const u8) ![len]u8 {
    var bytes: [len]u8 = undefined;
    _ = try std.fmt.hexToBytes(&bytes, digits(text));
    return bytes;
}

fn decodeHexAlloc(arena: Allocator, text: []const u8) ![]u8 {
    const hex = digits(text);
    const bytes = try arena.alloc(u8, hex.len / 2);
    _ = try std.fmt.hexToBytes(bytes, hex);
    return bytes;
}

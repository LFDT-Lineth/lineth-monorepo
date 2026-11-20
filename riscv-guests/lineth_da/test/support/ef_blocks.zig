//! The blocks of the Ethereum Foundation's zkevm fixtures: the cases of a corpus directory, and
//! for every block a case accepts the facts the fixture lists for it, hex-decoded; or one block at
//! a known position.

const std = @import("std");

const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;
pub const Address = [20]u8;

/// What the fixture lists about the transactions of a block.
pub const Profile = struct {
    transactions: usize,
    /// Bit `n` is set when the block holds a transaction of type `n`.
    types: u8,
    /// The longest calldata of a transaction, in bytes.
    max_data_len: usize,
    /// Whether a transaction has an access list with an entry.
    access_list: bool,
    /// Whether a transaction has an authorization list with an entry.
    authorization_list: bool,
};

/// An access-list entry of a transaction.
pub const AccessListEntry = struct {
    address: Address,
    storage_keys: []const Hash,
};

/// An authorization of a set-code transaction.
pub const Authorization = struct {
    chain_id: u256,
    address: Address,
    nonce: u64,
    y_parity: u256,
    r: u256,
    s: u256,
};

/// The fields of a transaction as the fixture lists them.
pub const Transaction = struct {
    type: u8,
    nonce: u64,
    /// `gasPrice`, or `maxFeePerGas` of a fee-market or set-code transaction.
    gas_price: u256,
    /// `maxPriorityFeePerGas` of a fee-market or set-code transaction.
    max_priority_fee_per_gas: ?u256,
    gas_limit: u64,
    /// Null for a contract creation.
    to: ?Address,
    value: u256,
    data: []const u8,
    access_list: []const AccessListEntry,
    authorization_list: []const Authorization,
};

/// A block a fixture accepts, as the fixture lists it.
pub const Fact = struct {
    /// The block's RLP.
    rlp: []const u8,
    hash: Hash,
    parent_hash: Hash,
    prev_randao: Hash,
    number: u64,
    timestamp: u64,
    /// The transactions, in order.
    transactions: []const Transaction,
    /// The sender of every transaction, in order.
    senders: []const Address,
    /// Whether a transaction of the block is an EIP-4844 blob transaction (type 3).
    blob_transaction: bool,
    profile: Profile,
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

const RawAccessListEntry = struct {
    address: []const u8 = "0x",
    storageKeys: []const []const u8 = &.{},
};

const RawAuthorization = struct {
    chainId: []const u8 = "0x0",
    address: []const u8 = "0x",
    nonce: []const u8 = "0x0",
    yParity: []const u8 = "0x0",
    r: []const u8 = "0x0",
    s: []const u8 = "0x0",
};

const RawTransaction = struct {
    type: []const u8 = "0x00",
    sender: []const u8,
    nonce: []const u8 = "0x0",
    gasPrice: ?[]const u8 = null,
    maxFeePerGas: ?[]const u8 = null,
    maxPriorityFeePerGas: ?[]const u8 = null,
    gasLimit: []const u8 = "0x0",
    to: []const u8 = "",
    value: []const u8 = "0x0",
    data: []const u8 = "0x",
    accessList: []const RawAccessListEntry = &.{},
    authorizationList: []const RawAuthorization = &.{},
};

const Block = struct {
    rlp: ?[]const u8 = null,
    blockHeader: ?Header = null,
    transactions: ?[]const RawTransaction = null,
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
    const raw_transactions = block.transactions orelse &.{};
    const transactions = try arena.alloc(Transaction, raw_transactions.len);
    const senders = try arena.alloc(Address, raw_transactions.len);
    var blob_transaction = false;
    var profile: Profile = .{ .transactions = transactions.len, .types = 0, .max_data_len = 0, .access_list = false, .authorization_list = false };
    for (raw_transactions, transactions, senders) |raw, *transaction, *sender| {
        sender.* = try decodeHex(20, raw.sender);
        transaction.* = try transactionOf(arena, raw);
        if (transaction.type == 3) blob_transaction = true;
        if (transaction.type < 8) profile.types |= @as(u8, 1) << @intCast(transaction.type);
        profile.max_data_len = @max(profile.max_data_len, transaction.data.len);
        profile.access_list = profile.access_list or transaction.access_list.len > 0;
        profile.authorization_list = profile.authorization_list or transaction.authorization_list.len > 0;
    }
    return .{
        .rlp = try decodeHexAlloc(arena, rlp_text),
        .hash = try decodeHex(32, header.hash),
        .parent_hash = try decodeHex(32, header.parentHash),
        .prev_randao = try decodeHex(32, header.mixHash),
        .number = try std.fmt.parseInt(u64, header.number, 0),
        .timestamp = try std.fmt.parseInt(u64, header.timestamp, 0),
        .transactions = transactions,
        .senders = senders,
        .blob_transaction = blob_transaction,
        .profile = profile,
    };
}

fn transactionOf(arena: Allocator, raw: RawTransaction) !Transaction {
    const access_list = try arena.alloc(AccessListEntry, raw.accessList.len);
    for (raw.accessList, access_list) |raw_entry, *entry| {
        const keys = try arena.alloc(Hash, raw_entry.storageKeys.len);
        for (raw_entry.storageKeys, keys) |key, *decoded| decoded.* = try decodeHex(32, key);
        entry.* = .{ .address = try decodeHex(20, raw_entry.address), .storage_keys = keys };
    }
    const authorization_list = try arena.alloc(Authorization, raw.authorizationList.len);
    for (raw.authorizationList, authorization_list) |raw_authorization, *authorization| {
        authorization.* = .{
            .chain_id = try std.fmt.parseInt(u256, raw_authorization.chainId, 0),
            .address = try decodeHex(20, raw_authorization.address),
            .nonce = try std.fmt.parseInt(u64, raw_authorization.nonce, 0),
            .y_parity = try std.fmt.parseInt(u256, raw_authorization.yParity, 0),
            .r = try std.fmt.parseInt(u256, raw_authorization.r, 0),
            .s = try std.fmt.parseInt(u256, raw_authorization.s, 0),
        };
    }
    const fee_cap = raw.gasPrice orelse raw.maxFeePerGas orelse return error.TransactionWithoutGasPrice;
    return .{
        .type = try std.fmt.parseInt(u8, raw.type, 0),
        .nonce = try std.fmt.parseInt(u64, raw.nonce, 0),
        .gas_price = try std.fmt.parseInt(u256, fee_cap, 0),
        .max_priority_fee_per_gas = if (raw.maxPriorityFeePerGas) |fee| try std.fmt.parseInt(u256, fee, 0) else null,
        .gas_limit = try std.fmt.parseInt(u64, raw.gasLimit, 0),
        .to = if (digits(raw.to).len == 0) null else try decodeHex(20, raw.to),
        .value = try std.fmt.parseInt(u256, raw.value, 0),
        .data = try decodeHexAlloc(arena, raw.data),
        .access_list = access_list,
        .authorization_list = authorization_list,
    };
}

/// A block at a known position, with the ID of the chain its case runs on.
pub const Located = struct {
    chain_id: u64,
    fact: Fact,
};

/// The case `case_name` of the fixture file `path`, which is relative to the corpus directory
/// `corpus_dir`. It and the file's other allocations belong to `arena`.
pub fn locateCase(arena: Allocator, io: std.Io, corpus_dir: []const u8, path: []const u8, case_name: []const u8) !Case {
    const full_path = try std.fs.path.join(arena, &.{ corpus_dir, path });
    const text = try std.Io.Dir.cwd().readFileAlloc(io, full_path, arena, .limited(1 << 30));
    return caseOf(arena, text, path, case_name);
}

/// The case `case_name` of the fixture JSON `text`, which came from the file `path`. The case and
/// the allocations of parsing belong to `arena`.
pub fn caseOf(arena: Allocator, text: []const u8, path: []const u8, case_name: []const u8) !Case {
    const cases = try std.json.parseFromSliceLeaky(std.json.ArrayHashMap(RawCase), arena, text, .{ .ignore_unknown_fields = true });
    const raw = cases.map.get(case_name) orelse return error.CaseNotFound;
    const blocks = try arena.alloc(?Fact, raw.blocks.len);
    for (raw.blocks, blocks) |block, *fact| fact.* = try factOf(arena, block);
    return .{
        .path = path,
        .name = case_name,
        .chain_id = try std.fmt.parseInt(u64, raw.config.chainid, 0),
        .blocks = blocks,
    };
}

/// Block `index` of the case `case_name` in the fixture file `path`, which is relative to the corpus
/// directory `corpus_dir`. The block is one the fixture accepts, or `error.BlockNotAccepted`; it and
/// the file's other allocations belong to `arena`.
pub fn locate(arena: Allocator, io: std.Io, corpus_dir: []const u8, path: []const u8, case_name: []const u8, index: usize) !Located {
    const case = try locateCase(arena, io, corpus_dir, path, case_name);
    if (index >= case.blocks.len) return error.BlockNotFound;
    return .{
        .chain_id = case.chain_id,
        .fact = case.blocks[index] orelse return error.BlockNotAccepted,
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

//! The truncation of full block RLPs into the blocks of the DA payload. A truncated block keeps
//! the timestamp, the block hash, the prevRandao, the signature- and chain-id-stripped
//! transactions and their senders; `encodeBlocks` serializes a conflation's truncated blocks into
//! the canonical payload its zstd frame decompresses to.
//!
//! Full blocks are Amsterdam blocks: `[header, transactions, ommers, withdrawals]` in canonical
//! RLP, the encoding ending with the block, and the transactions are the ones the header's
//! transactions root commits to.

const std = @import("std");

const executor = @import("zesu_executor");
const input = @import("zesu_input");
const mpt = @import("zesu_mpt");
const rlp_decode = @import("zesu_rlp_decode");
const accelerators = @import("zesu_accelerators");

const Allocator = std.mem.Allocator;

const rlp = mpt.rlp;
const tx_decode = executor.executor_tx_decode;
const tx_signing = executor.executor_tx_signing;

pub const Hash = [32]u8;
pub const Address = [20]u8;

pub const Error = error{
    /// An allocator refused a request, whichever step made it.
    OutOfMemory,
    /// The bytes are not a canonical Amsterdam block, a transaction in it is not a canonical
    /// transaction of its type, or the transactions are not the ones the header commits to.
    MalformedBlock,
    /// A transaction's signature does not recover a sender under the chain ID, which has to be
    /// positive: its r or s is out of range, or the `v` of a legacy transaction is none of 27, 28
    /// and 35 or 36 above twice the chain ID.
    SenderRecoveryFailed,
};

pub const TruncatedBlock = struct {
    timestamp: u64,
    /// keccak256 of the block header's RLP.
    block_hash: Hash,
    /// The header's parent hash.
    parent_hash: Hash,
    prev_randao: Hash,
    /// The transactions without signature and chain ID.
    transactions: []const []const u8,
    froms: []const Address,
};

/// The truncation of `block_rlp` for the chain `chain_id`. The result is allocated from `output`;
/// `scratch` holds what the truncation needs along the way and can be reset afterwards. A
/// rejection leaves its allocations behind, so both allocators are arenas.
pub fn truncateBlockRlp(output: Allocator, scratch: Allocator, block_rlp: []const u8, chain_id: u64) Error!TruncatedBlock {
    // An EIP-155 signature is over a pre-image that commits to a chain ID, which is positive.
    if (chain_id == 0) return error.SenderRecoveryFailed;
    var block = Reader{ .rest = try singleListPayload(block_rlp) };
    const header = try block.next();
    const transactions_payload = try block.list();
    var ommers = Reader{ .rest = try block.list() };
    var withdrawals = Reader{ .rest = try block.list() };
    if (!block.atEnd()) return error.MalformedBlock;

    const header_fields = try parseHeader(try header.listPayload());
    // The block's own timestamp is a 64-bit integer, an ommer's a 256-bit one.
    const timestamp = std.math.cast(u64, header_fields.timestamp) orelse return error.MalformedBlock;
    while (!ommers.atEnd()) _ = try parseHeader(try (try ommers.next()).listPayload());
    while (!withdrawals.atEnd()) try parseWithdrawal(try (try withdrawals.next()).listPayload());

    // zesu reports a request its allocator refuses as a rejected encoding; the record tells the two apart.
    var refusals: Refusals = .{ .backing = scratch };
    const decoding = refusals.allocator();

    const wire = try wireTransactions(scratch, transactions_payload);
    const transactions_root = executor.computeRawTxRoot(decoding, wire) catch return refusals.failure(error.MalformedBlock);
    if (!std.mem.eql(u8, &transactions_root, &header_fields.transactions_root)) return error.MalformedBlock;

    try checkLegacySignatureValues(transactions_payload, chain_id);
    const decoded = rlp_decode.decodeTxList(decoding, transactions_payload) catch return refusals.failure(error.MalformedBlock);
    const senders_input = tx_decode.decodeTxsFromInput(decoding, decoded) catch return refusals.failure(error.MalformedBlock);
    if (decoded.len != wire.len) return error.MalformedBlock;

    const stripped = try output.alloc([]const u8, decoded.len);
    const froms = try output.alloc(Address, decoded.len);
    for (decoded, senders_input, wire, stripped, froms) |tx, tx_input, wire_tx, *stripped_tx, *from| {
        // Blob and set-code transactions name a destination, and a blob transaction a blob.
        if (tx.to == null and (tx.tx_type == 3 or tx.tx_type == 4)) return error.MalformedBlock;
        if (tx.tx_type == 3 and tx.blob_hashes.len == 0) return error.MalformedBlock;
        // The decoded fields are the transaction's own when they encode back to its bytes.
        const hash = tx_signing.txHash(decoding, &tx_input, chain_id) catch return refusals.failure(error.MalformedBlock);
        var wire_hash: Hash = undefined;
        accelerators.keccak256(wire_tx, &wire_hash);
        if (!std.mem.eql(u8, &hash, &wire_hash)) return error.MalformedBlock;

        stripped_tx.* = try strippedTransaction(output, scratch, tx);
        const sender = tx_signing.recoverSender(decoding, &tx_input, chain_id, .amsterdam) catch return refusals.failure(error.SenderRecoveryFailed);
        from.* = sender orelse return error.SenderRecoveryFailed;
    }

    var block_hash: Hash = undefined;
    accelerators.keccak256(header.encoded, &block_hash);
    return .{
        .timestamp = timestamp,
        .block_hash = block_hash,
        .parent_hash = header_fields.parent_hash,
        .prev_randao = header_fields.prev_randao,
        .transactions = stripped,
        .froms = froms,
    };
}

/// The canonical payload of a conflation's truncated blocks, in the RLP list layout
/// `[[timestamp, blockHash, prevRandao, [transaction…], [from…]]…]`.
pub fn encodeBlocks(allocator: Allocator, blocks: []const TruncatedBlock) Allocator.Error![]u8 {
    var builder: RlpBuilder = .{ .allocator = allocator };
    errdefer builder.bytes.deinit(allocator);
    const all_blocks = builder.openList();
    for (blocks) |block| {
        const entry = builder.openList();
        try builder.unsigned(block.timestamp);
        try builder.string(&block.block_hash);
        try builder.string(&block.prev_randao);
        const transactions = builder.openList();
        for (block.transactions) |transaction| try builder.string(transaction);
        try builder.closeList(transactions);
        const froms = builder.openList();
        for (block.froms) |from| try builder.string(&from);
        try builder.closeList(froms);
        try builder.closeList(entry);
    }
    try builder.closeList(all_blocks);
    return builder.bytes.toOwnedSlice(allocator);
}

/// The transaction without its signature and chain ID: the legacy field list, or the type byte
/// and the typed field list.
fn strippedTransaction(output: Allocator, scratch: Allocator, tx: input.Transaction) Error![]u8 {
    var builder: RlpBuilder = .{ .allocator = scratch };
    if (tx.tx_type != 0) try builder.bytes.append(scratch, tx.tx_type);
    const fields = builder.openList();
    try builder.unsigned(tx.nonce);
    if (tx.tx_type <= 1) {
        try builder.unsigned(tx.gas_price);
    } else {
        try builder.unsigned(tx.gas_priority_fee.?);
        try builder.unsigned(tx.gas_price);
    }
    try builder.unsigned(tx.gas_limit);
    try builder.string(if (tx.to) |*address| address else &.{});
    try builder.unsigned(tx.value);
    try builder.string(tx.data);
    if (tx.tx_type != 0) {
        const access_list = builder.openList();
        for (tx.access_list) |entry| {
            const item = builder.openList();
            try builder.string(&entry.address);
            const keys = builder.openList();
            for (entry.storage_keys) |key| try builder.string(&key);
            try builder.closeList(keys);
            try builder.closeList(item);
        }
        try builder.closeList(access_list);
    }
    if (tx.tx_type == 3) {
        try builder.unsigned(tx.max_fee_per_blob_gas);
        const hashes = builder.openList();
        for (tx.blob_hashes) |hash| try builder.string(&hash);
        try builder.closeList(hashes);
    }
    if (tx.tx_type == 4) {
        const authorizations = builder.openList();
        for (tx.authorization_list) |authorization| {
            const item = builder.openList();
            try builder.unsigned(authorization.chain_id);
            try builder.string(&authorization.address);
            try builder.unsigned(authorization.nonce);
            try builder.unsigned(authorization.v);
            try builder.unsigned(authorization.r);
            try builder.unsigned(authorization.s);
            try builder.closeList(item);
        }
        try builder.closeList(authorizations);
    }
    try builder.closeList(fields);
    return output.dupe(u8, builder.bytes.items);
}

const HeaderFields = struct {
    parent_hash: Hash,
    transactions_root: Hash,
    timestamp: u256,
    prev_randao: Hash,
};

/// The fields of an Amsterdam header's RLP list, all of which have to be present and canonical.
fn parseHeader(payload: []const u8) Error!HeaderFields {
    var fields = Reader{ .rest = payload };
    const parent_hash = (try fields.fixed(32)).*;
    _ = try fields.fixed(32); // ommers hash
    _ = try fields.fixed(20); // coinbase
    _ = try fields.fixed(32); // state root
    const transactions_root = (try fields.fixed(32)).*;
    _ = try fields.fixed(32); // receipts root
    _ = try fields.fixed(256); // logs bloom
    _ = try fields.integer(); // difficulty
    _ = try fields.integer(); // number
    _ = try fields.integer(); // gas limit
    _ = try fields.integer(); // gas used
    const timestamp = try fields.unsigned(u256);
    _ = try fields.string(); // extra data
    const prev_randao = (try fields.fixed(32)).*;
    _ = try fields.fixed(8); // nonce
    _ = try fields.integer(); // base fee per gas
    _ = try fields.fixed(32); // withdrawals root
    _ = try fields.unsigned(u64); // blob gas used
    _ = try fields.unsigned(u64); // excess blob gas
    _ = try fields.fixed(32); // parent beacon block root
    _ = try fields.fixed(32); // requests hash
    _ = try fields.fixed(32); // block access list hash
    _ = try fields.unsigned(u64); // slot number
    if (!fields.atEnd()) return error.MalformedBlock;
    return .{ .parent_hash = parent_hash, .transactions_root = transactions_root, .timestamp = timestamp, .prev_randao = prev_randao };
}

fn parseWithdrawal(payload: []const u8) Error!void {
    var fields = Reader{ .rest = payload };
    _ = try fields.unsigned(u64); // index
    _ = try fields.unsigned(u64); // validator index
    _ = try fields.fixed(20); // address
    _ = try fields.unsigned(u256); // amount
    if (!fields.atEnd()) return error.MalformedBlock;
}

/// The `v` of a legacy transaction is 27 or 28 for an unprotected signature, and 35 or 36 above
/// twice the chain ID for an EIP-155 one, which commits to the chain ID.
fn checkLegacySignatureValues(transactions_payload: []const u8, chain_id: u64) Error!void {
    const protected: u256 = 35 + 2 * @as(u256, chain_id);
    var entries = Reader{ .rest = transactions_payload };
    while (!entries.atEnd()) {
        const fields_payload = switch ((try entries.next()).value) {
            .list => |payload| payload,
            .bytes => continue,
        };
        var fields = Reader{ .rest = fields_payload };
        for (0..6) |_| _ = try fields.next();
        const v = try fields.unsigned(u256);
        if (v != 27 and v != 28 and v != protected and v != protected + 1) return error.SenderRecoveryFailed;
    }
}

/// The lists a transaction nests: its fields, an access list, an access-list entry and its keys.
const transaction_depth = 4;

/// The wire form of each entry of a body's transaction list, which is what the transactions trie
/// holds: a legacy transaction's own list, or a typed transaction's type byte and field list.
/// Every entry is well formed to the depth of a transaction.
fn wireTransactions(scratch: Allocator, payload: []const u8) Error![]const []const u8 {
    var wire: std.ArrayList([]const u8) = .empty;
    var entries = Reader{ .rest = payload };
    while (!entries.atEnd()) {
        const entry = try entries.next();
        const wire_form, const fields = switch (entry.value) {
            .list => .{ entry.encoded, entry.encoded },
            .bytes => |envelope| if (envelope.len < 2) return error.MalformedBlock else .{ envelope, envelope[1..] },
        };
        if (!wellFormed(fields, transaction_depth)) return error.MalformedBlock;
        try wire.append(scratch, wire_form);
    }
    return wire.items;
}

/// Whether `bytes` are consecutive RLP items that nest lists to `depth` at most, each declared
/// length within the bytes that remain.
fn wellFormed(bytes: []const u8, depth: usize) bool {
    var rest = bytes;
    while (rest.len > 0) {
        if (!itemFits(rest)) return false;
        const decoded = rlp.decodeItem(rest) catch return false;
        switch (decoded.item) {
            .bytes => {},
            .list => |payload| if (depth == 0 or !wellFormed(payload, depth - 1)) return false,
        }
        rest = rest[decoded.consumed..];
    }
    return true;
}

/// Whether the item `bytes` start with, its header and the payload length the header declares,
/// fits in `bytes`.
fn itemFits(bytes: []const u8) bool {
    if (bytes.len == 0) return false;
    const first = bytes[0];
    if (first < 0x80) return true;
    const is_string = first < 0xc0;
    const last_short: u8 = if (is_string) 0xb7 else 0xf7;
    if (first <= last_short) return bytes.len - 1 >= first - (if (is_string) @as(u8, 0x80) else 0xc0);
    const len_len: usize = first - last_short;
    if (bytes.len - 1 < len_len) return false;
    var len: u64 = 0;
    for (bytes[1..][0..len_len]) |byte| len = len << 8 | byte;
    return len <= bytes.len - 1 - len_len;
}

/// The payload of `encoded`, which is one list and nothing else.
fn singleListPayload(encoded: []const u8) Error![]const u8 {
    var reader = Reader{ .rest = encoded };
    const payload = try (try reader.next()).listPayload();
    if (!reader.atEnd()) return error.MalformedBlock;
    return payload;
}

/// An allocator that records whether its backing allocator refused a request.
const Refusals = struct {
    backing: Allocator,
    refused: bool = false,

    const vtable: Allocator.VTable = .{ .alloc = alloc, .resize = resize, .remap = remap, .free = free };

    fn allocator(self: *Refusals) Allocator {
        return .{ .ptr = self, .vtable = &vtable };
    }

    /// `rejection` unless a request was refused, which is `error.OutOfMemory`.
    fn failure(self: Refusals, rejection: Error) Error {
        return if (self.refused) error.OutOfMemory else rejection;
    }

    fn alloc(context: *anyopaque, len: usize, alignment: std.mem.Alignment, ret_addr: usize) ?[*]u8 {
        const self: *Refusals = @ptrCast(@alignCast(context));
        const memory = self.backing.rawAlloc(len, alignment, ret_addr);
        if (memory == null) self.refused = true;
        return memory;
    }

    fn resize(context: *anyopaque, memory: []u8, alignment: std.mem.Alignment, new_len: usize, ret_addr: usize) bool {
        const self: *Refusals = @ptrCast(@alignCast(context));
        return self.backing.rawResize(memory, alignment, new_len, ret_addr);
    }

    fn remap(context: *anyopaque, memory: []u8, alignment: std.mem.Alignment, new_len: usize, ret_addr: usize) ?[*]u8 {
        const self: *Refusals = @ptrCast(@alignCast(context));
        return self.backing.rawRemap(memory, alignment, new_len, ret_addr);
    }

    fn free(context: *anyopaque, memory: []u8, alignment: std.mem.Alignment, ret_addr: usize) void {
        const self: *Refusals = @ptrCast(@alignCast(context));
        self.backing.rawFree(memory, alignment, ret_addr);
    }
};

/// Reads consecutive canonical RLP items.
const Reader = struct {
    rest: []const u8,

    const Item = struct {
        value: rlp.RlpItem,
        /// The item's full encoding, header included.
        encoded: []const u8,

        fn listPayload(item: Item) Error![]const u8 {
            return switch (item.value) {
                .list => |payload| payload,
                .bytes => error.MalformedBlock,
            };
        }
    };

    fn atEnd(self: Reader) bool {
        return self.rest.len == 0;
    }

    fn next(self: *Reader) Error!Item {
        if (!itemFits(self.rest)) return error.MalformedBlock;
        const decoded = rlp.decodeItemCanonical(self.rest) catch return error.MalformedBlock;
        const encoded = self.rest[0..decoded.consumed];
        self.rest = self.rest[decoded.consumed..];
        return .{ .value = decoded.item, .encoded = encoded };
    }

    fn list(self: *Reader) Error![]const u8 {
        return (try self.next()).listPayload();
    }

    fn string(self: *Reader) Error![]const u8 {
        return switch ((try self.next()).value) {
            .bytes => |bytes| bytes,
            .list => error.MalformedBlock,
        };
    }

    fn fixed(self: *Reader, comptime len: usize) Error!*const [len]u8 {
        const bytes = try self.string();
        if (bytes.len != len) return error.MalformedBlock;
        return bytes[0..len];
    }

    /// A non-negative integer of any size: big-endian without leading zero bytes.
    fn integer(self: *Reader) Error![]const u8 {
        const bytes = try self.string();
        if (bytes.len > 0 and bytes[0] == 0) return error.MalformedBlock;
        return bytes;
    }

    fn unsigned(self: *Reader, comptime T: type) Error!T {
        const bytes = try self.integer();
        if (bytes.len > @sizeOf(T)) return error.MalformedBlock;
        var value: T = 0;
        for (bytes) |byte| value = value << 8 | byte;
        return value;
    }
};

/// Writes RLP into a byte buffer; a list's length prefix is inserted when the list is closed.
const RlpBuilder = struct {
    allocator: Allocator,
    bytes: std.ArrayList(u8) = .empty,

    /// The position a list's payload starts at, for `closeList`.
    const Marker = usize;

    fn string(self: *RlpBuilder, data: []const u8) Allocator.Error!void {
        if (data.len == 1 and data[0] < 0x80) return self.bytes.append(self.allocator, data[0]);
        var prefix: [9]u8 = undefined;
        try self.bytes.appendSlice(self.allocator, lengthPrefix(0x80, data.len, &prefix));
        try self.bytes.appendSlice(self.allocator, data);
    }

    fn unsigned(self: *RlpBuilder, value: u256) Allocator.Error!void {
        var big_endian: [32]u8 = undefined;
        std.mem.writeInt(u256, &big_endian, value, .big);
        try self.string(big_endian[@clz(value) / 8 ..]);
    }

    fn openList(self: *RlpBuilder) Marker {
        return self.bytes.items.len;
    }

    fn closeList(self: *RlpBuilder, start: Marker) Allocator.Error!void {
        var prefix: [9]u8 = undefined;
        try self.bytes.insertSlice(self.allocator, start, lengthPrefix(0xc0, self.bytes.items.len - start, &prefix));
    }

    /// The prefix of a string (`base` 0x80) or list (`base` 0xc0) of `len` payload bytes.
    fn lengthPrefix(base: u8, len: usize, out: *[9]u8) []const u8 {
        if (len <= 55) {
            out[0] = base + @as(u8, @intCast(len));
            return out[0..1];
        }
        var big_endian: [8]u8 = undefined;
        std.mem.writeInt(u64, &big_endian, len, .big);
        const len_len = 8 - @clz(@as(u64, len)) / 8;
        out[0] = base + 55 + @as(u8, @intCast(len_len));
        @memcpy(out[1..][0..len_len], big_endian[8 - len_len ..]);
        return out[0 .. 1 + len_len];
    }
};

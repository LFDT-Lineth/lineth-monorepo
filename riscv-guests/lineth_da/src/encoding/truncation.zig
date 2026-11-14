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
const accelerators = @import("zkvm_accelerators");

const Allocator = std.mem.Allocator;

const rlp = mpt.rlp;
const rlp_encode = executor.executor_rlp_encode;
const tx_decode = executor.executor_tx_decode;
const tx_signing = executor.executor_tx_signing;

pub const Hash = [32]u8;
pub const Address = [20]u8;

pub const Error = error{
    /// An allocator refused a request, whichever step made it.
    OutOfMemory,
    /// The bytes are not a canonical Amsterdam block, a transaction in it is of a type outside
    /// legacy, access-list, fee-market and set-code (a blob transaction included) or cannot be
    /// decoded, or the transactions are not the ones the header commits to.
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
    for (decoded, senders_input, stripped, froms) |tx, tx_input, *stripped_tx, *from| {
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
/// `[[timestamp, blockHash, prevRandao, [transaction…], [from…]]…]`. The result is allocated from
/// `output`; `scratch` holds the encodings of the nested lists and can be reset afterwards.
pub fn encodeBlocks(output: Allocator, scratch: Allocator, blocks: []const TruncatedBlock) Allocator.Error![]u8 {
    var all_blocks: Items = .{ .allocator = scratch };
    for (blocks) |block| {
        var entry: Items = .{ .allocator = scratch };
        try entry.uint(block.timestamp);
        try entry.string(&block.block_hash);
        try entry.string(&block.prev_randao);
        var transactions: Items = .{ .allocator = scratch };
        for (block.transactions) |transaction| try transactions.string(transaction);
        try entry.list(transactions);
        var froms: Items = .{ .allocator = scratch };
        for (block.froms) |from| try froms.string(&from);
        try entry.list(froms);
        try all_blocks.list(entry);
    }
    return output.dupe(u8, try all_blocks.finish());
}

/// The transaction without its signature and chain ID: the legacy field list, or the type byte
/// and the typed field list.
fn strippedTransaction(output: Allocator, scratch: Allocator, tx: input.Transaction) Error![]u8 {
    var fields: Items = .{ .allocator = scratch };
    try fields.uint(tx.nonce);
    if (tx.tx_type <= 1) {
        try fields.uint(tx.gas_price);
    } else {
        try fields.uint(tx.gas_priority_fee.?);
        try fields.uint(tx.gas_price);
    }
    try fields.uint(tx.gas_limit);
    try fields.string(if (tx.to) |*address| address else &.{});
    try fields.uint(tx.value);
    try fields.string(tx.data);
    if (tx.tx_type != 0) {
        var access_list: Items = .{ .allocator = scratch };
        for (tx.access_list) |entry| {
            var keys: Items = .{ .allocator = scratch };
            for (entry.storage_keys) |key| try keys.string(&key);
            var item: Items = .{ .allocator = scratch };
            try item.string(&entry.address);
            try item.list(keys);
            try access_list.list(item);
        }
        try fields.list(access_list);
    }
    if (tx.tx_type == 4) {
        var authorizations: Items = .{ .allocator = scratch };
        for (tx.authorization_list) |authorization| {
            var item: Items = .{ .allocator = scratch };
            try item.uint(authorization.chain_id);
            try item.string(&authorization.address);
            try item.uint(authorization.nonce);
            try item.uint(authorization.v);
            try item.uint(authorization.r);
            try item.uint(authorization.s);
            try authorizations.list(item);
        }
        try fields.list(authorizations);
    }

    const encoded = try fields.finish();
    if (tx.tx_type == 0) return output.dupe(u8, encoded);
    const stripped = try output.alloc(u8, 1 + encoded.len);
    stripped[0] = tx.tx_type;
    @memcpy(stripped[1..], encoded);
    return stripped;
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
/// holds: a legacy transaction's own list, or the type byte and field list of an access-list,
/// fee-market or set-code transaction. Every entry is well formed to the depth of a transaction.
fn wireTransactions(scratch: Allocator, payload: []const u8) Error![]const []const u8 {
    var wire: std.ArrayList([]const u8) = .empty;
    var entries = Reader{ .rest = payload };
    while (!entries.atEnd()) {
        const entry = try entries.next();
        const wire_form, const fields = switch (entry.value) {
            .list => .{ entry.encoded, entry.encoded },
            .bytes => |envelope| if (envelope.len < 2 or !isEnvelopeType(envelope[0])) return error.MalformedBlock else .{ envelope, envelope[1..] },
        };
        if (!wellFormed(fields, transaction_depth)) return error.MalformedBlock;
        try wire.append(scratch, wire_form);
    }
    return wire.items;
}

/// Whether `type_byte` is the EIP-2718 type of an access-list (1), fee-market (2) or set-code (4) transaction.
fn isEnvelopeType(type_byte: u8) bool {
    return type_byte == 1 or type_byte == 2 or type_byte == 4;
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

/// The encoded items of one RLP list, built with zesu's RLP encoder.
const Items = struct {
    allocator: Allocator,
    encoded: std.ArrayList([]const u8) = .empty,

    fn uint(self: *Items, value: u256) Allocator.Error!void {
        try self.encoded.append(self.allocator, try rlp_encode.encodeU256(self.allocator, value));
    }

    fn string(self: *Items, data: []const u8) Allocator.Error!void {
        try self.encoded.append(self.allocator, try rlp_encode.encodeBytes(self.allocator, data));
    }

    /// Appends the list of the items of `nested`.
    fn list(self: *Items, nested: Items) Allocator.Error!void {
        try self.encoded.append(self.allocator, try nested.finish());
    }

    fn finish(self: Items) Allocator.Error![]u8 {
        return rlp_encode.encodeList(self.allocator, self.encoded.items);
    }
};

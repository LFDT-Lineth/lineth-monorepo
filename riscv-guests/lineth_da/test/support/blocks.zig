//! Signed transactions and Amsterdam blocks, described field by field and encoded with the test-side
//! RLP encoder, together with the signature-stripped form of each transaction.

const std = @import("std");

const rlp = @import("rlp.zig");
const secp256k1 = @import("secp256k1.zig");

const Allocator = std.mem.Allocator;
const Item = rlp.Item;

pub const Address = [20]u8;
pub const Hash = [32]u8;

pub const AccessListEntry = struct {
    address: Address,
    storage_keys: []const Hash,
};

pub const Authorization = struct {
    chain_id: u256,
    address: Address,
    nonce: u64,
    y_parity: u8,
    r: u256,
    s: u256,
};

pub const Kind = enum(u8) { legacy = 0, access_list = 1, fee_market = 2, blob = 3, set_code = 4 };

/// A transaction without its signature and chain ID.
pub const Tx = struct {
    kind: Kind,
    nonce: u64,
    /// The gas price of a legacy or access-list transaction, the max fee per gas of the others.
    gas_price: u128,
    max_priority_fee_per_gas: u128 = 0,
    gas_limit: u64,
    to: ?Address,
    value: u256,
    data: []const u8,
    access_list: []const AccessListEntry = &.{},
    max_fee_per_blob_gas: u128 = 0,
    blob_hashes: []const Hash = &.{},
    authorizations: []const Authorization = &.{},
};

pub const Signature = struct {
    /// The legacy `v` or the typed `yParity`.
    v: u256,
    r: u256,
    s: u256,
};

fn word(value: u256) [32]u8 {
    var bytes: [32]u8 = undefined;
    std.mem.writeInt(u256, &bytes, value, .big);
    return bytes;
}

fn keccak(bytes: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &hash, .{});
    return hash;
}

fn items(allocator: Allocator, comptime count: usize, values: [count]Item) Allocator.Error![]Item {
    return allocator.dupe(Item, &values);
}

fn accessListItem(allocator: Allocator, entries: []const AccessListEntry) Allocator.Error!Item {
    const out = try allocator.alloc(Item, entries.len);
    for (entries, out) |entry, *slot| {
        const keys = try allocator.alloc(Item, entry.storage_keys.len);
        for (entry.storage_keys, keys) |key, *key_slot| key_slot.* = rlp.string(try allocator.dupe(u8, &key));
        slot.* = rlp.list(try items(allocator, 2, .{ rlp.string(try allocator.dupe(u8, &entry.address)), rlp.list(keys) }));
    }
    return rlp.list(out);
}

/// The fields every form of the transaction carries, in wire order, without chain ID and signature.
fn bodyItems(allocator: Allocator, tx: Tx) Allocator.Error![]Item {
    var out: std.ArrayList(Item) = .empty;
    try out.append(allocator, try rlp.int(allocator, tx.nonce));
    switch (tx.kind) {
        .legacy, .access_list => try out.append(allocator, try rlp.int(allocator, tx.gas_price)),
        else => {
            try out.append(allocator, try rlp.int(allocator, tx.max_priority_fee_per_gas));
            try out.append(allocator, try rlp.int(allocator, tx.gas_price));
        },
    }
    try out.append(allocator, try rlp.int(allocator, tx.gas_limit));
    try out.append(allocator, rlp.string(if (tx.to) |address| try allocator.dupe(u8, &address) else &.{}));
    try out.append(allocator, try rlp.int(allocator, tx.value));
    try out.append(allocator, rlp.string(tx.data));
    if (tx.kind != .legacy) try out.append(allocator, try accessListItem(allocator, tx.access_list));
    if (tx.kind == .blob) {
        try out.append(allocator, try rlp.int(allocator, tx.max_fee_per_blob_gas));
        const hashes = try allocator.alloc(Item, tx.blob_hashes.len);
        for (tx.blob_hashes, hashes) |hash, *slot| slot.* = rlp.string(try allocator.dupe(u8, &hash));
        try out.append(allocator, rlp.list(hashes));
    }
    if (tx.kind == .set_code) {
        const authorizations = try allocator.alloc(Item, tx.authorizations.len);
        for (tx.authorizations, authorizations) |authorization, *slot| {
            slot.* = rlp.list(try items(allocator, 6, .{
                try rlp.int(allocator, authorization.chain_id),
                rlp.string(try allocator.dupe(u8, &authorization.address)),
                try rlp.int(allocator, authorization.nonce),
                try rlp.int(allocator, authorization.y_parity),
                try rlp.int(allocator, authorization.r),
                try rlp.int(allocator, authorization.s),
            }));
        }
        try out.append(allocator, rlp.list(authorizations));
    }
    return out.toOwnedSlice(allocator);
}

fn typed(allocator: Allocator, kind: Kind, fields: []const Item) Allocator.Error![]u8 {
    const body = try rlp.encode(allocator, rlp.list(fields));
    return std.mem.concat(allocator, u8, &.{ &.{@intFromEnum(kind)}, body });
}

/// The signature-stripped form of the transaction: what a DA payload carries for it.
pub fn stripped(allocator: Allocator, tx: Tx) Allocator.Error![]u8 {
    const fields = try bodyItems(allocator, tx);
    if (tx.kind == .legacy) return rlp.encode(allocator, rlp.list(fields));
    return typed(allocator, tx.kind, fields);
}

/// The digest the sender signs. A legacy transaction signs an EIP-155 pre-image under `chain_id`
/// unless it is unprotected.
pub fn signingDigest(allocator: Allocator, tx: Tx, chain_id: u64, protected: bool) Allocator.Error!Hash {
    const body = try bodyItems(allocator, tx);
    if (tx.kind == .legacy) {
        const fields = if (protected)
            try std.mem.concat(allocator, Item, &.{ body, try items(allocator, 3, .{ try rlp.int(allocator, chain_id), rlp.string(&.{}), rlp.string(&.{}) }) })
        else
            body;
        return keccak(try rlp.encode(allocator, rlp.list(fields)));
    }
    const fields = try std.mem.concat(allocator, Item, &.{ &.{try rlp.int(allocator, chain_id)}, body });
    return keccak(try typed(allocator, tx.kind, fields));
}

/// The transaction in a block's transaction list: a legacy transaction is a list, a typed one a
/// string of its type byte and field list.
pub fn withSignature(allocator: Allocator, tx: Tx, chain_id: u64, signature: Signature) Allocator.Error!Item {
    const body = try bodyItems(allocator, tx);
    const signature_items = try items(allocator, 3, .{
        try rlp.int(allocator, signature.v),
        try rlp.int(allocator, signature.r),
        try rlp.int(allocator, signature.s),
    });
    if (tx.kind == .legacy) return rlp.list(try std.mem.concat(allocator, Item, &.{ body, signature_items }));
    const fields = try std.mem.concat(allocator, Item, &.{ &.{try rlp.int(allocator, chain_id)}, body, signature_items });
    return rlp.string(try typed(allocator, tx.kind, fields));
}

/// The transaction signed with `secret`. A legacy transaction is EIP-155 protected for `chain_id`.
pub fn signed(allocator: Allocator, tx: Tx, chain_id: u64, secret: [32]u8) Allocator.Error!Item {
    const digest = try signingDigest(allocator, tx, chain_id, true);
    const signature = secp256k1.sign(secret, digest);
    const v: u256 = if (tx.kind == .legacy) @as(u256, chain_id) * 2 + 35 + signature.recovery_id else signature.recovery_id;
    return withSignature(allocator, tx, chain_id, .{
        .v = v,
        .r = std.mem.readInt(u256, &signature.r, .big),
        .s = std.mem.readInt(u256, &signature.s, .big),
    });
}

/// The legacy transaction signed without EIP-155 protection (`v` of 27 or 28).
pub fn signedUnprotected(allocator: Allocator, tx: Tx, secret: [32]u8) Allocator.Error!Item {
    const digest = try signingDigest(allocator, tx, 0, false);
    const signature = secp256k1.sign(secret, digest);
    return withSignature(allocator, tx, 0, .{
        .v = 27 + @as(u256, signature.recovery_id),
        .r = std.mem.readInt(u256, &signature.r, .big),
        .s = std.mem.readInt(u256, &signature.s, .big),
    });
}

/// An Amsterdam header with its 23 fields.
pub const Header = struct {
    parent_hash: Hash = @splat(0x11),
    ommers_hash: Hash = @splat(0x12),
    coinbase: Address = @splat(0x13),
    state_root: Hash = @splat(0x14),
    transactions_root: Hash = @splat(0x15),
    receipts_root: Hash = @splat(0x16),
    logs_bloom: [256]u8 = @splat(0x17),
    difficulty: u256 = 0,
    number: u64 = 1000,
    gas_limit: u64 = 30_000_000,
    gas_used: u64 = 21_000,
    timestamp: u64 = 1_700_000_000,
    extra_data: []const u8 = "extra",
    prev_randao: Hash = @splat(0x18),
    nonce: [8]u8 = @splat(0),
    base_fee_per_gas: u64 = 7,
    withdrawals_root: Hash = @splat(0x19),
    blob_gas_used: u64 = 0,
    excess_blob_gas: u64 = 0,
    parent_beacon_block_root: Hash = @splat(0x1a),
    requests_hash: Hash = @splat(0x1b),
    block_access_list_hash: Hash = @splat(0x1c),
    slot_number: u64 = 5,
};

pub fn headerItems(allocator: Allocator, header: Header) Allocator.Error![]Item {
    return items(allocator, 23, .{
        rlp.string(try allocator.dupe(u8, &header.parent_hash)),
        rlp.string(try allocator.dupe(u8, &header.ommers_hash)),
        rlp.string(try allocator.dupe(u8, &header.coinbase)),
        rlp.string(try allocator.dupe(u8, &header.state_root)),
        rlp.string(try allocator.dupe(u8, &header.transactions_root)),
        rlp.string(try allocator.dupe(u8, &header.receipts_root)),
        rlp.string(try allocator.dupe(u8, &header.logs_bloom)),
        try rlp.int(allocator, header.difficulty),
        try rlp.int(allocator, header.number),
        try rlp.int(allocator, header.gas_limit),
        try rlp.int(allocator, header.gas_used),
        try rlp.int(allocator, header.timestamp),
        rlp.string(header.extra_data),
        rlp.string(try allocator.dupe(u8, &header.prev_randao)),
        rlp.string(try allocator.dupe(u8, &header.nonce)),
        try rlp.int(allocator, header.base_fee_per_gas),
        rlp.string(try allocator.dupe(u8, &header.withdrawals_root)),
        try rlp.int(allocator, header.blob_gas_used),
        try rlp.int(allocator, header.excess_blob_gas),
        rlp.string(try allocator.dupe(u8, &header.parent_beacon_block_root)),
        rlp.string(try allocator.dupe(u8, &header.requests_hash)),
        rlp.string(try allocator.dupe(u8, &header.block_access_list_hash)),
        try rlp.int(allocator, header.slot_number),
    });
}

/// The keccak256 of the header's RLP.
pub fn blockHash(allocator: Allocator, header: Header) Allocator.Error!Hash {
    return keccak(try rlp.encode(allocator, rlp.list(try headerItems(allocator, header))));
}

pub const Withdrawal = struct {
    index: u64,
    validator_index: u64,
    address: Address,
    amount: u256,
};

pub fn withdrawalItem(allocator: Allocator, withdrawal: Withdrawal) Allocator.Error!Item {
    return rlp.list(try items(allocator, 4, .{
        try rlp.int(allocator, withdrawal.index),
        try rlp.int(allocator, withdrawal.validator_index),
        rlp.string(try allocator.dupe(u8, &withdrawal.address)),
        try rlp.int(allocator, withdrawal.amount),
    }));
}

/// The block `[header, transactions, ommers, withdrawals]`, its header items given directly so a
/// test can alter them.
pub fn blockItem(allocator: Allocator, header_items: []const Item, transactions: []const Item, ommers: []const Item, withdrawals: []const Item) Allocator.Error!Item {
    return rlp.list(try items(allocator, 4, .{
        rlp.list(header_items),
        rlp.list(transactions),
        rlp.list(ommers),
        rlp.list(withdrawals),
    }));
}

/// The block with no ommers and one withdrawal per entry of `withdrawals`.
pub fn block(allocator: Allocator, header: Header, transactions: []const Item, withdrawals: []const Withdrawal) Allocator.Error![]u8 {
    const withdrawal_items = try allocator.alloc(Item, withdrawals.len);
    for (withdrawals, withdrawal_items) |withdrawal, *slot| slot.* = try withdrawalItem(allocator, withdrawal);
    return rlp.encode(allocator, try blockItem(allocator, try headerItems(allocator, header), transactions, &.{}, withdrawal_items));
}

/// A transaction with random fields of the given kind.
pub fn randomTx(allocator: Allocator, random: std.Random, kind: Kind) Allocator.Error!Tx {
    const data = try allocator.alloc(u8, random.uintLessThan(usize, 120));
    random.bytes(data);

    const entries = try allocator.alloc(AccessListEntry, if (kind == .legacy) 0 else random.uintLessThan(usize, 4));
    for (entries) |*entry| {
        entry.address = randomBytes(Address, random);
        const keys = try allocator.alloc(Hash, random.uintLessThan(usize, 4));
        for (keys) |*key| key.* = randomBytes(Hash, random);
        entry.storage_keys = keys;
    }

    const blob_hashes = try allocator.alloc(Hash, if (kind == .blob) 1 + random.uintLessThan(usize, 3) else 0);
    for (blob_hashes) |*hash| hash.* = randomBytes(Hash, random);

    const authorizations = try allocator.alloc(Authorization, if (kind == .set_code) random.uintLessThan(usize, 3) else 0);
    for (authorizations) |*authorization| authorization.* = .{
        .chain_id = randomInt(random, 8),
        .address = randomBytes(Address, random),
        .nonce = random.int(u64) >> random.uintLessThan(u6, 60),
        .y_parity = random.uintLessThan(u8, 2),
        .r = randomInt(random, 32),
        .s = randomInt(random, 32),
    };

    return .{
        .kind = kind,
        .nonce = random.int(u64) >> random.uintLessThan(u6, 63),
        .gas_price = @truncate(randomInt(random, 16)),
        .max_priority_fee_per_gas = @truncate(randomInt(random, 16)),
        .gas_limit = random.int(u64) >> random.uintLessThan(u6, 40),
        // Blob and set-code transactions have a destination.
        .to = if (kind == .blob or kind == .set_code or random.boolean()) randomBytes(Address, random) else null,
        .value = randomInt(random, 32),
        .data = data,
        .access_list = entries,
        .max_fee_per_blob_gas = @truncate(randomInt(random, 16)),
        .blob_hashes = blob_hashes,
        .authorizations = authorizations,
    };
}

pub fn randomBytes(comptime T: type, random: std.Random) T {
    var bytes: T = undefined;
    random.bytes(&bytes);
    return bytes;
}

/// An integer of up to `max_bytes` bytes whose length is random, zero included.
pub fn randomInt(random: std.Random, max_bytes: u8) u256 {
    const len = random.uintAtMost(u8, max_bytes);
    if (len == 0) return 0;
    var bytes: [32]u8 = @splat(0);
    random.bytes(bytes[32 - len ..]);
    return std.mem.readInt(u256, &bytes, .big);
}

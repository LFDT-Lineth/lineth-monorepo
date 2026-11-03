const std = @import("std");

const lineth_da = @import("lineth_da");
const support = @import("support");

const truncation = lineth_da.encoding.truncation;
const blocks = support.blocks;
const rlp = support.rlp;
const secp256k1 = support.secp256k1;

const Allocator = std.mem.Allocator;
const Item = rlp.Item;

const gpa = std.testing.allocator;

fn hex(comptime digits: []const u8) [digits.len / 2]u8 {
    return comptime blk: {
        var bytes: [digits.len / 2]u8 = undefined;
        _ = std.fmt.hexToBytes(&bytes, digits) catch @compileError("invalid hex " ++ digits);
        break :blk bytes;
    };
}

fn lit(comptime digits: []const u8) *const [digits.len / 2]u8 {
    const bytes = comptime hex(digits);
    return &bytes;
}

fn secretKey(index: u8) [32]u8 {
    var secret: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(&.{ 'k', 'e', 'y', index }, &secret, .{});
    return secret;
}

fn truncate(arena: Allocator, block_rlp: []const u8, chain: u64) !truncation.TruncatedBlock {
    var scratch = std.heap.ArenaAllocator.init(gpa);
    defer scratch.deinit();
    return truncation.truncateBlockRlp(arena, scratch.allocator(), block_rlp, chain);
}

/// The payload of `truncated_blocks`, allocated from `output`.
fn encode(output: Allocator, truncated_blocks: []const truncation.TruncatedBlock) ![]u8 {
    var scratch = std.heap.ArenaAllocator.init(gpa);
    defer scratch.deinit();
    return truncation.encodeBlocks(output, scratch.allocator(), truncated_blocks);
}

const chain_id = 59144;

/// The block whose transaction list is `transactions`, under the default header committing to them.
fn blockOf(arena: Allocator, transactions: []const Item) ![]u8 {
    return blocks.block(arena, try blocks.withTransactions(arena, .{}, transactions), transactions, &.{});
}

// The transactions of the hand-derived vectors, with the stripped form worked out by hand from
// the rule: legacy transactions keep `[nonce, gasPrice, gas, to, value, data]`, typed ones keep
// their type byte and the field list without chain ID, yParity, r and s.

const legacy_tx: blocks.Tx = .{
    .kind = .legacy,
    .nonce = 9,
    .gas_price = 20_000_000_000,
    .gas_limit = 21_000,
    .to = hex("35" ** 20),
    .value = 1_000_000_000_000_000_000,
    .data = &.{},
};
const legacy_stripped = hex("e9" ++ // list of 41 bytes
    "09" ++ // nonce 9
    "8504a817c800" ++ // gas price 20 gwei
    "825208" ++ // gas 21000
    "94" ++ "35" ** 20 ++ // to
    "880de0b6b3a7640000" ++ // value 1 ether
    "80"); // data

const access_list_tx: blocks.Tx = .{
    .kind = .access_list,
    .nonce = 1,
    .gas_price = 1_000_000_000,
    .gas_limit = 21_000,
    .to = hex("11" ** 20),
    .value = 0,
    .data = lit("abcd"),
    .access_list = &.{.{ .address = hex("22" ** 20), .storage_keys = &.{hex("33" ** 32)} }},
};
const access_list_stripped = hex("01" ++ // type
    "f85c" ++ // list of 92 bytes
    "01" ++ // nonce
    "843b9aca00" ++ // gas price 1 gwei
    "825208" ++ // gas
    "94" ++ "11" ** 20 ++ // to
    "80" ++ // value 0
    "82abcd" ++ // data
    "f838" ++ "f7" ++ "94" ++ "22" ** 20 ++ "e1" ++ "a0" ++ "33" ** 32); // access list of one entry with one key

const fee_market_tx: blocks.Tx = .{
    .kind = .fee_market,
    .nonce = 2,
    .gas_price = 2_000_000_000,
    .max_priority_fee_per_gas = 1_000_000_000,
    .gas_limit = 200_000,
    .to = hex("44" ** 20),
    .value = 5,
    .data = &.{0x00},
};
const fee_market_stripped = hex("02" ++ // type
    "e7" ++ // list of 39 bytes
    "02" ++ // nonce
    "843b9aca00" ++ // max priority fee 1 gwei
    "8477359400" ++ // max fee 2 gwei
    "83030d40" ++ // gas 200000
    "94" ++ "44" ** 20 ++ // to
    "05" ++ // value 5
    "00" ++ // data: the single byte 0x00
    "c0"); // empty access list

const blob_tx: blocks.Tx = .{
    .kind = .blob,
    .nonce = 3,
    .gas_price = 100,
    .max_priority_fee_per_gas = 1,
    .gas_limit = 100_000,
    .to = hex("55" ** 20),
    .value = 0,
    .data = &.{},
    .max_fee_per_blob_gas = 1_000_000,
    .blob_hashes = &.{hex("01" ++ "66" ** 31)},
};
const set_code_tx: blocks.Tx = .{
    .kind = .set_code,
    .nonce = 4,
    .gas_price = 2,
    .max_priority_fee_per_gas = 1,
    .gas_limit = 100_000,
    .to = hex("77" ** 20),
    .value = 0,
    .data = lit("1234"),
    .authorizations = &.{.{ .chain_id = 1, .address = hex("88" ** 20), .nonce = 0, .y_parity = 1, .r = 0x99, .s = 0xaa }},
};
const set_code_stripped = hex("04" ++ // type
    "f83f" ++ // list of 63 bytes
    "04" ++ // nonce
    "01" ++ // max priority fee
    "02" ++ // max fee
    "830186a0" ++ // gas 100000
    "94" ++ "77" ** 20 ++ // to
    "80" ++ // value 0
    "821234" ++ // data
    "c0" ++ // empty access list
    "dd" ++ "dc" ++ "01" ++ "94" ++ "88" ** 20 ++ "80" ++ "01" ++ "8199" ++ "81aa"); // authorization [chainId, address, nonce, yParity, r, s]

const vectors = [_]struct { tx: blocks.Tx, stripped: []const u8 }{
    .{ .tx = legacy_tx, .stripped = &legacy_stripped },
    .{ .tx = access_list_tx, .stripped = &access_list_stripped },
    .{ .tx = fee_market_tx, .stripped = &fee_market_stripped },
    .{ .tx = set_code_tx, .stripped = &set_code_stripped },
};

test "the stripped form of each transaction type is the hand-derived one" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    for (vectors, 0..) |vector, i| {
        const secret = secretKey(@intCast(i));
        // The vectors are worked out by hand and also match the test-side encoding of the rule.
        try std.testing.expectEqualSlices(u8, vector.stripped, try blocks.stripped(a, vector.tx));

        const block = try blockOf(a, &.{try blocks.signed(a, vector.tx, chain_id, secret)});
        const truncated = try truncate(a, block, chain_id);
        try std.testing.expectEqual(1, truncated.transactions.len);
        try std.testing.expectEqualSlices(u8, vector.stripped, truncated.transactions[0]);
        try std.testing.expectEqualSlices(u8, &secp256k1.addressOf(secret), &truncated.froms[0]);
    }
}

test "the EIP-155 example transaction is stripped and its sender recovered" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    // The signed transaction and sender of the EIP-155 specification's example (chain ID 1).
    const signed = hex("f86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83");
    const sender = hex("9d8a62f656a8d1615c1294fd71e9cfb3e4855a4f");

    const truncated = try truncate(a, try blockOf(a, &.{rlp.raw(&signed)}), 1);
    try std.testing.expectEqualSlices(u8, &legacy_stripped, truncated.transactions[0]);
    try std.testing.expectEqualSlices(u8, &sender, &truncated.froms[0]);

    // The signature commits to chain ID 1: no other chain accepts the transaction.
    try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, try blockOf(a, &.{rlp.raw(&signed)}), chain_id));
}

test "a block's truncation carries its header facts and the hash of its header" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const header: blocks.Header = .{
        .parent_hash = hex("a1" ** 32),
        .prev_randao = hex("b2" ** 32),
        .timestamp = 1_761_000_123,
        .number = 77,
    };
    const block = try blocks.block(a, header, &.{}, &.{.{ .index = 1, .validator_index = 2, .address = hex("c3" ** 20), .amount = 3 }});
    const truncated = try truncate(a, block, chain_id);

    try std.testing.expectEqual(1_761_000_123, truncated.timestamp);
    try std.testing.expectEqualSlices(u8, &header.parent_hash, &truncated.parent_hash);
    try std.testing.expectEqualSlices(u8, &header.prev_randao, &truncated.prev_randao);
    try std.testing.expectEqualSlices(u8, &(try blocks.blockHash(a, header)), &truncated.block_hash);
    try std.testing.expectEqual(0, truncated.transactions.len);
    try std.testing.expectEqual(0, truncated.froms.len);
}

test "the block hash covers the whole header" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const base = try truncate(a, try blocks.block(a, .{}, &.{}, &.{}), chain_id);
    const other_state = try truncate(a, try blocks.block(a, .{ .state_root = hex("ee" ** 32) }, &.{}, &.{}), chain_id);
    const other_slot = try truncate(a, try blocks.block(a, .{ .slot_number = 6 }, &.{}, &.{}), chain_id);
    try std.testing.expect(!std.mem.eql(u8, &base.block_hash, &other_state.block_hash));
    try std.testing.expect(!std.mem.eql(u8, &base.block_hash, &other_slot.block_hash));
}

test "typed transactions commit to their own chain ID and unprotected legacy ones to none" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(40);
    const sender = secp256k1.addressOf(secret);

    const typed = try blockOf(a, &.{try blocks.signed(a, fee_market_tx, 1, secret)});
    for ([_]u64{ 1, 2, chain_id }) |any_chain| {
        const truncated = try truncate(a, typed, any_chain);
        try std.testing.expectEqualSlices(u8, &sender, &truncated.froms[0]);
    }

    const unprotected = try blockOf(a, &.{try blocks.signedUnprotected(a, legacy_tx, secret)});
    for ([_]u64{ 1, chain_id }) |any_chain| {
        const truncated = try truncate(a, unprotected, any_chain);
        try std.testing.expectEqualSlices(u8, &sender, &truncated.froms[0]);
        try std.testing.expectEqualSlices(u8, &legacy_stripped, truncated.transactions[0]);
    }

    const protected = try blockOf(a, &.{try blocks.signed(a, legacy_tx, chain_id, secret)});
    try std.testing.expectEqualSlices(u8, &sender, &(try truncate(a, protected, chain_id)).froms[0]);
    try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, protected, 1));
}

/// `presented` under the signature of `signing`: what a sender publishes who signed the one and
/// presents the other.
fn presentedWithSignatureOf(a: Allocator, signing: blocks.Tx, presented: blocks.Tx, secret: [32]u8) !Item {
    const digest = try blocks.signingDigest(a, signing, chain_id, true);
    const signature = secp256k1.sign(secret, digest);
    const v: u256 = if (signing.kind == .legacy) @as(u256, chain_id) * 2 + 35 + signature.recovery_id else signature.recovery_id;
    return blocks.withSignature(a, presented, chain_id, .{
        .v = v,
        .r = std.mem.readInt(u256, &signature.r, .big),
        .s = std.mem.readInt(u256, &signature.s, .big),
    });
}

test "a transaction's sender is bound to its signed content" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(41);

    // The signature of one transaction on another: whoever recovers is not the signer.
    for ([_]blocks.Tx{ legacy_tx, fee_market_tx }) |tx| {
        var other = tx;
        other.nonce += 1;
        const forged = try presentedWithSignatureOf(a, tx, other, secret);
        const truncated = truncate(a, try blockOf(a, &.{forged}), chain_id) catch |err| {
            try std.testing.expect(err == error.SenderRecoveryFailed);
            continue;
        };
        try std.testing.expect(!std.mem.eql(u8, &secp256k1.addressOf(secret), &truncated.froms[0]));
    }
}

test "signatures outside the accepted range do not recover a sender" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(42);

    const digest = try blocks.signingDigest(a, fee_market_tx, chain_id, true);
    const signature = secp256k1.sign(secret, digest);
    const r = std.mem.readInt(u256, &signature.r, .big);
    const s = std.mem.readInt(u256, &signature.s, .big);
    const parity: u256 = signature.recovery_id;

    const accepted = try blocks.withSignature(a, fee_market_tx, chain_id, .{ .v = parity, .r = r, .s = s });
    _ = try truncate(a, try blockOf(a, &.{accepted}), chain_id);

    const rejected = [_]blocks.Signature{
        .{ .v = parity, .r = 0, .s = s },
        .{ .v = parity, .r = secp256k1.order, .s = s },
        .{ .v = parity, .r = r, .s = 0 },
        // The other form of the same signature: s above half the order (EIP-2).
        .{ .v = parity ^ 1, .r = r, .s = secp256k1.order - s },
        .{ .v = 2, .r = r, .s = s },
    };
    for (rejected) |signature_fields| {
        const forged = try blocks.withSignature(a, fee_market_tx, chain_id, signature_fields);
        try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, try blockOf(a, &.{forged}), chain_id));
    }
}

test "a legacy transaction whose v is none of 27, 28 and the chain's EIP-155 values is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const digest = try blocks.signingDigest(a, legacy_tx, chain_id, true);
    const signature = secp256k1.sign(secretKey(46), digest);
    const r = std.mem.readInt(u256, &signature.r, .big);
    const s = std.mem.readInt(u256, &signature.s, .big);

    const protected: u256 = 2 * @as(u256, chain_id) + 35;
    const rejected = [_]u256{
        0,
        1,
        26,
        29,
        34,
        // The values of the neighbouring chain IDs.
        protected - 1,
        protected + 2,
        // Beyond the chain IDs of a 64-bit integer, and the widest integer there is.
        2 * (@as(u256, 1) << 64) + 35,
        std.math.maxInt(u256),
    };
    for (rejected) |v| {
        const forged = try blocks.withSignature(a, legacy_tx, chain_id, .{ .v = v, .r = r, .s = s });
        try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, try blockOf(a, &.{forged}), chain_id));
    }
}

test "the payload of truncated blocks has the specified RLP layout" {
    // One block: [[timestamp, blockHash, prevRandao, [transaction…], [from…]]], worked out by hand.
    const one_block = [_]truncation.TruncatedBlock{.{
        .timestamp = 1_700_000_000,
        .block_hash = hex("aa" ** 32),
        .parent_hash = hex("00" ** 32),
        .prev_randao = hex("bb" ** 32),
        .transactions = &.{lit("0102")},
        .froms = &.{hex("cc" ** 20)},
    }};
    const expected = hex("f863" ++ // the list of blocks: 99 bytes
        "f861" ++ // the block: 97 bytes
        "846553f100" ++ // timestamp
        "a0" ++ "aa" ** 32 ++ // block hash
        "a0" ++ "bb" ** 32 ++ // prevRandao
        "c3" ++ "820102" ++ // transactions: one string of the two bytes 0102
        "d5" ++ "94" ++ "cc" ** 20); // froms: one address
    const payload = try encode(gpa, &one_block);
    defer gpa.free(payload);
    try std.testing.expectEqualSlices(u8, &expected, payload);

    // Two blocks, the second with a long transaction (a string past 55 bytes), two transactions and
    // two senders: a sibling entry after the first, a long block entry and a long list of blocks.
    const two_blocks = [_]truncation.TruncatedBlock{ one_block[0], .{
        .timestamp = 1,
        .block_hash = hex("dd" ** 32),
        .parent_hash = hex("00" ** 32),
        .prev_randao = hex("ee" ** 32),
        .transactions = &.{ lit("03"), lit("07" ** 56) },
        .froms = &.{ hex("f1" ** 20), hex("f2" ** 20) },
    } };
    const expected_two = hex("f90110" ++ // the list of blocks: 272 bytes
        "f861" ++ "846553f100" ++ "a0" ++ "aa" ** 32 ++ "a0" ++ "bb" ** 32 ++ "c3820102" ++ "d5" ++ "94" ++ "cc" ** 20 ++ // the first block
        "f8ab" ++ // the second block: 171 bytes
        "01" ++ // timestamp
        "a0" ++ "dd" ** 32 ++ // block hash
        "a0" ++ "ee" ** 32 ++ // prevRandao
        "f83b" ++ "03" ++ "b838" ++ "07" ** 56 ++ // transactions: the byte 03 and a string of 56 bytes
        "ea" ++ "94" ++ "f1" ** 20 ++ "94" ++ "f2" ** 20); // froms: two addresses
    const payload_two = try encode(gpa, &two_blocks);
    defer gpa.free(payload_two);
    try std.testing.expectEqualSlices(u8, &expected_two, payload_two);

    const none = try encode(gpa, &.{});
    defer gpa.free(none);
    try std.testing.expectEqualSlices(u8, lit("c0"), none);

    // A block without transactions has two empty lists; a zero timestamp is the empty string.
    const empty_block = [_]truncation.TruncatedBlock{.{
        .timestamp = 0,
        .block_hash = hex("aa" ** 32),
        .parent_hash = hex("00" ** 32),
        .prev_randao = hex("bb" ** 32),
        .transactions = &.{},
        .froms = &.{},
    }};
    const empty_payload = try encode(gpa, &empty_block);
    defer gpa.free(empty_payload);
    try std.testing.expectEqualSlices(u8, lit("f847" ++ "f845" ++ "80" ++ "a0" ++ "aa" ** 32 ++ "a0" ++ "bb" ** 32 ++ "c0" ++ "c0"), empty_payload);
}

test "the payload of a block is its truncation encoded" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(43);

    const block = try blockOf(a, &.{
        try blocks.signed(a, access_list_tx, chain_id, secret),
        try blocks.signed(a, legacy_tx, chain_id, secret),
    });
    const truncated = try truncate(a, block, chain_id);
    const payload = try encode(a, &.{truncated});

    // [[timestamp, hash, randao, [tx strings], [froms]]] built with the test-side encoder.
    const transaction_items = [_]Item{ rlp.string(truncated.transactions[0]), rlp.string(truncated.transactions[1]) };
    const from_items = [_]Item{ rlp.string(&truncated.froms[0]), rlp.string(&truncated.froms[1]) };
    const entry = [_]Item{
        try rlp.int(a, 1_700_000_000),
        rlp.string(&truncated.block_hash),
        rlp.string(&truncated.prev_randao),
        rlp.list(&transaction_items),
        rlp.list(&from_items),
    };
    try std.testing.expectEqualSlices(u8, try rlp.encode(a, rlp.list(&.{rlp.list(&entry)})), payload);
}

/// What a block's truncation has to be, from the transactions and keys it was built from.
const Expected = struct {
    stripped: []const []const u8,
    froms: []const [20]u8,
    /// The block's entry of the payload: `[timestamp, blockHash, prevRandao, [transaction…], [from…]]`.
    entry: Item,
};

/// The payload of a conflation whose blocks have the entries `entries`.
fn payloadOf(a: Allocator, entries: []const Item) ![]u8 {
    return rlp.encode(a, rlp.list(entries));
}

fn expectedOf(a: Allocator, header: blocks.Header, txs: []const blocks.Tx, secrets: []const [32]u8) !Expected {
    const stripped = try a.alloc([]const u8, txs.len);
    const froms = try a.alloc([20]u8, txs.len);
    const transaction_items = try a.alloc(Item, txs.len);
    const from_items = try a.alloc(Item, txs.len);
    for (txs, secrets, 0..) |tx, secret, i| {
        stripped[i] = try blocks.stripped(a, tx);
        froms[i] = secp256k1.addressOf(secret);
        transaction_items[i] = rlp.string(stripped[i]);
        from_items[i] = rlp.string(&froms[i]);
    }
    const hash = try blocks.blockHash(a, header);
    const entry = try a.dupe(Item, &.{
        try rlp.int(a, header.timestamp),
        rlp.string(try a.dupe(u8, &hash)),
        rlp.string(try a.dupe(u8, &header.prev_randao)),
        rlp.list(transaction_items),
        rlp.list(from_items),
    });
    return .{ .stripped = stripped, .froms = froms, .entry = rlp.list(entry) };
}

test "random blocks truncate to what their fields and signers say" {
    var prng = std.Random.DefaultPrng.init(2026);
    const random = prng.random();
    const kinds = blocks.accepted_kinds;

    for (0..60) |_| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        const a = arena.allocator();

        const chain = 1 + (random.int(u64) >> random.uintLessThan(u6, 60));
        const block_count = 1 + random.uintLessThan(usize, 4);
        const truncated_blocks = try a.alloc(truncation.TruncatedBlock, block_count);
        const entries = try a.alloc(Item, block_count);
        for (truncated_blocks, entries) |*truncated, *entry| {
            const extra = blocks.randomBytes([8]u8, random);
            const header: blocks.Header = .{
                .parent_hash = blocks.randomBytes([32]u8, random),
                .prev_randao = blocks.randomBytes([32]u8, random),
                .timestamp = random.int(u64) >> random.uintLessThan(u6, 40),
                .number = random.int(u64) >> random.uintLessThan(u6, 40),
                .extra_data = try a.dupe(u8, &extra),
            };

            const count = random.uintLessThan(usize, 8);
            const txs = try a.alloc(blocks.Tx, count);
            const secrets = try a.alloc([32]u8, count);
            const items = try a.alloc(Item, count);
            for (txs, secrets, items) |*tx, *secret, *item| {
                tx.* = try blocks.randomTx(a, random, kinds[random.uintLessThan(usize, kinds.len)]);
                secret.* = secretKey(random.uintLessThan(u8, 5));
                item.* = try blocks.signed(a, tx.*, chain, secret.*);
            }

            const committed = try blocks.withTransactions(a, header, items);
            truncated.* = try truncate(a, try blocks.block(a, committed, items, &.{}), chain);
            const expected = try expectedOf(a, committed, txs, secrets);

            try std.testing.expectEqual(header.timestamp, truncated.timestamp);
            try std.testing.expectEqualSlices(u8, &header.parent_hash, &truncated.parent_hash);
            try std.testing.expectEqualSlices(u8, &header.prev_randao, &truncated.prev_randao);
            try std.testing.expectEqual(count, truncated.transactions.len);
            for (expected.stripped, truncated.transactions) |want, got| try std.testing.expectEqualSlices(u8, want, got);
            for (expected.froms, truncated.froms) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
            entry.* = expected.entry;
        }
        try std.testing.expectEqualSlices(u8, try payloadOf(a, entries), try encode(a, truncated_blocks));
    }
}

test "the stripped transactions do not depend on who signed them" {
    var prng = std.Random.DefaultPrng.init(2027);
    const random = prng.random();
    const kinds = blocks.accepted_kinds;

    for (0..20) |_| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        const a = arena.allocator();

        const txs = try a.alloc(blocks.Tx, 4);
        for (txs) |*tx| tx.* = try blocks.randomTx(a, random, kinds[random.uintLessThan(usize, kinds.len)]);

        var results: [2]truncation.TruncatedBlock = undefined;
        for (&results, 0..) |*result, signer| {
            const items = try a.alloc(Item, txs.len);
            for (txs, items) |tx, *item| item.* = try blocks.signed(a, tx, chain_id, secretKey(@intCast(50 + signer)));
            result.* = try truncate(a, try blockOf(a, items), chain_id);
        }
        for (results[0].transactions, results[1].transactions) |first, second| try std.testing.expectEqualSlices(u8, first, second);
        try std.testing.expect(!std.mem.eql(u8, &results[0].froms[0], &results[1].froms[0]));
    }
}

test "large blocks keep the transactions and senders in order" {
    var prng = std.Random.DefaultPrng.init(2028);
    const random = prng.random();
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const count = 300;
    const txs = try a.alloc(blocks.Tx, count);
    const secrets = try a.alloc([32]u8, count);
    const items = try a.alloc(Item, count);
    for (txs, secrets, items, 0..) |*tx, *secret, *item, i| {
        tx.* = try blocks.randomTx(a, random, .fee_market);
        tx.nonce = i;
        secret.* = secretKey(@intCast(i % 7));
        item.* = try blocks.signed(a, tx.*, chain_id, secret.*);
    }
    const truncated = try truncate(a, try blockOf(a, items), chain_id);
    const expected = try expectedOf(a, try blocks.withTransactions(a, .{}, items), txs, secrets);
    for (expected.stripped, truncated.transactions) |want, got| try std.testing.expectEqualSlices(u8, want, got);
    for (expected.froms, truncated.froms) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
    try std.testing.expectEqualSlices(u8, try payloadOf(a, &.{expected.entry}), try encode(a, &.{truncated}));
}

// Malformed blocks. Each starts from a well-formed block and alters one thing.

fn headerWith(a: Allocator, index: usize, replacement: Item) ![]Item {
    const header_items = try blocks.headerItems(a, .{});
    header_items[index] = replacement;
    return header_items;
}

fn blockWithHeader(a: Allocator, header_items: []const Item) ![]u8 {
    return rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{}, &.{}));
}

test "a block with bytes after it is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const block = try blocks.block(a, .{}, &.{}, &.{});

    _ = try truncate(a, block, chain_id);
    try std.testing.expectError(error.MalformedBlock, truncate(a, try std.mem.concat(a, u8, &.{ block, &.{0} }), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, block[0 .. block.len - 1], chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, &.{}, chain_id));
}

test "a header with a missing, extra or misshapen field is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const all = try blocks.headerItems(a, .{});

    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, all[0..22]), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try std.mem.concat(a, Item, &.{ all, &.{rlp.string(&.{1})} })), chain_id));

    const short_hash = [_]u8{0x11} ** 31;
    const long_bloom = [_]u8{0x17} ** 257;
    const short_nonce = [_]u8{0} ** 7;
    const nine_bytes = [_]u8{1} ** 9;
    const cases = [_]struct { index: usize, replacement: Item }{
        .{ .index = 0, .replacement = rlp.string(&short_hash) }, // parent hash
        .{ .index = 6, .replacement = rlp.string(&long_bloom) }, // logs bloom
        .{ .index = 11, .replacement = try rlp.int(a, 1 << 64) }, // timestamp of 2^64
        .{ .index = 11, .replacement = rlp.string(&nine_bytes) }, // timestamp beyond 64 bits
        .{ .index = 11, .replacement = rlp.list(&.{}) }, // timestamp as a list
        .{ .index = 13, .replacement = rlp.string(&short_hash) }, // prevRandao
        .{ .index = 14, .replacement = rlp.string(&short_nonce) }, // nonce
        .{ .index = 17, .replacement = rlp.string(&nine_bytes) }, // blob gas used
        .{ .index = 18, .replacement = rlp.string(&nine_bytes) }, // excess blob gas
        .{ .index = 22, .replacement = rlp.string(&nine_bytes) }, // slot number
    };
    for (cases) |case| {
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, case.index, case.replacement)), chain_id));
    }
}

test "the largest blob gas used, excess blob gas and slot number of a header are accepted" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const largest: blocks.Header = .{ .blob_gas_used = std.math.maxInt(u64), .excess_blob_gas = std.math.maxInt(u64), .slot_number = std.math.maxInt(u64) };
    const truncated = try truncate(a, try blocks.block(a, largest, &.{}, &.{}), chain_id);
    try std.testing.expectEqualSlices(u8, &(try blocks.blockHash(a, largest)), &truncated.block_hash);
}

test "non-canonical RLP is rejected wherever it occurs" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    // An integer with a leading zero byte, and a single byte below 0x80 given a length prefix.
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, 8, rlp.raw(lit("820001")))), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, 8, rlp.raw(lit("8105")))), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, 12, rlp.raw(lit("8105")))), chain_id));

    // A legacy transaction whose one-byte data below 0x80 carries a length prefix.
    var one_byte_data = legacy_tx;
    one_byte_data.data = &.{0x05};
    const signed_item = try blocks.signed(a, one_byte_data, chain_id, secretKey(44));
    _ = try truncate(a, try blockOf(a, &.{signed_item}), chain_id);
    const fields = try a.dupe(Item, signed_item.list);
    fields[5] = rlp.raw(lit("8105"));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.list(fields)}), chain_id));
}

test "a block body with ommers or withdrawals of the wrong shape is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const header_items = try blocks.headerItems(a, .{});

    // Ommers have to be headers.
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{rlp.string(&.{1})}, &.{})), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{rlp.list(header_items[0..22])}, &.{})), chain_id));
    _ = try truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{rlp.list(header_items)}, &.{})), chain_id);

    // An ommer's timestamp is a 256-bit integer, where the block's own is a 64-bit one.
    const base = try truncate(a, try blocks.block(a, .{}, &.{}, &.{}), chain_id);
    for ([_]u256{ 1 << 64, std.math.maxInt(u256) }) |timestamp| {
        const ommer = try headerWith(a, 11, try rlp.int(a, timestamp));
        const truncated = try truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{rlp.list(ommer)}, &.{})), chain_id);
        try std.testing.expectEqual(base.timestamp, truncated.timestamp);
        try std.testing.expectEqualSlices(u8, &base.block_hash, &truncated.block_hash);
    }
    const wide_timestamp = [_]u8{1} ** 33;
    const wide_ommer = try headerWith(a, 11, rlp.string(&wide_timestamp));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{rlp.list(wide_ommer)}, &.{})), chain_id));

    // Withdrawals have four fields.
    const three_fields = rlp.list(&.{ rlp.string(&.{1}), rlp.string(&.{2}), rlp.string(lit("c3" ** 20)) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{}, &.{three_fields})), chain_id));

    // A block has four parts.
    const three_parts = rlp.list(&.{ rlp.list(header_items), rlp.list(&.{}), rlp.list(&.{}) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, three_parts), chain_id));
    const five_parts = rlp.list(&.{ rlp.list(header_items), rlp.list(&.{}), rlp.list(&.{}), rlp.list(&.{}), rlp.list(&.{}) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, five_parts), chain_id));
}

/// The block whose one withdrawal has the fields `fields`.
fn blockWithWithdrawal(a: Allocator, fields: []const Item) ![]u8 {
    return rlp.encode(a, try blocks.blockItem(a, try blocks.headerItems(a, .{}), &.{}, &.{}, &.{rlp.list(fields)}));
}

test "a withdrawal's fields are held in the widths of their types" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const address = rlp.string(lit("c3" ** 20));
    const largest = [_]Item{ try rlp.int(a, std.math.maxInt(u64)), try rlp.int(a, std.math.maxInt(u64)), address, try rlp.int(a, std.math.maxInt(u256)) };
    _ = try truncate(a, try blockWithWithdrawal(a, &largest), chain_id);

    const nine_bytes = [_]u8{1} ** 9;
    const short_address = [_]u8{0xc3} ** 19;
    const long_address = [_]u8{0xc3} ** 21;
    const wide_amount = [_]u8{1} ** 33;
    const cases = [_]struct { index: usize, replacement: Item }{
        .{ .index = 0, .replacement = rlp.string(&nine_bytes) }, // index
        .{ .index = 1, .replacement = rlp.string(&nine_bytes) }, // validator index
        .{ .index = 2, .replacement = rlp.string(&short_address) },
        .{ .index = 2, .replacement = rlp.string(&long_address) },
        .{ .index = 3, .replacement = rlp.string(&wide_amount) }, // amount
    };
    for (cases) |case| {
        var fields = largest;
        fields[case.index] = case.replacement;
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithWithdrawal(a, &fields), chain_id));
    }
}

test "malformed transactions are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(45);

    // A transaction type beyond 4, the zero type and an envelope too short to hold a body.
    for ([_][]const u8{ lit("05c0"), lit("00c0"), lit("01"), lit("ff00") }) |envelope| {
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(envelope)}), chain_id));
    }

    // A typed transaction body with a missing field.
    const signed_item = try blocks.signed(a, fee_market_tx, chain_id, secret);
    const body = signed_item.string[1..];
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(try std.mem.concat(a, u8, &.{ lit("02"), body[0 .. body.len - 1] }))}), chain_id));

    // A set-code transaction has a destination.
    var without_destination = set_code_tx;
    without_destination.to = null;
    const without_destination_block = try blockOf(a, &.{try blocks.signed(a, without_destination, chain_id, secret)});
    try std.testing.expectError(error.MalformedBlock, truncate(a, without_destination_block, chain_id));
    // Contract creation is for the other types.
    var creation = fee_market_tx;
    creation.to = null;
    const created = try truncate(a, try blockOf(a, &.{try blocks.signed(a, creation, chain_id, secret)}), chain_id);
    try std.testing.expectEqualSlices(u8, try blocks.stripped(a, creation), created.transactions[0]);
}

test "a heap too small for a valid block is reported as exhausted whichever step runs out" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    var items: [vectors.len]Item = undefined;
    for (vectors, &items, 0..) |vector, *item, i| item.* = try blocks.signed(a, vector.tx, chain_id, secretKey(@intCast(70 + i)));
    const block = try blockOf(a, &items);

    var buffer: [1 << 20]u8 = undefined;
    var size: usize = 0;
    const truncated = while (size <= buffer.len) : (size += 16) {
        var heap = std.heap.FixedBufferAllocator.init(buffer[0..size]);
        break truncation.truncateBlockRlp(a, heap.allocator(), block, chain_id) catch |err| {
            try std.testing.expect(err == error.OutOfMemory);
            continue;
        };
    } else return error.TestUnexpectedResult;
    try std.testing.expect(size > 0);
    try std.testing.expectEqual(vectors.len, truncated.transactions.len);
}

// Fields wider than the integers they are held in.

const Field = enum { nonce, gas_limit, gas_price, max_priority_fee_per_gas };

fn withField(tx: blocks.Tx, field: Field, value: u256) blocks.Tx {
    var out = tx;
    switch (field) {
        .nonce => out.nonce = value,
        .gas_limit => out.gas_limit = value,
        .gas_price => out.gas_price = value,
        .max_priority_fee_per_gas => out.max_priority_fee_per_gas = value,
    }
    return out;
}

const max_u64: u256 = std.math.maxInt(u64);
const max_u128: u256 = std.math.maxInt(u128);

/// A field of a transaction with the largest value the guest decodes for it.
const bounded_fields = [_]struct { tx: blocks.Tx, field: Field, largest: u256 }{
    .{ .tx = legacy_tx, .field = .nonce, .largest = max_u64 },
    .{ .tx = legacy_tx, .field = .gas_limit, .largest = max_u64 },
    .{ .tx = legacy_tx, .field = .gas_price, .largest = max_u128 },
    .{ .tx = access_list_tx, .field = .nonce, .largest = max_u64 },
    .{ .tx = access_list_tx, .field = .gas_limit, .largest = max_u64 },
    .{ .tx = access_list_tx, .field = .gas_price, .largest = max_u128 },
    .{ .tx = fee_market_tx, .field = .nonce, .largest = max_u64 },
    .{ .tx = fee_market_tx, .field = .gas_limit, .largest = max_u64 },
    .{ .tx = fee_market_tx, .field = .gas_price, .largest = max_u128 },
    .{ .tx = fee_market_tx, .field = .max_priority_fee_per_gas, .largest = max_u128 },
    .{ .tx = set_code_tx, .field = .nonce, .largest = max_u64 },
    .{ .tx = set_code_tx, .field = .gas_limit, .largest = max_u64 },
    .{ .tx = set_code_tx, .field = .gas_price, .largest = max_u128 },
    .{ .tx = set_code_tx, .field = .max_priority_fee_per_gas, .largest = max_u128 },
};

test "the largest nonce, gas and fees the guest decodes are accepted with their stripped form" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(60);

    for (bounded_fields) |case| {
        const tx = withField(case.tx, case.field, case.largest);
        const truncated = try truncate(a, try blockOf(a, &.{try blocks.signed(a, tx, chain_id, secret)}), chain_id);
        try std.testing.expectEqualSlices(u8, try blocks.stripped(a, tx), truncated.transactions[0]);
        try std.testing.expectEqualSlices(u8, &secp256k1.addressOf(secret), &truncated.froms[0]);
    }
}

test "a field one bit wider than the guest decodes is rejected, whatever the signature covers" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(61);

    for (bounded_fields) |case| {
        const widest = withField(case.tx, case.field, case.largest);
        const wider = withField(case.tx, case.field, case.largest + 1);
        // Signed over its own fields.
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{try blocks.signed(a, wider, chain_id, secret)}), chain_id));
        // Signed over the largest value the field can hold, which a clamping decoder would take it for.
        const lookalike = try presentedWithSignatureOf(a, widest, wider, secret);
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{lookalike}), chain_id));
    }
}

test "a blob transaction is rejected wherever it stands in the block" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(62);

    const accepted = try blocks.signed(a, fee_market_tx, chain_id, secret);
    const blob = try blocks.signed(a, blob_tx, chain_id, secret);
    try std.testing.expectEqual(1, (try truncate(a, try blockOf(a, &.{accepted}), chain_id)).transactions.len);

    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{blob}), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{ accepted, blob }), chain_id));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{ blob, accepted }), chain_id));

    var prng = std.Random.DefaultPrng.init(2029);
    for (0..20) |_| {
        const random_blob = try blocks.randomTx(a, prng.random(), .blob);
        const item = try blocks.signed(a, random_blob, chain_id, secret);
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{ accepted, item }), chain_id));
    }
}

test "a transaction that is not in its canonical wire form is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(63);

    // A legacy transaction carried as a byte string, and bytes after a typed transaction's body.
    const legacy_bytes = try rlp.encode(a, try blocks.signed(a, legacy_tx, chain_id, secret));
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(legacy_bytes)}), chain_id));
    const typed_envelope = (try blocks.signed(a, fee_market_tx, chain_id, secret)).string;
    const with_trailing = try std.mem.concat(a, u8, &.{ typed_envelope, "junk" });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(with_trailing)}), chain_id));
}

// Blocks whose body is not the one the header commits to.

test "the transactions of a block are the ones its header commits to" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(64);

    var tx_items: [3]Item = undefined;
    for (&tx_items, 0..) |*item, i| {
        var tx = fee_market_tx;
        tx.nonce = i;
        item.* = try blocks.signed(a, tx, chain_id, secret);
    }
    // Another transaction by the same signer, never part of the block.
    var replacement_tx = fee_market_tx;
    replacement_tx.nonce = 1;
    replacement_tx.data = lit("ff");
    const replacement = try blocks.signed(a, replacement_tx, chain_id, secret);

    const committed = [_]Item{ tx_items[0], tx_items[1] };
    const header = try blocks.withTransactions(a, .{}, &committed);
    const accepted = try truncate(a, try blocks.block(a, header, &committed, &.{}), chain_id);
    try std.testing.expectEqual(2, accepted.transactions.len);

    const bodies = [_][]const Item{
        &.{ tx_items[0], replacement },
        &.{ tx_items[1], tx_items[0] },
        &.{tx_items[0]},
        &.{ tx_items[0], tx_items[1], tx_items[2] },
        &.{},
    };
    for (bodies) |body| {
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blocks.block(a, header, body, &.{}), chain_id));
    }
    // A header with no transactions commits to the empty list only.
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blocks.block(a, .{}, &committed, &.{}), chain_id));
}

// The chain ID.

test "a chain ID of zero is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(65);

    // Under chain ID 0 an EIP-155 signature would commit to the chain ID in its pre-image.
    const blocks_under_zero = [_][]const u8{
        try blockOf(a, &.{try blocks.signed(a, legacy_tx, 0, secret)}),
        try blockOf(a, &.{try blocks.signedUnprotected(a, legacy_tx, secret)}),
        try blockOf(a, &.{try blocks.signed(a, fee_market_tx, 0, secret)}),
        try blockOf(a, &.{}),
    };
    for (blocks_under_zero) |block| {
        try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, block, 0));
    }
}

// Declared lengths that wrap around.

/// An item whose long-form header declares a payload length of `maxInt(u64) - 8 + excess`: added to
/// the header, the length wraps around to a small number.
fn wrappingLength(first: u8, excess: u3) [9]u8 {
    var item: [9]u8 = undefined;
    item[0] = first;
    std.mem.writeInt(u64, item[1..9], std.math.maxInt(u64) - 8 + @as(u64, excess), .big);
    return item;
}

test "an item declaring a length that wraps around is rejected wherever it occurs" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(66);

    // The block itself, a header field, a transaction entry, a legacy transaction's data field and
    // an access-list entry of a typed transaction.
    const legacy_fields = (try blocks.signed(a, legacy_tx, chain_id, secret)).list;
    for ([_]u8{ 0xbf, 0xff }) |first| {
        for (0..8) |excess| {
            const wrapping = wrappingLength(first, @intCast(excess));
            const item = rlp.raw(&wrapping);

            try std.testing.expectError(error.MalformedBlock, truncate(a, &wrapping, chain_id));
            try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, 8, item)), chain_id));
            try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{item}), chain_id));

            const nested = try a.dupe(Item, legacy_fields);
            nested[5] = item;
            try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.list(nested)}), chain_id));

            const typed_fields = [_]Item{
                try rlp.int(a, 1),
                try rlp.int(a, 0),
                try rlp.int(a, 1),
                try rlp.int(a, 2),
                try rlp.int(a, 21_000),
                rlp.string(lit("11" ** 20)),
                try rlp.int(a, 0),
                rlp.string(&.{}),
                rlp.list(&.{item}),
            };
            const envelope = try std.mem.concat(a, u8, &.{ lit("02"), try rlp.encode(a, rlp.list(&typed_fields)) });
            try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(envelope)}), chain_id));
        }
    }
}

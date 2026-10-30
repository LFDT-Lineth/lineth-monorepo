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

const chain_id = 59144;

/// The one-transaction block of `item` under the default header.
fn blockOf(arena: Allocator, transactions: []const Item) ![]u8 {
    return blocks.block(arena, .{}, transactions, &.{});
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
const blob_stripped = hex("03" ++ // type
    "f845" ++ // list of 69 bytes
    "03" ++ // nonce
    "01" ++ // max priority fee
    "64" ++ // max fee 100
    "830186a0" ++ // gas 100000
    "94" ++ "55" ** 20 ++ // to
    "80" ++ // value 0
    "80" ++ // data
    "c0" ++ // empty access list
    "830f4240" ++ // max fee per blob gas 1000000
    "e1" ++ "a0" ++ "01" ++ "66" ** 31); // one blob versioned hash

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
    .{ .tx = blob_tx, .stripped = &blob_stripped },
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

test "a transaction's sender is bound to its signed content" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(41);

    // The signature of one transaction on another: whoever recovers is not the signer.
    for ([_]blocks.Tx{ legacy_tx, fee_market_tx }) |tx| {
        const digest = try blocks.signingDigest(a, tx, chain_id, true);
        const signature = secp256k1.sign(secret, digest);
        var other = tx;
        other.nonce += 1;
        const v: u256 = if (tx.kind == .legacy) @as(u256, chain_id) * 2 + 35 + signature.recovery_id else signature.recovery_id;
        const forged = try blocks.withSignature(a, other, chain_id, .{
            .v = v,
            .r = std.mem.readInt(u256, &signature.r, .big),
            .s = std.mem.readInt(u256, &signature.s, .big),
        });
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
    const payload = try truncation.encodeBlocks(gpa, &one_block);
    defer gpa.free(payload);
    try std.testing.expectEqualSlices(u8, &expected, payload);

    const none = try truncation.encodeBlocks(gpa, &.{});
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
    const empty_payload = try truncation.encodeBlocks(gpa, &empty_block);
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
    const payload = try truncation.encodeBlocks(a, &.{truncated});

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
    payload: []const u8,
};

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
    const entry = [_]Item{
        try rlp.int(a, header.timestamp),
        rlp.string(&hash),
        rlp.string(&header.prev_randao),
        rlp.list(transaction_items),
        rlp.list(from_items),
    };
    return .{
        .stripped = stripped,
        .froms = froms,
        .payload = try rlp.encode(a, rlp.list(&.{rlp.list(&entry)})),
    };
}

test "random blocks truncate to what their fields and signers say" {
    var prng = std.Random.DefaultPrng.init(2026);
    const random = prng.random();
    const kinds = std.enums.values(blocks.Kind);

    for (0..60) |_| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        const a = arena.allocator();

        const chain = 1 + (random.int(u64) >> random.uintLessThan(u6, 60));
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

        const truncated = try truncate(a, try blocks.block(a, header, items, &.{}), chain);
        const expected = try expectedOf(a, header, txs, secrets);

        try std.testing.expectEqual(header.timestamp, truncated.timestamp);
        try std.testing.expectEqualSlices(u8, &header.parent_hash, &truncated.parent_hash);
        try std.testing.expectEqualSlices(u8, &header.prev_randao, &truncated.prev_randao);
        try std.testing.expectEqual(count, truncated.transactions.len);
        for (expected.stripped, truncated.transactions) |want, got| try std.testing.expectEqualSlices(u8, want, got);
        for (expected.froms, truncated.froms) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
        try std.testing.expectEqualSlices(u8, expected.payload, try truncation.encodeBlocks(a, &.{truncated}));
    }
}

test "the stripped transactions do not depend on who signed them" {
    var prng = std.Random.DefaultPrng.init(2027);
    const random = prng.random();
    const kinds = std.enums.values(blocks.Kind);

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
    const expected = try expectedOf(a, .{}, txs, secrets);
    for (expected.stripped, truncated.transactions) |want, got| try std.testing.expectEqualSlices(u8, want, got);
    for (expected.froms, truncated.froms) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
    try std.testing.expectEqualSlices(u8, expected.payload, try truncation.encodeBlocks(a, &.{truncated}));
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
    const wide_timestamp = [_]u8{1} ** 9;
    const cases = [_]struct { index: usize, replacement: Item }{
        .{ .index = 0, .replacement = rlp.string(&short_hash) }, // parent hash
        .{ .index = 6, .replacement = rlp.string(&long_bloom) }, // logs bloom
        .{ .index = 11, .replacement = rlp.string(&wide_timestamp) }, // timestamp beyond 64 bits
        .{ .index = 11, .replacement = rlp.list(&.{}) }, // timestamp as a list
        .{ .index = 13, .replacement = rlp.string(&short_hash) }, // prevRandao
        .{ .index = 14, .replacement = rlp.string(&short_nonce) }, // nonce
        .{ .index = 22, .replacement = rlp.string(&wide_timestamp) }, // slot number
    };
    for (cases) |case| {
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockWithHeader(a, try headerWith(a, case.index, case.replacement)), chain_id));
    }
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

    // Withdrawals have four fields.
    const three_fields = rlp.list(&.{ rlp.string(&.{1}), rlp.string(&.{2}), rlp.string(lit("c3" ** 20)) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, try blocks.blockItem(a, header_items, &.{}, &.{}, &.{three_fields})), chain_id));

    // A block has four parts.
    const three_parts = rlp.list(&.{ rlp.list(header_items), rlp.list(&.{}), rlp.list(&.{}) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, three_parts), chain_id));
    const five_parts = rlp.list(&.{ rlp.list(header_items), rlp.list(&.{}), rlp.list(&.{}), rlp.list(&.{}), rlp.list(&.{}) });
    try std.testing.expectError(error.MalformedBlock, truncate(a, try rlp.encode(a, five_parts), chain_id));
}

test "malformed transactions are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const secret = secretKey(45);

    // A transaction type beyond the five, the zero type and an envelope too short to hold a body.
    for ([_][]const u8{ lit("05c0"), lit("00c0"), lit("01"), lit("ff00") }) |envelope| {
        try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(envelope)}), chain_id));
    }

    // A typed transaction body with a missing field.
    const signed_item = try blocks.signed(a, fee_market_tx, chain_id, secret);
    const body = signed_item.string[1..];
    try std.testing.expectError(error.MalformedBlock, truncate(a, try blockOf(a, &.{rlp.string(try std.mem.concat(a, u8, &.{ lit("02"), body[0 .. body.len - 1] }))}), chain_id));

    // Blob and set-code transactions have a destination.
    for ([_]blocks.Tx{ blob_tx, set_code_tx }) |tx| {
        var without_destination = tx;
        without_destination.to = null;
        const block = try blockOf(a, &.{try blocks.signed(a, without_destination, chain_id, secret)});
        try std.testing.expectError(error.MalformedBlock, truncate(a, block, chain_id));
    }
    // Contract creation is for the other types.
    var creation = fee_market_tx;
    creation.to = null;
    const created = try truncate(a, try blockOf(a, &.{try blocks.signed(a, creation, chain_id, secret)}), chain_id);
    try std.testing.expectEqualSlices(u8, try blocks.stripped(a, creation), created.transactions[0]);
}

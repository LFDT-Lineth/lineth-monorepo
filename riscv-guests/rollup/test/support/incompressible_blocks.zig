//! A short chain of blocks in the Osaka header shape whose payload does not compress: each carries
//! one signed EIP-1559 transaction whose calldata is bytes of a seeded PRNG. A header holds the
//! parent hash, transactions root, number, gas limit, timestamp, prevRandao and base fee, and zero
//! in its other fields. The blocks are the same on every run.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const executor = @import("zesu_executor");
const lineth_da = @import("lineth_da");

const Allocator = std.mem.Allocator;
const Fact = ef_blocks.Fact;
const Hash = ef_blocks.Hash;
const Address = ef_blocks.Address;

const rlp_encode = executor.executor_rlp_encode;
const tx_signing = executor.executor_tx_signing;
const truncation = lineth_da.encoding.truncation;
const Ecdsa = std.crypto.sign.ecdsa.Ecdsa(std.crypto.ecc.Secp256k1, std.crypto.hash.sha3.Keccak256);

pub const block_count = 3;
const calldata_len = 100_000;
const prng_seed = 0x1ca11d;
const secret_key: [32]u8 = @splat(0x11);
const recipient: Address = @splat(0x22);
const first_parent_hash: Hash = @splat(0x33);
const first_number: u64 = 1_000;
const first_timestamp: u64 = 1_700_000_000;
const block_interval_seconds: u64 = 2;
const header_gas_limit: u64 = 60_000_000;
const base_fee_per_gas: u64 = 1_000_000_000;
const max_fee_per_gas: u64 = 2 * base_fee_per_gas;
const max_priority_fee_per_gas: u64 = 1_000_000;
const intrinsic_gas: u64 = 21_000;
const gas_per_calldata_byte: u64 = 16;

const eip_1559_type: u8 = 2;
const eip_1559_type_bit: u8 = 1 << eip_1559_type;
const zero_hash: Hash = @splat(0);
const zero_address: Address = @splat(0);
const logs_bloom_len = 256;
const nonce_len = 8;

/// `block_count` chained blocks of chain `chain_id`, each with one transaction of random calldata
/// from the sender of the fixed test key, whose nonces count up from 0.
pub fn build(a: Allocator, chain_id: u64) ![]const Fact {
    const key_pair = try Ecdsa.KeyPair.fromSecretKey(try Ecdsa.SecretKey.fromBytes(secret_key));
    var prng = std.Random.DefaultPrng.init(prng_seed);

    const blocks = try a.alloc(Fact, block_count);
    var parent_hash = first_parent_hash;
    for (blocks, 0..) |*block, index| {
        const data = try a.alloc(u8, calldata_len);
        prng.random().bytes(data);
        var prev_randao: Hash = undefined;
        prng.random().bytes(&prev_randao);
        block.* = try blockOf(a, chain_id, key_pair, .{
            .number = first_number + index,
            .timestamp = first_timestamp + block_interval_seconds * index,
            .parent_hash = parent_hash,
            .prev_randao = prev_randao,
            .nonce = index,
            .data = data,
        });
        parent_hash = block.hash;
    }
    return blocks;
}

const BlockSpec = struct {
    number: u64,
    timestamp: u64,
    parent_hash: Hash,
    prev_randao: Hash,
    nonce: u64,
    data: []const u8,
};

fn blockOf(a: Allocator, chain_id: u64, key_pair: Ecdsa.KeyPair, spec: BlockSpec) !Fact {
    const sender = addressOf(key_pair.public_key);
    const transaction: ef_blocks.Transaction = .{
        .type = eip_1559_type,
        .nonce = spec.nonce,
        .gas_price = max_fee_per_gas,
        .max_priority_fee_per_gas = max_priority_fee_per_gas,
        .gas_limit = intrinsic_gas + gas_per_calldata_byte * spec.data.len,
        .to = recipient,
        .value = 0,
        .data = spec.data,
        .access_list = &.{},
        .authorization_list = &.{},
    };
    const fields = try transactionFields(a, chain_id, transaction);
    const signature = try sign(key_pair, try signingPayload(a, fields));

    // The y-parity is the one under which the block's own sender recovery returns the key's address.
    var scratch = std.heap.ArenaAllocator.init(a);
    defer scratch.deinit();
    for ([_]u1{ 0, 1 }) |y_parity| {
        const envelope = try signedEnvelope(a, fields, y_parity, signature);
        const block_rlp = try blockRlp(a, spec, envelope);
        const truncated = truncation.truncateBlockRlp(a, scratch.allocator(), block_rlp, chain_id) catch continue;
        if (!std.mem.eql(u8, &truncated.froms[0], &sender)) continue;
        const transactions = try a.dupe(ef_blocks.Transaction, &.{transaction});
        return .{
            .rlp = block_rlp,
            .hash = truncated.block_hash,
            .parent_hash = spec.parent_hash,
            .prev_randao = spec.prev_randao,
            .number = spec.number,
            .timestamp = spec.timestamp,
            .transactions = transactions,
            .senders = try a.dupe(Address, &.{sender}),
            .blob_transaction = false,
            .profile = .{
                .transactions = 1,
                .types = eip_1559_type_bit,
                .max_data_len = spec.data.len,
                .access_list = false,
                .authorization_list = false,
            },
        };
    }
    return error.SenderNotRecovered;
}

fn addressOf(public_key: Ecdsa.PublicKey) Address {
    const sec1 = public_key.toUncompressedSec1();
    var digest: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(sec1[1..], &digest, .{});
    return digest[digest.len - @sizeOf(Address) ..].*;
}

/// The items of the signing payload, which the signed transaction's items follow with the signature.
fn transactionFields(a: Allocator, chain_id: u64, transaction: ef_blocks.Transaction) ![]const []const u8 {
    const empty_access_list = try rlp_encode.encodeList(a, &.{});
    return a.dupe([]const u8, &.{
        try rlp_encode.encodeU64(a, chain_id),
        try rlp_encode.encodeU64(a, transaction.nonce),
        try rlp_encode.encodeU256(a, transaction.max_priority_fee_per_gas.?),
        try rlp_encode.encodeU256(a, transaction.gas_price),
        try rlp_encode.encodeU64(a, transaction.gas_limit),
        try rlp_encode.encodeBytes(a, &transaction.to.?),
        try rlp_encode.encodeU256(a, transaction.value),
        try rlp_encode.encodeBytes(a, transaction.data),
        empty_access_list,
    });
}

fn signingPayload(a: Allocator, fields: []const []const u8) ![]u8 {
    return std.mem.concat(a, u8, &.{ &.{eip_1559_type}, try rlp_encode.encodeList(a, fields) });
}

/// The signature of `payload` with `s` in the low half of the group order (EIP-2).
fn sign(key_pair: Ecdsa.KeyPair, payload: []const u8) !Ecdsa.Signature {
    var signature = try key_pair.sign(payload, null);
    const s = std.mem.readInt(u256, &signature.s, .big);
    if (s > tx_signing.SECP256K1N_OVER_2) std.mem.writeInt(u256, &signature.s, tx_signing.SECP256K1N - s, .big);
    return signature;
}

fn signedEnvelope(a: Allocator, fields: []const []const u8, y_parity: u1, signature: Ecdsa.Signature) ![]u8 {
    const signed = try std.mem.concat(a, []const u8, &.{ fields, &.{
        try rlp_encode.encodeU64(a, y_parity),
        try rlp_encode.encodeU256(a, std.mem.readInt(u256, &signature.r, .big)),
        try rlp_encode.encodeU256(a, std.mem.readInt(u256, &signature.s, .big)),
    } });
    return std.mem.concat(a, u8, &.{ &.{eip_1559_type}, try rlp_encode.encodeList(a, signed) });
}

fn blockRlp(a: Allocator, spec: BlockSpec, envelope: []const u8) ![]u8 {
    const transactions_root = try executor.computeRawTxRoot(a, &.{envelope});
    const header = try rlp_encode.encodeList(a, &.{
        try rlp_encode.encodeBytes(a, &spec.parent_hash),
        try rlp_encode.encodeBytes(a, &zero_hash), // ommers hash
        try rlp_encode.encodeBytes(a, &zero_address), // coinbase
        try rlp_encode.encodeBytes(a, &zero_hash), // state root
        try rlp_encode.encodeBytes(a, &transactions_root),
        try rlp_encode.encodeBytes(a, &zero_hash), // receipts root
        try rlp_encode.encodeBytes(a, &([_]u8{0} ** logs_bloom_len)),
        try rlp_encode.encodeU64(a, 0), // difficulty
        try rlp_encode.encodeU64(a, spec.number),
        try rlp_encode.encodeU64(a, header_gas_limit),
        try rlp_encode.encodeU64(a, 0), // gas used
        try rlp_encode.encodeU64(a, spec.timestamp),
        try rlp_encode.encodeBytes(a, &.{}), // extra data
        try rlp_encode.encodeBytes(a, &spec.prev_randao),
        try rlp_encode.encodeBytes(a, &([_]u8{0} ** nonce_len)),
        try rlp_encode.encodeU64(a, base_fee_per_gas),
        try rlp_encode.encodeBytes(a, &zero_hash), // withdrawals root
        try rlp_encode.encodeU64(a, 0), // blob gas used
        try rlp_encode.encodeU64(a, 0), // excess blob gas
        try rlp_encode.encodeBytes(a, &zero_hash), // parent beacon block root
        try rlp_encode.encodeBytes(a, &zero_hash), // requests hash
    });
    const transactions = try rlp_encode.encodeList(a, &.{try rlp_encode.encodeBytes(a, envelope)});
    const empty_list = try rlp_encode.encodeList(a, &.{});
    return rlp_encode.encodeList(a, &.{ header, transactions, empty_list, empty_list });
}

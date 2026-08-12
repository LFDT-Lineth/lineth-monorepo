//! Shared transaction-fixture builders for tests.
//!
//! `buildLegacyTxRlp` is a pure RLP encoder for a legacy (type-0) transaction: the caller supplies
//! `v`/`r`/`s` directly, so it serves equally as an unsigned EIP-155 preimage builder (`v=chainId,
//! r=s=0`) and as the final signed-tx encoder (`v` the raw wire value). `buildSignedLegacyTx`,
//! `buildSignedEip1559Tx`, `buildSignedBlobTx`, and `buildSignedEip7702Tx` build on top of it (and
//! their own typed-tx payload encoders) to produce a genuinely secp256k1-signed, sender-recoverable
//! transaction from named fields plus a deterministic per-label private key (`fixturePrivateKey`)
//! — the same label always reproduces the same key, and libsecp256k1 signs with RFC-6979
//! deterministic nonces, so the same (label, fields) pair always reproduces the same signature
//! bytes, run to run.
//!
//! Typed-tx (EIP-1559/EIP-4844/EIP-7702) field order and signature-field handling mirror the
//! vendored decoder's typed-transaction branches exactly: chainId/nonce/fees/gas/to/value/data/
//! accessList (plus blob-specific maxFeePerBlobGas/blobVersionedHashes for type 3, or an
//! authorizationList for type 4, left empty here since these fixtures only need `tx.type == 4` for
//! dispatch, not real authorization content), then a bare `y_parity` (0 or 1 — the legacy tx
//! instead folds chain id and parity together into its wire `v`) followed by `r`/`s`. Signing
//! preimages drop the signature fields entirely; the legacy tx's EIP-155 preimage takes the other
//! approach, reusing the signed tx's own 9-field shape with `v=chainId, r=s=0`.

const std = @import("std");
const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");
const secp256k1 = @import("zesu_secp256k1");

const rlp = executor.executor_rlp_encode;

/// RLP-encodes a legacy transaction as `[nonce, gasPrice, gasLimit, to, value, data, v, r, s]`,
/// matching the field order the decoder's legacy branch expects. `to = null` encodes as an empty
/// RLP string (contract creation). `v` is the raw wire-format value, not y_parity: pass
/// `chain_id*2 + 35 + recid` for a signed EIP-155 transaction, `27 + recid` for a signed
/// pre-EIP-155 one, or the bare `chain_id` with `r = s = 0` for the EIP-155 signing preimage (a
/// zero `u256` RLP-encodes as an empty string, the preimage's `""` fields).
pub fn buildLegacyTxRlp(
    alloc: std.mem.Allocator,
    nonce: u64,
    gas_price: u128,
    gas_limit: u64,
    to: ?[20]u8,
    value: u256,
    data: []const u8,
    v: u256,
    r: u256,
    s: u256,
) ![]const u8 {
    const to_encoded = if (to) |addr| try rlp.encodeBytes(alloc, &addr) else try rlp.encodeBytes(alloc, &.{});
    const items = [_][]const u8{
        try rlp.encodeU64(alloc, nonce),
        try rlp.encodeU128(alloc, gas_price),
        try rlp.encodeU64(alloc, gas_limit),
        to_encoded,
        try rlp.encodeU256(alloc, value),
        try rlp.encodeBytes(alloc, data),
        try rlp.encodeU256(alloc, v),
        try rlp.encodeU256(alloc, r),
        try rlp.encodeU256(alloc, s),
    };
    return rlp.encodeList(alloc, &items);
}

/// Deterministic per-label private key: the same label always signs with the same key, and — via
/// libsecp256k1's RFC-6979 deterministic nonces — always produces the same signature bytes for the
/// same message, run to run.
pub fn fixturePrivateKey(comptime label: []const u8) [32]u8 {
    return mpt.keccak256("l2exec-range-fixture/" ++ label);
}

/// A derived ECDSA signature in the shape every builder below needs to re-encode a signed
/// transaction: `y_parity` (0 or 1) for typed txs, folded into the legacy `v` convention by
/// `buildSignedLegacyTx` itself.
const DerivedSignature = struct { y_parity: u64, r: u256, s: u256 };

/// Signs `msg_hash` with `private_key` via zesu's real secp256k1 backend, propagating context/sign
/// failures as errors rather than a bare `null`.
fn signHash(msg_hash: [32]u8, private_key: [32]u8) !DerivedSignature {
    const ctx = secp256k1.getContext() orelse return error.Secp256k1ContextUnavailable;
    const signature = ctx.sign(msg_hash, private_key) orelse return error.FixtureTxSigningFailed;
    return .{
        .r = std.mem.readInt(u256, signature.sig[0..32], .big),
        .s = std.mem.readInt(u256, signature.sig[32..64], .big),
        .y_parity = @as(u64, signature.recid),
    };
}

pub const LegacyTxArgs = struct {
    nonce: u64,
    gas_price: u128,
    gas: u64,
    /// `null` signs a contract-creation transaction.
    to: ?[20]u8,
    value: u256,
    data: []const u8 = &.{},
    chain_id: u64,
};

/// Builds and signs a real legacy (type-0) transaction: RLP-encodes the unsigned EIP-155 preimage
/// (`buildLegacyTxRlp` with `v=chainId, r=s=0`), hashes it, signs with the label's deterministic
/// private key, then re-encodes with the derived `v = chainId*2 + 35 + recid`.
pub fn buildSignedLegacyTx(alloc: std.mem.Allocator, comptime label: []const u8, args: LegacyTxArgs) ![]const u8 {
    const unsigned_rlp = try buildLegacyTxRlp(alloc, args.nonce, args.gas_price, args.gas, args.to, args.value, args.data, args.chain_id, 0, 0);
    const sig = try signHash(mpt.keccak256(unsigned_rlp), fixturePrivateKey(label));
    const v: u256 = @as(u256, args.chain_id) * 2 + 35 + @as(u256, sig.y_parity);
    return buildLegacyTxRlp(alloc, args.nonce, args.gas_price, args.gas, args.to, args.value, args.data, v, sig.r, sig.s);
}

/// RLP-encodes the `to` field the way every typed-tx branch below expects: an address, or an
/// empty RLP string for contract creation.
fn rlpToField(alloc: std.mem.Allocator, to: ?[20]u8) ![]const u8 {
    if (to) |addr| return rlp.encodeBytes(alloc, &addr);
    return rlp.encodeBytes(alloc, &.{});
}

fn emptyAccessListRlp(alloc: std.mem.Allocator) ![]const u8 {
    return rlp.encodeList(alloc, &.{});
}

pub const Eip1559TxArgs = struct {
    nonce: u64,
    max_priority_fee: u128,
    max_fee_per_gas: u128,
    gas: u64,
    to: ?[20]u8,
    value: u256,
    data: []const u8 = &.{},
    chain_id: u64,
};

/// RLP-encodes a type-0x02 (EIP-1559) payload: the type byte followed by
/// `rlp([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data,
/// accessList])`, plus `y_parity, r, s` appended when `signature` is given. Mirrors the decoder's
/// type-2 branch field-for-field; `signature = null` yields the unsigned signing preimage (9
/// fields, no signature slots at all).
fn buildEip1559Payload(alloc: std.mem.Allocator, args: Eip1559TxArgs, signature: ?DerivedSignature) ![]const u8 {
    var items: [12][]const u8 = undefined;
    items[0] = try rlp.encodeU64(alloc, args.chain_id);
    items[1] = try rlp.encodeU64(alloc, args.nonce);
    items[2] = try rlp.encodeU128(alloc, args.max_priority_fee);
    items[3] = try rlp.encodeU128(alloc, args.max_fee_per_gas);
    items[4] = try rlp.encodeU64(alloc, args.gas);
    items[5] = try rlpToField(alloc, args.to);
    items[6] = try rlp.encodeU256(alloc, args.value);
    items[7] = try rlp.encodeBytes(alloc, args.data);
    items[8] = try emptyAccessListRlp(alloc);
    var n: usize = 9;
    if (signature) |sig| {
        items[9] = try rlp.encodeU64(alloc, sig.y_parity);
        items[10] = try rlp.encodeU256(alloc, sig.r);
        items[11] = try rlp.encodeU256(alloc, sig.s);
        n = 12;
    }
    return rlp.concat(alloc, &.{ &.{0x02}, try rlp.encodeList(alloc, items[0..n]) });
}

/// Builds and signs a real EIP-1559 transaction: hashes `buildEip1559Payload`'s unsigned preimage,
/// signs with the label's deterministic private key, then re-encodes with the derived signature.
pub fn buildSignedEip1559Tx(alloc: std.mem.Allocator, comptime label: []const u8, args: Eip1559TxArgs) ![]const u8 {
    const preimage = try buildEip1559Payload(alloc, args, null);
    const sig = try signHash(mpt.keccak256(preimage), fixturePrivateKey(label));
    return buildEip1559Payload(alloc, args, sig);
}

pub const BlobTxArgs = struct {
    nonce: u64,
    max_priority_fee: u128,
    max_fee_per_gas: u128,
    gas: u64,
    /// Blob transactions require a real recipient (EIP-4844); legacy/EIP-1559 fixtures instead
    /// take an optional `to` for contract creation.
    to: [20]u8,
    value: u256,
    data: []const u8 = &.{},
    chain_id: u64,
    max_fee_per_blob_gas: u128,
    versioned_hashes: []const [32]u8,
};

fn versionedHashesRlp(alloc: std.mem.Allocator, hashes: []const [32]u8) ![]const u8 {
    const items = try alloc.alloc([]const u8, hashes.len);
    for (hashes, 0..) |h, i| items[i] = try rlp.encodeBytes(alloc, &h);
    return rlp.encodeList(alloc, items);
}

/// RLP-encodes a type-0x03 (EIP-4844 blob) payload: the type byte followed by
/// `rlp([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList,
/// maxFeePerBlobGas, blobVersionedHashes])`, plus `y_parity, r, s` appended when `signature` is
/// given. Mirrors the decoder's type-3 branch field-for-field.
fn buildBlobTxPayload(alloc: std.mem.Allocator, args: BlobTxArgs, signature: ?DerivedSignature) ![]const u8 {
    var items: [14][]const u8 = undefined;
    items[0] = try rlp.encodeU64(alloc, args.chain_id);
    items[1] = try rlp.encodeU64(alloc, args.nonce);
    items[2] = try rlp.encodeU128(alloc, args.max_priority_fee);
    items[3] = try rlp.encodeU128(alloc, args.max_fee_per_gas);
    items[4] = try rlp.encodeU64(alloc, args.gas);
    items[5] = try rlp.encodeBytes(alloc, &args.to);
    items[6] = try rlp.encodeU256(alloc, args.value);
    items[7] = try rlp.encodeBytes(alloc, args.data);
    items[8] = try emptyAccessListRlp(alloc);
    items[9] = try rlp.encodeU128(alloc, args.max_fee_per_blob_gas);
    items[10] = try versionedHashesRlp(alloc, args.versioned_hashes);
    var n: usize = 11;
    if (signature) |sig| {
        items[11] = try rlp.encodeU64(alloc, sig.y_parity);
        items[12] = try rlp.encodeU256(alloc, sig.r);
        items[13] = try rlp.encodeU256(alloc, sig.s);
        n = 14;
    }
    return rlp.concat(alloc, &.{ &.{0x03}, try rlp.encodeList(alloc, items[0..n]) });
}

/// Builds and signs a real EIP-4844 blob transaction: hashes `buildBlobTxPayload`'s unsigned
/// preimage, signs with the label's deterministic private key, then re-encodes with the derived
/// signature.
pub fn buildSignedBlobTx(alloc: std.mem.Allocator, comptime label: []const u8, args: BlobTxArgs) ![]const u8 {
    const preimage = try buildBlobTxPayload(alloc, args, null);
    const sig = try signHash(mpt.keccak256(preimage), fixturePrivateKey(label));
    return buildBlobTxPayload(alloc, args, sig);
}

pub const Eip7702TxArgs = struct {
    nonce: u64,
    max_priority_fee: u128,
    max_fee_per_gas: u128,
    gas: u64,
    /// EIP-7702 requires a real recipient (no contract-creation form), like blob transactions.
    to: [20]u8,
    value: u256,
    data: []const u8 = &.{},
    chain_id: u64,
};

/// RLP-encodes a type-0x04 (EIP-7702) payload: the type byte followed by
/// `rlp([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList,
/// authorizationList])`, plus `y_parity, r, s` appended when `signature` is given. Mirrors the
/// decoder's type-4 branch field-for-field; the authorization list is always empty here (RLP-encoded
/// the same way `emptyAccessListRlp` encodes its own empty list), since these fixtures only need
/// `tx.type == 4` to reach the guest's type-4 dispatch, not real authorization content.
fn buildEip7702Payload(alloc: std.mem.Allocator, args: Eip7702TxArgs, signature: ?DerivedSignature) ![]const u8 {
    var items: [13][]const u8 = undefined;
    items[0] = try rlp.encodeU64(alloc, args.chain_id);
    items[1] = try rlp.encodeU64(alloc, args.nonce);
    items[2] = try rlp.encodeU128(alloc, args.max_priority_fee);
    items[3] = try rlp.encodeU128(alloc, args.max_fee_per_gas);
    items[4] = try rlp.encodeU64(alloc, args.gas);
    items[5] = try rlp.encodeBytes(alloc, &args.to);
    items[6] = try rlp.encodeU256(alloc, args.value);
    items[7] = try rlp.encodeBytes(alloc, args.data);
    items[8] = try emptyAccessListRlp(alloc);
    items[9] = try emptyAccessListRlp(alloc);
    var n: usize = 10;
    if (signature) |sig| {
        items[10] = try rlp.encodeU64(alloc, sig.y_parity);
        items[11] = try rlp.encodeU256(alloc, sig.r);
        items[12] = try rlp.encodeU256(alloc, sig.s);
        n = 13;
    }
    return rlp.concat(alloc, &.{ &.{0x04}, try rlp.encodeList(alloc, items[0..n]) });
}

/// Builds and signs a real EIP-7702 transaction: hashes `buildEip7702Payload`'s unsigned preimage,
/// signs with the label's deterministic private key, then re-encodes with the derived signature.
pub fn buildSignedEip7702Tx(alloc: std.mem.Allocator, comptime label: []const u8, args: Eip7702TxArgs) ![]const u8 {
    const preimage = try buildEip7702Payload(alloc, args, null);
    const sig = try signHash(mpt.keccak256(preimage), fixturePrivateKey(label));
    return buildEip7702Payload(alloc, args, sig);
}

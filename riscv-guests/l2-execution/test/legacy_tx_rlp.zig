//! Shared legacy (type-0) transaction RLP encoder for test fixtures.

const std = @import("std");
const executor = @import("zesu_executor");

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

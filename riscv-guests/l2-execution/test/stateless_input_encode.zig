//! Test-only SSZ encoder for the vanilla `SszStatelessInput` (Amsterdam stateless block execution) —
//! the exact byte-level inverse of the decoder this package's guest consumes. Every wire-shape type
//! below mirrors that decoder's corresponding section, in the same order, so a schema change is a
//! side-by-side edit. This exists purely so tests can build a `StatelessInput` as readable, diffable
//! Zig and turn it into the bytes the guest's real decode path accepts — the vendored decoder ships
//! with no matching encoder of its own.
//!
//! Each wire container below is a plain Zig struct — fixed fields as arrays/ints, variable fields as
//! slices — serialized by `ssz.serialize`'s generic comptime reflection over the struct's fields in
//! declaration order: the same 4-byte little-endian offset-table convention SSZ and the decoder both
//! use, so declaring a field as a fixed array vs. a slice is itself what selects "inline in the fixed
//! head" vs. "offset into the variable region". The wire's byte-list convention needs a specific Zig
//! shape to come out right:
//!   - a `List[ByteList[N], M]` (a list of variable-length byte blobs — transactions/witness-nodes/
//!     codes/headers) is a slice of byte slices (`[]const []const u8`): the outer list is variable
//!     (gets an offset table), each inner blob is raw bytes (`@sizeOf(u8) == 1`, so the library packs
//!     it with no per-item framing of its own).
//! Container layouts match the decoder exactly (fixed region sizes):
//!   SszStatelessInput:    20 bytes  [4+4+8+4]
//!   SszNewPayloadRequest: 44 bytes  [4+4+32+4]
//!   SszExecutionPayload: 540 bytes  (Amsterdam/V4 shape)
//!   SszExecutionWitness:  12 bytes  [4+4+4]
//!   SszWithdrawal:        44 bytes  fixed (8+8+20+8)
//!
//! Always produces the tightly-packed (canonical, zero-gap) form: every offset points immediately
//! past its own fixed head or the previous variable field, matching the real bytes this package's
//! guest is fed in practice — the natural result of serializing fields in declaration order with no
//! padding, not a property this file arranges by hand. Always produces the Amsterdam (V4) execution-
//! payload shape — this package's guest fixes its fork to Amsterdam, and V4 is the wire shape that
//! fork carries. Output starts directly at the two schema bytes; the Ere length prefix belongs to the
//! optional outer transport framing the decoder strips before this format begins.

const std = @import("std");
const input = @import("zesu_input");
const ssz = @import("ssz");

// ── Wire-shape containers ──────────────────────────────────────────────────────
//
// `input.Withdrawal` and `input.ExecutionWitness` already match their wire shape field-for-field
// (same fields, same order, same types), and `input.ExecutionRequests` already matches
// SszExecutionRequests's 5-slot offset table (deposits, withdrawals, consolidations,
// builder_deposits, builder_exits, in that order) — all three are used directly below with no shadow
// type of their own. The three containers below need a dedicated wire shape because the decoded
// convenience type either orders fields differently than the wire (`NewPayloadRequest`), carries a
// wire-irrelevant field alongside the wire one (`ExecutionPayload`'s decoded `transactions` alongside
// wire `raw_transactions`).

/// SszExecutionPayload's 540-byte fixed region, field-for-field in wire order. `base_fee_per_gas` is
/// a `u256` (the wire's real width) rather than the decoded convenience type's `u64` — only the low 8
/// bytes are ever set, and the library zero-fills the rest of the 32-byte integer. `slot_number` is a
/// plain `u64` rather than `?u64`: it is unconditionally present on the wire, and is optional in the
/// decoded convenience type only pending a genuine zero-value default.
const SszExecutionPayload = struct {
    parent_hash: [32]u8,
    fee_recipient: [20]u8,
    state_root: [32]u8,
    receipts_root: [32]u8,
    logs_bloom: [256]u8,
    prev_randao: [32]u8,
    block_number: u64,
    gas_limit: u64,
    gas_used: u64,
    timestamp: u64,
    extra_data: []const u8,
    base_fee_per_gas: u256,
    block_hash: [32]u8,
    transactions: []const []const u8,
    withdrawals: []const input.Withdrawal,
    blob_gas_used: u64,
    excess_blob_gas: u64,
    block_access_list: []const u8,
    slot_number: u64,
};

/// SszNewPayloadRequest's 44-byte fixed head, in wire order: execution_payload offset, versioned_
/// hashes offset, parent_beacon_block_root inline, execution_requests offset. The decoded convenience
/// type declares `parent_beacon_block_root` before `versioned_hashes`; the wire orders them the other
/// way, so the field order here is what actually selects the wire's layout, not the decoded type's.
const SszNewPayloadRequest = struct {
    execution_payload: SszExecutionPayload,
    versioned_hashes: []const [32]u8,
    parent_beacon_block_root: [32]u8,
    execution_requests: input.ExecutionRequests,
};

/// SszChainConfig's 8-byte fixed region contains only the chain ID. Fork identity travels in the
/// two-byte schema prefix.
const SszChainConfig = struct {
    chain_id: u64,
};

/// SszStatelessInput's 20-byte fixed region: offsets for variable fields and an inline chain ID.
/// `public_keys` is a packed list of fixed 65-byte ByteVectors (uncompressed secp256k1, 0x04 prefix
/// retained) — `[]const [65]u8`, not `[]const []const u8`, so the library packs them with no per-item
/// offset table, matching the wire's "no framing, just concatenation" convention for fixed-size items.
const SszStatelessInput = struct {
    new_payload_request: SszNewPayloadRequest,
    witness: input.ExecutionWitness,
    chain_config: SszChainConfig,
    public_keys: []const [65]u8,
};

const PUBKEY_SIZE: usize = 65;

/// Encode a `StatelessInput` into the SSZ `SszStatelessInput` bytes the decoder accepts. The exact
/// byte-level inverse of `decode`: `decode(alloc, encode(alloc, si))` reproduces `si`.
///
/// `chain_config.fork_name` carries no wire bytes of its own — it is a display string the decoder
/// derives from the schema ID, so encoding reads it for that byte and leaves
/// `fork_name` unread.
pub fn encode(alloc: std.mem.Allocator, si: input.StatelessInput) ![]u8 {
    const ep = si.new_payload_request.execution_payload;

    const public_keys = try alloc.alloc([PUBKEY_SIZE]u8, si.public_keys.len);
    defer alloc.free(public_keys);
    for (si.public_keys, 0..) |key, i| {
        if (key.len != PUBKEY_SIZE) return error.InvalidPublicKeySize;
        @memcpy(&public_keys[i], key);
    }

    const body = SszStatelessInput{
        .new_payload_request = .{
            .execution_payload = .{
                .parent_hash = ep.parent_hash,
                .fee_recipient = ep.fee_recipient,
                .state_root = ep.state_root,
                .receipts_root = ep.receipts_root,
                .logs_bloom = ep.logs_bloom,
                .prev_randao = ep.prev_randao,
                .block_number = ep.block_number,
                .gas_limit = ep.gas_limit,
                .gas_used = ep.gas_used,
                .timestamp = ep.timestamp,
                .extra_data = ep.extra_data,
                .base_fee_per_gas = ep.base_fee_per_gas,
                .block_hash = ep.block_hash,
                // The wire format's transaction list holds opaque RLP bytes, not the decoded
                // `Transaction` struct — `raw_transactions` is the field that round-trips through the
                // wire, exactly like the decoder populates it straight from this same list.
                .transactions = ep.raw_transactions,
                .withdrawals = ep.withdrawals,
                .blob_gas_used = ep.blob_gas_used,
                .excess_blob_gas = ep.excess_blob_gas,
                .block_access_list = ep.block_access_list,
                .slot_number = ep.slot_number orelse 0,
            },
            .versioned_hashes = si.new_payload_request.versioned_hashes,
            .parent_beacon_block_root = si.new_payload_request.parent_beacon_block_root,
            .execution_requests = si.new_payload_request.execution_requests,
        },
        .witness = si.witness,
        .chain_config = .{
            .chain_id = si.chain_config.chain_id,
        },
        .public_keys = public_keys,
    };

    var out: std.ArrayList(u8) = .empty;
    defer out.deinit(alloc);
    // The 2-byte schema ID is
    // Linea's own outer framing, not part of the SSZ container itself — prepended here directly to
    // the same buffer `ssz.serialize` appends the body into, ahead of it.
    try out.append(alloc, @truncate(si.chain_config.schema_id >> 8));
    try out.append(alloc, @truncate(si.chain_config.schema_id));
    try ssz.serialize(SszStatelessInput, body, &out, alloc);

    return out.toOwnedSlice(alloc);
}

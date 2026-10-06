//! Test-only RLP encoder for the EIP-7928 Block Access List wire format that
//! `zesu_input.ExecutionPayload.block_access_list` carries — the byte-level inverse of zesu's own
//! `stateless/executor/bal.zig` `decode` (documented wire shape, reproduced below from reading that
//! file). Needed by `real_multiblock_fixture_gen.zig`: splicing Block A's header (see that file)
//! makes Block B's declared `block_access_list` stale too (Amsterdam's EIP-2935 pre-block system
//! call writes the spliced parent hash into state), and — unlike `state_root`/`receipts_root` —
//! there is no zesu function this tool could call to re-encode a fresh one: `bal.zig` exposes only
//! `decode` and a HASH-only `encodeAndHash`; its actual RLP-producing encoder is private (a good
//! candidate to make public in zesu itself, which would let this whole file be deleted in favor of
//! calling that directly). This mirrors `stateless_input_encode.zig`'s own precedent in this suite
//! (a test-side encoder complementing a zesu module that ships with no matching *public* encoder of
//! its own).
//!
//! Wire shape (one entry per accessed address, address-ascending — exactly the order
//! `executor.buildAccessedEntries` already returns):
//!   outer_list [
//!     entry [
//!       address (20 bytes),
//!       storageChanges [ [slot:compact_u256, [[blockAccessIndex:u64, postValue:u256], ...]], ... ],
//!       storageReads   [ slot:compact_u256, ... ],
//!       balanceChanges [ [blockAccessIndex:u64, postBalance:u256], ... ],
//!       nonceChanges   [ [blockAccessIndex:u64, postNonce:u64], ... ],
//!       codeChanges    [ [blockAccessIndex:u64, postCode:bytes], ... ],
//!     ],
//!     ...
//!   ]
//!
//! Per EIP-7928, `blockAccessIndex` is the changing transaction's 0-based position within the
//! block: a field touched by several transactions carries one change entry per touch, in that
//! order, so a reader can reconstruct the field's value at any point during the block, not just at
//! the end. This encoder only ever emits at most one change entry per field, always at index 0.
//! That is correct rather than a shortcut specifically because `real_multiblock_fixture_gen.zig`
//! only ever calls this for Block B, which has exactly one transaction — index 0 is the only index
//! any change in this block could carry. It is not a general capability of this encoder: the type
//! it actually receives, `AccessedEntry` (zesu's own), already collapses every field to a single
//! pre/post pair for the whole block, discarding which transaction produced the final value — so
//! even a willing caller could not recover per-transaction indices from it for a multi-transaction
//! block. The fixture generator asserts the single-transaction precondition before calling this
//! encoder, so re-pointing it at a different, multi-transaction pair fails loudly there rather than
//! silently producing a plausible-looking but wrong access list here.

const std = @import("std");
const executor = @import("zesu_executor");

const types = executor.executor_types;
const rlp = executor.executor_rlp_encode;

fn u256FromHash(h: [32]u8) u256 {
    return std.mem.readInt(u256, &h, .big);
}

fn encodeEntry(alloc: std.mem.Allocator, entry: types.AccessedEntry) ![]u8 {
    const address_rlp = try rlp.encodeBytes(alloc, &entry.address);

    // storageChanges: at most one [slot, [[0, postValue]]] per changed slot — see this file's
    // header comment for why index 0 is the genuinely correct index here, not a placeholder.
    // `AccessedEntry` already collapses a whole block's writes to that slot down to the single
    // final value, so a single-element inner change list is the faithful encoding of that value.
    var storage_changes_items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, entry.storage_changes.len);
    for (entry.storage_changes) |sc| {
        const slot_rlp = try rlp.encodeU256(alloc, u256FromHash(sc.slot));
        const bai_rlp = try rlp.encodeU64(alloc, 0);
        const value_rlp = try rlp.encodeU256(alloc, sc.post_value);
        const pair_rlp = try rlp.encodeList(alloc, &.{ bai_rlp, value_rlp });
        const changes_list_rlp = try rlp.encodeList(alloc, &.{pair_rlp});
        try storage_changes_items.append(alloc, try rlp.encodeList(alloc, &.{ slot_rlp, changes_list_rlp }));
    }
    const storage_changes_rlp = try rlp.encodeList(alloc, storage_changes_items.items);

    // storageReads: compact-u256-encoded slot per read-but-unchanged slot.
    var storage_reads_items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, entry.storage_reads.len);
    for (entry.storage_reads) |slot| try storage_reads_items.append(alloc, try rlp.encodeU256(alloc, u256FromHash(slot)));
    const storage_reads_rlp = try rlp.encodeList(alloc, storage_reads_items.items);

    const empty_list_rlp = try rlp.encodeList(alloc, &.{});

    // balanceChanges / nonceChanges: empty when the block's own pre/post values agree (nothing
    // changed), one entry at index 0 when they differ — the two states a single-transaction block
    // can produce for a field per EIP-7928.
    const balance_changes_rlp = if (entry.pre_balance != entry.post_balance) blk: {
        const bai_rlp = try rlp.encodeU64(alloc, 0);
        const value_rlp = try rlp.encodeU256(alloc, entry.post_balance);
        const pair_rlp = try rlp.encodeList(alloc, &.{ bai_rlp, value_rlp });
        break :blk try rlp.encodeList(alloc, &.{pair_rlp});
    } else empty_list_rlp;

    const nonce_changes_rlp = if (entry.pre_nonce != entry.post_nonce) blk: {
        const bai_rlp = try rlp.encodeU64(alloc, 0);
        const value_rlp = try rlp.encodeU64(alloc, entry.post_nonce);
        const pair_rlp = try rlp.encodeList(alloc, &.{ bai_rlp, value_rlp });
        break :blk try rlp.encodeList(alloc, &.{pair_rlp});
    } else empty_list_rlp;

    // codeChanges: the wire format needs the actual deployed bytecode, not just its hash —
    // `AccessedEntry` only carries `post_code_hash`. No account in this repo's currently-generated
    // fixture ever has a genuine code change (see `real_multiblock_fixture_gen.zig`'s header
    // comment for why), so that case is reported loudly here rather than silently encoded wrong.
    if (!std.mem.eql(u8, &entry.pre_code_hash, &entry.post_code_hash)) {
        return error.CodeChangeEncodingNotSupported;
    }
    const code_changes_rlp = empty_list_rlp;

    return rlp.encodeList(alloc, &.{
        address_rlp,
        storage_changes_rlp,
        storage_reads_rlp,
        balance_changes_rlp,
        nonce_changes_rlp,
        code_changes_rlp,
    });
}

/// Encode a full Block Access List from already-computed, address-sorted accessed entries — the
/// byte-level inverse of zesu's `bal.zig` `decode`. `entries` MUST already be sorted ascending by
/// address (exactly what `executor.buildAccessedEntries` returns), and MUST come from a
/// single-transaction block (see this file's header comment for why `bai` is always 0).
pub fn encode(alloc: std.mem.Allocator, entries: []const types.AccessedEntry) ![]u8 {
    var items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, entries.len);
    for (entries) |entry| try items.append(alloc, try encodeEntry(alloc, entry));
    return rlp.encodeList(alloc, items.items);
}

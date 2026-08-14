//! Engineers a real, multi-block sequence borrowed from the EF execution-spec-tests zkevm corpus
//! into a self-consistent input for the l2-execution guest's REAL, full pipeline
//! (`l2_execution.runL2Execution`, not `test/conflation_plan.zig`'s `StubEngine`) — computed fresh
//! at test run time by `buildEngineeredInput`, not read from a checked-in file, so it can never go
//! stale relative to the logic that produces it.
//!
//! No existing scenario in this suite drives a genuine multi-block range through the REAL per-block
//! engine (`src/execution.zig`'s `executeStatelessInputWithLogs`) end to end — every scenario in
//! `test/l2_execution_range_test.zig` fakes per-block execution via `StubEngine`. This file closes
//! that gap; `real_multiblock_test.zig` is what actually exercises the input it builds.

const std = @import("std");

const zkevm_fixture = @import("zkevm_fixture.zig");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");
const mpt = @import("zesu_mpt");
const executor = @import("zesu_executor");
const primitives = @import("zesu_primitives");
const input = @import("zesu_input");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const stateless_input_encode = @import("stateless_input_encode");
const vanilla_wrap = @import("vanilla_wrap");
const bal_encode = @import("block_access_list_encode.zig");

const rlp_enc = executor.executor_rlp_encode;

/// The chosen corpus test case's own JSON, embedded straight from the execution-spec-tests
/// dependency tree (wired in `build.zig`, mirroring `evm_execution_fixtures.zig`'s own
/// `zkevm_stateless_block.json` embed) — no committed copy of either the corpus JSON or the
/// engineered SSZ it produces. Four consecutive valid blocks (numbers 1-4), each carrying one real
/// signed transaction, no withdrawals, no EIP-7685 execution requests, and — since the transaction
/// only ever reads `SLOTNUM`/`NUMBER` into storage — no dependency on any block's hash, making
/// every block's witness carry exactly its own immediate parent header (no deeper ancestor chain to
/// keep consistent).
const FIXTURE_JSON = @embedFile("slotnum_distinct_per_block.json");
const TEST_NAME = "tests/amsterdam/eip7843_slotnum/test_slotnum.py::test_slotnum_distinct_per_block[fork_Amsterdam-blockchain_test]";
const BLOCK_INDICES = [_]usize{ 0, 1, 2, 3 };

/// 0-indexed RLP header fields this tool splices in a block's header wherever it appears as the
/// ancestor entry in its child's witness — see `engineerHop`'s doc comment for why each is needed.
/// Field order up to here: parent_hash=0, ommers_hash=1, beneficiary=2, state_root=3,
/// transactions_root=4, receipts_root=5, logs_bloom=6, difficulty=7, number=8, gas_limit=9,
/// gas_used=10, timestamp=11, extra_data=12, mix_hash=13, nonce=14, base_fee_per_gas=15.
const STATE_ROOT_FIELD_INDEX: usize = 3;
const GAS_USED_FIELD_INDEX: usize = 10;
const BASE_FEE_FIELD_INDEX: usize = 15;

/// One RLP header field to overwrite, already encoded — `spliceHeaderFields` only re-assembles the
/// item list and re-encodes it as a list, so callers decide field semantics (u64 vs. 32-byte hash).
const HeaderFieldOverride = struct { index: usize, encoded: []const u8 };

/// Walks `header_bytes`' outer RLP list, replaces the items at each override's index, and
/// re-encodes — the byte-level surgery that lets this tool change a handful of a real header's
/// fields while carrying every other field (including every later-fork trailing field) through
/// byte-for-byte.
fn spliceHeaderFields(alloc: std.mem.Allocator, header_bytes: []const u8, overrides: []const HeaderFieldOverride) ![]const u8 {
    const outer = try mpt.rlp.decodeItem(header_bytes);
    const payload = switch (outer.item) {
        .list => |p| p,
        .bytes => return error.HeaderNotAnRlpList,
    };

    var items = std.ArrayListUnmanaged([]const u8).empty;
    var rest = payload;
    while (rest.len > 0) {
        const r = try mpt.rlp.decodeItem(rest);
        try items.append(alloc, rest[0..r.consumed]);
        rest = rest[r.consumed..];
    }

    for (overrides) |o| {
        if (o.index >= items.items.len) return error.HeaderTooShortForOverride;
        items.items[o.index] = o.encoded;
    }
    return rlp_enc.encodeList(alloc, items.items);
}

/// `[nonce, balance, storage_root, code_hash]` — the same account-leaf RLP shape zesu's own
/// (private) `output.encodeAccountRlp` and `conflation_plan.zig`'s `buildAccountRlp` use; mirrored
/// here for the same reason those two exist: zesu ships no public encoder of its own.
fn encodeAccountRlp(alloc: std.mem.Allocator, nonce: u64, balance: u256, storage_root: [32]u8, code_hash: [32]u8) ![]const u8 {
    const items = [_][]const u8{
        try rlp_enc.encodeU64(alloc, nonce),
        try rlp_enc.encodeU256(alloc, balance),
        try rlp_enc.encodeBytes(alloc, &storage_root),
        try rlp_enc.encodeBytes(alloc, &code_hash),
    };
    return rlp_enc.encodeList(alloc, &items);
}

/// Re-derives a block's post-state root through `mpt.updateAccountChainedIndexed`/
/// `updateStorageChainedIndexed` — the same indexed-insertion primitives `conflation_plan.zig`'s
/// `buildWorld` uses to make a world state walkable — from the block's own real, computed
/// `accessed` entries, and inserts every resulting node into `node_index` along the way.
///
/// Needed because zesu's own post-execution root computation (`computeStateRootDelta`, reached
/// through `computeStatelessInputWithLogsFn`) only ever returns the new root's hash: internally it
/// inserts newly-hashed trie nodes into the shared index only when a deletion forces a branch
/// collapse, so a block whose accounts only ever change VALUE (the common case, and every block in
/// this fixture) leaves its own post-state root's nodes undiscoverable in `node_index` afterward.
/// Re-deriving the identical set of account/storage changes through the indexed primitives instead
/// computes the same root — MPT root computation is deterministic in the set of updates applied —
/// while actually populating `node_index`, which the NEXT hop's own pre-state resolution needs.
/// The caller asserts the two roots agree, so a mismatch (a wrong assumption about which accounts
/// `accessed` covers) fails loudly here rather than silently shipping an unwalkable fixture.
fn rederiveWalkablePostStateRoot(
    alloc: std.mem.Allocator,
    pre_state_root: [32]u8,
    accessed: []const executor.executor_types.AccessedEntry,
    node_index: *mpt.NodeIndex,
) ![32]u8 {
    var state_root = pre_state_root;
    for (accessed) |entry| {
        var storage_root: [32]u8 = blk: {
            const existing = try mpt.verifyAccountIndexed(state_root, entry.address, node_index);
            break :blk if (existing) |acc| acc.storage_root else mpt.builder.EMPTY_TRIE_HASH;
        };
        for (entry.storage_changes) |sc| {
            try mpt.updateStorageChainedIndexed(alloc, &storage_root, sc.slot, sc.post_value, node_index);
        }

        const is_empty = entry.post_nonce == 0 and entry.post_balance == 0 and
            std.mem.eql(u8, &entry.post_code_hash, &primitives.KECCAK_EMPTY) and
            std.mem.eql(u8, &storage_root, &mpt.builder.EMPTY_TRIE_HASH);
        const account_rlp: ?[]const u8 = if (is_empty)
            null
        else
            try encodeAccountRlp(alloc, entry.post_nonce, entry.post_balance, storage_root, entry.post_code_hash);
        try mpt.updateAccountChainedIndexed(alloc, &state_root, mpt.keccak256(&entry.address), account_rlp, node_index);
    }
    return state_root;
}

/// Every hash currently in `node_index` — a before-snapshot so a later
/// `nodesAddedSince` call can tell which entries a hop's own re-derivation actually created.
fn snapshotNodeIndexKeys(alloc: std.mem.Allocator, node_index: *const mpt.NodeIndex) !std.AutoHashMapUnmanaged([32]u8, void) {
    var seen = std.AutoHashMapUnmanaged([32]u8, void).empty;
    var it = node_index.iterator();
    while (it.next()) |entry| try seen.put(alloc, entry.key_ptr.*, {});
    return seen;
}

/// The RLP bytes of every `node_index` entry NOT present in `before` — the trie nodes one hop's
/// `rederiveWalkablePostStateRoot` call actually created. These must be carried into `curr`'s own
/// `witness.nodes` (not just left in this tool's in-memory `node_index`): the checked-in fixture is
/// re-decoded fresh by every later reader (`real_multiblock_test.zig`, the real guest), which
/// rebuilds its OWN combined index purely from the payloads' own encoded witness nodes.
fn nodesAddedSince(alloc: std.mem.Allocator, node_index: *const mpt.NodeIndex, before: *const std.AutoHashMapUnmanaged([32]u8, void)) ![]const []const u8 {
    var out = std.ArrayListUnmanaged([]const u8).empty;
    var it = node_index.iterator();
    while (it.next()) |entry| {
        if (!before.contains(entry.key_ptr.*)) try out.append(alloc, entry.value_ptr.*);
    }
    return out.toOwnedSlice(alloc);
}

/// Engineers one block-to-block transition so `curr` runs, under a forced constant base fee, atop
/// `prev`'s real (possibly already re-engineered) outcome:
///
///   - re-splices `curr`'s ancestor header (`witness.headers[last]`, which the corpus's own real
///     chain already carries as `prev`'s real header): `gas_used` becomes `prev`'s own
///     `gas_limit / 2` (the EIP-1559 gas target — the one point where the base-fee-derivation
///     formula outputs zero change block-to-block), `base_fee_per_gas` becomes `forced_base_fee`,
///     and `state_root` becomes `prev`'s CURRENT declared state root — needed because splicing
///     changes `prev`'s hash, and Amsterdam's EIP-2935 pre-block system call writes that hash into
///     state on every block regardless of its own transactions, so a `prev` this function already
///     re-engineered has a state root that no longer matches what its own header (as still held by
///     the corpus) declares;
///   - propagates the resulting hash: `curr.parent_hash` and `prev.block_hash` both become it;
///   - forces `curr.base_fee_per_gas` to `forced_base_fee` and checks every one of `curr`'s
///     transactions still clears it;
///   - re-derives `curr`'s actual `state_root`/`receipts_root`/`block_access_list` by really running
///     its execution once via `computeStatelessInputWithLogsFn` — the same real per-block engine
///     `l2_execution.runL2Execution` itself calls, stopping short of the final root-equality gate,
///     since that check is exactly what this function's re-derivation routes around — then
///     re-derives the SAME post-state root a second time via `rederiveWalkablePostStateRoot`, so
///     the next hop's own pre-state resolution finds a walkable trie in `node_index` rather than
///     just a hash.
///
/// `prev` and `curr` are mutated in place; `node_index` is the ONE index shared across every hop
/// (mirrors `l2_execution.zig`'s own combined index), since each hop's re-derivation both reads
/// nodes an earlier hop inserted and inserts new ones the next hop needs.
fn engineerHop(
    alloc: std.mem.Allocator,
    prev: *input.StatelessInput,
    curr: *input.StatelessInput,
    forced_base_fee: u64,
    node_index: *mpt.NodeIndex,
) !void {
    const prev_ep = &prev.new_payload_request.execution_payload;
    const curr_ep = &curr.new_payload_request.execution_payload;

    if (curr.witness.headers.len == 0) return error.MissingAncestorHeaderWitness;
    const ancestor_bytes = curr.witness.headers[curr.witness.headers.len - 1];
    const ancestor_hash = mpt.keccak256(ancestor_bytes);
    if (!std.mem.eql(u8, &ancestor_hash, &curr_ep.parent_hash)) return error.AncestorHeaderNotDeclaredParent;
    if (!std.mem.eql(u8, &ancestor_hash, &prev_ep.block_hash)) return error.AncestorHeaderNotPrevsOwnHash;

    const spliced = try spliceHeaderFields(alloc, ancestor_bytes, &.{
        .{ .index = STATE_ROOT_FIELD_INDEX, .encoded = try rlp_enc.encodeBytes(alloc, &prev_ep.state_root) },
        .{ .index = GAS_USED_FIELD_INDEX, .encoded = try rlp_enc.encodeU64(alloc, prev_ep.gas_limit / 2) },
        .{ .index = BASE_FEE_FIELD_INDEX, .encoded = try rlp_enc.encodeU64(alloc, forced_base_fee) },
    });
    const new_hash = mpt.keccak256(spliced);

    const new_headers = try alloc.dupe([]const u8, curr.witness.headers);
    new_headers[new_headers.len - 1] = spliced;
    curr.witness.headers = new_headers;
    curr_ep.parent_hash = new_hash;
    prev_ep.block_hash = new_hash;

    curr_ep.base_fee_per_gas = forced_base_fee;
    for (curr_ep.transactions) |tx| {
        if (tx.gas_price < @as(u128, forced_base_fee)) return error.TransactionFeeBelowForcedBaseFee;
    }

    const fork_name = curr.chain_config.fork_name orelse return error.MissingForkName;
    const recomputed = try l2_execution.test_api.computeStatelessInputWithLogsFn(alloc, curr.*, fork_name, node_index);

    // See `rederiveWalkablePostStateRoot`'s doc comment: `recomputed.proof.post_state_root` is the
    // real outcome, but its own trie nodes are not (in general) discoverable in `node_index`
    // afterward. Re-deriving the same root through the indexed primitives makes it walkable for the
    // next hop; the equality check confirms the re-derivation is faithful. The newly-created nodes
    // are captured and folded into `curr`'s own witness so they survive the SSZ round-trip: a fresh
    // reader of the engineered input (`real_multiblock_test.zig`, running the real guest) rebuilds
    // its own node index purely from the payloads' encoded witnesses, without this function's
    // in-memory one.
    const nodes_before = try snapshotNodeIndexKeys(alloc, node_index);
    const walkable_state_root = try rederiveWalkablePostStateRoot(alloc, prev_ep.state_root, recomputed.accessed, node_index);
    if (!std.mem.eql(u8, &walkable_state_root, &recomputed.proof.post_state_root)) {
        return error.WalkableStateRootRederivationMismatch;
    }
    const new_nodes = try nodesAddedSince(alloc, node_index, &nodes_before);
    curr.witness.nodes = try std.mem.concat(alloc, []const u8, &.{ curr.witness.nodes, new_nodes });

    curr_ep.state_root = recomputed.proof.post_state_root;
    curr_ep.receipts_root = recomputed.proof.receipts_root;

    // `block_access_list_encode.zig` always attributes a change to transaction index 0 — asserted
    // here to be the genuinely correct index (the block has exactly one transaction) so re-pointing
    // this function at a multi-transaction block fails loudly instead of silently writing a
    // plausible-looking but wrong access list.
    if (curr_ep.transactions.len != 1) return error.BlockAccessListEncoderNeedsExactlyOneTransaction;
    curr_ep.block_access_list = try bal_encode.encode(alloc, recomputed.accessed);
}

/// Builds the engineered multi-block input and returns its encoded `L2ExecutionProofPrivateInput`
/// SSZ bytes — the same bytes `real_multiblock_test.zig` decodes and runs through the real guest
/// pipeline. `alloc` is expected to be an arena (matching every other caller of this module's own
/// zesu-backed primitives): nothing here is ever individually freed.
pub fn buildEngineeredInput(alloc: std.mem.Allocator) ![]const u8 {
    zesu_allocator.set(alloc);

    const blocks = try zkevm_fixture.parseBlocks(alloc, FIXTURE_JSON);

    const decoded = try alloc.alloc(input.StatelessInput, BLOCK_INDICES.len);
    for (BLOCK_INDICES, 0..) |block_index, slot| {
        var found: ?zkevm_fixture.StatelessBlock = null;
        for (blocks) |blk| {
            if (!std.mem.eql(u8, blk.test_name, TEST_NAME)) continue;
            if (blk.block_index == block_index) {
                found = blk;
                break;
            }
        }
        const blk = found orelse return error.FixtureBlockNotFound;
        decoded[slot] = try ssz_decode.decode(alloc, blk.input);
    }

    // Conflation-level invariants `l2_execution.zig` enforces across the whole range: every block
    // shares the first block's fee_recipient and chain_id.
    const first_ep = decoded[0].new_payload_request.execution_payload;
    for (decoded[1..]) |si| {
        const ep = si.new_payload_request.execution_payload;
        if (!std.mem.eql(u8, &ep.fee_recipient, &first_ep.fee_recipient)) return error.FixtureFeeRecipientMismatch;
        if (si.chain_config.chain_id != decoded[0].chain_config.chain_id) return error.FixtureChainIdMismatch;
    }

    // The forced constant base fee for the whole range: the first block's own real value. The
    // first block is otherwise never modified — its own declared base fee is the anchor everything
    // else is forced to match.
    const forced_base_fee = first_ep.base_fee_per_gas;

    var combined_nodes = std.ArrayListUnmanaged([]const u8).empty;
    for (decoded) |si| try combined_nodes.appendSlice(alloc, si.witness.nodes);
    var node_index = try mpt.buildNodeIndex(alloc, combined_nodes.items);
    defer node_index.deinit();

    for (1..decoded.len) |slot| {
        try engineerHop(alloc, &decoded[slot - 1], &decoded[slot], forced_base_fee, &node_index);
    }

    const payloads = try alloc.alloc(l2_execution_ssz.LineaPayloadInput, decoded.len);
    for (decoded, 0..) |si, slot| {
        payloads[slot] = .{ .stateless_input_ssz = try stateless_input_encode.encode(alloc, si), .forced_transactions = &.{} };
    }

    const extended_input = l2_execution_ssz.L2ExecutionProofPrivateInput{
        .parent_ftx_rolling_hash = @splat(0),
        .parent_last_processed_ftx_number = 0,
        .chain_config = .{
            .chain_id = decoded[0].chain_config.chain_id,
            .coinbase = first_ep.fee_recipient,
            .l2_message_service_address = vanilla_wrap.DUMMY_L2_MESSAGE_SERVICE_ADDRESS,
        },
        .payloads = payloads,
    };
    return l2_execution_ssz.encodeInput(alloc, extended_input);
}

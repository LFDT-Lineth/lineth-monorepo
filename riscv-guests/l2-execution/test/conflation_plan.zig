//! Test DSL for the l2-execution guest's conflation logic.
//!
//! `ConflationPlan` fabricates a self-consistent multi-block guest input — real keccak-rooted
//! state tries, a real hash-chained header sequence, and the real SSZ envelope byte path — from a
//! small set of knobs with realistic defaults, so a scenario only states what it deviates from.
//! `StubEngine` stands in for per-block EVM execution at `l2_execution.runL2ExecutionWithEngine`'s
//! seam, trusting each payload's own declared outcome instead of running the EVM. `run`/
//! `expectReject` drive the two together end to end, through the guest's real conflation logic.
//!
//! Every block in a plan starts from the same world state (`world0`) except the last, which
//! transitions to a second world state (`world1`) differing only in the L1<->L2 bridge storage a
//! scenario declares via `bridgeStorage` — mirroring how a real range conflates N blocks against
//! one pre-state and commits one post-state.

const std = @import("std");

const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");
const input = @import("zesu_input");
const primitives = @import("zesu_primitives");
const rlp_decode = @import("zesu_rlp_decode");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const stateless_input_encode = @import("stateless_input_encode");

const types = executor.executor_types;
const tx_decode = executor.executor_tx_decode;
const tx_signing = executor.executor_tx_signing;
const rlp = executor.executor_rlp_encode;

// ─── Constants ──────────────────────────────────────────────────────────────────────────────────

const ZERO_ADDRESS: [20]u8 = @splat(0);
const ZERO_HASH: [32]u8 = @splat(0);

/// An arbitrary, visually-distinct default coinbase — every payload's `fee_recipient` matches it
/// by default, satisfying the guest's own `FeeRecipientMismatch` check with no per-block effort.
const DEFAULT_COINBASE: [20]u8 = @splat(0xc0);

/// A realistic L2MessageService address a caller opts into directly when it wants declared bridge
/// storage to be observable — a plain field assignment (e.g. `.l2_message_service_address =
/// conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS`), the same way every other override in this
/// DSL works (`block.base_fee = ...`, `block.fee_recipient = ...`).
pub const DEFAULT_L2_MESSAGE_SERVICE_ADDRESS: [20]u8 = @splat(0xee);

/// The vanilla wire schema's Amsterdam fork byte.
const AMSTERDAM_FORK_BYTE: u8 = 0x15;

/// A realistic gas limit; every header/payload shares it, and `gas_used` is pinned at exactly
/// half of it (see `buildHeader`'s doc comment for why).
const DEFAULT_GAS_LIMIT: u64 = 30_000_000;
const DEFAULT_BASE_FEE: u64 = 1_000_000_000;
const DEFAULT_BASE_TIMESTAMP: u64 = 1_700_000_000;
const BLOCK_TIME_SECONDS: u64 = 12;

/// L2MessageService's storage layout: the guest's own layout constants of the same name,
/// mirrored here since only the slot-address FORMULA (not these raw numbers) is reachable
/// through `test_api`.
const LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT: u64 = 280;
const L1_ROLLING_HASHES_MAPPING_BASE_SLOT: u64 = 281;

/// One packed dummy deposit-request-sized item — enough to make `execution_requests.deposits`
/// non-empty regardless of its actual (unparsed, opaque) content.
const NON_EMPTY_DEPOSIT_BYTES: [192]u8 = @splat(0xde);

const NON_EMPTY_WITHDRAWALS = [_]input.Withdrawal{
    .{ .index = 0, .validator_index = 0, .address = ZERO_ADDRESS, .amount = 1 },
};

const TWO_DEFAULT_BLOCKS = [_]BlockPlan{ .{}, .{} };

fn isZeroAddress(address: [20]u8) bool {
    return std.mem.allEqual(u8, &address, 0);
}

/// Default timestamps: strictly increasing, one realistic block time apart.
fn defaultTimestamp(block_index: usize) u64 {
    return DEFAULT_BASE_TIMESTAMP + @as(u64, @intCast(block_index)) * BLOCK_TIME_SECONDS;
}

// ─── Bridge storage ─────────────────────────────────────────────────────────────────────────────

/// The L1->L2 bridge rolling-hash state realized in a world's L2MessageService storage: `number`
/// at the fixed slot, `hash` at the mapping slot keyed by `number`.
pub const BridgeValue = struct {
    number: u64 = 0,
    hash: [32]u8 = ZERO_HASH,
};

// ─── Per-block plan ─────────────────────────────────────────────────────────────────────────────

pub const BlockPlan = struct {
    /// Signed transaction RLPs included in this block, in order.
    signed_tx_rlps: []const []const u8 = &.{},
    /// Logs for each entry in `signed_tx_rlps`, index-aligned; a shorter slice leaves the
    /// remaining transactions with no logs.
    tx_logs: []const []const types.Log = &.{},
    /// Forced-transaction witnesses this block declares (§6.5).
    forced_transactions: []const l2_execution_ssz.ForcedTransactionWitness = &.{},

    /// Overrides the range-level constant base fee for this block only.
    base_fee: ?u64 = null,
    /// Overrides the range-level coinbase-derived fee recipient for this block only.
    fee_recipient: ?[20]u8 = null,
    /// Overrides the INNER vanilla stateless-input's own chain_config.chain_id (as opposed to
    /// the range-level `ConflationPlan.chain_id`, which every block matches by default).
    chain_id: ?u64 = null,
    /// Overrides this block's derived (strictly-increasing) timestamp.
    timestamp: ?u64 = null,
    /// Populates `execution_requests.deposits` with one packed dummy item — enough to trip the
    /// guest's Lineth-does-not-support-EIP-7685 rejection.
    non_empty_execution_requests: bool = false,
    /// Populates payload-level `withdrawals` with one dummy entry — enough to trip the guest's
    /// no-beacon-chain-withdrawals rejection.
    non_empty_withdrawals: bool = false,
    /// Overrides the vanilla schema's fork byte (default: Amsterdam's, 0x15). 0x11 (Prague) is
    /// the documented decodable alternative — zesu's SSZ decoder accepts it and yields
    /// `fork_name = "Prague"`, reaching the guest's own fork check as a genuine mismatch rather
    /// than a decode failure.
    active_fork_idx: ?u8 = null,
};

// ─── The built (pre-envelope-encoding) value and the execution-seam stub ───────────────────────────

/// The pieces `StubEngine` needs per call, plus the fully-assembled envelope `run()` encodes and
/// round-trips. Built once by `ConflationPlan.build()`, then driven through the guest's real
/// conflation logic by `run()`.
pub const Built = struct {
    /// Block count; also the number of `executeStatelessInputWithLogs` calls `run()` expects.
    blocks_len: usize,
    /// Declared logs, indexed `[block_index][tx_index]`.
    tx_logs: []const []const []const types.Log,
    /// World1's node RLPs, inserted into the shared `node_index` only when `StubEngine` processes
    /// the last block — mirroring how the real engine admits post-state nodes mid-conflation.
    world1_nodes: []const []const u8,
    envelope: l2_execution_ssz.L2ExecutionProofPrivateInput,
    /// The range's parent and end block hashes, exactly as the guest parrots them into
    /// `public_inputs.parent_block_hash`/`end_block_hash` (`payloads[0]`'s parent_hash and
    /// `payloads[n-1]`'s block_hash, after any hooks) — exposed since a scenario asserting the
    /// full PI needs to compare against these independently-derived values, and the header RLP
    /// bytes they come from are otherwise private to this file's `buildHeader`.
    parent_block_hash: [32]u8 = ZERO_HASH,
    end_block_hash: [32]u8 = ZERO_HASH,
};

/// Structural counterpart of `execution.ProofOutputWithLogs` — same fields, so
/// `runL2ExecutionWithEngine`'s generic `Engine.executeStatelessInputWithLogs(...)` call sites and
/// the parity test's field-by-field comparison both work without importing that type by name.
const StubProofOutput = struct {
    pre_state_root: [32]u8,
    post_state_root: [32]u8,
    receipts_root: [32]u8,
    receipts: []const types.Receipt,
    fork_name: []const u8,
};

/// Contract stub for the `runL2ExecutionWithEngine(comptime Engine, ...)` seam: trusts each
/// payload's own declared state roots and this plan's declared logs instead of running the EVM.
pub const StubEngine = struct {
    /// The plan currently driving this stub, set by `ConflationPlan.run()` for the duration of
    /// one `runL2ExecutionWithEngine` call, and cleared after. `null` outside that window — the
    /// parity test drives this engine directly with `active` left `null`, exercising its
    /// no-declared-plan default (zero receipts) against real fixture data instead of any
    /// particular scenario.
    pub var active: ?*const Built = null;
    var next_index: usize = 0;

    pub fn executeStatelessInputWithLogs(
        alloc: std.mem.Allocator,
        si: input.StatelessInput,
        fork_name: []const u8,
        node_index: *mpt.NodeIndex,
    ) !StubProofOutput {
        const ep = &si.new_payload_request.execution_payload;
        const call_index = next_index;
        next_index += 1;

        const pre_state_root = rlp_decode.findPreStateRoot(si.witness.headers, ep.block_number) orelse blk: {
            const is_genesis = ep.block_number == 0 and std.mem.allEqual(u8, &ep.parent_hash, 0);
            if (!is_genesis) return error.MissingParentHeaderWitness;
            break :blk ep.state_root;
        };

        var receipts = std.ArrayListUnmanaged(types.Receipt).empty;
        if (active) |built| {
            const decoded_txs = try tx_decode.decodeTxs(alloc, ep.raw_transactions);
            const block_logs: []const []const types.Log = if (call_index < built.tx_logs.len) built.tx_logs[call_index] else &.{};

            for (decoded_txs, 0..) |*tx, tx_idx| {
                const from = try tx_signing.recoverSender(alloc, tx, si.chain_config.chain_id) orelse
                    return error.StubEngineSenderRecoveryFailed;
                const logs: []const types.Log = if (tx_idx < block_logs.len) block_logs[tx_idx] else &.{};
                try receipts.append(alloc, .{
                    .type = tx.type,
                    .tx_hash = mpt.keccak256(ep.raw_transactions[tx_idx]),
                    .tx_index = @intCast(tx_idx),
                    .block_hash = ep.block_hash,
                    .block_number = ep.block_number,
                    .from = from,
                    .to = tx.to,
                    .cumulative_gas_used = 0,
                    .gas_used = 0,
                    .contract_address = null,
                    // Receipt.logs is a mutable slice in zesu's own type (real execution builds
                    // it fresh per block); this plan's declared logs are read-only fixture data,
                    // so reusing them via `@constCast` is safe.
                    .logs = @constCast(logs),
                    .logs_bloom = @splat(0),
                    .status = 1,
                    .effective_gas_price = 0,
                });
            }

            if (built.blocks_len > 0 and call_index == built.blocks_len - 1) {
                for (built.world1_nodes) |node_rlp| try node_index.put(mpt.keccak256(node_rlp), node_rlp);
            }
        }

        return .{
            .pre_state_root = pre_state_root,
            .post_state_root = ep.state_root,
            .receipts_root = ep.receipts_root,
            .receipts = try receipts.toOwnedSlice(alloc),
            .fork_name = fork_name,
        };
    }
};

// ─── Real MPT world-state construction ─────────────────────────────────────────────────────────

const WorldState = struct {
    root: [32]u8,
    nodes: []const []const u8,
};

fn collectNodeRlps(alloc: std.mem.Allocator, index: *mpt.NodeIndex) ![]const []const u8 {
    var out = std.ArrayListUnmanaged([]const u8).empty;
    var it = index.valueIterator();
    while (it.next()) |v| try out.append(alloc, v.*);
    return out.toOwnedSlice(alloc);
}

fn buildAccountRlp(alloc: std.mem.Allocator, nonce: u64, balance: u256, storage_root: [32]u8, code_hash: [32]u8) ![]const u8 {
    const items = [_][]const u8{
        try rlp.encodeU64(alloc, nonce),
        try rlp.encodeU256(alloc, balance),
        try rlp.encodeBytes(alloc, &storage_root),
        try rlp.encodeBytes(alloc, &code_hash),
    };
    return rlp.encodeList(alloc, &items);
}

/// A real MPT world state for the L2MessageService's two bridge storage slots. `null` `l2ms_address`
/// (bridge-suppressed mode) yields the canonical empty-trie root and no nodes at all — genuinely
/// correct MPT semantics for an account-less trie, not a shortcut. Otherwise, a single account leaf
/// whose storage trie holds `bridge.number`/`bridge.hash` at the guest's own slot layout; a zero
/// value is naturally omitted by the trie, matching real EVM "unset slot" semantics.
fn buildWorld(alloc: std.mem.Allocator, l2ms_address: ?[20]u8, bridge: BridgeValue) !WorldState {
    const address = l2ms_address orelse return .{ .root = mpt.builder.EMPTY_TRIE_HASH, .nodes = &.{} };

    var index = try mpt.buildNodeIndex(alloc, &.{});

    var storage_root = mpt.builder.EMPTY_TRIE_HASH;
    const number_slot = l2_execution.test_api.u64ToSlot32Fn(LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT);
    try mpt.updateStorageChainedIndexed(alloc, &storage_root, number_slot, @as(u256, bridge.number), &index);

    const rolling_hash_slot = l2_execution.test_api.mappingSlotFn(
        l2_execution.test_api.u64ToSlot32Fn(L1_ROLLING_HASHES_MAPPING_BASE_SLOT),
        l2_execution.test_api.u64ToSlot32Fn(bridge.number),
    );
    const hash_value = std.mem.readInt(u256, &bridge.hash, .big);
    try mpt.updateStorageChainedIndexed(alloc, &storage_root, rolling_hash_slot, hash_value, &index);

    const account_rlp = try buildAccountRlp(alloc, 0, 0, storage_root, primitives.KECCAK_EMPTY);
    var state_root = mpt.builder.EMPTY_TRIE_HASH;
    try mpt.updateAccountChainedIndexed(alloc, &state_root, mpt.keccak256(&address), account_rlp, &index);

    return .{ .root = state_root, .nodes = try collectNodeRlps(alloc, &index) };
}

// ─── Real header construction ──────────────────────────────────────────────────────────────────

/// A real, RLP-list-encoded Ethereum block header carrying the given parent hash, number, state
/// root, and timestamp — every other field a realistic constant. `gas_used` is pinned at exactly
/// half of `gas_limit` (the EIP-1559 gas target), so a constant `base_fee_per_gas` across the
/// whole header chain reproduces itself under the real base-fee formula, matching this DSL's
/// constant-base-fee payload default. Stops right after `base_fee_per_gas` (fields 0-15): the
/// guest's own header consumers (`rlp_decode.findPreStateRoot`, and `decodeParentHeader` on the
/// real execution path) read no field past it, so every later-fork optional field is simply
/// absent rather than needing invented values for something nothing here checks.
fn buildHeader(alloc: std.mem.Allocator, parent_hash: [32]u8, number: u64, state_root: [32]u8, timestamp: u64) ![]const u8 {
    const zero_hash: [32]u8 = @splat(0);
    const zero_addr: [20]u8 = @splat(0);
    const zero_bloom: [256]u8 = @splat(0);

    const items = [_][]const u8{
        try rlp.encodeBytes(alloc, &parent_hash), // [0]
        try rlp.encodeBytes(alloc, &zero_hash), // [1] ommers_hash
        try rlp.encodeBytes(alloc, &zero_addr), // [2] beneficiary
        try rlp.encodeBytes(alloc, &state_root), // [3]
        try rlp.encodeBytes(alloc, &zero_hash), // [4] transactions_root
        try rlp.encodeBytes(alloc, &zero_hash), // [5] receipts_root
        try rlp.encodeBytes(alloc, &zero_bloom), // [6] logs_bloom
        try rlp.encodeU64(alloc, 0), // [7] difficulty
        try rlp.encodeU64(alloc, number), // [8]
        try rlp.encodeU64(alloc, DEFAULT_GAS_LIMIT), // [9]
        try rlp.encodeU64(alloc, DEFAULT_GAS_LIMIT / 2), // [10] gas_used == gas target
        try rlp.encodeU64(alloc, timestamp), // [11]
        try rlp.encodeBytes(alloc, &.{}), // [12] extra_data
        try rlp.encodeBytes(alloc, &zero_hash), // [13] mix_hash
        try rlp.encodeU64(alloc, 0), // [14] nonce
        try rlp.encodeU64(alloc, DEFAULT_BASE_FEE), // [15] base_fee_per_gas
    };
    return rlp.encodeList(alloc, &items);
}

fn ownedSingleHeader(alloc: std.mem.Allocator, header: []const u8) ![]const []const u8 {
    const out = try alloc.alloc([]const u8, 1);
    out[0] = header;
    return out;
}

// ─── The plan ───────────────────────────────────────────────────────────────────────────────────

pub const ConflationPlan = struct {
    /// One entry per block; the slice length IS the range's block count (empty selects the
    /// `EmptyPayloads` rejection case).
    blocks: []const BlockPlan = &TWO_DEFAULT_BLOCKS,

    chain_id: u64 = 59144,
    coinbase: [20]u8 = DEFAULT_COINBASE,
    /// Zero means bridge-suppressed mode: the L1<->L2 bridge rolling-hash reads and the L2->L1
    /// message scan are both skipped.
    l2_message_service_address: [20]u8 = ZERO_ADDRESS,
    parent_ftx_rolling_hash: [32]u8 = ZERO_HASH,
    parent_last_processed_ftx_number: u64 = 0,
    /// The first block's number. 0 selects genesis mode: a real parent header cannot exist for
    /// block 0, so `payloads[0]`'s parent_hash is the all-zero hash and its witness carries no
    /// parent header at all.
    start_block_number: u64 = 1_000_000,

    bridge_parent: ?BridgeValue = null,
    bridge_end: ?BridgeValue = null,

    // ── Post-derivation hooks, applied to the built (not-yet-encoded) value ──
    /// Truncates payload `i`'s encoded stateless_input_ssz bytes so the vanilla SSZ decoder
    /// rejects them outright.
    corrupt_stateless_input_at: ?usize = null,
    /// Overrides payload `i`'s parent_hash after derivation, breaking the natural hash chain.
    override_parent_hash_at: ?ParentHashOverride = null,
    /// Drops payload `i`'s witness headers entirely.
    drop_witness_headers_at: ?usize = null,
    /// Forces `payloads[0]`'s parent_hash to a nonzero value — meaningful in genesis mode, where
    /// the natural derived value is the all-zero hash and the guest requires exactly that.
    override_genesis_parent_hash: ?[32]u8 = null,

    pub const ParentHashOverride = struct { index: usize, parent_hash: [32]u8 };

    /// Declares this range's L1<->L2 bridge storage at the range's pre-state (`.parent`) or
    /// post-state (`.end`) — realized as real storage under the L2MessageService's own layout by
    /// `build()`. These declared values are only realized in the trie if
    /// `l2_message_service_address` is non-zero at build time — a zero address keeps the range
    /// suppressed and the declared values are simply never observed.
    pub fn bridgeStorage(self: *ConflationPlan, which: enum { parent, end }, value: BridgeValue) void {
        switch (which) {
            .parent => self.bridge_parent = value,
            .end => self.bridge_end = value,
        }
    }

    /// Derives a fully self-consistent, real-MPT/real-header multi-block input, applies any
    /// declared hooks, and encodes every payload's vanilla bytes — everything `run()` needs short
    /// of the outer envelope round-trip.
    pub fn build(self: ConflationPlan, alloc: std.mem.Allocator) !Built {
        const n = self.blocks.len;
        const chain_config = l2_execution_ssz.ChainConfig{
            .l2_message_service_address = self.l2_message_service_address,
            .coinbase = self.coinbase,
            .chain_id = self.chain_id,
        };

        if (n == 0) {
            return .{
                .blocks_len = 0,
                .tx_logs = &.{},
                .world1_nodes = &.{},
                .envelope = .{
                    .parent_ftx_rolling_hash = self.parent_ftx_rolling_hash,
                    .parent_last_processed_ftx_number = self.parent_last_processed_ftx_number,
                    .chain_config = chain_config,
                    .payloads = &.{},
                },
            };
        }

        const genesis = self.start_block_number == 0;
        const l2ms_for_world: ?[20]u8 = if (isZeroAddress(self.l2_message_service_address)) null else self.l2_message_service_address;
        const world0_bridge = self.bridge_parent orelse BridgeValue{};
        const world0 = try buildWorld(alloc, l2ms_for_world, world0_bridge);
        // Inherits world0's values (no divergence) unless `.end` was declared — "equal to world0
        // when no bridge storage declared" holds field-by-field, not just in the no-bridge-at-all
        // case.
        const world1_bridge = self.bridge_end orelse world0_bridge;
        const world1 = try buildWorld(alloc, l2ms_for_world, world1_bridge);

        // ── Headers: a real hash chain. header[i] represents block (start+i); its state_root is
        // that block's OWN post-state (world0 for every block but the last, world1 for the last)
        // — exactly mirroring payload[i].state_root, since a header's state_root IS its block's
        // post-execution root. The range parent header (block start-1, non-genesis only) carries
        // world0's root: the range's pre-state is, by definition, block `start`'s pre-state.
        //
        // Genesis mode's block 0 has no witnessable pre-state at all (the real engine's own
        // pre_state_root fallback is `payload.state_root` itself whenever no header resolves) —
        // this DSL derives a distinct world0/world1 pair meaningfully whenever `n > 1`, or
        // whenever the range starts after genesis.
        var headers = try alloc.alloc([]const u8, n);
        const first_ts = self.blocks[0].timestamp orelse defaultTimestamp(0);
        var range_parent_header: ?[]const u8 = null;
        var built_parent_block_hash: [32]u8 = ZERO_HASH;
        var built_end_block_hash: [32]u8 = ZERO_HASH;
        if (!genesis) {
            const parent_ts = if (first_ts >= BLOCK_TIME_SECONDS) first_ts - BLOCK_TIME_SECONDS else 0;
            range_parent_header = try buildHeader(alloc, ZERO_HASH, self.start_block_number - 1, world0.root, parent_ts);
        }
        var prev_header: ?[]const u8 = range_parent_header;
        for (0..n) |i| {
            const ts = self.blocks[i].timestamp orelse defaultTimestamp(i);
            const state_root_i = if (i == n - 1) world1.root else world0.root;
            const parent_hash_field = if (prev_header) |h| mpt.keccak256(h) else ZERO_HASH;
            headers[i] = try buildHeader(alloc, parent_hash_field, self.start_block_number + i, state_root_i, ts);
            prev_header = headers[i];
        }

        // ── Payloads: one vanilla StatelessInput per block, each witnessing only its own parent
        // header (genesis's block 0 witnesses none), then hook-adjusted and encoded.
        var payloads = try alloc.alloc(l2_execution_ssz.LineaPayloadInput, n);
        var tx_logs = try alloc.alloc([]const []const types.Log, n);

        for (0..n) |i| {
            const block = self.blocks[i];
            const witness_header: ?[]const u8 = if (i == 0) range_parent_header else headers[i - 1];
            const natural_parent_hash = if (witness_header) |h| mpt.keccak256(h) else ZERO_HASH;
            const parent_hash_field = if (i == 0) (self.override_genesis_parent_hash orelse natural_parent_hash) else natural_parent_hash;
            const state_root_i = if (i == n - 1) world1.root else world0.root;

            var si = input.StatelessInput{
                .new_payload_request = .{
                    .execution_payload = .{
                        .parent_hash = parent_hash_field,
                        .fee_recipient = block.fee_recipient orelse self.coinbase,
                        .state_root = state_root_i,
                        .receipts_root = ZERO_HASH,
                        .logs_bloom = @splat(0),
                        .prev_randao = ZERO_HASH,
                        .block_number = self.start_block_number + i,
                        .gas_limit = DEFAULT_GAS_LIMIT,
                        .gas_used = DEFAULT_GAS_LIMIT / 2,
                        .timestamp = block.timestamp orelse defaultTimestamp(i),
                        .extra_data = &.{},
                        .base_fee_per_gas = block.base_fee orelse DEFAULT_BASE_FEE,
                        .block_hash = mpt.keccak256(headers[i]),
                        .transactions = &.{},
                        .raw_transactions = block.signed_tx_rlps,
                        .withdrawals = if (block.non_empty_withdrawals) &NON_EMPTY_WITHDRAWALS else &.{},
                        .blob_gas_used = 0,
                        .excess_blob_gas = 0,
                        .slot_number = self.start_block_number + i,
                        .block_access_list = &.{},
                    },
                    .parent_beacon_block_root = ZERO_HASH,
                    .versioned_hashes = &.{},
                    .execution_requests = if (block.non_empty_execution_requests)
                        .{ .deposits = &NON_EMPTY_DEPOSIT_BYTES }
                    else
                        .{},
                },
                .witness = .{
                    .nodes = world0.nodes,
                    .codes = &.{},
                    .headers = if (witness_header) |h| try ownedSingleHeader(alloc, h) else &.{},
                },
                .chain_config = .{
                    .chain_id = block.chain_id orelse self.chain_id,
                    .active_fork_idx = block.active_fork_idx orelse AMSTERDAM_FORK_BYTE,
                    .activation_block = 0,
                    .activation_timestamp = null,
                },
                .public_keys = &.{},
            };

            if (self.drop_witness_headers_at) |idx| {
                if (idx == i) si.witness.headers = &.{};
            }
            if (self.override_parent_hash_at) |o| {
                if (o.index == i) si.new_payload_request.execution_payload.parent_hash = o.parent_hash;
            }

            if (i == 0) built_parent_block_hash = si.new_payload_request.execution_payload.parent_hash;
            if (i == n - 1) built_end_block_hash = si.new_payload_request.execution_payload.block_hash;

            var ssz_bytes = try stateless_input_encode.encode(alloc, si);
            if (self.corrupt_stateless_input_at) |idx| {
                if (idx == i) ssz_bytes = ssz_bytes[0..@min(ssz_bytes.len, 1)];
            }

            payloads[i] = .{ .stateless_input_ssz = ssz_bytes, .forced_transactions = block.forced_transactions };
            tx_logs[i] = block.tx_logs;
        }

        return .{
            .blocks_len = n,
            .tx_logs = tx_logs,
            .world1_nodes = world1.nodes,
            .envelope = .{
                .parent_ftx_rolling_hash = self.parent_ftx_rolling_hash,
                .parent_last_processed_ftx_number = self.parent_last_processed_ftx_number,
                .chain_config = chain_config,
                .payloads = payloads,
            },
            .parent_block_hash = built_parent_block_hash,
            .end_block_hash = built_end_block_hash,
        };
    }

    /// Builds the plan, round-trips the whole envelope through the guest's real SSZ byte path
    /// (`l2_execution_ssz.encodeInput`/`decodeInput`), then drives it through
    /// `l2_execution.runL2ExecutionWithEngine` bound to `StubEngine` — so every scenario walks
    /// the guest's real conflation logic end to end, on real bytes, with only per-block execution
    /// stubbed out.
    pub fn run(self: ConflationPlan, alloc: std.mem.Allocator) !l2_execution_ssz.L2ExecutionProofOutput {
        const built = try self.build(alloc);
        const raw = try l2_execution_ssz.encodeInput(alloc, built.envelope);
        const decoded = try l2_execution_ssz.decodeInput(alloc, raw);

        StubEngine.active = &built;
        StubEngine.next_index = 0;
        defer StubEngine.active = null;

        return l2_execution.runL2ExecutionWithEngine(StubEngine, alloc, decoded);
    }

    /// Runs the plan and asserts it fails with exactly `expected_error`.
    pub fn expectReject(self: ConflationPlan, alloc: std.mem.Allocator, expected_error: anyerror) !void {
        try std.testing.expectError(expected_error, self.run(alloc));
    }
};

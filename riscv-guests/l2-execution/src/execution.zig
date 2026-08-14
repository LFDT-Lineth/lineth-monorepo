const std = @import("std");

const executor = @import("zesu_executor");
const zesu_allocator = @import("zesu_allocator");
const primitives = @import("zesu_primitives");
const mpt = @import("zesu_mpt");
const db_mod = @import("zesu_db");
const context_mod = @import("zesu_context");
const input = @import("zesu_input");
const hardfork = @import("zesu_hardfork");
const rlp_decode = @import("zesu_rlp_decode");

const types = executor.executor_types;
const transition_mod = executor.executor_transition;
const output_mod = executor.executor_output;
const tx_decode = executor.executor_tx_decode;
const block_validation = executor.executor_block_validation;

/// Number of RLP header fields between `parent_hash` ([0]) and `number` ([8]): ommers_hash,
/// beneficiary, state_root, transactions_root, receipts_root, logs_bloom, difficulty. Mirrors the
/// field skip in zesu's `rlp_decode.decodeParentHeader`.
const HEADER_FIELDS_BETWEEN_PARENT_HASH_AND_NUMBER = 7;

// Log-preserving stateless-execution seam.
//
// Zesu's own `executor.executeStatelessInput`'s `ProofOutput` keeps only a per-receipt
// `logs_bloom` and drops the actual event logs. Rollup phases need the full logs (address +
// topics + data) to derive L1 message events, so this file re-runs the SAME preamble and
// block-execution path but returns the un-projected `Receipt` slice (with `.logs` intact)
// alongside the pre/post state roots.
//
// `buildEnv`/`finalizeOutputWithLogs` below are adapted from Zesu's private `executor/main.zig`
// glue (not exposed, so copied). Block/BAL validation and the accessed-entry builder are NOT
// copied: `executor.executor_block_validation` and `executor.buildAccessedEntries` call zesu's
// real implementations directly. Withdrawals are never converted/passed through: `l2_execution.zig`
// rejects any payload with a non-empty withdrawals list before this seam ever runs (Linea is an L2
// rollup — it has no beacon-chain withdrawals), so `buildEnv` below is always given an empty list.

/// Log-preserving counterpart of zesu's `output.ProofOutput`: identical roots, but `receipts`
/// carries the full, un-projected `Receipt` (each with its `[]Log`) instead of the vanilla guest's
/// bloom-only `ReceiptData`.
pub const ProofOutputWithLogs = struct {
    pre_state_root: primitives.Hash,
    post_state_root: primitives.Hash,
    receipts_root: primitives.Hash,
    receipts: []const types.Receipt,
    fork_name: []const u8,
};

/// A block's real transition outcome, computed but not yet checked against the payload's own
/// declared commitments (`state_root`/`receipts_root`/`block_access_list`) — everything
/// `block_validation.validatePostExecution` needs to run that check, for a caller that instead
/// needs to DISCOVER what those values genuinely are (test tooling re-deriving them after altering
/// a block's conditions) rather than verify a claim against them.
pub const ComputedBlock = struct {
    proof: ProofOutputWithLogs,
    accessed: []const types.AccessedEntry,
    env: types.Env,
    spec: primitives.SpecId,
    total_gas_used: u64,
    blob_gas_used: u64,
};

// ─── Private helpers (adapted from Zesu executor/main.zig) ───────────────────────────────────────

fn buildEnv(
    req: input.NewPayloadRequest,
    block_hashes: []types.BlockHashEntry,
    withdrawals: []types.Withdrawal,
    parent: ?rlp_decode.ParentHeader,
) types.Env {
    const ep = &req.execution_payload;
    return .{
        .coinbase = ep.fee_recipient,
        .gas_limit = ep.gas_limit,
        .number = ep.block_number,
        .timestamp = ep.timestamp,
        .difficulty = 0,
        .base_fee = ep.base_fee_per_gas,
        .random = ep.prev_randao,
        .excess_blob_gas = ep.excess_blob_gas,
        .parent_beacon_block_root = req.parent_beacon_block_root,
        .parent_hash = ep.parent_hash,
        .block_hashes = block_hashes,
        .withdrawals = withdrawals,
        .slot_number = ep.slot_number,
        .gas_used_header = ep.gas_used,
        .blob_gas_used_header = ep.blob_gas_used,
        .parent_gas_limit = if (parent) |p| p.gas_limit else null,
        .parent_gas_used = if (parent) |p| p.gas_used else null,
        .parent_timestamp = if (parent) |p| p.timestamp else null,
        .parent_base_fee = if (parent) |p| p.base_fee_per_gas else null,
        .parent_blob_gas_used = if (parent) |p| p.blob_gas_used else null,
        .parent_excess_blob_gas = if (parent) |p| p.excess_blob_gas else null,
    };
}

fn finalizeOutputWithLogs(
    alloc: std.mem.Allocator,
    pre_state_root: [32]u8,
    result: transition_mod.TransitionResult,
    node_index: *mpt.NodeIndex,
    spec: primitives.SpecId,
    witness_db: anytype,
) !ProofOutputWithLogs {
    const post_state_root = try output_mod.computeStateRootDelta(alloc, pre_state_root, result.alloc, result.deleted_accounts, node_index, witness_db);
    const receipts_root = try output_mod.computeReceiptsRoot(alloc, result.receipts);
    return .{
        .pre_state_root = pre_state_root,
        .post_state_root = post_state_root,
        .receipts_root = receipts_root,
        .receipts = result.receipts,
        .fork_name = hardfork.specName(spec),
    };
}

// ─── Shared preamble ──────────────────────────────────────────────────────────────────────────────

/// The pieces of `StatelessInput` a block's computation needs, derived once and shared by both
/// public entry points below: `pre_state_root` (read off the witness-verified parent header, or
/// `ep.state_root` at genesis), the block-hash table for the `BLOCKHASH` opcode, and the decoded
/// parent header `validateBlock` checks the declared base fee/gas limit/timestamp against.
const Preamble = struct {
    pre_state_root: [32]u8,
    block_hashes: []types.BlockHashEntry,
    parent_header: ?rlp_decode.ParentHeader,
};

/// A resolvable witness header is required for every non-genesis block: without one, this would
/// fall back to the payload's OWN claimed (post-execution) state_root as its pre-state root, which
/// is self-referential and disconnected from the real state behind `ep.parent_hash`. Genesis —
/// block 0 with an all-zero parent hash — is the only exemption. The witness's headers are also
/// walked here into a hash chain, verified to terminate at `ep.parent_hash`, so the parent header
/// `validateBlock` checks against is one this block's own witness actually proves.
fn derivePreamble(alloc: std.mem.Allocator, si: input.StatelessInput) !Preamble {
    const ep = &si.new_payload_request.execution_payload;

    const pre_state_root = rlp_decode.findPreStateRoot(si.witness.headers, ep.block_number) orelse blk: {
        const is_genesis = ep.block_number == 0 and std.mem.allEqual(u8, &ep.parent_hash, 0);
        if (!is_genesis) return error.MissingParentHeaderWitness;
        break :blk ep.state_root;
    };

    const HeaderInfo = struct { number: u64, parent_hash: [32]u8, hash: [32]u8 };
    var header_infos = std.ArrayListUnmanaged(HeaderInfo).empty;
    defer header_infos.deinit(alloc);
    // Not deinit'd: `block_hashes.items` is returned as `Preamble.block_hashes` and must outlive
    // this function, unlike `header_infos` above, which never leaves it. `alloc` is the caller's
    // arena throughout this codebase's own convention, so this is reclaimed with everything else
    // when that arena is torn down, not leaked.
    var block_hashes = std.ArrayListUnmanaged(types.BlockHashEntry).empty;
    for (si.witness.headers) |hdr_rlp| {
        const hash = mpt.keccak256(hdr_rlp);
        const outer = mpt.rlp.decodeItem(hdr_rlp) catch return error.InvalidWitness;
        var rest = switch (outer.item) {
            .list => |p| p,
            .bytes => return error.InvalidWitness,
        };
        const parent_hash_result = mpt.rlp.decodeItem(rest) catch return error.InvalidWitness;
        const parent_hash_bytes = switch (parent_hash_result.item) {
            .bytes => |b| b,
            .list => return error.InvalidWitness,
        };
        if (parent_hash_bytes.len != 32) return error.InvalidWitness;
        var parent_hash: [32]u8 = undefined;
        @memcpy(&parent_hash, parent_hash_bytes);
        rest = rest[parent_hash_result.consumed..];
        var skip: usize = 0;
        while (skip < HEADER_FIELDS_BETWEEN_PARENT_HASH_AND_NUMBER and rest.len > 0) : (skip += 1) {
            const field_result = mpt.rlp.decodeItem(rest) catch return error.InvalidWitness;
            rest = rest[field_result.consumed..];
        }
        if (rest.len == 0) return error.InvalidWitness;
        const block_number_result = mpt.rlp.decodeItem(rest) catch return error.InvalidWitness;
        const block_number_bytes = switch (block_number_result.item) {
            .bytes => |b| b,
            .list => return error.InvalidWitness,
        };
        if (block_number_bytes.len > 8) return error.InvalidWitness;
        var number: u64 = 0;
        for (block_number_bytes) |b| number = (number << 8) | b;
        try block_hashes.append(alloc, .{ .number = number, .hash = hash });
        try header_infos.append(alloc, .{ .number = number, .parent_hash = parent_hash, .hash = hash });
    }

    var parent_header: ?rlp_decode.ParentHeader = null;
    if (header_infos.items.len > 0) {
        for (0..header_infos.items.len - 1) |k| {
            if (!std.mem.eql(u8, &header_infos.items[k].hash, &header_infos.items[k + 1].parent_hash)) {
                return error.InvalidWitness;
            }
        }
        const last = header_infos.items[header_infos.items.len - 1];
        if (!std.mem.eql(u8, &last.hash, &ep.parent_hash)) {
            return error.InvalidWitness;
        }
        parent_header = rlp_decode.decodeParentHeader(si.witness.headers[si.witness.headers.len - 1]) catch
            return error.InvalidWitness;
    }

    return .{ .pre_state_root = pre_state_root, .block_hashes = block_hashes.items, .parent_header = parent_header };
}

/// Runs a block's real transition and computes its outcome, stopping short of checking that
/// outcome against the payload's own declared commitments — see `ComputedBlock`'s doc comment.
/// Every step here (env construction, block validation, `WitnessDatabase`/`Context` wiring,
/// `transitionWithContext`, access-log draining, root computation) is exactly what
/// `executeBlockStatelessWithLogs` below needs too; that function is this one plus the one
/// remaining check, not a second implementation of it.
fn computeBlockStatelessWithLogs(
    alloc: std.mem.Allocator,
    pre_state_root: [32]u8,
    node_index: *mpt.NodeIndex,
    req: input.NewPayloadRequest,
    witness_codes: []const []const u8,
    block_hashes: []types.BlockHashEntry,
    parent_header: ?rlp_decode.ParentHeader,
    fork_name: []const u8,
    chain_id: u64,
    public_keys: []const []const u8,
) !ComputedBlock {
    const ep = &req.execution_payload;

    const spec = hardfork.specForBlock(fork_name, ep.timestamp) orelse return error.UnsupportedFork;

    const env = buildEnv(req, block_hashes, &.{}, parent_header);
    try block_validation.validateBlock(env, spec);
    const txs = try tx_decode.decodeTxsFromInput(alloc, ep.transactions);

    var ctx = context_mod.Context(db_mod.WitnessDatabase).new(
        try db_mod.WitnessDatabase.init(alloc, node_index, pre_state_root, witness_codes, block_hashes),
        spec,
    );
    ctx.block = transition_mod.buildBlockEnv(env, spec);
    ctx.cfg.chain_id = chain_id;
    ctx.cfg.disable_base_fee = (env.base_fee == null);

    const empty_pre_alloc = std.AutoHashMapUnmanaged(types.Address, types.AllocAccount).empty;
    const result = try transition_mod.transitionWithContext(
        alloc,
        &ctx,
        empty_pre_alloc,
        env,
        txs,
        spec,
        chain_id,
        hardfork.blockReward(spec),
        public_keys,
    );
    if (ctx.ctx_error != .ok) return error.InvalidWitness;
    var access_log = ctx.journaled_state.takeAccessLog();
    defer access_log.deinit();
    const accessed = try executor.buildAccessedEntries(alloc, access_log, result.alloc, result.deleted_accounts, result.system_address_user_touched);
    const proof = try finalizeOutputWithLogs(alloc, pre_state_root, result, node_index, spec, ctx.getDb());

    return .{
        .proof = proof,
        .accessed = accessed,
        .env = env,
        .spec = spec,
        .total_gas_used = result.cumulative_gas,
        .blob_gas_used = result.blob_gas_used,
    };
}

// ─── Public API ────────────────────────────────────────────────────────────────────────────────

/// High-level, log-preserving stateless execution from a fully-decoded StatelessInput. Mirrors
/// zesu's `executor.executeStatelessInput` (same preamble: derives `pre_state_root`, builds the
/// block-hash table, validates the header chain) and its `executeBlockStateless` body (same
/// WitnessDatabase + Context wiring, same BAL validation), but returns `ProofOutputWithLogs` — full
/// receipts (with `.logs`) instead of the vanilla guest's bloom-only projection.
///
/// `node_index` is caller-built and caller-owned (NOT built here, unlike zesu's own
/// `executeStatelessInput`): `l2_execution.runL2Execution` already builds ONE `NodeIndex` combining
/// every payload's witness nodes (it needs that same combined index for its own Linea-specific
/// reads), and building a second, per-payload index here from `si.witness.nodes` alone — a strict
/// subset of what the caller already indexed — would just be duplicate work the guest can't afford
/// to pay for twice. `computeStateRootDelta` (via `finalizeOutputWithLogs` below) mutates
/// `node_index` in place (inserting the post-execution trie's updated nodes under their own,
/// distinct hashes); sharing one evolving index across the whole payload range is correct, not just
/// cheaper, since it's the same content-addressed trie throughout.
pub fn executeStatelessInputWithLogs(
    alloc: std.mem.Allocator,
    si: input.StatelessInput,
    fork_name: []const u8,
    node_index: *mpt.NodeIndex,
) !ProofOutputWithLogs {
    zesu_allocator.set(alloc);

    const preamble = try derivePreamble(alloc, si);
    const computed = try computeBlockStatelessWithLogs(
        alloc,
        preamble.pre_state_root,
        node_index,
        si.new_payload_request,
        si.witness.codes,
        preamble.block_hashes,
        preamble.parent_header,
        fork_name,
        si.chain_config.chain_id,
        si.public_keys,
    );

    const ep = &si.new_payload_request.execution_payload;
    try block_validation.validatePostExecution(alloc, computed.env, computed.spec, computed.total_gas_used, computed.blob_gas_used, ep.block_access_list, computed.accessed, .{
        .computed_state_root = computed.proof.post_state_root,
        .expected_state_root = ep.state_root,
        .computed_receipts_root = computed.proof.receipts_root,
        .expected_receipts_root = ep.receipts_root,
    });
    return computed.proof;
}

/// Same computation as `executeStatelessInputWithLogs`, without the final check against the
/// payload's own declared `state_root`/`receipts_root`/`block_access_list` — for a caller that
/// needs to discover a block's real outcome (test tooling re-deriving it after altering the block's
/// conditions, e.g. its base fee) rather than verify a claim against it. Exposed via
/// `l2_execution.test_api`, mirroring `executeStatelessInputWithLogsFn`'s own exposure there.
pub fn computeStatelessInputWithLogs(
    alloc: std.mem.Allocator,
    si: input.StatelessInput,
    fork_name: []const u8,
    node_index: *mpt.NodeIndex,
) !ComputedBlock {
    zesu_allocator.set(alloc);

    const preamble = try derivePreamble(alloc, si);
    return computeBlockStatelessWithLogs(
        alloc,
        preamble.pre_state_root,
        node_index,
        si.new_payload_request,
        si.witness.codes,
        preamble.block_hashes,
        preamble.parent_header,
        fork_name,
        si.chain_config.chain_id,
        si.public_keys,
    );
}

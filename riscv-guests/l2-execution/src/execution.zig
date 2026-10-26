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
const block_rlp_size = @import("zesu_block_rlp_size");

const block_hash = @import("block_hash.zig");

// Error classification (zesu-error re-namespacing and the exit-code taxonomy) lives in
// guest_errors; this file only wraps zesu calls with it.
const guest_errors = @import("guest_errors.zig");
const zesuErr = guest_errors.zesuErr;

// Log-preserving stateless-execution seam.
//
// Zesu's own `executor.executeStatelessInput`'s `ProofOutput` keeps only a per-receipt
// `logs_bloom` and drops the actual event logs. Rollup phases need the full logs (address +
// topics + data) to derive L1 message events, so this file re-runs the SAME preamble and
// block-execution path but returns the un-projected `Receipt` slice (with `.logs`) intact
// alongside the pre/post state roots.
//
// `buildEnv`/`finalizeOutputWithLogs` below and `block_hash.zig` are adapted from Zesu's private
// `executor/main.zig`/`executor/block_hash.zig` glue (not exposed, so copied). Block validation
// and the transition itself call zesu's real implementations directly. Withdrawals are never
// converted/passed through: `l2_execution.zig` rejects any payload with a non-empty withdrawals
// list before this seam ever runs (Linea is an L2 rollup — it has no beacon-chain withdrawals), so
// `buildEnv` below is always given an empty list.

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
/// declared commitments (`block_hash`/`state_root`/`receipts_root`/`block_access_list`/requests) —
/// everything `block_validation.validatePostExecution` needs to run that check, for a caller that
/// instead needs to DISCOVER what those values genuinely are (test tooling re-deriving them after
/// altering a block's conditions) rather than verify a claim against them.
pub const ComputedBlock = struct {
    proof: ProofOutputWithLogs,
    env: types.Env,
    spec: primitives.SpecId,
    total_gas_used: u64,
    blob_gas_used: u64,
    /// EIP-7928 block access list hash as executed (Amsterdam+), or null before Amsterdam.
    bal_hash: ?[32]u8,
    /// EIP-7685 requests hash as executed.
    requests_hash: [32]u8,
};

// ─── Private helpers (adapted from Zesu executor/main.zig) ───────────────────────────────────────

fn buildEnv(
    req: input.NewPayloadRequest,
    block_hashes: []types.BlockHashEntry,
    withdrawals: []types.Withdrawal,
    parent: ?rlp_decode.ParentHeader,
    spec: primitives.SpecId,
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
        .block_rlp_size = block_rlp_size.compute(ep, spec),
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
    const post_state_root = output_mod.computeStateRootDelta(alloc, pre_state_root, result.alloc, result.deleted_accounts, node_index, witness_db) catch |e| return zesuErr(e);
    const receipts_root = output_mod.computeReceiptsRoot(alloc, result.receipts) catch |e| return zesuErr(e);
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
/// public entry points below: `pre_state_root` (the witness-verified parent header's, or
/// `ep.state_root` at genesis), the block-hash table for the `BLOCKHASH` opcode, and the decoded
/// parent header `validateBlock` checks the declared base fee/gas limit/timestamp against.
const Preamble = struct {
    pre_state_root: [32]u8,
    block_hashes: []types.BlockHashEntry,
    parent_header: ?rlp_decode.ParentHeader,
};

/// A witness header chain is required for every non-genesis block: zesu's
/// `decodeWitnessHeaders` checks that the headers link into a hash chain ending at
/// `ep.parent_hash`, that the parent's number is one below the payload's, and takes the pre-state
/// root from that parent, so it is one this block's own witness actually proves. Genesis — block
/// 0 with an all-zero parent hash and no headers — is the only exemption: it executes against its
/// own declared `state_root` with no parent checks.
fn derivePreamble(alloc: std.mem.Allocator, si: input.StatelessInput) !Preamble {
    const ep = &si.new_payload_request.execution_payload;

    if (si.witness.headers.len == 0) {
        const is_genesis = ep.block_number == 0 and std.mem.allEqual(u8, &ep.parent_hash, 0);
        if (!is_genesis) return error.MissingParentHeaderWitness;
        return .{ .pre_state_root = ep.state_root, .block_hashes = &.{}, .parent_header = null };
    }

    const headers = executor.decodeWitnessHeaders(alloc, si.witness.headers, ep.parent_hash, ep.block_number) catch |e| switch (e) {
        // A header chain that does not link up or anchor at the payload's parent: a witness
        // failure, kept in the Linea witness category as before.
        error.InvalidWitness => return error.InvalidWitness,
        else => return zesuErr(e),
    };
    return .{ .pre_state_root = headers.pre_state_root, .block_hashes = headers.block_hashes, .parent_header = headers.parent };
}

/// Runs a block's real transition and computes its outcome, stopping short of checking that
/// outcome against the payload's own declared commitments — see `ComputedBlock`'s doc comment.
/// Every step here (env construction, block validation, `WitnessDatabase`/`Context` wiring,
/// `transitionWithContext`, root computation) is exactly what `executeStatelessInputWithLogs`
/// below needs too; that function is this one plus the remaining checks, not a second
/// implementation of it.
///
/// `txs` are the payload's transactions, already decoded from `raw_transactions` by the caller.
fn computeBlockStatelessWithLogs(
    alloc: std.mem.Allocator,
    pre_state_root: [32]u8,
    node_index: *mpt.NodeIndex,
    req: input.NewPayloadRequest,
    txs: []types.TxInput,
    witness_codes: []const []const u8,
    block_hashes: []types.BlockHashEntry,
    parent_header: ?rlp_decode.ParentHeader,
    spec: primitives.SpecId,
    chain_id: u64,
) !ComputedBlock {
    const env = buildEnv(req, block_hashes, &.{}, parent_header, spec);
    block_validation.validateBlock(env, spec) catch |e| return zesuErr(e);

    const witness_db = db_mod.WitnessDatabase.init(alloc, node_index, pre_state_root, witness_codes, block_hashes) catch |e| return zesuErr(e);
    var ctx = context_mod.Context(db_mod.WitnessDatabase).new(
        witness_db,
        spec,
    );
    ctx.block = transition_mod.buildBlockEnv(env, spec);
    ctx.cfg.chain_id = chain_id;
    ctx.cfg.disable_base_fee = (env.base_fee == null);

    const empty_pre_alloc = std.AutoHashMapUnmanaged(types.Address, types.AllocAccount).empty;
    const result = transition_mod.transitionWithContext(
        alloc,
        &ctx,
        empty_pre_alloc,
        env,
        txs,
        spec,
        chain_id,
        hardfork.blockReward(spec),
    ) catch |e| return zesuErr(e);
    // A witness miss surfaced through zesu's WitnessDatabase during live execution — the
    // taxonomy's documented "witness-backed database" case — gets its own Linea-layer name so it
    // stays in ExitCode.witness_resolution rather than collapsing to engine_reject like a
    // zesu-internal InvalidWitness.
    if (ctx.ctx_error != .ok) return error.WitnessDbResolution;
    const proof = finalizeOutputWithLogs(alloc, pre_state_root, result, node_index, spec, ctx.getDb()) catch |e| return zesuErr(e);

    return .{
        .proof = proof,
        .env = env,
        .spec = spec,
        .total_gas_used = result.cumulative_gas,
        .blob_gas_used = result.blob_gas_used,
        .bal_hash = result.bal_hash,
        .requests_hash = result.requests_hash,
    };
}

/// The payload's declared commitments that are not header fields of their own, but hashes the
/// header (and so `block_hash`) commits to: the EIP-7685 requests hash and the EIP-7928 block
/// access list hash. Mirrors zesu's `executeStatelessInput`.
const DeclaredHashes = struct { requests_hash: [32]u8, block_access_list_hash: [32]u8 };

fn declaredHashes(alloc: std.mem.Allocator, req: input.NewPayloadRequest) !DeclaredHashes {
    const er = req.execution_requests;
    const requests_hash = transition_mod.computeRequestsHash(alloc, er.deposits, er.withdrawals, er.consolidations, er.builder_deposits, er.builder_exits) catch |e| return zesuErr(e);
    return .{
        .requests_hash = requests_hash,
        .block_access_list_hash = mpt.keccak256(req.execution_payload.block_access_list),
    };
}

fn resolveSpec(fork_name: []const u8, ep: *const input.ExecutionPayload) !primitives.SpecId {
    return hardfork.specForBlock(fork_name, ep.timestamp) orelse error.UnsupportedFork;
}

/// Decode the payload's opaque transaction bytes into `ep.transactions` and return their executor
/// form. zesu's SSZ decoder leaves `transactions` empty and keeps only `raw_transactions`: a
/// transaction that fails to decode invalidates the block, not the input, so it is decoded here,
/// after the block-hash check, like zesu's `executeStatelessInput`. `ep.transactions` must be
/// populated before `buildEnv`: `block_rlp_size.compute` needs it to line up with
/// `raw_transactions`.
fn decodeTransactions(alloc: std.mem.Allocator, ep: *input.ExecutionPayload) ![]types.TxInput {
    const transactions = try alloc.alloc(input.Transaction, ep.raw_transactions.len);
    for (ep.raw_transactions, transactions) |raw, *tx| tx.* = rlp_decode.decodeSingleTx(alloc, raw) catch |e| return zesuErr(e);
    ep.transactions = transactions;
    return tx_decode.decodeTxsFromInput(alloc, ep.transactions) catch |e| return zesuErr(e);
}

// ─── Public API ────────────────────────────────────────────────────────────────────────────────

/// High-level, log-preserving stateless execution from a fully-decoded StatelessInput. Mirrors
/// zesu's `executor.executeStatelessInput` (same block-hash check, same preamble: derives
/// `pre_state_root`, builds the block-hash table, validates the header chain) and its
/// `executeBlockStateless` body (same WitnessDatabase + Context wiring, same post-execution
/// validation of gas, BAL hash, roots and requests hash), but returns `ProofOutputWithLogs` — full
/// receipts (with `.logs`) instead of the vanilla guest's bloom-only projection.
///
/// Blob transactions and versioned hashes are rejected by `l2_execution.zig` before this runs, so
/// zesu's versioned-hash check is not repeated here.
///
/// `node_index` is caller-built and caller-owned (NOT built here, unlike zesu's own
/// `executeStatelessInput`): `l2_execution.runL2Execution` already builds ONE `NodeIndex` combining
/// every payload's witness nodes (it needs that same combined index for its own Linea-specific
/// reads), and building a second, per-payload index here from `si.witness.nodes` alone — a strict
/// subset of what the caller already indexed — would just be duplicate work the guest can't afford
/// to pay for twice. `computeStateRootDelta` (via `finalizeOutputWithLogs` below) mutates
/// `node_index` in place (inserting the post-execution trie's updated nodes under their own,
/// distinct hashes); sharing one evolving index
/// across the whole payload range is correct, not just cheaper, since it's the same
/// content-addressed trie throughout.
pub fn executeStatelessInputWithLogs(
    alloc: std.mem.Allocator,
    si_in: input.StatelessInput,
    fork_name: []const u8,
    node_index: *mpt.NodeIndex,
) !ProofOutputWithLogs {
    zesu_allocator.set(alloc);

    var si = si_in;
    const ep = &si.new_payload_request.execution_payload;
    // The witness header chain first: an unresolvable parent is a Linea-classified failure
    // (`MissingParentHeaderWitness`), reported as such even when the payload is also malformed.
    const preamble = try derivePreamble(alloc, si);
    const spec = try resolveSpec(fork_name, ep);
    const declared = try declaredHashes(alloc, si.new_payload_request);
    block_hash.validate(alloc, si.new_payload_request, spec, declared.requests_hash, declared.block_access_list_hash) catch |e| return zesuErr(e);
    const txs = try decodeTransactions(alloc, ep);

    const computed = try computeBlockStatelessWithLogs(
        alloc,
        preamble.pre_state_root,
        node_index,
        si.new_payload_request,
        txs,
        si.witness.codes,
        preamble.block_hashes,
        preamble.parent_header,
        spec,
        si.chain_config.chain_id,
    );

    const bal: ?block_validation.BalHashes = if (computed.bal_hash) |h| .{ .computed = h, .expected = declared.block_access_list_hash } else null;
    block_validation.validatePostExecution(computed.env, computed.spec, computed.total_gas_used, computed.blob_gas_used, bal, .{
        .computed_state_root = computed.proof.post_state_root,
        .expected_state_root = ep.state_root,
        .computed_receipts_root = computed.proof.receipts_root,
        .expected_receipts_root = ep.receipts_root,
    }) catch |e| return zesuErr(e);
    if (primitives.isEnabledIn(spec, .prague) and !std.mem.eql(u8, &declared.requests_hash, &computed.requests_hash)) {
        return zesuErr(error.InvalidRequests);
    }
    return computed.proof;
}

/// Same computation as `executeStatelessInputWithLogs`, without any check against the payload's
/// own declared `block_hash`/`state_root`/`receipts_root`/`block_access_list`/requests — for a
/// caller that needs to discover a block's real outcome (test tooling re-deriving it after altering
/// the block's conditions, e.g. its base fee) rather than verify a claim against it. Exposed via
/// `l2_execution.test_api`, mirroring `executeStatelessInputWithLogsFn`'s own exposure there.
pub fn computeStatelessInputWithLogs(
    alloc: std.mem.Allocator,
    si_in: input.StatelessInput,
    fork_name: []const u8,
    node_index: *mpt.NodeIndex,
) !ComputedBlock {
    zesu_allocator.set(alloc);

    var si = si_in;
    const ep = &si.new_payload_request.execution_payload;
    const spec = try resolveSpec(fork_name, ep);
    const txs = try decodeTransactions(alloc, ep);
    const preamble = try derivePreamble(alloc, si);
    return computeBlockStatelessWithLogs(
        alloc,
        preamble.pre_state_root,
        node_index,
        si.new_payload_request,
        txs,
        si.witness.codes,
        preamble.block_hashes,
        preamble.parent_header,
        spec,
        si.chain_config.chain_id,
    );
}

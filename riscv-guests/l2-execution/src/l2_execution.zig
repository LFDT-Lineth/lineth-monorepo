//! l2-execution guest logic: the Linea-specific layer on top of per-block stateless execution.
//!
//! Faithful translation of the Python oracle (`rollup_spec.l2_execution.run_l2_execution_guest` and
//! its helpers) to Zig, wired against zesu's exposed modules:
//!   - per-block execution + full logs: `execution.executeStatelessInputWithLogs`;
//!   - vanilla stateless-input decode: `zesu_ssz_decode.decode`;
//!   - witness-backed MPT account/storage reads: `zesu_mpt.verifyAccountIndexed` /
//!     `verifyStorageIndexed` over a NodeIndex built ONCE from ALL payloads' witness state nodes
//!     combined (mirrors the Python reference's `_build_node_index` over `all_witnesses`) — the
//!     SAME index is then passed into `execution.executeStatelessInputWithLogs` for every payload,
//!     so the guest never pays for `mpt.buildNodeIndex` more than this one time.
//!
//! Transaction-sender recovery for forced transactions NOT in the block (the Invalid/Refused §6.5
//! sub-cases) needs the same ECDSA recovery zesu's block executor performs internally
//! (`executor.executor_tx_signing.recoverSender`). For transactions that ARE in the block (the
//! INCLUDED case), the sender is read directly off the block's own receipts instead
//! (`Receipt.from`, populated by the SAME `recoverSender` call inside zesu's `transition()`, in the
//! block's transaction order) — no second recovery needed.

const std = @import("std");

const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");
const input = @import("zesu_input");
const primitives = @import("zesu_primitives");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");
const rlp_decode = @import("zesu_rlp_decode");
const l2_execution_ssz = @import("l2_execution_ssz");

const execution = @import("execution.zig");

const types = executor.executor_types;
const tx_decode = executor.executor_tx_decode;
const tx_signing = executor.executor_tx_signing;

// ─── Constants (Readme.md §6.3 / §2.1 / §2.6) ─────────────────────────────────────────────────────

/// The fork this guest binary is compiled for. Per Readme.md §2.6, the fork is hardcoded into the
/// l2-execution guest binary — one conflation = one fork = one exec programVk — so it is NEVER read
/// from the (prover-controlled) input; see the fork check in `runL2Execution`.
const GUEST_FORK: []const u8 = "Amsterdam";

/// L2MessageService's `MessageSent` event topic0 (the L2->L1 bridge message signature).
const BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0: [32]u8 = .{
    0xe8, 0x56, 0xc2, 0xb8, 0xbd, 0x4e, 0xb0, 0x02,
    0x7c, 0xe3, 0x2e, 0xea, 0xf5, 0x95, 0xc2, 0x1b,
    0x0b, 0x6b, 0x46, 0x44, 0xb3, 0x26, 0xe5, 0xb7,
    0xbd, 0x80, 0xa1, 0xcf, 0x8d, 0xb7, 0x2e, 0x6c,
};

/// Storage layout of L2MessageService (see the Python oracle's docstring for provenance).
const LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT: u64 = 280;
const L1_ROLLING_HASHES_MAPPING_BASE_SLOT: u64 = 281;

const ForcedTransactionAcceptance = struct {
    pub const INCLUDED: u8 = 0;
    pub const BAD_NONCE: u8 = 1;
    pub const BAD_BALANCE: u8 = 2;
    pub const FILTERED_ADDRESS_FROM: u8 = 3;
    pub const FILTERED_ADDRESS_TO: u8 = 4;
};

// ─── Small hashing/encoding helpers (§2.1 / §6.3) ─────────────────────────────────────────────────

fn u64ToSlot32(n: u64) [32]u8 {
    var out: [32]u8 = @splat(0);
    std.mem.writeInt(u64, out[24..32], n, .big);
    return out;
}

fn u256ToBytes32(n: u256) [32]u8 {
    var out: [32]u8 = undefined;
    std.mem.writeInt(u256, &out, n, .big);
    return out;
}

/// Solidity mapping slot: keccak256(key_padded32 || base_slot).
fn mappingSlot(base_slot: [32]u8, key: [32]u8) [32]u8 {
    var buf: [64]u8 = undefined;
    @memcpy(buf[0..32], &key);
    @memcpy(buf[32..64], &base_slot);
    return mpt.keccak256(&buf);
}

/// `ChainConfig.hash(base_fee)`: keccak256(chainID_be32 || coinbase || l2MessageServiceAddress ||
/// baseFee_be32).
fn chainConfigHash(chain_config: l2_execution_ssz.ChainConfig, base_fee: u64) [32]u8 {
    var buf: [104]u8 = undefined;
    @memcpy(buf[0..32], &u64ToSlot32(chain_config.chain_id));
    @memcpy(buf[32..52], &chain_config.coinbase);
    @memcpy(buf[52..72], &chain_config.l2_message_service_address);
    @memcpy(buf[72..104], &u64ToSlot32(base_fee));
    return mpt.keccak256(&buf);
}

/// keccak256(prev || txHash || deadline_be32 || from) — §6.3's forced-tx rolling-hash update.
fn addToForcedTxRollingHash(prev: [32]u8, tx_hash: [32]u8, deadline: u64, from_address: [20]u8) [32]u8 {
    var buf: [116]u8 = undefined;
    @memcpy(buf[0..32], &prev);
    @memcpy(buf[32..64], &tx_hash);
    @memcpy(buf[64..96], &u64ToSlot32(deadline));
    @memcpy(buf[96..116], &from_address);
    return mpt.keccak256(&buf);
}

/// Hash of a list of 32-byte digests (e.g. `l2_l1_messages_hash`'s message-hash preimages). Named
/// `hashDigestList`, matching the Python oracle's `hash_digest_list` (renamed from `hash_hash_list`
/// for the same reason): "hash a HashList" reads as a typo, not a type name; `Digest` avoids the
/// verb/noun clash.
fn hashDigestList(alloc: std.mem.Allocator, values: []const [32]u8) ![32]u8 {
    const buf = try alloc.alloc(u8, values.len * 32);
    defer alloc.free(buf);
    for (values, 0..) |v, i| @memcpy(buf[i * 32 ..][0..32], &v);
    return mpt.keccak256(buf);
}

fn hashAddressList(alloc: std.mem.Allocator, values: []const [20]u8) ![32]u8 {
    const buf = try alloc.alloc(u8, values.len * 20);
    defer alloc.free(buf);
    for (values, 0..) |v, i| @memcpy(buf[i * 20 ..][0..20], &v);
    return mpt.keccak256(buf);
}

// ─── Witness-backed MPT state reads (mirrors state_transition.py's L2State) ───────────────────────
//
// Semantics (must match the Python oracle exactly — see Readme.md's state_transition.py docstrings):
//   - account/slot proven absent from the trie  -> `null` / `0` (NOT an error);
//   - a witness node needed to resolve the path is missing from the pool -> `error.InvalidProof`
//     propagates (guest rejection). `verifyAccountIndexed`/`verifyStorageIndexed` already draw this
//     exact line: they return `null`/`0` for a proof of absence (empty branch slot / mismatched leaf
//     suffix / empty trie root) and `error.InvalidProof` only when `NodeIndex` lookup itself misses.
//     This is DELIBERATELY NOT `zesu_db.WitnessDatabase`: its `basic()`/`storage()` catch
//     `error.InvalidProof` and silently treat it as absence (a leniency WitnessDatabase needs for
//     precompile addresses that have no witness proof during live EVM execution) — that would mask a
//     genuinely incomplete witness here, where the Python spec's `_mpt_lookup` raises instead.

/// Account at `address` proven against `state_root`, or `null` if proven absent.
fn readAccount(state_root: [32]u8, address: [20]u8, node_index: *const mpt.NodeIndex) !?mpt.AccountState {
    return mpt.verifyAccountIndexed(state_root, address, node_index);
}

/// Storage value at (`address`, `slot`) proven against `state_root`; `0` if the account or slot is
/// absent. Mirrors `L2State.storage`: look up the account for its `storage_root`, then walk the
/// per-account storage trie.
fn readStorage(state_root: [32]u8, address: [20]u8, slot: [32]u8, node_index: *const mpt.NodeIndex) !u256 {
    const account = try readAccount(state_root, address, node_index) orelse return 0;
    return mpt.verifyStorageIndexed(account.storage_root, slot, node_index);
}

/// L2->L1 message extraction: for each log across a block's receipts whose address is the
/// L2MessageService and whose topic0 is the bridge's `MessageSent` signature, the message hash is
/// `topics[3]`.
fn extractL2L1Messages(
    alloc: std.mem.Allocator,
    out: *std.ArrayListUnmanaged([32]u8),
    receipts: []const types.Receipt,
    l2_ms_address: [20]u8,
) !void {
    for (receipts) |receipt| {
        for (receipt.logs) |log| {
            if (!std.mem.eql(u8, &log.address, &l2_ms_address)) continue;
            if (log.topics.len == 0 or !std.mem.eql(u8, &log.topics[0], &BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0)) continue;
            if (log.topics.len < 4) return error.InvalidBridgeMessageLog;
            try out.append(alloc, log.topics[3]);
        }
    }
}

const BridgeState = struct { hash: [32]u8, number: u64 };

/// True when `address` is the all-zero (20-byte) address. Used to detect the "no L2MessageService
/// configured" mode below.
fn isZeroAddress(address: [20]u8) bool {
    return std.mem.allEqual(u8, &address, 0);
}

/// `read_l1l2_bridge_state`: the L1->L2 bridge rolling hash and its message number, read from
/// L2MessageService storage at `state_root`.
fn readL1L2BridgeState(state_root: [32]u8, l2_ms_address: [20]u8, node_index: *const mpt.NodeIndex) !BridgeState {
    const number_val = try readStorage(state_root, l2_ms_address, u64ToSlot32(LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT), node_index);
    if (number_val > std.math.maxInt(u64)) return error.RollingHashNumberOverflow;
    const number: u64 = @intCast(number_val);

    const rolling_hash_slot = mappingSlot(u64ToSlot32(L1_ROLLING_HASHES_MAPPING_BASE_SLOT), u64ToSlot32(number));
    const hash_val = try readStorage(state_root, l2_ms_address, rolling_hash_slot, node_index);
    return .{ .hash = u256ToBytes32(hash_val), .number = number };
}

// ─── Forced transactions (§6.5, mirrors validate_forced_transactions) ─────────────────────────────

/// Max gas fee mirroring `is_valid_forced_transaction`'s BAD_BALANCE arithmetic: gas * price for
/// Legacy/EIP-2930 (types 0/1); gas * maxFeePerGas (+ blob gas * maxFeePerBlobGas for type 3) for
/// EIP-1559/4844/7702 (types 2/3/4).
fn maxGasFee(tx: *const types.TxInput) u256 {
    if (tx.type == 2 or tx.type == 3 or tx.type == 4) {
        var fee: u256 = @as(u256, tx.gas) * @as(u256, tx.max_fee_per_gas orelse 0);
        if (tx.type == 3) {
            const blob_gas: u256 = @as(u256, tx.blob_versioned_hashes.len) * @as(u256, primitives.GAS_PER_BLOB);
            fee += blob_gas * @as(u256, tx.max_fee_per_blob_gas orelse 0);
        }
        return fee;
    }
    return @as(u256, tx.gas) * @as(u256, tx.gas_price orelse 0);
}

/// `validate_forced_transactions`: scans one payload's declared forced transactions, updates the
/// FTX rolling hash for every one of them, and asserts each has the declared outcome. Returns the
/// addresses bubbled up for L1-side sanction-list checking (Refused sub-cases).
fn validateForcedTransactions(
    alloc: std.mem.Allocator,
    curr_rolling_hash: *[32]u8,
    last_processed_ftx_number: *u64,
    chain_id: u64,
    payload: input.ExecutionPayload,
    block_pre_state_root: [32]u8,
    node_index: *const mpt.NodeIndex,
    forced_transactions: []const l2_execution_ssz.ForcedTransactionWitness,
) ![]const [20]u8 {
    var rejected = std.ArrayListUnmanaged([20]u8).empty;

    const payload_tx_hashes = try alloc.alloc([32]u8, payload.raw_transactions.len);
    for (payload.raw_transactions, 0..) |raw, i| payload_tx_hashes[i] = mpt.keccak256(raw);

    for (forced_transactions) |ftx| {
        if (ftx.number != last_processed_ftx_number.* + 1) return error.ForcedTxOutOfOrder;
        if (ftx.deadline < payload.block_number) return error.ForcedTxDeadlineExceeded;

        const decoded = try tx_decode.decodeTxs(alloc, &.{ftx.signed_tx_rlp});
        const tx = &decoded[0];
        const from_address = try tx_signing.recoverSender(alloc, tx, chain_id) orelse return error.ForcedTxSenderRecoveryFailed;
        const tx_hash = mpt.keccak256(ftx.signed_tx_rlp);

        // Rolling hash update for EVERY FTX in the range, regardless of outcome.
        curr_rolling_hash.* = addToForcedTxRollingHash(curr_rolling_hash.*, tx_hash, ftx.deadline, from_address);
        last_processed_ftx_number.* = ftx.number;

        if (ftx.acceptance == ForcedTransactionAcceptance.FILTERED_ADDRESS_FROM) {
            try rejected.append(alloc, from_address);
            continue;
        }
        if (ftx.acceptance == ForcedTransactionAcceptance.FILTERED_ADDRESS_TO) {
            const to = tx.to orelse return error.FilteredAddressToOnContractCreation;
            try rejected.append(alloc, to);
            continue;
        }

        var tx_in_block = false;
        for (payload_tx_hashes) |h| {
            if (std.mem.eql(u8, &h, &tx_hash)) {
                tx_in_block = true;
                break;
            }
        }

        if (ftx.acceptance == ForcedTransactionAcceptance.INCLUDED) {
            if (!tx_in_block) return error.IncludedForcedTxNotInBlock;
            continue;
        }
        if (ftx.acceptance != ForcedTransactionAcceptance.BAD_NONCE and
            ftx.acceptance != ForcedTransactionAcceptance.BAD_BALANCE)
        {
            return error.UnknownForcedTxAcceptance;
        }
        if (tx_in_block) return error.InvalidForcedTxFoundInBlock;

        const sender_account = try readAccount(block_pre_state_root, from_address, node_index) orelse
            return error.ForcedTxSenderAbsent;

        if (ftx.acceptance == ForcedTransactionAcceptance.BAD_NONCE) {
            if (sender_account.nonce == (tx.nonce orelse 0)) return error.BadNonceMismatch;
            continue;
        }

        // BAD_BALANCE.
        const max_fee = maxGasFee(tx);
        if (sender_account.balance >= max_fee + tx.value) return error.BadBalanceMismatch;
    }

    return rejected.toOwnedSlice(alloc);
}

// ─── Top-level guest logic (mirrors run_l2_execution_guest) ───────────────────────────────────────

/// l2-execution: emits the 16-field l2-execution PI for a contiguous block range, translating
/// `rollup_spec.l2_execution.run_l2_execution_guest` step by step. Per-block execution is delegated
/// to `execution.executeStatelessInputWithLogs`; this function adds only the Linea logic on top —
/// conflation-level linking, the empty-`executionRequests` policy, forced transactions, L2->L1
/// messages, and the L1->L2 bridge rolling-hash reads.
pub fn runL2Execution(alloc: std.mem.Allocator, in: l2_execution_ssz.L2ExecutionProofPrivateInput) !l2_execution_ssz.L2ExecutionProofOutput {
    zesu_allocator.set(alloc);

    if (in.payloads.len == 0) return error.EmptyPayloads;

    // Decode each payload's vanilla stateless input ONCE; parsed objects are shared between
    // execution and the Linea logic below.
    const stateless_inputs = try alloc.alloc(input.StatelessInput, in.payloads.len);
    for (in.payloads, 0..) |payload, i| {
        stateless_inputs[i] = ssz_decode.decode(alloc, payload.stateless_input_ssz) catch return error.InvalidStatelessInput;
    }

    // Combined NodeIndex over ALL payloads' witness state nodes (mirrors `_build_node_index` over
    // Python's `all_witnesses`), used for every Linea-extra MPT read below (FTX-sender accounts,
    // the L1->L2 bridge rolling-hash slots).
    var combined_nodes = std.ArrayListUnmanaged([]const u8).empty;
    for (stateless_inputs) |si| try combined_nodes.appendSlice(alloc, si.witness.nodes);
    var node_index = try mpt.buildNodeIndex(alloc, combined_nodes.items);
    defer node_index.deinit();

    const first_payload = stateless_inputs[0].new_payload_request.execution_payload;
    // The engine validates each payload's parentHash against its witness parent header, so the
    // range's parent block hash is the first payload's parentHash and the start block number is the
    // first payload's block number.
    const parent_block_hash = first_payload.parent_hash;
    const start_block_number = first_payload.block_number;
    const base_fee = first_payload.base_fee_per_gas; // asserted constant across the range (§2.1)
    const l2_ms_address = in.chain_config.l2_message_service_address;
    // "No L2MessageService configured" mode: a zero l2MessageServiceAddress means this range's
    // chain has no bridge contract, so there is nothing to read or scan. Both the L1->L2 bridge
    // rolling-hash boundary reads and the L2->L1 message-log extraction are suppressed and the four
    // bridge PI fields are pinned to zero (mirrors the Python oracle's read_l1l2_bridge_state
    // zero-address branch). This is a real semantic, not test scaffolding — but it is also what lets
    // a vanilla EF stateless input (which has no L2MessageService account, and whose witness only
    // covers nodes execution touched) be dummy-wrapped and run through this guest unchanged.
    const bridge_suppressed = isZeroAddress(l2_ms_address);

    var current_parent_hash = parent_block_hash;
    var current_ftx_rolling_hash = in.parent_ftx_rolling_hash;
    var current_last_processed_ftx_number = in.parent_last_processed_ftx_number;

    var l2_l1_messages = std.ArrayListUnmanaged([32]u8).empty;
    var tx_froms = std.ArrayListUnmanaged([20]u8).empty;
    var filtered_addresses = std.ArrayListUnmanaged([20]u8).empty;

    var range_pre_state_root: [32]u8 = undefined;
    var range_post_state_root: [32]u8 = undefined;
    var last_payload: input.ExecutionPayload = undefined;

    for (in.payloads, stateless_inputs, 0..) |linea_payload, si, idx| {
        const payload = si.new_payload_request.execution_payload;

        // ── Conflation-level invariants the engine cannot know (it validates each block in
        // isolation against its own witness parent) ──
        if (si.chain_config.chain_id != in.chain_config.chain_id) return error.ChainIdMismatch;
        if (!std.mem.eql(u8, &payload.parent_hash, &current_parent_hash)) return error.ParentHashChainMismatch;
        if (payload.base_fee_per_gas != base_fee) return error.BaseFeeNotConstant;
        if (!std.mem.eql(u8, &payload.fee_recipient, &in.chain_config.coinbase)) return error.FeeRecipientMismatch;
        // A real, hash-verified parent header MUST be resolvable from this payload's own witness,
        // UNLESS this genuinely is genesis (block 0), which has no parent to prove — theoretically
        // supported (some Lineth deployment could start a range there), but constrained to the
        // standard Ethereum convention (parent_hash == zero) so the exemption can't be (ab)used to
        // skip the header check for anything other than a real genesis block. This guards against a
        // real gap: `execution.zig`'s `pre_state_root` derivation (`rlp_decode.findPreStateRoot(...)
        // orelse ep.state_root`) falls back to the payload's OWN claimed (post-execution) state_root
        // as its pre-state root whenever no witness header matches — self-referential, and
        // completely disconnected from the real state behind `payload.parent_hash`. Combined with a
        // no-op block, that lets a forged witness pick an arbitrary starting trie and forge whatever
        // it reads from it (e.g. the first payload's `readL1L2BridgeState` reads, below, which land
        // straight in the public output). Requiring this to resolve forces `execution.zig`'s own
        // header-chain verification to run for real (never silently skipped) and ties
        // `payload.block_number` to the real parent's real number — closing the
        // block-number-contiguity gap noted below as a side effect, since `findPreStateRoot` only
        // matches a header that's part of the hash-chain verified back to `payload.parent_hash`.
        if (payload.block_number == 0) {
            if (!std.mem.allEqual(u8, &payload.parent_hash, 0)) return error.InvalidGenesisParentHash;
        } else if (rlp_decode.findPreStateRoot(si.witness.headers, payload.block_number) == null) {
            return error.MissingParentHeaderWitness;
        }
        // Monotonic timestamps follow from the engine's per-block check (zesu's
        // block_validation.validateBlock: `env.timestamp <= env.parent_timestamp` ->
        // error.InvalidBlockTimestampOlderThanParent), fed the witness-verified parent header's own
        // timestamp — guaranteed reachable now that a real parent header is required above.

        // ── Lineth policy: this rollup does not support EIP-7685 requests ──
        const requests = si.new_payload_request.execution_requests;
        if (requests.deposits.len != 0 or requests.withdrawals.len != 0 or requests.consolidations.len != 0) {
            return error.ExecutionRequestsNotSupported;
        }

        // ── Lineth policy: no beacon-chain withdrawals — this is an L2 rollup, not L1. Rejected here
        // (cheap length check) rather than processed, so no proving cycles are ever spent crediting
        // a withdrawal that can't legitimately exist on this chain.
        if (payload.withdrawals.len != 0) {
            return error.WithdrawalsNotSupported;
        }

        // ── Fork is hardcoded (§2.6), never taken from input: reject any payload that doesn't
        // declare the guest's own fork, then execute against GUEST_FORK regardless — so a mismatched
        // claim fails cleanly instead of silently executing under a different fork's rules.
        if (si.chain_config.fork_name == null or !std.mem.eql(u8, si.chain_config.fork_name.?, GUEST_FORK)) {
            return error.UnsupportedFork;
        }

        // ── State transition (delegated) ──
        // Reuses the SAME combined `node_index` built above (not a fresh per-payload one — see
        // `executeStatelessInputWithLogs`'s doc comment): it's a superset of `si.witness.nodes`
        // alone, so every proof this payload's execution needs is already indexed.
        const result = try execution.executeStatelessInputWithLogs(alloc, si, GUEST_FORK, &node_index);
        // Validity check the delegated seam deliberately does NOT do (executeStatelessInputWithLogs
        // is a faithful executeStatelessInput replica and returns computed roots without judging
        // them): reject any block whose recomputed post-state / receipts roots disagree with the
        // payload's claimed values — exactly the check zesu's own `executor.executeStatelessInput`
        // performs. Kept here at the caller level so the delegated seam stays a pure execution
        // replica.
        if (!std.mem.eql(u8, &result.post_state_root, &payload.state_root)) return error.PostStateRootMismatch;
        if (!std.mem.eql(u8, &result.receipts_root, &payload.receipts_root)) return error.ReceiptsRootMismatch;
        if (idx == 0) range_pre_state_root = result.pre_state_root;
        range_post_state_root = result.post_state_root;
        last_payload = payload;

        // Linea PI: each receipt's `.from` is the sender zesu's own transition() already recovered
        // (same recoverSender + chain_id, in block transaction order) — reused rather than
        // re-derived.
        for (result.receipts) |receipt| try tx_froms.append(alloc, receipt.from);

        // Forced transactions (§6.5): the Invalid sub-cases read the sender account at this block's
        // PARENT state root by walking the combined witness MPT.
        const block_filtered = try validateForcedTransactions(
            alloc,
            &current_ftx_rolling_hash,
            &current_last_processed_ftx_number,
            in.chain_config.chain_id,
            payload,
            result.pre_state_root,
            &node_index,
            linea_payload.forced_transactions,
        );
        try filtered_addresses.appendSlice(alloc, block_filtered);

        // L2->L1 messages from the block's logs (skipped entirely when no L2MessageService is
        // configured — see `bridge_suppressed`).
        if (!bridge_suppressed) try extractL2L1Messages(alloc, &l2_l1_messages, result.receipts, l2_ms_address);

        current_parent_hash = payload.block_hash;
    }

    // L1->L2 bridge rolling-hash boundary reads, at the range's parent (pre) and end (post) state
    // roots, by walking the combined witness MPT. Suppressed to zeros when no L2MessageService is
    // configured (see `bridge_suppressed`); the end>=parent check then trivially holds (0>=0).
    const parent_bridge: BridgeState = if (bridge_suppressed)
        .{ .hash = @splat(0), .number = 0 }
    else
        try readL1L2BridgeState(range_pre_state_root, l2_ms_address, &node_index);
    const end_bridge: BridgeState = if (bridge_suppressed)
        .{ .hash = @splat(0), .number = 0 }
    else
        try readL1L2BridgeState(range_post_state_root, l2_ms_address, &node_index);
    if (end_bridge.number < parent_bridge.number) return error.RollingHashNumberDecreased;

    const public_inputs = l2_execution_ssz.L2ExecutionProofPublicInput{
        .parent_block_hash = parent_block_hash,
        .end_block_hash = last_payload.block_hash,
        .end_block_number = last_payload.block_number,
        .end_block_timestamp = last_payload.timestamp,
        .l2_l1_messages_hash = try hashDigestList(alloc, l2_l1_messages.items),
        .parent_l1_l2_bridge_rolling_hash = parent_bridge.hash,
        .parent_l1_l2_bridge_rolling_hash_message_number = parent_bridge.number,
        .end_l1_l2_bridge_rolling_hash = end_bridge.hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_bridge.number,
        .dynamic_chain_config_hash = chainConfigHash(in.chain_config, base_fee),
        .parent_ftx_rolling_hash = in.parent_ftx_rolling_hash,
        .parent_processed_ftx_number = in.parent_last_processed_ftx_number,
        .end_ftx_rolling_hash = current_ftx_rolling_hash,
        .end_processed_ftx_number = current_last_processed_ftx_number,
        .filtered_addresses_hash = try hashAddressList(alloc, filtered_addresses.items),
        .tx_froms_hash = try hashAddressList(alloc, tx_froms.items),
    };

    return .{
        .public_inputs = public_inputs,
        .start_block_number = start_block_number,
        .l2_l1_messages = try l2_l1_messages.toOwnedSlice(alloc),
        .tx_froms = try tx_froms.toOwnedSlice(alloc),
        .filtered_addresses = try filtered_addresses.toOwnedSlice(alloc),
    };
}

// ─── Exposed for unit tests only (test/l2_execution_test.zig) ─────────────────────────────────────

pub const test_api = struct {
    pub const u64ToSlot32Fn = u64ToSlot32;
    pub const mappingSlotFn = mappingSlot;
    pub const chainConfigHashFn = chainConfigHash;
    pub const addToForcedTxRollingHashFn = addToForcedTxRollingHash;
    pub const hashDigestListFn = hashDigestList;
    pub const hashAddressListFn = hashAddressList;
    pub const readAccountFn = readAccount;
    pub const readStorageFn = readStorage;
    pub const readL1L2BridgeStateFn = readL1L2BridgeState;
    pub const extractL2L1MessagesFn = extractL2L1Messages;
    pub const validateForcedTransactionsFn = validateForcedTransactions;
    pub const recoverSenderFn = tx_signing.recoverSender;
    pub const Acceptance = ForcedTransactionAcceptance;
};

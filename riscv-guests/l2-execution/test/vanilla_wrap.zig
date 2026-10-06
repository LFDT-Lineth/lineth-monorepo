//! Shared "dummy-fill" bridge: wraps a VANILLA EF stateless input (schema 0x0001) into an extended
//! `L2ExecutionProofPrivateInput` (schema 0x0002) so the extended l2-execution guest can run on the
//! same corpus the vanilla guest runs on (the EF execution-spec-tests zkevm fixtures).
//!
//! Real wiring: `test/extended_vanilla_runner.zig` calls this in-process (host reference-test
//! guard).
//!
//! Dummy-fill choices (see `runL2Execution`'s conflation invariants in l2_execution.zig):
//!   - `payloads`: a single payload whose `stateless_input_ssz` is the vanilla bytes VERBATIM
//!     (zero-copy) and `forced_transactions` empty — a single payload trivially satisfies the
//!     parentHash-chaining / baseFee-constant checks (there's only one block in the "range").
//!   - `chain_config.chain_id`: copied from the vanilla input's own `chain_config.chain_id`, so the
//!     `ChainIdMismatch` check (which compares the payload's chain_id against this field) passes.
//!   - `chain_config.coinbase`: copied from the vanilla payload's `fee_recipient`, so the
//!     `FeeRecipientMismatch` check passes.
//!   - `chain_config.l2_message_service_address`: `DUMMY_L2_MESSAGE_SERVICE_ADDRESS` below, the zero
//!     address. This is not an arbitrary placeholder: it's the sentinel `runL2Execution` recognizes
//!     as "no L2MessageService configured" and responds to by suppressing the L1<->L2 bridge reads
//!     entirely (both boundary rolling hashes and message numbers pinned to zero, no L2->L1 message
//!     extraction) rather than attempting a witness-backed read that EF fixtures — which carry no
//!     L2MessageService account and no post-state witness coverage — cannot satisfy. See
//!     `l2_execution.zig`'s `bridge_suppressed` handling and its Python mirror in
//!     `rollup_spec/l2_execution.py`.
//!   - `parent_ftx_rolling_hash` / `parent_last_processed_ftx_number`: zero — there is no prior
//!     range, so both start at their genesis values.

const std = @import("std");

const ssz_decode = @import("zesu_ssz_decode");
const l2_execution_ssz = @import("l2_execution_ssz");

/// The dummy L2MessageService address: the zero address, `runL2Execution`'s sentinel for "no
/// L2MessageService configured" (see the file-level doc comment above). Kept as a single named
/// constant purely for readability at call sites.
pub const DUMMY_L2_MESSAGE_SERVICE_ADDRESS: [20]u8 = @splat(0);

/// Wrap a vanilla EF stateless input (raw SSZ `SszStatelessInput` bytes, schema 0x0001 framing as
/// produced by the EF fixtures) into an extended `L2ExecutionProofPrivateInput` (schema 0x0002),
/// re-encoded and ready to feed to `l2_execution_ssz.decodeInput` / `l2_execution.runL2Execution`.
///
/// The vanilla bytes are carried through verbatim as the single payload's `stateless_input_ssz` —
/// this function never re-encodes or mutates them, only reads `chain_id`/`fee_recipient` off the
/// decoded form to fill the extended envelope's `chain_config`.
pub fn wrapVanillaAsExtended(alloc: std.mem.Allocator, vanilla_stateless_input_ssz: []const u8) ![]u8 {
    const si = try ssz_decode.decode(alloc, vanilla_stateless_input_ssz);
    const fee_recipient = si.new_payload_request.execution_payload.fee_recipient;

    const payloads = try alloc.alloc(l2_execution_ssz.LineaPayloadInput, 1);
    payloads[0] = .{
        .stateless_input_ssz = vanilla_stateless_input_ssz,
        .forced_transactions = &.{},
    };

    const extended_input = l2_execution_ssz.L2ExecutionProofPrivateInput{
        .parent_ftx_rolling_hash = @splat(0),
        .parent_last_processed_ftx_number = 0,
        .chain_config = .{
            .chain_id = si.chain_config.chain_id,
            .coinbase = fee_recipient,
            .l2_message_service_address = DUMMY_L2_MESSAGE_SERVICE_ADDRESS,
        },
        .payloads = payloads,
    };

    return l2_execution_ssz.encodeInput(alloc, extended_input);
}

/// True when the vanilla stateless input declares any EIP-7685 execution request
/// (deposits / withdrawals / consolidations). The extended guest rejects these outright by Linea
/// policy (`error.ExecutionRequestsNotSupported`), so a harness feeding real EF fixtures should
/// SKIP such inputs rather than hand the prover a guaranteed-reject block. Kept here — the one place
/// that already SSZ-decodes the vanilla input — so the Go harness need not learn the SSZ layout.
pub fn vanillaHasExecutionRequests(alloc: std.mem.Allocator, vanilla_stateless_input_ssz: []const u8) !bool {
    const si = try ssz_decode.decode(alloc, vanilla_stateless_input_ssz);
    const r = si.new_payload_request.execution_requests;
    return r.deposits.len != 0 or r.withdrawals.len != 0 or r.consolidations.len != 0 or
        r.builder_deposits.len != 0 or r.builder_exits.len != 0;
}

/// True when the execution payload carries a non-empty beacon-chain withdrawals list, which the
/// guest rejects by Linea policy (`error.WithdrawalsNotSupported`). Same SKIP role as
/// `vanillaHasExecutionRequests`.
pub fn vanillaHasWithdrawals(alloc: std.mem.Allocator, vanilla_stateless_input_ssz: []const u8) !bool {
    const si = try ssz_decode.decode(alloc, vanilla_stateless_input_ssz);
    return si.new_payload_request.execution_payload.withdrawals.len != 0;
}

/// True when the input declares a blob transaction or Engine API versioned hashes, both rejected
/// by Linea policy.
pub fn vanillaHasBlobTransaction(alloc: std.mem.Allocator, vanilla_stateless_input_ssz: []const u8) !bool {
    const si = try ssz_decode.decode(alloc, vanilla_stateless_input_ssz);
    if (si.new_payload_request.versioned_hashes.len != 0) return true;
    for (si.new_payload_request.execution_payload.transactions) |tx| {
        if (tx.tx_type == 3) return true;
    }
    return false;
}

/// The current flat chain-config schema carries the active fork in its schema ID and has no
/// activation schedule to filter from the reference corpus.
pub fn vanillaHasForkActivationSchedule(alloc: std.mem.Allocator, vanilla_stateless_input_ssz: []const u8) !bool {
    _ = alloc;
    _ = vanilla_stateless_input_ssz;
    return false;
}

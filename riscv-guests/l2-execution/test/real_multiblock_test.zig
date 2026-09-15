//! Spike test: proves that a real, multi-block sequence borrowed from the EF execution-spec-tests
//! zkevm corpus and engineered into a self-consistent input (see `real_multiblock_fixture_gen.zig`'s
//! header comment and `engineerHop`'s doc comment for the full mechanism and provenance) runs
//! successfully through the l2-execution guest's FULL, REAL pipeline (`l2_execution.runL2Execution`,
//! delegating per-block execution to `src/execution.zig`'s `executeStatelessInputWithLogs` — NOT
//! `test/conflation_plan.zig`'s `StubEngine`).
//!
//! Every existing multi-block scenario in `test/l2_execution_range_test.zig` runs through
//! `StubEngine`, which trusts each payload's own declared post-state instead of running the EVM.
//! This is the one test in the suite that drives a genuine multi-block range through the real
//! per-block engine end to end.
//!
//! Builds the engineered input fresh via `real_multiblock_fixture_gen.buildEngineeredInput` on every
//! run rather than loading a checked-in file: the corpus JSON it derives from is embedded straight
//! from the execution-spec-tests dependency tree (wired in `build.zig`), so there is nothing to
//! regenerate or forget to re-commit — the input is always exactly what the current engineering
//! logic produces.
//!
//! Deliberately does NOT assert anything about the specific value of any state root: proving EVM
//! correctness is this repo's existing single-block conformance suite's job
//! (`extended-vanilla`/`test/l2_execution_test.zig`). This test only proves the CONFLATION
//! mechanism — chaining, base fee, senders, chain config — works end to end for a real multi-block
//! range through the real engine.

const std = @import("std");
const testing = std.testing;

const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const input = @import("zesu_input");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const fixture_gen = @import("real_multiblock_fixture_gen.zig");

const api = l2_execution.test_api;

/// Recovers a raw signed transaction's sender the same way `l2_execution_range_test.zig`'s own
/// `recoverFixtureSender` does: decode the standalone RLP, then run it through the guest's own
/// (test-exposed) sender-recovery primitive — never a hand-copied literal.
fn recoverSender(alloc: std.mem.Allocator, signed_tx_rlp: []const u8, chain_id: u64) ![20]u8 {
    const decoded = try executor.executor_tx_decode.decodeTxs(alloc, &.{signed_tx_rlp});
    var tx = decoded[0];
    return (try api.recoverSenderFn(alloc, &tx, chain_id)).?;
}

test "a real multi-block EF-corpus range runs through the full guest pipeline (not StubEngine)" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const fixture_bytes = try fixture_gen.buildEngineeredInput(alloc);
    const decoded_input = try l2_execution_ssz.decodeInput(alloc, fixture_bytes);
    try testing.expect(decoded_input.payloads.len >= 3);

    // Independently decode every payload's vanilla stateless input so every expectation below is
    // derived from the fixture's own real bytes rather than a hardcoded literal — the same
    // discipline `l2_execution_range_test.zig`'s own scenarios follow.
    const stateless_inputs = try alloc.alloc(input.StatelessInput, decoded_input.payloads.len);
    for (decoded_input.payloads, 0..) |payload, i| stateless_inputs[i] = try ssz_decode.decode(alloc, payload.stateless_input_ssz);

    // Sanity on the fixture itself: a genuine multi-block, real-child range (not unrelated blocks
    // masquerading as a range) — every block's number is exactly one more than the previous, and
    // every block carries at least one real transaction.
    for (stateless_inputs[1..], stateless_inputs[0 .. stateless_inputs.len - 1]) |curr, prev| {
        const curr_number = curr.new_payload_request.execution_payload.block_number;
        const prev_number = prev.new_payload_request.execution_payload.block_number;
        try testing.expectEqual(prev_number + 1, curr_number);
    }
    for (stateless_inputs) |si| {
        try testing.expect(si.new_payload_request.execution_payload.transactions.len >= 1);
    }

    // ── Run the REAL, full guest pipeline ────────────────────────────────────────────────────
    const output = try l2_execution.runL2Execution(alloc, decoded_input);

    const first_payload = stateless_inputs[0].new_payload_request.execution_payload;
    const last_payload = stateless_inputs[stateless_inputs.len - 1].new_payload_request.execution_payload;

    // No error (the call above would have returned one) — end-to-end acceptance.
    try testing.expectEqual(last_payload.block_number, output.public_inputs.end_block_number);

    // parent_block_hash: the first block's ORIGINAL (untouched) parent_hash — the real genesis hash
    // this range's first block was never modified to point away from.
    try testing.expectEqualSlices(u8, &first_payload.parent_hash, &output.public_inputs.parent_block_hash);

    // end_block_hash: the last block's own self-description (never touched — only ITS parent_hash
    // was ever spliced to point at the re-hashed second-to-last block).
    try testing.expectEqualSlices(u8, &last_payload.block_hash, &output.public_inputs.end_block_hash);

    // tx_froms / tx_froms_hash: the real senders recovered from every block's real transactions, in
    // block order, via the guest's own (test-exposed) recovery + hashing primitives — never a
    // hardcoded literal.
    var expected_tx_froms = std.ArrayListUnmanaged([20]u8).empty;
    for (stateless_inputs) |si| {
        const payload = si.new_payload_request.execution_payload;
        for (payload.raw_transactions) |raw| {
            try expected_tx_froms.append(alloc, try recoverSender(alloc, raw, si.chain_config.chain_id));
        }
    }

    try testing.expectEqual(expected_tx_froms.items.len, output.tx_froms.len);
    for (expected_tx_froms.items, output.tx_froms) |expected, actual| {
        try testing.expectEqualSlices(u8, &expected, &actual);
    }
    const expected_tx_froms_hash = try api.hashAddressListFn(alloc, expected_tx_froms.items);
    try testing.expectEqualSlices(u8, &expected_tx_froms_hash, &output.public_inputs.tx_froms_hash);

    // dynamic_chain_config_hash: a deterministic Linea-layer formula over known values (chain_id,
    // coinbase, l2MessageServiceAddress, base fee) — independently recomputed, not trusted from
    // the pipeline under test.
    const chain_config = l2_execution_ssz.ChainConfig{
        .l2_message_service_address = decoded_input.chain_config.l2_message_service_address,
        .coinbase = decoded_input.chain_config.coinbase,
        .chain_id = decoded_input.chain_config.chain_id,
    };
    const expected_chain_config_hash = api.chainConfigHashFn(chain_config, first_payload.base_fee_per_gas);
    try testing.expectEqualSlices(u8, &expected_chain_config_hash, &output.public_inputs.dynamic_chain_config_hash);

    // Deliberately NOT asserted: any specific state_root/receipts_root value — this fixture's
    // whole point is proving the conflation mechanism works, not re-covering EVM correctness
    // (already covered by this repo's existing single-block conformance suite).
}

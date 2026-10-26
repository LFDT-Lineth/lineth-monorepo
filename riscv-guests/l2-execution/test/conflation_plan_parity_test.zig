//! Stub-realism guard for the conflation-plan test DSL's `StubEngine`.
//!
//! One test proves `StubEngine` (with no plan driving it) computes the same roots and receipt
//! count as the real per-block execution seam on real data — the guarantee every scenario built
//! on this DSL implicitly relies on. A second, separate test smoke-checks the `ConflationPlan`
//! DSL itself: a default plan should run clean through the guest's real conflation logic before
//! any scenario suite is built on top of it. A third calls the real per-block seam directly, the
//! same way the first test does, to prove its own rejection of an unresolvable, non-genesis
//! parent header — independent of any conflation-level gate.

const std = @import("std");
const testing = std.testing;

const fixtures = @import("evm_execution_fixtures");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");
const mpt = @import("zesu_mpt");
const input = @import("zesu_input");
const l2_execution = @import("l2_execution");
const conflation_plan = @import("conflation_plan.zig");

test "StubEngine with no active plan matches the real execution seam on the committed EF fixture" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const fixture = try fixtures.loadStatelessBlock(alloc, fixtures.embedded.zkevm_stateless_block);
    const si = try ssz_decode.decode(alloc, fixture.input);

    // executeStatelessInputWithLogs (both the real one and the stub) expects the zesu_allocator
    // singleton set by the caller.
    zesu_allocator.set(alloc);

    var real_index = try mpt.buildNodeIndex(alloc, si.witness.nodes);
    defer real_index.deinit();
    const real = try l2_execution.test_api.executeStatelessInputWithLogsFn(alloc, si, si.chain_config.fork_name.?, &real_index);

    var stub_index = try mpt.buildNodeIndex(alloc, si.witness.nodes);
    defer stub_index.deinit();
    conflation_plan.StubEngine.active = null;
    const stub = try conflation_plan.StubEngine.executeStatelessInputWithLogs(alloc, si, si.chain_config.fork_name.?, &stub_index);

    try testing.expectEqualSlices(u8, &real.pre_state_root, &stub.pre_state_root);
    try testing.expectEqualSlices(u8, &real.post_state_root, &stub.post_state_root);
    try testing.expectEqualSlices(u8, &real.receipts_root, &stub.receipts_root);
    try testing.expectEqual(real.receipts.len, stub.receipts.len);
    try testing.expectEqualStrings(real.fork_name, stub.fork_name);
}

test "a default 2-block ConflationPlan runs through StubEngine end to end" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const plan = conflation_plan.ConflationPlan{};
    const output = try plan.run(alloc);

    try testing.expectEqual(plan.start_block_number, output.start_block_number);
    try testing.expectEqual(plan.start_block_number + 1, output.public_inputs.end_block_number);
}

test "executeStatelessInputWithLogs rejects a non-genesis block with no resolvable parent header witness" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // executeStatelessInputWithLogs expects the zesu_allocator singleton set by the caller (see
    // the first test above).
    zesu_allocator.set(alloc);

    // A minimal StatelessInput: block_number and parent_hash are both nonzero — not genesis — and
    // witness.headers is empty, so no header can ever resolve a pre-state root. The seam must
    // reject at this very first step, before touching any other field, so everything else below
    // is zeroed.
    const si = input.StatelessInput{
        .new_payload_request = .{
            .execution_payload = .{
                .parent_hash = @splat(0xaa),
                .fee_recipient = @splat(0),
                .state_root = @splat(0),
                .receipts_root = @splat(0),
                .logs_bloom = @splat(0),
                .prev_randao = @splat(0),
                .block_number = 5,
                .gas_limit = 0,
                .gas_used = 0,
                .timestamp = 0,
                .extra_data = &.{},
                .base_fee_per_gas = 0,
                .block_hash = @splat(0),
                .transactions = &.{},
                .withdrawals = &.{},
                .blob_gas_used = 0,
                .excess_blob_gas = 0,
            },
            .parent_beacon_block_root = @splat(0),
        },
        .witness = .{
            .nodes = &.{},
            .codes = &.{},
            .headers = &.{},
        },
    };

    var node_index = try mpt.buildNodeIndex(alloc, &.{});
    defer node_index.deinit();

    // fork_name is never inspected: rejection happens before the fork lookup that would use it.
    try testing.expectError(
        error.MissingParentHeaderWitness,
        l2_execution.test_api.executeStatelessInputWithLogsFn(alloc, si, "Amsterdam", &node_index),
    );
}

// ─── Block-hash binding and raw-transaction policy checks ─────────────────────────────────────────

const executor = @import("zesu_executor");

test "the engine's header encoder reproduces the committed EF fixture's block hash" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const fixture = try fixtures.loadStatelessBlock(alloc, fixtures.embedded.zkevm_stateless_block);
    const si = try ssz_decode.decode(alloc, fixture.input);
    const req = si.new_payload_request;
    const er = req.execution_requests;
    const requests_hash = try executor.executor_transition.computeRequestsHash(alloc, er.deposits, er.withdrawals, er.consolidations, er.builder_deposits, er.builder_exits);
    const bal_hash = mpt.keccak256(req.execution_payload.block_access_list);

    try l2_execution.test_api.block_hash.validate(alloc, req, .amsterdam, requests_hash, bal_hash);
}

test "executeStatelessInputWithLogs rejects a payload whose fields no longer match its block hash" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    zesu_allocator.set(alloc);

    const fixture = try fixtures.loadStatelessBlock(alloc, fixtures.embedded.zkevm_stateless_block);
    var si = try ssz_decode.decode(alloc, fixture.input);
    // `extra_data` feeds nothing but the header, so changing it leaves execution untouched and
    // isolates the block-hash binding.
    si.new_payload_request.execution_payload.extra_data = "tampered";

    var node_index = try mpt.buildNodeIndex(alloc, si.witness.nodes);
    defer node_index.deinit();
    try testing.expectError(
        error.EngineReject,
        l2_execution.test_api.executeStatelessInputWithLogsFn(alloc, si, si.chain_config.fork_name.?, &node_index),
    );
}

test "executeStatelessInputWithLogs rejects a payload whose block access list was altered" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    zesu_allocator.set(alloc);

    const fixture = try fixtures.loadStatelessBlock(alloc, fixtures.embedded.zkevm_stateless_block);
    var si = try ssz_decode.decode(alloc, fixture.input);
    // An empty RLP list is a well-formed but wrong BAL: the header commits to the real one's hash.
    si.new_payload_request.execution_payload.block_access_list = &.{0xc0};

    var node_index = try mpt.buildNodeIndex(alloc, si.witness.nodes);
    defer node_index.deinit();
    try testing.expectError(
        error.EngineReject,
        l2_execution.test_api.executeStatelessInputWithLogsFn(alloc, si, si.chain_config.fork_name.?, &node_index),
    );
}

test "blob transactions are recognised from their raw EIP-2718 type byte" {
    const is_blob = l2_execution.test_api.isBlobTransactionFn;
    try testing.expect(is_blob(&.{ 0x03, 0xf8, 0x00 }));
    try testing.expect(!is_blob(&.{ 0x02, 0xf8, 0x00 }));
    try testing.expect(!is_blob(&.{ 0xf8, 0x6b })); // legacy: RLP list prefix
    try testing.expect(!is_blob(&.{}));
}

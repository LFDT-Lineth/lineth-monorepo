//! Builds a real, multi-block sequence borrowed from the EF execution-spec-tests zkevm corpus into
//! an input for the l2-execution guest's REAL, full pipeline (`l2_execution.runL2Execution`, not
//! `test/conflation_plan.zig`'s `StubEngine`) — computed fresh at test run time by
//! `buildEngineeredInput`, not read from a checked-in file, so it can never go stale relative to the
//! logic that produces it.
//!
//! No existing scenario in this suite drives a genuine multi-block range through the REAL per-block
//! engine (`src/execution.zig`'s `executeStatelessInputWithLogs`) end to end — every scenario in
//! `test/l2_execution_range_test.zig` fakes per-block execution via `StubEngine`. This file closes
//! that gap; `real_multiblock_test.zig` is what actually exercises the input it builds.
//!
//! The corpus blocks are used verbatim. They already satisfy every conflation-level invariant
//! `runL2Execution` enforces (a contiguous parent-hash chain, one fee recipient, one chain id, and a
//! constant base fee: 7, the EIP-1559 floor, where the per-block adjustment rounds to zero), and the
//! engine checks each payload's block hash, so the blocks cannot be altered without re-deriving
//! their hashes and block access lists. `buildEngineeredInput` asserts those invariants up front,
//! so a corpus bump that breaks one fails loudly here rather than as an opaque guest rejection.

const std = @import("std");

const zkevm_fixture = @import("zkevm_fixture.zig");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");
const input = @import("zesu_input");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const vanilla_wrap = @import("vanilla_wrap");

/// The chosen corpus test case's own JSON, embedded straight from the execution-spec-tests
/// dependency tree (wired in `build.zig`, mirroring `evm_execution_fixtures.zig`'s own
/// `zkevm_stateless_block.json` embed) — no committed copy of either the corpus JSON or the
/// input it produces. Four consecutive valid blocks (numbers 1-4), each carrying one real
/// signed transaction, no withdrawals, no EIP-7685 execution requests, and — since the transaction
/// only ever reads `SLOTNUM`/`NUMBER` into storage — no dependency on any block's hash, making
/// every block's witness carry exactly its own immediate parent header (no deeper ancestor chain to
/// keep consistent).
const FIXTURE_JSON = @embedFile("slotnum_distinct_per_block.json");
const TEST_NAME = "tests/amsterdam/eip7843_slotnum/test_slotnum.py::test_slotnum_distinct_per_block[fork_Amsterdam-blockchain_test]";
const BLOCK_INDICES = [_]usize{ 0, 1, 2, 3 };

/// Builds the multi-block input and returns its encoded `L2ExecutionProofPrivateInput` SSZ bytes —
/// the same bytes `real_multiblock_test.zig` decodes and runs through the real guest pipeline.
/// `alloc` is expected to be an arena (matching every other caller of this module's own zesu-backed
/// primitives): nothing here is ever individually freed.
pub fn buildEngineeredInput(alloc: std.mem.Allocator) ![]const u8 {
    zesu_allocator.set(alloc);

    const blocks = try zkevm_fixture.parseBlocks(alloc, FIXTURE_JSON);

    const decoded = try alloc.alloc(input.StatelessInput, BLOCK_INDICES.len);
    const raw_inputs = try alloc.alloc([]const u8, BLOCK_INDICES.len);
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
        raw_inputs[slot] = blk.input;
        decoded[slot] = try ssz_decode.decode(alloc, blk.input);
    }

    // Conflation-level invariants `l2_execution.zig` enforces across the whole range, checked on
    // the corpus itself (see this file's header comment for why the blocks are not altered).
    const first_ep = decoded[0].new_payload_request.execution_payload;
    for (decoded[1..], decoded[0 .. decoded.len - 1]) |si, prev| {
        const ep = si.new_payload_request.execution_payload;
        if (!std.mem.eql(u8, &ep.fee_recipient, &first_ep.fee_recipient)) return error.FixtureFeeRecipientMismatch;
        if (si.chain_config.chain_id != decoded[0].chain_config.chain_id) return error.FixtureChainIdMismatch;
        if (ep.base_fee_per_gas != first_ep.base_fee_per_gas) return error.FixtureBaseFeeNotConstant;
        if (!std.mem.eql(u8, &ep.parent_hash, &prev.new_payload_request.execution_payload.block_hash)) return error.FixtureParentHashChainBroken;
    }

    const payloads = try alloc.alloc(l2_execution_ssz.LineaPayloadInput, decoded.len);
    for (raw_inputs, 0..) |raw, slot| {
        payloads[slot] = .{ .stateless_input_ssz = raw, .forced_transactions = &.{} };
    }

    const extended_input = l2_execution_ssz.L2ExecutionProofPrivateInput{
        .parent_ftx_rolling_hash = @splat(0),
        .parent_last_processed_ftx_number = 0,
        .chain_config = .{
            .chain_id = decoded[0].chain_config.chain_id,
            .coinbase = first_ep.fee_recipient,
            .l2_message_service_address = vanilla_wrap.DUMMY_L2_MESSAGE_SERVICE_ADDRESS,
            .system_contracts = vanilla_wrap.defaultSystemContracts(decoded[0].chain_config.chain_id),
        },
        .payloads = payloads,
    };
    return l2_execution_ssz.encodeInput(alloc, extended_input);
}

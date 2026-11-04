//! Conflations of real blocks: a fixed selection of valid blocks of the EF zkevm fixtures, each
//! truncated by `encoding.truncation`, and the truncated blocks of a conflation encoded by
//! `encodeBlocks` into the canonical payload its zstd frame decompresses to; and one block with a
//! blob transaction, which truncation rejects. The selection names its blocks by fixture file,
//! case and position, so every machine reads the same blocks whatever the order its file system
//! lists the corpus in; the reference suite checks that the blocks cover what they are picked for.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;

const Allocator = std.mem.Allocator;

/// What a selected block stands for. The blocks carry no blob transaction and come from fixture
/// files of at most 8 MB, which keeps reading the selection fast.
pub const Kind = enum {
    /// No transaction.
    empty_block,
    /// A legacy transaction.
    legacy,
    /// An access-list transaction (type 1) whose access list has sixteen entries.
    access_list,
    /// A fee-market transaction (type 2).
    fee_market,
    /// A set-code transaction (type 4) with an authorization list, an access list and calldata.
    set_code,
    /// A transaction with 32 KiB of calldata.
    calldata_heavy,
    /// Twenty transactions.
    many_transactions,
    /// A set-code transaction of 161 KB whose access list has 1397 entries.
    large_lists,
    /// A transaction of 257 KiB, calldata for the most part.
    large_block,
};

/// Where a selected block is: block `block` of the case `case` in the fixture file `path`, which
/// is relative to the corpus directory.
const Pick = struct {
    path: []const u8,
    case: []const u8,
    block: usize = 0,
};

const picks: std.EnumArray(Kind, Pick) = .init(.{
    .empty_block = .{
        .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists_cross_index/bal_post_execution_calls_net_storage_at_last_index.json",
        .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists_cross_index.py::test_bal_post_execution_calls_net_storage_at_last_index[fork_Amsterdam-blockchain_test]",
    },
    .legacy = .{
        .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists_cross_index/bal_tx_change_kept_when_system_call_restores_slot.json",
        .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists_cross_index.py::test_bal_tx_change_kept_when_system_call_restores_slot[fork_Amsterdam-blockchain_test]",
    },
    .access_list = .{
        .path = "amsterdam/eip8037_state_creation_gas_cost_increase/state_gas_pricing/intrinsic_within_cap_gas_limit_above_cap.json",
        .case = "tests/amsterdam/eip8037_state_creation_gas_cost_increase/test_state_gas_pricing.py::test_intrinsic_within_cap_gas_limit_above_cap[fork_Amsterdam-blockchain_test_from_state_test]",
    },
    .fee_market = .{
        .path = "london/eip1559_fee_market_change/tx_type/eip1559_tx_validity.json",
        .case = "tests/london/eip1559_fee_market_change/test_tx_type.py::test_eip1559_tx_validity[fork_Amsterdam-valid-blockchain_test_from_state_test]",
    },
    .set_code = .{
        .path = "amsterdam/eip8037_state_creation_gas_cost_increase/state_gas_set_code/auth_with_calldata_and_access_list.json",
        .case = "tests/amsterdam/eip8037_state_creation_gas_cost_increase/test_state_gas_set_code.py::test_auth_with_calldata_and_access_list[fork_Amsterdam-blockchain_test_from_state_test]",
    },
    .calldata_heavy = .{
        .path = "amsterdam/eip7976_increase_calldata_floor_cost/additional_coverage/memory_expansion_with_calldata.json",
        .case = "tests/amsterdam/eip7976_increase_calldata_floor_cost/test_additional_coverage.py::TestMemoryExpansion::test_memory_expansion_with_calldata[fork_Amsterdam-blockchain_test_from_state_test-32kb]",
    },
    .many_transactions = .{
        .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists_eip7002/bal_7002_partial_sweep.json",
        .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists_eip7002.py::test_bal_7002_partial_sweep[fork_Amsterdam-blockchain_test]",
    },
    .large_lists = .{
        .path = "osaka/eip7825_transaction_gas_limit_cap/tx_gas_limit/tx_gas_limit_cap_authorized_tx.json",
        .case = "tests/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_tx_gas_limit_cap_authorized_tx[fork_Amsterdam-blockchain_test_from_state_test-exceed_tx_gas_limit_False-correct_intrinsic_cost_in_transaction_gas_limit_True]",
    },
    .large_block = .{
        .path = "amsterdam/eip8037_state_creation_gas_cost_increase/state_gas_calldata_floor/calldata_floor_exceeding_tx_gas_limit_cap.json",
        .case = "tests/amsterdam/eip8037_state_creation_gas_cost_increase/test_state_gas_calldata_floor.py::test_calldata_floor_exceeding_tx_gas_limit_cap[fork_Amsterdam-blockchain_test_from_state_test-at_cap]",
    },
});

/// The block with a blob transaction, which truncation rejects.
const blob_transaction_pick: Pick = .{
    .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists/bal_blob_fee_leaves_sender_only.json",
    .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists.py::test_bal_blob_fee_leaves_sender_only[fork_Amsterdam-blockchain_test_from_state_test]",
};

/// A selected block: what the fixture lists about its transactions, its RLP, the ID of its chain
/// and its truncation.
pub const Entry = struct {
    profile: ef_blocks.Profile,
    rlp: []const u8,
    chain_id: u64,
    truncated: truncation.TruncatedBlock,
};

/// The blocks of the selection.
pub const Selection = struct {
    arena: std.heap.ArenaAllocator,
    entries: std.EnumArray(Kind, Entry),
    /// A block with one blob transaction.
    blob_transaction: ef_blocks.Located,

    /// Reads the selected blocks from `corpus_dir`, a directory of fixture files, absolute or
    /// relative to the working directory.
    pub fn load(gpa: Allocator, io: std.Io, corpus_dir: []const u8) !Selection {
        var arena = std.heap.ArenaAllocator.init(gpa);
        errdefer arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(gpa);
        defer scratch.deinit();

        var entries: std.EnumArray(Kind, Entry) = undefined;
        for (std.enums.values(Kind)) |kind| {
            const pick = picks.get(kind);
            const located = ef_blocks.locate(scratch.allocator(), io, corpus_dir, pick.path, pick.case, pick.block) catch |err| {
                std.debug.print("selected block {s} of {s} is not in the corpus: {s}\n", .{ @tagName(kind), pick.path, @errorName(err) });
                return err;
            };
            const fact = located.fact;
            if (fact.blob_transaction) return error.BlobTransactionSelected;
            entries.set(kind, .{
                .profile = fact.profile,
                .rlp = try arena.allocator().dupe(u8, fact.rlp),
                .chain_id = located.chain_id,
                .truncated = try truncation.truncateBlockRlp(arena.allocator(), scratch.allocator(), fact.rlp, located.chain_id),
            });
            _ = scratch.reset(.retain_capacity);
        }
        const blob_transaction = try ef_blocks.locate(arena.allocator(), io, corpus_dir, blob_transaction_pick.path, blob_transaction_pick.case, blob_transaction_pick.block);
        return .{ .arena = arena, .entries = entries, .blob_transaction = blob_transaction };
    }

    pub fn deinit(self: *Selection) void {
        self.arena.deinit();
    }

    /// The canonical payload of the conflation of the blocks `kinds`, in that order; the caller
    /// owns it.
    pub fn conflationPayload(self: Selection, allocator: Allocator, kinds: []const Kind) ![]u8 {
        var scratch = std.heap.ArenaAllocator.init(allocator);
        defer scratch.deinit();
        const blocks = try scratch.allocator().alloc(truncation.TruncatedBlock, kinds.len);
        for (kinds, blocks) |kind, *block| block.* = self.entries.get(kind).truncated;
        return truncation.encodeBlocks(allocator, scratch.allocator(), blocks);
    }
};

/// The conflation the sample inputs of the smoke guests carry.
pub const sample = [_]Kind{ .legacy, .fee_market, .set_code };

/// The selection of `corpus_dir`, read once per process and kept.
pub fn shared(io: std.Io, corpus_dir: []const u8) !*const Selection {
    if (cached == null) cached = try Selection.load(std.heap.page_allocator, io, corpus_dir);
    return &cached.?;
}

var cached: ?Selection = null;

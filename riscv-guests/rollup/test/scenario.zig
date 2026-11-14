//! Builders of valid rollup requests for the tests and the fixture generator. Conflations of real
//! blocks of the Ethereum Foundation's zkevm fixtures and of Linea mainnet, their canonical payloads compressed against
//! the dictionary into the DA stream, the stream cut into blob and calldata chunks with the
//! commitments and proofs of Constantine, and the l2-execution proofs that tie them together are
//! computed with the test-side encoders of `lineth_da` and put into the wire types of `rollup_ssz`.

const std = @import("std");

const core_serde = @import("core_serde");
const da_support = @import("da_support");
const devmode_prover = @import("devmode_prover");
const ef_blocks = @import("ef_blocks");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const rollup = @import("rollup");
const support = @import("support");
const zstd_encoder = @import("zstd_encoder");

const blob_payload = lineth_da.encoding.blob_payload;
const da_stream = da_support.da_stream;
const l2_execution_proof = core_serde.l2_execution_proof;
const rollup_ssz = rollup.rollup_ssz;
const truncation = lineth_da.encoding.truncation;

const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;
pub const Address = [20]u8;
pub const Fact = ef_blocks.Fact;

pub const dictionary = lineth_da.dictionary_v2025_04_21;
/// The longest payload of a blob.
pub const blob_capacity = blob_payload.payload_capacity;

pub fn keccak(bytes: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &hash, .{});
    return hash;
}

fn keccakPair(left: Hash, right: Hash) Hash {
    return keccak(&(left ++ right));
}

// ── The selected blocks ──────────────────────────────────────────────────────────────────────────

/// The chain ID of the EF fixtures.
pub const ef_chain_id: u64 = 1;
/// The chain ID of Linea mainnet.
pub const linea_chain_id: u64 = 59144;

/// A block of a fixture case and what the selection claims it is, which loading checks.
const Pick = struct {
    /// The fixture file, relative to the corpus directory.
    path: []const u8,
    case: []const u8,
    block: usize = 0,
    /// Bit `n` is set for each type `n` of the block's transactions.
    types: u8,
    transactions: usize,
    access_list: bool = false,
    contract_creation: bool = false,
    authorization_list: bool = false,
};

/// Accepted blocks of at least two transactions, one for each criterion of the selection.
const ef_picks = [_]Pick{
    // Legacy transactions, one of them a contract creation.
    .{
        .path = "cancun/eip1153_tstore/tstorage_clear_after_tx/tstore_clear_after_deployment_tx.json",
        .case = "tests/cancun/eip1153_tstore/test_tstorage_clear_after_tx.py::test_tstore_clear_after_deployment_tx[fork_Amsterdam-blockchain_test]",
        .types = 0b1,
        .transactions = 2,
        .contract_creation = true,
    },
    // Fee-market transactions.
    .{
        .path = "amsterdam/eip8037_state_creation_gas_cost_increase/state_gas_multi_block/coinbase_fee_with_state_gas_refund.json",
        .case = "tests/amsterdam/eip8037_state_creation_gas_cost_increase/test_state_gas_multi_block.py::test_coinbase_fee_with_state_gas_refund[fork_Amsterdam-blockchain_test]",
        .types = 0b100,
        .transactions = 2,
    },
    // Set-code transactions.
    .{
        .path = "prague/eip7702_set_code_tx/set_code_txs/delegation_clearing_storage_readable_in_later_tx.json",
        .case = "tests/prague/eip7702_set_code_tx/test_set_code_txs.py::test_delegation_clearing_storage_readable_in_later_tx[fork_Amsterdam-blockchain_test]",
        .types = 0b10000,
        .transactions = 2,
        .authorization_list = true,
    },
    // Legacy and access-list transactions.
    .{
        .path = "amsterdam/eip7778_block_gas_accounting_without_refunds/transaction_types/trailing_tx_admission_all_transaction_types.json",
        .case = "tests/amsterdam/eip7778_block_gas_accounting_without_refunds/test_transaction_types.py::test_trailing_tx_admission_all_transaction_types[fork_Amsterdam-tx_type_1-refund_type_RefundTypes.STORAGE_CLEAR-blockchain_test-exact_block_gas_limit-typed_head_tx]",
        .types = 0b11,
        .transactions = 2,
    },
    // Legacy and fee-market transactions.
    .{
        .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists_eip7708/transfer_logs_and_bal_balance_changes.json",
        .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists_eip7708.py::test_transfer_logs_and_bal_balance_changes[fork_Amsterdam-blockchain_test]",
        .types = 0b101,
        .transactions = 2,
    },
    // Legacy and set-code transactions.
    .{
        .path = "prague/eip7702_set_code_tx/set_code_txs/authorization_reusing_nonce.json",
        .case = "tests/prague/eip7702_set_code_tx/test_set_code_txs.py::test_authorization_reusing_nonce[fork_Amsterdam-blockchain_test]",
        .types = 0b10001,
        .transactions = 2,
        .authorization_list = true,
    },
    // Access-list and set-code transactions, one with an access list entry.
    .{
        .path = "prague/eip7702_set_code_tx/set_code_txs/eoa_tx_after_set_code.json",
        .case = "tests/prague/eip7702_set_code_tx/test_set_code_txs.py::test_eoa_tx_after_set_code[fork_Amsterdam-tx_type_1-blockchain_test-same_block]",
        .types = 0b10010,
        .transactions = 2,
        .access_list = true,
        .authorization_list = true,
    },
    // Fee-market and set-code transactions.
    .{
        .path = "prague/eip7702_set_code_tx/set_code_txs/eoa_tx_after_set_code.json",
        .case = "tests/prague/eip7702_set_code_tx/test_set_code_txs.py::test_eoa_tx_after_set_code[fork_Amsterdam-tx_type_2-blockchain_test-same_block]",
        .types = 0b10100,
        .transactions = 2,
        .authorization_list = true,
    },
    // The most transactions of the corpus, in its largest block that compresses poorly.
    .{
        .path = "amsterdam/eip8037_state_creation_gas_cost_increase/state_gas_reservoir/block_2d_gas_valid_when_cumulative_exceeds_limit.json",
        .case = "tests/amsterdam/eip8037_state_creation_gas_cost_increase/test_state_gas_reservoir.py::test_block_2d_gas_valid_when_cumulative_exceeds_limit[fork_Amsterdam-blockchain_test]",
        .types = 0b1,
        .transactions = 1020,
    },
    // The largest block that compresses well.
    .{
        .path = "prague/eip6110_deposits/deposits/deposit.json",
        .case = "tests/prague/eip6110_deposits/test_deposits.py::test_deposit[fork_Amsterdam-blockchain_test-multiple_deposit_from_same_eoa_high_count]",
        .types = 0b1,
        .transactions = 200,
    },
};

/// An accepted block with a blob transaction.
const blob_transaction_pick: Pick = .{
    .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists/bal_blob_fee_leaves_sender_only.json",
    .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists.py::test_bal_blob_fee_leaves_sender_only[fork_Amsterdam-blockchain_test_from_state_test]",
    .types = 0b1000,
    .transactions = 1,
};

/// Ten consecutive accepted blocks with one EIP-155 legacy transaction each.
const chain_pick: Pick = .{
    .path = "cancun/eip4788_beacon_root/beacon_root_contract/multi_block_beacon_root_timestamp_calls.json",
    .case = "tests/cancun/eip4788_beacon_root/test_beacon_root_contract.py::test_multi_block_beacon_root_timestamp_calls[fork_Amsterdam-blockchain_test-block_count_10-buffer_wraparound]",
    .types = 0b1,
    .transactions = 1,
};
const chain_length = 10;

/// The Linea mainnet window: consecutive blocks of legacy and fee-market transactions, in the gzip
/// of one case of the shape of the EF fixtures.
const linea_case = "linea_mainnet_27434569_27434718";
const linea_blocks = 150;
const linea_transactions = 2576;

/// The blocks every request is built from.
pub const Selection = struct {
    arena: std.heap.ArenaAllocator,
    /// The blocks of `ef_picks`, in order.
    picks: []const Fact,
    /// A block with a blob transaction.
    blob_transaction: Fact,
    /// Ten consecutive blocks of the EF fixtures.
    chain: []const Fact,
    /// 150 consecutive blocks of Linea mainnet, whose canonical payloads fill more than two blobs.
    linea: []const Fact,

    /// Reads the selected blocks from `corpus_dir`, a directory of fixture files, and the Linea
    /// window.
    pub fn load(gpa: Allocator, io: std.Io, corpus_dir: []const u8) !Selection {
        var arena = std.heap.ArenaAllocator.init(gpa);
        errdefer arena.deinit();
        const a = arena.allocator();

        const picks = try a.alloc(Fact, ef_picks.len);
        for (ef_picks, picks) |pick, *slot| slot.* = try readBlock(a, io, corpus_dir, pick);
        const blob_transaction = try readBlock(a, io, corpus_dir, blob_transaction_pick);
        if (!blob_transaction.blob_transaction) return fail(error.NoBlobTransaction, "the selected blob block has no blob transaction", .{});

        const chain_case = try readCase(a, io, corpus_dir, chain_pick);
        if (chain_case.blocks.len != chain_length) return fail(error.ChainLength, "the chain case has {d} blocks, not {d}", .{ chain_case.blocks.len, chain_length });
        const chain = try a.alloc(Fact, chain_length);
        for (chain_case.blocks, chain, 0..) |block, *slot, index| {
            slot.* = block orelse return fail(error.ChainBlockNotAccepted, "block {d} of the chain case is not accepted by its fixture", .{index});
            try checkProfile(chain_pick, slot.*);
        }
        try checkChained(chain, "the chain case");

        return .{
            .arena = arena,
            .picks = picks,
            .blob_transaction = blob_transaction,
            .chain = chain,
            .linea = try readLinea(a, io),
        };
    }

    pub fn deinit(self: *Selection) void {
        self.arena.deinit();
    }
};

fn fail(err: anyerror, comptime fmt: []const u8, args: anytype) anyerror {
    std.debug.print("selected blocks: " ++ fmt ++ "\n", args);
    return err;
}

fn checkChained(blocks: []const Fact, what: []const u8) !void {
    for (blocks[1..], 1..) |block, index| {
        const previous = blocks[index - 1];
        if (block.number != previous.number + 1 or !std.mem.eql(u8, &block.parent_hash, &previous.hash)) {
            return fail(error.ChainBroken, "block {d} of {s} does not descend from block {d}", .{ index, what, index - 1 });
        }
    }
}

fn checkProfile(pick: Pick, block: Fact) !void {
    var creation = false;
    for (block.transactions) |transaction| creation = creation or transaction.to == null;
    const profile = block.profile;
    if (profile.types != pick.types or profile.transactions != pick.transactions or profile.access_list != pick.access_list or
        profile.authorization_list != pick.authorization_list or creation != pick.contract_creation)
    {
        return fail(error.PickChanged, "{s} is not the block it is selected for", .{pick.case});
    }
}

fn readCase(a: Allocator, io: std.Io, corpus_dir: []const u8, pick: Pick) !ef_blocks.Case {
    const case = ef_blocks.locateCase(a, io, corpus_dir, pick.path, pick.case) catch |err| {
        return fail(err, "{s} of {s} is not in the corpus: {s}", .{ pick.case, pick.path, @errorName(err) });
    };
    if (case.chain_id != ef_chain_id) return fail(error.UnexpectedChainId, "{s} runs on chain {d}, not {d}", .{ pick.case, case.chain_id, ef_chain_id });
    return case;
}

fn readBlock(a: Allocator, io: std.Io, corpus_dir: []const u8, pick: Pick) !Fact {
    const case = try readCase(a, io, corpus_dir, pick);
    if (pick.block >= case.blocks.len) return fail(error.BlockNotFound, "{s} has no block {d}", .{ pick.case, pick.block });
    const block = case.blocks[pick.block] orelse return fail(error.BlockNotAccepted, "block {d} of {s} is not accepted by its fixture", .{ pick.block, pick.case });
    try checkProfile(pick, block);
    return block;
}

fn readLinea(a: Allocator, io: std.Io) ![]const Fact {
    const compressed = try std.Io.Dir.cwd().readFileAlloc(io, ef_fixtures.linea_window, a, .limited(64 << 20));
    var input: std.Io.Reader = .fixed(compressed);
    var window: [std.compress.flate.max_window_len]u8 = undefined;
    var gunzip: std.compress.flate.Decompress = .init(&input, .gzip, &window);
    const text = try gunzip.reader.allocRemaining(a, .limited(1 << 30));
    const case = try ef_blocks.caseOf(a, text, ef_fixtures.linea_window, linea_case);

    if (case.chain_id != linea_chain_id) return fail(error.UnexpectedChainId, "the Linea window runs on chain {d}, not {d}", .{ case.chain_id, linea_chain_id });
    if (case.blocks.len != linea_blocks) return fail(error.LineaLength, "the Linea window has {d} blocks, not {d}", .{ case.blocks.len, linea_blocks });
    const blocks = try a.alloc(Fact, linea_blocks);
    var transactions: usize = 0;
    var types: u8 = 0;
    for (case.blocks, blocks, 0..) |block, *slot, index| {
        slot.* = block orelse return fail(error.LineaBlockNotAccepted, "block {d} of the Linea window is not accepted", .{index});
        transactions += slot.transactions.len;
        types |= slot.profile.types;
    }
    if (transactions != linea_transactions or types != 0b101) return fail(error.LineaChanged, "the Linea window holds {d} transactions of the types {b}", .{ transactions, types });
    try checkChained(blocks, "the Linea window");
    return blocks;
}

var cached: ?Selection = null;

/// The selection of the fixtures the tests run against, read on first use and kept.
pub fn selection() !*const Selection {
    if (cached == null) cached = try Selection.load(std.heap.page_allocator, std.testing.io, ef_fixtures.amsterdam_blocks);
    return &cached.?;
}

// ── Conflations of real blocks ───────────────────────────────────────────────────────────────────

/// One conflation: its full block RLPs and what the proofs and the DA stream derive from them.
pub const Conflation = struct {
    /// The chain the blocks run on.
    chain_id: u64,
    block_rlps: []const []const u8,
    /// The bytes the conflation's zstd frame decompresses to.
    payload: []const u8,
    first_number: u64,
    parent_hash: Hash,
    end_hash: Hash,
    end_timestamp: u64,
    block_hashes: []const Hash,
    /// The sender of every transaction, in block and transaction order.
    senders: []const Address,
};

/// The conflation of `blocks`: what the fixture lists, and the canonical payload the truncation of
/// the package derives from the blocks' RLPs.
pub fn conflationOf(a: Allocator, scratch: Allocator, chain_id: u64, blocks: []const Fact) !Conflation {
    const rlps = try a.alloc([]const u8, blocks.len);
    const hashes = try a.alloc(Hash, blocks.len);
    const truncated = try a.alloc(truncation.TruncatedBlock, blocks.len);
    var senders: std.ArrayList(Address) = .empty;
    for (blocks, rlps, hashes, truncated) |block, *rlp, *hash, *slot| {
        rlp.* = block.rlp;
        hash.* = block.hash;
        slot.* = try truncation.truncateBlockRlp(a, scratch, block.rlp, chain_id);
        try senders.appendSlice(a, block.senders);
    }
    return .{
        .chain_id = chain_id,
        .block_rlps = rlps,
        .payload = try truncation.encodeBlocks(a, scratch, truncated),
        .first_number = blocks[0].number,
        .parent_hash = blocks[0].parent_hash,
        .end_hash = blocks[blocks.len - 1].hash,
        .end_timestamp = blocks[blocks.len - 1].timestamp,
        .block_hashes = hashes,
        .senders = try senders.toOwnedSlice(a),
    };
}

/// The conflation of `blocks` of the chain `chain_id`.
pub fn conflationFrom(a: Allocator, chain_id: u64, blocks: []const Fact) !Conflation {
    var scratch = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer scratch.deinit();
    return conflationOf(a, scratch.allocator(), chain_id, blocks);
}

/// Consecutive conflations of `blocks`, `counts[i]` blocks in the `i`-th, from the first block.
fn splitConflations(a: Allocator, chain_id: u64, blocks: []const Fact, counts: []const usize) ![]Conflation {
    const out = try a.alloc(Conflation, counts.len);
    var start: usize = 0;
    for (counts, out) |count, *conflation| {
        conflation.* = try conflationFrom(a, chain_id, blocks[start..][0..count]);
        start += count;
    }
    return out;
}

/// Consecutive conflations of the EF chain's blocks.
pub fn chainConflations(a: Allocator, counts: []const usize) ![]Conflation {
    return splitConflations(a, ef_chain_id, (try selection()).chain, counts);
}

/// Consecutive conflations of the Linea window's blocks.
pub fn lineaConflations(a: Allocator, counts: []const usize) ![]Conflation {
    return splitConflations(a, linea_chain_id, (try selection()).linea, counts);
}

/// `next` as the proof range right behind `previous`: its first block number continues the numbers
/// of `previous` and its parent hash is the hash `previous` ends with.
pub fn follow(previous: Conflation, next: Conflation) Conflation {
    var out = next;
    out.first_number = previous.first_number + previous.block_rlps.len;
    out.parent_hash = previous.end_hash;
    return out;
}

// ── The DA stream and its chunks ─────────────────────────────────────────────────────────────────

pub const ChunkKind = enum { blob, calldata };

/// The bytes of the stream a chunk owns.
pub const Own = union(enum) {
    /// Everything the chunks before it left.
    rest,
    bytes: usize,
    /// Up to the end of the segment of this 1-based number.
    through_segment: usize,
};

/// The bytes of a blob in front of the ones it owns.
pub const Prefix = union(enum) {
    bytes: usize,
    /// As many as fill the blob's payload to its capacity.
    full,
};

/// How one chunk is cut from the stream. The owned bytes are those of the stream; a blob may also
/// hold foreign bytes, which belong to the neighbouring proofs, in front of them and, in the last
/// chunk, behind them.
pub const ChunkSpec = struct {
    kind: ChunkKind = .blob,
    own: Own = .rest,
    prefix: Prefix = .{ .bytes = 0 },
    suffix: usize = 0,
};

/// The l2-execution proof of a conflation, beyond what its blocks determine. The proof itself is a
/// devmode proof for its public input and program VK.
pub const ProofSpec = struct {
    /// Eight canonical KoalaBear limbs.
    program_vk: Hash = @splat(0x2a),
    l2_l1_messages: []const Hash = &.{},
    filtered_addresses: []const Address = &.{},
    bridge_rolling_hash: Hash = @splat(0),
    bridge_message_number: u64 = 0,
    messaging_offsets: []const u16 = &.{},
};

/// How a segment of the stream is made wrong.
pub const SegmentFault = enum {
    none,
    /// The frame decompresses to the payload with its last byte changed.
    other_payload,
    /// The frame is bytes that are no zstd frame.
    not_a_frame,
};

pub const Options = struct {
    /// Every frame holds its payload in raw blocks, as a compressor stores data it cannot shrink,
    /// which makes the stream as long as the payloads.
    stored_frames: bool = false,
    parent_data_rolling_hash: Hash = @splat(0x47),
    /// The dataRollingHash before the first chunk, which a range starting inside that chunk opens.
    boundary_prev_data_rolling_hash: Hash = @splat(0x39),
    dynamic_chain_config_hash: Hash = @splat(0xc0),
    /// One blob chunk owning the whole stream by default.
    chunks: []const ChunkSpec = &.{.{}},
    /// The proofs of the conflations in order; a missing one is the default `ProofSpec`.
    proofs: []const ProofSpec = &.{},
    /// The faults of the conflations' segments in order; a missing one is `none`.
    segment_faults: []const SegmentFault = &.{},
};

/// A valid rollup request. Its slices are the arena's and may be edited before `input` is taken.
pub const Request = struct {
    chain_id: u64,
    parent_data_rolling_hash: Hash,
    parent_data_tail_take_bytes: u64,
    boundary_prev_data_rolling_hash: ?Hash,
    conflations: []rollup_ssz.ConflationWitness,
    chunks: []rollup_ssz.ChunkWitness,
    proofs: []l2_execution_proof.VerifiableL2ExecutionProof,
    /// The segments of the stream: each conflation's frame behind its length.
    segments: []const []const u8,
    /// The dataRollingHash after the last chunk.
    end_data_rolling_hash: Hash,
    /// The bytes of the last blob behind the last segment.
    final_data_tail_discard_bytes: u64,

    /// The input over this request's slices, which it must outlive.
    pub fn input(self: *const Request) rollup_ssz.RollupProofPrivateInput {
        return .{
            .parent_data_rolling_hash = self.parent_data_rolling_hash,
            .parent_data_tail_take_bytes = self.parent_data_tail_take_bytes,
            .chain_id = self.chain_id,
            .conflations = .{ .items = self.conflations },
            .chunks = .{ .items = self.chunks },
            .l2_execution_proofs = .{ .items = self.proofs },
            .boundary_prev_data_rolling_hash = .{ .items = if (self.boundary_prev_data_rolling_hash) |*hash| @as(*const [1]Hash, @ptrCast(hash)) else &.{} },
        };
    }
};

/// The request proving `conflations`: their canonical payloads compressed into one segment each,
/// the segments cut into chunks as `options.chunks` says, and one l2-execution proof per conflation.
/// The request is for the chain of the first conflation.
pub fn build(a: Allocator, conflations_in: []const Conflation, options: Options) !Request {
    const encoder = try zstd_encoder.Encoder.init(a, dictionary);
    defer encoder.deinit();

    const segments = try a.alloc([]const u8, conflations_in.len);
    const segment_ends = try a.alloc(usize, conflations_in.len);
    var stream_len: usize = 0;
    for (conflations_in, segments, segment_ends, 0..) |conflation, *segment, *segment_end, index| {
        const fault: SegmentFault = if (index < options.segment_faults.len) options.segment_faults[index] else .none;
        const frame = switch (fault) {
            .none => if (options.stored_frames) try storedFrame(a, conflation.payload) else try encoder.compress(a, conflation.payload),
            .other_payload => blk: {
                const other = try a.dupe(u8, conflation.payload);
                other[other.len - 1] ^= 0x01;
                break :blk try encoder.compress(a, other);
            },
            .not_a_frame => try a.dupe(u8, "these bytes are not a zstd frame"),
        };
        segment.* = try da_stream.lengthPrefixed(a, frame);
        stream_len += segment.len;
        segment_end.* = stream_len;
    }
    const stream = try std.mem.concat(a, u8, segments);

    const chunks = try a.alloc(rollup_ssz.ChunkWitness, options.chunks.len);
    var prng = std.Random.DefaultPrng.init(0x7a11);
    var position: usize = 0;
    var take: u64 = 0;
    var discard: u64 = 0;
    for (options.chunks, chunks, 0..) |spec, *chunk, index| {
        const owned = switch (spec.own) {
            .rest => stream.len - position,
            .bytes => |len| len,
            .through_segment => |number| segment_ends[number - 1] - position,
        };
        const owned_bytes = stream[position..][0..owned];
        position += owned;
        switch (spec.kind) {
            .calldata => chunk.* = .{ .chunk_hash = keccak(owned_bytes), .is_calldata = .{ .value = true }, .calldata_bytes = .{ .bytes = owned_bytes } },
            .blob => {
                const prefix_len = switch (spec.prefix) {
                    .bytes => |len| len,
                    .full => blob_capacity - owned - spec.suffix,
                };
                const payload = try a.alloc(u8, prefix_len + owned + spec.suffix);
                prng.random().bytes(payload[0..prefix_len]);
                @memcpy(payload[prefix_len..][0..owned], owned_bytes);
                prng.random().bytes(payload[prefix_len + owned ..]);
                chunk.* = try blobChunk(a, payload);
                if (index == 0 and prefix_len > 0) take = owned + spec.suffix;
                if (index == options.chunks.len - 1) discard = spec.suffix;
            },
        }
    }

    const chunk_hashes = try a.alloc(Hash, chunks.len);
    for (chunks, chunk_hashes) |chunk, *hash| hash.* = chunk.chunk_hash;
    var parent_data_rolling_hash = options.parent_data_rolling_hash;
    var end_data_rolling_hash = parent_data_rolling_hash;
    var folded: []const Hash = chunk_hashes;
    if (take > 0) {
        parent_data_rolling_hash = keccakPair(options.boundary_prev_data_rolling_hash, chunk_hashes[0]);
        end_data_rolling_hash = parent_data_rolling_hash;
        folded = chunk_hashes[1..];
    }
    for (folded) |chunk_hash| end_data_rolling_hash = keccakPair(end_data_rolling_hash, chunk_hash);

    const witnesses = try a.alloc(rollup_ssz.ConflationWitness, conflations_in.len);
    const proofs = try a.alloc(l2_execution_proof.VerifiableL2ExecutionProof, conflations_in.len);
    for (conflations_in, witnesses, proofs, 0..) |conflation, *witness, *proof, index| {
        witness.* = try support.conflationOf(a, conflation.block_rlps);
        proof.* = proofOf(conflation, index, if (index < options.proofs.len) options.proofs[index] else .{}, options.dynamic_chain_config_hash);
    }
    try prove(a, proofs);

    return .{
        .chain_id = conflations_in[0].chain_id,
        .parent_data_rolling_hash = parent_data_rolling_hash,
        .parent_data_tail_take_bytes = take,
        .boundary_prev_data_rolling_hash = if (take > 0) options.boundary_prev_data_rolling_hash else null,
        .conflations = witnesses,
        .chunks = chunks,
        .proofs = proofs,
        .segments = segments,
        .end_data_rolling_hash = end_data_rolling_hash,
        .final_data_tail_discard_bytes = discard,
    };
}

const zstd_magic_number: u32 = 0xFD2FB528;

/// A zstd frame of one segment holding `payload` in raw blocks, which declares the dictionary of the
/// stream and the payload's length as the frames of the encoder do.
fn storedFrame(a: Allocator, payload: []const u8) ![]u8 {
    const max_block = 1 << 17;
    const dictionary_id = dictionary[4..8];
    var frame: std.ArrayList(u8) = .empty;
    try frame.appendSlice(a, &std.mem.toBytes(std.mem.nativeToLittle(u32, zstd_magic_number)));
    // A single segment with a 4-byte content size and a 4-byte dictionary ID.
    try frame.append(a, 0b10_1_0_0_0_11);
    try frame.appendSlice(a, dictionary_id);
    try frame.appendSlice(a, &std.mem.toBytes(std.mem.nativeToLittle(u32, @intCast(payload.len))));
    var rest = payload;
    while (true) {
        const block = rest[0..@min(rest.len, max_block)];
        rest = rest[block.len..];
        const last: u32 = @intFromBool(rest.len == 0);
        // The block header: last-block flag, block type 0 (raw), 21-bit size.
        const header = last | @as(u32, @intCast(block.len)) << 3;
        try frame.appendSlice(a, &std.mem.toBytes(std.mem.nativeToLittle(u32, header))[0..3].*);
        try frame.appendSlice(a, block);
        if (last == 1) return frame.toOwnedSlice(a);
    }
}

/// A blob chunk packing `payload`, anchored by the versioned hash of its commitment, which comes
/// with the blob proof.
pub fn blobChunk(a: Allocator, payload: []const u8) !rollup_ssz.ChunkWitness {
    const blob = try a.create([blob_payload.blob_len]u8);
    try blob_payload.pack(payload, blob);
    const opening = da_support.kzg.open(blob);
    const commitment = try a.dupe(u8, &opening.commitment);
    return .{
        .chunk_hash = da_stream.versionedHash(&opening.commitment),
        .is_calldata = .{ .value = false },
        .blob_bytes = .{ .bytes = blob },
        .kzg_commitment = .{ .bytes = commitment },
        .kzg_proof = .{ .bytes = try a.dupe(u8, &opening.proof) },
    };
}

/// The forced-transaction rolling hash `index` proofs into the chain: the proof at `index` starts from
/// `ftxHash(index)` and ends at `ftxHash(index + 1)`.
pub fn ftxHash(index: usize) Hash {
    var seed: [11]u8 = "ftx-rolling".*;
    seed[10] = @truncate(index);
    return keccak(&seed);
}

fn proofOf(conflation: Conflation, index: usize, spec: ProofSpec, chain_config_hash: Hash) l2_execution_proof.VerifiableL2ExecutionProof {
    const senders = std.mem.sliceAsBytes(conflation.senders);
    return .{
        .program_vk = spec.program_vk,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = conflation.parent_hash,
                .end_block_hash = conflation.end_hash,
                .end_block_number = conflation.first_number + conflation.block_rlps.len - 1,
                .end_block_timestamp = conflation.end_timestamp,
                .l2_l1_messages = .{ .items = spec.l2_l1_messages },
                .end_l1_l2_bridge_rolling_hash = spec.bridge_rolling_hash,
                .end_l1_l2_bridge_rolling_hash_message_number = spec.bridge_message_number,
                .dynamic_chain_config_hash = chain_config_hash,
                .parent_ftx_rolling_hash = ftxHash(index),
                .parent_ftx_number = 10 + 2 * index,
                .end_ftx_rolling_hash = ftxHash(index + 1),
                .end_processed_ftx_number = 12 + 2 * index,
                .filtered_addresses = .{ .items = spec.filtered_addresses },
                .tx_froms_hash = keccak(senders),
                .block_count = conflation.block_rlps.len,
                .l2_messaging_blocks_offsets = .{ .items = spec.messaging_offsets },
            },
            .start_block_number = conflation.first_number,
            .proof = .{ .bytes = &.{} },
        },
    };
}

/// Replaces the proof of every entry of `proofs` with one that is valid for its program VK and its
/// public input as they are now, which an edit of either leaves unproved. An entry whose public
/// input is beyond a bound of its wire schema has no hash to prove and is left as it is.
pub fn prove(a: Allocator, proofs: []l2_execution_proof.VerifiableL2ExecutionProof) !void {
    for (proofs) |*verifiable| {
        const hash = l2_execution_proof.hashPublicInputs(a, verifiable.proof.public_inputs) catch |err| switch (err) {
            error.OutOfMemory => return error.OutOfMemory,
            error.BoundsViolation => continue,
        };
        verifiable.proof.proof = .{ .bytes = try devmode_prover.prove(a, verifiable.program_vk, hash) };
    }
}

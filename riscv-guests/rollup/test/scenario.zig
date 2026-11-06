//! Builders of valid rollup requests for the tests and the fixture generator. Conflations of real
//! blocks of the Ethereum Foundation's zkevm fixtures, their canonical payloads compressed against
//! the dictionary into the DA stream, the stream cut into blob and calldata chunks with the
//! commitments and proofs of Constantine, and the l2-execution proofs that tie them together are
//! computed with the test-side encoders of `lineth_da` and put into the wire types of `rollup_ssz`.

const std = @import("std");

const core_serde = @import("core_serde");
const da_support = @import("da_support");
const ef_blocks = @import("ef_blocks");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const rollup = @import("rollup");
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
/// The chain the selected blocks run on.
pub const selected_chain_id: u64 = 1;

pub fn keccak(bytes: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &hash, .{});
    return hash;
}

fn keccakPair(left: Hash, right: Hash) Hash {
    return keccak(&(left ++ right));
}

// ── The selected blocks ──────────────────────────────────────────────────────────────────────────

const Pick = struct {
    /// The fixture file, relative to the corpus directory.
    path: []const u8,
    case: []const u8,
};

const chain_pick: Pick = .{
    .path = "cancun/eip4788_beacon_root/beacon_root_contract/multi_block_beacon_root_timestamp_calls.json",
    .case = "tests/cancun/eip4788_beacon_root/test_beacon_root_contract.py::test_multi_block_beacon_root_timestamp_calls[fork_Amsterdam-blockchain_test-block_count_10-buffer_wraparound]",
};

const blob_transaction_pick: Pick = .{
    .path = "amsterdam/eip7928_block_level_access_lists/block_access_lists/bal_blob_fee_leaves_sender_only.json",
    .case = "tests/amsterdam/eip7928_block_level_access_lists/test_block_access_lists.py::test_bal_blob_fee_leaves_sender_only[fork_Amsterdam-blockchain_test_from_state_test]",
};

const large_block_pick: Pick = .{
    .path = "osaka/eip7825_transaction_gas_limit_cap/tx_gas_limit/tx_gas_limit_cap_authorized_tx.json",
    .case = "tests/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_tx_gas_limit_cap_authorized_tx[fork_Amsterdam-blockchain_test_from_state_test-exceed_tx_gas_limit_False-correct_intrinsic_cost_in_transaction_gas_limit_True]",
};

/// The blocks every request is built from.
pub const Selection = struct {
    arena: std.heap.ArenaAllocator,
    /// Ten consecutive blocks with one EIP-155 legacy transaction each.
    chain: []const Fact,
    /// A block with a blob transaction.
    blob_transaction: Fact,
    /// A block of a set-code transaction of 161 KB whose access list has 1397 entries; its frame is
    /// longer than 20 KB and shorter than a blob.
    large_block: Fact,

    /// Reads the selected blocks from `corpus_dir`, a directory of fixture files.
    pub fn load(gpa: Allocator, io: std.Io, corpus_dir: []const u8) !Selection {
        var arena = std.heap.ArenaAllocator.init(gpa);
        errdefer arena.deinit();
        const a = arena.allocator();

        const chain_case = try readCase(a, io, corpus_dir, chain_pick);
        const chain = try a.alloc(Fact, chain_case.blocks.len);
        for (chain_case.blocks, chain, 0..) |block, *slot, index| {
            slot.* = block orelse return fail(error.ChainBlockNotAccepted, "block {d} of the chain case is not accepted by its fixture", .{index});
            if (slot.transactions.len == 0) return fail(error.ChainBlockWithoutTransaction, "block {d} of the chain case has no transaction", .{index});
            if (index > 0 and !std.mem.eql(u8, &slot.parent_hash, &chain[index - 1].hash)) {
                return fail(error.ChainBroken, "block {d} of the chain case does not descend from block {d}", .{ index, index - 1 });
            }
        }
        if (chain.len != 10) return fail(error.ChainLength, "the chain case has {d} blocks, not 10", .{chain.len});

        const blob_transaction = try readBlock(a, io, corpus_dir, blob_transaction_pick);
        if (!blob_transaction.blob_transaction) return fail(error.NoBlobTransaction, "the selected blob block has no blob transaction", .{});
        const large_block = try readBlock(a, io, corpus_dir, large_block_pick);
        return .{ .arena = arena, .chain = chain, .blob_transaction = blob_transaction, .large_block = large_block };
    }

    pub fn deinit(self: *Selection) void {
        self.arena.deinit();
    }
};

fn fail(err: anyerror, comptime fmt: []const u8, args: anytype) anyerror {
    std.debug.print("selected blocks: " ++ fmt ++ "\n", args);
    return err;
}

fn readCase(a: Allocator, io: std.Io, corpus_dir: []const u8, pick: Pick) !ef_blocks.Case {
    const case = ef_blocks.locateCase(a, io, corpus_dir, pick.path, pick.case) catch |err| {
        return fail(err, "{s} of {s} is not in the corpus: {s}", .{ pick.case, pick.path, @errorName(err) });
    };
    if (case.chain_id != selected_chain_id) return fail(error.UnexpectedChainId, "{s} runs on chain {d}, not {d}", .{ pick.case, case.chain_id, selected_chain_id });
    return case;
}

fn readBlock(a: Allocator, io: std.Io, corpus_dir: []const u8, pick: Pick) !Fact {
    const case = try readCase(a, io, corpus_dir, pick);
    if (case.blocks.len == 0) return fail(error.BlockNotFound, "{s} has no block", .{pick.case});
    return case.blocks[0] orelse return fail(error.BlockNotAccepted, "block 0 of {s} is not accepted by its fixture", .{pick.case});
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

/// The conflation of `blocks` on the selection's chain.
pub fn conflationFrom(a: Allocator, blocks: []const Fact) !Conflation {
    var scratch = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer scratch.deinit();
    return conflationOf(a, scratch.allocator(), selected_chain_id, blocks);
}

/// Consecutive conflations of the chain's blocks, `counts[i]` blocks in the `i`-th, from block 0.
pub fn chainConflations(a: Allocator, counts: []const usize) ![]Conflation {
    const chain = (try selection()).chain;
    const out = try a.alloc(Conflation, counts.len);
    var start: usize = 0;
    for (counts, out) |count, *conflation| {
        conflation.* = try conflationFrom(a, chain[start..][0..count]);
        start += count;
    }
    return out;
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

/// The l2-execution proof of a conflation, beyond what its blocks determine.
pub const ProofSpec = struct {
    program_vk: Hash = @splat(0xaa),
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
    chain_id: u64 = selected_chain_id,
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

    pub fn input(self: Request) rollup_ssz.RollupProofPrivateInput {
        return .{
            .parent_data_rolling_hash = self.parent_data_rolling_hash,
            .parent_data_tail_take_bytes = self.parent_data_tail_take_bytes,
            .chain_id = self.chain_id,
            .conflations = self.conflations,
            .chunks = self.chunks,
            .l2_execution_proofs = self.proofs,
            .boundary_prev_data_rolling_hash = self.boundary_prev_data_rolling_hash,
        };
    }
};

/// The request proving `conflations`: their canonical payloads compressed into one segment each,
/// the segments cut into chunks as `options.chunks` says, and one l2-execution proof per conflation.
pub fn build(a: Allocator, conflations_in: []const Conflation, options: Options) !Request {
    const encoder = try zstd_encoder.Encoder.init(a, dictionary);
    defer encoder.deinit();

    const segments = try a.alloc([]const u8, conflations_in.len);
    const segment_ends = try a.alloc(usize, conflations_in.len);
    var stream_len: usize = 0;
    for (conflations_in, segments, segment_ends, 0..) |conflation, *segment, *segment_end, index| {
        const fault: SegmentFault = if (index < options.segment_faults.len) options.segment_faults[index] else .none;
        const frame = switch (fault) {
            .none => try encoder.compress(a, conflation.payload),
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
            .calldata => chunk.* = .{ .chunk_hash = keccak(owned_bytes), .is_calldata = true, .calldata_bytes = owned_bytes },
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
        witness.* = .{ .block_rlps = conflation.block_rlps };
        proof.* = try proofOf(a, conflation, index, if (index < options.proofs.len) options.proofs[index] else .{}, options.dynamic_chain_config_hash);
    }

    return .{
        .chain_id = options.chain_id,
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

/// A blob chunk packing `payload`, anchored by the versioned hash of its commitment, which comes
/// with the blob proof.
pub fn blobChunk(a: Allocator, payload: []const u8) !rollup_ssz.ChunkWitness {
    const blob = try a.create([blob_payload.blob_len]u8);
    try blob_payload.pack(payload, blob);
    const opening = da_support.kzg.open(blob);
    const commitment = try a.dupe(u8, &opening.commitment);
    return .{
        .chunk_hash = da_stream.versionedHash(&opening.commitment),
        .is_calldata = false,
        .blob_bytes = blob,
        .kzg_commitment = commitment,
        .kzg_proof = try a.dupe(u8, &opening.proof),
    };
}

/// The forced-transaction rolling hash `index` proofs into the chain: the proof at `index` starts from
/// `ftxHash(index)` and ends at `ftxHash(index + 1)`.
pub fn ftxHash(index: usize) Hash {
    var seed: [11]u8 = "ftx-rolling".*;
    seed[10] = @truncate(index);
    return keccak(&seed);
}

fn proofOf(a: Allocator, conflation: Conflation, index: usize, spec: ProofSpec, chain_config_hash: Hash) !l2_execution_proof.VerifiableL2ExecutionProof {
    const senders = std.mem.sliceAsBytes(conflation.senders);
    return .{
        .program_vk = spec.program_vk,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = conflation.parent_hash,
                .end_block_hash = conflation.end_hash,
                .end_block_number = conflation.first_number + conflation.block_rlps.len - 1,
                .end_block_timestamp = conflation.end_timestamp,
                .l2_l1_messages = spec.l2_l1_messages,
                .end_l1_l2_bridge_rolling_hash = spec.bridge_rolling_hash,
                .end_l1_l2_bridge_rolling_hash_message_number = spec.bridge_message_number,
                .dynamic_chain_config_hash = chain_config_hash,
                .parent_ftx_rolling_hash = ftxHash(index),
                .parent_ftx_number = 10 + 2 * index,
                .end_ftx_rolling_hash = ftxHash(index + 1),
                .end_processed_ftx_number = 12 + 2 * index,
                .filtered_addresses = spec.filtered_addresses,
                .tx_froms_hash = keccak(senders),
                .block_count = conflation.block_rlps.len,
                .l2_messaging_blocks_offsets = spec.messaging_offsets,
            },
            .start_block_number = conflation.first_number,
            .proof = try a.dupe(u8, &.{ 0xab, 0xcd, @as(u8, @truncate(index)) }),
        },
    };
}

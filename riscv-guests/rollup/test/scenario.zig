//! Builders of valid rollup requests for the tests and the fixture generator. Conflations of signed
//! blocks, their canonical payloads compressed against the dictionary into the DA stream, the stream
//! cut into blob and calldata chunks with the commitments and proofs of c-kzg-4844, and the
//! l2-execution proofs that tie them together are all computed with the test-side encoders of
//! `lineth_da` and put into the wire types of `rollup_ssz`.

const std = @import("std");

const core_serde = @import("core_serde");
const lineth_da = @import("lineth_da");
const rollup = @import("rollup");
const da_support = @import("da_support");
const zstd_compressor = @import("zstd_compressor");

const blocks = da_support.blocks;
const da_stream = da_support.da_stream;
const rlp = da_support.rlp;
const secp256k1 = da_support.secp256k1;
const blob_payload = lineth_da.encoding.blob_payload;
const l2_execution_proof = core_serde.l2_execution_proof;
const rollup_ssz = rollup.rollup_ssz;

const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;
pub const Address = [20]u8;

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

// ── Conflations of signed blocks ─────────────────────────────────────────────────────────────────

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

pub const ChainOptions = struct {
    chain_id: u64 = 59144,
    first_number: u64 = 100,
    /// The parent hash of the first block.
    parent_hash: Hash = @splat(0x01),
    seed: u64 = 2026,
    /// The kinds the transactions are drawn from.
    kinds: []const blocks.Kind = std.enums.values(blocks.Kind),
};

pub const ConflationSpec = struct {
    blocks: usize = 1,
    /// Random bytes in the data of the first transaction of the conflation. They do not compress, so
    /// they make the payload and its frame at least that long.
    bulk_bytes: usize = 0,
    /// The block whose parent hash is not the hash of the block before it.
    detached_block: ?usize = null,
};

fn secretKey(index: usize) [32]u8 {
    var secret: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(&.{ 'k', 'e', 'y', @intCast(index) }, &secret, .{});
    return secret;
}

/// Consecutive conflations of random signed transactions, one per spec, each block's parent hash the
/// hash of the block before it.
pub fn conflations(a: Allocator, chain: ChainOptions, specs: []const ConflationSpec) ![]Conflation {
    var prng = std.Random.DefaultPrng.init(chain.seed);
    const random = prng.random();
    const kinds = chain.kinds;

    const out = try a.alloc(Conflation, specs.len);
    var number = chain.first_number;
    var parent_hash = chain.parent_hash;
    for (specs, out) |spec, *conflation| {
        const rlps = try a.alloc([]const u8, spec.blocks);
        const hashes = try a.alloc(Hash, spec.blocks);
        const entries = try a.alloc(rlp.Item, spec.blocks);
        var senders: std.ArrayList(Address) = .empty;
        const first_parent_hash = parent_hash;
        var timestamp: u64 = 0;

        for (rlps, hashes, entries, 0..) |*block_rlp, *block_hash, *entry, index| {
            const tx_count = 1 + random.uintLessThan(usize, 3);
            const txs = try a.alloc(blocks.Tx, tx_count);
            const items = try a.alloc(rlp.Item, tx_count);
            const stripped_items = try a.alloc(rlp.Item, tx_count);
            const sender_items = try a.alloc(rlp.Item, tx_count);
            for (txs, items, stripped_items, sender_items, 0..) |*tx, *item, *stripped_item, *sender_item, tx_index| {
                tx.* = try blocks.randomTx(a, random, kinds[random.uintLessThan(usize, kinds.len)]);
                if (index == 0 and tx_index == 0 and spec.bulk_bytes > 0) {
                    const data = try a.alloc(u8, spec.bulk_bytes);
                    random.bytes(data);
                    tx.data = data;
                }
                const secret = secretKey(random.uintLessThan(usize, 4));
                item.* = try blocks.signed(a, tx.*, chain.chain_id, secret);
                stripped_item.* = rlp.string(try blocks.stripped(a, tx.*));
                const sender = try a.create(Address);
                sender.* = secp256k1.addressOf(secret);
                sender_item.* = rlp.string(sender);
                try senders.append(a, sender.*);
            }

            timestamp = 1_700_000_000 + 12 * number;
            const detached = spec.detached_block == index;
            const header = try blocks.withTransactions(a, .{
                .parent_hash = if (detached) blocks.randomBytes(Hash, random) else parent_hash,
                .prev_randao = blocks.randomBytes(Hash, random),
                .number = number,
                .timestamp = timestamp,
            }, items);
            block_rlp.* = try blocks.block(a, header, items, &.{});
            block_hash.* = try blocks.blockHash(a, header);
            entry.* = rlp.list(try a.dupe(rlp.Item, &.{
                try rlp.int(a, header.timestamp),
                rlp.string(block_hash),
                rlp.string(try a.dupe(u8, &header.prev_randao)),
                rlp.list(stripped_items),
                rlp.list(sender_items),
            }));
            parent_hash = block_hash.*;
            number += 1;
        }

        conflation.* = .{
            .block_rlps = rlps,
            .payload = try rlp.encode(a, rlp.list(entries)),
            .first_number = number - spec.blocks,
            .parent_hash = first_parent_hash,
            .end_hash = parent_hash,
            .end_timestamp = timestamp,
            .block_hashes = hashes,
            .senders = try senders.toOwnedSlice(a),
        };
    }
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
    chain_id: u64 = 59144,
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
    const segments = try a.alloc([]const u8, conflations_in.len);
    const segment_ends = try a.alloc(usize, conflations_in.len);
    var stream_len: usize = 0;
    for (conflations_in, segments, segment_ends, 0..) |conflation, *segment, *segment_end, index| {
        const fault: SegmentFault = if (index < options.segment_faults.len) options.segment_faults[index] else .none;
        const frame = switch (fault) {
            .none => try zstd_compressor.compress(a, conflation.payload, .{ .dictionary = dictionary }),
            .other_payload => blk: {
                const other = try a.dupe(u8, conflation.payload);
                other[other.len - 1] ^= 0x01;
                break :blk try zstd_compressor.compress(a, other, .{ .dictionary = dictionary });
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

//! The DA stream: segments `[4-byte big-endian length][frame]` appended in order and packed into
//! blobs of `payload_capacity` bytes, each sealed with its KZG commitment, proof, versioned hash and
//! data rolling hash. A blob is sealed lazily: a full blob stays open until a segment needs room or
//! the stream is flushed. A stream resumes inside the blob its last rollup shared with the next one,
//! and the bytes appended after it have to reproduce that blob.
//!
//! One thread at a time may use a stream.

const std = @import("std");

const lineth_da = @import("lineth_da");
const kzg_prover = @import("kzg_prover");

const blob_kzg = lineth_da.encoding.blob_kzg;
const blob_payload = lineth_da.encoding.blob_payload;
const rolling_hash = lineth_da.encoding.rolling_hash;
const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;
pub const blob_len = blob_payload.blob_len;
pub const payload_capacity = blob_payload.payload_capacity;
pub const length_prefix_len = 4;
/// The longest frame whose segment length fits the four-byte prefix of the stream.
pub const max_frame_len = std.math.maxInt(u32) - length_prefix_len;

pub const Error = kzg_prover.Error || error{
    OutOfMemory,
    /// The frame has no bytes.
    EmptyFrame,
    /// The segment of the frame is longer than the prefix can state.
    FrameTooLong,
    /// The position has a carry-over tail and no shared blob.
    MissingSharedBlob,
    /// The shared blob is not the canonical packing of a payload.
    NonCanonicalSharedBlob,
    /// The carry-over tail is not shorter than the payload of the shared blob.
    TailTooLong,
    /// The shared blob does not fold from the parent hash to the data rolling hash.
    SharedBlobHashMismatch,
    /// The appended bytes differ from the shared blob the stream resumed inside.
    SeededBlobMismatch,
    /// The shared blob has bytes left to reproduce.
    FlushWhileReproducing,
};

/// The end of the last proven rollup. With a zero tail the stream starts on a fresh blob after
/// `data_rolling_hash`; otherwise the rollup ended inside `shared_blob`, whose last tail bytes of
/// payload carry over to the next rollup, and `parent_blob_data_rolling_hash` is the hash before
/// that blob.
pub const Position = extern struct {
    data_rolling_hash: Hash,
    parent_blob_data_rolling_hash: Hash,
    shared_blob_carry_over_tail_bytes: u32,
    shared_blob: ?*const [blob_len]u8,

    pub fn fresh(data_rolling_hash: Hash) Position {
        return .{
            .data_rolling_hash = data_rolling_hash,
            .parent_blob_data_rolling_hash = data_rolling_hash,
            .shared_blob_carry_over_tail_bytes = 0,
            .shared_blob = null,
        };
    }
};

/// Where the segment of a conflation ends inside a blob, followed by `unused_tail_bytes` of payload.
pub const SegmentEnd = extern struct {
    conflation_end_block: u64,
    unused_tail_bytes: u32,
};

/// A sealed blob. Its `segment_ends` belong to the allocator that made it.
pub const SealedBlob = extern struct {
    blob: [blob_len]u8,
    chunk_hash: Hash,
    kzg_commitment: [kzg_prover.point_len]u8,
    kzg_proof: [kzg_prover.point_len]u8,
    prev_data_rolling_hash: Hash,
    data_rolling_hash: Hash,
    payload_bytes: u32,
    segment_end_count: u32,
    segment_ends: ?[*]const SegmentEnd,

    pub fn segmentEnds(self: *const SealedBlob) []const SegmentEnd {
        return if (self.segment_ends) |ends| ends[0..self.segment_end_count] else &.{};
    }
};

/// Releases blobs a stream returned.
pub fn freeSealed(allocator: Allocator, blobs: []const SealedBlob) void {
    for (blobs) |*blob| allocator.free(blob.segmentEnds());
    allocator.free(blobs);
}

const PendingEnd = struct {
    conflation_end_block: u64,
    offset: u32,
};

/// The blob being filled.
const OpenBlob = struct {
    /// Room for the longest payload. The first `used` bytes are the payload so far; a blob that
    /// reproduces a shared blob holds all `capacity` bytes of the payload it has to reach.
    payload: *[blob_payload.stream_len]u8,
    /// The payload bytes the blob takes: `payload_capacity`, or less for a shared blob that a flush ended early.
    capacity: usize,
    used: usize,
    prev_data_rolling_hash: Hash,
    ends: std.ArrayList(PendingEnd),
    /// The commitment of the shared blob this blob reproduces.
    shared_commitment: ?[kzg_prover.point_len]u8,

    fn create(allocator: Allocator, prev_data_rolling_hash: Hash) Allocator.Error!OpenBlob {
        return .{
            .payload = try allocator.create([blob_payload.stream_len]u8),
            .capacity = payload_capacity,
            .used = 0,
            .prev_data_rolling_hash = prev_data_rolling_hash,
            .ends = .empty,
            .shared_commitment = null,
        };
    }

    fn clone(self: OpenBlob, allocator: Allocator) Allocator.Error!OpenBlob {
        var copy = try create(allocator, self.prev_data_rolling_hash);
        errdefer copy.deinit(allocator);
        const filled = if (self.shared_commitment != null) self.capacity else self.used;
        @memcpy(copy.payload[0..filled], self.payload[0..filled]);
        copy.capacity = self.capacity;
        copy.used = self.used;
        copy.shared_commitment = self.shared_commitment;
        try copy.ends.appendSlice(allocator, self.ends.items);
        return copy;
    }

    fn deinit(self: *OpenBlob, allocator: Allocator) void {
        allocator.destroy(self.payload);
        self.ends.deinit(allocator);
    }

    /// Starts the blob after `prev_data_rolling_hash`, keeping the room of `ends`.
    fn restart(self: *OpenBlob, prev_data_rolling_hash: Hash) void {
        self.capacity = payload_capacity;
        self.used = 0;
        self.prev_data_rolling_hash = prev_data_rolling_hash;
        self.ends.clearRetainingCapacity();
        self.shared_commitment = null;
    }

    fn isReproducing(self: OpenBlob) bool {
        return self.shared_commitment != null and self.used < self.capacity;
    }
};

pub const Stream = struct {
    allocator: Allocator,
    /// The data rolling hash after the last sealed blob.
    data_rolling_hash: Hash,
    current: ?OpenBlob,

    /// Opens the stream at `position`. A position inside a shared blob costs a KZG commitment.
    pub fn open(allocator: Allocator, position: *const Position) Error!Stream {
        const carry: usize = position.shared_blob_carry_over_tail_bytes;
        if (carry == 0) return .{ .allocator = allocator, .data_rolling_hash = position.data_rolling_hash, .current = null };

        const shared_blob = position.shared_blob orelse return error.MissingSharedBlob;
        var blob = try OpenBlob.create(allocator, position.parent_blob_data_rolling_hash);
        errdefer blob.deinit(allocator);
        const payload = blob_payload.unpack(shared_blob, blob.payload) catch return error.NonCanonicalSharedBlob;
        if (carry >= payload.len) return error.TailTooLong;
        const commitment = try kzg_prover.commit(shared_blob);
        const next = rolling_hash.next(position.parent_blob_data_rolling_hash, blob_kzg.versionedHash(&commitment));
        if (!std.mem.eql(u8, &next, &position.data_rolling_hash)) return error.SharedBlobHashMismatch;

        blob.capacity = payload.len;
        blob.used = payload.len - carry;
        blob.shared_commitment = commitment;
        return .{ .allocator = allocator, .data_rolling_hash = position.data_rolling_hash, .current = blob };
    }

    pub fn deinit(self: *Stream) void {
        if (self.current) |*blob| blob.deinit(self.allocator);
    }

    /// Appends the segment of a conflation whose last block is `conflation_end_block` and returns
    /// the blobs it sealed, which the caller releases with `freeSealed`. A failed append leaves the
    /// stream as it was.
    pub fn append(self: *Stream, frame: []const u8, conflation_end_block: u64) Error![]SealedBlob {
        if (frame.len == 0) return error.EmptyFrame;
        if (frame.len > max_frame_len) return error.FrameTooLong;
        const segment = try self.allocator.alloc(u8, length_prefix_len + frame.len);
        defer self.allocator.free(segment);
        std.mem.writeInt(u32, segment[0..length_prefix_len], @intCast(frame.len), .big);
        @memcpy(segment[length_prefix_len..], frame);

        if (self.current) |blob| if (blob.isReproducing()) {
            const count = @min(segment.len, blob.capacity - blob.used);
            if (!std.mem.eql(u8, segment[0..count], blob.payload[blob.used..][0..count])) return error.SeededBlobMismatch;
        };

        const room = if (self.current) |blob| blob.capacity - blob.used else payload_capacity;
        const seals = if (segment.len <= room) 0 else 1 + (segment.len - room - 1) / payload_capacity;
        const sealed = try self.allocator.alloc(SealedBlob, seals);
        var built: usize = 0;
        errdefer {
            for (sealed[0..built]) |*blob| self.allocator.free(blob.segmentEnds());
            self.allocator.free(sealed);
        }

        var work = if (self.current) |blob| try blob.clone(self.allocator) else try OpenBlob.create(self.allocator, self.data_rolling_hash);
        errdefer work.deinit(self.allocator);
        try work.ends.ensureUnusedCapacity(self.allocator, 1);

        var written: usize = 0;
        while (written < segment.len) {
            if (work.used == work.capacity) {
                try seal(self.allocator, &sealed[built], &work);
                work.restart(sealed[built].data_rolling_hash);
                built += 1;
            }
            const count = @min(segment.len - written, work.capacity - work.used);
            @memcpy(work.payload[work.used..][0..count], segment[written..][0..count]);
            work.used += count;
            written += count;
        }
        work.ends.appendAssumeCapacity(.{ .conflation_end_block = conflation_end_block, .offset = @intCast(work.used) });
        std.debug.assert(built == seals);

        if (self.current) |*blob| blob.deinit(self.allocator);
        self.current = work;
        if (built > 0) self.data_rolling_hash = sealed[built - 1].data_rolling_hash;
        return sealed;
    }

    /// Seals the open blob at the end of the stream: no blob when none is open, which the caller
    /// releases with `freeSealed`.
    pub fn flush(self: *Stream) Error![]SealedBlob {
        if (self.current == null) return &.{};
        const blob = &self.current.?;
        if (blob.isReproducing()) return error.FlushWhileReproducing;
        const sealed = try self.allocator.alloc(SealedBlob, 1);
        errdefer self.allocator.free(sealed);
        try seal(self.allocator, &sealed[0], blob);

        blob.deinit(self.allocator);
        self.current = null;
        self.data_rolling_hash = sealed[0].data_rolling_hash;
        return sealed;
    }
};

/// Packs `blob` into `out`, which holds the blob and its proof, hashes and segment ends afterwards.
fn seal(allocator: Allocator, out: *SealedBlob, blob: *const OpenBlob) Error!void {
    const ends = try allocator.alloc(SegmentEnd, blob.ends.items.len);
    errdefer allocator.free(ends);
    blob_payload.pack(blob.payload[0..blob.used], &out.blob) catch unreachable;
    const commitment = blob.shared_commitment orelse try kzg_prover.commit(&out.blob);
    out.kzg_proof = try kzg_prover.prove(&out.blob, &commitment);
    out.kzg_commitment = commitment;
    out.chunk_hash = blob_kzg.versionedHash(&commitment);
    out.prev_data_rolling_hash = blob.prev_data_rolling_hash;
    out.data_rolling_hash = rolling_hash.next(blob.prev_data_rolling_hash, out.chunk_hash);
    out.payload_bytes = @intCast(blob.used);
    for (ends, blob.ends.items) |*end, pending| {
        end.* = .{ .conflation_end_block = pending.conflation_end_block, .unused_tail_bytes = @intCast(blob.used - pending.offset) };
    }
    out.segment_end_count = @intCast(ends.len);
    out.segment_ends = if (ends.len == 0) null else ends.ptr;
}

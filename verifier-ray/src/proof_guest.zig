//! Expands the pointer-free guest image written by prover-ray's EncodeGuest.
//!
//! The byte layout is the comment on EncodeGuest in
//! prover-ray/wiop/proofserialization/compact.go. Decoding allocates every
//! slice from `allocator` and rebuilds null leaf slots, so `leaves.len` stays
//! the tree height the Merkle walk indexes. Zero limbs are filled back in
//! before anything hashes the row.
//!
//! `decode_buffer_len` is the fixed buffer the native and R5 loaders pass in.
//! It has to hold the expanded proof (materialized zero limbs plus the slice
//! headers), which is larger than the packed image. The committed RISC-V
//! image expands to about 17MB of that buffer.

const std = @import("std");
const verifier = @import("verifier.zig");
const protocol = @import("protocol/root.zig");
const pcs = @import("query/pcs.zig");
const fri = @import("query/fri.zig");
const merkle = @import("crypto/merkle.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const field = @import("field/koalabear.zig");
const extf = @import("field/koalabear_ext.zig");
const value = @import("field/value.zig");

pub const decode_buffer_len: usize = 24 * 1024 * 1024;

const magic = "LPR1";
const max_count: u32 = 1 << 20;
const max_image_len: u32 = 0x40000000;

pub const Error = error{
    InvalidProofImage,
    TruncatedProofImage,
    NonCanonicalFieldElement,
} || std.mem.Allocator.Error;

const Reader = struct {
    buf: []const u8,
    off: usize,

    fn need(self: *Reader, n: usize) Error!void {
        if (n > self.buf.len - self.off) return error.TruncatedProofImage;
    }

    fn readU8(self: *Reader) Error!u8 {
        try self.need(1);
        const v = self.buf[self.off];
        self.off += 1;
        return v;
    }

    fn readU16(self: *Reader) Error!u16 {
        try self.need(2);
        const v = std.mem.readInt(u16, self.buf[self.off..][0..2], .little);
        self.off += 2;
        return v;
    }

    fn readU32(self: *Reader) Error!u32 {
        try self.need(4);
        const v = std.mem.readInt(u32, self.buf[self.off..][0..4], .little);
        self.off += 4;
        return v;
    }

    fn readU64(self: *Reader) Error!u64 {
        try self.need(8);
        const v = std.mem.readInt(u64, self.buf[self.off..][0..8], .little);
        self.off += 8;
        return v;
    }

    fn count(self: *Reader) Error!usize {
        const n = try self.readU32();
        if (n > max_count) return error.InvalidProofImage;
        return n;
    }

    /// The bitset bytes for `n` flags. Bit i is byte i>>3, mask 1<<(i&7).
    fn bits(self: *Reader, n: usize) Error![]const u8 {
        if (n == 0) return &.{};
        const nb = (n + 7) / 8;
        try self.need(nb);
        const raw = self.buf[self.off..][0..nb];
        self.off += nb;
        return raw;
    }

    fn elem(self: *Reader) Error!field.Element {
        const raw = try self.readU32();
        if (raw >= field.modulus) return error.NonCanonicalFieldElement;
        return .{ .value = raw };
    }

    fn ext(self: *Reader) Error!extf.Ext {
        return .{
            .B0 = .{ .a0 = try self.elem(), .a1 = try self.elem() },
            .B1 = .{ .a0 = try self.elem(), .a1 = try self.elem() },
            .B2 = .{ .a0 = try self.elem(), .a1 = try self.elem() },
        };
    }

    fn digest(self: *Reader) Error!poseidon2.Digest {
        var d: poseidon2.Digest = undefined;
        for (&d) |*limb| limb.* = try self.elem();
        return d;
    }

    fn digests(self: *Reader, allocator: std.mem.Allocator) Error![]poseidon2.Digest {
        const n = try self.count();
        if (n == 0) return &.{};
        const out = try allocator.alloc(poseidon2.Digest, n);
        for (out) |*d| d.* = try self.digest();
        return out;
    }

    fn rowIndex(self: *Reader, n: usize) Error!usize {
        const i = try self.readU32();
        if (@as(usize, i) >= n) return error.InvalidProofImage;
        return i;
    }
};

fn bitOn(raw: []const u8, i: usize) bool {
    const mask: u8 = @as(u8, 1) << @as(u3, @intCast(i & 7));
    return (raw[i >> 3] & mask) != 0;
}

/// A zero-length result is a mutable empty slice, so callers can fill the
/// nonempty case through the same pointer type.
fn allocSlice(comptime T: type, allocator: std.mem.Allocator, n: usize) Error![]T {
    if (n == 0) return &.{};
    return allocator.alloc(T, n);
}

pub fn decode(allocator: std.mem.Allocator, image: []const u8) Error!verifier.VerifyInput {
    if (image.len < 8) return error.TruncatedProofImage;
    if (!std.mem.eql(u8, image[0..4], magic)) return error.InvalidProofImage;
    const n = std.mem.readInt(u32, image[4..8], .little);
    if (n < 8 or @as(usize, n) > image.len or n > max_image_len) return error.InvalidProofImage;

    var r = Reader{ .buf = image[8..n], .off = 0 };
    const rows = try readRows(allocator, &r);
    const rounds = try readRounds(allocator, &r);
    const module_sizes = try readModuleSizes(allocator, &r);
    const public_inputs = try readScalars(allocator, &r);
    const queries = try readQueries(allocator, &r, rows);
    const caps = try readInputCaps(allocator, &r, rows);
    const fri_proof = try readFri(allocator, &r);
    if (r.off != r.buf.len) return error.InvalidProofImage;

    return .{
        .proof = .{
            .rounds = rounds,
            .module_sizes = module_sizes,
            .pcs_opening = .{ .proof = .{
                .input_queries = queries,
                .input_caps = caps,
                .fri_proof = fri_proof,
            } },
        },
        .public_inputs = public_inputs,
    };
}

fn readRows(allocator: std.mem.Allocator, r: *Reader) Error![]merkle.RowOpening {
    const n = try r.count();
    if (n == 0) return &.{};
    const rows = try allocator.alloc(merkle.RowOpening, n);
    for (rows) |*row| row.* = try readRow(allocator, r);
    return rows;
}

fn readRow(allocator: std.mem.Allocator, r: *Reader) Error!merkle.RowOpening {
    const n_base: usize = @intCast(try r.readU16());
    const n_ext: usize = @intCast(try r.readU16());
    const base_bits = try r.bits(n_base);
    const base = try allocSlice(field.Element, allocator, n_base);
    for (base, 0..) |*e, i| {
        e.* = if (bitOn(base_bits, i)) try r.elem() else field.Element.zero();
    }
    const ext_bits = try r.bits(n_ext);
    const exts = try allocSlice(extf.Ext, allocator, n_ext);
    for (exts, 0..) |*e, i| {
        e.* = if (bitOn(ext_bits, i)) try r.ext() else extf.Ext.zero();
    }
    return .{ .base = base, .ext = exts };
}

fn readScalars(allocator: std.mem.Allocator, r: *Reader) Error![]value.Scalar {
    const n = try r.count();
    if (n == 0) return &.{};
    const flags = try r.bits(n);
    const out = try allocator.alloc(value.Scalar, n);
    for (out, 0..) |*cell, i| {
        const e = try r.ext();
        cell.* = if (bitOn(flags, i)) .{ .ext = e } else .{ .base = e.B0.a0 };
    }
    return out;
}

fn readRounds(allocator: std.mem.Allocator, r: *Reader) Error![]protocol.RoundMessage {
    const n = try r.count();
    if (n == 0) return &.{};
    const out = try allocator.alloc(protocol.RoundMessage, n);
    for (out) |*round| {
        const cells = try readScalars(allocator, r);
        const flag = try r.readU8();
        const commitment: ?protocol.Commitment = switch (flag) {
            0 => null,
            1 => try r.digest(),
            else => return error.InvalidProofImage,
        };
        round.* = .{ .cells = cells, .commitment = commitment };
    }
    return out;
}

fn readModuleSizes(allocator: std.mem.Allocator, r: *Reader) Error![]usize {
    const n = try r.count();
    if (n == 0) return &.{};
    const out = try allocator.alloc(usize, n);
    for (out) |*size| {
        const v = try r.readU64();
        if (comptime @bitSizeOf(usize) < 64) {
            if (v > std.math.maxInt(usize)) return error.InvalidProofImage;
        }
        size.* = @intCast(v);
    }
    return out;
}

fn readQueries(
    allocator: std.mem.Allocator,
    r: *Reader,
    rows: []const merkle.RowOpening,
) Error![]const []const merkle.InputTreeOpening {
    const n_queries = try r.count();
    if (n_queries == 0) return &.{};
    const queries = try allocator.alloc([]const merkle.InputTreeOpening, n_queries);
    for (queries) |*query| {
        const n_trees = try r.count();
        if (n_trees == 0) {
            query.* = &.{};
            continue;
        }
        const trees = try allocator.alloc(merkle.InputTreeOpening, n_trees);
        for (trees) |*tree| {
            const siblings = try r.digests(allocator);
            const n_levels = try r.count();
            const present = try r.bits(n_levels);
            const leaves = try allocSlice(?merkle.RowPair, allocator, n_levels);
            for (leaves, 0..) |*leaf, i| {
                if (!bitOn(present, i)) {
                    leaf.* = null;
                    continue;
                }
                const half0 = try r.rowIndex(rows.len);
                const half1 = try r.rowIndex(rows.len);
                leaf.* = merkle.RowPair{ rows[half0], rows[half1] };
            }
            tree.* = .{ .siblings = siblings, .leaves = leaves };
        }
        query.* = trees;
    }
    return queries;
}

fn readInputCaps(
    allocator: std.mem.Allocator,
    r: *Reader,
    rows: []const merkle.RowOpening,
) Error![]const pcs.InputCap {
    const n = try r.count();
    if (n == 0) return &.{};
    const caps = try allocator.alloc(pcs.InputCap, n);
    for (caps) |*cap| {
        const nodes = try r.digests(allocator);
        const n_tables = try r.count();
        const tables = try allocSlice(pcs.InputCapTable, allocator, n_tables);
        for (tables) |*table| {
            const size_log2 = try r.readU8();
            const n_rows = try r.count();
            const cap_rows = try allocSlice(merkle.RowOpening, allocator, n_rows);
            for (cap_rows) |*row| {
                const idx = try r.rowIndex(rows.len);
                row.* = rows[idx];
            }
            table.* = .{ .size_log2 = size_log2, .rows = cap_rows };
        }
        cap.* = .{ .nodes = nodes, .tables = tables };
    }
    return caps;
}

fn readFri(allocator: std.mem.Allocator, r: *Reader) Error!fri.Proof {
    const round_roots = try r.digests(allocator);
    const n_caps = try r.count();
    const round_caps = try allocSlice(merkle.MerkleCap, allocator, n_caps);
    for (round_caps) |*cap| {
        const nodes = try r.digests(allocator);
        const n_aux = try r.count();
        const present = try r.bits(n_aux);
        const aux = try allocSlice(?poseidon2.Digest, allocator, n_aux);
        for (aux, 0..) |*slot, i| {
            slot.* = if (bitOn(present, i)) try r.digest() else null;
        }
        cap.* = .{ .nodes = nodes, .aux = aux };
    }
    const n_final = try r.count();
    const final_poly = try allocSlice(extf.Ext, allocator, n_final);
    for (final_poly) |*e| e.* = try r.ext();

    const n_running = try r.count();
    const running = try allocSlice([]const merkle.Branch, allocator, n_running);
    for (running) |*query| {
        const n_branches = try r.count();
        if (n_branches == 0) {
            query.* = &.{};
            continue;
        }
        const branches = try allocator.alloc(merkle.Branch, n_branches);
        for (branches) |*branch| {
            const siblings = try r.digests(allocator);
            branch.* = .{ .siblings = siblings, .leaf = try r.digest() };
        }
        query.* = branches;
    }
    return .{
        .round_roots = round_roots,
        .round_caps = round_caps,
        .final_poly = final_poly,
        .running_queries = running,
    };
}

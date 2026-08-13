const protocol = @import("protocol/root.zig");
const field = @import("field/koalabear.zig");
const ext = @import("field/koalabear_ext.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const merkle = @import("crypto/merkle.zig");
const pcs = @import("query/pcs.zig");
const fri = @import("query/fri.zig");
const verifier = @import("verifier.zig");

/// Decodes the binary wire format prover-ray's verifier-ray/codegen.EncodeProof
/// emits (see that function's doc comment for the exact byte layout) into a
/// verifier.VerifyInput.
///
/// No self-describing type tags are used except for Scalar (base vs extension
/// field is a genuine per-cell runtime fact, not a static schema property) —
/// every other field's shape is fixed at compile time and shared with the Go
/// encoder by construction: every variable-length list is a little-endian u32
/// count followed by that many encodings back-to-back, and nested
/// variable-length elements are self-delimiting recursively.
///
/// Decoded slices point into a caller-owned DecodeScratch(comptime systems)
/// buffer — comptime-sized fixed-capacity arrays derived from `systems`
/// (mirroring protocol/public_input.zig's BoundRoundMessages), so decoding
/// never needs a runtime allocator. This preserves verifier-ray's existing
/// zero-allocator convention (there is no std.mem.Allocator anywhere in src/
/// today) — required for the R5 zkVM environment, which has none available.
pub const DecodeError = error{
    UnexpectedEof,
    InvalidBool,
    InvalidScalarTag,
    TooManyRounds,
    TooManyCells,
    TooManyModuleSizes,
    TooManyEntries,
    TooManyShifts,
    TooManyRowColumns,
    TooManyQueries,
    TooManyInputTrees,
    TooManyLevels,
    TooManyRunningQueries,
    TooManyRounds2,
};

/// DecodeScratch(comptime systems) is the fixed-capacity backing storage every
/// slice `decodeVerifyInput` returns points into. Every array dimension is
/// derived from already-comptime `systems` fields:
///
///   - rounds / cells per round: systems.public_input.round_cell_counts
///   - module_sizes: dynamic module count, derived from systems.pcs.columns
///   - entry_claims: systems.pcs.max_entries outer, each column's own
///     .shifts.len inner (from systems.pcs.columns)
///   - input_queries: systems.pcs.envelope_params.num_queries outer,
///     systems.pcs.num_batches inner (at most one distinct tree per batch)
///   - InputTreeOpening siblings/leaves: systems.pcs.max_size_log2 (tree depth)
///   - fri_proof.round_roots / final_poly / running_queries: bounded by
///     systems.pcs.max_size_log2 (max fold-round count)
pub fn DecodeScratch(comptime systems: verifier.Systems) type {
    const pcs_system = systems.pcs;
    const round_count = systems.public_input.round_cell_counts.len;
    const cell_cap = comptime blk: {
        var m: usize = 0;
        for (systems.public_input.round_cell_counts) |c| {
            if (c > m) m = c;
        }
        break :blk @max(m, 1);
    };
    const num_dynamic_modules = comptime blk: {
        var n: usize = 0;
        for (pcs_system.columns) |col| {
            if (col.size == .dynamic) n += 1;
        }
        break :blk n;
    };
    const max_entries = @max(pcs_system.max_entries, 1);
    const max_shifts_per_entry = comptime blk: {
        var m: usize = 0;
        for (pcs_system.columns) |col| {
            if (col.shifts.len > m) m = col.shifts.len;
        }
        break :blk @max(m, 1);
    };
    const num_queries = @max(pcs_system.envelope_params.num_queries, 1);
    // At most one distinct input tree per committed batch (routeInputRoots
    // dedups by Merkle root, so a query's opening can never carry more).
    const max_input_trees = @max(pcs_system.num_batches, 1);
    // A tree's depth is bounded by the envelope's max size_log2 — the
    // largest bundle any column can ever occupy.
    const max_tree_depth = @max(pcs_system.max_size_log2, 1);
    // Folding rounds are bounded the same way (one fewer than max_size_log2
    // levels, but +1 keeps every array non-degenerate and simplifies bounds).
    const max_fold_rounds = @max(pcs_system.max_size_log2, 1);
    // A RowOpening's base/ext width is the number of columns sharing one
    // (batch, size_log2, is_ext) bucket (see pcs.zig's bundleBatchWidths) —
    // NOT related to any single column's shift count. Worst case, every
    // column in the system shares one bucket, so max_entries is always a
    // safe (if loose) bound.
    const max_row_width = max_entries;

    return struct {
        const Self = @This();

        round_commitments: [round_count]?protocol.Commitment = undefined,
        round_cell_counts_actual: [round_count]usize = undefined,
        cell_storage: [round_count][cell_cap]protocol.Scalar = undefined,
        rounds_buf: [round_count]protocol.RoundMessage = undefined,

        module_sizes_storage: [num_dynamic_modules]usize = undefined,
        module_sizes_len: usize = 0,

        entry_claims_counts: [max_entries]usize = undefined,
        entry_claims_storage: [max_entries][max_shifts_per_entry]ext.Ext = undefined,
        entry_claims_buf: [max_entries][]const ext.Ext = undefined,

        input_query_counts: [num_queries]usize = undefined,
        input_tree_siblings_counts: [num_queries][max_input_trees]usize = undefined,
        input_tree_siblings_storage: [num_queries][max_input_trees][max_tree_depth]poseidon2.Digest = undefined,
        input_tree_leaves_counts: [num_queries][max_input_trees]usize = undefined,
        input_tree_leaves_storage: [num_queries][max_input_trees][max_tree_depth]?merkle.RowPair = undefined,
        input_tree_row_base_storage: [num_queries][max_input_trees][max_tree_depth][2][max_row_width]field.Element = undefined,
        input_tree_row_ext_storage: [num_queries][max_input_trees][max_tree_depth][2][max_row_width]ext.Ext = undefined,
        input_trees_storage: [num_queries][max_input_trees]merkle.InputTreeOpening = undefined,
        input_queries_buf: [num_queries][]const merkle.InputTreeOpening = undefined,

        round_roots_count: usize = 0,
        round_roots_storage: [max_fold_rounds]poseidon2.Digest = undefined,
        final_poly_count: usize = 0,
        final_poly_storage: [max_tree_depth]ext.Ext = undefined,

        running_query_counts: [num_queries]usize = undefined,
        running_branch_siblings_counts: [num_queries][max_fold_rounds]usize = undefined,
        running_branch_siblings_storage: [num_queries][max_fold_rounds][max_tree_depth]poseidon2.Digest = undefined,
        running_branches_storage: [num_queries][max_fold_rounds]merkle.Branch = undefined,
        running_queries_buf: [num_queries][]const merkle.Branch = undefined,

        public_input_storage: [cell_cap * round_count]protocol.Scalar = undefined,
    };
}

const Reader = struct {
    bytes: []const u8,
    pos: usize = 0,

    fn take(self: *Reader, n: usize) DecodeError![]const u8 {
        if (self.pos + n > self.bytes.len) return DecodeError.UnexpectedEof;
        const out = self.bytes[self.pos .. self.pos + n];
        self.pos += n;
        return out;
    }

    fn readByte(self: *Reader) DecodeError!u8 {
        const b = try self.take(1);
        return b[0];
    }

    fn readBool(self: *Reader) DecodeError!bool {
        return switch (try self.readByte()) {
            0 => false,
            1 => true,
            else => DecodeError.InvalidBool,
        };
    }

    fn readCount(self: *Reader) DecodeError!usize {
        const b = try self.take(4);
        return std.mem.readInt(u32, b[0..4], .little);
    }

    fn readUsize(self: *Reader) DecodeError!usize {
        const b = try self.take(8);
        return @intCast(std.mem.readInt(u64, b[0..8], .little));
    }

    fn readElement(self: *Reader) DecodeError!field.Element {
        const b = try self.take(field.bytes);
        return field.Element.fromBytesCanonicalSlice(b) catch return DecodeError.UnexpectedEof;
    }

    fn readExt(self: *Reader) DecodeError!ext.Ext {
        return .{
            .B0 = .{ .a0 = try self.readElement(), .a1 = try self.readElement() },
            .B1 = .{ .a0 = try self.readElement(), .a1 = try self.readElement() },
            .B2 = .{ .a0 = try self.readElement(), .a1 = try self.readElement() },
        };
    }

    fn readOctuplet(self: *Reader) DecodeError!poseidon2.Digest {
        var out: poseidon2.Digest = undefined;
        for (&out) |*e| e.* = try self.readElement();
        return out;
    }

    fn readScalar(self: *Reader) DecodeError!protocol.Scalar {
        return switch (try self.readByte()) {
            0 => .{ .base = try self.readElement() },
            1 => .{ .ext = try self.readExt() },
            else => DecodeError.InvalidScalarTag,
        };
    }
};

const std = @import("std");

/// Decodes `bytes` into a verifier.VerifyInput whose slices point into
/// `scratch`. `scratch` must outlive the returned VerifyInput.
pub fn decodeVerifyInput(
    comptime systems: verifier.Systems,
    bytes: []const u8,
    scratch: *DecodeScratch(systems),
) DecodeError!verifier.VerifyInput {
    var r = Reader{ .bytes = bytes };

    try decodeRounds(systems, &r, scratch);
    try decodeModuleSizes(systems, &r, scratch);
    try decodePcsOpening(systems, &r, scratch);
    const public_inputs = try decodePublicInput(&r, scratch);

    return .{
        .proof = .{
            .rounds = scratch.rounds_buf[0..],
            .module_sizes = scratch.module_sizes_storage[0..scratch.module_sizes_len],
            .pcs_opening = .{
                .entry_claims = scratch.entry_claims_buf[0..],
                .proof = .{
                    .input_queries = scratch.input_queries_buf[0..],
                    .fri_proof = .{
                        .round_roots = scratch.round_roots_storage[0..scratch.round_roots_count],
                        .final_poly = scratch.final_poly_storage[0..scratch.final_poly_count],
                        .running_queries = scratch.running_queries_buf[0..],
                    },
                },
            },
        },
        .public_inputs = public_inputs,
    };
}

fn decodeRounds(comptime systems: verifier.Systems, r: *Reader, scratch: *DecodeScratch(systems)) DecodeError!void {
    const round_count = systems.public_input.round_cell_counts.len;
    const cell_cap = scratch.cell_storage[0].len;

    const n = try r.readCount();
    if (n != round_count) return DecodeError.TooManyRounds;

    for (0..round_count) |i| {
        const has_commitment = try r.readBool();
        scratch.round_commitments[i] = if (has_commitment) try r.readOctuplet() else null;

        const cell_count = try r.readCount();
        if (cell_count > cell_cap) return DecodeError.TooManyCells;
        scratch.round_cell_counts_actual[i] = cell_count;
        for (0..cell_count) |j| {
            scratch.cell_storage[i][j] = try r.readScalar();
        }
        scratch.rounds_buf[i] = .{
            .commitment = scratch.round_commitments[i],
            .cells = scratch.cell_storage[i][0..cell_count],
        };
    }
}

fn decodeModuleSizes(comptime systems: verifier.Systems, r: *Reader, scratch: *DecodeScratch(systems)) DecodeError!void {
    const cap = scratch.module_sizes_storage.len;
    const n = try r.readCount();
    if (n > cap) return DecodeError.TooManyModuleSizes;
    for (0..n) |i| {
        scratch.module_sizes_storage[i] = try r.readUsize();
    }
    // A system with fewer dynamic modules than the proof declares sizes for
    // is a mismatch the reconstruct() call inside verify() will itself catch
    // (via MissingDynamicModuleSize/LayoutOverflow) — no need to duplicate
    // that check here. Any remainder slots stay unused (module_sizes is
    // sliced to n by the caller — see below).
    scratch.module_sizes_len = n;
}

fn decodePcsOpening(comptime systems: verifier.Systems, r: *Reader, scratch: *DecodeScratch(systems)) DecodeError!void {
    const pcs_system = systems.pcs;
    const max_entries = scratch.entry_claims_storage.len;
    const max_shifts = scratch.entry_claims_storage[0].len;

    const num_entries = try r.readCount();
    if (num_entries > max_entries) return DecodeError.TooManyEntries;
    for (0..num_entries) |i| {
        const shifts = try r.readCount();
        if (shifts > max_shifts) return DecodeError.TooManyShifts;
        scratch.entry_claims_counts[i] = shifts;
        for (0..shifts) |k| {
            scratch.entry_claims_storage[i][k] = try r.readExt();
        }
        scratch.entry_claims_buf[i] = scratch.entry_claims_storage[i][0..shifts];
    }

    const num_queries = try r.readCount();
    const query_cap = scratch.input_query_counts.len;
    if (num_queries > query_cap) return DecodeError.TooManyQueries;
    for (0..num_queries) |q| {
        const num_trees = try r.readCount();
        const tree_cap = scratch.input_tree_siblings_counts[q].len;
        if (num_trees > tree_cap) return DecodeError.TooManyInputTrees;
        scratch.input_query_counts[q] = num_trees;
        for (0..num_trees) |t| {
            try decodeInputTreeOpening(pcs_system, r, scratch, q, t);
        }
        scratch.input_queries_buf[q] = scratch.input_trees_storage[q][0..num_trees];
    }

    try decodeFriProof(r, scratch);
}

fn decodeInputTreeOpening(
    comptime pcs_system: pcs.System,
    r: *Reader,
    scratch: anytype,
    q: usize,
    t: usize,
) DecodeError!void {
    _ = pcs_system;
    const depth_cap = scratch.input_tree_siblings_storage[q][t].len;

    const num_siblings = try r.readCount();
    if (num_siblings > depth_cap) return DecodeError.TooManyLevels;
    for (0..num_siblings) |i| {
        scratch.input_tree_siblings_storage[q][t][i] = try r.readOctuplet();
    }
    scratch.input_tree_siblings_counts[q][t] = num_siblings;

    const num_leaves = try r.readCount();
    if (num_leaves > depth_cap) return DecodeError.TooManyLevels;
    for (0..num_leaves) |i| {
        const present = try r.readBool();
        if (!present) {
            scratch.input_tree_leaves_storage[q][t][i] = null;
            continue;
        }
        const row0 = try decodeRowOpening(r, scratch, q, t, i, 0);
        const row1 = try decodeRowOpening(r, scratch, q, t, i, 1);
        scratch.input_tree_leaves_storage[q][t][i] = .{ row0, row1 };
    }
    scratch.input_tree_leaves_counts[q][t] = num_leaves;

    scratch.input_trees_storage[q][t] = .{
        .siblings = scratch.input_tree_siblings_storage[q][t][0..num_siblings],
        .leaves = scratch.input_tree_leaves_storage[q][t][0..num_leaves],
    };
}

fn decodeRowOpening(r: *Reader, scratch: anytype, q: usize, t: usize, level: usize, side: usize) DecodeError!merkle.RowOpening {
    const base_cap = scratch.input_tree_row_base_storage[q][t][level][side].len;
    const ext_cap = scratch.input_tree_row_ext_storage[q][t][level][side].len;

    const base_count = try r.readCount();
    if (base_count > base_cap) return DecodeError.TooManyRowColumns;
    for (0..base_count) |i| {
        scratch.input_tree_row_base_storage[q][t][level][side][i] = try r.readElement();
    }

    const ext_count = try r.readCount();
    if (ext_count > ext_cap) return DecodeError.TooManyRowColumns;
    for (0..ext_count) |i| {
        scratch.input_tree_row_ext_storage[q][t][level][side][i] = try r.readExt();
    }

    return .{
        .base = scratch.input_tree_row_base_storage[q][t][level][side][0..base_count],
        .ext = scratch.input_tree_row_ext_storage[q][t][level][side][0..ext_count],
    };
}

fn decodeFriProof(r: *Reader, scratch: anytype) DecodeError!void {
    const round_roots_cap = scratch.round_roots_storage.len;
    const round_roots_count = try r.readCount();
    if (round_roots_count > round_roots_cap) return DecodeError.TooManyRounds2;
    for (0..round_roots_count) |i| {
        scratch.round_roots_storage[i] = try r.readOctuplet();
    }
    scratch.round_roots_count = round_roots_count;

    const final_poly_cap = scratch.final_poly_storage.len;
    const final_poly_count = try r.readCount();
    if (final_poly_count > final_poly_cap) return DecodeError.TooManyRounds2;
    for (0..final_poly_count) |i| {
        scratch.final_poly_storage[i] = try r.readExt();
    }
    scratch.final_poly_count = final_poly_count;

    const num_running_queries = try r.readCount();
    const query_cap = scratch.running_query_counts.len;
    if (num_running_queries > query_cap) return DecodeError.TooManyRunningQueries;
    for (0..num_running_queries) |q| {
        const num_rounds = try r.readCount();
        const round_cap = scratch.running_branch_siblings_counts[q].len;
        if (num_rounds > round_cap) return DecodeError.TooManyRounds2;
        scratch.running_query_counts[q] = num_rounds;
        for (0..num_rounds) |j| {
            const leaf = try r.readOctuplet();
            const sib_cap = scratch.running_branch_siblings_storage[q][j].len;
            const sib_count = try r.readCount();
            if (sib_count > sib_cap) return DecodeError.TooManyLevels;
            for (0..sib_count) |k| {
                scratch.running_branch_siblings_storage[q][j][k] = try r.readOctuplet();
            }
            scratch.running_branch_siblings_counts[q][j] = sib_count;
            scratch.running_branches_storage[q][j] = .{
                .leaf = leaf,
                .siblings = scratch.running_branch_siblings_storage[q][j][0..sib_count],
            };
        }
        scratch.running_queries_buf[q] = scratch.running_branches_storage[q][0..num_rounds];
    }
}

fn decodePublicInput(r: *Reader, scratch: anytype) DecodeError!verifier.PublicInput {
    const cap = scratch.public_input_storage.len;
    const n = try r.readCount();
    if (n > cap) return DecodeError.TooManyCells;
    for (0..n) |i| {
        scratch.public_input_storage[i] = try r.readScalar();
    }
    return scratch.public_input_storage[0..n];
}

//! The official EIP-4844 reference vectors of c-kzg-4844 (the `tests/` directory of the pinned
//! release): the cases of a suite with their decoded inputs and outputs.

const std = @import("std");

const c_kzg_paths = @import("c_kzg_paths");

const Allocator = std.mem.Allocator;

pub const blob_len = 4096 * 32;

/// One case of a suite: the decoded input fields (null where the case omits one) and the expected
/// output.
pub const Case = struct {
    name: []const u8,
    blob: ?[]u8,
    commitment: ?[]u8,
    proof: ?[]u8,
    output: []const u8,

    pub fn deinit(self: Case, allocator: Allocator) void {
        allocator.free(self.name);
        for ([_]?[]u8{ self.blob, self.commitment, self.proof }) |bytes| if (bytes) |b| allocator.free(b);
        allocator.free(self.output);
    }

    /// The input as the verifier's fixed-size arguments; null for a case whose input has the wrong size.
    pub fn blobArray(self: Case) ?*const [blob_len]u8 {
        return if (self.blob) |bytes| if (bytes.len == blob_len) bytes[0..blob_len] else null else null;
    }

    pub fn commitmentArray(self: Case) ?*const [48]u8 {
        return if (self.commitment) |bytes| if (bytes.len == 48) bytes[0..48] else null else null;
    }

    pub fn proofArray(self: Case) ?*const [48]u8 {
        return if (self.proof) |bytes| if (bytes.len == 48) bytes[0..48] else null else null;
    }
};

/// The value of `key: value` in the yaml text, without quotes.
fn yamlValue(text: []const u8, key: []const u8) ?[]const u8 {
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r");
        if (!std.mem.startsWith(u8, trimmed, key) or !std.mem.startsWith(u8, trimmed[key.len..], ": ")) continue;
        return std.mem.trim(u8, trimmed[key.len + 2 ..], "'");
    }
    return null;
}

fn decodeHex(allocator: Allocator, hex: ?[]const u8) !?[]u8 {
    const text = hex orelse return null;
    const digits = if (std.mem.startsWith(u8, text, "0x")) text[2..] else text;
    const bytes = try allocator.alloc(u8, digits.len / 2);
    errdefer allocator.free(bytes);
    _ = try std.fmt.hexToBytes(bytes, digits);
    return bytes;
}

fn loadCase(allocator: Allocator, io: std.Io, suite_dir: std.Io.Dir, name: []const u8) !Case {
    const path = try std.fmt.allocPrint(allocator, "{s}/data.yaml", .{name});
    defer allocator.free(path);
    const text = try suite_dir.readFileAlloc(io, path, allocator, .limited(1 << 20));
    defer allocator.free(text);
    return .{
        .name = try allocator.dupe(u8, name),
        .blob = try decodeHex(allocator, yamlValue(text, "blob")),
        .commitment = try decodeHex(allocator, yamlValue(text, "commitment")),
        .proof = try decodeHex(allocator, yamlValue(text, "proof")),
        .output = try allocator.dupe(u8, yamlValue(text, "output") orelse return error.MissingOutput),
    };
}

fn openSuite(allocator: Allocator, io: std.Io, suite: []const u8) !std.Io.Dir {
    const path = try std.fmt.allocPrint(allocator, "{s}/{s}/kzg-mainnet", .{ c_kzg_paths.tests_dir, suite });
    defer allocator.free(path);
    return std.Io.Dir.openDirAbsolute(io, path, .{ .iterate = true });
}

/// Every case of the suite, such as `verify_blob_kzg_proof`; release them with `freeCases`.
pub fn loadSuite(allocator: Allocator, io: std.Io, suite: []const u8) !std.ArrayList(Case) {
    var dir = try openSuite(allocator, io, suite);
    defer dir.close(io);
    var cases: std.ArrayList(Case) = .empty;
    errdefer freeCases(allocator, &cases);
    var it = dir.iterate();
    while (try it.next(io)) |entry| {
        if (entry.kind != .directory) continue;
        try cases.append(allocator, try loadCase(allocator, io, dir, entry.name));
    }
    return cases;
}

/// One case of the `verify_blob_kzg_proof_batch` suite: the decoded entries of its three lists,
/// whatever their sizes and counts, and the expected output.
pub const BatchCase = struct {
    name: []const u8,
    blobs: []const []u8,
    commitments: []const []u8,
    proofs: []const []u8,
    output: []const u8,

    pub fn deinit(self: BatchCase, allocator: Allocator) void {
        allocator.free(self.name);
        for ([_][]const []u8{ self.blobs, self.commitments, self.proofs }) |list| freeList(allocator, list);
        allocator.free(self.output);
    }
};

/// The entries of the block list under `key:` in the yaml text; empty for `key: []`.
fn yamlList(allocator: Allocator, text: []const u8, key: []const u8) ![]const []u8 {
    var entries: std.ArrayList([]u8) = .empty;
    errdefer {
        for (entries.items) |bytes| allocator.free(bytes);
        entries.deinit(allocator);
    }
    var lines = std.mem.splitScalar(u8, text, '\n');
    var inside = false;
    while (lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r");
        if (!inside) {
            inside = std.mem.startsWith(u8, trimmed, key) and std.mem.eql(u8, trimmed[key.len..], ":");
            continue;
        }
        if (!std.mem.startsWith(u8, trimmed, "- ")) break;
        try entries.append(allocator, (try decodeHex(allocator, std.mem.trim(u8, trimmed[2..], "'"))).?);
    }
    return entries.toOwnedSlice(allocator);
}

fn loadBatchCase(allocator: Allocator, io: std.Io, suite_dir: std.Io.Dir, name: []const u8) !BatchCase {
    const path = try std.fmt.allocPrint(allocator, "{s}/data.yaml", .{name});
    defer allocator.free(path);
    const text = try suite_dir.readFileAlloc(io, path, allocator, .limited(1 << 24));
    defer allocator.free(text);
    const blobs = try yamlList(allocator, text, "blobs");
    errdefer freeList(allocator, blobs);
    const commitments = try yamlList(allocator, text, "commitments");
    errdefer freeList(allocator, commitments);
    const proofs = try yamlList(allocator, text, "proofs");
    errdefer freeList(allocator, proofs);
    const duped_name = try allocator.dupe(u8, name);
    errdefer allocator.free(duped_name);
    return .{
        .name = duped_name,
        .blobs = blobs,
        .commitments = commitments,
        .proofs = proofs,
        .output = try allocator.dupe(u8, yamlValue(text, "output") orelse return error.MissingOutput),
    };
}

fn freeList(allocator: Allocator, list: []const []u8) void {
    for (list) |bytes| allocator.free(bytes);
    allocator.free(list);
}

/// Every case of the `verify_blob_kzg_proof_batch` suite; release them with `freeBatchCases`.
pub fn loadBatchSuite(allocator: Allocator, io: std.Io) !std.ArrayList(BatchCase) {
    var dir = try openSuite(allocator, io, "verify_blob_kzg_proof_batch");
    defer dir.close(io);
    var cases: std.ArrayList(BatchCase) = .empty;
    errdefer freeBatchCases(allocator, &cases);
    var it = dir.iterate();
    while (try it.next(io)) |entry| {
        if (entry.kind != .directory) continue;
        try cases.append(allocator, try loadBatchCase(allocator, io, dir, entry.name));
    }
    return cases;
}

pub fn freeBatchCases(allocator: Allocator, cases: *std.ArrayList(BatchCase)) void {
    for (cases.items) |case| case.deinit(allocator);
    cases.deinit(allocator);
}

pub fn freeCases(allocator: Allocator, cases: *std.ArrayList(Case)) void {
    for (cases.items) |case| case.deinit(allocator);
    cases.deinit(allocator);
}

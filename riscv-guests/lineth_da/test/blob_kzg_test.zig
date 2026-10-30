const std = @import("std");

const bls_scalar = @import("bls_scalar");
const kzg_vectors = @import("kzg_vectors");
const lineth_da = @import("lineth_da");

const blob_kzg = lineth_da.encoding.blob_kzg;

const Fr = bls_scalar.Fr;

const gpa = std.testing.allocator;
const io = std.testing.io;

const blob_len = blob_kzg.blob_len;

/// One case of the c-kzg-4844 reference vectors: the decoded input fields (null where the case
/// omits one) and the expected output.
const Case = struct {
    name: []const u8,
    blob: ?[]u8,
    commitment: ?[]u8,
    proof: ?[]u8,
    output: []const u8,

    fn deinit(self: Case) void {
        gpa.free(self.name);
        for ([_]?[]u8{ self.blob, self.commitment, self.proof }) |bytes| if (bytes) |b| gpa.free(b);
        gpa.free(self.output);
    }

    /// The input as the verifier's fixed-size arguments; null for a case whose input has the wrong size.
    fn blobArray(self: Case) ?*const [blob_len]u8 {
        return if (self.blob) |bytes| if (bytes.len == blob_len) bytes[0..blob_len] else null else null;
    }

    fn commitmentArray(self: Case) ?*const [48]u8 {
        return if (self.commitment) |bytes| if (bytes.len == 48) bytes[0..48] else null else null;
    }

    fn proofArray(self: Case) ?*const [48]u8 {
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

fn decodeHex(hex: ?[]const u8) !?[]u8 {
    const text = hex orelse return null;
    const digits = if (std.mem.startsWith(u8, text, "0x")) text[2..] else text;
    const bytes = try gpa.alloc(u8, digits.len / 2);
    errdefer gpa.free(bytes);
    _ = try std.fmt.hexToBytes(bytes, digits);
    return bytes;
}

fn loadCase(suite_dir: std.Io.Dir, name: []const u8) !Case {
    const path = try std.fmt.allocPrint(gpa, "{s}/data.yaml", .{name});
    defer gpa.free(path);
    const text = try suite_dir.readFileAlloc(io, path, gpa, .limited(1 << 20));
    defer gpa.free(text);
    return .{
        .name = try gpa.dupe(u8, name),
        .blob = try decodeHex(yamlValue(text, "blob")),
        .commitment = try decodeHex(yamlValue(text, "commitment")),
        .proof = try decodeHex(yamlValue(text, "proof")),
        .output = try gpa.dupe(u8, yamlValue(text, "output") orelse return error.MissingOutput),
    };
}

fn openSuite(suite: []const u8) !std.Io.Dir {
    const path = try std.fmt.allocPrint(gpa, "{s}/{s}/kzg-mainnet", .{ kzg_vectors.tests_dir, suite });
    defer gpa.free(path);
    return std.Io.Dir.openDirAbsolute(io, path, .{ .iterate = true });
}

/// Every case of the suite.
fn loadSuite(suite: []const u8) !std.ArrayList(Case) {
    var dir = try openSuite(suite);
    defer dir.close(io);
    var cases: std.ArrayList(Case) = .empty;
    errdefer freeCases(&cases);
    var it = dir.iterate();
    while (try it.next(io)) |entry| {
        if (entry.kind != .directory) continue;
        try cases.append(gpa, try loadCase(dir, entry.name));
    }
    return cases;
}

fn freeCases(cases: *std.ArrayList(Case)) void {
    for (cases.items) |case| case.deinit();
    cases.deinit(gpa);
}

fn expectCase(case: Case, ok: bool) !void {
    if (!ok) std.debug.print("reference vector {s} fails\n", .{case.name});
    try std.testing.expect(ok);
}

fn hexOf(bytes: []const u8) ![]u8 {
    const hex = try gpa.alloc(u8, 2 + bytes.len * 2);
    hex[0..2].* = "0x".*;
    _ = std.fmt.bufPrint(hex[2..], "{x}", .{bytes}) catch unreachable;
    return hex;
}

test "blob proofs verify exactly as the reference vectors say" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);

    var accepted: usize = 0;
    var rejected: usize = 0;
    for (cases.items) |case| {
        const verified = if (case.blobArray()) |blob|
            if (case.commitmentArray()) |commitment|
                if (case.proofArray()) |proof| blob_kzg.verify(blob, commitment, proof) else false
            else
                false
        else
            false;
        // The reference answers `true` for a verifying proof, `false` for one that does not verify
        // and null for malformed input; only a verifying proof is accepted.
        try expectCase(case, verified == std.mem.eql(u8, case.output, "true"));
        if (verified) accepted += 1 else rejected += 1;
    }
    try std.testing.expectEqual(9, accepted);
    try std.testing.expectEqual(20, rejected);
}

test "challenges match the reference vectors" {
    var cases = try loadSuite("compute_challenge");
    defer freeCases(&cases);

    try std.testing.expect(cases.items.len >= 9);
    for (cases.items) |case| {
        const challenge = blob_kzg.computeChallenge(case.blobArray().?, case.commitmentArray().?);
        const hex = try hexOf(&challenge.toBytes());
        defer gpa.free(hex);
        try expectCase(case, std.mem.eql(u8, hex, case.output));
    }
}

test "the versioned hash of a commitment is 0x01 followed by its sha256 without the first byte" {
    // The compressed point at infinity: sha256 5f0657f3…4014.
    var infinity: [48]u8 = @splat(0);
    infinity[0] = 0xc0;
    const expected: [32]u8 = .{
        0x01, 0x06, 0x57, 0xf3, 0x75, 0x54, 0xc7, 0x81, 0x40, 0x2a, 0x22, 0x91, 0x7d, 0xee, 0x2f, 0x75,
        0xde, 0xf7, 0xab, 0x96, 0x6d, 0x7b, 0x77, 0x09, 0x05, 0x39, 0x8e, 0xba, 0x3c, 0x44, 0x40, 0x14,
    };
    try std.testing.expectEqualSlices(u8, &expected, &blob_kzg.versionedHash(&infinity));
}

// Roots of unity and powers by plain integer arithmetic, independent of the scalar-field module.
const modulus = bls_scalar.modulus;

fn mulMod(a: u256, b: u256) u256 {
    return @intCast(@as(u512, a) * b % modulus);
}

fn powMod(base: u256, exponent: u256) u256 {
    var result: u256 = 1;
    var power = base;
    var rest = exponent;
    while (rest > 0) : (rest >>= 1) {
        if (rest & 1 == 1) result = mulMod(result, power);
        power = mulMod(power, power);
    }
    return result;
}

/// The 4096th roots of unity ω^k of EIP-4844's domain, for ω = 7^((r - 1) / 4096).
fn rootTable() !*[4096]u256 {
    const roots = try gpa.create([4096]u256);
    const omega = powMod(7, (modulus - 1) / 4096);
    roots[0] = 1;
    for (1..4096) |k| roots[k] = mulMod(roots[k - 1], omega);
    return roots;
}

fn bitReverse(index: usize) usize {
    return @bitReverse(@as(u12, @intCast(index)));
}

fn writeElement(blob: *[blob_len]u8, index: usize, value: u256) void {
    std.mem.writeInt(u256, blob[index * 32 ..][0..32], value, .big);
}

fn element(value: u256) Fr {
    var bytes: [32]u8 = undefined;
    std.mem.writeInt(u256, &bytes, value, .big);
    return Fr.fromBytes(&bytes).?;
}

fn valueOf(fr: Fr) u256 {
    return std.mem.readInt(u256, &fr.toBytes(), .big);
}

test "the evaluation of a blob of monomial values is the monomial at the point" {
    const blob = try gpa.create([blob_len]u8);
    defer gpa.destroy(blob);
    const roots = try rootTable();
    defer gpa.destroy(roots);
    var prng = std.Random.DefaultPrng.init(7);

    for ([_]usize{ 0, 1, 2, 3, 100, 4095 }) |degree| {
        // The polynomial x^degree has the value ω^(brp(i) · degree) at the i-th element's point.
        for (0..4096) |i| writeElement(blob, i, roots[bitReverse(i) * degree % 4096]);
        for (0..5) |_| {
            const z = prng.random().int(u256) % modulus;
            try std.testing.expectEqual(powMod(z, degree), valueOf(blob_kzg.evaluate(blob, element(z)).?));
        }
        try std.testing.expectEqual(powMod(0, degree), valueOf(blob_kzg.evaluate(blob, Fr.zero).?));
    }
}

test "the evaluation at a point of the domain is the element at that point" {
    const blob = try gpa.create([blob_len]u8);
    defer gpa.destroy(blob);
    const roots = try rootTable();
    defer gpa.destroy(roots);
    var prng = std.Random.DefaultPrng.init(8);
    for (0..4096) |i| writeElement(blob, i, prng.random().int(u256) >> 3);

    for ([_]usize{ 0, 1, 2, 5, 2048, 4095 }) |k| {
        const expected = std.mem.readInt(u256, blob[bitReverse(k) * 32 ..][0..32], .big);
        try std.testing.expectEqual(expected, valueOf(blob_kzg.evaluate(blob, element(roots[k])).?));
    }
}

test "the evaluation is linear in the blob" {
    const first = try gpa.create([blob_len]u8);
    defer gpa.destroy(first);
    const second = try gpa.create([blob_len]u8);
    defer gpa.destroy(second);
    const sum = try gpa.create([blob_len]u8);
    defer gpa.destroy(sum);
    var prng = std.Random.DefaultPrng.init(9);
    for (0..4096) |i| {
        const a = prng.random().int(u256) % modulus;
        const b = prng.random().int(u256) % modulus;
        writeElement(first, i, a);
        writeElement(second, i, b);
        writeElement(sum, i, @intCast((@as(u512, a) + b) % modulus));
    }
    const z = element(prng.random().int(u256) % modulus);
    const expected = (@as(u512, valueOf(blob_kzg.evaluate(first, z).?)) + valueOf(blob_kzg.evaluate(second, z).?)) % modulus;
    try std.testing.expectEqual(@as(u256, @intCast(expected)), valueOf(blob_kzg.evaluate(sum, z).?));
}

test "a blob with an element at or above the modulus has no evaluation" {
    const blob = try gpa.create([blob_len]u8);
    defer gpa.destroy(blob);
    const roots = try rootTable();
    defer gpa.destroy(roots);
    @memset(blob, 0);
    for ([_]usize{ 0, 1234, 4095 }) |index| {
        writeElement(blob, index, modulus);
        try std.testing.expect(blob_kzg.evaluate(blob, element(5)) == null);
        // Also where the point lies in the domain and names another element.
        try std.testing.expect(blob_kzg.evaluate(blob, element(roots[3])) == null);
        writeElement(blob, index, modulus - 1);
        try std.testing.expect(blob_kzg.evaluate(blob, element(5)) != null);
        writeElement(blob, index, 0);
    }
}

/// A verifying case of the reference vectors.
fn validCase(cases: []const Case, name: []const u8) !Case {
    for (cases) |case| if (std.mem.eql(u8, case.name, name)) return case;
    return error.MissingCase;
}

fn expectRejected(what: []const u8, verified: bool) !void {
    if (verified) std.debug.print("a proof verified despite {s}\n", .{what});
    try std.testing.expect(!verified);
}

test "a proof verifies only for its own blob and commitment" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);
    const first = try validCase(cases.items, "verify_blob_kzg_proof_case_correct_proof_2");
    const second = try validCase(cases.items, "verify_blob_kzg_proof_case_correct_proof_3");
    const blob = first.blobArray().?;
    const commitment = first.commitmentArray().?;
    const proof = first.proofArray().?;
    try std.testing.expect(blob_kzg.verify(blob, commitment, proof));

    try expectRejected("the proof of another blob", blob_kzg.verify(blob, commitment, second.proofArray().?));
    try expectRejected("the commitment of another blob", blob_kzg.verify(blob, second.commitmentArray().?, proof));
    try expectRejected("the blob of another commitment", blob_kzg.verify(second.blobArray().?, commitment, proof));

    // A changed element: the low byte of an element, which stays a field element.
    const changed = try gpa.dupe(u8, blob);
    defer gpa.free(changed);
    changed[100 * 32 + 31] ^= 1;
    try expectRejected("a changed blob element", blob_kzg.verify(changed[0..blob_len], commitment, proof));

    var flipped_commitment = commitment.*;
    flipped_commitment[47] ^= 1;
    try expectRejected("a commitment with a flipped bit", blob_kzg.verify(blob, &flipped_commitment, proof));
    var flipped_proof = proof.*;
    flipped_proof[47] ^= 1;
    try expectRejected("a proof with a flipped bit", blob_kzg.verify(blob, commitment, &flipped_proof));
}

const std = @import("std");
const dummy = @import("verifier_ray");
const api = @import("api_surface.zig");

const verifier = dummy.verifier;
const protocol = dummy.protocol;
const image_relocation = dummy.image_relocation;
const Element = dummy.field.Element;
const Ext = dummy.field.Ext;
const Scalar = protocol.Scalar;

// ── Verifier ─────────────────────────────────────────────────────────────────

const spec = protocol.Spec{};

/// A two-input system, for unit tests.
const systems = verifier.Systems{ .public_input = .{ .refs = &.{
    .{ .statement_index = 0, .round = 0, .index = 0 },
    .{ .statement_index = 1, .round = 0, .index = 1 },
} } };

fn el(v: u32) Element {
    return .{ .value = v };
}

fn extOf(l: [6]u32) Ext {
    return .{ .B0 = .{ .a0 = el(l[0]), .a1 = el(l[1]) }, .B1 = .{ .a0 = el(l[2]), .a1 = el(l[3]) }, .B2 = .{ .a0 = el(l[4]), .a1 = el(l[5]) } };
}

const honest_x = [_]Scalar{ .{ .base = el(5) }, .{ .ext = extOf(.{ 1, 2, 3, 4, 5, 6 }) } };
const honest_y = [_]Scalar{ .{ .base = el(3125) }, .{ .ext = extOf(.{ 1, 32, 243, 1024, 3125, 7776 }) } };
const honest_rounds = [_]protocol.RoundMessage{.{ .cells = &honest_y }};
const empty_opening = verifier.PcsOpening{ .proof = [_]u8{0} ** 96 };
const honest_proof = verifier.Proof{ .rounds = &honest_rounds, .pcs_opening = empty_opening };

test "an honest proof verifies" {
    try verifier.verify(spec, systems, honest_proof, &honest_x);
    var workspace: verifier.Workspace(systems.public_input) = undefined;
    try verifier.verifyWithWorkspace(spec, systems, honest_proof, &honest_x, &workspace);
}

test "changing any public input or any limb is rejected" {
    var x = honest_x;
    x[0] = .{ .base = el(6) };
    try std.testing.expectError(error.DummyRelationViolated, verifier.verify(spec, systems, honest_proof, &x));

    for (0..6) |k| {
        var l = honest_x[1].ext.limbs();
        l[k] += 1;
        x = honest_x;
        x[1] = .{ .ext = extOf(l) };
        try std.testing.expectError(error.DummyRelationViolated, verifier.verify(spec, systems, honest_proof, &x));
    }
}

test "wrong counts and non-dummy proofs are rejected" {
    try std.testing.expectError(error.InvalidPublicInputCount, verifier.verify(spec, systems, honest_proof, honest_x[0..1]));

    const short = [_]protocol.RoundMessage{.{ .cells = honest_y[0..1] }};
    try std.testing.expectError(error.InvalidProofLength, verifier.verify(spec, systems, .{ .rounds = &short, .pcs_opening = empty_opening }, &honest_x));

    // What a real proof looks like: a committed round, several rounds.
    const committed = [_]protocol.RoundMessage{.{ .commitment = [_]Element{el(1)} ** 8, .cells = &honest_y }};
    try std.testing.expectError(error.NotADummyProof, verifier.verify(spec, systems, .{ .rounds = &committed, .pcs_opening = empty_opening }, &honest_x));
    const two = [_]protocol.RoundMessage{ honest_rounds[0], honest_rounds[0] };
    try std.testing.expectError(error.NotADummyProof, verifier.verify(spec, systems, .{ .rounds = &two, .pcs_opening = empty_opening }, &honest_x));
}

test "pair: equal statements verify, different statements are rejected" {
    const a = verifier.VerifyInput{ .proof = honest_proof, .public_inputs = &honest_x };
    try verifier.verifyPair(spec, systems, a, a);

    const other_x = [_]Scalar{ .{ .base = el(2) }, honest_x[1] };
    const other_y = [_]Scalar{ .{ .base = el(32) }, honest_y[1] };
    const other_rounds = [_]protocol.RoundMessage{.{ .cells = &other_y }};
    const b = verifier.VerifyInput{ .proof = .{ .rounds = &other_rounds, .pcs_opening = empty_opening }, .public_inputs = &other_x };
    try verifier.verify(spec, systems, b.proof, b.public_inputs);
    try std.testing.expectError(error.InconsistentPublicInputs, verifier.verifyPair(spec, systems, a, b));
}

test "pair statement encoding is pinned" {
    const statement = [_]Scalar{ .{ .base = el(0x01020304) }, .{ .ext = extOf(.{ 1, 2, 3, 4, 5, 6 }) } };
    var out: [verifier.pairStatementEncodedLen(2)]u8 = undefined;
    try verifier.encodePairStatement(&statement, &out);

    const expected = "lineth-verifier-dummy-pair-v1" ++
        "\x00\x00\x00\x00\x00\x00\x00\x02" ++
        "\x01\x02\x03\x04" ++ "\x00\x00\x00\x00" ** 5 ++
        "\x00\x00\x00\x01\x00\x00\x00\x02\x00\x00\x00\x03\x00\x00\x00\x04\x00\x00\x00\x05\x00\x00\x00\x06";
    try std.testing.expectEqualSlices(u8, expected, &out);

    var wrong: [3]u8 = undefined;
    try std.testing.expectError(error.InvalidPairStatementBuffer, verifier.encodePairStatement(&statement, &wrong));
}

// ── Go-written proof images ──────────────────────────────────────────────────

/// The base address the fixtures are encoded for: prover-ray's GuestBase.
const encoded_base: usize = 0x08800000;

fn Aligned(comptime bytes: []const u8) type {
    return struct { buf: [bytes.len]u8 align(8) = bytes[0..bytes.len].* };
}

test "an honest Go-written image verifies" {
    var img = Aligned(@embedFile("dummy_image")){};
    try std.testing.expectEqual(@as(u8, 0), api.verifyImage(&img.buf, img.buf.len, encoded_base));
}

test "an image whose public inputs were changed after proving is rejected" {
    var img = Aligned(@embedFile("dummy_image_tampered")){};
    try std.testing.expectEqual(@as(u8, 1), api.verifyImage(&img.buf, img.buf.len, encoded_base));
}

test "an honest Go-written pair image verifies and yields the pair statement" {
    var img = Aligned(@embedFile("dummy_pair_image")){};
    var out: [api.statement_len]u8 = undefined;
    try std.testing.expectEqual(@as(u8, 0), api.verifyPairImage(&img.buf, img.buf.len, encoded_base, &out));
    try std.testing.expect(std.mem.startsWith(u8, &out, verifier.pair_statement_domain));
}

test "a pair image of two different statements is rejected" {
    var img = Aligned(@embedFile("dummy_pair_image_mismatch")){};
    var out: [api.statement_len]u8 = undefined;
    try std.testing.expectEqual(@as(u8, 1), api.verifyPairImage(&img.buf, img.buf.len, encoded_base, &out));
}

test "a real verifier-ray proof is rejected" {
    // verifier-ray/codegen/abicheck's fixture, encoded for its own base: it
    // loads (same image format) but is not a dummy proof.
    var img = Aligned(@embedFile("real_image")){};
    try std.testing.expectEqual(@as(u8, 1), api.verifyImage(&img.buf, img.buf.len, 0x400000000));
}

// ── Malformed images ─────────────────────────────────────────────────────────

const Honest = Aligned(@embedFile("dummy_image"));

/// Resets img to the honest image, relocated in place (its pointers refer to
/// img's own address, so img must not be copied afterwards).
fn resetRelocated(img: *Honest) void {
    img.* = .{};
    image_relocation.rebase(&img.buf, img.buf.len, encoded_base, @intFromPtr(&img.buf));
}

fn validate(img: *const Honest) image_relocation.Error!void {
    return image_relocation.validateImage(&img.buf, img.buf.len, @intFromPtr(&img.buf));
}

fn offsetAt(img: *const Honest, at: usize) usize {
    return std.mem.readInt(u64, img.buf[at..][0..8], .little) - @intFromPtr(&img.buf);
}

test "the relocated honest image validates" {
    var img: Honest = undefined;
    resetRelocated(&img);
    try validate(&img);
}

test "a truncated image is rejected" {
    var img: Honest = undefined;
    resetRelocated(&img);
    try std.testing.expectError(error.PointerPastImage, image_relocation.validateImage(&img.buf, 64, @intFromPtr(&img.buf)));
    try std.testing.expectError(error.PointerPastImage, image_relocation.validateImage(&img.buf, img.buf.len - 1, @intFromPtr(&img.buf)));
}

test "a pointer or length outside the image is rejected" {
    var img: Honest = undefined;
    resetRelocated(&img);
    std.mem.writeInt(u64, img.buf[128..136], @intFromPtr(&img.buf) - 8, .little); // public_inputs.ptr
    try std.testing.expectError(error.PointerBeforeImage, validate(&img));

    resetRelocated(&img);
    std.mem.writeInt(u64, img.buf[136..144], 1 << 40, .little); // public_inputs.len
    try std.testing.expectError(error.PointerPastImage, validate(&img));
}

test "a bad scalar tag, commitment flag or non-canonical limb is rejected" {
    var img: Honest = undefined;

    resetRelocated(&img);
    const pi = offsetAt(&img, 128);
    img.buf[pi + 24] = 2;
    try std.testing.expectError(error.PointerPastImage, validate(&img));

    resetRelocated(&img);
    std.mem.writeInt(u32, img.buf[pi..][0..4], dummy.field.modulus, .little);
    try std.testing.expectError(error.PointerPastImage, validate(&img));

    resetRelocated(&img);
    img.buf[offsetAt(&img, 0) + 48] = 2; // round 0's commitment presence flag
    try std.testing.expectError(error.PointerPastImage, validate(&img));
}

test "the embedded circuit has the Go circuit's public-input count" {
    // prover-ray/dummy's NumPublicInputs; the honest fixture is made with it.
    try std.testing.expectEqual(@as(usize, 40), dummy.system_0_public_input.refs.len);
}

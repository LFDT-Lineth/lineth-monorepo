const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const program_vk = verifier_ray.query.program_vk;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

// Hand-built cases pinning the program-VK check directly via ScalarRef/round
// lookups into a runtime ctx, mirroring shared_randomness_test.zig — so an
// adversary cannot bypass it by altering proof cells or commitments.

fn baseScalar(v: u32) protocol.Scalar {
    return .{ .base = field.Element.init(v) };
}

fn octuplet(seed: u32) [8]field.Element {
    var out: [8]field.Element = undefined;
    for (&out, 0..) |*e, i| e.* = field.Element.init(seed + @as(u32, @intCast(i)));
    return out;
}

fn vkToScalars(com: [8]field.Element) [8]protocol.Scalar {
    var out: [8]protocol.Scalar = undefined;
    for (&out, com) |*s, limb| s.* = .{ .base = limb };
    return out;
}

// makeCtx builds a 2-round Context: round 0 is the program round, carrying its
// commitment and the VK cells (the "public input" cells, already merged in by
// verifier.verify's bindRoundMessages by the time any sub-verifier runs, and
// living on round 0 as the prover's lazy cells do); round 1 carries the trace
// commitment.
fn makeCtx(
    com0: ?[8]field.Element,
    com1: [8]field.Element,
    vk: *const [8]protocol.Scalar,
    rounds_buf: *[2]protocol.RoundMessage,
) protocol.Context {
    rounds_buf.* = .{
        .{ .commitment = com0, .cells = vk },
        .{ .commitment = com1, .cells = &.{} },
    };
    return .{ .all_coins = &.{}, .rounds = rounds_buf };
}

const program_round_refs = blk: {
    var refs: [8]program_vk.ScalarRef = undefined;
    for (&refs, 0..) |*ref, i| {
        ref.* = .{ .round = 0, .index = i };
    }
    break :blk refs;
};
const program_round_system = program_vk.System{
    .commitment_round = .{ .round = 0, .has_commitment = true },
    .vk_refs = &program_round_refs,
};

test "program vk accepts the program-round commitment" {
    const com0 = octuplet(1);
    const vk = vkToScalars(com0);

    var rounds_buf: [2]protocol.RoundMessage = undefined;
    try program_vk.verify(program_round_system, makeCtx(com0, octuplet(100), &vk, &rounds_buf));
}

test "program vk rejects a claimed VK limb that differs from the commitment" {
    const com0 = octuplet(1);
    var vk = vkToScalars(com0);
    // As if the prover claimed the VK of another program.
    vk[7] = baseScalar(0xDEAD);

    var rounds_buf: [2]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ProgramVKMismatch,
        program_vk.verify(program_round_system, makeCtx(com0, octuplet(100), &vk, &rounds_buf)),
    );
}

test "program vk rejects a VK bound to another round's commitment" {
    // The claimed VK is the trace-round commitment: only the program round's
    // commitment identifies the program.
    const com1 = octuplet(100);
    const vk = vkToScalars(com1);

    var rounds_buf: [2]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ProgramVKMismatch,
        program_vk.verify(program_round_system, makeCtx(octuplet(1), com1, &vk, &rounds_buf)),
    );
}

test "program vk rejects an extension-field-encoded VK limb" {
    const com0 = octuplet(1);
    var vk = vkToScalars(com0);
    // Same numeric value, lifted to the extension: a verifier that lifts via
    // toExt() would wrongly accept it.
    vk[2] = .{ .ext = ext.Ext.lift(com0[2]) };

    var rounds_buf: [2]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ProgramVKNotBaseField,
        program_vk.verify(program_round_system, makeCtx(com0, octuplet(100), &vk, &rounds_buf)),
    );
}

test "program vk rejects a program round missing its commitment" {
    const vk = vkToScalars(octuplet(1));

    var rounds_buf: [2]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.MissingRoundCommitment,
        program_vk.verify(program_round_system, makeCtx(null, octuplet(100), &vk, &rounds_buf)),
    );
}

test "empty program-vk system verifies trivially" {
    const empty_ctx: protocol.Context = .{ .all_coins = &.{}, .rounds = &[_]protocol.RoundMessage{} };
    try program_vk.verify(program_vk.System{}, empty_ctx);
}

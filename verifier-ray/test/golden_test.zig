const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vectors = @import("test_vectors");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const poseidon2 = verifier_ray.crypto.poseidon2;
const lagrange = verifier_ray.pcs.lagrange;
const polynomial = verifier_ray.pcs.polynomial;

test "koalabear base field matches prover-ray golden cases" {
    for (vectors.field_cases) |case| {
        const a = elem(case.a);
        const b = elem(case.b);

        try expectElem(a.add(b), case.add);
        try expectElem(a.sub(b), case.sub);
        try expectElem(a.mul(b), case.mul);
        try expectElem(a.square(), case.square_a);
        try expectElem(a.neg(), case.neg_a);
        try std.testing.expectEqualSlices(u8, &case.bytes_a, &a.toBytes());

        if (case.has_inv_a) {
            try expectElem(a.inverse(), case.inv_a);
        }
        if (case.has_div_ab) {
            try expectElem(a.div(b), case.div_ab);
        }
    }
}

test "koalabear extension field matches prover-ray golden cases" {
    for (vectors.ext_cases) |case| {
        const a = extElem(case.a);
        const b = extElem(case.b);
        const scalar = elem(case.scalar);

        try expectExt(a.add(b), case.add);
        try expectExt(a.sub(b), case.sub);
        try expectExt(a.mul(b), case.mul);
        try expectExt(a.square(), case.square_a);
        try expectExt(a.neg(), case.neg_a);
        try expectExt(a.mulByBase(scalar), case.mul_by_base);
        try std.testing.expectEqualSlices(u8, &case.bytes_a, &a.toBytes());

        if (case.has_inv_a) {
            try expectExt(a.inverse(), case.inv_a);
        }
    }
}

test "canonical polynomial evaluation matches prover-ray golden cases" {
    for (vectors.canonical_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(polynomial.evaluateBaseCanonical(coeffs[0..case.coeffs.len], elem(case.point)), case.expected);
    }

    for (vectors.canonical_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateBaseCanonicalAtExt(coeffs[0..case.coeffs.len], extElem(case.point)),
            case.expected,
        );
    }

    for (vectors.canonical_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateExtCanonicalAtBase(coeffs[0..case.coeffs.len], elem(case.point)),
            case.expected,
        );
    }

    for (vectors.canonical_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateExtCanonical(coeffs[0..case.coeffs.len], extElem(case.point)),
            case.expected,
        );
    }
}

test "lagrange polynomial evaluation matches prover-ray golden cases" {
    for (vectors.lagrange_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(try lagrange.evaluateBaseAtBase(coeffs[0..case.coeffs.len], elem(case.point)), case.expected);
    }

    for (vectors.lagrange_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateBaseAtExt(coeffs[0..case.coeffs.len], extElem(case.point)),
            case.expected,
        );
    }

    for (vectors.lagrange_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateExtAtBase(coeffs[0..case.coeffs.len], elem(case.point)),
            case.expected,
        );
    }

    for (vectors.lagrange_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateExtAtExt(coeffs[0..case.coeffs.len], extElem(case.point)),
            case.expected,
        );
    }
}

test "lagrange evaluation returns domain value at roots of unity" {
    const base_values = [_]field.Element{
        elem(3),
        elem(1),
        elem(4),
        elem(1),
    };
    const ext_values = [_]ext.Ext{
        extElem(.{ 3, 1, 4, 1 }),
        extElem(.{ 5, 9, 2, 6 }),
        extElem(.{ 5, 3, 5, 8 }),
        extElem(.{ 9, 7, 9, 3 }),
    };

    const omega = try field.rootOfUnityBy(base_values.len);
    var domain_point = field.Element.one();
    for (base_values, ext_values) |base_value, ext_value| {
        try expectElem(try lagrange.evaluateBaseAtBase(&base_values, domain_point), base_value.value);
        try expectExt(try lagrange.evaluateBaseAtExt(&base_values, ext.Ext.lift(domain_point)), .{
            base_value.value,
            0,
            0,
            0,
        });
        try expectExt(try lagrange.evaluateExtAtBase(&ext_values, domain_point), extValues(ext_value));
        try expectExt(try lagrange.evaluateExtAtExt(&ext_values, ext.Ext.lift(domain_point)), extValues(ext_value));
        domain_point = domain_point.mul(omega);
    }
}

test "poseidon2 compression and merkle-damgard match prover-ray golden cases" {
    for (vectors.poseidon_compress_cases) |case| {
        try expectDigest(poseidon2.compress(digest(case.left), digest(case.right)), case.expected);
    }

    for (vectors.poseidon_md_cases) |case| {
        var h = poseidon2.MDHasher.init();
        var message: [32]field.Element = undefined;
        fillElems(&message, case.message);
        h.writeElements(message[0..case.message.len]);
        try expectDigest(h.sumElement(), case.expected);
    }
}

test "fiat-shamir transcript matches prover-ray golden cases" {
    for (vectors.fiat_shamir_cases) |case| {
        var transcript = fiat_shamir.Transcript.init();

        var base_updates: [32]field.Element = undefined;
        fillElems(&base_updates, case.base_updates);
        transcript.updateElements(base_updates[0..case.base_updates.len]);

        var ext_updates: [16]ext.Ext = undefined;
        fillExts(&ext_updates, case.ext_updates);
        transcript.updateExt(ext_updates[0..case.ext_updates.len]);

        try expectDigest(transcript.randomField(), case.random_field);
        try expectExt(transcript.randomExt(), case.random_ext);
    }
}

fn elem(value: u32) field.Element {
    return field.Element.init(value);
}

fn extElem(limbs: [4]u32) ext.Ext {
    return .{ .limbs = .{
        elem(limbs[0]),
        elem(limbs[1]),
        elem(limbs[2]),
        elem(limbs[3]),
    } };
}

fn digest(values: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, values) |*dst, value| {
        dst.* = elem(value);
    }
    return out;
}

fn fillElems(out: []field.Element, values: []const u32) void {
    for (values, 0..) |value, i| {
        out[i] = elem(value);
    }
}

fn fillExts(out: []ext.Ext, values: []const [4]u32) void {
    for (values, 0..) |value, i| {
        out[i] = extElem(value);
    }
}

fn expectElem(actual: field.Element, expected: u32) !void {
    try std.testing.expectEqual(expected, actual.value);
}

fn expectExt(actual: ext.Ext, expected: [4]u32) !void {
    for (actual.limbs, expected) |actual_limb, expected_limb| {
        try expectElem(actual_limb, expected_limb);
    }
}

fn extValues(value: ext.Ext) [4]u32 {
    return .{
        value.limbs[0].value,
        value.limbs[1].value,
        value.limbs[2].value,
        value.limbs[3].value,
    };
}

fn expectDigest(actual: poseidon2.Digest, expected: [8]u32) !void {
    for (actual, expected) |actual_limb, expected_limb| {
        try expectElem(actual_limb, expected_limb);
    }
}

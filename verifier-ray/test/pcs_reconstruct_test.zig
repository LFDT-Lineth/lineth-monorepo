const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const reconstruct = verifier_ray.pcs.reconstruct;
const layout_mod = verifier_ray.pcs.layout;
const pl = verifier_ray.pcs.paired_leaf;

fn e(v: u32) ext.Ext {
    return ext.Ext.fromUints(.{ v, v +% 1, v +% 2, v +% 3, v +% 4, v +% 5 });
}

// ── shiftedPoint ─────────────────────────────────────────────────────────────

test "shiftedPoint: shift 0 is zeta; size-2 shift 1 is -zeta" {
    const zeta = e(123);
    try std.testing.expect((try reconstruct.shiftedPoint(1, 0, zeta)).eql(zeta));
    // omega_2 = -1, so zeta·omega^1 = -zeta.
    try std.testing.expect((try reconstruct.shiftedPoint(1, 1, zeta)).eql(zeta.neg()));
    // size 2^0: only shift 0, returns zeta.
    try std.testing.expect((try reconstruct.shiftedPoint(0, 0, zeta)).eql(zeta));
}

test "shiftedPoint: size-4 rotations are zeta times powers of omega_4" {
    const zeta = e(9);
    const omega = try field.rootOfUnityBy(4);
    for (0..4) |shift| {
        const got = try reconstruct.shiftedPoint(2, shift, zeta);
        const want = zeta.mulByBase(omega.pow(@as(u64, shift)));
        try std.testing.expect(got.eql(want));
    }
    try std.testing.expectError(reconstruct.Error.ShiftOutOfRange, reconstruct.shiftedPoint(2, 4, zeta));
}

// ── quotientAtValue ──────────────────────────────────────────────────────────

test "quotientAtValue: single claim equals (value - y)/(x - z)" {
    const value = e(50);
    const x = e(7);
    const claim = reconstruct.Claim{ .point = e(3), .value = e(20) };
    const got = try reconstruct.quotientAtValue(value, x, &.{claim});
    const want = value.sub(claim.value).mul(x.sub(claim.point).inverse());
    try std.testing.expect(got.eql(want));
}

test "quotientAtValue: sums over multiple claims" {
    const value = e(100);
    const x = e(11);
    const claims = [_]reconstruct.Claim{
        .{ .point = e(2), .value = e(30) },
        .{ .point = e(5), .value = e(40) },
    };
    const got = try reconstruct.quotientAtValue(value, x, &claims);
    var want = ext.Ext.zero();
    for (claims) |c| {
        want = want.add(value.sub(c.value).mul(x.sub(c.point).inverse()));
    }
    try std.testing.expect(got.eql(want));
}

test "quotientAtValue: errors when a claim point equals the query point" {
    const x = e(7);
    const claim = reconstruct.Claim{ .point = e(7), .value = e(1) };
    try std.testing.expectError(
        reconstruct.Error.ClaimPointOnQueryPoint,
        reconstruct.quotientAtValue(e(2), x, &.{claim}),
    );
}

// ── rowValueForEntry ─────────────────────────────────────────────────────────

test "rowValueForEntry: base rows are lifted, ext rows returned directly" {
    const row = pl.RowOpening{
        .base = &.{ field.Element.init(7), field.Element.init(8) },
        .ext = &.{ e(100), e(200) },
    };
    const base_entry = layout_mod.DeepEntry{
        .batch_idx = 0,
        .size_log2 = 1,
        .row_idx = 1,
        .is_ext = false,
        .alpha_power = 0,
        .shifts = &.{0},
    };
    try std.testing.expect(reconstruct.rowValueForEntry(row, base_entry).eql(ext.Ext.lift(field.Element.init(8))));

    const ext_entry = layout_mod.DeepEntry{
        .batch_idx = 0,
        .size_log2 = 1,
        .row_idx = 0,
        .is_ext = true,
        .alpha_power = 0,
        .shifts = &.{0},
    };
    try std.testing.expect(reconstruct.rowValueForEntry(row, ext_entry).eql(e(100)));
}

// ── reconstructQueryValueAt ──────────────────────────────────────────────────

test "reconstructQueryValueAt: two-entry Horner lands each on its alpha power" {
    const running = e(5);
    const alpha = e(3);
    const x = e(13);

    // entry index 0 → alpha_power 0; entry index 1 → alpha_power 1.
    const c0 = [_]reconstruct.Claim{.{ .point = e(1), .value = e(2) }};
    const c1 = [_]reconstruct.Claim{.{ .point = e(4), .value = e(6) }};
    const entries = [_]reconstruct.ResolvedEntry{
        .{ .row_value = e(10), .claims = &c0 },
        .{ .row_value = e(20), .claims = &c1 },
    };

    const got = try reconstruct.reconstructQueryValueAt(&entries, alpha, x, running);

    // Expected: running·α² + term1·α + term0, with term_k the column quotient.
    const term0 = try reconstruct.quotientAtValue(e(10), x, &c0);
    const term1 = try reconstruct.quotientAtValue(e(20), x, &c1);
    const want = running.mul(alpha).mul(alpha)
        .add(term1.mul(alpha))
        .add(term0);
    try std.testing.expect(got.eql(want));
}

test "reconstructQueryValueAt: empty bundle returns running unchanged" {
    const running = e(77);
    const got = try reconstruct.reconstructQueryValueAt(&.{}, e(3), e(9), running);
    try std.testing.expect(got.eql(running));
}

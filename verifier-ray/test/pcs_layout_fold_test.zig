const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const params_mod = verifier_ray.pcs.params;
const layout_mod = verifier_ray.pcs.layout;
const fold = verifier_ray.pcs.fold;
const tree = verifier_ray.pcs.tree;

fn e(v: u32) ext.Ext {
    return ext.Ext.fromUints(.{ v, v +% 1, v +% 2, v +% 3, v +% 4, v +% 5 });
}

// ── layout.buildLayout (comptime canonical enumeration) ──────────────────────

test "buildLayout: single batch, one size, base-then-ext, alpha powers" {
    const shapes = comptime [_]layout_mod.Shape{
        &.{ .{}, .{ .base_width = 2, .ext_width = 1 } }, // size 2^0 empty, 2^1 present
    };
    const shifts = comptime [_]layout_mod.BatchShifts{
        &.{
            .{}, // size 0 absent
            .{ .base = &.{ &.{0}, &.{ 0, 1 } }, .ext = &.{&.{0}} }, // size 1
        },
    };
    const l = comptime try layout_mod.buildLayout(&shapes, &shifts);

    try std.testing.expectEqual(@as(usize, 1), l.len); // only size 2^1 present
    const bundle = l[0];
    try std.testing.expectEqual(@as(u8, 1), bundle.size_log2);
    try std.testing.expectEqual(@as(usize, 3), bundle.entries.len); // 2 base + 1 ext

    // Canonical order: base row 0, base row 1, then ext row 0. Alpha powers 0,1,2.
    try std.testing.expect(!bundle.entries[0].is_ext);
    try std.testing.expectEqual(@as(usize, 0), bundle.entries[0].alpha_power);
    try std.testing.expectEqual(@as(usize, 0), bundle.entries[0].row_idx);
    try std.testing.expect(!bundle.entries[1].is_ext);
    try std.testing.expectEqual(@as(usize, 1), bundle.entries[1].alpha_power);
    try std.testing.expectEqual(@as(usize, 2), bundle.entries[1].shifts.len); // {0,1}
    try std.testing.expect(bundle.entries[2].is_ext);
    try std.testing.expectEqual(@as(usize, 2), bundle.entries[2].alpha_power);

    try std.testing.expectEqual(@as(usize, 1), bundle.batch_order.len);
    try std.testing.expectEqual(@as(usize, 0), bundle.batch_order[0]);
    try std.testing.expectEqual(@as(u8, 1), layout_mod.maxSizeLog2(l));
}

test "buildLayout: descending size order and per-size alpha reset" {
    // Two batches; batch 0 has sizes {2^0, 2^1}, batch 1 has size {2^1}.
    const shapes = comptime [_]layout_mod.Shape{
        &.{ .{ .base_width = 1 }, .{ .base_width = 1 } },
        &.{ .{}, .{ .base_width = 1 } },
    };
    const shifts = comptime [_]layout_mod.BatchShifts{
        &.{ .{ .base = &.{&.{0}} }, .{ .base = &.{&.{0}} } },
        &.{ .{}, .{ .base = &.{&.{0}} } },
    };
    const l = comptime try layout_mod.buildLayout(&shapes, &shifts);

    // Bundles in DESCENDING size: 2^1 first, then 2^0.
    try std.testing.expectEqual(@as(usize, 2), l.len);
    try std.testing.expectEqual(@as(u8, 1), l[0].size_log2);
    try std.testing.expectEqual(@as(u8, 0), l[1].size_log2);

    // Size 2^1: batch 0 then batch 1, alpha powers 0 then 1.
    try std.testing.expectEqual(@as(usize, 2), l[0].entries.len);
    try std.testing.expectEqual(@as(usize, 0), l[0].entries[0].batch_idx);
    try std.testing.expectEqual(@as(usize, 0), l[0].entries[0].alpha_power);
    try std.testing.expectEqual(@as(usize, 1), l[0].entries[1].batch_idx);
    try std.testing.expectEqual(@as(usize, 1), l[0].entries[1].alpha_power);
    try std.testing.expectEqual(@as(usize, 2), l[0].batch_order.len);

    // Size 2^0: only batch 0, alpha power resets to 0.
    try std.testing.expectEqual(@as(usize, 1), l[1].entries.len);
    try std.testing.expectEqual(@as(usize, 0), l[1].entries[0].alpha_power);
}

test "buildLayout: rejects empty shift list and out-of-range shift" {
    const bad_empty = comptime blk: {
        const shapes = [_]layout_mod.Shape{&.{.{ .base_width = 1 }}};
        const shifts = [_]layout_mod.BatchShifts{&.{.{ .base = &.{&.{}} }}};
        break :blk layout_mod.buildLayout(&shapes, &shifts);
    };
    try std.testing.expectError(layout_mod.Error.EmptyShiftList, bad_empty);

    const bad_range = comptime blk: {
        // size 2^0 = 1, shift 1 is out of [0,1).
        const shapes = [_]layout_mod.Shape{&.{.{ .base_width = 1 }}};
        const shifts = [_]layout_mod.BatchShifts{&.{.{ .base = &.{&.{1}} }}};
        break :blk layout_mod.buildLayout(&shapes, &shifts);
    };
    try std.testing.expectError(layout_mod.Error.ShiftOutOfRange, bad_range);
}

// ── fold.octupletToExt ───────────────────────────────────────────────────────

test "octupletToExt: round-trips the first six coords, requires zero tail" {
    const x = e(42);
    const oct = tree.Octuplet{
        x.B0.a0, x.B0.a1, x.B1.a0, x.B1.a1, x.B2.a0, x.B2.a1,
        field.Element.zero(), field.Element.zero(),
    };
    try std.testing.expect((try fold.octupletToExt(oct)).eql(x));

    var bad = oct;
    bad[6] = field.Element.init(1);
    try std.testing.expectError(fold.Error.OctupletTailNonZero, fold.octupletToExt(bad));
}

// ── fold.checkFolds ──────────────────────────────────────────────────────────
//
// Build a self-consistent fold chain by forward-applying the same recurrence
// checkFolds verifies, then confirm it accepts the valid chain and rejects
// mutations. num_rounds = 2, one query, an aux level introduced at round 0.

const MAX_ROUNDS = 4;

fn foldStepRef(self: ext.Ext, sib: ext.Ext, x_inv: field.Element, alpha: ext.Ext) ext.Ext {
    var sum = self.add(sib);
    const diff = self.sub(sib).mulByBase(x_inv).mul(alpha);
    sum = sum.add(diff);
    return sum.halve();
}

fn buildValidQuery(
    p: params_mod.Params,
    s: usize,
    alphas: []const ext.Ext,
    round0_self: ext.Ext,
    round0_sib: ext.Ext,
) !fold.ResolvedQuery(MAX_ROUNDS) {
    var rq = fold.ResolvedQuery(MAX_ROUNDS){};
    const num_rounds = p.numRounds();

    // Round 0 is driven by an aux level (the top polynomial).
    rq.aux_present[0] = true;
    rq.aux[0] = .{ .self = round0_self, .sibling = round0_sib };

    var x_inv = (try params_mod.domainPoint(@intCast(p.log_codeword_size), s)).inverse();
    var j: u8 = 0;
    while (j < num_rounds) : (j += 1) {
        const self = if (rq.aux_present[j]) rq.aux[j].self else rq.rounds[j].self;
        const sib = if (rq.aux_present[j]) rq.aux[j].sibling else rq.rounds[j].sibling;
        const folded = foldStepRef(self, sib, x_inv, alphas[j]);
        if (j < num_rounds - 1) {
            // The next running layer's self is the fold output. Its sibling is
            // arbitrary but must itself fold consistently only if a further
            // round uses it; for the LAST-but-one round the sibling is unused by
            // checkFolds' equality (only .self is compared), so any value works.
            rq.rounds[j + 1].self = folded;
            rq.rounds[j + 1].sibling = e(1000 + @as(u32, j)); // unread at the boundary compare
        } else {
            rq.final = folded;
        }
        x_inv = x_inv.square();
    }
    return rq;
}

test "checkFolds: accepts a valid 2-round chain, rejects a mutated final" {
    const p = params_mod.Params{
        .log_codeword_size = 5,
        .log_plaintext_size = 2,
        .num_queries = 1,
        .log_final_poly_size = 0,
    };
    try p.validate();
    try std.testing.expectEqual(@as(u8, 2), p.numRounds());

    const alphas = [_]ext.Ext{ e(7), e(11) };
    const positions = [_]usize{3};

    const rq = try buildValidQuery(p, positions[0], &alphas, e(100), e(200));
    const valid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq};
    try fold.checkFolds(MAX_ROUNDS, p, &valid, &alphas, &positions);

    // Mutate the final target → FinalMismatch.
    var rq_bad = rq;
    rq_bad.final = rq_bad.final.add(ext.Ext.one());
    const invalid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq_bad};
    try std.testing.expectError(fold.Error.FinalMismatch, fold.checkFolds(MAX_ROUNDS, p, &invalid, &alphas, &positions));
}

test "checkFolds: rejects a mutated intermediate running leaf" {
    const p = params_mod.Params{
        .log_codeword_size = 6,
        .log_plaintext_size = 3,
        .num_queries = 1,
        .log_final_poly_size = 0,
    };
    try p.validate();
    try std.testing.expectEqual(@as(u8, 3), p.numRounds());

    const alphas = [_]ext.Ext{ e(3), e(5), e(9) };
    const positions = [_]usize{5};

    const rq = try buildValidQuery(p, positions[0], &alphas, e(1), e(2));
    const valid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq};
    try fold.checkFolds(MAX_ROUNDS, p, &valid, &alphas, &positions);

    // Corrupt rounds[1].self (a running leaf checked by round-0's fold output).
    var rq_bad = rq;
    rq_bad.rounds[1].self = rq_bad.rounds[1].self.add(ext.Ext.one());
    const invalid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq_bad};
    try std.testing.expectError(fold.Error.FoldMismatch, fold.checkFolds(MAX_ROUNDS, p, &invalid, &alphas, &positions));
}

test "checkFolds: boundary-round aux must be constant" {
    // num_rounds = 1: round 0 folds, and an aux level at the boundary round 1
    // must have self == sibling.
    const p = params_mod.Params{
        .log_codeword_size = 4,
        .log_plaintext_size = 1,
        .num_queries = 1,
        .log_final_poly_size = 0,
    };
    try p.validate();
    try std.testing.expectEqual(@as(u8, 1), p.numRounds());

    const alphas = [_]ext.Ext{e(13)};
    const positions = [_]usize{2};

    var rq = try buildValidQuery(p, positions[0], &alphas, e(50), e(60));
    // Add a constant boundary aux (self == sibling) → accepted.
    rq.aux_present[1] = true;
    rq.aux[1] = .{ .self = e(77), .sibling = e(77) };
    const valid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq};
    try fold.checkFolds(MAX_ROUNDS, p, &valid, &alphas, &positions);

    // Non-constant boundary aux → BoundaryNotConstant.
    var rq_bad = rq;
    rq_bad.aux[1].sibling = e(78);
    const invalid = [_]fold.ResolvedQuery(MAX_ROUNDS){rq_bad};
    try std.testing.expectError(fold.Error.BoundaryNotConstant, fold.checkFolds(MAX_ROUNDS, p, &invalid, &alphas, &positions));
}

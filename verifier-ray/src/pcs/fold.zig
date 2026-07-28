//! FRI fold-recurrence checking for the verifier.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `fri.go`: `octupletToExt`, `inputPair`, `resolvedQuery`, and `checkFolds`.
//!
//! `checkFolds` is pure arithmetic over values the caller has already
//! authenticated (Merkle branches) and reconstructed (DEEP quotients): no
//! Merkle proof, row, or root ever passes through it. It walks the fold
//! recurrence for every query and checks each folded value against the next
//! layer's leaf (or, at the last round, the final polynomial target).

const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const tree = @import("tree.zig");
const params_mod = @import("params.zig");

pub const Error = error{
    OctupletTailNonZero,
    FoldMismatch,
    FinalMismatch,
    BoundaryNotConstant,
};

/// One fold round's conjugate values. `self` is the on-path evaluation, `sibling`
/// its conjugate. Mirrors prover-ray `inputPair`.
pub const InputPair = struct {
    self: ext.Ext = ext.Ext.zero(),
    sibling: ext.Ext = ext.Ext.zero(),
};

/// Every fold input for one query, already authenticated and reconstructed.
/// `rounds[j]` is the running-codeword pair at round j (rounds[0] is unused,
/// always zero). `aux[j]` (present iff `aux_present[j]`) is the pair of the
/// level introduced at round j; a level is always present at round 0.
/// `final` is the final-polynomial target for this query. Mirrors
/// prover-ray `resolvedQuery`, but with the `Aux` map expressed as two parallel
/// fixed arrays so the whole thing stays allocation-free (`max_rounds` bounds
/// the fold count; `numRounds + 1` slots cover the boundary round).
pub fn ResolvedQuery(comptime max_rounds: usize) type {
    return struct {
        rounds: [max_rounds + 1]InputPair = [_]InputPair{.{}} ** (max_rounds + 1),
        aux: [max_rounds + 1]InputPair = [_]InputPair{.{}} ** (max_rounds + 1),
        aux_present: [max_rounds + 1]bool = [_]bool{false} ** (max_rounds + 1),
        final: ext.Ext = ext.Ext.zero(),
    };
}

/// Decode an octuplet leaf into an extension element, requiring its last two
/// coordinates to be zero. Ports `fri.octupletToExt`.
pub fn octupletToExt(o: tree.Octuplet) Error!ext.Ext {
    if (!o[6].isZero() or !o[7].isZero()) return Error.OctupletTailNonZero;
    return .{
        .B0 = .{ .a0 = o[0], .a1 = o[1] },
        .B1 = .{ .a0 = o[2], .a1 = o[3] },
        .B2 = .{ .a0 = o[4], .a1 = o[5] },
    };
}

/// One FRI fold step in the extension field:
///
///   out = (self + sib)/2 + alpha · (self - sib)/(2x)
///
/// where `x_inv` is 1/x for the round's domain point x. Matches the arithmetic
/// in `fri.checkFolds`.
fn foldStep(self: ext.Ext, sib: ext.Ext, x_inv: field.Element, alpha: ext.Ext) ext.Ext {
    var sum = self.add(sib);
    var diff = self.sub(sib);
    diff = diff.mulByBase(x_inv);
    diff = diff.mul(alpha);
    sum = sum.add(diff);
    return sum.halve();
}

/// Verify the FRI fold recurrence for every query. `p` must already be
/// restricted to the opened top size (its `numRounds()` drives the loop).
/// `resolved[k]` carries query k's authenticated/reconstructed pairs;
/// `fold_alphas` has at least `numRounds` entries; `positions[k]` is query k's
/// codeword position. Ports `fri.checkFolds`.
///
/// `max_rounds` is the comptime capacity of the ResolvedQuery arrays and must be
/// >= p.numRounds().
pub fn checkFolds(
    comptime max_rounds: usize,
    p: params_mod.Params,
    resolved: []const ResolvedQuery(max_rounds),
    fold_alphas: []const ext.Ext,
    positions: []const usize,
) (Error || params_mod.Error)!void {
    const num_rounds = p.numRounds();
    std.debug.assert(num_rounds <= max_rounds);

    for (resolved, 0..) |rq, query_idx| {
        const s = positions[query_idx];
        var x_inv = (try params_mod.domainPoint(@intCast(p.log_codeword_size), s)).inverse();

        var j: u8 = 0;
        while (j < num_rounds) : (j += 1) {
            var self = rq.rounds[j].self;
            var sib = rq.rounds[j].sibling;
            // A level introduced at this round takes over as the primary pair
            // being folded (it already has the running codeword baked in).
            if (rq.aux_present[j]) {
                self = rq.aux[j].self;
                sib = rq.aux[j].sibling;
            }

            const folded = foldStep(self, sib, x_inv, fold_alphas[j]);

            // The fold output must equal the next layer's queried leaf, or — at
            // the last round — the final-polynomial target.
            if (j < num_rounds - 1) {
                if (!folded.eql(rq.rounds[j + 1].self)) return Error.FoldMismatch;
            } else {
                if (!folded.eql(rq.final)) return Error.FinalMismatch;
            }

            x_inv = x_inv.square();
        }

        // A level at the boundary round numRounds() is authenticated but not
        // folded: its batched DEEP quotient must be constant, i.e. equal at both
        // conjugate positions. (numRounds()==0 is handled by the caller.)
        if (num_rounds > 0 and rq.aux_present[num_rounds]) {
            if (!rq.aux[num_rounds].self.eql(rq.aux[num_rounds].sibling)) {
                return Error.BoundaryNotConstant;
            }
        }
    }
}

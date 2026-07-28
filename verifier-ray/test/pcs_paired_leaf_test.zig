const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const treemod = verifier_ray.pcs.tree;
const pl = verifier_ray.pcs.paired_leaf;

fn octEql(a: treemod.Octuplet, b: treemod.Octuplet) bool {
    for (a, b) |x, y| {
        if (!x.eql(y)) return false;
    }
    return true;
}

fn elem(v: u32) field.Element {
    return field.Element.init(v);
}

fn e(v: u32) ext.Ext {
    return ext.Ext.fromUints(.{ v, v + 1, v + 2, v + 3, v + 4, v + 5 });
}

// ── absorbLeafHeader / writeRowOpeningElements order ─────────────────────────

test "hashRowOpening: reproduces the exact prover-side header+element order" {
    const row = pl.RowOpening{
        .base = &.{ elem(7), elem(8) },
        .ext = &.{e(100)},
    };

    // Recompute the same digest the long way, in the documented order:
    // tag, baseWidth, extWidth, then base cells, then ext cells flattened.
    var h = poseidon2.MDHasher.init();
    h.writeElement(field.Element.init(pl.leaf_domain_tag));
    h.writeElement(field.Element.init(2)); // base width
    h.writeElement(field.Element.init(1)); // ext width
    h.writeElement(elem(7));
    h.writeElement(elem(8));
    const ee = e(100);
    h.writeElement(ee.B0.a0);
    h.writeElement(ee.B0.a1);
    h.writeElement(ee.B1.a0);
    h.writeElement(ee.B1.a1);
    h.writeElement(ee.B2.a0);
    h.writeElement(ee.B2.a1);
    const expected = h.sumDigest();

    try std.testing.expect(octEql(pl.hashRowOpening(row), expected));
}

test "hashRowOpening: shape tag distinguishes same values, different widths" {
    // 2 base + 0 ext vs 0 base + ... can't share values; instead compare
    // (1 base, 0 ext) vs (0 base, 0 ext-with-one-ext-cell) all-zero rows.
    const zero_base = pl.RowOpening{ .base = &.{elem(0)}, .ext = &.{} };
    const zero_ext = pl.RowOpening{ .base = &.{}, .ext = &.{ext.Ext.zero()} };
    // Different (baseWidth, extWidth) ⇒ different header ⇒ different digest,
    // even though every row value is zero.
    try std.testing.expect(!octEql(pl.hashRowOpening(zero_base), pl.hashRowOpening(zero_ext)));
}

test "hashAuxPair: even-before-odd order is independent of self_is_even" {
    const r0 = pl.RowOpening{ .base = &.{elem(1)}, .ext = &.{} };
    const r1 = pl.RowOpening{ .base = &.{elem(2)}, .ext = &.{} };

    // self_is_even = true  → writes (r0, r1); pair = {self=r0, sib=r1}
    const d_even = pl.hashAuxPair(.{ r0, r1 }, true);
    // self_is_even = false → writes (r1, r0); pair = {self=r1, sib=r0}
    // For it to be the SAME digest, the physical even row must come first in
    // both. So the caller passes the pair already ordered {even, odd} and the
    // flag only reflects which one is "self". Same even-first bytes ⇒ same hash.
    const d_odd = pl.hashAuxPair(.{ r1, r0 }, false);
    try std.testing.expect(octEql(d_even, d_odd));
}

// ── InputTreeOpening.recoverRoot on a hand-built multi-size tree ─────────────
//
// Table sizes {2, 4}: bottom (size 4) supplies the 4 tree leaves; the size-2
// table attaches as a conjugate-pair aux level one depth shallower (depth 0).
// Tree height: leaves.len == 2 (depths 0,1). Bottom → index 1, size-2 → index 0.

const BottomRows = [4]pl.RowOpening;
const AuxRows = [2]pl.RowOpening;

fn buildRoot(bottom: BottomRows, aux: AuxRows) treemod.Octuplet {
    // Bottom leaves: H(header || row_j) for j in 0..4  (Merkleize bottom path).
    var leaf: [4]treemod.Octuplet = undefined;
    for (0..4) |j| leaf[j] = pl.hashRowOpening(bottom[j]);

    // Internal nodes carry the size-2 aux pair. Merkleize digests the size-2
    // table as ONE pair (size/2 == 1) placed at the level; here that single
    // aux pair sits on the node above leaves {0,1} vs the node above {2,3}?
    // No: the size-2 table has 2 rows → its pair leaf is H(header||row0||row1),
    // a single aux digest attached to the WHOLE level (depth 0). Its two
    // physical rows are aux[0], aux[1]. The node-level aux is the pair digest.
    const aux_digest = pl.hashAuxPair(.{ aux[0], aux[1] }, true);

    // depth-1 internal nodes (no aux at depth 1 in this table): parents of the
    // leaf pairs. Merkleize attaches aux only where a table of that size exists;
    // size-4 would attach at depth 1 but there's no size-8 bottom-shift here, so
    // depth-1 nodes have nil aux.
    const n01 = treemod.hashNode(leaf[0], leaf[1], null);
    const n23 = treemod.hashNode(leaf[2], leaf[3], null);
    // root (depth 0) carries the size-2 aux pair.
    return treemod.hashNode(n01, n23, aux_digest);
}

test "recoverRoot: multi-size {2,4} tree, every bottom position, with aux" {
    const bottom = BottomRows{
        .{ .base = &.{elem(11)}, .ext = &.{} },
        .{ .base = &.{elem(12)}, .ext = &.{} },
        .{ .base = &.{elem(13)}, .ext = &.{} },
        .{ .base = &.{elem(14)}, .ext = &.{} },
    };
    const aux = AuxRows{
        .{ .base = &.{elem(21)}, .ext = &.{} },
        .{ .base = &.{elem(22)}, .ext = &.{} },
    };
    const root = buildRoot(bottom, aux);
    const leaf = [_]treemod.Octuplet{
        pl.hashRowOpening(bottom[0]), pl.hashRowOpening(bottom[1]),
        pl.hashRowOpening(bottom[2]), pl.hashRowOpening(bottom[3]),
    };

    for (0..4) |idx| {
        // Bottom pair for this query: (self=row idx, sib=row idx^1).
        const bottom_pair = pl.RowPair{ bottom[idx], bottom[idx ^ 1] };
        // Size-2 aux level: the on-path row is at base = idx/2, conjugate at
        // base^1. The opening carries (self, conjugate) in that order, and
        // hashAuxPair re-derives even-first from the query parity — matching the
        // single committed pair digest regardless of idx.
        const base = idx / 2;
        const aux_pair = pl.RowPair{ aux[base], aux[base ^ 1] };
        // Depth-0 sibling digest: the uncle node on the other side of the root.
        const sib_digest = if (idx < 2)
            treemod.hashNode(leaf[2], leaf[3], null) // n23
        else
            treemod.hashNode(leaf[0], leaf[1], null); // n01

        const opening = pl.InputTreeOpening{
            .siblings = &.{sib_digest},
            .leaves = &.{ aux_pair, bottom_pair },
        };
        try std.testing.expect(octEql(try opening.recoverRoot(idx), root));
    }
}

test "recoverRoot: bottom-only tree (no aux level) reconstructs" {
    const bottom = BottomRows{
        .{ .base = &.{elem(1)}, .ext = &.{e(50)} },
        .{ .base = &.{elem(2)}, .ext = &.{e(60)} },
        .{ .base = &.{elem(3)}, .ext = &.{e(70)} },
        .{ .base = &.{elem(4)}, .ext = &.{e(80)} },
    };
    const leaf = [_]treemod.Octuplet{
        pl.hashRowOpening(bottom[0]), pl.hashRowOpening(bottom[1]),
        pl.hashRowOpening(bottom[2]), pl.hashRowOpening(bottom[3]),
    };
    const n01 = treemod.hashNode(leaf[0], leaf[1], null);
    const n23 = treemod.hashNode(leaf[2], leaf[3], null);
    const root = treemod.hashNode(n01, n23, null);

    for (0..4) |idx| {
        const bottom_pair = pl.RowPair{ bottom[idx], bottom[idx ^ 1] };
        const sib_digest = if (idx < 2) n23 else n01;
        const opening = pl.InputTreeOpening{
            .siblings = &.{sib_digest},
            .leaves = &.{ null, bottom_pair }, // depth-0 aux absent
        };
        try std.testing.expect(octEql(try opening.recoverRoot(idx), root));
    }
}

test "recoverRoot: wrong sibling digest fails" {
    const bottom = BottomRows{
        .{ .base = &.{elem(1)}, .ext = &.{} },
        .{ .base = &.{elem(2)}, .ext = &.{} },
        .{ .base = &.{elem(3)}, .ext = &.{} },
        .{ .base = &.{elem(4)}, .ext = &.{} },
    };
    const leaf = [_]treemod.Octuplet{
        pl.hashRowOpening(bottom[0]), pl.hashRowOpening(bottom[1]),
        pl.hashRowOpening(bottom[2]), pl.hashRowOpening(bottom[3]),
    };
    const n01 = treemod.hashNode(leaf[0], leaf[1], null);
    const n23 = treemod.hashNode(leaf[2], leaf[3], null);
    const root = treemod.hashNode(n01, n23, null);

    const bottom_pair = pl.RowPair{ bottom[0], bottom[1] };
    const bad = pl.InputTreeOpening{
        .siblings = &.{n01}, // wrong: should be n23 for idx 0
        .leaves = &.{ null, bottom_pair },
    };
    try std.testing.expect(!octEql(try bad.recoverRoot(0), root));
}

// ── levelIndex / pairAtLevel ─────────────────────────────────────────────────

test "levelIndex maps sizes to depths for a 4-leaf opening" {
    const bp = pl.RowPair{
        .{ .base = &.{elem(1)}, .ext = &.{} },
        .{ .base = &.{elem(2)}, .ext = &.{} },
    };
    const ap = pl.RowPair{
        .{ .base = &.{elem(3)}, .ext = &.{} },
        .{ .base = &.{elem(4)}, .ext = &.{} },
    };
    const opening = pl.InputTreeOpening{
        .siblings = &.{treemod.Octuplet{ elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0) }},
        .leaves = &.{ ap, bp },
    };
    // tree_leaves = 2^len(leaves) = 4. Bottom size 4 → last index (1).
    try std.testing.expectEqual(@as(usize, 1), try opening.levelIndex(4));
    // size 2 → ctz(2)-1 = 0.
    try std.testing.expectEqual(@as(usize, 0), try opening.levelIndex(2));

    // pairAtLevel returns the right pair.
    const got_bottom = try opening.pairAtLevel(4);
    try std.testing.expect(got_bottom[0].base[0].eql(elem(1)));
    const got_aux = try opening.pairAtLevel(2);
    try std.testing.expect(got_aux[0].base[0].eql(elem(3)));
}

test "levelIndex errors" {
    const bp = pl.RowPair{
        .{ .base = &.{elem(1)}, .ext = &.{} },
        .{ .base = &.{elem(2)}, .ext = &.{} },
    };
    const opening = pl.InputTreeOpening{
        .siblings = &.{},
        .leaves = &.{bp},
    };
    try std.testing.expectError(pl.Error.LevelSizeNotPowerOfTwo, opening.levelIndex(3));
    // tree has 2 leaves; size 4 exceeds it.
    try std.testing.expectError(pl.Error.LevelSizeExceedsTree, opening.levelIndex(4));
}

test "recoverRoot errors on missing bottom and malformed lengths" {
    const bp = pl.RowPair{
        .{ .base = &.{elem(1)}, .ext = &.{} },
        .{ .base = &.{elem(2)}, .ext = &.{} },
    };
    // Missing bottom (last leaf is null).
    try std.testing.expectError(pl.Error.MissingBottomLevel, (pl.InputTreeOpening{
        .siblings = &.{},
        .leaves = &.{null},
    }).recoverRoot(0));
    // siblings length must be leaves.len - 1.
    try std.testing.expectError(pl.Error.MalformedProof, (pl.InputTreeOpening{
        .siblings = &.{ treemod.Octuplet{ elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0) }, treemod.Octuplet{ elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0), elem(0) } },
        .leaves = &.{bp},
    }).recoverRoot(0));
}

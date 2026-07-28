//! Multi-size paired-leaf Merkle openings for the PCS input trees.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `pcs.go`: `RowOpening`, `RowPair`, `InputTreeOpening`, the leaf-hashing
//! contract (`absorbLeafHeader` / `hashRowOpening` / `hashAuxPair` — which MUST
//! byte-match `commitment.go`'s `Merkleize`), and the branch-folding logic
//! (`foldOneLevel`, `InputTreeOpening.RecoverRoot`, `levelIndex`, `pairAtLevel`).
//!
//! A `MultiSizeTable` commits rows of several sizes into ONE tree: the bottom
//! (largest) table's rows are the tree leaves, and every smaller table is
//! digested as conjugate pairs attached one tree depth shallower than its own
//! size, so it folds the same way the bottom leaves do. An opening therefore
//! carries, per level it touches, the conjugate row pair (`RowPair`) plus the
//! sibling digests needed to climb to the root.

const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const treemod = @import("tree.zig");

pub const Octuplet = treemod.Octuplet;

pub const Error = error{
    MissingBottomLevel,
    MalformedProof,
    PositionNotFullyConsumed,
    LevelSizeNotPowerOfTwo,
    LevelSizeExceedsTree,
    LevelAbsentFromBranch,
};

/// Domain-separation tag for Merkle leaves: the ASCII "Lfri_lf1" as a
/// big-endian u64, reduced mod p by `field.Element.init` exactly as prover-ray
/// `absorbLeafHeader` reduces it via gnark's SetUint64. Without this tag a table
/// with the same row values but a different (base,ext) shape would collide.
pub const leaf_domain_tag: u64 = 0x4c66_7269_5f6c_6631; // "Lfri_lf1"

/// One row preimage: the base and extension cells of a single committed row.
/// The verifier receives these as proof slices.
pub const RowOpening = struct {
    base: []const field.Element,
    ext: []const ext.Ext,

    pub fn matchesShape(self: RowOpening, base_width: usize, ext_width: usize) bool {
        return self.base.len == base_width and self.ext.len == ext_width;
    }
};

/// A conjugate row pair at one tree level: `pair[0]` is the on-path (self) row,
/// `pair[1]` its conjugate (sibling) row. Matches prover-ray `RowPair`.
pub const RowPair = [2]RowOpening;

/// One PCS input-tree opening for a single FRI query. `leaves[i]` is the
/// (optional) conjugate pair attached at tree depth `i`; `leaves[last]` is the
/// mandatory bottom-level pair. `siblings` holds the transmitted sibling digests
/// for the levels *above* the bottom, so `len(siblings) == len(leaves) - 1`
/// (the bottom level's own sibling digest is derived from its pair, not sent).
pub const InputTreeOpening = struct {
    siblings: []const Octuplet,
    leaves: []const ?RowPair,

    /// Re-derive the tree root from this opening at bottom-leaf position `idx`.
    /// Ports `fri.InputTreeOpening.RecoverRoot`.
    pub fn recoverRoot(self: InputTreeOpening, idx: usize) Error!Octuplet {
        const num_levels = self.leaves.len;
        if (num_levels == 0 or self.leaves[num_levels - 1] == null) return Error.MissingBottomLevel;
        if (self.siblings.len != num_levels - 1) return Error.MalformedProof;

        // Bottom level: hash its two rows directly and fold them together. The
        // bottom pair contributes no transmitted sibling and no aux digest.
        const bottom = self.leaves[num_levels - 1].?;
        var ancestor = hashRowOpening(bottom[0]);
        const sibling = hashRowOpening(bottom[1]);
        var res = foldOneLevel(ancestor, sibling, null, idx);
        ancestor = res.digest;
        var cur_pos = res.next_pos;

        // Climb the remaining levels, hashing in each level's optional aux pair.
        var i = num_levels - 1;
        while (i != 0) {
            i -= 1;
            res = foldOneLevel(ancestor, self.siblings[i], self.leaves[i], cur_pos);
            ancestor = res.digest;
            cur_pos = res.next_pos;
        }

        if (cur_pos > 0) return Error.PositionNotFullyConsumed;
        return ancestor;
    }

    /// Resolve a level of size `level_size` to its index into `leaves`. The
    /// bottom level keeps its own (unshifted) depth; every other level's pair
    /// attaches one depth shallower than its size. Ports `levelIndex`.
    pub fn levelIndex(self: InputTreeOpening, level_size: usize) Error!usize {
        if (level_size == 0 or level_size & (level_size - 1) != 0) return Error.LevelSizeNotPowerOfTwo;
        const tree_leaves = @as(usize, 1) << @intCast(self.leaves.len);
        if (level_size > tree_leaves) return Error.LevelSizeExceedsTree;
        if (level_size == tree_leaves) return self.leaves.len - 1;
        // one depth shallower than level_size: trailingZeros(level_size) - 1
        const tz = @ctz(level_size);
        if (tz == 0) return Error.LevelAbsentFromBranch;
        return tz - 1;
    }

    /// Return the conjugate pair at `level_size`. Ports `pairAtLevel`.
    pub fn pairAtLevel(self: InputTreeOpening, level_size: usize) Error!RowPair {
        const idx = try self.levelIndex(level_size);
        return self.leaves[idx] orelse Error.LevelAbsentFromBranch;
    }
};

/// Hash a single row preimage into a leaf digest: header then row elements.
/// MUST match prover-ray `Merkleize`'s bottom-leaf hashing. Ports
/// `fri.hashRowOpening`.
pub fn hashRowOpening(row: RowOpening) Octuplet {
    var hasher = poseidon2.MDHasher.init();
    absorbLeafHeader(&hasher, row.base.len, row.ext.len);
    writeRowOpeningElements(&hasher, row);
    return hasher.sumDigest();
}

/// Hash a conjugate pair into one aux/leaf digest, writing the header once then
/// the even row before the odd row (regardless of which is `self`), matching
/// prover-ray `Merkleize`'s pair hashing. `self_is_even` selects the order.
/// Ports `fri.hashAuxPair`.
pub fn hashAuxPair(pair: RowPair, self_is_even: bool) Octuplet {
    var hasher = poseidon2.MDHasher.init();
    absorbLeafHeader(&hasher, pair[0].base.len, pair[0].ext.len);
    if (self_is_even) {
        writeRowOpeningElements(&hasher, pair[0]);
        writeRowOpeningElements(&hasher, pair[1]);
    } else {
        writeRowOpeningElements(&hasher, pair[1]);
        writeRowOpeningElements(&hasher, pair[0]);
    }
    return hasher.sumDigest();
}

/// Write the leaf domain tag, base width, and ext width into the hasher before
/// any row values. Ports `fri.absorbLeafHeader`. Prover and verifier MUST call
/// this identically.
pub fn absorbLeafHeader(hasher: *poseidon2.MDHasher, base_width: usize, ext_width: usize) void {
    hasher.writeElement(field.Element.init(leaf_domain_tag));
    hasher.writeElement(field.Element.init(@as(u64, base_width)));
    hasher.writeElement(field.Element.init(@as(u64, ext_width)));
}

/// Absorb one row's elements: every base cell (1 element), then every ext cell
/// flattened as [B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1]. Ports
/// `fri.writeRowOpeningElements` / `commitment.go` `writeRowElements`.
pub fn writeRowOpeningElements(hasher: *poseidon2.MDHasher, row: RowOpening) void {
    for (row.base) |b| hasher.writeElement(b);
    for (row.ext) |e| {
        hasher.writeElement(e.B0.a0);
        hasher.writeElement(e.B0.a1);
        hasher.writeElement(e.B1.a0);
        hasher.writeElement(e.B1.a1);
        hasher.writeElement(e.B2.a0);
        hasher.writeElement(e.B2.a1);
    }
}

const FoldResult = struct { digest: Octuplet, next_pos: usize };

/// Fold one Merkle level: order the running digest and sibling by the parity of
/// the running position, hash in the optional aux pair, and shift the position.
/// Ports `fri.foldOneLevel`.
fn foldOneLevel(ancestor: Octuplet, sibling: Octuplet, aux: ?RowPair, cur_pos: usize) FoldResult {
    const self_is_even = cur_pos & 1 == 0;
    var left = ancestor;
    var right = sibling;
    if (!self_is_even) {
        left = sibling;
        right = ancestor;
    }
    var aux_digest: ?Octuplet = null;
    if (aux) |a| aux_digest = hashAuxPair(a, self_is_even);
    return .{ .digest = treemod.hashNode(left, right, aux_digest), .next_pos = cur_pos >> 1 };
}

const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("poseidon2.zig");

/// Merkle authentication for a running FRI layer: a plain complete binary
/// tree over Poseidon2 octuplet leaves (prover-ray's `newCompleteBinaryTree` /
/// `buildTreeExt`), with no auxiliary siblings. This is the running-layer
/// half of prover-ray's `tree.go` `Branch`; the input-tree half below (row-
/// preimage branches over the multi-size aux-pair tree, prover-ray's
/// `pcs.go` `InputTreeOpening`) is the PCS layer's own commitment structure.
pub const Error = error{
    EmptyBranch,
    MissingBottomLevel,
    SiblingCountMismatch,
    InvalidLevelSize,
    LevelSizeTooLarge,
    LevelSizeAbsent,
};

/// A Merkle opening for one running-layer leaf. Unlike a conventional Merkle
/// proof, the branch carries the leaf itself: a FRI query reads the leaf
/// value directly out of the authenticated branch rather than through a
/// separate lookup.
pub const Branch = struct {
    /// The deepest leaf reachable through this branch.
    leaf: poseidon2.Digest,
    /// Sibling digests from the shallowest (just below the root) to the
    /// deepest; `siblings[siblings.len - 1]` is `leaf`'s own conjugate.
    siblings: []const poseidon2.Digest,

    /// Recovers the tree root by re-hashing `leaf` up to the root along
    /// `siblings`. `idx`'s bits, least significant first, decide at each
    /// level whether the running digest is the left or right child.
    pub fn recoverRoot(self: Branch, idx: usize) Error!poseidon2.Digest {
        if (self.siblings.len == 0) return Error.EmptyBranch;

        var ancestor = self.leaf;
        var curr_pos = idx;
        var i = self.siblings.len;
        while (i != 0) {
            i -= 1;
            const sibling = self.siblings[i];
            const left = if (curr_pos & 1 != 0) sibling else ancestor;
            const right = if (curr_pos & 1 != 0) ancestor else sibling;
            ancestor = hashNode(left, right, null);
            curr_pos >>= 1;
        }
        return ancestor;
    }
};

/// node = compress(left, right); if aux != null: node = compress(node, aux).
/// Mirrors prover-ray's `tree.go` `hashNode`, which calls the Poseidon2
/// compression function directly rather than the sponge.
pub fn hashNode(left: poseidon2.Digest, right: poseidon2.Digest, aux: ?poseidon2.Digest) poseidon2.Digest {
    var node = poseidon2.compress(left, right);
    if (aux) |a| node = poseidon2.compress(node, a);
    return node;
}

// ─── Input-tree branches: row preimages over the multi-size aux-pair tree ──
//
// Unlike the running-layer Branch above, a query here authenticates row
// *preimages*, not bare digests: the PCS/DEEP layer needs the actual base and
// extension values to reconstruct quotients, not just a leaf hash. Mirrors
// prover-ray's `pcs.go` `InputTreeOpening` / `RowPair` / `RowOpening`.

/// Domain-separates Merkle leaves so a table with the same row values but a
/// different (base, ext) width shape hashes to a different digest. Without
/// this, e.g. an all-zero base row and an all-zero ext row collide. Mirrors
/// prover-ray's `leafDomainTag` / `absorbLeafHeader`; prover and verifier must
/// call this identically or roots will not reconstruct.
const leaf_domain_tag: u64 = 0x4c66_7269_5f6c_6631; // "Lfri_lf1"

fn absorbLeafHeader(hasher: *poseidon2.MDHasher, base_width: usize, ext_width: usize) void {
    hasher.writeElements(&.{
        field.Element.init(leaf_domain_tag),
        field.Element.init(@as(u64, base_width)),
        field.Element.init(@as(u64, ext_width)),
    });
}

/// One committed row's preimage: prover-ray's `RowOpening`.
pub const RowOpening = struct {
    base: []const field.Element,
    ext: []const ext.Ext,
};

/// One level's conjugate row pair, as committed by `MultiSizeTable.Merkleize`:
/// prover-ray's `RowPair`.
pub const RowPair = [2]RowOpening;

fn writeRowOpeningElements(hasher: *poseidon2.MDHasher, row: RowOpening) void {
    hasher.writeElements(row.base);
    for (row.ext) |e| {
        hasher.writeElements(&.{ e.B0.a0, e.B0.a1, e.B1.a0, e.B1.a1, e.B2.a0, e.B2.a1 });
    }
}

/// Hashes a single row preimage into a leaf digest: prover-ray's
/// `hashRowOpening`, used for the bottom (largest) table's individual rows.
pub fn hashRowOpening(row: RowOpening) poseidon2.Digest {
    var hasher = poseidon2.MDHasher.init();
    absorbLeafHeader(&hasher, row.base.len, row.ext.len);
    writeRowOpeningElements(&hasher, row);
    return hasher.sumDigest();
}

/// Hashes an aux level's conjugate pair in the same even-before-odd order
/// `MultiSizeTable.Merkleize` used, regardless of which row is `self`. The
/// header is written once per pair (both rows share the same shape). Mirrors
/// prover-ray's `hashAuxPair`.
pub fn hashRowPair(pair: RowPair, self_is_even: bool) poseidon2.Digest {
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

/// A Merkle branch whose path leaves are opened as row preimages: prover-ray's
/// `InputTreeOpening`. `leaves[i]` (when present) holds the conjugate row
/// pair introduced at depth `i` -- one tree depth shallower than its own
/// size, per `Merkleize` -- except the last slot, which holds the bottom
/// (largest) table's own leaf pair at its native depth. `siblings` carries
/// every other level's already-hashed conjugate digest, one entry shorter
/// than `leaves`: the bottom level's own sibling digest is derived
/// (`hashRowOpening` of `leaves[len-1][1]`) rather than transmitted.
pub const InputTreeOpening = struct {
    siblings: []const poseidon2.Digest,
    leaves: []const ?RowPair,

    /// Folds this branch's rows up to the tree root. Mirrors prover-ray's
    /// `InputTreeOpening.RecoverRoot`.
    pub fn recoverRoot(self: InputTreeOpening, idx: usize) Error!poseidon2.Digest {
        const num_levels = self.leaves.len;
        if (num_levels == 0) return Error.MissingBottomLevel;
        const bottom = self.leaves[num_levels - 1] orelse return Error.MissingBottomLevel;
        if (self.siblings.len != num_levels - 1) return Error.SiblingCountMismatch;

        var step = foldOneLevel(hashRowOpening(bottom[0]), hashRowOpening(bottom[1]), null, idx);

        var i = num_levels - 1;
        while (i != 0) {
            i -= 1;
            step = foldOneLevel(step.ancestor, self.siblings[i], self.leaves[i], step.curr_pos);
        }
        return step.ancestor;
    }

    /// Resolves `level_size` to its index into `leaves`. Mirrors prover-ray's
    /// `levelIndex`: the bottom level keeps its own (unshifted) depth; every
    /// other level's pair attaches one depth shallower than its size.
    fn levelIndex(self: InputTreeOpening, level_size: usize) Error!usize {
        if (!isPowerOfTwo(level_size)) return Error.InvalidLevelSize;

        const tree_leaves = shl(1, self.leaves.len);
        if (level_size > tree_leaves) return Error.LevelSizeTooLarge;
        if (level_size == tree_leaves) return self.leaves.len - 1;

        const trailing = @ctz(level_size);
        if (trailing == 0) return Error.LevelSizeAbsent;
        return trailing - 1;
    }

    /// Returns the full conjugate pair at `level_size`, so callers can
    /// validate (or read) both the on-path row and its conjugate uniformly,
    /// regardless of whether this level is the top one. Mirrors prover-ray's
    /// `pairAtLevel`.
    pub fn pairAtLevel(self: InputTreeOpening, level_size: usize) Error!RowPair {
        const idx = try self.levelIndex(level_size);
        return self.leaves[idx] orelse return Error.LevelSizeAbsent;
    }
};

const FoldStep = struct { ancestor: poseidon2.Digest, curr_pos: usize };

/// One step of `recoverRoot`'s upward walk: hashes `aux` (if present) into an
/// aux digest via `hashRowPair` before combining with `hashNode`. Mirrors
/// prover-ray's `foldOneLevel`.
fn foldOneLevel(ancestor: poseidon2.Digest, sibling: poseidon2.Digest, aux: ?RowPair, curr_pos: usize) FoldStep {
    const self_is_even = curr_pos & 1 == 0;
    const left = if (self_is_even) ancestor else sibling;
    const right = if (self_is_even) sibling else ancestor;
    const aux_digest: ?poseidon2.Digest = if (aux) |pair| hashRowPair(pair, self_is_even) else null;
    return .{ .ancestor = hashNode(left, right, aux_digest), .curr_pos = curr_pos >> 1 };
}

fn isPowerOfTwo(value: usize) bool {
    return value != 0 and (value & (value - 1)) == 0;
}

fn shl(base_val: usize, amount: usize) usize {
    const shift: std.math.Log2Int(usize) = @intCast(amount);
    return base_val << shift;
}

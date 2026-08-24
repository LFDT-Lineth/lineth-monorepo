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
    IndexOutOfRange,
    InvalidCap,
    InvalidFrontier,
};

/// Selects the shared prefix for a binary Merkle tree. At least one path step
/// remains below the frontier so a query still carries the leaf and its
/// conjugate value.
pub fn capDepth(num_queries: usize, height: usize) usize {
    if (num_queries <= 1 or height <= 1) return 0;
    return @min(@bitSizeOf(usize) - @clz(num_queries - 1), height - 1);
}

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
        // All bits of the leaf position must have been consumed by the walk: an
        // `idx` larger than the tree's leaf count would leave residual high bits,
        // meaning the branch does not authenticate a leaf that exists in the tree.
        // Mirrors prover-ray's `tree.go` currPos>0 guard. Redundant when the
        // caller has already bounded `idx < 2^siblings.len`, but defense-in-depth.
        if (curr_pos != 0) return Error.IndexOutOfRange;
        return ancestor;
    }

    /// Authenticates the suffix of this branch against an already-authenticated
    /// frontier. The frontier depth is inferred from its power-of-two length.
    pub fn authenticateToCap(self: Branch, idx: usize, frontier: []const poseidon2.Digest) Error!void {
        _ = try frontierDepth(frontier);
        if (self.siblings.len == 0 or idx >> @intCast(self.siblings.len) >= frontier.len) {
            return Error.InvalidFrontier;
        }
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
        if (!poseidon2.eql(ancestor, frontier[curr_pos])) return Error.InvalidCap;
    }
};

/// A separately authenticated Merkle frontier. Depth zero is represented by
/// an empty proof cap; its root is supplied by the caller.
pub const MerkleCap = struct {
    nodes: []const poseidon2.Digest,
    aux: []const ?poseidon2.Digest,

    pub fn validate(self: MerkleCap, depth: usize) Error!void {
        if (depth == 0) {
            if (self.nodes.len != 0 or self.aux.len != 0) return Error.InvalidCap;
            return;
        }
        if (depth >= @bitSizeOf(usize)) return Error.InvalidCap;
        const count = @as(usize, 1) << @intCast(depth);
        if (self.nodes.len != count or self.aux.len != count - 1) return Error.InvalidCap;
    }

    fn recoverNode(self: MerkleCap, depth: usize, node_depth: usize, index: usize) Error!poseidon2.Digest {
        if (node_depth == depth) return self.nodes[index];
        const left = try self.recoverNode(depth, node_depth + 1, index * 2);
        const right = try self.recoverNode(depth, node_depth + 1, index * 2 + 1);
        const heap_index = (@as(usize, 1) << @intCast(node_depth)) - 1 + index;
        return hashNode(left, right, self.aux[heap_index]);
    }

    pub fn recoverRoot(self: MerkleCap, depth: usize) Error!poseidon2.Digest {
        try self.validate(depth);
        if (depth == 0) return Error.InvalidCap;
        return self.recoverNode(depth, 0, 0);
    }

    pub fn authenticate(self: MerkleCap, depth: usize, root: poseidon2.Digest) Error!void {
        if (depth == 0) {
            try self.validate(depth);
            return;
        }
        if (!poseidon2.eql(try self.recoverRoot(depth), root)) return Error.InvalidCap;
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
    writeRowOpeningElements(&hasher, row);
    return hasher.sumDigest();
}

/// Hashes an aux level's conjugate pair in the same even-before-odd order
/// `MultiSizeTable.Merkleize` used, regardless of which row is `self`. The
/// header is written once per pair (both rows share the same shape). Mirrors
/// prover-ray's `hashAuxPair`.
pub fn hashRowPair(pair: RowPair, self_is_even: bool) poseidon2.Digest {
    var hasher = poseidon2.MDHasher.init();
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
        // Every bit of the leaf position must be consumed (see Branch.recoverRoot).
        if (step.curr_pos != 0) return Error.IndexOutOfRange;
        return step.ancestor;
    }

    /// Authenticates the portion of a sparse row-opening branch below an
    /// already-authenticated input-tree frontier.
    pub fn authenticateToCap(self: InputTreeOpening, idx: usize, frontier: []const poseidon2.Digest) Error!void {
        const depth = try frontierDepth(frontier);
        const height = self.leaves.len;
        if (height == 0 or self.siblings.len != height - 1 - depth or depth >= height) {
            return Error.InvalidFrontier;
        }
        const bottom = self.leaves[height - 1] orelse return Error.MissingBottomLevel;
        for (self.leaves[0..depth]) |pair| {
            if (pair != null) return Error.InvalidCap;
        }
        var step = foldOneLevel(hashRowOpening(bottom[0]), hashRowOpening(bottom[1]), null, idx);
        var i = height - 1;
        while (i > depth) {
            i -= 1;
            step = foldOneLevel(step.ancestor, self.siblings[i - depth], self.leaves[i], step.curr_pos);
        }
        if (step.curr_pos >= frontier.len or !poseidon2.eql(step.ancestor, frontier[step.curr_pos])) {
            return Error.InvalidCap;
        }
    }

    /// Resolves `level_size` to its index into `leaves`. Mirrors prover-ray's
    /// `levelIndex`: the bottom level keeps its own (unshifted) depth; every
    /// other level's pair attaches one depth shallower than its size.
    fn levelIndex(self: InputTreeOpening, level_size: usize) Error!usize {
        if (!isPowerOfTwo(level_size)) return Error.InvalidLevelSize;
        // leaves.len is proof-controlled: bound it before shl's shift so an
        // oversized branch errors instead of overflow-trapping the cast.
        if (self.leaves.len >= @bitSizeOf(usize)) return Error.LevelSizeTooLarge;

        const tree_leaves = @as(usize, 1) << @as(u6, @intCast(self.leaves.len));
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

fn frontierDepth(frontier: []const poseidon2.Digest) Error!usize {
    if (!isPowerOfTwo(frontier.len)) return Error.InvalidFrontier;
    return @ctz(frontier.len);
}

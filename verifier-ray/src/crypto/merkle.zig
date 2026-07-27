const poseidon2 = @import("poseidon2.zig");

/// Merkle authentication for a running FRI layer: a plain complete binary
/// tree over Poseidon2 octuplet leaves (prover-ray's `newCompleteBinaryTree` /
/// `buildTreeExt`), with no auxiliary siblings. This is the running-layer
/// half of prover-ray's `tree.go` `Branch`; the input-tree half (row-preimage
/// branches over the multi-size aux-pair tree, `pcs.go`'s `InputTreeOpening`)
/// lands with the PCS layer.
pub const Error = error{EmptyBranch};

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

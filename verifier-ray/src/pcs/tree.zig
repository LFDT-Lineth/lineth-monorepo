//! Merkle branch verification for FRI running-layer trees.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `tree.go`: `hashNode`, the `Branch` opening shape, and `Branch.RecoverRoot`.
//! Tree *construction* (NewTree / newCompleteBinaryTree / OpenBranch) is
//! prover-side and omitted — the verifier only re-derives a root from a branch.

const commitment = @import("../crypto/commitment.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");

/// A tree node / leaf digest. Same as prover-ray `field.Octuplet`.
pub const Octuplet = commitment.Commitment; // [8]field.Element

pub const Error = error{
    MalformedProof,
    EmptyProof,
    PositionNotFullyConsumed,
};

/// Hash two child digests and an optional auxiliary leaf into a parent digest.
///
///   res = C(left, right)            when aux == null
///   res = C(C(left, right), aux)    when aux != null
///
/// C is the Poseidon2 compression function called directly (not MD hashing),
/// matching prover-ray `fri.hashNode`.
pub fn hashNode(left: Octuplet, right: Octuplet, aux: ?Octuplet) Octuplet {
    var res = poseidon2.compress(left, right);
    if (aux) |a| res = poseidon2.compress(res, a);
    return res;
}

/// A Merkle opening proof for a single leaf. Unlike a textbook Merkle proof the
/// branch carries the opened `leaf` itself (prover-ray opens every leaf of a
/// query in one branch). `siblings` runs greatest-uncle-first (just below the
/// root) down to the leaf's immediate sibling; `aux_siblings[i]` is the optional
/// auxiliary leaf hashed in at the same level as `siblings[i]`.
///
/// `len(siblings) == len(aux_siblings)` is required.
pub const Branch = struct {
    leaf: Octuplet,
    siblings: []const Octuplet,
    aux_siblings: []const ?Octuplet,

    /// Re-derive the tree root from this branch at 0-based leaf position `idx`.
    /// Ports `fri.Branch.RecoverRoot`. Folds bottom-up: at each level the parity
    /// bit of the running position decides whether the running digest is the
    /// left or right child.
    pub fn recoverRoot(self: Branch, idx: usize) Error!Octuplet {
        if (self.siblings.len != self.aux_siblings.len) return Error.MalformedProof;
        if (self.siblings.len == 0) return Error.EmptyProof;

        var ancestor = self.leaf;
        var cur_pos = idx;

        var i = self.siblings.len;
        while (i != 0) {
            i -= 1;
            var left = ancestor;
            var right = self.siblings[i];
            if (cur_pos & 1 != 0) {
                left = self.siblings[i];
                right = ancestor;
            }
            ancestor = hashNode(left, right, self.aux_siblings[i]);
            cur_pos >>= 1;
        }

        // Every bit of the position must have been consumed; a leftover means
        // the branch length disagrees with idx.
        if (cur_pos > 0) return Error.PositionNotFullyConsumed;
        return ancestor;
    }
};

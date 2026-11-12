//! EIP-4844 blob KZG proof verification without recomputing the commitment: the versioned hash of
//! a commitment, and the batch verification of blob proofs by Constantine. The verification runs
//! through the `lineth_zkvm_kzg_verify_blob_batch` accelerator of the zkvm providers, which owns the
//! KZG context.

const accelerators = @import("zkvm_accelerators");
const blob_payload = @import("blob_payload.zig");

pub const blob_len = blob_payload.blob_len;
pub const commitment_len = 48;
pub const proof_len = 48;

/// `0x01 ‖ sha256(commitment)[1..]`.
pub fn versionedHash(commitment: *const [commitment_len]u8) [32]u8 {
    var hash: [32]u8 = undefined;
    accelerators.sha256(commitment, &hash);
    hash[0] = 0x01;
    return hash;
}

/// Whether every proof opens its commitment to its blob, as the consensus specs'
/// `verify_blob_kzg_proof_batch` defines it; an empty batch verifies. Batches whose lengths differ,
/// a blob with an element of at least the scalar field's order, an invalid commitment or an invalid
/// proof do not verify.
pub fn verifyBatch(
    blobs: []const [blob_len]u8,
    commitments: []const [commitment_len]u8,
    proofs: []const [proof_len]u8,
) bool {
    return accelerators.kzgVerifyBlobBatch(blobs, commitments, proofs);
}

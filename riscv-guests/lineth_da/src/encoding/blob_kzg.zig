//! EIP-4844 blob KZG proof verification without recomputing the commitment: the versioned hash of
//! a commitment, and the verification of a blob proof by Constantine. The verification runs through
//! the `lineth_zkvm_kzg_verify_blob` accelerator of the zkvm providers, which owns the KZG context.

const accelerators = @import("zesu_accelerators");
const blob_payload = @import("blob_payload.zig");

pub const blob_len = blob_payload.blob_len;
pub const commitment_len = 48;
pub const proof_len = 48;

extern fn lineth_zkvm_kzg_verify_blob(
    blob: *const [blob_len]u8,
    commitment: *const [commitment_len]u8,
    proof: *const [proof_len]u8,
    verified: *bool,
) i32;

/// `0x01 ‖ sha256(commitment)[1..]`.
pub fn versionedHash(commitment: *const [commitment_len]u8) [32]u8 {
    var hash: [32]u8 = undefined;
    accelerators.sha256(commitment, &hash);
    hash[0] = 0x01;
    return hash;
}

/// Whether `proof` opens `commitment` to `blob`, i.e. whether `commitment` commits to `blob`. A blob
/// with an element of at least the scalar field's order, an invalid commitment or an invalid proof
/// does not verify.
pub fn verify(
    blob: *const [blob_len]u8,
    commitment: *const [commitment_len]u8,
    proof: *const [proof_len]u8,
) bool {
    var verified = false;
    return lineth_zkvm_kzg_verify_blob(blob, commitment, proof, &verified) == 0 and verified;
}

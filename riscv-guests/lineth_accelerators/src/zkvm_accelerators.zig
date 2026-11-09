//! The zkVM accelerator calls the guests make: the `zkvm_*` C symbols of the zkvm-standards
//! accelerator interface and Lineth's `lineth_zkvm_*` extensions, each behind a thin Zig wrapper.
//! The providers of this package define the symbols.

/// An EIP-4844 blob: 4096 field elements of 32 bytes.
pub const blob_len = 131072;
/// A compressed G1 point, the length of a KZG commitment and of a blob proof.
pub const commitment_len = 48;
pub const proof_len = 48;

extern fn zkvm_keccak256(data: [*]const u8, len: usize, output: *[32]u8) i32;
extern fn zkvm_sha256(data: [*]const u8, len: usize, output: *[32]u8) i32;
extern fn lineth_zkvm_kzg_verify_blob_batch(
    blobs: [*]const [blob_len]u8,
    commitments: [*]const [commitment_len]u8,
    proofs: [*]const [proof_len]u8,
    n: usize,
    verified: *bool,
) i32;

pub fn keccak256(data: []const u8, output: *[32]u8) void {
    _ = zkvm_keccak256(data.ptr, data.len, output);
}

/// `keccak256` returning the digest.
pub fn keccak256Digest(data: []const u8) [32]u8 {
    var digest: [32]u8 = undefined;
    keccak256(data, &digest);
    return digest;
}

pub fn sha256(data: []const u8, output: *[32]u8) void {
    _ = zkvm_sha256(data.ptr, data.len, output);
}

/// Whether every proof opens its commitment to its blob; false for batches of unequal lengths.
pub fn kzgVerifyBlobBatch(
    blobs: []const [blob_len]u8,
    commitments: []const [commitment_len]u8,
    proofs: []const [proof_len]u8,
) bool {
    if (blobs.len != commitments.len or blobs.len != proofs.len) return false;
    var verified = false;
    return lineth_zkvm_kzg_verify_blob_batch(blobs.ptr, commitments.ptr, proofs.ptr, blobs.len, &verified) == 0 and verified;
}

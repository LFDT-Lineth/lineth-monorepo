const field = @import("../field/koalabear.zig");
const poseidon2 = @import("poseidon2.zig");

/// Number of independent Poseidon2 output blocks representing one message.
/// Mirrors prover-ray's `multisethashing.chunkSize` (SIS security bound,
/// ≥128 bits for at most 2^16 insertions/removals).
pub const chunk_size = 41;

/// Number of field elements in the accumulator. Mirrors prover-ray's
/// `multisethashing.MSetHashSizeNumFieldElement`.
pub const size = chunk_size * poseidon2.block_size;

pub const MSetHash = [size]field.Element;

/// Ports prover-ray's `multisethashing.Hash(root)`: inserts `root` into a
/// fresh (zero) accumulator, i.e. `MSetHash{}.Insert(root...)`.
///
/// Only the `Hash` (insert-into-zero) path is ported — this codebase's only
/// caller (`query.shared_randomness`) never needs `Remove`/`Add`/`Combine`/
/// `Identity`/`ToSeed`.
pub fn hash(root: poseidon2.Digest) MSetHash {
    var hsh = poseidon2.MDHasher.init();
    hsh.writeElements(&root);

    const zeros = poseidon2.zeroDigest();
    var out: MSetHash = undefined;
    for (0..chunk_size) |i| {
        const chunk = hsh.sumDigest();
        @memcpy(out[i * poseidon2.block_size ..][0..poseidon2.block_size], &chunk);
        if (i < chunk_size - 1) hsh.writeElements(&zeros);
    }
    return out;
}

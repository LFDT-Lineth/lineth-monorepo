const poseidon2 = @import("../crypto/poseidon2.zig");

pub const Poseidon2CompressFn = *const fn (poseidon2.Digest, poseidon2.Digest) poseidon2.Digest;

pub const Backend = struct {
    poseidon2_compress: Poseidon2CompressFn,
};

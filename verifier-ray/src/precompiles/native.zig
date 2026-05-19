const interface = @import("interface.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");

pub const backend = interface.Backend{
    .poseidon2_compress = poseidon2.compress,
};

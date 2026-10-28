// Shared zkVM accelerator types, mirroring the type section of
// include/zkvm_accelerators.h.
//
// Copied from the upstream zkVM standard https://github.com/eth-act/zkvm-standards/blob/main/standards/c-interface-accelerators/zkvm_accelerators.h

pub const zkvm_status = enum(c_int) {
    ZKVM_EOK = 0, // Success
    ZKVM_EFAIL = -1, // Failure
};

pub const zkvm_bytes_16 = extern struct {
    data: [16]u8 align(8),
};

pub const zkvm_bytes_32 = extern struct {
    data: [32]u8 align(8),
};

pub const zkvm_bytes_48 = extern struct {
    data: [48]u8 align(8),
};

pub const zkvm_bytes_64 = extern struct {
    data: [64]u8 align(8),
};

pub const zkvm_bytes_96 = extern struct {
    data: [96]u8 align(8),
};

pub const zkvm_bytes_128 = extern struct {
    data: [128]u8 align(8),
};

pub const zkvm_bytes_192 = extern struct {
    data: [192]u8 align(8),
};

//! Namespace of every source file in the lineth_da package.

pub const zstd = @import("zstd.zig");
/// The zkVM accelerator interface the package's hashing, signature recovery and point evaluation go
/// through; the `zkvm_*` providers of `lineth_accelerators` define its symbols.
pub const accelerators = @import("zesu_accelerators");
/// The scalar field of BLS12-381, which the blob verification of `encoding` computes in.
pub const bls_scalar = @import("bls_scalar");
pub const encoding = @import("encoding.zig");
pub const decompress = @import("decompress.zig");

/// The zstd dictionary the data-availability frames are compressed against.
pub const dictionary_v2025_04_21 = @embedFile("zstd_dictionary_v2025_04_21");

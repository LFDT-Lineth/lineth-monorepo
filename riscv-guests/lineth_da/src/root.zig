//! Namespace of every source file in the lineth_da package.

pub const zstd_decoder = @import("zstd_decoder.zig");
pub const encoding = @import("encoding.zig");
pub const decompress = @import("decompress.zig");

/// The zstd dictionary the data-availability frames are compressed against.
pub const dictionary_v2025_04_21 = @embedFile("zstd_dictionary_v2025_04_21");

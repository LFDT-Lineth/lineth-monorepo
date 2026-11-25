//! The host layer over the package: the conflation compressor and the DA stream behind the C ABI of
//! the `lineth_da` shared library.

const std = @import("std");

pub const compressor = @import("compressor.zig");
pub const stream = @import("stream.zig");
pub const c_api = @import("c_api.zig");

/// The code of the library keeps no per-thread buffer for a signal stack.
pub const std_options: std.Options = .{ .signal_stack_size = null };

comptime {
    _ = c_api;
}

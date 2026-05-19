const interface = @import("interface.zig");
const native = @import("native.zig");

pub const backend: interface.Backend = native.backend;

const field = @import("../field/koalabear.zig");

pub const Error = error{Unsupported};

pub fn hash(_: []const field.Element) Error![8]field.Element {
    return Error.Unsupported;
}

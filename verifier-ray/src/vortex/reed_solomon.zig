const field = @import("../field/koalabear.zig");

pub const Error = error{Unsupported};

pub fn checkCodeword(_: []const field.Element) Error!void {
    return Error.Unsupported;
}

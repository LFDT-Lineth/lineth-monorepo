const custom_std = @import("custom_std.zig");

export fn main() noreturn {
    const a: i64 = 42;
    const b: i64 = 7;

    _ = a + b;
    _ = a - b;
    _ = a * b;
    _ = @divTrunc(a, b);
    _ = @rem(a, b);

    custom_std.exit(0);
}

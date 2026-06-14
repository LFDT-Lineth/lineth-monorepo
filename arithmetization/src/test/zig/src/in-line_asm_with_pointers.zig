const custom_std = @import("custom_std.zig");

export fn main() noreturn {

    const buf_0 = [_]u8{0} ** 0;
    const data_0: [*c]const u8 = &buf_0;

    asm volatile (
        \\mv a0, %[data_0]
        \\li a7, 93
        \\ecall
        :
        : [data_0] "r" (data_0),
    );
    unreachable;
}

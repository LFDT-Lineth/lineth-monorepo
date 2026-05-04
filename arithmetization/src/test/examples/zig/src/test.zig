export fn main() noreturn {
    const a: i64 = 42;
    const b: i64 = 7;

    _ = a + b;
    _ = a - b;
    _ = a * b;
    _ = @divTrunc(a, b);
    _ = @rem(a, b);

    // no OS to return to, signal halt via ecall
    asm volatile (
        \\li a0, 0   # exit code 0
        \\li a7, 93  # syscall number for exit
        \\ecall
    );
    unreachable;
}

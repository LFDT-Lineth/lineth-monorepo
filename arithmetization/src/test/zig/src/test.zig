const lineth_accel = @import("lineth_zkvm_accel");

export fn main() noreturn {
    const a: i64 = 42;
    const b: i64 = 7;

    _ = a + b;
    _ = a - b;
    _ = a * b;
    _ = @divTrunc(a, b);
    _ = @rem(a, b);

    lineth_accel.zkvm_exit(0);
}

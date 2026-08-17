// Zig implementation of the proposed zkVM standard runtime interface
// (include/zkvm_std.h).

// Halt guest execution and signal termination to the host. Never returns; the
// exit code is reported to the host (0 = success, non-zero = failure).
// See include/zkvm_std.h (zkvm_exit).
pub fn zkvm_exit(code: u32) callconv(.c) noreturn {
    // no OS to return to, signal halt via ecall
    asm volatile (
        \\mv a0, %[code]
        \\li a7, 93
        \\ecall
        :
        : [code] "r" (code),
    );
    unreachable;
}

pub fn panic() noreturn {
    zkvm_exit(1);
}

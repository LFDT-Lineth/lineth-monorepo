pub fn exit(code: u32) noreturn {
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
    exit(1);
}

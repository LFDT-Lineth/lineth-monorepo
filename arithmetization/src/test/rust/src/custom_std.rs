// Set stack pointer (SP) to the one defined in the linker script and call main
core::arch::global_asm!(
    ".global _start",
    "_start:",
    "la sp, _stack_start", // SP from linker script
    "call main",
);

// Exit the program with the given code using ecall
fn exit(code: i32) -> ! {
    unsafe {
        core::arch::asm!(
            "ecall",
            in("a0") code, // exit code
            in("a7") 93i32, // syscall number for exit (93)
            options(noreturn)
        );
    }
}

// Panic handler that exits with a specific code to indicate a panic occurred
// This is required for no_std environments
#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    exit(i32::MAX) // use max int to indicate panic
}

// Address where the zkVM writes input data before execution (from linker script)
extern "C" {
    static _in_start: u8;
}

// Read `len` bytes from `addr` of input region into the provided buffer
pub fn read_memory(buf: *mut u8, len: usize) {
    unsafe {
        let base = &raw const _in_start;
        for i in 0..len {
            *buf.add(i) = *base.add(i);
        }
    }
}

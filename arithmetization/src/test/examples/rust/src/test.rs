#![no_std]
#![no_main]

// no_mangle so the linker can find this entry point by its exact name
core::arch::global_asm!(
    ".global _start",
    "_start:",
    "li sp, 0x087fffff", // set stack pointer to a known memory region
    "call main",
);

#[no_mangle]
fn main() -> ! {
    let r = add(2, 7);
    exit(r) // no OS to return to, signal halt via ecall
}

fn add(op1: u8, op2: u8) -> u16 {
    let r = (op1 as u16).wrapping_add(op2 as u16);
    r
}

fn exit(r: u16) -> ! {
    unsafe {
        core::arch::asm!(
            "mv a0, {0}",  // exit code
            "li a7, 93",   // syscall number for exit
            "ecall",
            in(reg) r,
            options(noreturn)
        );
    }
}

// required by the compiler even if unreachable — no std means no default panic handler
#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    exit(3);
}


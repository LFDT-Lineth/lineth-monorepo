#![no_std]
#![no_main]

// To run:
// zkc-test test.rs IN_BYTES="0x05" (pass)
// zkc-test test.rs IN_BYTES="0x42" (fail)

include!("custom_std.rs");

// zero-initialized static: lives in .bss
static STATIC_ZERO_VAR: u8 = 0;

// inlined at compile time: no memory address, no ELF section
const CONST_VAR: u8 = 1;

// immutable static: lives in .rodata
static STATIC_VAR: u8 = 2;

// mutable static: lives in .data
static mut STATIC_MUT_VAR: u8 = 3;

#[no_mangle]
fn main() -> ! {
    // local variable: lives on the stack, no ELF section
    let local_var: u8 = 4;
    // variable from input: value read from the input region (IN_BYTES)
    let var_from_input = read_input();
    let r = STATIC_ZERO_VAR
        + CONST_VAR
        + STATIC_VAR
        + unsafe { STATIC_MUT_VAR }
        + local_var
        + var_from_input;
    if r != 15 {
        // 0 + 1 + 2 + 3 + 4 + 5 = 15
        exit(1); // test failed
    }
    exit(0) // test passed
}

fn read_input() -> u8 {
    static mut BUF: [u8; 1] = [0u8; 1];
    unsafe {
        read_memory(&raw mut BUF as *mut u8, 1);
        BUF[0]
    }
}

#![no_std]
#![no_main]

// Note:
// 500 = 0x01f4 (big-endian), written as 0xf401 in RAM
// IN_BYTES should be passed as big-endian
//
// To run:
// riscv-test test.rs IN_BYTES="0x01f4" (pass)
// riscv-test test.rs IN_BYTES="0x4242" (fail)
include!("custom_std.rs");

// zero-initialized static: lives in .bss
static STATIC_ZERO_VAR: u16 = 0;

// inlined at compile time: no memory address, no ELF section
const CONST_VAR: u16 = 100;

// immutable static: lives in .rodata
static STATIC_VAR: u16 = 200;

// mutable static: lives in .data
static mut STATIC_MUT_VAR: u16 = 300;

#[no_mangle]
fn main() -> ! {
    // local variable: lives on the stack, no ELF section
    let local_var: u16 = 400;
    // variable from input: value read from the input region (IN_BYTES)
    let var_from_input = read_16_bits_input();
    let r = STATIC_ZERO_VAR
        + CONST_VAR
        + STATIC_VAR
        + unsafe { STATIC_MUT_VAR }
        + local_var
        + var_from_input;
    if r != 1500 {
        // 0 + 100 + 200 + 300 + 400 + 500 = 1500
        exit(1); // test failed
    }
    exit(0) // test passed
}

fn read_16_bits_input() -> u16 {
    static mut BUF: [u8; 2] = [0u8; 2];
    unsafe {
        read_memory(&raw mut BUF as *mut u8, 2);
        u16::from_le_bytes(BUF)
    }
}

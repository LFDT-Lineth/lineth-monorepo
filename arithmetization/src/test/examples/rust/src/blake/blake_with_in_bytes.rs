#![no_std]
#![no_main]

/// Blake2b‑F compression function (used in EIP‑152 precompile, zkVMs, etc.)
///
/// Reference: RFC 7693, plus the Java gist:
/// https://gist.github.com/DavePearce/fca4c7fcfac840dc362b1c907d672093
///
/// Note: this is a freestanding implementation that reads input from memory
///
/// To run using test vector 5 (IN_BYTES is big-endian and is reversed before it reaches RAM):
/// riscv-test blake/blake_with_in_bytes.rs IN_BYTES="0x239900d4ed8623b95a92f1dba88ad31895cc3345ded552c22d79ab2a39c5877dd1a2ffdb6fbb124bb7c45a68142f214ce9f6129fb697276a0d4d1c983fa580ba010000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006362615be0cd19137e21791f83d9abfb41bd6b9b05688c2b3e6c1f510e527fade682d1a54ff53a5f1d36f13c6ef372fe94f82bbb67ae8584caa73b6a09e667f2bdc9480c000000"
use core::convert::TryInto;
use core::result::Result;
use core::result::Result::Err;
use core::result::Result::Ok;

include!("../custom_std.rs");
include!("blake_core.rs");

#[no_mangle]
fn main() -> ! {
    let (input, expected) = get_test_vector();

    let code = match blake2b_f_eip152(input) {
        Ok(result) if result == expected => 0, // success
        Ok(_) => 1,                            // wrong result
        Err(_) => 2,                           // compression failed
    };

    exit(code);
}

fn get_test_vector() -> (&'static [u8], &'static [u8]) {
    // NOTE: `main.go` hex-decodes big-endian `IN_BYTES` and emits the
    // reversed raw bytes, so RAM contains *raw* bytes, not ASCII hex.
    // The lengths below count raw bytes,
    // and we return byte slices straight from the buffer instead
    // of running them back through `hex_to_input` / `hex_to_expected`.
    const INPUT_LEN: usize = 213;
    const OUTPUT_LEN: usize = 64;
    const TOTAL_LEN: usize = INPUT_LEN + OUTPUT_LEN;
    static mut BUF: [u8; TOTAL_LEN] = [0u8; TOTAL_LEN];
    unsafe {
        read_memory(&raw mut BUF as *mut u8, TOTAL_LEN);
        let input = &BUF[..INPUT_LEN];
        let expected = &BUF[INPUT_LEN..INPUT_LEN + OUTPUT_LEN];
        (input, expected)
    }
}

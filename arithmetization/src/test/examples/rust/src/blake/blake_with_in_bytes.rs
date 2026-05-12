#![no_std]
#![no_main]

/// Blake2b‑F compression function (used in EIP‑152 precompile, zkVMs, etc.)
///
/// Reference: RFC 7693, plus the Java gist:
/// https://gist.github.com/DavePearce/fca4c7fcfac840dc362b1c907d672093
///
/// Note: this is a freestanding implementation that reads input from memory
///
/// To run using test vector 5:
/// zkc-test blake/blake_with_in_bytes.rs IN_BYTES="0x0000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001ba80a53f981c4d0d6a2797b69f12f6e94c212f14685ac4b74b12bb6fdbffa2d17d87c5392aab792dc252d5de4533cc9518d38aa8dbf1925ab92386edd4009923"
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
    // NOTE: `main.go` already hex-decodes `IN_BYTES`, so RAM contains *raw*
    // bytes — not ASCII hex. The lengths below count raw bytes,
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

/// Blake2b‑F compression function (used in EIP‑152 precompile, zkVMs, etc.)
///
/// Reference: RFC 7693, plus the Java gist:
/// https://gist.github.com/DavePearce/fca4c7fcfac840dc362b1c907d672093
///
/// To run:
/// mkdir -p bin && rustc src/blake/blake_from_main.rs -o bin/blake/blake_from_main && ./bin/blake/blake_from_main; echo $?; rm -rf bin
///
/// Note: this is a standard Rust implementation that can be run from main(). It cannot be used in the zkVM as-is, but is useful for testing the core logic and as a reference.
use std::convert::TryInto;
use std::process;

include!("blake_core.rs");
include!("blake_test_vectors.rs");

fn main() {
    let input = hex_to_input(_TEST_VECTOR_5[0]);
    let expected = hex_to_expected(_TEST_VECTOR_5[1]);

    let code = match blake2b_f_eip152(&input) {
        Ok(result) if result == expected.as_slice() => 0, // success
        Ok(_) => 1,                                       // wrong result
        Err(_) => 2,                                      // compression failed
    };

    println!("{}", if code == 0 { "PASS" } else { "FAIL" });

    process::exit(code);
}

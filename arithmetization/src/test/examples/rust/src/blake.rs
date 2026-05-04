#![no_std]
#![no_main]
use core::convert::TryInto;

// Note: to execute from main use the imports below instead,
// replace _start, exit, panic with main (using process::exit)
// and run rustc src/blake.rs -o bin/blake && ./bin/blake; echo $?

// use std::process;
// use std::convert::TryInto;

// fn main() {
//     // ...
//     process::exit(code);
// }

/// Blake2b‑F compression function (used in EIP‑152 precompile, zkVMs, etc.)
///
/// Reference: RFC 7693, plus the Java gist:
/// https://gist.github.com/DavePearce/fca4c7fcfac840dc362b1c907d672093

/// Initial state vector (IV) for Blake2b
const IV: [u64; 8] = [
    0x6a09e667f3bcc908,
    0xbb67ae8584caa73b,
    0x3c6ef372fe94f82b,
    0xa54ff53a5f1d36f1,
    0x510e527fade682d1,
    0x9b05688c2b3e6c1f,
    0x1f83d9abfb41bd6b,
    0x5be0cd19137e2179,
];

/// Sigma permutation table (10 rounds × 16 entries)
const SIGMA: [[usize; 16]; 10] = [
    [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15],
    [14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3],
    [11, 8, 12, 0, 5, 2, 15, 13, 10, 14, 3, 6, 7, 1, 9, 4],
    [7, 9, 3, 1, 13, 12, 11, 14, 2, 6, 5, 10, 4, 0, 15, 8],
    [9, 0, 5, 7, 2, 4, 10, 15, 14, 1, 11, 12, 6, 8, 3, 13],
    [2, 12, 6, 10, 0, 11, 8, 3, 4, 13, 7, 5, 15, 14, 1, 9],
    [12, 5, 1, 15, 14, 13, 4, 10, 0, 7, 6, 3, 9, 2, 8, 11],
    [13, 11, 7, 14, 12, 1, 3, 9, 5, 0, 15, 4, 8, 6, 2, 10],
    [6, 15, 14, 9, 11, 3, 0, 8, 12, 2, 13, 7, 1, 4, 10, 5],
    [10, 2, 8, 4, 7, 6, 1, 5, 15, 11, 9, 14, 3, 12, 13, 0],
];

/// Rotation constants for the G mixing function
const R1: u32 = 32;
const R2: u32 = 24;
const R3: u32 = 16;
const R4: u32 = 63;

/// The G mixing function: mixes two words from the message into the state.
#[inline(always)]
fn g(v: &mut [u64; 16], a: usize, b: usize, c: usize, d: usize, x: u64, y: u64) {
    v[a] = v[a].wrapping_add(v[b]).wrapping_add(x);
    v[d] = (v[d] ^ v[a]).rotate_right(R1);

    v[c] = v[c].wrapping_add(v[d]);
    v[b] = (v[b] ^ v[c]).rotate_right(R2);

    v[a] = v[a].wrapping_add(v[b]).wrapping_add(y);
    v[d] = (v[d] ^ v[a]).rotate_right(R3);

    v[c] = v[c].wrapping_add(v[d]);
    v[b] = (v[b] ^ v[c]).rotate_right(R4);
}

/// Blake2b‑F compression function.
///
/// # Arguments
///
/// * `rounds` - number of rounds (typically 12 for Blake2b)
/// * `h` - current state (8 × u64), modified in place
/// * `m` - message block (16 × u64)
/// * `t` - offset counters `[t0, t1]`
/// * `f` - if `true`, this is the final block
///
/// After the call, `h` contains the new state.
pub fn blake2b_f(rounds: u32, h: &mut [u64; 8], m: &[u64; 16], t: [u64; 2], f: bool) {
    // Initialize working vector v[0..15]
    let mut v: [u64; 16] = [0u64; 16];

    // v[0..8] = h[0..8]
    v[..8].copy_from_slice(h);

    // v[8..16] = IV[0..8]
    v[8..].copy_from_slice(&IV);

    // Mix in the counter and finalization flag
    v[12] ^= t[0];
    v[13] ^= t[1];
    if f {
        v[14] ^= u64::MAX; // finalization flag
    }

    // Run the rounds
    for i in 0..(rounds as usize) {
        let s = &SIGMA[i % 10];

        // Column step
        g(&mut v, 0, 4, 8, 12, m[s[0]], m[s[1]]);
        g(&mut v, 1, 5, 9, 13, m[s[2]], m[s[3]]);
        g(&mut v, 2, 6, 10, 14, m[s[4]], m[s[5]]);
        g(&mut v, 3, 7, 11, 15, m[s[6]], m[s[7]]);

        // Diagonal step
        g(&mut v, 0, 5, 10, 15, m[s[8]], m[s[9]]);
        g(&mut v, 1, 6, 11, 12, m[s[10]], m[s[11]]);
        g(&mut v, 2, 7, 8, 13, m[s[12]], m[s[13]]);
        g(&mut v, 3, 4, 9, 14, m[s[14]], m[s[15]]);
    }

    // Finalize: h[i] = h[i] ^ v[i] ^ v[i+8]
    for i in 0..8 {
        h[i] ^= v[i] ^ v[i + 8];
    }
}

/// Read a little‑endian u64 from a byte slice at offset `i * 8`.
fn read_u64_le(data: &[u8], i: usize) -> u64 {
    let off = i * 8;
    u64::from_le_bytes(data[off..off + 8].try_into().unwrap())
}

/// Write a little‑endian u64 into a byte slice at offset `i * 8`.
fn write_u64_le(data: &mut [u8], i: usize, val: u64) {
    let off = i * 8;
    data[off..off + 8].copy_from_slice(&val.to_le_bytes());
}

/// Runs the Blake2b‑F compression given the EIP‑152 input format:
///
/// ```text
/// input (213 bytes):
///   [0..4]     rounds      (big‑endian u32)
///   [4..68]    h           (8 × u64, little‑endian)
///   [68..196]  m           (16 × u64, little‑endian)
///   [196..212] t           (2 × u64, little‑endian)
///   [212]      f           (0 or 1)
/// ```
///
/// Returns the new state `h` as 64 bytes (little‑endian).
pub fn blake2b_f_eip152(input: &[u8]) -> Result<[u8; 64], &'static str> {
    if input.len() != 213 {
        return Err("input must be exactly 213 bytes");
    }

    // Parse rounds (big‑endian u32)
    let rounds = u32::from_be_bytes(input[0..4].try_into().unwrap());

    // Parse h (8 × u64, little‑endian)
    let mut h = [0u64; 8];
    for i in 0..8 {
        h[i] = read_u64_le(&input[4..], i);
    }

    // Parse m (16 × u64, little‑endian)
    let mut m = [0u64; 16];
    for i in 0..16 {
        m[i] = read_u64_le(&input[68..], i);
    }

    // Parse t (2 × u64, little‑endian)
    let t0 = read_u64_le(&input[196..], 0);
    let t1 = read_u64_le(&input[196..], 1);

    // Parse f (single byte, must be 0 or 1)
    let f_byte = input[212];
    if f_byte > 1 {
        return Err("f must be 0 or 1");
    }
    let f = f_byte == 1;

    // Run compression
    blake2b_f(rounds, &mut h, &m, [t0, t1], f);

    // Encode result as 64 bytes (little‑endian)
    let mut out = [0u8; 64];
    for i in 0..8 {
        write_u64_le(&mut out, i, h[i]);
    }

    Ok(out)
}

core::arch::global_asm!(
    ".global _start",
    "_start:",
    "li sp, 0x087fffff", // set stack pointer to a known memory region
    "call main",
);

#[no_mangle]
fn main() -> ! {
    // EIP-152 test vector as raw bytes — no heap allocation needed
    let input: [u8; 213] = [
        0x00, 0x00, 0x00, 0x0c, // rounds = 12
        // h (64 bytes)
        0x48, 0xc9, 0xbd, 0xf2, 0x67, 0xe6, 0x09, 0x6a,
        0x3b, 0xa7, 0xca, 0x84, 0x85, 0xae, 0x67, 0xbb,
        0x2b, 0xf8, 0x94, 0xfe, 0x72, 0xf3, 0x6e, 0x3c,
        0xf1, 0x36, 0x1d, 0x5f, 0x3a, 0xf5, 0x4f, 0xa5,
        0xd1, 0x82, 0xe6, 0xad, 0x7f, 0x52, 0x0e, 0x51,
        0x1f, 0x6c, 0x3e, 0x2b, 0x8c, 0x68, 0x05, 0x9b,
        0x6b, 0xbd, 0x41, 0xfb, 0xab, 0xd9, 0x83, 0x1f,
        0x79, 0x21, 0x7e, 0x13, 0x19, 0xcd, 0xe0, 0x5b,
        // m (128 bytes)
        0x61, 0x62, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        // t[0] (8 bytes)
        0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        // t[1] (8 bytes)
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        // f = true
        0x01,
    ];

    // expected output from EIP-152 test vector
    let expected: [u8; 64] = [
        0xba, 0x80, 0xa5, 0x3f, 0x98, 0x1c, 0x4d, 0x0d,
        0x6a, 0x27, 0x97, 0xb6, 0x9f, 0x12, 0xf6, 0xe9,
        0x4c, 0x21, 0x2f, 0x14, 0x68, 0x5a, 0xc4, 0xb7,
        0x4b, 0x12, 0xbb, 0x6f, 0xdb, 0xff, 0xa2, 0xd1,
        0x7d, 0x87, 0xc5, 0x39, 0x2a, 0xab, 0x79, 0x2d,
        0xc2, 0x52, 0xd5, 0xde, 0x45, 0x33, 0xcc, 0x95,
        0x18, 0xd3, 0x8a, 0xa8, 0xdb, 0xf1, 0x92, 0x5a,
        0xb9, 0x23, 0x86, 0xed, 0xd4, 0x00, 0x99, 0x23,
    ];
    

    let code = match blake2b_f_eip152(&input) {
        Ok(result) if result == expected => 0, // success
        Ok(_) => 1,                            // wrong result
        Err(_) => 2,                           // compression failed
    };

    exit(code)
}

fn exit(code: i32) -> ! {
    unsafe {
        core::arch::asm!(
            "mv a0, {0}",  // exit code
            "li a7, 93",   // syscall number for exit
            "ecall",
            in(reg) code,
            options(noreturn)
        );
    }
}

// required by the compiler even if unreachable — no std means no default panic handler
#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    exit(3);
}
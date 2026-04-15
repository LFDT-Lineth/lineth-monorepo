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

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions for parsing / encoding (matching the Java gist's test vector)
// ─────────────────────────────────────────────────────────────────────────────

/// Parse a hex string into a Vec<u8>.
pub fn hex_to_bytes(s: &str) -> Vec<u8> {
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).unwrap())
        .collect()
}

/// Encode bytes as lowercase hex string.
pub fn bytes_to_hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{:02x}", b)).collect()
}

/// Read a little‑endian u64 from a byte slice at offset `i * 8`.
pub fn read_u64_le(data: &[u8], i: usize) -> u64 {
    let off = i * 8;
    u64::from_le_bytes(data[off..off + 8].try_into().unwrap())
}

/// Write a little‑endian u64 into a byte slice at offset `i * 8`.
pub fn write_u64_le(data: &mut [u8], i: usize, val: u64) {
    let off = i * 8;
    data[off..off + 8].copy_from_slice(&val.to_le_bytes());
}

// ─────────────────────────────────────────────────────────────────────────────
// EIP‑152 style interface (input = 213 bytes, output = 64 bytes)
// ─────────────────────────────────────────────────────────────────────────────

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

// ─────────────────────────────────────────────────────────────────────────────
// Tests (including the EIP‑152 test vector from the Java gist)
// ─────────────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_eip152_vector() {
        // Test vector from EIP‑152 / the Java gist
        let input_hex = concat!(
            "0000000c",                                                                 // rounds = 12
            "48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5",         // h (part 1)
            "d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b",         // h (part 2)
            "6162630000000000000000000000000000000000000000000000000000000000",         // m (part 1)
            "0000000000000000000000000000000000000000000000000000000000000000",         // m (part 2)
            "0000000000000000000000000000000000000000000000000000000000000000",         // m (part 3)
            "0000000000000000000000000000000000000000000000000000000000000000",         // m (part 4)
            "0300000000000000",                                                         // t[0]
            "0000000000000000",                                                         // t[1]
            "01"                                                                        // f = true
        );

        let expected_hex = concat!(
            "ba80a53f981c4d0d6a2797b69f12f6e94c212f14685ac4b74b12bb6fdbffa2d1",
            "7d87c5392aab792dc252d5de4533cc9518d38aa8dbf1925ab92386edd4009923"
        );

        let input = hex_to_bytes(input_hex);
        let result = blake2b_f_eip152(&input).expect("compression failed");
        let result_hex = bytes_to_hex(&result);

        assert_eq!(result_hex, expected_hex);
    }

    #[test]
    fn test_g_function() {
        // Simple sanity check for g function
        let mut v: [u64; 16] = [
            0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
        ];
        g(&mut v, 0, 4, 8, 12, 0x100, 0x200);
        // Just check it doesn't panic and modifies v
        assert_ne!(v[0], 0);
        assert_ne!(v[4], 4);
        assert_ne!(v[8], 8);
        assert_ne!(v[12], 12);
    }
}

fn main() {
    // Quick demo: run the EIP‑152 test vector
    let input_hex = concat!(
        "0000000c",
        "48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5",
        "d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b",
        "6162630000000000000000000000000000000000000000000000000000000000",
        "0000000000000000000000000000000000000000000000000000000000000000",
        "0000000000000000000000000000000000000000000000000000000000000000",
        "0000000000000000000000000000000000000000000000000000000000000000",
        "0300000000000000",
        "0000000000000000",
        "01"
    );

    let input = hex_to_bytes(input_hex);
    println!("Input ({} bytes): {}", input.len(), input_hex);

    match blake2b_f_eip152(&input) {
        Ok(result) => {
            println!("Output: {}", bytes_to_hex(&result));
        }
        Err(e) => {
            eprintln!("Error: {}", e);
        }
    }
}

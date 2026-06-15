#![no_std]
#![no_main]

//! Keccak-256 verifier over **N_VECTORS** test vectors laid out back-to-back
//! in the VM input region.
//!
//! ## Per-vector layout (720 bytes)
//! - **680 bytes** — big-endian 5440-bit field (zeros on the left, logical message on the right).
//! - **8 bytes**   — `msg_len_bits` as **little-endian** `u64` (bit length of the logical message, ≤ 5440).
//! - **32 bytes**  — expected Keccak-256 digest (compare with this program's output).
//!
//! ## Whole input region
//! `vector_0 || vector_1 || … || vector_{N_VECTORS-1}` (no count prefix; no
//! separator). Total = `N_VECTORS * 720` bytes. 
//!
//! Exit codes:
//! - `0` : every vector's computed digest matched its expected digest.
//! - `1` : at least one vector's digest mismatched.
//! - `2` : invalid `msg_len_bits` (> 5440) for some vector.

use core::convert::TryInto;

include!("../custom_std.rs");

const RATE_BYTES: usize = 136; // 1088 bits / 8
const OUTPUT_BYTES: usize = 32; // 256 bits / 8

/// Fixed-width input for padded Keccak entrypoints: 5440 bits = 680 bytes.
pub const KECCAK256_PADDED_BITS: usize = 5440;
pub const KECCAK256_PADDED_BYTES: usize = KECCAK256_PADDED_BITS / 8;

const LENGTH_FIELD_BYTES: usize = 8;
/// Input payload of one vector: padded field + length (no expected digest).
const INPUT_LEN: usize = KECCAK256_PADDED_BYTES + LENGTH_FIELD_BYTES;
/// Expected digest length.
const OUTPUT_LEN: usize = OUTPUT_BYTES;
/// Bytes consumed by a single test vector in the input region.
const VECTOR_BYTES: usize = INPUT_LEN + OUTPUT_LEN;

/// Number of test vectors packed into the input region. Must match the
/// size of the IN_BYTES blob built by the harness; if the harness ships
/// fewer vectors, the program will read past the end of the valid data.
const fn parse_usize_env(s: &str) -> usize {
    let b = s.as_bytes();
    let mut i = 0usize;
    let mut n = 0usize;
    while i < b.len() {
        let c = b[i];
        assert!(c >= b'0' && c <= b'9');
        n = n * 10 + (c - b'0') as usize;
        i += 1;
    }
    n
}

const N_VECTORS: usize = parse_usize_env(env!("KECCAK_N_VECTORS"));

/// Total bytes the program expects to find at `_in_start`.
const TOTAL_INPUT_BYTES: usize = N_VECTORS * VECTOR_BYTES;

/// Max extracted message size after stripping left pad (= full field in the worst case).
const MAX_KECCAK_MSG_BYTES: usize = KECCAK256_PADDED_BYTES;
/// Sponge input buffer: `msg || 0x01 || … || 0x80` (Keccak multi-rate), worst-case size.
const MAX_PADDED_KECCAK_BYTES: usize =
    (MAX_KECCAK_MSG_BYTES + 1 + RATE_BYTES - 1) / RATE_BYTES * RATE_BYTES;

// Round constants
const RC: [u64; 24] = [
    0x0000000000000001,
    0x0000000000008082,
    0x800000000000808a,
    0x8000000080008000,
    0x000000000000808b,
    0x0000000080000001,
    0x8000000080008081,
    0x8000000000008009,
    0x000000000000008a,
    0x0000000000000088,
    0x0000000080008009,
    0x000000008000000a,
    0x000000008000808b,
    0x800000000000008b,
    0x8000000000008089,
    0x8000000000008003,
    0x8000000000008002,
    0x8000000000000080,
    0x000000000000800a,
    0x800000008000000a,
    0x8000000080008081,
    0x8000000000008080,
    0x0000000080000001,
    0x8000000080008008,
];

// ρ and π are applied as one fused 24-step update (matches Keccak-f[1600] reference).
const RHO: [u32; 24] = [
    1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44,
];

const PI: [usize; 24] = [
    10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1,
];

type State = [u64; 25];

/// THETA phase (diffusion)
fn theta(state: &mut State) {
    let mut c = [0u64; 5];
    let mut d = [0u64; 5];

    for x in 0..5 {
        c[x] = state[x] ^ state[x + 5] ^ state[x + 10] ^ state[x + 15] ^ state[x + 20];
    }

    for x in 0..5 {
        d[x] = c[(x + 4) % 5] ^ c[(x + 1) % 5].rotate_left(1);
    }

    for x in 0..5 {
        for y in 0..5 {
            state[x + 5 * y] ^= d[x];
        }
    }
}

/// ρ and π (fused Lane shift + rotation sequence from the Keccak reference).
fn rho_pi(state: &mut State) {
    let mut last = state[1];
    for x in 0..24 {
        let tmp = state[PI[x]];
        state[PI[x]] = last.rotate_left(RHO[x]);
        last = tmp;
    }
}

/// CHI phase (non-linear)
fn chi(state: &mut State) {
    let mut temp = *state;

    for x in 0..5 {
        for y in 0..5 {
            let idx = x + 5 * y;
            temp[idx] =
                state[idx] ^ ((!state[((x + 1) % 5) + 5 * y]) & state[((x + 2) % 5) + 5 * y]);
        }
    }

    *state = temp;
}

/// IOTA phase (round constant)
fn iota(state: &mut State, round: usize) {
    state[0] ^= RC[round];
}

/// Full Keccak-f[1600] permutation (24 rounds)
fn keccak_f(state: &mut State) {
    for round in 0..24 {
        theta(state);
        rho_pi(state);
        chi(state);
        iota(state, round);
    }
}

/// Convert bytes to u64 lane (little-endian)
fn bytes_to_lane(bytes: &[u8]) -> u64 {
    let mut lane = 0u64;
    for (i, &b) in bytes.iter().enumerate() {
        lane |= (b as u64) << (8 * i);
    }
    lane
}

/// Convert u64 lane to bytes (little-endian)
fn lane_to_bytes(lane: u64) -> [u8; 8] {
    [
        (lane & 0xff) as u8,
        ((lane >> 8) & 0xff) as u8,
        ((lane >> 16) & 0xff) as u8,
        ((lane >> 24) & 0xff) as u8,
        ((lane >> 32) & 0xff) as u8,
        ((lane >> 40) & 0xff) as u8,
        ((lane >> 48) & 0xff) as u8,
        ((lane >> 56) & 0xff) as u8,
    ]
}

/// Pad `msg` into `buf` for Keccak-256; returns total padded length written.
/// Ethereum Keccak-256: `0x01`, then `0x00` until length is a multiple of the rate,
/// then OR `0x80` into the last byte.
fn pad_message_into(msg: &[u8], buf: &mut [u8]) -> usize {
    let m = msg.len();
    let need = m + 1;
    let total = (need + RATE_BYTES - 1) / RATE_BYTES * RATE_BYTES;
    buf[..m].copy_from_slice(msg);
    buf[m] = 0x01;
    for i in m + 1..total {
        buf[i] = 0;
    }
    buf[total - 1] |= 0x80;
    total
}

/// Keccak-256 over raw message bytes.
fn keccak256_bytes(msg: &[u8]) -> [u8; OUTPUT_BYTES] {
    let mut padded_storage = [0u8; MAX_PADDED_KECCAK_BYTES];
    let plen = pad_message_into(msg, &mut padded_storage);
    let padded = &padded_storage[..plen];

    let mut state: State = [0u64; 25];

    for block_idx in 0..(padded.len() / RATE_BYTES) {
        let block = &padded[block_idx * RATE_BYTES..(block_idx + 1) * RATE_BYTES];

        for lane_idx in 0..17 {
            let lane_bytes = &block[lane_idx * 8..(lane_idx + 1) * 8];
            state[lane_idx] ^= bytes_to_lane(lane_bytes);
        }

        keccak_f(&mut state);
    }

    let mut output = [0u8; OUTPUT_BYTES];
    for lane_idx in 0..4 {
        let lane_bytes = lane_to_bytes(state[lane_idx]);
        for i in 0..8 {
            output[lane_idx * 8 + i] = lane_bytes[i];
        }
    }

    output
}

/// Interpret `padded` as one **big-endian** 5440-bit word. Writes the logical message into `out`;
/// returns its byte length.
fn extract_left_padded_message_5440(
    padded: &[u8; KECCAK256_PADDED_BYTES],
    msg_len_bits: u64,
    out: &mut [u8; KECCAK256_PADDED_BYTES],
) -> Result<usize, ()> {
    if msg_len_bits > KECCAK256_PADDED_BITS as u64 {
        return Err(());
    }
    let len = msg_len_bits as usize;
    if len == 0 {
        return Ok(0);
    }

    let skip = KECCAK256_PADDED_BITS - len;
    let mut acc: u8 = 0;
    let mut acc_bits: u32 = 0;
    let mut out_len = 0;

    for bit_i in skip..skip + len {
        let byte_idx = bit_i / 8;
        let bit_in_byte = 7 - (bit_i % 8);
        let bit = (padded[byte_idx] >> bit_in_byte) & 1;
        acc = (acc << 1) | bit;
        acc_bits += 1;
        if acc_bits == 8 {
            out[out_len] = acc;
            out_len += 1;
            acc = 0;
            acc_bits = 0;
        }
    }
    if acc_bits > 0 {
        out[out_len] = acc << (8 - acc_bits);
        out_len += 1;
    }

    Ok(out_len)
}

/// Keccak-256 over a **5440-bit left-padded** message: strip leading zero bits using
/// `msg_len_bits`, then hash.
pub fn keccak256_padded_5440(
    padded: &[u8; KECCAK256_PADDED_BYTES],
    msg_len_bits: u64,
) -> Result<[u8; OUTPUT_BYTES], ()> {
    let mut msg_buf = [0u8; KECCAK256_PADDED_BYTES];
    let msg_len = extract_left_padded_message_5440(padded, msg_len_bits, &mut msg_buf)?;
    Ok(keccak256_bytes(&msg_buf[..msg_len]))
}

#[no_mangle]
fn main() -> ! {
    let region = input_region();

    for v in 0..N_VECTORS {
        let base = v * VECTOR_BYTES;

        // Per-vector slice boundaries.
        let padded_end = base + KECCAK256_PADDED_BYTES;
        let len_end = padded_end + LENGTH_FIELD_BYTES;
        let expected_end = len_end + OUTPUT_LEN;

        // try_into on a slice produces an owned array, which is fine here:
        // 680 + 8 bytes copied per iteration is dwarfed by the 24-round
        // Keccak-f permutation that follows on each padded block.
        let padded: [u8; KECCAK256_PADDED_BYTES] =
            match region[base..padded_end].try_into() {
                Ok(a) => a,
                Err(_) => exit(2),
            };
        let len_field: [u8; LENGTH_FIELD_BYTES] =
            match region[padded_end..len_end].try_into() {
                Ok(b) => b,
                Err(_) => exit(2),
            };
        let msg_len_bits = u64::from_le_bytes(len_field);
        let expected = &region[len_end..expected_end];

        match keccak256_padded_5440(&padded, msg_len_bits) {
            Ok(result) if digest_eq(&result, expected) => {}
            Ok(_) => exit(1),
            Err(()) => exit(2),
        }
    }

    exit(0);
}

fn digest_eq(computed: &[u8; OUTPUT_BYTES], expected: &[u8]) -> bool {
    if expected.len() != OUTPUT_BYTES {
        return false;
    }
    let mut ok = true;
    for i in 0..OUTPUT_BYTES {
        ok &= computed[i] == expected[i];
    }
    ok
}

/// Returns a static slice over the whole input region. The host fills
/// `_in_start..` with `TOTAL_INPUT_BYTES` bytes before the VM is
/// kicked, so reading lazily (rather than memcpy-ing 7.2 MB into a
/// `static mut BUF`) saves the copy and the static allocation.
fn input_region() -> &'static [u8] {
    unsafe { core::slice::from_raw_parts(&raw const _in_start, TOTAL_INPUT_BYTES) }
}

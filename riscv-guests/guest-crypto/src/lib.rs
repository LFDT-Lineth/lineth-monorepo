//! C-ABI crypto backend for the l2-execution zkVM guest: secp256k1 ECDSA (recover/verify) via
//! `k256`, EIP-2537 BLS12-381 group operations via `revm-precompile`'s arkworks backend, and
//! EIP-4844 KZG point evaluation via a two-pairing check over an embedded `[s]₂`.
//!
//! Every entry point takes raw big-endian byte encodings (the unpadded EIP wire formats), returns
//! an i32 status (0 ok, nonzero err), and writes results through out-pointers. Invalid input is
//! always an error status; entry points never panic on input. Pair-list arguments arrive as a
//! pointer to packed `point ‖ scalar` / `g1 ‖ g2` records plus a record count, byte-matching the
//! caller's extern-struct layouts.
//!
//! BLS operations dispatch statically through [`DefaultCrypto`]'s trait methods, which take the
//! unpadded coordinate tuples and apply the EIP-2537 validation rules. The crate's `crypto()`
//! provider registry caches its provider through the global allocator, and the guest's bump arena
//! reclaims every allocation between entry points, so that registry is bypassed.
#![cfg_attr(target_os = "none", no_std)]

extern crate alloc;

mod arena;
mod kzg;
mod secp;
#[cfg(test)]
mod tests;

use alloc::vec::Vec;
use revm_precompile::bls12_381::{G1Point, G1PointScalar, G2Point, G2PointScalar};
use revm_precompile::{Crypto, DefaultCrypto, PrecompileHalt};

#[cfg(all(target_os = "none", not(test)))]
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}

const OK: i32 = 0;
const ERR: i32 = 1;

const FP_LEN: usize = 48;
const G1_LEN: usize = 2 * FP_LEN;
const G2_LEN: usize = 4 * FP_LEN;
const SCALAR_LEN: usize = 32;
const G1_MSM_PAIR_LEN: usize = G1_LEN + SCALAR_LEN;
const G2_MSM_PAIR_LEN: usize = G2_LEN + SCALAR_LEN;
const PAIRING_PAIR_LEN: usize = G1_LEN + G2_LEN;

fn status(ok: bool) -> i32 {
    if ok {
        OK
    } else {
        ERR
    }
}

fn g1_point(bytes: &[u8]) -> G1Point {
    (
        bytes[..FP_LEN].try_into().unwrap(),
        bytes[FP_LEN..G1_LEN].try_into().unwrap(),
    )
}

fn g2_point(bytes: &[u8]) -> G2Point {
    (
        bytes[..FP_LEN].try_into().unwrap(),
        bytes[FP_LEN..2 * FP_LEN].try_into().unwrap(),
        bytes[2 * FP_LEN..3 * FP_LEN].try_into().unwrap(),
        bytes[3 * FP_LEN..G2_LEN].try_into().unwrap(),
    )
}

fn write_result<const N: usize>(result: Result<[u8; N], PrecompileHalt>, out: &mut [u8; N]) -> i32 {
    match result {
        Ok(bytes) => {
            *out = bytes;
            OK
        }
        Err(_) => ERR,
    }
}

/// # Safety
/// `msg`, `sig`, `output` must be valid for the declared array sizes.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_secp256k1_ecrecover(
    msg: *const [u8; 32],
    sig: *const [u8; 64],
    recid: u8,
    output: *mut [u8; 64],
) -> i32 {
    arena::reset();
    status(secp::ecrecover(&*msg, &*sig, recid, &mut *output))
}

/// # Safety
/// `msg`, `sig`, `pubkey`, `verified` must be valid for the declared sizes.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_secp256k1_verify(
    msg: *const [u8; 32],
    sig: *const [u8; 64],
    pubkey: *const [u8; 64],
    verified: *mut bool,
) -> i32 {
    arena::reset();
    *verified = secp::verify(&*msg, &*sig, &*pubkey);
    OK
}

/// # Safety
/// `p1`, `p2`, `result` must be valid 96-byte buffers.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_g1_add(
    p1: *const [u8; 96],
    p2: *const [u8; 96],
    result: *mut [u8; 96],
) -> i32 {
    arena::reset();
    write_result(
        DefaultCrypto.bls12_381_g1_add(g1_point(&*p1), g1_point(&*p2)),
        &mut *result,
    )
}

/// # Safety
/// `pairs` must be valid for `num_pairs` packed 128-byte `g1 ‖ scalar` records.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_g1_msm(
    pairs: *const u8,
    num_pairs: usize,
    result: *mut [u8; 96],
) -> i32 {
    arena::reset();
    let Some(bytes) = pair_bytes(pairs, num_pairs, G1_MSM_PAIR_LEN) else {
        return ERR;
    };
    let mut records = bytes.chunks_exact(G1_MSM_PAIR_LEN).map(|pair| {
        Ok::<G1PointScalar, PrecompileHalt>((
            g1_point(pair),
            pair[G1_LEN..].try_into().unwrap(),
        ))
    });
    write_result(DefaultCrypto.bls12_381_g1_msm(&mut records), &mut *result)
}

/// # Safety
/// `p1`, `p2`, `result` must be valid 192-byte buffers.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_g2_add(
    p1: *const [u8; 192],
    p2: *const [u8; 192],
    result: *mut [u8; 192],
) -> i32 {
    arena::reset();
    write_result(
        DefaultCrypto.bls12_381_g2_add(g2_point(&*p1), g2_point(&*p2)),
        &mut *result,
    )
}

/// # Safety
/// `pairs` must be valid for `num_pairs` packed 224-byte `g2 ‖ scalar` records.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_g2_msm(
    pairs: *const u8,
    num_pairs: usize,
    result: *mut [u8; 192],
) -> i32 {
    arena::reset();
    let Some(bytes) = pair_bytes(pairs, num_pairs, G2_MSM_PAIR_LEN) else {
        return ERR;
    };
    let mut records = bytes.chunks_exact(G2_MSM_PAIR_LEN).map(|pair| {
        Ok::<G2PointScalar, PrecompileHalt>((
            g2_point(pair),
            pair[G2_LEN..].try_into().unwrap(),
        ))
    });
    write_result(DefaultCrypto.bls12_381_g2_msm(&mut records), &mut *result)
}

/// # Safety
/// `pairs` must be valid for `num_pairs` packed 288-byte `g1 ‖ g2` records.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_pairing(
    pairs: *const u8,
    num_pairs: usize,
    verified: *mut bool,
) -> i32 {
    arena::reset();
    if num_pairs == 0 {
        *verified = true;
        return OK;
    }
    let Some(bytes) = pair_bytes(pairs, num_pairs, PAIRING_PAIR_LEN) else {
        return ERR;
    };
    let mut records: Vec<(G1Point, G2Point)> = Vec::with_capacity(num_pairs);
    for pair in bytes.chunks_exact(PAIRING_PAIR_LEN) {
        records.push((g1_point(pair), g2_point(&pair[G1_LEN..])));
    }
    match DefaultCrypto.bls12_381_pairing_check(&records) {
        Ok(v) => {
            *verified = v;
            OK
        }
        Err(_) => ERR,
    }
}

/// # Safety
/// `field_element` must be a valid 48-byte buffer; `result` a valid 96-byte buffer.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_map_fp_to_g1(
    field_element: *const [u8; 48],
    result: *mut [u8; 96],
) -> i32 {
    arena::reset();
    write_result(
        DefaultCrypto.bls12_381_fp_to_g1(&*field_element),
        &mut *result,
    )
}

/// # Safety
/// `field_element` must be a valid 96-byte buffer; `result` a valid 192-byte buffer.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_bls12_map_fp2_to_g2(
    field_element: *const [u8; 96],
    result: *mut [u8; 192],
) -> i32 {
    arena::reset();
    let fe = &*field_element;
    let fp2 = (
        fe[..FP_LEN].try_into().unwrap(),
        fe[FP_LEN..].try_into().unwrap(),
    );
    write_result(DefaultCrypto.bls12_381_fp2_to_g2(fp2), &mut *result)
}

/// # Safety
/// All pointers must be valid for the declared array sizes.
#[no_mangle]
pub unsafe extern "C" fn guest_crypto_kzg_point_eval(
    commitment: *const [u8; 48],
    z: *const [u8; 32],
    y: *const [u8; 32],
    proof: *const [u8; 48],
    verified: *mut bool,
) -> i32 {
    arena::reset();
    status(kzg::point_eval(
        &*commitment,
        &*z,
        &*y,
        &*proof,
        &mut *verified,
    ))
}

unsafe fn pair_bytes<'a>(pairs: *const u8, num_pairs: usize, pair_len: usize) -> Option<&'a [u8]> {
    if num_pairs == 0 || pairs.is_null() {
        return None;
    }
    let len = num_pairs.checked_mul(pair_len)?;
    Some(core::slice::from_raw_parts(pairs, len))
}

const _: () = {
    // The pair strides must match the extern-struct layouts on the Zig side.
    assert!(G1_MSM_PAIR_LEN == 128);
    assert!(G2_MSM_PAIR_LEN == 224);
    assert!(PAIRING_PAIR_LEN == 288);
};

//! EIP-4844 KZG point evaluation without the full trusted setup: `verify_kzg_proof` touches only
//! `[s]₂` (the second G2 monomial of the official Ethereum KZG ceremony), embedded below, so the
//! check reduces to the two-pairing equation
//!
//!     e(commitment − [y]₁, −G₂) · e(proof, [s]₂ − [z]₂) == 1
//!
//! Commitments and proofs are ZCash-compressed G1; z and y must be canonical scalars (< r).

use ark_bls12_381::{Bls12_381, Fq, Fq2, Fr, G1Affine, G1Projective, G2Affine, G2Projective};
use ark_ec::pairing::{MillerLoopOutput, Pairing};
use ark_ec::{AffineRepr, CurveGroup, PrimeGroup};
use ark_ff::{BigInt, One, PrimeField};
use ark_serialize::CanonicalDeserialize;

/// `[s]₂` as raw uncompressed ZCash coordinate order (x_c1 ‖ x_c0 ‖ y_c1 ‖ y_c0, 48 bytes each),
/// pinned against the ceremony's `trusted_setup.txt` by this crate's tests and by the host test
/// suite that parses the setup file itself.
const S_G2_RAW: [u8; 192] = hex_192(
    b"15bfd7dd8cdeb128843bc287230af38926187075cbfbefa81009a2ce615ac53d2914e5870cb452d2afaaab24f3499f72\
      185cbfee53492714734429b7b38608e23926c911cceceac9a36851477ba4c60b087041de621000edc98edada20c1def2\
      1666c54b0a32529503432fcae0181b4bef79de09fc63671fda5ed1ba9bfa07899495346f3d7ac9cd23048ef30d0a154f\
      014353bdb96b626dd7d5ee8599d1fca2131569490e28de18e82451a496a9c9794ce26d105941f383ee689bfbbb832a99",
);

pub fn s_g2() -> G2Affine {
    let fq = |range: core::ops::Range<usize>| {
        fq_from_be(S_G2_RAW[range].try_into().unwrap()).unwrap()
    };
    let x = Fq2::new(fq(48..96), fq(0..48));
    let y = Fq2::new(fq(144..192), fq(96..144));
    G2Affine::new_unchecked(x, y)
}

fn fq_from_be(bytes: &[u8; 48]) -> Option<Fq> {
    let mut limbs = [0u64; 6];
    for (i, limb) in limbs.iter_mut().enumerate() {
        let hi = 48 - 8 * i;
        *limb = u64::from_be_bytes(bytes[hi - 8..hi].try_into().unwrap());
    }
    Fq::from_bigint(BigInt::new(limbs))
}

/// Decodes a ZCash-format compressed G1 point (48 bytes: 3 flag bits over big-endian x), the
/// EIP-4844 encoding for KZG commitments and proofs. ark-bls12-381's own compressed format IS the
/// ZCash format, and `Validate::Yes` enforces canonical x, on-curve, and subgroup membership.
fn g1_from_zcash_compressed(bytes: &[u8; 48]) -> Option<G1Affine> {
    G1Affine::deserialize_compressed(bytes.as_slice()).ok()
}

fn final_exp_is_one(f: <Bls12_381 as Pairing>::TargetField, verified: &mut bool) -> bool {
    match Bls12_381::final_exponentiation(MillerLoopOutput(f)) {
        Some(out) => {
            *verified = out.0.is_one();
            true
        }
        None => false,
    }
}

fn fr_from_be(bytes: &[u8; 32]) -> Option<Fr> {
    let mut limbs = [0u64; 4];
    for (i, limb) in limbs.iter_mut().enumerate() {
        let hi = 32 - 8 * i;
        *limb = u64::from_be_bytes(bytes[hi - 8..hi].try_into().unwrap());
    }
    Fr::from_bigint(BigInt::new(limbs))
}

pub fn point_eval(
    commitment: &[u8; 48],
    z: &[u8; 32],
    y: &[u8; 32],
    proof: &[u8; 48],
    verified: &mut bool,
) -> bool {
    let Some(c) = g1_from_zcash_compressed(commitment) else {
        return false;
    };
    let Some(pi) = g1_from_zcash_compressed(proof) else {
        return false;
    };
    let (Some(z), Some(y)) = (fr_from_be(z), fr_from_be(y)) else {
        return false;
    };

    let p_minus_y = (c.into_group() - G1Projective::generator() * y).into_affine();
    let x_minus_z = (s_g2().into_group() - G2Projective::generator() * z).into_affine();

    let mut f = <Bls12_381 as Pairing>::TargetField::one();
    if !p_minus_y.is_zero() {
        f *= crate::arena::scope(|| Bls12_381::miller_loop(p_minus_y, -G2Affine::generator()).0);
    }
    if !pi.is_zero() && !x_minus_z.is_zero() {
        f *= crate::arena::scope(|| Bls12_381::miller_loop(pi, x_minus_z).0);
    }
    final_exp_is_one(f, verified)
}

const fn hex_192(s: &[u8]) -> [u8; 192] {
    assert!(s.len() == 384);
    let mut out = [0u8; 192];
    let mut i = 0;
    while i < 192 {
        out[i] = hex_nibble(s[2 * i]) << 4 | hex_nibble(s[2 * i + 1]);
        i += 1;
    }
    out
}

const fn hex_nibble(c: u8) -> u8 {
    match c {
        b'0'..=b'9' => c - b'0',
        b'a'..=b'f' => c - b'a' + 10,
        _ => panic!("invalid hex digit"),
    }
}

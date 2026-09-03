//! Zig bindings to Constantine's `ctt_eth_evm_*` EVM precompile functions, exposing the
//! `guest_crypto` module interface consumed by zkvm_provide.zig. The `guest_crypto` dependency
//! in build.zig.zon resolves to the Constantine package.
//!
//! ABI note: the guest's zkvm_* seam uses the RAW unpadded EIP encodings (Fp = 48 bytes,
//! G1 = 96, G2 = 192). Constantine's ctt_eth_evm_* take the PADDED EIP-2537 byte layout
//! (Fp padded to 64, G1 = 128, G2 = 256) and write padded outputs, so each BLS12-381 wrapper
//! zero-pads every 48-byte limb to 64 on the way in and strips the padding on the way out; the
//! pairing/MSM inputs are repacked record-by-record. BN254's seam encoding already matches the
//! EIP-196/197 layout, so those wrappers pass buffers through unchanged.
//!
//! Deliberate stubs (not full parity):
//!   • ecrecover / secp256k1Verify — Constantine's ecrecover takes the EIP 128-byte input and
//!     returns the keccak'd ADDRESS, not the raw 64-byte pubkey this seam uses; stubbed ERR.
//!   • kzgPointEvalVerify — ctt_eth_evm_kzg_point_evaluation needs a ctt_eth_kzg_context built
//!     from the full 4096-point trusted setup (file-loaded); stubbed false.

const std = @import("std");

extern fn ctt_eth_evm_bls12381_g1add(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_g2add(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_g1msm(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_g2msm(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_pairingcheck(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_map_fp_to_g1(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bls12381_map_fp2_to_g2(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bn254_g1add(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bn254_g1mul(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_evm_bn254_ecpairingcheck(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
// Defined in the constantine package's ctt_stubs_rv64.o (ABI mismatch, see header).
extern fn guest_crypto_secp256k1_ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) c_int;
extern fn guest_crypto_secp256k1_verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) c_int;

const OK: c_int = 0; // cttEVM_Success

pub fn ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) bool {
    return guest_crypto_secp256k1_ecrecover(msg, sig, recid, output) == OK;
}

pub fn secp256k1Verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) void {
    _ = guest_crypto_secp256k1_verify(msg, sig, pubkey, verified);
}

// ── raw↔padded helpers ────────────────────────────────────────────────────────────────────────
// Copy `count` 48-byte limbs from `src` into 64-byte slots in `dst` (16-byte zero left-pad).
fn padLimbs(dst: [*]u8, src: [*]const u8, count: usize) void {
    var i: usize = 0;
    while (i < count) : (i += 1) {
        const d = dst + i * 64;
        @memset(d[0..16], 0);
        @memcpy(d[16..64], (src + i * 48)[0..48]);
    }
}

// Strip 16-byte left-padding: `count` 64-byte limbs from `src` into 48-byte limbs in `dst`.
fn unpadLimbs(dst: [*]u8, src: [*]const u8, count: usize) void {
    var i: usize = 0;
    while (i < count) : (i += 1) {
        @memcpy((dst + i * 48)[0..48], (src + i * 64)[16..64]);
    }
}

pub fn g1Add(p1: *const [96]u8, p2: *const [96]u8, result: *[96]u8) bool {
    var in: [256]u8 = undefined; // two padded G1 (128 each)
    padLimbs(&in, p1, 2);
    padLimbs(in[128..].ptr, p2, 2);
    var out: [128]u8 = undefined;
    if (ctt_eth_evm_bls12381_g1add(&out, 128, &in, 256) != OK) return false;
    unpadLimbs(result, &out, 2);
    return true;
}

pub fn g2Add(p1: *const [192]u8, p2: *const [192]u8, result: *[192]u8) bool {
    var in: [512]u8 = undefined; // two padded G2 (256 each)
    padLimbs(&in, p1, 4);
    padLimbs(in[256..].ptr, p2, 4);
    var out: [256]u8 = undefined;
    if (ctt_eth_evm_bls12381_g2add(&out, 256, &in, 512) != OK) return false;
    unpadLimbs(result, &out, 4);
    return true;
}

pub fn g1Msm(pairs: anytype, result: *[96]u8) bool {
    // raw record = 96 (point) + 32 (scalar) = 128; ctt record = 128 (padded point) + 32 = 160
    const n = pairs.len;
    const raw = pairBytes(pairs, 96 + 32);
    var in: [4096 * 160]u8 = undefined; // bounded scratch; MSM degree is gas-limited far below this
    if (n > 4096) return false;
    var i: usize = 0;
    while (i < n) : (i += 1) {
        padLimbs(in[i * 160 ..].ptr, raw + i * 128, 2); // point: 2 limbs
        @memcpy(in[i * 160 + 128 ..][0..32], (raw + i * 128 + 96)[0..32]); // scalar
    }
    var out: [128]u8 = undefined;
    if (ctt_eth_evm_bls12381_g1msm(&out, 128, &in, n * 160) != OK) return false;
    unpadLimbs(result, &out, 2);
    return true;
}

pub fn g2Msm(pairs: anytype, result: *[192]u8) bool {
    // raw record = 192 + 32 = 224; ctt record = 256 + 32 = 288
    const n = pairs.len;
    const raw = pairBytes(pairs, 192 + 32);
    var in: [2048 * 288]u8 = undefined;
    if (n > 2048) return false;
    var i: usize = 0;
    while (i < n) : (i += 1) {
        padLimbs(in[i * 288 ..].ptr, raw + i * 224, 4); // point: 4 limbs
        @memcpy(in[i * 288 + 256 ..][0..32], (raw + i * 224 + 192)[0..32]); // scalar
    }
    var out: [256]u8 = undefined;
    if (ctt_eth_evm_bls12381_g2msm(&out, 256, &in, n * 288) != OK) return false;
    unpadLimbs(result, &out, 4);
    return true;
}

pub fn pairingCheck(pairs: anytype, verified: *bool) bool {
    // raw record = 96 (g1) + 192 (g2) = 288; ctt record = 128 + 256 = 384
    const n = pairs.len;
    const raw = pairBytes(pairs, 96 + 192);
    var in: [1024 * 384]u8 = undefined;
    if (n > 1024) return false;
    var i: usize = 0;
    while (i < n) : (i += 1) {
        padLimbs(in[i * 384 ..].ptr, raw + i * 288, 2); // g1: 2 limbs
        padLimbs(in[i * 384 + 128 ..].ptr, raw + i * 288 + 96, 4); // g2: 4 limbs
    }
    var out: [32]u8 = undefined;
    if (ctt_eth_evm_bls12381_pairingcheck(&out, 32, &in, n * 384) != OK) return false;
    verified.* = (out[31] == 1); // 32-byte big-endian 0/1
    return true;
}

pub fn mapFpToG1(field_element: *const [48]u8, result: *[96]u8) bool {
    var in: [64]u8 = undefined;
    padLimbs(&in, field_element, 1);
    var out: [128]u8 = undefined;
    if (ctt_eth_evm_bls12381_map_fp_to_g1(&out, 128, &in, 64) != OK) return false;
    unpadLimbs(result, &out, 2);
    return true;
}

pub fn mapFp2ToG2(field_element: *const [96]u8, result: *[192]u8) bool {
    var in: [128]u8 = undefined;
    padLimbs(&in, field_element, 2);
    var out: [256]u8 = undefined;
    if (ctt_eth_evm_bls12381_map_fp2_to_g2(&out, 256, &in, 128) != OK) return false;
    unpadLimbs(result, &out, 4);
    return true;
}

// ── BN254 (alt_bn128): unpadded EIP-196/197 raw layout, 32-byte big-endian coords ───────────────
// Unlike BLS12-381, the seam's encodings ARE the EIP encodings (G1 = 64 bytes, pairing record =
// 192 bytes), so these wrappers pass the buffers through with no repacking.
pub fn bn254G1Add(p1: *const [64]u8, p2: *const [64]u8, result: *[64]u8) bool {
    var in: [128]u8 = undefined;
    @memcpy(in[0..64], p1);
    @memcpy(in[64..128], p2);
    return ctt_eth_evm_bn254_g1add(result, 64, &in, 128) == OK;
}

pub fn bn254G1Mul(point: *const [64]u8, scalar: *const [32]u8, result: *[64]u8) bool {
    var in: [96]u8 = undefined;
    @memcpy(in[0..64], point);
    @memcpy(in[64..96], scalar);
    return ctt_eth_evm_bn254_g1mul(result, 64, &in, 96) == OK;
}

pub fn bn254PairingCheck(pairs: anytype, verified: *bool) bool {
    const n = pairs.len;
    const raw = pairBytes(pairs, 64 + 128); // g1 = 64, g2 = 128
    var out: [32]u8 = undefined;
    if (ctt_eth_evm_bn254_ecpairingcheck(&out, 32, raw, n * 192) != OK) return false;
    verified.* = (out[31] == 1); // 32-byte big-endian 0/1
    return true;
}

pub fn kzgPointEvalVerify(
    commitment: *const [48]u8,
    z: *const [32]u8,
    y: *const [32]u8,
    proof: *const [48]u8,
) bool {
    _ = commitment;
    _ = z;
    _ = y;
    _ = proof;
    return false; // stubbed: ctt KZG needs a full trusted-setup context (file-loaded)
}

fn pairBytes(pairs: anytype, comptime stride: usize) [*]const u8 {
    const info = @typeInfo(@TypeOf(pairs)).pointer;
    const Pair = if (info.size == .slice) info.child else @typeInfo(info.child).array.child;
    comptime std.debug.assert(@sizeOf(Pair) == stride);
    return if (info.size == .slice) @ptrCast(pairs.ptr) else @ptrCast(pairs);
}

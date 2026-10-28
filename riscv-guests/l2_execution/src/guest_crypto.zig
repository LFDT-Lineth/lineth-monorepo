//! Zig bindings to Constantine's `ctt_eth_evm_*` precompile functions.
//!
//! BLS12-381 wrappers repack raw 48-byte limbs into Constantine's padded EIP-2537 layout.
//! BN254 uses the EIP-196/197 layout directly.
//!
//! secp256k1 ecrecover and verify use Constantine's raw-primitive exports.
//!
//! KZG point evaluation uses the trusted setup baked into the ELF.

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
extern fn ctt_eth_zkvm_secp256k1_verify(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;
extern fn ctt_eth_zkvm_secp256k1_ecrecover(r: [*]u8, r_len: usize, inputs: [*]const u8, inputs_len: usize) c_int;

const OK: c_int = 0; // cttEVM_Success

/// Recovers the raw 64-byte x‖y public key from a pre-hash and compact big-endian r‖s signature.
/// `recid` selects the y parity as 0/1 or Ethereum's 27/28.
pub fn ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) bool {
    const recid_norm: u8 = switch (recid) {
        0, 1 => recid,
        27, 28 => recid - 27,
        else => return false,
    };
    var in: [97]u8 = undefined; // digest ‖ recid ‖ r ‖ s
    @memcpy(in[0..32], msg);
    in[32] = recid_norm;
    @memcpy(in[33..97], sig);
    return ctt_eth_zkvm_secp256k1_ecrecover(output, 64, &in, 97) == OK;
}

/// Verifies a compact big-endian r‖s signature against a pre-hash and raw 64-byte x‖y key.
pub fn secp256k1Verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) void {
    var in: [160]u8 = undefined; // digest ‖ x ‖ y ‖ r ‖ s, big-endian
    @memcpy(in[0..32], msg);
    @memcpy(in[32..96], pubkey);
    @memcpy(in[96..160], sig);
    var out: [1]u8 = .{0};
    verified.* = ctt_eth_zkvm_secp256k1_verify(&out, 1, &in, 160) == OK and out[0] == 1;
}

// Copy 48-byte limbs into 64-byte slots with 16 bytes of left padding.
fn padLimbs(dst: [*]u8, src: [*]const u8, count: usize) void {
    var i: usize = 0;
    while (i < count) : (i += 1) {
        const d = dst + i * 64;
        @memset(d[0..16], 0);
        @memcpy(d[16..64], (src + i * 48)[0..48]);
    }
}

// Strip 16-byte left padding from 64-byte limbs.
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
    // Raw record = 96-byte point + 32-byte scalar; padded record adds 32 bytes.
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
    // Raw record = 192-byte point + 32-byte scalar; padded record adds 64 bytes.
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
    // Raw record = 96-byte G1 + 192-byte G2; padded record adds 96 bytes.
    const n = pairs.len;
    // An empty pairing product verifies as the multiplicative identity.
    if (n == 0) {
        verified.* = true;
        return true;
    }
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

// BN254 uses the EIP-196/197 raw layout directly.
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
    // Empty input verifies as the multiplicative identity.
    if (n == 0) {
        verified.* = true;
        return true;
    }
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
    const ctx = kzgContext() orelse return false;
    var in: [192]u8 = undefined;
    // versioned_hash = 0x01 ‖ sha256(commitment)[1:]
    std.crypto.hash.sha2.Sha256.hash(commitment, in[0..32], .{});
    in[0] = 0x01;
    @memcpy(in[32..64], z);
    @memcpy(in[64..96], y);
    @memcpy(in[96..144], commitment);
    @memcpy(in[144..192], proof);
    var out: [64]u8 = undefined;
    return ctt_eth_evm_kzg_point_evaluation(ctx, &out, 64, &in, 192) == OK;
}

const CttKzgContext = opaque {};

extern fn ctt_eth_kzg_context_new_embedded(ctx: *?*CttKzgContext) c_int;
extern fn ctt_eth_evm_kzg_point_evaluation(
    ctx: ?*const CttKzgContext,
    r: [*]u8,
    r_len: usize,
    inputs: [*]const u8,
    inputs_len: usize,
) c_int;

var kzg_ctx: ?*CttKzgContext = null;
var kzg_ctx_failed = false;

/// Lazily builds the KZG trusted-setup context once.
fn kzgContext() ?*CttKzgContext {
    if (kzg_ctx) |c| return c;
    if (kzg_ctx_failed) return null;
    if (ctt_eth_kzg_context_new_embedded(&kzg_ctx) != 0) { // tsSuccess == 0
        kzg_ctx_failed = true;
        return null;
    }
    return kzg_ctx;
}

fn pairBytes(pairs: anytype, comptime stride: usize) [*]const u8 {
    const info = @typeInfo(@TypeOf(pairs)).pointer;
    const Pair = if (info.size == .slice) info.child else @typeInfo(info.child).array.child;
    comptime std.debug.assert(@sizeOf(Pair) == stride);
    return if (info.size == .slice) @ptrCast(pairs.ptr) else @ptrCast(pairs);
}

//! `zkvm_*` precompile providers for the ZkC guest.
//!
//! Zesu's freestanding build references every precompile as an `extern fn zkvm_*` symbol. The guest
//! ships as a *statically-linked* ELF (the zkvm-standards artifact), so there is no later link to
//! resolve anything: every one of those externs must be DEFINED in the binary. This module defines
//! all of them, from these sources:
//!
//!   • Lineth accelerator wrappers (`lineth_zkvm_accel`) — for the precompiles the prover accelerates
//!     (keccak today). We re-export each wrapper under the C name zesu references; HOW a wrapper
//!     accelerates is the wrapper module's own concern. The *set of wrappers that exist* is what is
//!     accelerated, and grows as the prover implements more.
//!   • Zig std.crypto — keccak256 (unless -Dkeccak-accel selects the wrapper), SHA-256, and
//!     secp256r1 (P-256) verification.
//!   • The guest_crypto Constantine backend — secp256k1 ecrecover/verify, the EIP-2537
//!     BLS12-381 operations, bn254 (EIP-196/197), and EIP-4844 KZG point evaluation.
//!   • zesu's own native crypto backend (`zesu_crypto_backend`) — modexp/RIPEMD-160/BLAKE2f. These
//!     have no C-library dependency, so — unlike the rest of zesu's native backend — they
//!     cross-compile straight to riscv64.
//!   • bn254 — Constantine's ctt_eth_evm_bn254_* (EIP-196/197).
//!
//! Only the freestanding RISC-V guest references these (pulled in by evm_execution_guest.zig for
//! `builtin.cpu.arch == .riscv64`); the native host build uses Zesu's C-backed crypto instead.

const std = @import("std");
const lineth_accel = @import("lineth_zkvm_accel"); // Lineth accelerator wrappers (source paths wired in build.zig)
const zesu_crypto_backend = @import("zesu_crypto_backend"); // zesu's own native crypto backend (modexp, RIPEMD-160, BLAKE2f — see src/zesu_crypto_backend.zig)
const guest_crypto = @import("guest_crypto"); // Constantine backend bindings (secp256k1, BLS12-381, bn254, KZG — see src/guest_crypto.zig)
const build_options = @import("build_options"); // keccak_accel: standard zig keccak vs Lineth wrapper

// The manifest: every `zkvm_*` symbol zesu references, and where each comes from — keccak is either
// the Lineth wrapper (prover-accelerated) or the standard zig keccak, selected at build time by
// -Dkeccak-accel; modexp/ripemd160/blake2f come from zesu_crypto_backend; secp256k1 and the
// BLS12-381/KZG and bn254 come from guest_crypto.
comptime {
    if (build_options.keccak_accel) {
        @export(&lineth_accel.zkvm_keccak256, .{ .name = "zkvm_keccak256" });
    } else {
        @export(&keccak256, .{ .name = "zkvm_keccak256" });
    }
    @export(&sha256, .{ .name = "zkvm_sha256" });
    @export(&secp256k1_verify, .{ .name = "zkvm_secp256k1_verify" });
    @export(&secp256k1_ecrecover, .{ .name = "zkvm_secp256k1_ecrecover" });
    @export(&ripemd160, .{ .name = "zkvm_ripemd160" });
    @export(&modexp, .{ .name = "zkvm_modexp" });
    @export(&bn254_g1_add, .{ .name = "zkvm_bn254_g1_add" });
    @export(&bn254_g1_mul, .{ .name = "zkvm_bn254_g1_mul" });
    @export(&bn254_pairing, .{ .name = "zkvm_bn254_pairing" });
    @export(&blake2f, .{ .name = "zkvm_blake2f" });
    @export(&kzg_point_eval, .{ .name = "zkvm_kzg_point_eval" });
    @export(&bls12_g1_add, .{ .name = "zkvm_bls12_g1_add" });
    @export(&bls12_g1_msm, .{ .name = "zkvm_bls12_g1_msm" });
    @export(&bls12_g2_add, .{ .name = "zkvm_bls12_g2_add" });
    @export(&bls12_g2_msm, .{ .name = "zkvm_bls12_g2_msm" });
    @export(&bls12_pairing, .{ .name = "zkvm_bls12_pairing" });
    @export(&bls12_map_fp_to_g1, .{ .name = "zkvm_bls12_map_fp_to_g1" });
    @export(&bls12_map_fp2_to_g2, .{ .name = "zkvm_bls12_map_fp2_to_g2" });
    @export(&secp256r1_verify, .{ .name = "zkvm_secp256r1_verify" });
    @export(&log, .{ .name = "zkvm_log" });
    // write_output (zkvm-standards io-interface): the Lineth custom-opcode accelerator, under the
    // standard extern name. Its signature already IS the C ABI zesu declares, so it exports
    // directly rather than through a shim like the precompiles above.
    @export(&lineth_accel.write_output, .{ .name = "write_output" });
}

const OK: i32 = 0;
const ERR: i32 = 1;

// Pairing/MSM pair layouts — must byte-match the C-ABI struct layout zesu passes to these zkvm_*
// symbols; forwarded straight to guest_crypto's `anytype` parameters.
const Bn254PairingPair = extern struct { g1: [64]u8, g2: [128]u8 };
const Bls12G1MsmPair = extern struct { point: [96]u8, scalar: [32]u8 };
const Bls12G2MsmPair = extern struct { point: [192]u8, scalar: [32]u8 };
const Bls12PairingPair = extern struct { g1: [96]u8, g2: [192]u8 };

// ── C-ABI shims: extern zkvm_* (ptr+len) → the providers' slice/array APIs ───────────────────────
// One per precompile that has no Lineth wrapper yet; all exported in the comptime block above.

// Standard zig keccak (std.crypto); used unless -Dkeccak-accel selects the wrapper.
fn keccak256(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    std.crypto.hash.sha3.Keccak256.hash(data[0..len], output, .{});
    return OK;
}
fn sha256(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    std.crypto.hash.sha2.Sha256.hash(data[0..len], output, .{});
    return OK;
}
fn ripemd160(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    const hash = zesu_crypto_backend.ripemd160(data[0..len]);
    output.* = [_]u8{0} ** 32;
    @memcpy(output[12..32], &hash);
    return OK;
}
fn secp256k1_ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) callconv(.c) i32 {
    return if (guest_crypto.ecrecover(msg, sig, recid, output)) OK else ERR;
}
fn secp256k1_verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) callconv(.c) i32 {
    guest_crypto.secp256k1Verify(msg, sig, pubkey, verified);
    return OK;
}
fn secp256r1_verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) callconv(.c) i32 {
    verified.* = verifyP256(msg, sig, pubkey) catch false;
    return OK;
}

/// P-256 (secp256r1) ECDSA verify over a pre-hashed message, via std.crypto.ecc.P256.
/// sig is the compact r‖s encoding (each 32 bytes, big-endian); pubkey is the uncompressed
/// point without the 0x04 prefix (x‖y, 64 bytes). std's generic Ecdsa types hash the message
/// themselves, so we drive the lower-level verifyPrehashed instead (the EVM precompile is
/// always over a 32-byte pre-hash).
fn verifyP256(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8) !bool {
    const EcdsaP256 = std.crypto.sign.ecdsa.Ecdsa(std.crypto.ecc.P256, std.crypto.hash.sha2.Sha256);
    var sec1: [65]u8 = undefined;
    sec1[0] = 0x04;
    @memcpy(sec1[1..65], pubkey);
    const pk = try EcdsaP256.PublicKey.fromSec1(&sec1);
    const signature = EcdsaP256.Signature.fromBytes(sig.*);
    signature.verifyPrehashed(msg.*, pk) catch return false;
    return true;
}
fn modexp(base: [*]const u8, base_len: usize, exp: [*]const u8, exp_len: usize, modulus: [*]const u8, mod_len: usize, output: [*]u8) callconv(.c) i32 {
    return if (zesu_crypto_backend.modexp(base[0..base_len], exp[0..exp_len], modulus[0..mod_len], output[0..mod_len])) OK else ERR;
}
// bn254 via Constantine's ctt_eth_evm_bn254_* (EIP-196/197 raw layout, no repacking).
fn bn254_g1_add(p1: *const [64]u8, p2: *const [64]u8, result: *[64]u8) callconv(.c) i32 {
    return if (guest_crypto.bn254G1Add(p1, p2, result)) OK else ERR;
}
fn bn254_g1_mul(point: *const [64]u8, scalar: *const [32]u8, result: *[64]u8) callconv(.c) i32 {
    return if (guest_crypto.bn254G1Mul(point, scalar, result)) OK else ERR;
}
fn bn254_pairing(pairs: [*]const Bn254PairingPair, num_pairs: usize, verified: *bool) callconv(.c) i32 {
    return if (guest_crypto.bn254PairingCheck(pairs[0..num_pairs], verified)) OK else ERR;
}
fn blake2f(rounds: u32, h: *[64]u8, m: *const [128]u8, t: *const [16]u8, f: u8) callconv(.c) i32 {
    return if (zesu_crypto_backend.blake2f(rounds, h, m, t, f)) OK else ERR;
}
fn kzg_point_eval(commitment: *const [48]u8, z: *const [32]u8, y: *const [32]u8, proof: *const [48]u8, verified: *bool) callconv(.c) i32 {
    // Real verification: the two-pairing check against the embedded [s]₂.
    verified.* = guest_crypto.kzgPointEvalVerify(commitment, z, y, proof);
    return OK;
}
fn bls12_g1_add(p1: *const [96]u8, p2: *const [96]u8, result: *[96]u8) callconv(.c) i32 {
    return if (guest_crypto.g1Add(p1, p2, result)) OK else ERR;
}
fn bls12_g1_msm(pairs: [*]const Bls12G1MsmPair, num_pairs: usize, result: *[96]u8) callconv(.c) i32 {
    return if (guest_crypto.g1Msm(pairs[0..num_pairs], result)) OK else ERR;
}
fn bls12_g2_add(p1: *const [192]u8, p2: *const [192]u8, result: *[192]u8) callconv(.c) i32 {
    return if (guest_crypto.g2Add(p1, p2, result)) OK else ERR;
}
fn bls12_g2_msm(pairs: [*]const Bls12G2MsmPair, num_pairs: usize, result: *[192]u8) callconv(.c) i32 {
    return if (guest_crypto.g2Msm(pairs[0..num_pairs], result)) OK else ERR;
}
fn bls12_pairing(pairs: [*]const Bls12PairingPair, num_pairs: usize, verified: *bool) callconv(.c) i32 {
    return if (guest_crypto.pairingCheck(pairs[0..num_pairs], verified)) OK else ERR;
}
fn bls12_map_fp_to_g1(field_element: *const [48]u8, result: *[96]u8) callconv(.c) i32 {
    return if (guest_crypto.mapFpToG1(field_element, result)) OK else ERR;
}
fn bls12_map_fp2_to_g2(field_element: *const [96]u8, result: *[192]u8) callconv(.c) i32 {
    return if (guest_crypto.mapFp2ToG2(field_element, result)) OK else ERR;
}

// ── Runtime: zkvm_log ────────────────────────────────────────────────────────────────────────────
// Not a precompile, but the same "statically-linked, define every extern locally" situation applies:
// zesu's own root module (zesu/src/zkvm/root.zig) declares `extern fn zkvm_log(level, msg_ptr,
// msg_len)`, and the reference implementation for this backend forwards it to a Linux write ecall on
// fd=1. The Linea zkVM captures ALL stdout bytes as observable program output, so a real call here
// would surface diagnostics as part of the guest's own emissions. NO-OP for now; re-enable once ZkC
// exposes a logging channel of its own.
fn log(level: u8, msg_ptr: [*]const u8, msg_len: usize) callconv(.c) void {
    _ = level;
    _ = msg_ptr;
    _ = msg_len;
}

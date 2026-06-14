//! `zkvm_*` precompile providers for the ZkC guest.
//!
//! Zesu's freestanding build references every precompile as an `extern fn zkvm_*` symbol. The guest
//! ships as a *statically-linked* ELF (the zkvm-standards artifact), so there is no later link to
//! resolve anything: every one of those externs must be DEFINED in the binary. This module defines
//! all of them, from two sources:
//!
//!   • Linea accelerator wrappers (`linea_zkvm_accel`) — for the precompiles the prover accelerates
//!     (keccak today). We re-export each wrapper under the C name zesu references; HOW a wrapper
//!     accelerates is the wrapper module's own concern. The *set of wrappers that exist* is what is
//!     accelerated, and grows as the prover implements more.
//!   • zesu-zkvm `stdlibs_accel` (`zesu_zkvm_accel`) — every precompile without a wrapper yet, via a
//!     thin C-ABI shim (ptr+len → slice/array). Pure rv64im code; we don't maintain our own crypto.
//!     When a precompile gains a wrapper, move its line to the wrapper export below and delete its shim.
//!
//! Only the freestanding RISC-V guest references these (pulled in by evm_execution_guest.zig for
//! `builtin.cpu.arch == .riscv64`); the native host build uses Zesu's C-backed crypto instead.

const zesu_accel = @import("zesu_zkvm_accel"); // zesu-zkvm's pure-Zig precompile backend (stdlibs_accel)
const linea_accel = @import("linea_zkvm_accel"); // Linea accelerator wrappers (source paths wired in build.zig)

// The manifest: every `zkvm_*` symbol zesu references, and where each comes from — keccak from the
// Linea wrapper, the rest from the stdlibs_accel shims defined below.
comptime {
    @export(&linea_accel.keccak.zkvm_keccak256, .{ .name = "zkvm_keccak256" });
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
}

const OK: i32 = 0;
const ERR: i32 = 1;

// Pairing/MSM pair layouts — must byte-match the C-ABI struct layout zesu passes to these zkvm_*
// symbols; forwarded straight to stdlibs_accel's `anytype` parameters.
const Bn254PairingPair = extern struct { g1: [64]u8, g2: [128]u8 };
const Bls12G1MsmPair = extern struct { point: [96]u8, scalar: [32]u8 };
const Bls12G2MsmPair = extern struct { point: [192]u8, scalar: [32]u8 };
const Bls12PairingPair = extern struct { g1: [96]u8, g2: [192]u8 };

// ── C-ABI shims: extern zkvm_* (ptr+len) → stdlibs_accel's slice/array API ───────────────────────
// One per precompile that has no Linea wrapper yet; all exported in the comptime block above.

fn sha256(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    zesu_accel.sha256(data[0..len], output);
    return OK;
}
fn ripemd160(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    zesu_accel.ripemd160(data[0..len], output);
    return OK;
}
fn secp256k1_ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) callconv(.c) i32 {
    return if (zesu_accel.ecrecover(msg, sig, recid, output)) OK else ERR;
}
fn secp256k1_verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) callconv(.c) i32 {
    zesu_accel.secp256k1_verify(msg, sig, pubkey, verified);
    return OK;
}
fn secp256r1_verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) callconv(.c) i32 {
    zesu_accel.secp256r1_verify(msg, sig, pubkey, verified);
    return OK;
}
fn modexp(base: [*]const u8, base_len: usize, exp: [*]const u8, exp_len: usize, modulus: [*]const u8, mod_len: usize, output: [*]u8) callconv(.c) i32 {
    return if (zesu_accel.modexp(base[0..base_len], exp[0..exp_len], modulus[0..mod_len], output[0..mod_len])) OK else ERR;
}
fn bn254_g1_add(p1: *const [64]u8, p2: *const [64]u8, result: *[64]u8) callconv(.c) i32 {
    return if (zesu_accel.bn254_g1_add(p1, p2, result)) OK else ERR;
}
fn bn254_g1_mul(point: *const [64]u8, scalar: *const [32]u8, result: *[64]u8) callconv(.c) i32 {
    return if (zesu_accel.bn254_g1_mul(point, scalar, result)) OK else ERR;
}
fn bn254_pairing(pairs: [*]const Bn254PairingPair, num_pairs: usize, verified: *bool) callconv(.c) i32 {
    return if (zesu_accel.bn254_pairing(pairs[0..num_pairs], verified)) OK else ERR;
}
fn blake2f(rounds: u32, h: *[64]u8, m: *const [128]u8, t: *const [16]u8, f: u8) callconv(.c) i32 {
    return if (zesu_accel.blake2f(rounds, h, m, t, f)) OK else ERR;
}
fn kzg_point_eval(commitment: *const [48]u8, z: *const [32]u8, y: *const [32]u8, proof: *const [48]u8, verified: *bool) callconv(.c) i32 {
    return if (zesu_accel.kzg_point_eval(commitment, z, y, proof, verified)) OK else ERR;
}
fn bls12_g1_add(p1: *const [96]u8, p2: *const [96]u8, result: *[96]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_g1_add(p1, p2, result)) OK else ERR;
}
fn bls12_g1_msm(pairs: [*]const Bls12G1MsmPair, num_pairs: usize, result: *[96]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_g1_msm(pairs[0..num_pairs], result)) OK else ERR;
}
fn bls12_g2_add(p1: *const [192]u8, p2: *const [192]u8, result: *[192]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_g2_add(p1, p2, result)) OK else ERR;
}
fn bls12_g2_msm(pairs: [*]const Bls12G2MsmPair, num_pairs: usize, result: *[192]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_g2_msm(pairs[0..num_pairs], result)) OK else ERR;
}
fn bls12_pairing(pairs: [*]const Bls12PairingPair, num_pairs: usize, verified: *bool) callconv(.c) i32 {
    return if (zesu_accel.bls12_pairing(pairs[0..num_pairs], verified)) OK else ERR;
}
fn bls12_map_fp_to_g1(field_element: *const [48]u8, result: *[96]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_map_fp_to_g1(field_element, result)) OK else ERR;
}
fn bls12_map_fp2_to_g2(field_element: *const [96]u8, result: *[192]u8) callconv(.c) i32 {
    return if (zesu_accel.bls12_map_fp2_to_g2(field_element, result)) OK else ERR;
}

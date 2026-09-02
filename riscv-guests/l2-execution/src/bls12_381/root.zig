// SPDX-License-Identifier: MIT
//! bls12_381 — the BLS12-381 pairing-friendly elliptic curve: the base
//! field `Fp` and its extension tower `Fp2`/`Fp6`/`Fp12`, the scalar
//! field `Fr`, and the two pairing groups `G1`/`G2`. This is the
//! foundation for BLS signatures, KZG polynomial commitments, and
//! threshold-BLS schemes — see `README.md` for the full multi-part arc
//! this module is Part 1 of.
//!
//! **Status: Parts 1-5 complete.** Full field-tower and group
//! arithmetic plus the pairing itself (`e: G1 x G2 -> Gt`,
//! `pairing.zig`): the optimal ate Miller loop (D-type-twist line
//! evaluation) and the full final exponentiation (easy part + the
//! Hayashida-Hayasaka-Teruya exact hard-part chain), every constant
//! independently verified (see `NOTICE`), constant-time scalar
//! multiplication, and KAT coverage in Debug and ReleaseFast — the
//! pairing is pinned byte-exactly against the IETF
//! pairing-friendly-curves draft's official optimal-ate test vector
//! plus a py_ecc cross-check, on top of a full bilinearity property
//! suite. Part 3 (`hash_to_curve.zig`) is RFC 9380 hash-to-curve for
//! both groups (`BLS12381G{1,2}_XMD:SHA-256_SSWU_RO_`/`_NU_`), pinned
//! byte-exact against RFC 9380's own Appendix J.9.1/J.10.1 vectors at
//! every stage (`u`, `Q0`/`Q1`, final `P`). Part 4 (`bls_sig.zig`) is
//! **COMPLETE**: BLS signatures per draft-irtf-cfrg-bls-signature-05,
//! minimal-pubkey-size/ProofOfPossession ciphersuite — wire codecs,
//! `keyGen`, `skToPk`, `keyValidate`, aggregation, AND the pairing-based
//! `sign`/`verify`/`coreAggregateVerify`/`fastAggregateVerify`/
//! `popProve`/`popVerify` cores, byte-exact against
//! `ethereum/bls12-381-tests` v0.1.2 vectors, with the mandatory
//! subgroup/`KeyValidate` checks fail-closed at every verify entry
//! point — see `bls_sig.zig`'s own module doc comment. Part 5
//! (`kzg.zig`) is **COMPLETE**: EIP-4844 (deneb) KZG polynomial
//! commitments — the embedded official ceremony trusted setup
//! (validated once per process, memoized), `blobToKzgCommitment`,
//! `computeKzgProof`/`verifyKzgProof`, the blob-level proof functions
//! and batch verification, plus reusable `Fr` FFT and Pippenger `G1`
//! MSM primitives — byte-exact against `ethereum/c-kzg-4844`'s KAT
//! vectors, see `kzg.zig`'s own module doc comment. Part 6
//! (`threshold.zig`) is **COMPLETE**: trusted-dealer threshold BLS —
//! Shamir secret sharing + Feldman VSS + Lagrange-in-the-exponent
//! combining, over Part 4's min-pk ciphersuite. The keystone
//! self-consistency chain (split → partial-sign → combine equals
//! `bls_sig.sign` byte-for-byte, verifying under the group public key)
//! transitively pins the threshold path to Part 4's eth-pinned vectors
//! — see `threshold.zig`'s own module doc comment and `SPEC.md`.
//! See `SPEC.md` for the design record and the BLS subgroup-check
//! pitfall this module's API is built around (deserialization does NOT
//! subgroup-check; callers at trust boundaries must).

const std = @import("std");

pub const fp = @import("fp.zig");
pub const fp2 = @import("fp2.zig");
pub const fp6 = @import("fp6.zig");
pub const fp12 = @import("fp12.zig");
pub const g1 = @import("g1.zig");
pub const g2 = @import("g2.zig");
pub const scalar = @import("scalar.zig");
pub const pairing = @import("pairing.zig");
pub const hash_to_curve = @import("hash_to_curve.zig");
pub const bls_sig = @import("bls_sig.zig");
pub const kzg = @import("kzg.zig");
pub const threshold = @import("threshold.zig");

pub const Fp = fp.Fp;
pub const Fp2 = fp2.Fp2;
pub const Fp6 = fp6.Fp6;
pub const Fp12 = fp12.Fp12;
pub const Fr = scalar.Fr;
pub const G1 = g1;
pub const G2 = g2;
pub const Gt = pairing.Gt;

pub const meta = .{
    // The module catalog's one-line entry. This IS the source of truth:
    // README.md's table is rendered from it by `zig build gen-catalog`.
    .doc = "BLS12-381 pairing-friendly curve — field tower/groups, optimal-ate pairing, hash-to-curve, BLS signatures, KZG commitments, threshold BLS.",
    // The catalog's Platform cell. Prose, because it carries nuance the
    // `platform` enum below cannot -- "any (packer: linux)", "amd64 asm +
    // portable fallback". Rendered by `gen-catalog` alongside `doc`.
    .platform_note = "any",
    .targets = .{.linux64},
    .platform = .any,
    .role = .util, // pure computation — no I/O, no wire framing beyond point (de)serialization
    .concurrency = .reentrant, // every type is a plain value type; kzg's only global is a write-once atomic memo of the embedded (compile-time-constant) trusted setup
    .model_after = "draft-irtf-cfrg-pairing-friendly-curves (the BLS12-381 parameter set) + the ZCash/IETF BLS12-381 point-serialization convention; std.crypto.ff supplies the constant-time Montgomery modular arithmetic Fp/Fr are built on",
    .deps = .{"entropy"}, // otherwise std only (std.crypto.ff); entropy backs `Fr.random`
};

// ── dark-tests aggregator (CONVENTIONS.md §6 step 3) ────────────────────
//
// A bare `pub const x = @import("x.zig")` re-export does NOT pull x's
// tests into the test binary on its own — every submodule must be named
// here too (the dark-tests rule; see CONVENTIONS.md).
test {
    _ = fp;
    _ = fp2;
    _ = fp6;
    _ = fp12;
    _ = g1;
    _ = g2;
    _ = scalar;
    _ = pairing;
    _ = hash_to_curve;
    _ = bls_sig;
    _ = kzg;
    _ = threshold;
}

test "meta.model_after names the pairing-friendly-curves draft" {
    try std.testing.expect(std.mem.indexOf(u8, meta.model_after, "pairing-friendly-curves") != null);
}

test "field tower encoded widths compose correctly" {
    try std.testing.expectEqual(@as(usize, 48), Fp.encoded_bytes);
    try std.testing.expectEqual(@as(usize, 96), Fp2.encoded_bytes);
    try std.testing.expectEqual(@as(usize, 288), Fp6.encoded_bytes);
    try std.testing.expectEqual(@as(usize, 576), Fp12.encoded_bytes);
    try std.testing.expectEqual(@as(usize, 32), Fr.encoded_bytes);
    try std.testing.expectEqual(@as(usize, 48), G1.compressed_bytes);
    try std.testing.expectEqual(@as(usize, 96), G1.uncompressed_bytes);
    try std.testing.expectEqual(@as(usize, 96), G2.compressed_bytes);
    try std.testing.expectEqual(@as(usize, 192), G2.uncompressed_bytes);
}

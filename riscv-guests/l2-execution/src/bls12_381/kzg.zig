// SPDX-License-Identifier: MIT
//! KZG polynomial commitments — EIP-4844 (Deneb "blob transactions")
//! `blob_to_kzg_commitment`/`compute_kzg_proof`/`verify_kzg_proof`/
//! `compute_blob_kzg_proof`/`verify_blob_kzg_proof`/
//! `verify_blob_kzg_proof_batch`, over BLS12-381's `G1`/`G2`/`Fr`
//! (Parts 1-2 of this module's arc) — Part 5 of `README.md`'s multi-part
//! arc. KZG commitments let a prover commit to a degree-4095 polynomial
//! (equivalently, a 128 KiB "blob" of 4096 32-byte field elements) with a
//! single 48-byte `G1` point, then open it at any point `z` with a
//! constant-size (48-byte) proof — the primitive behind Ethereum's blob
//! transactions (EIP-4844) and every KZG-based data-availability scheme.
//!
//! **Status: COMPLETE (crypto-core pass 2026-07-14).** Model-after:
//! `consensus-specs` `specs/deneb/polynomial-commitments.md` (fetched
//! 2026-07-14 from `github.com/ethereum/consensus-specs` `master` branch
//! — see `NOTICE` for the exact commit-independent citation and every
//! constant's provenance; re-fetched and re-read line-by-line during the
//! crypto-core pass, not recalled). Everything is REAL: the
//! trusted-setup loader (hex parsing + on-curve + subgroup validation of
//! every embedded point, memoized process-wide — see `loadTrustedSetup`),
//! blob<->polynomial (de)serialization with canonical-field-element
//! enforcement, `G1`/`KZGCommitment`/`KZGProof` validation
//! (`validateKzgG1`, allowing the identity point per spec), the
//! bit-reversal-permutation helpers, `computePowers`, AND the crypto
//! core: `g1Msm` (Pippenger bucket multi-scalar multiplication over
//! variable-time Jacobian arithmetic — KZG data is public, see
//! `SPEC.md`'s Part-5 threat-model note), the `Fr` radix-2 NTT
//! (`fft`/`ifft`) and the primitive `2^32` root of unity (derived at
//! runtime from the spec's own `PRIMITIVE_ROOT_OF_UNITY = 7` formula,
//! never a transcribed 32-byte constant),
//! `evaluatePolynomialInEvaluationForm` (barycentric evaluation with the
//! in-domain special case), `computeKzgProofImpl` (the evaluation-form
//! quotient construction, including the z-in-domain
//! removable-singularity case), `verifyKzgProofImpl`/
//! `verifyKzgProofBatchImpl` (the pairing-product checks), and
//! `computeChallenge` (Fiat-Shamir). Byte-exact against
//! `ethereum/c-kzg-4844`'s own KAT vectors across every EIP-4844 public
//! function — see the tests at the bottom of this file and `NOTICE`.
//!
//! Zig std GAP: yes, same shape as `pairing.zig`/`hash_to_curve.zig` —
//! nothing in `std` supplies KZG; this file is this module's own, built
//! entirely on Parts 1-2's already-real `Fr`/`G1`/`G2`/pairing primitives
//! plus `std.crypto.hash.sha2.Sha256` (the spec's `hash`, per
//! `consensus-specs` `specs/phase0/beacon-chain.md`: `hash(data: bytes)
//! -> Bytes32` is SHA256 — see `NOTICE`).

const std = @import("std");
const g1 = @import("g1.zig");
const g2 = @import("g2.zig");
const scalar = @import("scalar.zig");
const pairing = @import("pairing.zig");

pub const Fr = scalar.Fr;

// ── constants (EIP-4844 / deneb polynomial-commitments spec) ───────────
//
// Every value below is copied VERBATIM from the fetched spec text (see
// `NOTICE` for the exact fetch), not transcribed from memory — the same
// "cite, don't recall" discipline Parts 1-4 use for every curve constant.

/// The blob width: 4096 field elements = one degree-4095 polynomial.
/// `FIELD_ELEMENTS_PER_BLOB = 2^12` (spec "Preset" table).
pub const FIELD_ELEMENTS_PER_BLOB: usize = 4096;

/// Bytes per BLS scalar field element (`Fr`, big-endian, canonical <
/// `BLS_MODULUS`). Spec "Constants" table.
pub const BYTES_PER_FIELD_ELEMENT: usize = 32;

/// `BYTES_PER_FIELD_ELEMENT * FIELD_ELEMENTS_PER_BLOB` = 131072 (128 KiB)
/// — the wire size of one blob. Spec "Constants" table.
pub const BYTES_PER_BLOB: usize = BYTES_PER_FIELD_ELEMENT * FIELD_ELEMENTS_PER_BLOB;

/// Compressed `G1` point width — a `KZGCommitment`/`KZGProof`'s wire
/// size. Spec "Constants" table. Matches `g1.compressed_bytes` (Part 1) —
/// asserted equal in this file's tests.
pub const BYTES_PER_COMMITMENT: usize = 48;

/// Same width as `BYTES_PER_COMMITMENT` — a `KZGProof` is also a
/// compressed `G1` point. Spec "Constants" table.
pub const BYTES_PER_PROOF: usize = 48;

/// The number of `G2` points in the trusted setup's monomial basis:
/// `[1]G2, [s]G2, [s^2]G2, ..., [s^64]G2`. Spec "Trusted setup" preset
/// table (`KZG_SETUP_G2_LENGTH`). `verifyKzgProofImpl` only ever needs
/// index 1 (`[s]G2`), but the full ceremony output publishes all 65.
pub const NUM_G2_POINTS: usize = 65;

/// The BLS12-381 scalar field modulus `r`, as the spec's own decimal
/// constant re-expressed in bytes — REUSES `scalar.r_bytes` (already
/// independently re-derived and verified, `scalar.zig`/`NOTICE`) rather
/// than re-embedding a second copy of the same 32-byte value. Cross-
/// checked byte-for-byte against the spec's own decimal literal
/// `52435875175126190479447740508185965837690552500527637822603658699938581184513`
/// (see `NOTICE` for the independent conversion).
pub const BLS_MODULUS: [32]u8 = scalar.r_bytes;

/// The generator this module's Fr scalar field's roots of unity are
/// derived from (`compute_roots_of_unity`, spec "BLS12-381 helpers"):
/// `root_of_unity(order) = pow(PRIMITIVE_ROOT_OF_UNITY, (BLS_MODULUS-1)/order,
/// BLS_MODULUS)`. A small (7), directly spec-cited integer — NOT a
/// "guessed" field-element constant (the specific 32-byte 2^32 root it
/// generates is deliberately never embedded; it is DERIVED at runtime —
/// see `primitiveRootOfUnity2Pow32`'s doc comment).
pub const PRIMITIVE_ROOT_OF_UNITY: u64 = 7;

/// Fiat-Shamir domain separator for `compute_challenge` (single-proof
/// `verify_blob_kzg_proof`/`compute_blob_kzg_proof`). 16 bytes, spec
/// "Preset" table.
pub const FIAT_SHAMIR_PROTOCOL_DOMAIN = "FSBLOBVERIFY_V1_";

/// Fiat-Shamir domain separator for `verify_kzg_proof_batch`'s random
/// linear-combination challenge `r`. 16 bytes, spec "Preset" table.
pub const RANDOM_CHALLENGE_KZG_BATCH_DOMAIN = "RCKZGBATCH___V1_";

/// The compressed encoding of the `G1` point at infinity:
/// `0xc0` followed by 47 zero bytes — spec "Constants" table
/// (`G1_POINT_AT_INFINITY`). Matches this module's own ZCash/IETF
/// compressed-identity encoding (`g1.zig`'s `toBytesCompressed` on
/// `Affine.identity`; cross-checked in this file's tests) — `validateKzgG1`
/// special-cases exactly this value as the one `KZGCommitment`/`KZGProof`
/// allowed to skip the subgroup check (spec: "Perform ... KeyValidate ...
/// but do allow the identity point").
pub const G1_POINT_AT_INFINITY: Bytes48 = blk: {
    var out: Bytes48 = [_]u8{0} ** 48;
    out[0] = 0xc0;
    break :blk out;
};

// ── wire types ───────────────────────────────────────────────────────────

pub const Bytes32 = [32]u8;
pub const Bytes48 = [48]u8;

/// A blob: `FIELD_ELEMENTS_PER_BLOB` field elements, each
/// `BYTES_PER_FIELD_ELEMENT` big-endian bytes, concatenated —
/// `BYTES_PER_BLOB` (131072) bytes total. Not necessarily canonical per
/// element until `blobToPolynomial` validates it (spec's own `Blob` SSZ
/// type is likewise just a raw byte vector; the field-element canonical-
/// ness check happens at `blob_to_polynomial`/`bytes_to_bls_field`, not
/// at the type boundary — see `SPEC.md`'s threat-model note on this).
pub const Blob = [BYTES_PER_BLOB]u8;

/// 48-byte compressed `G1` point, per `g1.zig`'s wire convention (the
/// same ZCash/IETF encoding this module uses throughout — EIP-4844's own
/// `G1Point`/`KZGCommitment` SSZ type is a plain `Bytes48`, same
/// convention, cross-checked by `G1_POINT_AT_INFINITY` above).
pub const KZGCommitment = Bytes48;

/// Same wire shape as `KZGCommitment` (spec: "Same as for
/// `KZGCommitment`" validation too — `bytesToKzgProof` is `bytesToKzgCommitment`
/// under a different name).
pub const KZGProof = Bytes48;

// ── errors ───────────────────────────────────────────────────────────────

pub const KzgError = error{
    /// A 32-byte field element (a blob element, `z`, or `y`) is `>=
    /// BLS_MODULUS` — REJECTED, never reduced (spec's `bytes_to_bls_field`:
    /// "does not accept inputs greater than the BLS modulus").
    InvalidFieldElement,
    /// A `KZGCommitment`/`KZGProof` failed `validateKzgG1`: not a valid
    /// compressed `G1` encoding, not on-curve, or on-curve but not in the
    /// order-`r` subgroup (and not the identity, which is explicitly
    /// allowed).
    InvalidCommitment,
    /// `blobs`/`commitments`/`proofs`/`zs`/`ys` slice-length mismatch in
    /// a batch entry point.
    LengthMismatch,
    /// The embedded/loaded trusted setup's `G1`/`G2` line counts do not
    /// match `FIELD_ELEMENTS_PER_BLOB`/`NUM_G2_POINTS`.
    UnexpectedSetupShape,
    /// A setup line's hex decoded to the wrong byte width, or failed
    /// `Fp`/`Fp2` canonical parsing.
    MalformedTrustedSetup,
    /// The setup file ended before every expected line was read.
    TruncatedSetup,
    /// The setup file had extra, unexpected trailing lines.
    TrailingDataInSetup,
    /// A setup point decoded on-curve but is NOT in the order-`r`
    /// subgroup — see `g1.zig`/`g2.zig`'s `subgroupCheck`/`SPEC.md`'s
    /// central BLS pitfall; a trusted-setup file is exactly the kind of
    /// external, security-critical input this check exists for.
    PointNotInSubgroup,
} || std.mem.Allocator.Error;

// ── trusted setup ───────────────────────────────────────────────────────

/// The embedded official Ethereum KZG ceremony ("Summoning Ceremony")
/// trusted setup, `c-kzg-4844`'s `trusted_setup.txt` format — see
/// `NOTICE` for the exact fetch (URL + sha256) and ceremony attribution.
/// Format (this exact file, byte-for-byte, `data/trusted_setup.txt`):
/// line 1 = `4096` (`FIELD_ELEMENTS_PER_BLOB`), line 2 = `65`
/// (`NUM_G2_POINTS`), then 4096 hex lines of `G1` Lagrange points (96 hex
/// chars = 48 bytes compressed each), then 65 hex lines of `G2` monomial
/// points (192 hex chars = 96 bytes compressed each), then 4096 more hex
/// lines of `G1` monomial points. Confirmed structurally (line count,
/// line width) AND semantically (`g1_monomial[0]` decodes to exactly this
/// module's own `g1.Affine.generator` compressed bytes, `g2_monomial[0]`
/// to `g2.Affine.generator`'s — i.e. `[s^0]G = G`, the expected monomial-
/// basis identity) during this scaffolding pass — see `NOTICE`.
const trusted_setup_txt = @embedFile("data/trusted_setup.txt");

/// The parsed, validated trusted setup: every point on-curve AND
/// subgroup-checked at load time (`loadTrustedSetup`) — REAL, no
/// deferred validation anywhere downstream trusts an unchecked point.
///
/// **Basis-order note (mechanical, verified against `c-kzg-4844`'s own C
/// loader `src/setup/setup.c`, fetched 2026-07-14 — see `NOTICE`):**
/// `g1_lagrange` is stored in the trusted-setup FILE's NATURAL order
/// (`c-kzg-4844`'s own `load_trusted_setup` reads the file into
/// `g1_values_lagrange_brp` and only THEN calls
/// `bit_reversal_permutation(...)` on it in-place — i.e. the file itself
/// is NOT pre-permuted). Consumers computing `blob_to_kzg_commitment`
/// MUST apply `bitReversalPermutation` (this file, REAL) to
/// `g1_lagrange` (or equivalently to the blob-derived polynomial) before
/// using it as the direct per-index Lagrange basis — exactly what the
/// spec's own `blob_to_kzg_commitment` does
/// (`bit_reversal_permutation(KZG_SETUP_G1_LAGRANGE)`). `blobToKzgCommitment`
/// below already does this (the ONE call site that needs it in this
/// file's public API).
pub const TrustedSetup = struct {
    /// `FIELD_ELEMENTS_PER_BLOB` `G1` points, Lagrange (evaluation) basis
    /// over the roots of unity, FILE/NATURAL order (see struct doc
    /// comment's basis-order note — NOT bit-reversal-permuted).
    g1_lagrange: []g1.Affine,
    /// `FIELD_ELEMENTS_PER_BLOB` `G1` points, monomial basis:
    /// `g1_monomial[i] = [s^i] G1` for the ceremony's secret `s`.
    g1_monomial: []g1.Affine,
    /// `NUM_G2_POINTS` `G2` points, monomial basis:
    /// `g2_monomial[i] = [s^i] G2`. `verifyKzgProofImpl` needs
    /// `g2_monomial[1] = [s] G2`.
    g2_monomial: []g2.Affine,

    pub fn deinit(self: *TrustedSetup, allocator: std.mem.Allocator) void {
        allocator.free(self.g1_lagrange);
        allocator.free(self.g1_monomial);
        allocator.free(self.g2_monomial);
        self.* = undefined;
    }
};

// ── variable-time G1 helpers (public-data fast paths) ──────────────────
//
// KZG operates exclusively on PUBLIC data (`SPEC.md`'s Part-5
// threat-model note): blobs, commitments, proofs, the evaluation point,
// and the trusted setup are all public in every anticipated consumer, so
// the constant-time discipline `g1.zig`'s `add`/`scalarMul` apply (needed
// there for secret BLS keys) buys nothing here and costs ~3-5x (the
// branchless `ctSelect` resolution fully computes BOTH the general-case
// formula AND `double(a)` on every single addition). These are the SAME
// EFD formulas as `g1.zig`'s (`add-2007-bl`, its `madd-2007-bl` mixed
// variant, and the shared `dbl-2009-l` via `Jacobian.double`) with
// ordinary branches for the degenerate cases instead of `ctSelect`
// masks — verified equal to the constant-time versions across random
// AND every degenerate input class by this file's own tests. Never use
// these on secret scalars or secret points.

/// Variable-time Jacobian + Jacobian addition (EFD `add-2007-bl`, the
/// branchy twin of `g1.Jacobian.add` — see the section comment above).
fn jacAddVartime(a: g1.Jacobian, b: g1.Jacobian) g1.Jacobian {
    if (a.isIdentity()) return b;
    if (b.isIdentity()) return a;
    const z1z1 = a.z.square();
    const z2z2 = b.z.square();
    const ua = a.x.mul(z2z2); // U1
    const ub = b.x.mul(z1z1); // U2
    const sa = a.y.mul(b.z).mul(z2z2); // S1
    const sb = b.y.mul(a.z).mul(z1z1); // S2
    const h = ub.sub(ua);
    const s_diff = sb.sub(sa);
    if (h.isZero()) {
        if (s_diff.isZero()) return a.double(); // P == Q
        return g1.Jacobian.identity; // P == -Q
    }
    const i = h.add(h).square();
    const j = h.mul(i);
    const rr = s_diff.add(s_diff);
    const v = ua.mul(i);
    const x3 = rr.square().sub(j).sub(v.add(v));
    const s1j = sa.mul(j);
    const y3 = rr.mul(v.sub(x3)).sub(s1j.add(s1j));
    const z3 = a.z.add(b.z).square().sub(z1z1).sub(z2z2).mul(h);
    return .{ .x = x3, .y = y3, .z = z3 };
}

/// Variable-time Jacobian + Affine ("mixed") addition (EFD
/// `madd-2007-bl`: `add-2007-bl` specialized to `Z2 = 1`, saving the
/// `Z2`-dependent multiplications) — the inner-loop operation
/// `g1Msm`'s bucket accumulation lives on.
fn jacMixedAddVartime(a: g1.Jacobian, b: g1.Affine) g1.Jacobian {
    if (b.infinity) return a;
    if (a.isIdentity()) return g1.Jacobian.fromAffine(b);
    const z1z1 = a.z.square();
    const ub = b.x.mul(z1z1); // U2
    const sb = b.y.mul(a.z).mul(z1z1); // S2
    const h = ub.sub(a.x);
    const s_diff = sb.sub(a.y);
    if (h.isZero()) {
        if (s_diff.isZero()) return a.double(); // P == Q
        return g1.Jacobian.identity; // P == -Q
    }
    const hh = h.square();
    const i = blk: { // I = 4*HH
        const hh2 = hh.add(hh);
        break :blk hh2.add(hh2);
    };
    const j = h.mul(i);
    const rr = s_diff.add(s_diff);
    const v = a.x.mul(i);
    const x3 = rr.square().sub(j).sub(v.add(v));
    const yj = a.y.mul(j);
    const y3 = rr.mul(v.sub(x3)).sub(yj.add(yj));
    const z3 = a.z.add(h).square().sub(z1z1).sub(hh);
    return .{ .x = x3, .y = y3, .z = z3 };
}

/// Variable-time `[s]P` (big-endian byte-string scalar), plain
/// double-and-add with leading-zero skipping — used only where the
/// scalar is a fixed PUBLIC constant (the group order `r`, in
/// `subgroupCheckVartime`).
fn jacScalarMulVartime(p: g1.Jacobian, s: []const u8) g1.Jacobian {
    var acc = g1.Jacobian.identity;
    var started = false;
    for (s) |byte| {
        var bit: u3 = 7;
        while (true) : (bit -= 1) {
            if (started) acc = acc.double();
            if ((byte >> bit) & 1 == 1) {
                acc = jacAddVartime(acc, p);
                started = true;
            }
            if (bit == 0) break;
        }
    }
    return acc;
}

/// Variable-time `[r]P == O` subgroup check — `g1.Jacobian.subgroupCheck`'s
/// public-data twin (~3x faster: no double-and-add-ALWAYS, no per-bit
/// `ctSelect`), used for validating the embedded trusted setup's 8192
/// PUBLIC `G1` points. Equivalence with the constant-time check is
/// pinned by this file's own tests.
fn subgroupCheckVartime(p: g1.Jacobian) bool {
    return jacScalarMulVartime(p, &scalar.r_bytes).isIdentity();
}

fn trimLine(line: []const u8) []const u8 {
    return std.mem.trim(u8, line, " \t\r\n");
}

fn parseLineCount(line_opt: ?[]const u8) KzgError!usize {
    const line = trimLine(line_opt orelse return error.TruncatedSetup);
    return std.fmt.parseInt(usize, line, 10) catch error.MalformedTrustedSetup;
}

/// Parses one hex line into a `G1.Affine`, REAL end to end: hex decode
/// (exact-width check), `g1.fromBytesCompressed` (on-curve check), then
/// a subgroup check (REQUIRED — a trusted-setup file is untrusted input
/// until validated; see `KzgError.PointNotInSubgroup`'s doc comment).
/// The subgroup check is `subgroupCheckVartime` — setup points are
/// PUBLIC data (see the variable-time section comment above), and this
/// is the loader's dominant cost across 8192 `G1` points.
fn parseG1Line(line_raw: []const u8) KzgError!g1.Affine {
    const line = trimLine(line_raw);
    if (line.len != 2 * g1.compressed_bytes) return error.MalformedTrustedSetup;
    var bytes: [g1.compressed_bytes]u8 = undefined;
    _ = std.fmt.hexToBytes(&bytes, line) catch return error.MalformedTrustedSetup;
    const affine = g1.fromBytesCompressed(bytes) catch return error.MalformedTrustedSetup;
    if (!subgroupCheckVartime(g1.Jacobian.fromAffine(affine))) return error.PointNotInSubgroup;
    return affine;
}

/// `parseG1Line`'s `G2` mirror.
fn parseG2Line(line_raw: []const u8) KzgError!g2.Affine {
    const line = trimLine(line_raw);
    if (line.len != 2 * g2.compressed_bytes) return error.MalformedTrustedSetup;
    var bytes: [g2.compressed_bytes]u8 = undefined;
    _ = std.fmt.hexToBytes(&bytes, line) catch return error.MalformedTrustedSetup;
    const affine = g2.fromBytesCompressed(bytes) catch return error.MalformedTrustedSetup;
    if (!g2.Jacobian.fromAffine(affine).subgroupCheck()) return error.PointNotInSubgroup;
    return affine;
}

/// The process-wide, write-once cache of the parsed-and-validated
/// embedded setup (`loadTrustedSetup`'s memo). Sound to memoize because
/// the input is a `@embedFile`d COMPILE-TIME CONSTANT — every load
/// parses the identical bytes, so the first full validation's verdict
/// holds for every subsequent call; nothing is skipped, only not
/// repeated. Published via an atomic pointer (lock-free; a rare
/// first-load race means both racers validate and the loser frees its
/// copy — wasteful but correct). Backed by `std.heap.page_allocator`
/// and intentionally never freed: a process-lifetime read-only asset,
/// same reasoning as any other cached parse of embedded data.
var validated_setup_cache: std.atomic.Value(?*const TrustedSetup) = .init(null);

/// One worker's share of the parallel first-load validation: a
/// contiguous chunk of `G1` lines from each of the two `G1` sections
/// plus (for one worker) the 65 `G2` lines. Records the first error;
/// the spawner aggregates.
const SetupParseTask = struct {
    lag_lines: []const []const u8,
    lag_out: []g1.Affine,
    mon_lines: []const []const u8,
    mon_out: []g1.Affine,
    g2_lines: []const []const u8,
    g2_out: []g2.Affine,
    fail: ?KzgError = null,

    fn run(self: *SetupParseTask) void {
        for (self.lag_lines, self.lag_out) |line, *out|
            out.* = parseG1Line(line) catch |e| return self.abort(e);
        for (self.mon_lines, self.mon_out) |line, *out|
            out.* = parseG1Line(line) catch |e| return self.abort(e);
        for (self.g2_lines, self.g2_out) |line, *out|
            out.* = parseG2Line(line) catch |e| return self.abort(e);
    }

    fn abort(self: *SetupParseTask, e: KzgError) void {
        self.fail = e;
    }
};

/// The actual first-load parse + FULL validation of the embedded file —
/// all 8257 points hex-decoded, decompressed (on-curve by construction)
/// and subgroup-checked, exactly as before the memoization existed; the
/// per-point work is fanned out across CPU cores (the checks are
/// independent reads of disjoint outputs), falling back to serial if
/// spawning fails. Result arrays are `page_allocator`-owned (they back
/// the process-wide cache).
fn parseAndValidateEmbeddedSetup() KzgError!TrustedSetup {
    const gpa = std.heap.page_allocator; // global-alloc-ok: backs validated_setup_cache, a documented process-wide cache (see its doc comment above)
    var lines = std.mem.tokenizeScalar(u8, trusted_setup_txt, '\n');

    const n_g1 = try parseLineCount(lines.next());
    const n_g2 = try parseLineCount(lines.next());
    if (n_g1 != FIELD_ELEMENTS_PER_BLOB) return error.UnexpectedSetupShape;
    if (n_g2 != NUM_G2_POINTS) return error.UnexpectedSetupShape;

    // Collect every line slice first (cheap), so the expensive per-point
    // work below can be chunked across workers.
    const lag_lines = try gpa.alloc([]const u8, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(lag_lines);
    const g2_lines = try gpa.alloc([]const u8, NUM_G2_POINTS);
    defer gpa.free(g2_lines);
    const mon_lines = try gpa.alloc([]const u8, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(mon_lines);
    for (lag_lines) |*l| l.* = lines.next() orelse return error.TruncatedSetup;
    for (g2_lines) |*l| l.* = lines.next() orelse return error.TruncatedSetup;
    for (mon_lines) |*l| l.* = lines.next() orelse return error.TruncatedSetup;
    if (lines.next() != null) return error.TrailingDataInSetup;

    const g1_lagrange = try gpa.alloc(g1.Affine, FIELD_ELEMENTS_PER_BLOB);
    errdefer gpa.free(g1_lagrange);
    const g1_monomial = try gpa.alloc(g1.Affine, FIELD_ELEMENTS_PER_BLOB);
    errdefer gpa.free(g1_monomial);
    const g2_monomial = try gpa.alloc(g2.Affine, NUM_G2_POINTS);
    errdefer gpa.free(g2_monomial);

    const n_workers = @min(@max(std.Thread.getCpuCount() catch 1, 1), 16);
    var tasks: [16]SetupParseTask = undefined;
    const chunk = (FIELD_ELEMENTS_PER_BLOB + n_workers - 1) / n_workers;
    for (tasks[0..n_workers], 0..) |*task, w| {
        const lo = @min(w * chunk, FIELD_ELEMENTS_PER_BLOB);
        const hi = @min(lo + chunk, FIELD_ELEMENTS_PER_BLOB);
        task.* = .{
            .lag_lines = lag_lines[lo..hi],
            .lag_out = g1_lagrange[lo..hi],
            .mon_lines = mon_lines[lo..hi],
            .mon_out = g1_monomial[lo..hi],
            // All 65 G2 lines ride with worker 0 (65 constant-time G2
            // checks ≈ a few hundred G1-check units — comparable to one
            // worker's G1 chunk, so this stays roughly balanced).
            .g2_lines = if (w == 0) g2_lines else g2_lines[0..0],
            .g2_out = if (w == 0) g2_monomial else g2_monomial[0..0],
        };
    }

    var threads: [16]?std.Thread = @splat(null);
    for (tasks[0..n_workers], 0..) |*task, w| {
        threads[w] = std.Thread.spawn(.{}, SetupParseTask.run, .{task}) catch null;
        if (threads[w] == null) task.run(); // spawn failed: do it inline
    }
    for (threads[0..n_workers]) |maybe| if (maybe) |t| t.join();
    for (tasks[0..n_workers]) |task| if (task.fail) |e| return e;

    return .{ .g1_lagrange = g1_lagrange, .g1_monomial = g1_monomial, .g2_monomial = g2_monomial };
}

/// Deep-copies the cached setup into caller-owned (freeable) memory.
fn dupeSetup(allocator: std.mem.Allocator, src: *const TrustedSetup) KzgError!TrustedSetup {
    const g1_lagrange = try allocator.dupe(g1.Affine, src.g1_lagrange);
    errdefer allocator.free(g1_lagrange);
    const g1_monomial = try allocator.dupe(g1.Affine, src.g1_monomial);
    errdefer allocator.free(g1_monomial);
    const g2_monomial = try allocator.dupe(g2.Affine, src.g2_monomial);
    return .{ .g1_lagrange = g1_lagrange, .g1_monomial = g1_monomial, .g2_monomial = g2_monomial };
}

/// Parses AND validates the embedded `trusted_setup.txt` (`NOTICE`) into
/// a heap-allocated `TrustedSetup` — REAL (every step is mechanical
/// hex-decode + the already-real `fromBytesCompressed` plus a subgroup
/// check per point; no MSM, FFT, or pairing math is involved in LOADING
/// the setup, only in USING it). Caller owns the result; free with
/// `TrustedSetup.deinit`.
///
/// Cost/caching note: the full validation — all `2*FIELD_ELEMENTS_PER_BLOB
/// + NUM_G2_POINTS` = 8257 points subgroup-checked (one `[r]P` scalar
/// multiplication each) — runs ONCE per process and is memoized
/// (`validated_setup_cache`); every subsequent call deep-copies the
/// already-validated points into the caller's allocator. Memoization is
/// semantically transparent because the input is a compile-time-constant
/// `@embedFile` — identical bytes in, identical verdict out; NO point
/// ever skips validation, it just isn't re-validated per call (the
/// scaffold's original per-call re-validation cost minutes per test
/// binary for zero added assurance). The first load's per-point checks
/// are additionally fanned out across CPU cores and use the
/// variable-time subgroup check for the PUBLIC `G1` setup points (see
/// `subgroupCheckVartime`).
pub fn loadTrustedSetup(allocator: std.mem.Allocator) KzgError!TrustedSetup {
    if (validated_setup_cache.load(.acquire)) |cached| return dupeSetup(allocator, cached);

    var fresh = try parseAndValidateEmbeddedSetup();
    const gpa = std.heap.page_allocator; // global-alloc-ok: publishes into validated_setup_cache, the same process-wide cache
    const boxed = gpa.create(TrustedSetup) catch return error.OutOfMemory;
    boxed.* = fresh;
    if (validated_setup_cache.cmpxchgStrong(null, boxed, .acq_rel, .acquire)) |raced| {
        // Another thread published first — ours was redundant work.
        gpa.destroy(boxed);
        fresh.deinit(gpa);
        return dupeSetup(allocator, raced.?);
    }
    return dupeSetup(allocator, boxed);
}

// ── BLS12-381 / field-element helpers (spec "BLS12-381 helpers") ───────
//
// All REAL — thin, mechanical wrappers over Parts 1/2's already-real
// `Fr`/`G1` primitives, no new field or curve arithmetic.

/// Untrusted bytes -> validated `Fr`: REJECTS `>= BLS_MODULUS` (spec's
/// `bytes_to_bls_field` — note this is a REJECTING parse, unlike
/// `scalar.Fr.reduceWide`'s wide-input REDUCING parse; `Fr.fromBytes`
/// already has exactly this rejecting behavior, so this is a thin
/// error-renaming wrapper).
pub fn bytesToBlsField(bytes: Bytes32) KzgError!Fr {
    return Fr.fromBytes(bytes) catch error.InvalidFieldElement;
}

/// `Fr` -> big-endian 32 bytes (spec's `bls_field_to_bytes`). Thin
/// wrapper over the already-real `Fr.toBytes`.
pub fn blsFieldToBytes(x: Fr) Bytes32 {
    return x.toBytes();
}

/// Spec's `validate_kzg_g1`: allow the identity point through
/// unconditionally (compared byte-for-byte against
/// `G1_POINT_AT_INFINITY`, the canonical compressed-infinity encoding —
/// no curve arithmetic needed for that comparison), otherwise require the
/// BLS "KeyValidate" check (on-curve + subgroup, `g1.fromBytesCompressed`
/// + `Jacobian.subgroupCheck`, mirroring `bls_sig.zig`'s own
/// `keyValidate` for the identical reason: an unchecked non-subgroup
/// point is the same small-subgroup attack class `SPEC.md`'s threat
/// model already centers on for every other pairing-based entry point in
/// this module).
pub fn validateKzgG1(bytes: Bytes48) KzgError!void {
    if (std.mem.eql(u8, &bytes, &G1_POINT_AT_INFINITY)) return;
    const affine = g1.fromBytesCompressed(bytes) catch return error.InvalidCommitment;
    if (!g1.Jacobian.fromAffine(affine).subgroupCheck()) return error.PointNotInSubgroup;
}

/// Spec's `bytes_to_kzg_commitment`: `validateKzgG1` then decode.
pub fn bytesToKzgCommitment(bytes: Bytes48) KzgError!g1.Affine {
    try validateKzgG1(bytes);
    return g1.fromBytesCompressed(bytes) catch unreachable; // re-decoding what validateKzgG1 just validated
}

/// Spec's `bytes_to_kzg_proof`: identical validation to
/// `bytesToKzgCommitment` (spec: "Same as for `KZGCommitment`").
pub fn bytesToKzgProof(bytes: Bytes48) KzgError!g1.Affine {
    return bytesToKzgCommitment(bytes);
}

/// Spec's `blob_to_polynomial`: splits a blob into its
/// `FIELD_ELEMENTS_PER_BLOB` `Fr` coefficients (evaluation-form
/// polynomial), REJECTING any non-canonical element (`bytesToBlsField`).
/// Caller-owned result (`allocator.free`) — `FIELD_ELEMENTS_PER_BLOB` `Fr`
/// values (131072 bytes) is too large to comfortably return by value on
/// the stack, same reasoning as `TrustedSetup`'s heap allocation.
pub fn blobToPolynomial(allocator: std.mem.Allocator, blob: *const Blob) KzgError![]Fr {
    const poly = try allocator.alloc(Fr, FIELD_ELEMENTS_PER_BLOB);
    errdefer allocator.free(poly);
    for (poly, 0..) |*fe, i| {
        const off = i * BYTES_PER_FIELD_ELEMENT;
        const fe_bytes = blob[off..][0..BYTES_PER_FIELD_ELEMENT].*;
        fe.* = bytesToBlsField(fe_bytes) catch return error.InvalidFieldElement;
    }
    return poly;
}

// ── bit-reversal permutation (spec "Helpers" — pure combinatorics) ─────
//
// Direct, mechanical translations of the spec's own Python
// (`is_power_of_two`/`reverse_bits`/`bit_reversal_permutation`) — no
// field or curve arithmetic. `FIELD_ELEMENTS_PER_BLOB`'s Lagrange basis
// and every roots-of-unity table this module produces are always used
// bit-reversal-permuted (spec's own convention, restated in every KZG
// function that touches them) — see `TrustedSetup`'s basis-order note.

pub fn isPowerOfTwo(value: usize) bool {
    return value > 0 and (value & (value - 1)) == 0;
}

/// Reverses the low `log2(order)` bits of `n` (spec's `reverse_bits`).
pub fn reverseBits(n: usize, order: usize) usize {
    std.debug.assert(isPowerOfTwo(order));
    const bits: u6 = std.math.log2_int(usize, order);
    var result: usize = 0;
    var x = n;
    var i: u6 = 0;
    while (i < bits) : (i += 1) {
        result = (result << 1) | (x & 1);
        x >>= 1;
    }
    return result;
}

/// Returns a caller-owned copy of `sequence` in bit-reversed-index order
/// (spec's `bit_reversal_permutation`) — an INVOLUTION (applying it twice
/// is the identity), exercised by this file's own tests.
pub fn bitReversalPermutation(comptime T: type, allocator: std.mem.Allocator, sequence: []const T) ![]T {
    const out = try allocator.alloc(T, sequence.len);
    errdefer allocator.free(out);
    for (out, 0..) |*slot, i| slot.* = sequence[reverseBits(i, sequence.len)];
    return out;
}

/// Spec's `compute_powers`: `[x^0, x^1, ..., x^(n-1)]`. REAL — plain
/// repeated `Fr.mul`, no new math.
pub fn computePowers(allocator: std.mem.Allocator, x: Fr, n: usize) ![]Fr {
    const out = try allocator.alloc(Fr, n);
    errdefer allocator.free(out);
    if (n == 0) return out;
    out[0] = Fr.one;
    for (1..n) |i| out[i] = out[i - 1].mul(x);
    return out;
}

// ── roots of unity ──────────────────────────────────────────────────────

/// A small non-negative integer as an `Fr` element (32-byte big-endian
/// embedding — always canonical for `u64`-sized values, `r` being
/// ~2^255).
fn frFromU64(v: u64) Fr {
    var bytes = [_]u8{0} ** 32;
    std.mem.writeInt(u64, bytes[24..32], v, .big);
    return Fr.fromBytes(bytes) catch unreachable; // v << r
}

/// `(r - 1) / 2^32`, big-endian — the exponent in the spec's own
/// `compute_roots_of_unity` formula specialized to `order = 2^32`.
/// Comptime-DERIVED from the already-verified `scalar.r_bytes` (never a
/// transcribed constant): `r` ends in the byte `0x01` (its low 32 bits
/// are `0x00000001` — visible in `r_bytes`' own hex literal), so `r - 1`
/// is a borrow-free decrement of the last byte, and dividing by `2^32`
/// is exactly a 4-byte right shift.
const root32_exponent_bytes: [32]u8 = blk: {
    var r_minus_1 = scalar.r_bytes;
    if (r_minus_1[31] != 0x01) @compileError("bls12_381/kzg: r_bytes' low byte is expected to be 0x01");
    r_minus_1[31] = 0x00;
    var shifted = [_]u8{0} ** 32;
    @memcpy(shifted[4..32], r_minus_1[0..28]);
    break :blk shifted;
};

/// The primitive `2^32`-th root of unity in `Fr`. BLS12-381's `Fr` has
/// `2^32 | (r - 1)` (a large 2-Sylow subgroup), so this element exists;
/// every `FIELD_ELEMENTS_PER_BLOB`-sized (or smaller power-of-two)
/// root-of-unity table this module needs is derived from it by repeated
/// squaring (`computeRootsOfUnity`). DERIVED AT RUNTIME from the spec's
/// own formula — `pow(PRIMITIVE_ROOT_OF_UNITY, (BLS_MODULUS - 1) / 2^32,
/// BLS_MODULUS)` (`compute_roots_of_unity`, spec "BLS12-381 helpers",
/// specialized to the maximal power-of-two order) — with the exponent
/// comptime-derived from `scalar.r_bytes` (`root32_exponent_bytes`),
/// deliberately NOT a hardcoded 32-byte constant (the exact class of
/// magic value this module's own history warns against transcribing —
/// `g2.zig`'s `cofactor_bytes` scaffold bug, `NOTICE`). Its order is
/// verified to be EXACTLY `2^32` by this file's tests
/// (`w^(2^32) == 1` and `w^(2^31) == -1 != 1`), and the derived
/// 4096-element domain is pinned against the REAL ceremony's Lagrange
/// basis by the monomial-vs-Lagrange commitment cross-check test —
/// stronger than comparing against any transcribed reference value.
fn primitiveRootOfUnity2Pow32() Fr {
    return frFromU64(PRIMITIVE_ROOT_OF_UNITY).pow(root32_exponent_bytes);
}

/// Roots of unity of the given power-of-two `order` (spec's
/// `compute_roots_of_unity`), NOT bit-reversal-permuted (callers apply
/// `bitReversalPermutation` separately, matching the spec's own
/// `bit_reversal_permutation(compute_roots_of_unity(...))` composition at
/// every call site). Derives the order-specific root from the primitive
/// `2^32` root by repeated squaring (`root_32 ^ (2^32 / order)`, via
/// `Fr.pow`) then `computePowers`.
pub fn computeRootsOfUnity(allocator: std.mem.Allocator, order: usize) ![]Fr {
    std.debug.assert(isPowerOfTwo(order));
    // The `2^32` bound is a property of BLS12-381's Fr multiplicative group
    // order (the largest power-of-two root of unity it has), not of the host
    // pointer width — compare in `u64`, not `usize`, so the literal `32`
    // shift stays valid on a 32-bit target where `Log2Int(usize)` is `u5`.
    std.debug.assert(@as(u64, order) <= (@as(u64, 1) << 32));
    const root_32 = primitiveRootOfUnity2Pow32();
    const log2_order: u6 = std.math.log2_int(usize, order);
    const shift: u6 = 32 - log2_order;
    const exponent: u64 = @as(u64, 1) << shift;
    var exponent_bytes: [32]u8 = [_]u8{0} ** 32;
    std.mem.writeInt(u64, exponent_bytes[24..32], exponent, .big);
    const root_order = root_32.pow(exponent_bytes);
    return computePowers(allocator, root_order, order);
}

// ── FFT over Fr ─────────────────────────────────────────────────────────

/// The forward Number-Theoretic Transform over `Fr`'s `order`-th roots
/// of unity: `fft(values)[k] = sum_i values[i] * roots[(i*k) % order]`,
/// `order = values.len = roots.len` (a power of two; `roots` is
/// `computeRootsOfUnity`'s NATURAL-order table). Construction: the
/// standard iterative radix-2 Cooley-Tukey butterfly network (the
/// textbook FFT specialized to a finite field) — bit-reversal-permute
/// the input (`bitReversalPermutation`, this file), then `log2(order)`
/// rounds of `(u, t) -> (u + w*t, u - w*t)` butterflies with twiddle
/// factors read straight from `roots` — `O(n log n)` `Fr`
/// multiplications. Verified against a naive `O(n^2)` DFT and the
/// `ifft(fft(x)) == x` round-trip by this file's tests, and end-to-end
/// against the REAL ceremony by the monomial-vs-Lagrange commitment
/// cross-check test (a blob's `ifft`-recovered coefficients committed
/// against the monomial setup reproduce the Lagrange-basis commitment
/// byte-exactly). This module's own EIP-4844 API surface stays in
/// evaluation form throughout and never calls `fft`/`ifft` itself —
/// they are reusable primitives (and that cross-check's oracle).
pub fn fft(allocator: std.mem.Allocator, values: []const Fr, roots: []const Fr) ![]Fr {
    std.debug.assert(values.len == roots.len);
    std.debug.assert(isPowerOfTwo(values.len));
    const out = try bitReversalPermutation(Fr, allocator, values);
    fftButterfliesInPlace(out, roots);
    return out;
}

/// The shared butterfly network: `out` is already in bit-reversed order;
/// `roots` supplies the twiddle factors (forward or inverse table).
fn fftButterfliesInPlace(out: []Fr, roots: []const Fr) void {
    const n = out.len;
    var m: usize = 2; // current butterfly span
    while (m <= n) : (m *= 2) {
        const half = m / 2;
        const twiddle_step = n / m;
        var k: usize = 0;
        while (k < n) : (k += m) {
            for (0..half) |j| {
                const w = roots[j * twiddle_step];
                const t = w.mul(out[k + j + half]);
                const u = out[k + j];
                out[k + j] = u.add(t);
                out[k + j + half] = u.sub(t);
            }
        }
    }
}

/// Inverse of `fft`: the same butterfly network run with the inverse
/// twiddle table (`roots[i]^-1 == roots[(order - i) % order]` — no
/// per-element `Fr.inv` needed, a root of unity's inverse is another
/// root), then every output scaled by `order^-1`.
pub fn ifft(allocator: std.mem.Allocator, values: []const Fr, roots: []const Fr) ![]Fr {
    std.debug.assert(values.len == roots.len);
    std.debug.assert(isPowerOfTwo(values.len));
    const n = values.len;

    const inv_roots = try allocator.alloc(Fr, n);
    defer allocator.free(inv_roots);
    inv_roots[0] = roots[0]; // == one
    for (1..n) |i| inv_roots[i] = roots[n - i];

    const out = try bitReversalPermutation(Fr, allocator, values);
    fftButterfliesInPlace(out, inv_roots);

    const n_inv = frFromU64(@intCast(n)).inv() catch unreachable; // n != 0
    for (out) |*x| x.* = x.mul(n_inv);
    return out;
}

// ── MSM ─────────────────────────────────────────────────────────────────

/// Bucket-window width for `g1Msm`, by input size — the classic
/// Pippenger trade-off (`ceil(256/c)` windows, each costing `n` bucket
/// insertions + `~2*2^c` aggregation additions; `c ≈ log2(n) - 4` is
/// near-optimal for this range, capped at 8 so the bucket array stays a
/// small fixed stack-friendly allocation).
fn msmWindowBits(n: usize) usize {
    if (n < 4) return 2;
    if (n < 16) return 3;
    if (n < 64) return 4;
    if (n < 256) return 5;
    if (n < 1024) return 6;
    if (n < 4096) return 7;
    return 8;
}

/// Bits `[bit_off, bit_off + c)` of a 32-byte big-endian scalar, as the
/// little-endian window digit Pippenger's bucket phase consumes.
fn scalarWindowDigit(bytes: *const [32]u8, bit_off: usize, c: usize) usize {
    var digit: usize = 0;
    for (0..c) |i| {
        const b = bit_off + i;
        if (b >= 256) break;
        const bit: usize = (bytes[31 - (b >> 3)] >> @as(u3, @intCast(b & 7))) & 1;
        digit |= bit << @as(std.math.Log2Int(usize), @intCast(i));
    }
    return digit;
}

/// Multi-scalar multiplication in `G1`: `sum_i scalars[i] * points[i]`
/// (spec's `g1_lincomb`) — the operation `blobToKzgCommitment` and
/// `computeKzgProofImpl`'s quotient-commitment step both reduce to, and
/// the computational core every KZG operation bottlenecks on.
/// Construction: Pippenger's bucket method (see e.g. Bernstein's
/// "Pippenger's exponentiation algorithm" survey; every production MSM —
/// `blst`, `zkcrypto` — is a variant): split each 256-bit scalar into
/// `ceil(256/c)`-many `c`-bit windows (`msmWindowBits`); per window,
/// drop each point into the bucket indexed by its digit
/// (`jacMixedAddVartime` — the points are affine), then aggregate
/// buckets with the running-sum trick (`sum_d d * bucket[d]` in `2*(2^c
/// - 1)` additions), and fold windows most-significant-first with `c`
/// doublings between them. All arithmetic is the variable-time
/// public-data path (see the vartime section comment) — `O(n * 256/c)`
/// mixed additions total, versus naive double-and-add's `O(n * 256)`
/// full add+double pairs. `points.len == 0` returns
/// `G1.Jacobian.identity` (spec's explicit empty-input case); identity
/// points and zero scalars drop out naturally (digit-0 buckets are
/// never materialized).
pub fn g1Msm(allocator: std.mem.Allocator, points: []const g1.Affine, scalars: []const Fr) KzgError!g1.Jacobian {
    std.debug.assert(points.len == scalars.len);
    if (points.len == 0) return g1.Jacobian.identity;

    // Serialize every scalar once up front (windows re-read them
    // ceil(256/c) times).
    const scalar_bytes = try allocator.alloc([32]u8, scalars.len);
    defer allocator.free(scalar_bytes);
    for (scalar_bytes, scalars) |*bytes, s| bytes.* = s.toBytes();

    const c = msmWindowBits(points.len);
    const n_buckets = (@as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(c))) - 1; // digit 0 excluded
    const buckets = try allocator.alloc(g1.Jacobian, n_buckets);
    defer allocator.free(buckets);

    const n_windows = (256 + c - 1) / c;
    var acc = g1.Jacobian.identity;
    var w = n_windows;
    while (w > 0) {
        w -= 1;
        if (w != n_windows - 1) {
            for (0..c) |_| acc = acc.double();
        }

        @memset(buckets, g1.Jacobian.identity);
        var any = false;
        for (points, scalar_bytes) |point, *bytes| {
            const digit = scalarWindowDigit(bytes, w * c, c);
            if (digit == 0) continue;
            buckets[digit - 1] = jacMixedAddVartime(buckets[digit - 1], point);
            any = true;
        }
        if (!any) continue;

        // sum_d (d+1) * buckets[d] via the running-sum trick.
        var running = g1.Jacobian.identity;
        var window_sum = g1.Jacobian.identity;
        var d = n_buckets;
        while (d > 0) {
            d -= 1;
            running = jacAddVartime(running, buckets[d]);
            window_sum = jacAddVartime(window_sum, running);
        }
        acc = jacAddVartime(acc, window_sum);
    }
    return acc;
}

// ── polynomial evaluation ───────────────────────────────────────────────

/// Batch (Montgomery-trick) inversion, in place: replaces every element
/// with its inverse at the cost of ONE `Fr.inv` plus `3n` multiplications
/// (versus `n` full Fermat inversions). Every element MUST be nonzero
/// (asserted — all call sites in this file exclude the zero denominators
/// structurally first).
fn batchInvInPlace(allocator: std.mem.Allocator, values: []Fr) KzgError!void {
    if (values.len == 0) return;
    const prefix = try allocator.alloc(Fr, values.len);
    defer allocator.free(prefix);
    var acc = Fr.one;
    for (values, prefix) |v, *p| {
        std.debug.assert(!v.isZero());
        p.* = acc;
        acc = acc.mul(v);
    }
    var inv_acc = acc.inv() catch unreachable; // no zero factors (asserted above)
    var i = values.len;
    while (i > 0) {
        i -= 1;
        const original = values[i];
        values[i] = inv_acc.mul(prefix[i]);
        inv_acc = inv_acc.mul(original);
    }
}

/// Spec's `evaluate_polynomial_in_evaluation_form`: evaluates the
/// (evaluation-form) `polynomial` at an arbitrary `z` via the
/// barycentric formula,
/// ```
/// f(z) = (z^WIDTH - 1) / WIDTH * sum_i (f(DOMAIN[i]) * DOMAIN[i]) / (z - DOMAIN[i])
/// ```
/// over `roots_of_unity_brp` (`WIDTH = polynomial.len =
/// FIELD_ELEMENTS_PER_BLOB`), EXCEPT when `z` is itself in the domain (a
/// root of unity) — then the answer is just `polynomial[index_of(z)]`,
/// no formula needed (spec: "If we are asked to evaluate within the
/// domain, we already know the answer"; skipping this branch would
/// silently 0/0 the general formula since `z - DOMAIN[i] == 0` for that
/// term). `roots_of_unity_brp` is expected ALREADY bit-reversal-permuted
/// (`bitReversalPermutation`, this file) by the caller — matching every
/// spec call site's own
/// `bit_reversal_permutation(compute_roots_of_unity(...))` composition.
/// The denominators are inverted with `batchInvInPlace` (an allocator is
/// therefore required — a deliberate, documented deviation from the
/// scaffold's allocator-free signature; per-element `Fr.inv` would cost
/// 4096 full Fermat exponentiations per evaluation).
pub fn evaluatePolynomialInEvaluationForm(allocator: std.mem.Allocator, polynomial: []const Fr, z: Fr, roots_of_unity_brp: []const Fr) KzgError!Fr {
    const width = polynomial.len;
    std.debug.assert(width == FIELD_ELEMENTS_PER_BLOB);
    std.debug.assert(roots_of_unity_brp.len == width);

    // In-domain: indexing answers directly (and the formula below would
    // divide by zero).
    for (roots_of_unity_brp, 0..) |root, i| {
        if (root.eql(z)) return polynomial[i];
    }

    const denom_invs = try allocator.alloc(Fr, width);
    defer allocator.free(denom_invs);
    for (denom_invs, roots_of_unity_brp) |*d, root| d.* = z.sub(root); // nonzero: z not in domain
    try batchInvInPlace(allocator, denom_invs);

    var sum = Fr.zero;
    for (polynomial, roots_of_unity_brp, denom_invs) |p, root, dinv| {
        sum = sum.add(p.mul(root).mul(dinv));
    }

    const width_fr = frFromU64(@intCast(width));
    const inverse_width = width_fr.inv() catch unreachable; // width != 0
    const z_pow_width = z.pow(width_fr.toBytes());
    return sum.mul(z_pow_width.sub(Fr.one)).mul(inverse_width);
}

// ── KZG proof core ──────────────────────────────────────────────────────

const KzgProofAndY = struct { proof: KZGProof, y: Fr };

/// Spec's `compute_kzg_proof_impl`, shared by `computeKzgProof` and
/// `computeBlobKzgProof`. Construction (spec-verbatim, re-read from the
/// fetched text during this pass — `NOTICE`):
/// 1. `y = evaluatePolynomialInEvaluationForm(polynomial, z, roots_brp)`.
/// 2. For every domain point `x_i` (`roots_of_unity_brp[i]`), compute the
///    quotient polynomial's evaluation-form coefficient
///    `q(x_i) = (p(x_i) - y) / (x_i - z)` — EXCEPT when `x_i == z`
///    (denominator zero: `z` is itself a root of unity), where the spec's
///    `compute_quotient_eval_within_domain` special-cases it via
///    `q(z) = sum_{i, x_i != z} (p(x_i) - y) * x_i / (z * (z - x_i))`
///    (L'Hopital-style removable-singularity handling — see
///    dankradfeist.de/ethereum/2021/06/18/pcs-multiproofs.html "Dividing
///    when one of the points is zero", cited directly in the spec).
///    Denominators are batch-inverted (`batchInvInPlace`), with the
///    in-domain index masked out via a placeholder first; the
///    special-case sum reuses the same inverses (`1/(z - x_i) ==
///    -1/(x_i - z)`).
/// 3. `proof = g1Msm(bitReversalPermutation(setup.g1_lagrange), quotient_polynomial)`
///    (the SAME Lagrange-basis MSM `blobToKzgCommitment` uses, applied to
///    the quotient polynomial's evaluation-form coefficients instead of
///    the original blob's).
fn computeKzgProofImpl(allocator: std.mem.Allocator, polynomial: []const Fr, z: Fr, setup: *const TrustedSetup) KzgError!KzgProofAndY {
    const roots = try computeRootsOfUnity(allocator, FIELD_ELEMENTS_PER_BLOB);
    defer allocator.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, allocator, roots);
    defer allocator.free(roots_brp);

    const y = try evaluatePolynomialInEvaluationForm(allocator, polynomial, z, roots_brp);

    // denom_invs[i] = 1 / (x_i - z), with the in-domain index (if any)
    // held out of the batch inversion behind a `one` placeholder.
    var domain_index: ?usize = null;
    const denom_invs = try allocator.alloc(Fr, FIELD_ELEMENTS_PER_BLOB);
    defer allocator.free(denom_invs);
    for (denom_invs, roots_brp, 0..) |*d, root, i| {
        const diff = root.sub(z);
        if (diff.isZero()) {
            std.debug.assert(domain_index == null); // roots are distinct
            domain_index = i;
            d.* = Fr.one; // placeholder, overwritten below
        } else {
            d.* = diff;
        }
    }
    try batchInvInPlace(allocator, denom_invs);

    const quotient = try allocator.alloc(Fr, FIELD_ELEMENTS_PER_BLOB);
    defer allocator.free(quotient);
    for (quotient, polynomial, denom_invs, 0..) |*q, p, dinv, i| {
        q.* = if (domain_index == i) Fr.zero else p.sub(y).mul(dinv);
    }
    if (domain_index) |m| {
        // compute_quotient_eval_within_domain: q(z) = sum_{i != m}
        // (p_i - y) * x_i / (z * (z - x_i)); note 1/(z - x_i) ==
        // -denom_invs[i].
        const z_inv = z.inv() catch unreachable; // z is a root of unity, nonzero
        var sum = Fr.zero;
        for (polynomial, roots_brp, denom_invs, 0..) |p, root, dinv, i| {
            if (i == m) continue;
            sum = sum.add(p.sub(y).mul(root).mul(dinv.neg()).mul(z_inv));
        }
        quotient[m] = sum;
    }

    const basis = try bitReversalPermutation(g1.Affine, allocator, setup.g1_lagrange);
    defer allocator.free(basis);
    const proof_point = try g1Msm(allocator, basis, quotient);
    return .{ .proof = g1.toBytesCompressed(proof_point.toAffine()), .y = y };
}

/// Spec's `compute_kzg_proof`: public entry point — blob
/// deserialization (`blobToPolynomial`), the `z` canonical-field-element
/// check (`bytesToBlsField`), then the proof math (`computeKzgProofImpl`).
pub fn computeKzgProof(allocator: std.mem.Allocator, blob: *const Blob, z_bytes: Bytes32, setup: *const TrustedSetup) KzgError!struct { proof: KZGProof, y: Bytes32 } {
    const z = try bytesToBlsField(z_bytes);
    const polynomial = try blobToPolynomial(allocator, blob);
    defer allocator.free(polynomial);
    const result = try computeKzgProofImpl(allocator, polynomial, z, setup);
    return .{ .proof = result.proof, .y = blsFieldToBytes(result.y) };
}

/// Spec's `blob_to_kzg_commitment`: `g1_lincomb` (`g1Msm`) of the
/// bit-reversal-permuted Lagrange-basis setup points
/// (`bitReversalPermutation` — see `TrustedSetup`'s basis-order note)
/// with the blob's field elements (`blobToPolynomial`).
pub fn blobToKzgCommitment(allocator: std.mem.Allocator, blob: *const Blob, setup: *const TrustedSetup) KzgError!KZGCommitment {
    const polynomial = try blobToPolynomial(allocator, blob);
    defer allocator.free(polynomial);
    const basis = try bitReversalPermutation(g1.Affine, allocator, setup.g1_lagrange);
    defer allocator.free(basis);
    const commitment = try g1Msm(allocator, basis, polynomial);
    return g1.toBytesCompressed(commitment.toAffine());
}

// ── verify ──────────────────────────────────────────────────────────────

/// Affine `G2` negation (no Jacobian round-trip / no inversion needed).
fn g2AffineNeg(p: g2.Affine) g2.Affine {
    if (p.infinity) return p;
    return .{ .x = p.x, .y = p.y.neg(), .infinity = false };
}

/// Spec's `verify_kzg_proof_impl` — the single pairing-product check that
/// is KZG's entire "opening proof" soundness argument: `p(z) == y` iff
/// `p(X) - y = q(X) * (X - z)` for SOME polynomial `q` (the quotient,
/// which only exists as a polynomial — not merely a rational function —
/// if the division is exact, i.e. `z` really is a root of `p(X) - y`),
/// checked in the exponent via the pairing's bilinearity:
/// ```
/// e(proof, [s]_2 - [z]_2) == e(commitment - [y]_1, [1]_2)
/// ```
/// i.e., as ONE `pairing.pairingCheck` call (this module's own batched
/// pairing-product primitive, Part 2 — `e(A,B) == e(C,D)` iff
/// `e(A,B) * e(-C,D) == 1`, i.e. `pairingCheck(&.{ .{A,B}, .{-C,D} })`):
/// ```
/// X_minus_z = setup.g2_monomial[1] + (-z) * G2.generator   // [s]_2 - [z]_2
/// P_minus_y = commitment + (-y) * G1.generator             // commitment - [y]_1
/// accept iff pairingCheck(&.{
///     .{ .p = P_minus_y, .q = -G2.generator },
///     .{ .p = proof,     .q = X_minus_z },
/// })
/// ```
/// (the spec's own `[P_minus_y, -G2()], [proof, X_minus_z]` pairing list,
/// mechanically reordered into this module's `PairingPair`
/// (`p ∈ G1, q ∈ G2`) shape — see `pairing.zig`'s `pairingCheck` and
/// `bls_sig.zig`'s `verify` for this module's established
/// `pairingCheck`-over-`(-G1_generator, X)`-style idiom). The
/// scalar multiplications by `-z`/`-y` are `scalarMul(...)` followed by
/// `negate()` (`[r - x]P == -[x]P`); both are Parts 1/2's constant-time
/// primitives (only two of them per verify — not worth a vartime path).
/// Signs re-derived from the fetched spec text during this pass, not
/// assumed from the scaffold comment (they agree).
fn verifyKzgProofImpl(commitment: g1.Affine, z: Fr, y: Fr, proof: g1.Affine, setup: *const TrustedSetup) bool {
    const g2_gen = g2.Jacobian.fromAffine(g2.Affine.generator);
    const g1_gen = g1.Jacobian.fromAffine(g1.Affine.generator);

    // X_minus_z = [s]_2 - [z]_2
    const x_minus_z = g2.Jacobian.fromAffine(setup.g2_monomial[1])
        .add(g2_gen.scalarMul(z).negate())
        .toAffine();
    // P_minus_y = commitment - [y]_1
    const p_minus_y = g1.Jacobian.fromAffine(commitment)
        .add(g1_gen.scalarMul(y).negate())
        .toAffine();

    return pairing.pairingCheck(&.{
        .{ .p = p_minus_y, .q = g2AffineNeg(g2.Affine.generator) },
        .{ .p = proof, .q = x_minus_z },
    });
}

/// Spec's `verify_kzg_proof`: public entry point, bytes in. Decoding +
/// validating every input (`bytesToKzgCommitment`, `bytesToBlsField` x2,
/// `bytesToKzgProof`) is separated from the actual check
/// (`verifyKzgProofImpl`).
pub fn verifyKzgProof(commitment_bytes: Bytes48, z_bytes: Bytes32, y_bytes: Bytes32, proof_bytes: Bytes48, setup: *const TrustedSetup) KzgError!bool {
    const commitment = try bytesToKzgCommitment(commitment_bytes);
    const z = try bytesToBlsField(z_bytes);
    const y = try bytesToBlsField(y_bytes);
    const proof = try bytesToKzgProof(proof_bytes);
    return verifyKzgProofImpl(commitment, z, y, proof, setup);
}

/// Spec's `verify_kzg_proof_batch`: the batched generalization of
/// `verifyKzgProofImpl` over a random linear combination with powers of
/// a Fiat-Shamir challenge `r` — ONE pairing check for `N` proofs
/// instead of `N` separate ones, the same "share one final
/// exponentiation" payoff `pairing.pairingCheck` already gives Part
/// 2/4's own batch/aggregate callers. Construction (spec-verbatim):
/// ```
/// r = hash_to_bls_field(RANDOM_CHALLENGE_KZG_BATCH_DOMAIN
///     || be_bytes8(FIELD_ELEMENTS_PER_BLOB) || be_bytes8(N)
///     || (commitment_i || z_i || y_i || proof_i)*)          // transcript
/// r_powers           = [r^0, ..., r^(N-1)]
/// proof_lincomb      = g1Msm(proofs, r_powers)
/// proof_z_lincomb    = g1Msm(proofs, [z_i * r_powers[i]])
/// C_minus_y_lincomb  = g1Msm([commitments[i] - [ys[i]]_1], r_powers)
/// accept iff pairingCheck(&.{
///     .{ .p = proof_lincomb,                       .q = -setup.g2_monomial[1] },
///     .{ .p = C_minus_y_lincomb + proof_z_lincomb, .q = G2.generator },
/// })
/// ```
/// The transcript's commitment/proof bytes are re-serialized from the
/// already-validated points via `g1.toBytesCompressed` — byte-identical
/// to the caller's wire input, since `validateKzgG1` only admits
/// canonical compressed encodings and the codec round-trips those
/// exactly. Note the 8-byte width fields here versus `computeChallenge`'s
/// 16-byte one — the spec genuinely differs between the two transcripts
/// (re-checked against the fetched text).
fn verifyKzgProofBatchImpl(allocator: std.mem.Allocator, commitments: []const g1.Affine, zs: []const Fr, ys: []const Fr, proofs: []const g1.Affine, setup: *const TrustedSetup) KzgError!bool {
    const n = commitments.len;
    std.debug.assert(zs.len == n and ys.len == n and proofs.len == n);

    // Fiat-Shamir transcript -> r.
    var hasher = std.crypto.hash.sha2.Sha256.init(.{});
    hasher.update(RANDOM_CHALLENGE_KZG_BATCH_DOMAIN);
    var width8: [8]u8 = undefined;
    std.mem.writeInt(u64, &width8, FIELD_ELEMENTS_PER_BLOB, .big);
    hasher.update(&width8);
    var count8: [8]u8 = undefined;
    std.mem.writeInt(u64, &count8, @intCast(n), .big);
    hasher.update(&count8);
    for (commitments, zs, ys, proofs) |commitment, z, y, proof| {
        hasher.update(&g1.toBytesCompressed(commitment));
        hasher.update(&z.toBytes());
        hasher.update(&y.toBytes());
        hasher.update(&g1.toBytesCompressed(proof));
    }
    var digest: [32]u8 = undefined;
    hasher.final(&digest);
    const r = Fr.reduceWide(&digest); // spec's hash_to_bls_field

    const r_powers = try computePowers(allocator, r, n);
    defer allocator.free(r_powers);

    const proof_lincomb = try g1Msm(allocator, proofs, r_powers);

    const z_times_r = try allocator.alloc(Fr, n);
    defer allocator.free(z_times_r);
    for (z_times_r, zs, r_powers) |*zr, z, rp| zr.* = z.mul(rp);
    const proof_z_lincomb = try g1Msm(allocator, proofs, z_times_r);

    const g1_gen = g1.Jacobian.fromAffine(g1.Affine.generator);
    const c_minus_ys = try allocator.alloc(g1.Affine, n);
    defer allocator.free(c_minus_ys);
    for (c_minus_ys, commitments, ys) |*out, commitment, y| {
        out.* = g1.Jacobian.fromAffine(commitment)
            .add(g1_gen.scalarMul(y).negate())
            .toAffine();
    }
    const c_minus_y_lincomb = try g1Msm(allocator, c_minus_ys, r_powers);

    const rhs = jacAddVartime(c_minus_y_lincomb, proof_z_lincomb).toAffine();
    return pairing.pairingCheck(&.{
        .{ .p = proof_lincomb.toAffine(), .q = g2AffineNeg(setup.g2_monomial[1]) },
        .{ .p = rhs, .q = g2.Affine.generator },
    });
}

// ── Fiat-Shamir challenge ───────────────────────────────────────────────

/// Spec's `compute_challenge`: the Fiat-Shamir evaluation point `z` for
/// `computeBlobKzgProof`/`verifyBlobKzgProof(Batch)`:
/// ```
/// data = FIAT_SHAMIR_PROTOCOL_DOMAIN                      // 16 bytes
///      || be_bytes16(FIELD_ELEMENTS_PER_BLOB)             // 16 bytes
///      || blob                                            // 131072 bytes
///      || commitment                                      // 48 bytes
/// digest = Sha256(data)                                   // 32 bytes, spec's `hash`
/// return Fr.reduceWide(&digest)                           // spec's hash_to_bls_field: int(digest) mod BLS_MODULUS
/// ```
/// Pinned byte-exact (through `computeBlobKzgProof`'s output proof —
/// which depends on this challenge as its evaluation point) by the
/// `c-kzg-4844` `compute_blob_kzg_proof` KAT below.
pub fn computeChallenge(blob: *const Blob, commitment: KZGCommitment) Fr {
    var hasher = std.crypto.hash.sha2.Sha256.init(.{});
    hasher.update(FIAT_SHAMIR_PROTOCOL_DOMAIN);
    var degree_poly: [16]u8 = undefined;
    std.mem.writeInt(u128, &degree_poly, FIELD_ELEMENTS_PER_BLOB, .big);
    hasher.update(&degree_poly);
    hasher.update(blob);
    hasher.update(&commitment);
    var digest: [32]u8 = undefined;
    hasher.final(&digest);
    return Fr.reduceWide(&digest);
}

// ── blob-level public API (composes the above) ──────────────────────────

/// Spec's `compute_blob_kzg_proof`. Note (spec, verbatim): "This method
/// does not verify that the commitment is correct with respect to
/// `blob`" — `commitment_bytes` is only VALIDATED (`bytesToKzgCommitment`,
/// REAL), never recomputed/checked against `blob` here.
pub fn computeBlobKzgProof(allocator: std.mem.Allocator, blob: *const Blob, commitment_bytes: KZGCommitment, setup: *const TrustedSetup) KzgError!KZGProof {
    _ = try bytesToKzgCommitment(commitment_bytes); // validate, don't recompute (spec note above)
    const polynomial = try blobToPolynomial(allocator, blob);
    defer allocator.free(polynomial);
    const z = computeChallenge(blob, commitment_bytes);
    const result = try computeKzgProofImpl(allocator, polynomial, z, setup);
    return result.proof;
}

/// Spec's `verify_blob_kzg_proof`: decode + validate, derive the
/// challenge and evaluate the polynomial there, then the pairing check.
pub fn verifyBlobKzgProof(allocator: std.mem.Allocator, blob: *const Blob, commitment_bytes: Bytes48, proof_bytes: Bytes48, setup: *const TrustedSetup) KzgError!bool {
    const commitment = try bytesToKzgCommitment(commitment_bytes);
    const proof = try bytesToKzgProof(proof_bytes);
    const polynomial = try blobToPolynomial(allocator, blob);
    defer allocator.free(polynomial);

    const z = computeChallenge(blob, commitment_bytes);
    const roots = try computeRootsOfUnity(allocator, FIELD_ELEMENTS_PER_BLOB);
    defer allocator.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, allocator, roots);
    defer allocator.free(roots_brp);
    const y = try evaluatePolynomialInEvaluationForm(allocator, polynomial, z, roots_brp);

    return verifyKzgProofImpl(commitment, z, y, proof, setup);
}

/// Spec's `verify_blob_kzg_proof_batch`: per-blob decode/challenge/
/// evaluate (same shape as `verifyBlobKzgProof`), then ONE batched
/// pairing check (`verifyKzgProofBatchImpl`). Spec: "Will return True if
/// there are zero blobs" — handled explicitly, matching
/// `pairing.pairingCheck(&.{})`'s own vacuous-true convention for an
/// empty pair list elsewhere in this module.
pub fn verifyBlobKzgProofBatch(allocator: std.mem.Allocator, blobs: []const Blob, commitments_bytes: []const Bytes48, proofs_bytes: []const Bytes48, setup: *const TrustedSetup) KzgError!bool {
    if (blobs.len != commitments_bytes.len or blobs.len != proofs_bytes.len) return error.LengthMismatch;
    if (blobs.len == 0) return true; // spec's explicit vacuous-true case

    const commitments = try allocator.alloc(g1.Affine, blobs.len);
    defer allocator.free(commitments);
    const zs = try allocator.alloc(Fr, blobs.len);
    defer allocator.free(zs);
    const ys = try allocator.alloc(Fr, blobs.len);
    defer allocator.free(ys);
    const proofs = try allocator.alloc(g1.Affine, blobs.len);
    defer allocator.free(proofs);

    const roots = try computeRootsOfUnity(allocator, FIELD_ELEMENTS_PER_BLOB);
    defer allocator.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, allocator, roots);
    defer allocator.free(roots_brp);

    for (0..blobs.len) |i| {
        const blob = &blobs[i];
        commitments[i] = try bytesToKzgCommitment(commitments_bytes[i]);
        proofs[i] = try bytesToKzgProof(proofs_bytes[i]);
        const polynomial = try blobToPolynomial(allocator, blob);
        defer allocator.free(polynomial);
        zs[i] = computeChallenge(blob, commitments_bytes[i]);
        ys[i] = try evaluatePolynomialInEvaluationForm(allocator, polynomial, zs[i], roots_brp);
    }

    return verifyKzgProofBatchImpl(allocator, commitments, zs, ys, proofs, setup);
}

// ── tests ────────────────────────────────────────────────────────────────

test "constants match the spec's own values" {
    try std.testing.expectEqual(@as(usize, 4096), FIELD_ELEMENTS_PER_BLOB);
    try std.testing.expectEqual(@as(usize, 32), BYTES_PER_FIELD_ELEMENT);
    try std.testing.expectEqual(@as(usize, 131072), BYTES_PER_BLOB);
    try std.testing.expectEqual(@as(usize, 48), BYTES_PER_COMMITMENT);
    try std.testing.expectEqual(@as(usize, 48), BYTES_PER_PROOF);
    try std.testing.expectEqual(@as(usize, 65), NUM_G2_POINTS);
    try std.testing.expectEqual(@as(u64, 7), PRIMITIVE_ROOT_OF_UNITY);
    try std.testing.expectEqualStrings("FSBLOBVERIFY_V1_", FIAT_SHAMIR_PROTOCOL_DOMAIN);
    try std.testing.expectEqualStrings("RCKZGBATCH___V1_", RANDOM_CHALLENGE_KZG_BATCH_DOMAIN);
    try std.testing.expectEqual(@as(usize, 48), g1.compressed_bytes);
    try std.testing.expectEqual(@as(usize, 96), g2.compressed_bytes);
}

test "BLS_MODULUS matches the spec's decimal constant (independently converted)" {
    // 52435875175126190479447740508185965837690552500527637822603658699938581184513
    // converted to hex independently (see NOTICE) == scalar.r_bytes.
    var expected: [32]u8 = undefined;
    _ = try std.fmt.hexToBytes(&expected, "73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001");
    try std.testing.expectEqualSlices(u8, &expected, &BLS_MODULUS);
}

test "G1_POINT_AT_INFINITY matches g1.zig's own compressed-identity encoding" {
    try std.testing.expectEqualSlices(u8, &g1.toBytesCompressed(g1.Affine.identity), &G1_POINT_AT_INFINITY);
}

test "isPowerOfTwo" {
    try std.testing.expect(isPowerOfTwo(1));
    try std.testing.expect(isPowerOfTwo(4096));
    try std.testing.expect(!isPowerOfTwo(0));
    try std.testing.expect(!isPowerOfTwo(3));
    try std.testing.expect(!isPowerOfTwo(4095));
}

test "reverseBits matches the spec's own worked convention (order=8)" {
    // order=8 -> 3 bits: 1 (0b001) reverses to 4 (0b100), 3 (0b011) -> 6 (0b110).
    try std.testing.expectEqual(@as(usize, 0), reverseBits(0, 8));
    try std.testing.expectEqual(@as(usize, 4), reverseBits(1, 8));
    try std.testing.expectEqual(@as(usize, 6), reverseBits(3, 8));
    try std.testing.expectEqual(@as(usize, 7), reverseBits(7, 8));
}

test "bitReversalPermutation is an involution" {
    var seq: [8]u32 = .{ 10, 11, 12, 13, 14, 15, 16, 17 };
    const once = try bitReversalPermutation(u32, std.testing.allocator, &seq);
    defer std.testing.allocator.free(once);
    const twice = try bitReversalPermutation(u32, std.testing.allocator, once);
    defer std.testing.allocator.free(twice);
    try std.testing.expectEqualSlices(u32, &seq, twice);
    // And it actually permutes (not the identity) for this order.
    try std.testing.expect(!std.mem.eql(u32, &seq, once));
}

test "computePowers: x^0..x^(n-1), n=0 empty" {
    const two = Fr.one.add(Fr.one);
    const powers = try computePowers(std.testing.allocator, two, 5);
    defer std.testing.allocator.free(powers);
    try std.testing.expect(powers[0].eql(Fr.one));
    try std.testing.expect(powers[1].eql(two));
    try std.testing.expect(powers[4].eql(two.mul(two).mul(two).mul(two)));

    const empty = try computePowers(std.testing.allocator, two, 0);
    defer std.testing.allocator.free(empty);
    try std.testing.expectEqual(@as(usize, 0), empty.len);
}

test "loadTrustedSetup: shape, and g1/g2 monomial[0] are this module's own generators" {
    var setup = try loadTrustedSetup(std.testing.allocator);
    defer setup.deinit(std.testing.allocator);

    try std.testing.expectEqual(FIELD_ELEMENTS_PER_BLOB, setup.g1_lagrange.len);
    try std.testing.expectEqual(FIELD_ELEMENTS_PER_BLOB, setup.g1_monomial.len);
    try std.testing.expectEqual(NUM_G2_POINTS, setup.g2_monomial.len);

    // [s^0] G1 = G1 generator, [s^0] G2 = G2 generator — the defining
    // monomial-basis identity, and a strong structural sanity check that
    // parsing landed the right points in the right slots.
    try std.testing.expect(setup.g1_monomial[0].x.eql(g1.Affine.generator.x));
    try std.testing.expect(setup.g1_monomial[0].y.eql(g1.Affine.generator.y));
    try std.testing.expect(setup.g2_monomial[0].x.eql(g2.Affine.generator.x));
    try std.testing.expect(setup.g2_monomial[0].y.eql(g2.Affine.generator.y));

    // Every loaded point already passed subgroupCheck inside
    // loadTrustedSetup (or loading would have errored) — re-assert on a
    // couple of spot samples as a regression guard on that invariant.
    try std.testing.expect(g1.Jacobian.fromAffine(setup.g1_lagrange[0]).subgroupCheck());
    try std.testing.expect(g2.Jacobian.fromAffine(setup.g2_monomial[1]).subgroupCheck());
}

var test_setup_cache: ?TrustedSetup = null;

/// A SHARED, lazily-loaded `TrustedSetup` for every KAT test below.
/// `loadTrustedSetup` itself is already memoized process-wide (see its
/// doc comment) — this fixture just avoids even the per-call deep copy
/// for the many tests that only need a read-only setup. Uses
/// `std.heap.page_allocator` (not `std.testing.allocator`) and is
/// intentionally never `deinit`'d — a process-lifetime read-only test
/// fixture, not a per-test allocation `std.testing.allocator`'s leak
/// checker should track.
fn testSetup() *const TrustedSetup {
    if (test_setup_cache == null) {
        test_setup_cache = loadTrustedSetup(std.heap.page_allocator) catch // global-alloc-ok: process-lifetime test fixture, outlives testing.allocator's per-test teardown (see doc comment above)
            @panic("failed to load the embedded trusted setup for tests");
    }
    return &test_setup_cache.?;
}

// ── EIP-4844 KAT vectors ────────────────────────────────────────────────
//
// Source: `ethereum/c-kzg-4844` `tests/` (tag: `main` branch, fetched
// 2026-07-14 — see NOTICE for the exact paths and sha256 of every fetched
// file). All wired vectors below share ONE embedded blob (`data/
// kzg_test_vectors/blob_constant_2.hex`): the constant polynomial
// `p(x) = 2` (every one of its 4096 evaluation-form coefficients is the
// field element `2`) — chosen because it recurs, byte-identically,
// across FOUR of `c-kzg-4844`'s own test categories (confirmed by this
// pass: `blob_to_kzg_commitment_case_valid_blob_1`, `compute_kzg_proof_
// case_valid_blob_1_0`, `verify_kzg_proof_case_correct_proof_1_0`,
// `verify_blob_kzg_proof_case_correct_proof_1` all reference this exact
// blob), so ONE 131072-byte embed exercises four KAT categories instead
// of four. Its commitment (`[2] * G1` — the constant polynomial's
// commitment is the scalar `2` times the sum of ALL Lagrange basis
// points, which is exactly `[2] * G1_generator` since the Lagrange basis
// sums to the generator) independently equals `g1.zig`'s own
// `two_g_compressed_hex` cross-check KAT (`a572cbea90...`, computed there
// by a wholly different, non-KZG method — see that file's NOTICE
// entry) — a nice unplanned cross-module confirmation, checked explicitly
// below.
const test_blob_constant_2_hex = @embedFile("data/kzg_test_vectors/blob_constant_2.hex");

fn loadTestBlobConstant2() Blob {
    var blob: Blob = undefined;
    _ = std.fmt.hexToBytes(&blob, test_blob_constant_2_hex) catch unreachable;
    return blob;
}

fn hexBytesN(comptime n: usize, hex: []const u8) [n]u8 {
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

// commitment = a572cbea... — ALSO g1.zig's own independently-sourced
// "[2]G compressed" cross-check KAT (`two_g_compressed_hex`, computed
// there via a from-scratch Python affine-coordinates implementation, NOT
// via KZG) — see the doc comment above.
const commitment_constant_2_hex = "a572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e";

test "KAT source cross-check: the shared test blob's commitment matches g1.zig's independent [2]G KAT" {
    // This does not exercise blobToKzgCommitment (its own KAT below does)
    // — it only pins the c-kzg-4844-sourced commitment constant itself
    // against this module's own unrelated, already-verified [2]G value,
    // confirming the fetched vector is internally consistent with this
    // module's own curve arithmetic before any KZG code ever runs it.
    const expected = hexBytesN(48, commitment_constant_2_hex);
    const two_g = g1.Jacobian.fromAffine(g1.Affine.generator).double().toAffine();
    try std.testing.expectEqualSlices(u8, &expected, &g1.toBytesCompressed(two_g));
}

// A second, NON-constant shared test blob: `c-kzg-4844`'s "random" blob
// #4, added by the crypto-core pass because the constant-2 blob is
// PERMUTATION-INVARIANT — a bit-reversal-permutation or root-of-unity
// error is invisible against it (every Lagrange coefficient is the same
// scalar), while this blob's 4096 distinct pseudorandom field elements
// pin the exact basis ordering. Fetched 2026-07-14 from
// ethereum/c-kzg-4844 tests/blob_to_kzg_commitment/kzg-mainnet/
// blob_to_kzg_commitment_case_valid_blob_4/data.yaml (see NOTICE for the
// sha256s); the SAME blob recurs byte-identically across the
// compute_kzg_proof/verify_kzg_proof/compute_blob_kzg_proof/
// verify_blob_kzg_proof `_4`-suffixed cases (confirmed during the
// fetch), so — like the constant-2 blob — ONE embed serves five KAT
// categories.
const test_blob_random_4_hex = @embedFile("data/kzg_test_vectors/blob_random_4.hex");

fn loadTestBlobRandom4() Blob {
    var blob: Blob = undefined;
    _ = std.fmt.hexToBytes(&blob, test_blob_random_4_hex) catch unreachable;
    return blob;
}

// blob #4's commitment — ethereum/c-kzg-4844 tests/blob_to_kzg_commitment/
// kzg-mainnet/blob_to_kzg_commitment_case_valid_blob_4/data.yaml `output`.
const commitment_random_4_hex = "8f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7";

// blob_to_kzg_commitment KATs.
// Sources: ethereum/c-kzg-4844 tests/blob_to_kzg_commitment/kzg-mainnet/
//          blob_to_kzg_commitment_case_valid_blob_1/data.yaml (constant blob)
//          blob_to_kzg_commitment_case_valid_blob_4/data.yaml (random blob)
test "KAT: blobToKzgCommitment — c-kzg-4844 valid_blob_1 (constant) + valid_blob_4 (random) byte-exact" {
    const setup = testSetup();

    const blob_const = loadTestBlobConstant2();
    const expected_const = hexBytesN(48, commitment_constant_2_hex);
    const commitment_const = try blobToKzgCommitment(std.testing.allocator, &blob_const, setup);
    try std.testing.expectEqualSlices(u8, &expected_const, &commitment_const);

    const blob_random = loadTestBlobRandom4();
    const expected_random = hexBytesN(48, commitment_random_4_hex);
    const commitment_random = try blobToKzgCommitment(std.testing.allocator, &blob_random, setup);
    try std.testing.expectEqualSlices(u8, &expected_random, &commitment_random);
}

// compute_kzg_proof KAT (constant blob).
// Source: ethereum/c-kzg-4844 tests/compute_kzg_proof/kzg-mainnet/
//         compute_kzg_proof_case_valid_blob_1_0/data.yaml
// z = 0, y = 2 (the constant polynomial evaluates to 2 everywhere),
// proof = G1_POINT_AT_INFINITY (the quotient polynomial is identically
// zero: p(x) - y = 0 for every x since p is the constant 2).
test "KAT: computeKzgProof — compute_kzg_proof_case_valid_blob_1_0 byte-exact" {
    const setup = testSetup();
    const blob = loadTestBlobConstant2();
    const z_bytes: Bytes32 = [_]u8{0} ** 32;
    const expected_y: Bytes32 = blk: {
        var b = [_]u8{0} ** 32;
        b[31] = 2;
        break :blk b;
    };
    const expected_proof = G1_POINT_AT_INFINITY;

    const result = try computeKzgProof(std.testing.allocator, &blob, z_bytes, setup);
    try std.testing.expectEqualSlices(u8, &expected_proof, &result.proof);
    try std.testing.expectEqualSlices(u8, &expected_y, &result.y);
}

// compute_kzg_proof KATs (random blob, two evaluation points — one tiny
// `z = 2`, one full-width pseudorandom `z`).
// Sources: ethereum/c-kzg-4844 tests/compute_kzg_proof/kzg-mainnet/
//          compute_kzg_proof_case_valid_blob_4_2/data.yaml
//          compute_kzg_proof_case_valid_blob_4_5/data.yaml
test "KAT: computeKzgProof — compute_kzg_proof_case_valid_blob_4_{2,5} byte-exact (random blob)" {
    const setup = testSetup();
    const blob = loadTestBlobRandom4();

    { // case _4_2: z = 2
        var z_bytes: Bytes32 = [_]u8{0} ** 32;
        z_bytes[31] = 2;
        const expected_proof = hexBytesN(48, "a35c4f136a09a33c6437c26dc0c617ce6548a14bc4af7127690a411f5e1cde2f73157365212dbcea6432e0e7869cb006");
        const expected_y = hexBytesN(32, "549345dd3612e36fab0ab7baffe3faa5b820d56b71348c89ecaf63f7c4f85370");
        const result = try computeKzgProof(std.testing.allocator, &blob, z_bytes, setup);
        try std.testing.expectEqualSlices(u8, &expected_proof, &result.proof);
        try std.testing.expectEqualSlices(u8, &expected_y, &result.y);
    }
    { // case _4_5: full-width z
        const z_bytes = hexBytesN(32, "564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d36306");
        const expected_proof = hexBytesN(48, "873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a");
        const expected_y = hexBytesN(32, "24d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a1");
        const result = try computeKzgProof(std.testing.allocator, &blob, z_bytes, setup);
        try std.testing.expectEqualSlices(u8, &expected_proof, &result.proof);
        try std.testing.expectEqualSlices(u8, &expected_y, &result.y);
    }
}

// verify_kzg_proof KAT (accept case) — this one needs NO blob (verify
// takes only commitment/z/y/proof), so it is fully self-contained.
// Source: ethereum/c-kzg-4844 tests/verify_kzg_proof/kzg-mainnet/
//         verify_kzg_proof_case_correct_proof_1_0/data.yaml
test "KAT: verifyKzgProof accept — verify_kzg_proof_case_correct_proof_1_0" {
    const setup = testSetup();
    const commitment_bytes = hexBytesN(48, commitment_constant_2_hex);
    const z_bytes: Bytes32 = [_]u8{0} ** 32;
    const y_bytes: Bytes32 = blk: {
        var b = [_]u8{0} ** 32;
        b[31] = 2;
        break :blk b;
    };
    const proof_bytes = G1_POINT_AT_INFINITY;

    try std.testing.expect(try verifyKzgProof(commitment_bytes, z_bytes, y_bytes, proof_bytes, setup));
}

// verify_kzg_proof KATs for the random blob: the correct proof accepts,
// a wrong (but valid-subgroup) proof rejects.
// Sources: ethereum/c-kzg-4844 tests/verify_kzg_proof/kzg-mainnet/
//          verify_kzg_proof_case_correct_proof_4_2/data.yaml (output: true)
//          verify_kzg_proof_case_incorrect_proof_4_2/data.yaml (output: false)
test "KAT: verifyKzgProof — verify_kzg_proof_case_{correct,incorrect}_proof_4_2 (random blob)" {
    const setup = testSetup();
    const commitment_bytes = hexBytesN(48, commitment_random_4_hex);
    var z_bytes: Bytes32 = [_]u8{0} ** 32;
    z_bytes[31] = 2;
    const y_bytes = hexBytesN(32, "549345dd3612e36fab0ab7baffe3faa5b820d56b71348c89ecaf63f7c4f85370");

    const correct_proof = hexBytesN(48, "a35c4f136a09a33c6437c26dc0c617ce6548a14bc4af7127690a411f5e1cde2f73157365212dbcea6432e0e7869cb006");
    try std.testing.expect(try verifyKzgProof(commitment_bytes, z_bytes, y_bytes, correct_proof, setup));

    const incorrect_proof = hexBytesN(48, "94fce36bf7e9f0ed981728fcd829013de96f7d25f8b4fe885059ec24af36f801ffbf68ec4604ef6e5f5f800f5cf31238");
    try std.testing.expect(!(try verifyKzgProof(commitment_bytes, z_bytes, y_bytes, incorrect_proof, setup)));
}

// verify_kzg_proof KAT (reject case): commitment = identity (the
// all-zero blob's commitment), z=0, y=0 (correct for THAT commitment),
// but proof = the G1 generator (WRONG — the correct proof for the
// all-zero polynomial is also the identity, same reasoning as the
// constant-2 case above) -> output false.
// Source: ethereum/c-kzg-4844 tests/verify_kzg_proof/kzg-mainnet/
//         verify_kzg_proof_case_incorrect_proof_0_0/data.yaml
test "KAT: verifyKzgProof reject — verify_kzg_proof_case_incorrect_proof_0_0" {
    const setup = testSetup();
    const commitment_bytes = G1_POINT_AT_INFINITY;
    const z_bytes: Bytes32 = [_]u8{0} ** 32;
    const y_bytes: Bytes32 = [_]u8{0} ** 32;
    const proof_bytes = hexBytesN(48, "97f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb");

    try std.testing.expect(!(try verifyKzgProof(commitment_bytes, z_bytes, y_bytes, proof_bytes, setup)));
}

// compute_blob_kzg_proof KATs — these pin `computeChallenge` (the
// Fiat-Shamir transcript) byte-exactly: the output proof is an opening
// at the challenge point, so ANY transcript deviation changes it.
// Sources: ethereum/c-kzg-4844 tests/compute_blob_kzg_proof/kzg-mainnet/
//          compute_blob_kzg_proof_case_valid_blob_1/data.yaml
//            (constant blob -> proof = G1_POINT_AT_INFINITY: the quotient
//             polynomial is identically zero REGARDLESS of the challenge)
//          compute_blob_kzg_proof_case_valid_blob_4/data.yaml
//            (random blob -> a nontrivial, challenge-dependent proof)
test "KAT: computeBlobKzgProof — compute_blob_kzg_proof_case_valid_blob_{1,4} byte-exact" {
    const setup = testSetup();

    const blob_const = loadTestBlobConstant2();
    const commitment_const = hexBytesN(48, commitment_constant_2_hex);
    const proof_const = try computeBlobKzgProof(std.testing.allocator, &blob_const, commitment_const, setup);
    try std.testing.expectEqualSlices(u8, &G1_POINT_AT_INFINITY, &proof_const);

    const blob_random = loadTestBlobRandom4();
    const commitment_random = hexBytesN(48, commitment_random_4_hex);
    const expected_proof = hexBytesN(48, "8a9953b9de21f91395b66705990d222ce4e6a692f94a32b0ed0648df735e87d686dfe608a7acbdc605180540b55f7272");
    const proof_random = try computeBlobKzgProof(std.testing.allocator, &blob_random, commitment_random, setup);
    try std.testing.expectEqualSlices(u8, &expected_proof, &proof_random);
}

// verify_blob_kzg_proof KAT (accept case).
// Source: ethereum/c-kzg-4844 tests/verify_blob_kzg_proof/kzg-mainnet/
//         verify_blob_kzg_proof_case_correct_proof_1/data.yaml
test "KAT: verifyBlobKzgProof accept — verify_blob_kzg_proof_case_correct_proof_1" {
    const setup = testSetup();
    const blob = loadTestBlobConstant2();
    const commitment_bytes = hexBytesN(48, commitment_constant_2_hex);
    const proof_bytes = G1_POINT_AT_INFINITY;

    try std.testing.expect(try verifyBlobKzgProof(std.testing.allocator, &blob, commitment_bytes, proof_bytes, setup));
}

// verify_blob_kzg_proof KAT (random blob, accept case).
// Source: ethereum/c-kzg-4844 tests/verify_blob_kzg_proof/kzg-mainnet/
//         verify_blob_kzg_proof_case_correct_proof_4/data.yaml (output: true)
test "KAT: verifyBlobKzgProof accept — verify_blob_kzg_proof_case_correct_proof_4 (random blob)" {
    const setup = testSetup();
    const blob = loadTestBlobRandom4();
    const commitment_bytes = hexBytesN(48, commitment_random_4_hex);
    const proof_bytes = hexBytesN(48, "8a9953b9de21f91395b66705990d222ce4e6a692f94a32b0ed0648df735e87d686dfe608a7acbdc605180540b55f7272");

    try std.testing.expect(try verifyBlobKzgProof(std.testing.allocator, &blob, commitment_bytes, proof_bytes, setup));
}

// verify_blob_kzg_proof KAT (reject case): same blob/commitment, but
// proof = the G1 generator (wrong) -> output false.
// Source: ethereum/c-kzg-4844 tests/verify_blob_kzg_proof/kzg-mainnet/
//         verify_blob_kzg_proof_case_incorrect_proof_1/data.yaml
test "KAT: verifyBlobKzgProof reject — verify_blob_kzg_proof_case_incorrect_proof_1" {
    const setup = testSetup();
    const blob = loadTestBlobConstant2();
    const commitment_bytes = hexBytesN(48, commitment_constant_2_hex);
    const proof_bytes = hexBytesN(48, "97f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb");

    try std.testing.expect(!(try verifyBlobKzgProof(std.testing.allocator, &blob, commitment_bytes, proof_bytes, setup)));
}

// Batch verification over BOTH shared KAT blobs (each blob+commitment+
// proof tuple individually pinned byte-exact against c-kzg-4844 above);
// then the same batch with the two proofs SWAPPED must reject —
// exercising the random-linear-combination pairing check's soundness on
// per-entry-valid-subgroup but mismatched inputs.
test "verifyBlobKzgProofBatch: two-blob batch of pinned KAT tuples accepts; swapped proofs reject" {
    const setup = testSetup();
    const blobs = [_]Blob{ loadTestBlobConstant2(), loadTestBlobRandom4() };
    const commitments = [_]Bytes48{
        hexBytesN(48, commitment_constant_2_hex),
        hexBytesN(48, commitment_random_4_hex),
    };
    const proofs = [_]Bytes48{
        G1_POINT_AT_INFINITY,
        hexBytesN(48, "8a9953b9de21f91395b66705990d222ce4e6a692f94a32b0ed0648df735e87d686dfe608a7acbdc605180540b55f7272"),
    };
    try std.testing.expect(try verifyBlobKzgProofBatch(std.testing.allocator, &blobs, &commitments, &proofs, setup));

    const swapped = [_]Bytes48{ proofs[1], proofs[0] };
    try std.testing.expect(!(try verifyBlobKzgProofBatch(std.testing.allocator, &blobs, &commitments, &swapped, setup)));
}

test "verifyBlobKzgProofBatch: empty input returns true (spec's explicit vacuous case)" {
    const setup = testSetup();
    const result = try verifyBlobKzgProofBatch(std.testing.allocator, &.{}, &.{}, &.{}, setup);
    try std.testing.expect(result);
}

test "verifyBlobKzgProofBatch: length mismatch is rejected" {
    const setup = testSetup();
    const blob = loadTestBlobConstant2();
    try std.testing.expectError(error.LengthMismatch, verifyBlobKzgProofBatch(
        std.testing.allocator,
        &.{blob},
        &.{},
        &.{G1_POINT_AT_INFINITY},
        setup,
    ));
}

test "bytesToBlsField / blsFieldToBytes: canonical round-trip, non-canonical rejected" {
    var b = [_]u8{0} ** 32;
    b[31] = 7;
    const fe = try bytesToBlsField(b);
    try std.testing.expectEqualSlices(u8, &b, &blsFieldToBytes(fe));
    try std.testing.expectError(error.InvalidFieldElement, bytesToBlsField(BLS_MODULUS));
}

test "validateKzgG1: identity allowed, generator allowed, non-subgroup point rejected" {
    try validateKzgG1(G1_POINT_AT_INFINITY);
    try validateKzgG1(g1.toBytesCompressed(g1.Affine.generator));

    // x = 4: on-curve but NOT in the subgroup (same non-subgroup point
    // g1.zig's own subgroupCheck test uses — see that file's NOTICE-cited
    // independent verification).
    var comp = [_]u8{0} ** 48;
    comp[0] = 0x80;
    comp[47] = 4;
    try std.testing.expectError(error.PointNotInSubgroup, validateKzgG1(comp));
}

test "blobToPolynomial: canonical blob round-trips, non-canonical element rejected" {
    var blob: Blob = [_]u8{0} ** BYTES_PER_BLOB;
    blob[31] = 5; // element 0 = 5
    blob[63] = 9; // element 1 = 9
    const poly = try blobToPolynomial(std.testing.allocator, &blob);
    defer std.testing.allocator.free(poly);
    try std.testing.expectEqual(@as(usize, FIELD_ELEMENTS_PER_BLOB), poly.len);
    var five = [_]u8{0} ** 32;
    five[31] = 5;
    try std.testing.expect(poly[0].eql(try Fr.fromBytes(five)));

    var bad_blob: Blob = [_]u8{0} ** BYTES_PER_BLOB;
    bad_blob[0..32].* = BLS_MODULUS; // element 0 == r itself: non-canonical
    try std.testing.expectError(error.InvalidFieldElement, blobToPolynomial(std.testing.allocator, &bad_blob));
}

// ── property tests (crypto-core pass) ───────────────────────────────────

fn affineEql(a: g1.Affine, b: g1.Affine) bool {
    if (a.infinity or b.infinity) return a.infinity == b.infinity;
    return a.x.eql(b.x) and a.y.eql(b.y);
}

test "vartime G1 add/mixed-add/scalar-mul match the constant-time versions on random + every degenerate case" {
    const gen = g1.Jacobian.fromAffine(g1.Affine.generator);
    const p = gen.scalarMul(frFromU64(123456789)); // "random" subgroup point
    const q = gen.scalarMul(frFromU64(987654321));
    const idn = g1.Jacobian.identity;

    const cases = [_][2]g1.Jacobian{
        .{ p, q }, // general
        .{ p, p }, // P == Q (doubling case)
        .{ p, p.negate() }, // P == -Q (identity result)
        .{ idn, p }, // identity left
        .{ p, idn }, // identity right
        .{ idn, idn }, // both identity
    };
    for (cases) |case| {
        const expected = case[0].add(case[1]).toAffine();
        try std.testing.expect(affineEql(expected, jacAddVartime(case[0], case[1]).toAffine()));
        try std.testing.expect(affineEql(expected, jacMixedAddVartime(case[0], case[1].toAffine()).toAffine()));
    }

    // Vartime scalar mul (and hence subgroupCheckVartime) vs the
    // constant-time engine, incl. multiplying by the group order r.
    const s = frFromU64(0xdeadbeefcafef00d);
    try std.testing.expect(affineEql(p.scalarMul(s).toAffine(), jacScalarMulVartime(p, &s.toBytes()).toAffine()));
    try std.testing.expect(jacScalarMulVartime(p, &scalar.r_bytes).isIdentity());
    try std.testing.expectEqual(p.subgroupCheck(), subgroupCheckVartime(p));
}

test "g1Msm: empty input, zero scalars, identity points, and agreement with naive per-point scalar-mul" {
    const gpa = std.testing.allocator;
    const gen = g1.Jacobian.fromAffine(g1.Affine.generator);

    // Empty -> identity (spec's explicit case).
    try std.testing.expect((try g1Msm(gpa, &.{}, &.{})).isIdentity());

    // [3]G + [1]2G + [7]5G == [40]G, with a zero-scalar and an
    // identity-point entry mixed in (both must drop out).
    const points = [_]g1.Affine{
        g1.Affine.generator,
        gen.scalarMul(frFromU64(2)).toAffine(),
        gen.scalarMul(frFromU64(5)).toAffine(),
        g1.Affine.generator, // scalar 0
        g1.Affine.identity, // identity point
    };
    const scalars = [_]Fr{ frFromU64(3), frFromU64(1), frFromU64(7), Fr.zero, frFromU64(11) };
    const result = (try g1Msm(gpa, &points, &scalars)).toAffine();
    try std.testing.expect(affineEql(gen.scalarMul(frFromU64(40)).toAffine(), result));

    // All-zero scalars -> identity.
    const zeros = [_]Fr{ Fr.zero, Fr.zero };
    try std.testing.expect((try g1Msm(gpa, points[0..2], &zeros)).isIdentity());

    // A full-width (non-u64) scalar, against the constant-time engine.
    const wide = Fr.reduceWide("g1Msm full-width scalar test vector seed");
    const single = (try g1Msm(gpa, points[0..1], &.{wide})).toAffine();
    try std.testing.expect(affineEql(gen.scalarMul(wide).toAffine(), single));
}

test "roots of unity: the derived 2^32 root has order exactly 2^32; the 4096 domain is primitive and complete" {
    const gpa = std.testing.allocator;
    const w32 = primitiveRootOfUnity2Pow32();

    // w32^(2^32) == 1 and w32^(2^31) == -1 (so the order is exactly
    // 2^32, not a proper divisor).
    var exp_2_32 = [_]u8{0} ** 32;
    exp_2_32[27] = 0x01; // 2^32
    try std.testing.expect(w32.pow(exp_2_32).eql(Fr.one));
    var exp_2_31 = [_]u8{0} ** 32;
    exp_2_31[28] = 0x80; // 2^31
    const minus_one = Fr.zero.sub(Fr.one);
    try std.testing.expect(w32.pow(exp_2_31).eql(minus_one));

    // The 4096-element domain: starts at 1, w^4096 == 1, w^2048 == -1,
    // and no root other than index 0 equals 1 (primitivity => all 4096
    // are distinct, since w^i == w^j iff w^(i-j) == 1).
    const roots = try computeRootsOfUnity(gpa, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(roots);
    try std.testing.expect(roots[0].eql(Fr.one));
    const w = roots[1];
    try std.testing.expect(w.pow(frFromU64(4096).toBytes()).eql(Fr.one));
    try std.testing.expect(roots[2048].eql(minus_one));
    for (roots[1..]) |root| try std.testing.expect(!root.eql(Fr.one));
    // The table really is the powers of w.
    try std.testing.expect(roots[2].eql(w.mul(w)));
    try std.testing.expect(roots[4095].mul(w).eql(Fr.one));
}

test "fft: matches the naive O(n^2) DFT (n=8); ifft is its exact inverse" {
    const gpa = std.testing.allocator;
    const n = 8;
    const roots = try computeRootsOfUnity(gpa, n);
    defer gpa.free(roots);

    var values: [n]Fr = undefined;
    for (&values, 0..) |*v, i| v.* = frFromU64(@as(u64, @intCast(i)) * 1000 + 7);

    const transformed = try fft(gpa, &values, roots);
    defer gpa.free(transformed);

    // Naive DFT: X[k] = sum_i values[i] * roots[(i*k) % n].
    for (0..n) |k| {
        var expected = Fr.zero;
        for (values, 0..) |v, i| expected = expected.add(v.mul(roots[(i * k) % n]));
        try std.testing.expect(expected.eql(transformed[k]));
    }

    const back = try ifft(gpa, transformed, roots);
    defer gpa.free(back);
    for (values, back) |orig, round_tripped| try std.testing.expect(orig.eql(round_tripped));
}

// The strongest single check in this file: recover blob #4's COEFFICIENT
// form via ifft and commit it against the MONOMIAL setup — the result
// must equal the Lagrange-basis commitment (the c-kzg-4844 KAT value).
// This cross-validates, jointly and against the REAL ceremony: the
// primitive root of unity, the domain convention (blob element i is the
// evaluation at roots_brp[i]), the bit-reversal permutation, the ifft,
// and the MSM over both setup bases.
test "cross-check: ifft-recovered coefficients committed against the monomial setup reproduce blob #4's Lagrange commitment" {
    const gpa = std.testing.allocator;
    const setup = testSetup();
    const blob = loadTestBlobRandom4();

    const poly_brp = try blobToPolynomial(gpa, &blob); // evaluations, brp order
    defer gpa.free(poly_brp);
    const evals_natural = try bitReversalPermutation(Fr, gpa, poly_brp); // brp is an involution
    defer gpa.free(evals_natural);

    const roots = try computeRootsOfUnity(gpa, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(roots);
    const coeffs = try ifft(gpa, evals_natural, roots);
    defer gpa.free(coeffs);

    const commitment = try g1Msm(gpa, setup.g1_monomial, coeffs);
    const expected = hexBytesN(48, commitment_random_4_hex);
    try std.testing.expectEqualSlices(u8, &expected, &g1.toBytesCompressed(commitment.toAffine()));
}

test "batchInvInPlace: every element inverted (x * x^-1 == 1)" {
    const gpa = std.testing.allocator;
    var values = [_]Fr{ frFromU64(2), frFromU64(3), frFromU64(999999937), Fr.one, frFromU64(0xffff_ffff_ffff_ffff) };
    const originals = values;
    try batchInvInPlace(gpa, &values);
    for (originals, values) |orig, inv| try std.testing.expect(orig.mul(inv).eql(Fr.one));
}

test "evaluatePolynomialInEvaluationForm: linear polynomial evaluates exactly; in-domain z indexes directly" {
    const gpa = std.testing.allocator;
    const roots = try computeRootsOfUnity(gpa, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, gpa, roots);
    defer gpa.free(roots_brp);

    // p(x) = c0 + c1*x in evaluation form over the brp domain.
    const c0 = frFromU64(11);
    const c1 = frFromU64(123456789);
    const poly = try gpa.alloc(Fr, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(poly);
    for (poly, roots_brp) |*p, root| p.* = c0.add(c1.mul(root));

    // Out-of-domain z: the barycentric formula must reproduce c0 + c1*z.
    const z = frFromU64(7777);
    const y = try evaluatePolynomialInEvaluationForm(gpa, poly, z, roots_brp);
    try std.testing.expect(y.eql(c0.add(c1.mul(z))));

    // In-domain z: direct indexing.
    const y_domain = try evaluatePolynomialInEvaluationForm(gpa, poly, roots_brp[5], roots_brp);
    try std.testing.expect(y_domain.eql(poly[5]));
}

// The linear polynomial's commitment is INDEPENDENTLY predictable from
// the monomial setup: commit(c0 + c1*x) = [c0]G + [c1]([s]G1). Together
// with the blob #4 KAT this pins the Lagrange basis/BRP/domain
// conventions against the real ceremony a second, structurally different
// way.
test "cross-check: linear polynomial's Lagrange-basis commitment equals [c0]G + [c1][s]G1 from the monomial setup" {
    const gpa = std.testing.allocator;
    const setup = testSetup();
    const roots = try computeRootsOfUnity(gpa, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, gpa, roots);
    defer gpa.free(roots_brp);

    const c0 = frFromU64(11);
    const c1 = frFromU64(123456789);
    var blob: Blob = undefined;
    for (roots_brp, 0..) |root, i| {
        const value = c0.add(c1.mul(root));
        blob[i * BYTES_PER_FIELD_ELEMENT ..][0..BYTES_PER_FIELD_ELEMENT].* = value.toBytes();
    }

    const commitment = try blobToKzgCommitment(gpa, &blob, setup);
    const gen = g1.Jacobian.fromAffine(g1.Affine.generator);
    const expected = gen.scalarMul(c0)
        .add(g1.Jacobian.fromAffine(setup.g1_monomial[1]).scalarMul(c1));
    try std.testing.expectEqualSlices(u8, &g1.toBytesCompressed(expected.toAffine()), &commitment);
}

test "computeKzgProof at an in-domain z (a root of unity): y indexes the polynomial and the proof verifies" {
    const gpa = std.testing.allocator;
    const setup = testSetup();
    const blob = loadTestBlobRandom4();

    const roots = try computeRootsOfUnity(gpa, FIELD_ELEMENTS_PER_BLOB);
    defer gpa.free(roots);
    const roots_brp = try bitReversalPermutation(Fr, gpa, roots);
    defer gpa.free(roots_brp);
    const poly = try blobToPolynomial(gpa, &blob);
    defer gpa.free(poly);

    const z_bytes = roots_brp[5].toBytes();
    const result = try computeKzgProof(gpa, &blob, z_bytes, setup);
    try std.testing.expectEqualSlices(u8, &poly[5].toBytes(), &result.y);

    const commitment = hexBytesN(48, commitment_random_4_hex);
    try std.testing.expect(try verifyKzgProof(commitment, z_bytes, result.y, result.proof, setup));
}

test "round-trip: pseudorandom blob commits, proves, verifies; tampered y / proof / blob all reject" {
    const gpa = std.testing.allocator;
    const setup = testSetup();

    // Deterministic pseudorandom blob (canonical elements via reduceWide).
    var prng = std.Random.DefaultPrng.init(0x4b5a_47_2026_07_14);
    var blob: Blob = undefined;
    for (0..FIELD_ELEMENTS_PER_BLOB) |i| {
        var raw: [32]u8 = undefined;
        prng.random().bytes(&raw);
        blob[i * BYTES_PER_FIELD_ELEMENT ..][0..BYTES_PER_FIELD_ELEMENT].* = Fr.reduceWide(&raw).toBytes();
    }

    const commitment = try blobToKzgCommitment(gpa, &blob, setup);

    // Point-evaluation proof at a pseudorandom z.
    var z_bytes: [32]u8 = undefined;
    prng.random().bytes(&z_bytes);
    z_bytes = Fr.reduceWide(&z_bytes).toBytes();
    const opening = try computeKzgProof(gpa, &blob, z_bytes, setup);
    try std.testing.expect(try verifyKzgProof(commitment, z_bytes, opening.y, opening.proof, setup));

    // Wrong y (y + 1) must reject.
    const wrong_y = (try bytesToBlsField(opening.y)).add(Fr.one).toBytes();
    try std.testing.expect(!(try verifyKzgProof(commitment, z_bytes, wrong_y, opening.proof, setup)));

    // Wrong (but subgroup-valid) proof must reject.
    const wrong_proof = g1.toBytesCompressed(g1.Affine.generator);
    try std.testing.expect(!(try verifyKzgProof(commitment, z_bytes, opening.y, wrong_proof, setup)));

    // Blob-level proof round-trip + a tampered blob must reject.
    const blob_proof = try computeBlobKzgProof(gpa, &blob, commitment, setup);
    try std.testing.expect(try verifyBlobKzgProof(gpa, &blob, commitment, blob_proof, setup));
    var tampered = blob;
    tampered[0..BYTES_PER_FIELD_ELEMENT].* = frFromU64(1).toBytes(); // still canonical, different polynomial
    try std.testing.expect(!(try verifyBlobKzgProof(gpa, &tampered, commitment, blob_proof, setup)));
}

test "non-canonical field elements are rejected fail-closed at every bytes entry point" {
    const setup = testSetup();
    const blob = loadTestBlobConstant2();
    const commitment = hexBytesN(48, commitment_constant_2_hex);
    const zero: Bytes32 = [_]u8{0} ** 32;

    // z >= r in computeKzgProof.
    try std.testing.expectError(error.InvalidFieldElement, computeKzgProof(std.testing.allocator, &blob, BLS_MODULUS, setup));
    // z >= r and y >= r in verifyKzgProof.
    try std.testing.expectError(error.InvalidFieldElement, verifyKzgProof(commitment, BLS_MODULUS, zero, G1_POINT_AT_INFINITY, setup));
    try std.testing.expectError(error.InvalidFieldElement, verifyKzgProof(commitment, zero, BLS_MODULUS, G1_POINT_AT_INFINITY, setup));
    // A non-canonical blob element in the blob-level paths.
    var bad_blob = blob;
    bad_blob[0..32].* = BLS_MODULUS;
    try std.testing.expectError(error.InvalidFieldElement, blobToKzgCommitment(std.testing.allocator, &bad_blob, setup));
    try std.testing.expectError(error.InvalidFieldElement, verifyBlobKzgProof(std.testing.allocator, &bad_blob, commitment, G1_POINT_AT_INFINITY, setup));
}

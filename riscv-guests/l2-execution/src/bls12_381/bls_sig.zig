// SPDX-License-Identifier: MIT
//! bls_sig — Part 4 of the `bls12_381` arc: BLS signatures per
//! **draft-irtf-cfrg-bls-signature-05**, **minimal-pubkey-size,
//! ProofOfPossession ciphersuite ONLY**
//! (`BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_`): public keys live in
//! `G1` (48-byte compressed — `PublicKey`), signatures live in `G2`
//! (96-byte compressed — `Signature`), messages are hashed onto `G2`
//! via Part 3's `hash_to_curve.hashToCurveG2` under this ciphersuite's
//! own DST (`dst_sig`). This is the ciphersuite Ethereum's consensus
//! layer (and most production BLS deployments) actually use: small
//! (48-byte) public keys are stored/gossiped far more often than the
//! (96-byte) signatures they verify, so putting the smaller element in
//! the more-frequently-transmitted slot is the usual real-world
//! trade-off. Builds directly on Parts 1-3 (`g1.zig`/`g2.zig`'s point
//! arithmetic and wire codecs, `pairing.zig`'s `pairingCheck`,
//! `hash_to_curve.zig`'s `hashToCurveG2`) — this file adds nothing to
//! the field/group/pairing math itself, only the BLS-signature-specific
//! wiring and the "rogue-key"-defeating equations (plain verify,
//! proof-of-possession).
//!
//! **Out of scope for this file** (see `SPEC.md`'s "Out of scope"):
//! the **min-sig** variant (public keys in `G2`, signatures in `G1` —
//! the API below is shaped so a sibling `PublicKeyMinSig`/
//! `SignatureMinSig` pair COULD be added later without disturbing this
//! one, but none is implemented here — "Part 4b"); the **Basic** and
//! **MessageAugmentation** schemes (draft §3.1/§3.2 — different
//! rogue-key mitigations than ProofOfPossession's; this file implements
//! ONLY the ProofOfPossession scheme, §3.3).
//!
//! **Status: COMPLETE (crypto-core pass, 2026-07-14).** Everything is
//! REAL: wire codecs (`SecretKey`/`PublicKey`/`Signature`
//! `toBytes`/`fromBytes`, reusing `Fr`'s and `g1`'s/`g2`'s own codecs
//! directly), `keyGen` (draft §2.3 — HKDF-based), `skToPk`,
//! `keyValidate`, `aggregate`/`aggregatePublicKeys`, AND the
//! security-critical pairing-based cores — `sign`, `verify`,
//! `coreAggregateVerify`/`aggregateVerify`, `fastAggregateVerify`,
//! `popProve`/`popVerify` — each implementing exactly the draft
//! pseudocode its doc comment quotes, byte-exact against
//! `ethereum/bls12-381-tests` v0.1.2 vectors (see the KAT tests below
//! and `NOTICE`). Every verify-family entry point is TOTAL on
//! attacker-controlled input: the mandatory `signature_subgroup_check`
//! and `KeyValidate` preconditions run fail-closed (return `false`,
//! never panic) BEFORE any pairing work — see each function's doc
//! comment and `SPEC.md`'s threat model.
//!
//! Const-time discipline: the secret-key paths (`keyGen`, `skToPk`,
//! `sign`, `popProve`) use Part 1's constant-time double-and-add-always
//! `scalarMul` and contain no secret-dependent branches (`keyGen`'s
//! retry branch fires with probability ~2^-255 on the derived scalar
//! being zero — the draft's own construction); the verify family
//! (`verify`/`aggregate*`/`popVerify`) operates on PUBLIC data only and
//! is variable-time, like the pairing and hash-to-curve it is built on
//! (`SPEC.md`, "Constant-time choices").

const std = @import("std");
const g1 = @import("g1.zig");
const g2 = @import("g2.zig");
const scalarmod = @import("scalar.zig");
const pairingmod = @import("pairing.zig");
const hash_to_curve = @import("hash_to_curve.zig");

pub const Fr = scalarmod.Fr;

// ── ciphersuite constants ───────────────────────────────────────────
//
// draft-irtf-cfrg-bls-signature-05 §4.2.3, "BLS_SIG_BLS12381G2_XMD:
// SHA-256_SSWU_RO_POP_" — the minimal-pubkey-size, ProofOfPossession
// ciphersuite. Fetched 2026-07-14 from
// https://www.ietf.org/archive/id/draft-irtf-cfrg-bls-signature-05.txt
// — see NOTICE.

/// The ciphersuite ID AND `hash_to_point` DST for ordinary message
/// signing (`sign`/`verify`/`coreAggregateVerify`'s `hashToCurveG2`
/// `dst` argument — draft §4.2.3; see their doc comments).
pub const dst_sig = "BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_";

/// The SEPARATE DST used ONLY for proof-of-possession hashing
/// (`popProve`/`popVerify`, draft §4.2.3). Using `dst_sig` here by
/// mistake would let a PoP double as a forged ordinary signature over
/// the public-key bytes (the classic reason PoP schemes use a
/// domain-separated hash for the possession proof, not the message
/// DST) — exactly the kind of mistake `SPEC.md`'s threat model calls
/// out.
pub const dst_pop = "BLS_POP_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_";

pub const BlsError = error{
    /// `keyGen`'s IKM precondition (draft §2.3: "IKM MUST be at least
    /// 32 bytes long, but it MAY be longer").
    IkmTooShort,
    /// `aggregate`/`aggregatePublicKeys`/`coreAggregateVerify`/
    /// `aggregateVerify`/`fastAggregateVerify`'s precondition `n >= 1`
    /// (draft §2.8/§3.3.3/§3.3.4: an empty input set is INVALID, not
    /// vacuously true).
    EmptySet,
    /// `coreAggregateVerify`/`aggregateVerify`'s precondition that the
    /// public-key list and message list have the same length.
    LengthMismatch,
} || g1.G1Error || g2.G2Error || scalarmod.FrError;

// ── key / signature types ───────────────────────────────────────────

/// A BLS secret key: an `Fr` scalar `1 <= sk < r` (`keyGen`'s
/// postcondition — see that function). Wraps `scalar.zig`'s `Fr`
/// directly. `toBytes`/`fromBytes` are `Fr`'s own REAL 32-byte
/// big-endian codec — the draft does not mandate a secret-key wire
/// format; the plain scalar encoding is the common convention (e.g.
/// Ethereum deposit data, EIP-2335 keystores).
pub const SecretKey = struct {
    scalar: Fr,

    pub const encoded_bytes = Fr.encoded_bytes;

    /// REAL — delegates to `Fr.toBytes` directly.
    pub fn toBytes(self: SecretKey) [encoded_bytes]u8 {
        return self.scalar.toBytes();
    }

    /// REAL — delegates to `Fr.fromBytes` directly (rejects `>= r`,
    /// same canonical-encoding contract as `Fr` itself; does NOT
    /// reject `0`, since a zero scalar is merely a degenerate key, not
    /// a malformed encoding — `keyGen` itself never returns one, but a
    /// hand-crafted or corrupted key might decode to one).
    pub fn fromBytes(bytes: [encoded_bytes]u8) BlsError!SecretKey {
        return .{ .scalar = try Fr.fromBytes(bytes) };
    }

    /// Zeroize the secret scalar in place. `SecretKey` is entirely
    /// secret material (a single `Fr` field, no public parts), so this
    /// zeroes the whole struct. Idempotent. Hygiene only — no effect on
    /// any public key/signature already derived from it.
    pub fn deinit(self: *SecretKey) void {
        std.crypto.secureZero(u8, std.mem.asBytes(self));
    }
};

/// A BLS public key: a `G1` point (min-pk variant). Wraps `g1.Affine`
/// directly.
pub const PublicKey = struct {
    point: g1.Affine,

    pub const encoded_bytes = g1.compressed_bytes; // 48

    /// REAL — delegates to `g1.toBytesCompressed`/`fromBytesCompressed`
    /// directly. Decoding does NOT subgroup-check (same contract as
    /// `g1.fromBytesCompressed` itself — see that function's doc
    /// comment): callers crossing a trust boundary MUST additionally
    /// call `keyValidate` (below), matching draft §2.5's own
    /// `KeyValidate` obligation.
    pub fn toBytes(self: PublicKey) [encoded_bytes]u8 {
        return g1.toBytesCompressed(self.point);
    }

    pub fn fromBytes(bytes: [encoded_bytes]u8) BlsError!PublicKey {
        return .{ .point = try g1.fromBytesCompressed(bytes) };
    }
};

/// A BLS signature: a `G2` point (min-pk variant — the message-hash
/// group). Wraps `g2.Affine` directly.
pub const Signature = struct {
    point: g2.Affine,

    pub const encoded_bytes = g2.compressed_bytes; // 96

    /// REAL — same shape/caveat as `PublicKey.toBytes`/`fromBytes`, one
    /// tower level up: decoding does NOT subgroup-check; `verify` and
    /// friends MUST call `g2.Jacobian.subgroupCheck` on `sig.point`
    /// themselves (draft §2.7 step 3, "signature_subgroup_check") —
    /// see each function's doc comment below.
    pub fn toBytes(self: Signature) [encoded_bytes]u8 {
        return g2.toBytesCompressed(self.point);
    }

    pub fn fromBytes(bytes: [encoded_bytes]u8) BlsError!Signature {
        return .{ .point = try g2.fromBytesCompressed(bytes) };
    }
};

// ── key generation / derivation / validation (REAL) ─────────────────

/// The literal `"BLS-SIG-KEYGEN-SALT-"` ASCII string (20 octets) —
/// `keyGen`'s salt on its FIRST HKDF-Extract call (draft §2.3). NOT
/// pre-hashed: the draft explicitly notes "Setting salt to the value
/// H(\"BLS-SIG-KEYGEN-SALT-\") ... results in a KeyGen algorithm that is
/// compatible with version 4 of this document" — i.e. pre-hashing is
/// the OLDER (`-04`) behavior, not this (`-05`) one. See `keyGen`'s doc
/// comment for why this distinction matters (it is exactly why this
/// file does NOT reuse an EIP-2333 test vector).
const default_salt = "BLS-SIG-KEYGEN-SALT-";
comptime {
    std.debug.assert(default_salt.len == 20);
}

/// `L = ceil((3 * ceil(log2(r))) / 16)` (draft §2.3) for BLS12-381's
/// scalar-field order `r` — comptime-derived from `scalar.zig`'s own
/// independently-verified `modulus.bits()` (255), NOT transcribed as a
/// bare literal: `ceil(3*255/16) = ceil(47.8125) = 48`.
const l_bytes: usize = (3 * scalarmod.modulus.bits() + 15) / 16;
comptime {
    std.debug.assert(l_bytes == 48);
}

/// draft-irtf-cfrg-bls-signature-05 §2.3 `KeyGen(IKM, key_info)`. REAL
/// — mechanical HKDF wiring over already-REAL primitives
/// (`std.crypto.kdf.hkdf.HkdfSha256`; `Fr.reduceWide` for the
/// `OS2IP(OKM) mod r` step, itself REAL — `scalar.zig`). Construction
/// (quoted from the draft, fetched 2026-07-14 — see NOTICE):
///
/// ```
/// salt = "BLS-SIG-KEYGEN-SALT-"              // literal, FIRST iteration only
/// while True:
///     PRK = HKDF-Extract(salt, IKM || I2OSP(0, 1))
///     OKM = HKDF-Expand(PRK, key_info || I2OSP(L, 2), L)
///     SK  = OS2IP(OKM) mod r
///     if SK != 0:
///         return SK
///     salt = H(salt)                          // SHA-256 of the PREVIOUS salt, retry only
/// ```
///
/// `IKM` MUST be `>= 32` bytes (draft's own MUST — `error.IkmTooShort`
/// otherwise, rather than silently proceeding on weak input). `key_info`
/// defaults to the empty string in the draft; here it is an explicit
/// (possibly-empty) slice, matching this module's other explicit-
/// parameter style. `key_info.len` is bounded to fit a fixed 256-byte
/// stack buffer (`key_info.len <= 254`) — generous for every realistic
/// caller (the draft imposes no upper bound itself); widen the buffer
/// if a longer `key_info` caller ever appears.
///
/// **NOT wired against an external byte-exact KAT** (see this file's
/// module doc comment and the owner-facing scaffold report): the
/// draft's own Appendix B is explicitly "TBA" for KeyGen test vectors
/// as of `-05`, and **EIP-2333 is a DIFFERENT algorithm despite the
/// textual similarity** (same HKDF shape, same `"BLS-SIG-KEYGEN-SALT-"`
/// string) — EIP-2333's `derive_master_SK` PRE-HASHES the salt on the
/// FIRST call (`salt = H("BLS-SIG-KEYGEN-SALT-")`), which is exactly
/// the `-04`-compatible behavior the draft's own text above documents
/// as DIFFERENT from `-05`'s (raw ASCII string on the first call,
/// hashing only on retry). Reusing an EIP-2333 vector here without
/// adapting for that difference would silently pin the WRONG
/// algorithm — flagged rather than guessed. `TODO(fable)`: if a
/// byte-exact `-05`-compatible KeyGen vector surfaces, wire it; until
/// then this function is exercised only by the self-consistent
/// round-trip test below (`keyGen` -> `skToPk` -> ... -> `verify`).
pub fn keyGen(ikm: []const u8, key_info: []const u8) BlsError!SecretKey {
    if (ikm.len < 32) return error.IkmTooShort;
    std.debug.assert(key_info.len <= 254);

    const Hkdf = std.crypto.kdf.hkdf.HkdfSha256;
    var salt_buf: [32]u8 = undefined;
    var salt: []const u8 = default_salt;

    while (true) {
        var prk_state = Hkdf.extractInit(salt);
        prk_state.update(ikm);
        prk_state.update(&[_]u8{0}); // I2OSP(0, 1): a single zero octet
        var prk: [Hkdf.prk_length]u8 = undefined;
        prk_state.final(&prk);

        var ctx_buf: [256]u8 = undefined;
        @memcpy(ctx_buf[0..key_info.len], key_info);
        std.mem.writeInt(u16, ctx_buf[key_info.len..][0..2], @intCast(l_bytes), .big); // I2OSP(L, 2)
        const ctx = ctx_buf[0 .. key_info.len + 2];

        var okm: [l_bytes]u8 = undefined;
        Hkdf.expand(&okm, ctx, prk);

        const sk = Fr.reduceWide(&okm);
        if (!sk.isZero()) return .{ .scalar = sk };

        std.crypto.hash.sha2.Sha256.hash(salt, &salt_buf, .{});
        salt = &salt_buf;
    }
}

/// draft §2.4 `SkToPk(SK) = point_to_pubkey([SK]G1)` (min-pk variant —
/// `P` is the `G1` generator; the message hash and signature live in
/// the OTHER group, `G2`, for this ciphersuite). REAL: a thin,
/// judgment-free wrapper over Part 1's already-real, constant-time
/// `g1.Jacobian.scalarMul` — `SK` is a genuine secret here, so the
/// constant-time path is required and is exactly what `scalarMul`
/// already provides (`g1.zig`'s own constant-time discipline).
pub fn skToPk(sk: SecretKey) PublicKey {
    const p = g1.Jacobian.fromAffine(g1.Affine.generator).scalarMul(sk.scalar);
    return .{ .point = p.toAffine() };
}

/// draft §2.5 `KeyValidate(PK)`: `PK` must decode to a NON-IDENTITY
/// point in the order-`r` subgroup `G1`. REAL — a thin, judgment-free
/// composition of already-real Part-1 primitives (`Affine.infinity`,
/// `g1.Jacobian.subgroupCheck`); this is exactly the check `SPEC.md`'s
/// threat model (and this file's module doc comment) says every
/// trust-boundary entry point MUST perform on an externally-supplied
/// public key, made explicit and reusable here — `sign`/`verify`/
/// `coreAggregateVerify`/`fastAggregateVerify`/`popVerify`'s doc
/// comments all call this out as a mandatory precondition.
pub fn keyValidate(pk: PublicKey) bool {
    if (pk.point.infinity) return false;
    return g1.Jacobian.fromAffine(pk.point).subgroupCheck();
}

/// The negated `G1` generator, `-P` in the draft's own notation — the
/// fixed second operand every `pairingCheck`-based verify equation in
/// this file needs: `e(X,Y) == e(P,Z)` rearranges to
/// `pairingCheck({X,Y}, {-P,Z})` (`pairing.zig`'s product-check form).
/// REAL: `g1.Jacobian.negate` is already real (Part 1); this is just a
/// named, reusable wrapper so every verify-family function below can
/// share it instead of
/// recomputing it inline.
pub fn negatedG1Generator() g1.Affine {
    return g1.Jacobian.fromAffine(g1.Affine.generator).negate().toAffine();
}

// ── signing / verification (SECURITY-CRITICAL) ──────────────────────

/// draft §2.6 `CoreSign(SK, message)`, this ciphersuite's DST
/// (`dst_sig`). Construction (quoted from the draft's pseudocode,
/// fetched 2026-07-14 — see NOTICE):
///
/// ```
/// Q = hash_to_point(message)        // hash_to_curve.hashToCurveG2(message, dst_sig)
/// R = SK * Q                        // g2.Jacobian.scalarMul
/// signature = point_to_signature(R) // Signature{ .point = R.toAffine() }
/// return signature
/// ```
///
/// `hashToCurveG2` (Part 3) already lands `Q` in the order-`r` subgroup
/// (RFC 9380's `clear_cofactor`), so no extra subgroup check is needed
/// on `Q` here — only on EXTERNALLY-supplied points (`verify`'s `sig`
/// argument, `keyValidate`'s `pk`, both elsewhere).
pub fn sign(sk: SecretKey, msg: []const u8) Signature {
    const q = hash_to_curve.hashToCurveG2(msg, dst_sig);
    // Constant-time scalar multiplication (double-and-add-always,
    // g2.zig): `sk` is a genuine secret here. `msg` and `q` are public
    // (hash-to-curve itself is variable-time by design — SPEC.md's
    // threat model).
    const r = g2.Jacobian.fromAffine(q).scalarMul(sk.scalar);
    return .{ .point = r.toAffine() };
}

/// draft §2.7 `CoreVerify(PK, message, signature)` for the min-pk
/// variant, EXPRESSED in this module's own
/// `pairing.pairing(p: G1.Affine, q: G2.Affine) -> Gt` argument
/// convention: the draft's abstract `C1 = pairing(Q, xP)` / `C2 =
/// pairing(R, P)` becomes, with `Q, R ∈ G2` and `xP, P ∈ G1` for min-pk —
///
/// ```
/// C1 = pairing.pairing(xP, Q)   =  e(PK, H(message))
/// C2 = pairing.pairing(P, R)    =  e(G1_generator, signature)
/// accept iff C1 == C2
/// ```
///
/// — i.e. the SAME check, computed as ONE shared multi-Miller-loop +
/// ONE final exponentiation via
///
/// ```
/// pairing.pairingCheck(&.{
///     .{ .p = pk.point, .q = Q },
///     .{ .p = negatedG1Generator(), .q = sig.point },
/// })
/// ```
///
/// (`pairing.zig`'s product-check idiom: `e(A,B)*e(-C,D) == 1` iff
/// `e(A,B) == e(C,D)`) — the form `verify`/`coreAggregateVerify`/
/// `fastAggregateVerify`/`popVerify` should ALL actually use, not two
/// separate `pairing.pairing` calls (the whole point of `pairingCheck`
/// existing, see its own doc comment).
///
/// MANDATORY preconditions (draft §2.7 steps 2-4 — skipping ANY of
/// these is the classic BLS forgery/rogue-key class of bug this
/// module's `SPEC.md` centers its threat model on; `pk`/`sig` are
/// FULLY ATTACKER-CONTROLLED at this entry point, so every failure
/// below MUST return `false`, never panic or error):
///   1. `signature_subgroup_check(R)` — `g2.Jacobian.fromAffine(sig.
///      point).subgroupCheck()`.
///   2. `keyValidate(pk)` — this file's own `keyValidate`, ALREADY
///      real; call it (it itself covers the non-identity + subgroup
///      checks on `pk`).
///   3. `Q = hash_to_curve.hashToCurveG2(msg, dst_sig)`.
pub fn verify(pk: PublicKey, msg: []const u8, sig: Signature) bool {
    // Draft §2.7 steps 2-4, fail-closed: BOTH mandatory checks run
    // before any pairing work, and every failure returns false (never
    // panics — pk/sig are attacker-controlled at this entry point).
    if (!g2.Jacobian.fromAffine(sig.point).subgroupCheck()) return false; // signature_subgroup_check
    if (!keyValidate(pk)) return false; // non-identity + G1 subgroup
    const q = hash_to_curve.hashToCurveG2(msg, dst_sig);
    // e(PK, H(m)) == e(G1_gen, sig), as one shared multi-Miller-loop +
    // one final exponentiation (see doc comment above).
    return pairingmod.pairingCheck(&.{
        .{ .p = pk.point, .q = q },
        .{ .p = negatedG1Generator(), .q = sig.point },
    });
}

// ── aggregation (REAL: plain point summation, no pairing) ───────────

/// draft §2.8 `Aggregate((signature_1, ..., signature_n))`: sums the
/// signatures' `G2` points. REAL — mechanical Jacobian point
/// summation over already-real Part-1 `g2.Jacobian.add`; no pairing
/// and no security judgment involved (the SECURITY-CRITICAL part of
/// aggregation lives entirely on the VERIFY side —
/// `coreAggregateVerify`/`fastAggregateVerify`, above/below).
/// Precondition `n >= 1` (draft): `error.EmptySet` otherwise, matching
/// the draft's own INVALID-on-empty-input contract. Does NOT subgroup-
/// check its inputs (same convention as `Signature.fromBytes` — a
/// `Signature` value is assumed already validated by whatever produced
/// it; `coreAggregateVerify`/`fastAggregateVerify` re-check the
/// AGGREGATE result's subgroup membership themselves, per the draft).
pub fn aggregate(sigs: []const Signature) BlsError!Signature {
    if (sigs.len == 0) return error.EmptySet;
    var acc = g2.Jacobian.fromAffine(sigs[0].point);
    for (sigs[1..]) |s| acc = acc.add(g2.Jacobian.fromAffine(s.point));
    return .{ .point = acc.toAffine() };
}

/// The `G1`-side analogue of `aggregate`, used internally by
/// `fastAggregateVerify`'s construction (draft §3.3.4 steps 1-4: `PK =
/// point_to_pubkey(sum of pubkey_to_point(PK_i))`) and exposed
/// publicly since it is also the "eth_aggregate_pubkeys"-shaped
/// primitive (the consensus-spec-tests BLS suite's own handler name for
/// exactly this operation) callers commonly want standalone. REAL —
/// same reasoning as `aggregate`.
pub fn aggregatePublicKeys(pks: []const PublicKey) BlsError!PublicKey {
    if (pks.len == 0) return error.EmptySet;
    var acc = g1.Jacobian.fromAffine(pks[0].point);
    for (pks[1..]) |pk| acc = acc.add(g1.Jacobian.fromAffine(pk.point));
    return .{ .point = acc.toAffine() };
}

// ── aggregate verification (SECURITY-CRITICAL) ──────────────────────

/// draft §3.3.3 `CoreAggregateVerify` for the ProofOfPossession scheme
/// (draft §3.2's MESSAGE-AUGMENTATION variant, which prepends `PK_i ||`
/// to each message before hashing, is a DIFFERENT scheme and OUT OF
/// SCOPE here — see this file's module doc comment / `SPEC.md`).
/// Construction:
///
/// ```
/// R = signature_to_point(signature)
/// if !signature_subgroup_check(R): return false
/// C1 = 1_GT
/// for i in 1..n:
///     if !KeyValidate(PK_i): return false
///     C1 = C1 * pairing(Q_i, xP_i)     where Q_i = hash_to_point(message_i, dst)
/// C2 = pairing(R, P)
/// return C1 == C2
/// ```
///
/// In this module's `pairingCheck` product form (see `verify`'s doc
/// comment for the same idiom on a single pair): the pairs are
/// `{ pk_i, hashToCurveG2(msg_i, dst) }` for EVERY `i`, PLUS one
/// trailing `{ negatedG1Generator(), sig.point }`, product-checked
/// against `1_Gt` with ONE shared final exponentiation. Because this
/// function takes no allocator and `n` is caller-controlled, the pairs
/// are NOT materialized as a single heap slice; instead the raw Miller
/// values are accumulated over fixed-size STACK chunks
/// (`multiMillerLoop` per chunk, `Fp12.mul` between chunks) and final-
/// exponentiated ONCE at the end — exactly equivalent to a single
/// `pairing.pairingCheck` call over all `n+1` pairs (Miller values
/// multiply — `Fp12.conjugate` is multiplicative — and
/// `finalExponentiation` is a group homomorphism; `pairingCheck`'s own
/// doc comment makes the same argument), and exactly as cheap: the
/// chunk width below matches `pairing.zig`'s own internal Miller batch
/// width, so the shared-squaring saving is identical to what one giant
/// slice would get. `dst` is a parameter (not hardcoded to `dst_sig`)
/// so `aggregateVerify` below can be a pure wrapper — the
/// ProofOfPossession scheme itself only ever calls this with `dst_sig`.
///
/// Preconditions `n >= 1` and `pks.len == msgs.len`
/// (`error.EmptySet`/`error.LengthMismatch`) are checked BEFORE any
/// pairing work; every SECURITY failure (subgroup/`KeyValidate`)
/// returns `false`, never panics — `pks`/`sig` are attacker-controlled
/// at this entry point.
///
/// NOTE (draft §3.1): when used via the Basic scheme's
/// `AggregateVerify`, all messages MUST be distinct; the
/// ProofOfPossession scheme (this file) instead defends rogue keys via
/// `popProve`/`popVerify` at registration time, so distinct messages
/// are not required here — see `fastAggregateVerify`'s doc comment.
pub fn coreAggregateVerify(pks: []const PublicKey, msgs: []const []const u8, sig: Signature, dst: []const u8) BlsError!bool {
    if (pks.len == 0) return error.EmptySet;
    if (pks.len != msgs.len) return error.LengthMismatch;

    // Draft §3.3.3 step 3: signature_subgroup_check, before any pairing.
    if (!g2.Jacobian.fromAffine(sig.point).subgroupCheck()) return false;

    var f = pairingmod.Fp12.one;
    var pairs_buf: [8]pairingmod.PairingPair = undefined; // 8 == pairing.zig's miller_batch_max
    var pending: usize = 0;
    for (pks, msgs) |pk, msg| {
        // Draft §3.3.3 step 6: KeyValidate EVERY public key.
        if (!keyValidate(pk)) return false;
        pairs_buf[pending] = .{ .p = pk.point, .q = hash_to_curve.hashToCurveG2(msg, dst) };
        pending += 1;
        if (pending == pairs_buf.len) {
            f = f.mul(pairingmod.multiMillerLoop(pairs_buf[0..pending]));
            pending = 0;
        }
    }
    pairs_buf[pending] = .{ .p = negatedG1Generator(), .q = sig.point };
    pending += 1;
    f = f.mul(pairingmod.multiMillerLoop(pairs_buf[0..pending]));
    // One shared final exponentiation over the whole accumulated
    // Miller product — the `pairingCheck` idiom, chunk-accumulated.
    return pairingmod.finalExponentiation(f).eql(pairingmod.Fp12.one);
}

/// draft §3.3.3 `AggregateVerify`: `coreAggregateVerify` fixed to this
/// file's ciphersuite DST (`dst_sig`). Pure wrapper, not a distinct
/// algorithm — the ProofOfPossession scheme does not augment messages
/// with public keys (that is `MessageAugmentation`'s trick, draft §3.2,
/// out of scope here).
pub fn aggregateVerify(pks: []const PublicKey, msgs: []const []const u8, sig: Signature) BlsError!bool {
    return coreAggregateVerify(pks, msgs, sig, dst_sig);
}

/// draft §3.3.4 `FastAggregateVerify(PKs, message, signature)`:
/// aggregate the public keys (this file's own REAL
/// `aggregatePublicKeys`), then `CoreVerify` the aggregate key against
/// the single shared message. Construction:
///
/// ```
/// aggregate = pubkey_to_point(PK_1)
/// for i in 2..n: aggregate += pubkey_to_point(PK_i)
/// PK = point_to_pubkey(aggregate)
/// return CoreVerify(PK, message, signature)     // == this file's verify()
/// ```
///
/// i.e. `const agg_pk = try aggregatePublicKeys(pks); return
/// verify(agg_pk, msg, sig);` once `verify` itself is filled in.
///
/// **SECURITY PRECONDITION callers MUST satisfy (draft §3.3.4,
/// verbatim, fetched 2026-07-14 — see NOTICE): "The caller MUST know a
/// proof of possession for all PK_i, and PopVerify(PK_i, proof_i) MUST
/// be VALID"** for every key being aggregated. This is NOT checked by
/// this function (nor could it be — a PoP is checked ONCE, at
/// key-registration time, not per-verify) — it is precisely what makes
/// the ProofOfPossession scheme's `FastAggregateVerify` safe against
/// rogue-key attacks WITHOUT the more expensive per-message
/// `KeyValidate`-style aggregation checks that plain multi-signature
/// schemes need: `popProve`/`popVerify` (below) bind each secret key to
/// its claimed public key once, ahead of time. Skipping this
/// precondition at the CALLER's registration step is a real forgery
/// vector — see `SPEC.md`'s threat model.
pub fn fastAggregateVerify(pks: []const PublicKey, msg: []const u8, sig: Signature) BlsError!bool {
    if (pks.len == 0) return error.EmptySet;
    const agg_pk = try aggregatePublicKeys(pks);
    // `verify` performs the mandatory checks on the AGGREGATE
    // (signature subgroup check + KeyValidate of the aggregate key) —
    // exactly the draft's construction; per-key validity is the PoP
    // precondition above, established at registration, not here.
    return verify(agg_pk, msg, sig);
}

// ── proof of possession (SECURITY-CRITICAL) ─────────────────────────

/// draft §3.3.2 `PopProve(SK)`: a self-signature over the SIGNER'S OWN
/// public key bytes, hashed under the SEPARATE `dst_pop` DST (NOT
/// `dst_sig` — see that constant's doc comment for why conflating the
/// two would be a signature/PoP-confusion vulnerability). Construction:
///
/// ```
/// PK = SkToPk(SK)                     // this file's REAL skToPk
/// Q = hash_pubkey_to_point(PK)        // hashToCurveG2(&PK.toBytes(), dst_pop)
/// R = SK * Q                          // g2.Jacobian.scalarMul
/// proof = point_to_signature(R)
/// return proof
/// ```
pub fn popProve(sk: SecretKey) Signature {
    const pk = skToPk(sk);
    const q = hash_to_curve.hashToCurveG2(&pk.toBytes(), dst_pop);
    // Constant-time scalar multiplication — `sk` is a genuine secret
    // (same reasoning as `sign`; the hashed message here — the public
    // key's own serialization — is public).
    const r = g2.Jacobian.fromAffine(q).scalarMul(sk.scalar);
    return .{ .point = r.toAffine() };
}

/// draft §3.3.3 `PopVerify(PK, proof)`: verifies a `popProve` output
/// against the SAME `dst_pop` hash of `PK`'s own bytes. Same equation
/// shape as `verify` (see that function's doc comment for the
/// `pairingCheck` product-form idiom), over `hash_pubkey_to_point`
/// instead of `hash_to_point`:
///
/// ```
/// R = signature_to_point(proof)
/// if !signature_subgroup_check(R): return false
/// if !KeyValidate(PK): return false
/// Q = hash_pubkey_to_point(PK)        // hashToCurveG2(&PK.toBytes(), dst_pop)
/// return pairing.pairingCheck(&.{
///     .{ .p = PK.point, .q = Q },
///     .{ .p = negatedG1Generator(), .q = proof.point },
/// })
/// ```
pub fn popVerify(pk: PublicKey, proof: Signature) bool {
    // Same fail-closed discipline as `verify`: both mandatory checks
    // run before any pairing work; every failure returns false.
    if (!g2.Jacobian.fromAffine(proof.point).subgroupCheck()) return false; // proof_subgroup_check
    if (!keyValidate(pk)) return false; // non-identity + G1 subgroup
    const q = hash_to_curve.hashToCurveG2(&pk.toBytes(), dst_pop);
    return pairingmod.pairingCheck(&.{
        .{ .p = pk.point, .q = q },
        .{ .p = negatedG1Generator(), .q = proof.point },
    });
}

// ── test helpers ─────────────────────────────────────────────────────

fn hexBytes(comptime n: usize, comptime hex: *const [2 * n:0]u8) [n]u8 {
    @setEvalBranchQuota(100_000);
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

// ── tests: REAL, PASS today ──────────────────────────────────────────

test "SecretKey.deinit zeroizes the secret scalar (regression: fails if secureZero is removed)" {
    var sk_bytes = [_]u8{0} ** 32;
    sk_bytes[31] = 7;
    var sk = try SecretKey.fromBytes(sk_bytes);
    const zero_bytes = [_]u8{0} ** 32;
    try std.testing.expect(!std.mem.eql(u8, &sk.toBytes(), &zero_bytes));
    sk.deinit();
    // `Fr`'s in-memory (Montgomery) size may exceed its 32-byte canonical
    // encoding, so assert every raw byte of the struct is zero rather than
    // comparing against a fixed-width buffer.
    for (std.mem.asBytes(&sk)) |b| try std.testing.expectEqual(@as(u8, 0), b);
}

test "SecretKey/PublicKey/Signature byte codecs round-trip through Part-1-3 machinery" {
    var sk_bytes = [_]u8{0} ** 32;
    sk_bytes[31] = 7;
    const sk = try SecretKey.fromBytes(sk_bytes);
    try std.testing.expectEqualSlices(u8, &sk_bytes, &sk.toBytes());

    const pk: PublicKey = .{ .point = g1.Affine.generator };
    const pk_bytes = pk.toBytes();
    try std.testing.expectEqual(@as(usize, 48), pk_bytes.len);
    const pk2 = try PublicKey.fromBytes(pk_bytes);
    try std.testing.expect(pk2.point.x.eql(g1.Affine.generator.x));

    const sig: Signature = .{ .point = g2.Affine.generator };
    const sig_bytes = sig.toBytes();
    try std.testing.expectEqual(@as(usize, 96), sig_bytes.len);
    const sig2 = try Signature.fromBytes(sig_bytes);
    try std.testing.expect(sig2.point.x.eql(g2.Affine.generator.x));
}

test "keyGen rejects IKM shorter than 32 bytes" {
    try std.testing.expectError(error.IkmTooShort, keyGen(&[_]u8{0} ** 31, ""));
    _ = try keyGen(&[_]u8{0x42} ** 32, ""); // 32 bytes: must not error
}

test "keyGen is deterministic (same IKM/key_info -> same SK) and produces a nonzero scalar" {
    const ikm = [_]u8{0xab} ** 32;
    const a = try keyGen(&ikm, "");
    const b = try keyGen(&ikm, "");
    try std.testing.expect(a.scalar.eql(b.scalar));
    try std.testing.expect(!a.scalar.isZero());

    // Different key_info overwhelmingly likely gives a different SK
    // (not a correctness requirement, just a sanity check that
    // key_info actually participates in the derivation).
    const c = try keyGen(&ikm, "some-key-info");
    try std.testing.expect(!a.scalar.eql(c.scalar));
}

test "skToPk produces a subgroup-valid, non-identity public key; keyValidate accepts it" {
    const sk = try keyGen(&([_]u8{0x11} ** 32), "");
    const pk = skToPk(sk);
    try std.testing.expect(!pk.point.infinity);
    try std.testing.expect(keyValidate(pk));
}

test "keyValidate rejects the identity public key" {
    const pk: PublicKey = .{ .point = g1.Affine.identity };
    try std.testing.expect(!keyValidate(pk));
}

test "keyValidate rejects a non-subgroup G1 point" {
    // Same x=4 non-subgroup construction g1.zig's own subgroupCheck
    // test uses (independently verified there: on-curve, NOT in the
    // order-r subgroup).
    var comp = [_]u8{0} ** g1.compressed_bytes;
    comp[0] = 0x80;
    comp[g1.compressed_bytes - 1] = 4;
    const pk = try PublicKey.fromBytes(comp);
    try std.testing.expect(!keyValidate(pk));
}

test "aggregate/aggregatePublicKeys reject an empty input slice" {
    try std.testing.expectError(error.EmptySet, aggregate(&.{}));
    try std.testing.expectError(error.EmptySet, aggregatePublicKeys(&.{}));
}

test "aggregate of a single signature returns that signature unchanged" {
    const sig: Signature = .{ .point = g2.Affine.generator };
    const agg = try aggregate(&.{sig});
    try std.testing.expectEqualSlices(u8, &sig.toBytes(), &agg.toBytes());
}

test "aggregate is commutative/associative over G2 points (property; no signature semantics needed)" {
    const g = g2.Jacobian.fromAffine(g2.Affine.generator);
    const a: Signature = .{ .point = g.double().toAffine() }; // [2]G2
    const b: Signature = .{ .point = g.double().double().toAffine() }; // [4]G2
    const c: Signature = .{ .point = g.double().double().double().toAffine() }; // [8]G2

    const ab = try aggregate(&.{ a, b });
    const ba = try aggregate(&.{ b, a });
    try std.testing.expectEqualSlices(u8, &ab.toBytes(), &ba.toBytes());

    const abc = try aggregate(&.{ a, b, c });
    const expected = g2.Jacobian.fromAffine(a.point).add(g2.Jacobian.fromAffine(b.point)).add(g2.Jacobian.fromAffine(c.point)).toAffine();
    try std.testing.expectEqualSlices(u8, &g2.toBytesCompressed(expected), &abc.toBytes());
}

test "aggregate KAT: three published signatures over the same message combine to the published aggregate" {
    // Source: ethereum/bls12-381-tests, tag v0.1.2, JSON encoding,
    // aggregate/aggregate_0xabab...ab.json (message = 32 bytes of
    // 0xab). Downloaded 2026-07-14 from
    // https://github.com/ethereum/bls12-381-tests/releases/download/v0.1.2/bls_tests_json.tar.gz
    // — see NOTICE. This is the same ciphersuite/vector family the
    // Ethereum consensus-spec-tests general/phase0/bls/aggregate suite
    // publishes (same handler name, same ciphersuite); the JSON here
    // was fetched and verified directly, not merely assumed identical
    // to the (much larger, not directly fetched) consensus-spec-tests
    // release tarball.
    //
    // REAL today: `aggregate` itself is fully implemented (plain G2
    // point summation, no pairing) — this test genuinely PASSES right
    // now, unlike the sign/verify/fastAggregateVerify KATs below.
    const sig1 = Signature{ .point = try g2.fromBytesCompressed(hexBytes(96, "91347bccf740d859038fcdcaf233eeceb2a436bcaaee9b2aa3bfb70efe29dfb2677562ccbea1c8e061fb9971b0753c240622fab78489ce96768259fc01360346da5b9f579e5da0d941e4c6ba18a0e64906082375394f337fa1af2b7127b0d121")) };
    const sig2 = Signature{ .point = try g2.fromBytesCompressed(hexBytes(96, "9674e2228034527f4c083206032b020310face156d4a4685e2fcaec2f6f3665aa635d90347b6ce124eb879266b1e801d185de36a0a289b85e9039662634f2eea1e02e670bc7ab849d006a70b2f93b84597558a05b879c8d445f387a5d5b653df")) };
    const sig3 = Signature{ .point = try g2.fromBytesCompressed(hexBytes(96, "ae82747ddeefe4fd64cf9cedb9b04ae3e8a43420cd255e3c7cd06a8d88b7c7f8638543719981c5d16fa3527c468c25f0026704a6951bde891360c7e8d12ddee0559004ccdbe6046b55bae1b257ee97f7cdb955773d7cf29adf3ccbb9975e4eb9")) };
    const expected = hexBytes(96, "9712c3edd73a209c742b8250759db12549b3eaf43b5ca61376d9f30e2747dbcf842d8b2ac0901d2a093713e20284a7670fcf6954e9ab93de991bb9b313e664785a075fc285806fa5224c82bde146561b446ccfc706a64b8579513cfc4ff1d930");

    const agg = try aggregate(&.{ sig1, sig2, sig3 });
    try std.testing.expectEqualSlices(u8, &expected, &agg.toBytes());
}

test "coreAggregateVerify/aggregateVerify reject empty sets and length mismatches before any pairing work" {
    const sig: Signature = .{ .point = g2.Affine.generator };
    try std.testing.expectError(error.EmptySet, aggregateVerify(&.{}, &.{}, sig));

    const pk: PublicKey = .{ .point = g1.Affine.generator };
    try std.testing.expectError(error.LengthMismatch, aggregateVerify(&.{ pk, pk }, &.{"only one message"}, sig));
}

test "fastAggregateVerify rejects an empty pubkey slice before any pairing work" {
    const sig: Signature = .{ .point = g2.Affine.generator };
    try std.testing.expectError(error.EmptySet, fastAggregateVerify(&.{}, "msg", sig));
}

// ── tests: byte-exact KATs + round-trips over the pairing-based ─────
// ── cores (all REAL and passing since the crypto-core pass) ─────────

test "sign KAT: privkey/message -> published signature (ethereum/bls12-381-tests v0.1.2, sign/sign_case_11b8c7cad5238946.json)" {
    // Source: same tarball as the aggregate KAT above — see NOTICE.
    const sk = try SecretKey.fromBytes(hexBytes(32, "47b8192d77bf871b62e87859d653922725724a5c031afeabc60bcef5ff665138"));
    const msg = hexBytes(32, "0000000000000000000000000000000000000000000000000000000000000000");
    const expected = hexBytes(96, "b23c46be3a001c63ca711f87a005c200cc550b9429d5f4eb38d74322144f1b63926da3388979e5321012fb1a0526bcd100b5ef5fe72628ce4cd5e904aeaa3279527843fae5ca9ca675f4f51ed8f83bbf7155da9ecc9663100a885d5dc6df96d9");

    const sig = sign(sk, &msg);
    try std.testing.expectEqualSlices(u8, &expected, &sig.toBytes());
}

test "verify KAT: accepts a valid signature and rejects a mismatched one (ethereum/bls12-381-tests v0.1.2, verify/verify_valid_case_195246ee3bd3b6ec.json)" {
    // Source: same tarball as the aggregate KAT above — see NOTICE.
    // NOTE: this pubkey/message/signature triple is the SAME one used
    // as the third element of the aggregate KAT above and the third
    // pubkey of the fastAggregateVerify KAT below — all four vectors
    // are drawn from one mutually-consistent published set.
    //
    // The "tamper" case deliberately does NOT use the upstream suite's
    // own verify_tampered_signature_case_195246ee3bd3b6ec.json: that
    // vector flips trailing bytes of the compressed point into a
    // non-canonical encoding that fails to DESERIALIZE at all
    // (`g2.fromBytesCompressed` returns `error.NotOnCurve` — confirmed
    // empirically), which would test `Signature.fromBytes`'s rejection,
    // not `verify`'s pairing check, since this module's `verify` takes
    // an already-decoded `Signature`, not raw bytes. Instead: `sig1`
    // (the aggregate KAT's first signature, a genuine, on-curve,
    // in-subgroup G2 point — just for a DIFFERENT signer/message) is
    // substituted as a "wrong signature" for `pk`/`msg`, which isolates
    // exactly the pairing-equation rejection `verify` is responsible
    // for.
    const pk = try PublicKey.fromBytes(hexBytes(48, "b53d21a4cfd562c469cc81514d4ce5a6b577d8403d32a394dc265dd190b47fa9f829fdd7963afdf972e5e77854051f6f"));
    const msg = hexBytes(32, "abababababababababababababababababababababababababababababababab");
    const sig = try Signature.fromBytes(hexBytes(96, "ae82747ddeefe4fd64cf9cedb9b04ae3e8a43420cd255e3c7cd06a8d88b7c7f8638543719981c5d16fa3527c468c25f0026704a6951bde891360c7e8d12ddee0559004ccdbe6046b55bae1b257ee97f7cdb955773d7cf29adf3ccbb9975e4eb9"));
    const wrong_sig = try Signature.fromBytes(hexBytes(96, "91347bccf740d859038fcdcaf233eeceb2a436bcaaee9b2aa3bfb70efe29dfb2677562ccbea1c8e061fb9971b0753c240622fab78489ce96768259fc01360346da5b9f579e5da0d941e4c6ba18a0e64906082375394f337fa1af2b7127b0d121"));

    try std.testing.expect(verify(pk, &msg, sig));
    try std.testing.expect(!verify(pk, &msg, wrong_sig));
}

test "fastAggregateVerify KAT: three pubkeys + the aggregate signature over their shared message (ethereum/bls12-381-tests v0.1.2, fast_aggregate_verify/fast_aggregate_verify_valid_3d7576f3c0e3570a.json)" {
    // Source: same tarball as the aggregate KAT above — see NOTICE.
    // The signature here is the SAME aggregate the "aggregate KAT" test
    // above independently reproduces from its 3 inputs.
    const pk1 = try PublicKey.fromBytes(hexBytes(48, "a491d1b0ecd9bb917989f0e74f0dea0422eac4a873e5e2644f368dffb9a6e20fd6e10c1b77654d067c0618f6e5a7f79a"));
    const pk2 = try PublicKey.fromBytes(hexBytes(48, "b301803f8b5ac4a1133581fc676dfedc60d891dd5fa99028805e5ea5b08d3491af75d0707adab3b70c6a6a580217bf81"));
    const pk3 = try PublicKey.fromBytes(hexBytes(48, "b53d21a4cfd562c469cc81514d4ce5a6b577d8403d32a394dc265dd190b47fa9f829fdd7963afdf972e5e77854051f6f"));
    const msg = hexBytes(32, "abababababababababababababababababababababababababababababababab");
    const sig = try Signature.fromBytes(hexBytes(96, "9712c3edd73a209c742b8250759db12549b3eaf43b5ca61376d9f30e2747dbcf842d8b2ac0901d2a093713e20284a7670fcf6954e9ab93de991bb9b313e664785a075fc285806fa5224c82bde146561b446ccfc706a64b8579513cfc4ff1d930"));

    const ok = try fastAggregateVerify(&.{ pk1, pk2, pk3 }, &msg, sig);
    try std.testing.expect(ok);
}

test "PopProve/PopVerify round-trip (self-consistent — no external vector needed)" {
    const sk = try keyGen(&([_]u8{0x22} ** 32), "");
    const pk = skToPk(sk);
    const proof = popProve(sk);
    try std.testing.expect(popVerify(pk, proof));

    // A proof for a DIFFERENT key must not verify.
    const other_sk = try keyGen(&([_]u8{0x33} ** 32), "");
    const other_pk = skToPk(other_sk);
    try std.testing.expect(!popVerify(other_pk, proof));
}

test "end-to-end round trip: keyGen -> skToPk -> sign -> verify -> aggregate -> aggregateVerify (fresh key material, no external constant needed)" {
    const sk1 = try keyGen(&([_]u8{0x01} ** 32), "");
    const sk2 = try keyGen(&([_]u8{0x02} ** 32), "");
    const pk1 = skToPk(sk1);
    const pk2 = skToPk(sk2);

    const msg1 = "first message";
    const msg2 = "second message";
    const sig1 = sign(sk1, msg1);
    const sig2 = sign(sk2, msg2);

    try std.testing.expect(verify(pk1, msg1, sig1));
    try std.testing.expect(!verify(pk1, msg2, sig1)); // wrong message
    try std.testing.expect(!verify(pk2, msg1, sig1)); // wrong key

    const agg_sig = try aggregate(&.{ sig1, sig2 });
    const ok = try aggregateVerify(&.{ pk1, pk2 }, &.{ msg1, msg2 }, agg_sig);
    try std.testing.expect(ok);

    // Tamper: swapping the messages must fail.
    const bad = try aggregateVerify(&.{ pk1, pk2 }, &.{ msg2, msg1 }, agg_sig);
    try std.testing.expect(!bad);
}

// ── tests: the mandatory security checks actually FIRE ──────────────

test "verify rejects the identity public key (KeyValidate fires, fail-closed, no panic)" {
    // A well-formed signature by a REAL key over this message — the
    // only invalid ingredient is the identity pubkey, so a `true` or a
    // panic here would mean the mandatory KeyValidate step is missing.
    const sk = try keyGen(&([_]u8{0x55} ** 32), "");
    const msg = "identity pk must never verify";
    const sig = sign(sk, msg);

    const identity_pk: PublicKey = .{ .point = g1.Affine.identity };
    try std.testing.expect(!verify(identity_pk, msg, sig));
    try std.testing.expect(!popVerify(identity_pk, sig));

    // Aggregate entry points too: an identity pk anywhere in the set
    // must fail closed, not error and not panic.
    const good_pk = skToPk(sk);
    const two_msgs: []const []const u8 = &.{ msg, msg };
    try std.testing.expect(!(try aggregateVerify(&.{ good_pk, identity_pk }, two_msgs, sig)));
}

test "fastAggregateVerify rejects a ROGUE-KEY pair (cancels to identity) + the trivial identity signature" {
    // The complete forgery `keyValidate`'s aggregate-level check exists to
    // stop, spelled out: TWO individually-valid, non-identity, subgroup-
    // valid public keys whose SUM is the identity (`pk2 = -pk1`), paired
    // with the TRIVIAL identity signature. For an identity `PK`, the
    // verify equation `e(PK, Q) == e(G, sig)` becomes `1_GT ==
    // e(G, sig)`, which holds iff `sig` is ALSO the identity in G2 — and
    // the identity has order 1, so `signature_subgroup_check` (order
    // divides r for every element, including the identity) does not
    // reject it either. Nothing in the pairing arithmetic itself can
    // catch this forgery; ONLY `keyValidate`'s explicit check on the
    // aggregated key stands between an attacker who controls no secret
    // key at all and a signature that "verifies" for any message.
    const sk = try keyGen(&([_]u8{0x77} ** 32), "");
    const pk1 = skToPk(sk);
    try std.testing.expect(keyValidate(pk1)); // precondition: pk1 alone is fine
    const pk2: PublicKey = .{ .point = g1.Jacobian.fromAffine(pk1.point).negate().toAffine() };
    try std.testing.expect(keyValidate(pk2)); // precondition: pk2 alone is fine too

    const identity_sig: Signature = .{ .point = g2.Affine.identity };
    try std.testing.expect(!(try fastAggregateVerify(&.{ pk1, pk2 }, "any message at all", identity_sig)));
}

test "verify rejects a non-subgroup G2 signature (signature_subgroup_check fires, fail-closed, no panic)" {
    // Craft an on-curve E'(Fp2) point OUTSIDE the order-r subgroup:
    // `mapToCurveG2` (Part 3's SSWU + 3-isogeny) lands on the curve but
    // does NOT clear the ~636-bit cofactor — that is `hashToCurveG2`'s
    // separate final step. The test asserts the crafted point genuinely
    // fails the subgroup check (test precondition, not an assumption).
    const u = hash_to_curve.hashToFieldFp2(1, "non-subgroup G2 point seed", dst_sig);
    const raw = hash_to_curve.mapToCurveG2(u[0]);
    try std.testing.expect(g2.Jacobian.fromAffine(raw).isOnCurve());
    try std.testing.expect(!g2.Jacobian.fromAffine(raw).subgroupCheck());

    const sk = try keyGen(&([_]u8{0x66} ** 32), "");
    const pk = skToPk(sk);
    const bad_sig: Signature = .{ .point = raw };
    const msg = "non-subgroup sig must never verify";

    try std.testing.expect(!verify(pk, msg, bad_sig));
    try std.testing.expect(!popVerify(pk, bad_sig));
    try std.testing.expect(!(try aggregateVerify(&.{pk}, &.{msg}, bad_sig)));
    try std.testing.expect(!(try fastAggregateVerify(&.{pk}, msg, bad_sig)));
}

test "aggregateVerify with more signers than one Miller chunk (exercises the chunked pairing accumulator)" {
    // coreAggregateVerify accumulates PairingPairs in stack chunks of 8
    // (matching pairing.zig's internal batch width); 8 signers + the
    // trailing (-G1, sig) pair = 9 pairs, forcing one full chunk flush
    // plus a trailer chunk — the boundary a chunking bug would hide at.
    const n = 8;
    var sks: [n]SecretKey = undefined;
    var pks: [n]PublicKey = undefined;
    var msgs: [n][]const u8 = .{
        "chunk msg 0", "chunk msg 1", "chunk msg 2", "chunk msg 3",
        "chunk msg 4", "chunk msg 5", "chunk msg 6", "chunk msg 7",
    };
    var sigs: [n]Signature = undefined;
    for (0..n) |i| {
        var ikm = [_]u8{0x70} ** 32;
        ikm[31] = @intCast(i);
        sks[i] = try keyGen(&ikm, "");
        pks[i] = skToPk(sks[i]);
        sigs[i] = sign(sks[i], msgs[i]);
    }

    const agg_sig = try aggregate(&sigs);
    try std.testing.expect(try aggregateVerify(&pks, &msgs, agg_sig));

    // Tamper across the chunk boundary: swap the last two messages.
    std.mem.swap([]const u8, &msgs[n - 2], &msgs[n - 1]);
    try std.testing.expect(!(try aggregateVerify(&pks, &msgs, agg_sig)));
}

// ── fuzz harnesses (untrusted-wire decoders) ────────────────────────────

test "fuzz: SecretKey.fromBytes never crashes on arbitrary bytes" {
    try std.testing.fuzz({}, fuzzSecretKeyFromBytes, .{});
}

fn fuzzSecretKeyFromBytes(_: void, smith: *std.testing.Smith) !void {
    var buf: [SecretKey.encoded_bytes]u8 = undefined;
    smith.bytes(&buf);
    var sk = SecretKey.fromBytes(buf) catch return;
    defer sk.deinit();
    _ = sk.toBytes();
}

test "fuzz: PublicKey.fromBytes never crashes on arbitrary bytes" {
    try std.testing.fuzz({}, fuzzPublicKeyFromBytes, .{});
}

fn fuzzPublicKeyFromBytes(_: void, smith: *std.testing.Smith) !void {
    var buf: [PublicKey.encoded_bytes]u8 = undefined;
    smith.bytes(&buf);
    const pk = PublicKey.fromBytes(buf) catch return;
    _ = pk.toBytes();
    _ = keyValidate(pk);
}

test "fuzz: Signature.fromBytes never crashes on arbitrary bytes" {
    try std.testing.fuzz({}, fuzzSignatureFromBytes, .{});
}

fn fuzzSignatureFromBytes(_: void, smith: *std.testing.Smith) !void {
    var buf: [Signature.encoded_bytes]u8 = undefined;
    smith.bytes(&buf);
    const sig = Signature.fromBytes(buf) catch return;
    _ = sig.toBytes();
}

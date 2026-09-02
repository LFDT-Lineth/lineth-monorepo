# bls12_381 — SPEC

BLS12-381: the pairing-friendly elliptic curve behind BLS signatures,
KZG commitments, and threshold-BLS — see [README.md](README.md) for
purpose and API. Provenance: see [NOTICE](NOTICE).

## Purpose

Parts 1-5 of a multi-part arc (`README.md`). Part 1: the base field `Fp`,
the extension tower `Fp2`/`Fp6`/`Fp12`, the scalar field `Fr`, and the two
pairing groups `G1`/`G2` — everything the pairing function needs as its
foundation. Part 2 (`pairing.zig`): the pairing itself, `e: G1 x G2 ->
Gt`, `Gt ⊂ Fp12*` — the optimal ate Miller loop plus final
exponentiation. Part 3 (`hash_to_curve.zig`): RFC 9380 hash-to-curve
for `G1`/`G2` — the primitive BLS signatures use to hash a message onto
the curve. Part 4 (`bls_sig.zig`, **COMPLETE**): BLS signatures per
draft-irtf-cfrg-bls-signature-05, minimal-pubkey-size/ProofOfPossession
ciphersuite. Part 5 (`kzg.zig`, **COMPLETE**): EIP-4844 (deneb)
KZG polynomial commitments over the trusted-setup ceremony's `G1`/`G2`
points. Part 6 (`threshold.zig`, **COMPLETE**): trusted-dealer
threshold BLS — Shamir secret sharing + Feldman VSS +
Lagrange-in-the-exponent combining, over Part 4's min-pk ciphersuite —
see "Part 6 design" below.

## Model-after + seed

- **Source of truth**: `draft-irtf-cfrg-pairing-friendly-curves` (the
  BLS12-381 parameter family) plus the ZCash/IETF BLS12-381
  point-serialization convention — see `NOTICE` for the full citation
  list and the independent re-derivation/verification performed on
  every embedded constant.
- **`Fp`/`Fr` ride on `std.crypto.ff.Modulus`/`Fe`** (`fp.zig`/
  `scalar.zig`): std's own constant-time, allocation-free Montgomery
  modular-arithmetic machinery for a fixed-width odd modulus. This
  module supplies the BLS12-381-specific modulus values and the
  field-arithmetic entry points on top; it does NOT reimplement
  big-integer or Montgomery arithmetic itself. `Fp` uses a 384-bit
  (48-byte) container for the 381-bit `p`; `Fr` uses a 256-bit
  (32-byte) container for the 255-bit `r`.
- **Design references for the field-tower arithmetic** (implemented by
  the crypto-core pass): Devegili–Ó hÉigeartaigh–Scott–Dahab's
  tower-field multiplication paper (`Fp6`/`Fp12` mul/square/inv), Adj &
  Rodriguez-Henriquez ePrint 2012/685 Alg. 9 (`Fp2.sqrt`, the complex
  method), and the Bernstein–Lange Explicit-Formulas Database
  (`add-2007-bl`/`dbl-2009-l` Jacobian point formulas) — see `NOTICE`
  for full citations. `zkcrypto/bls12_381`/`blst` remain UNREAD as
  source code; every formula was validated against
  independently-computed vectors instead (NOTICE's "Verification
  performed (crypto-core pass)").

## Design & invariants

- **All arithmetic implemented** (crypto-core pass, 2026-07-14): the
  scaffold's REAL/STUB split is resolved — every field and group
  operation is now real, with one deliberate exception:
  `Fp12.cyclotomicSquare` remains `@panic("TODO(part2)")` (a
  pairing-only fast path that lands with its sole consumer, the Part-2
  final exponentiation — see its doc comment for the day-one test
  oracle Part 2 can use).
- **Montgomery-storage decision (settled)**: `Fp`/`Fr` store values
  CANONICALLY (non-Montgomery) at rest; `std.crypto.ff` operations
  preserve the first operand's form, so canonical-in/canonical-out
  holds everywhere and `toBytes`/`fromBytes` need no conversion at the
  struct boundary. Cost: `mul`/`sq` convert in/out of Montgomery form
  internally (a few extra Montgomery multiplications per call).
  Persistent Montgomery storage is a deferred Part-2 (pairing hot
  path) optimization — see Backlog.
- **Frobenius coefficients: derived in code, never transcribed.**
  `Fp6`'s `γ_1 = ξ^((p-1)/3)` (and `γ_2 = γ_1²`) and `Fp12`'s
  `γ = ξ^((p-1)/6)` are computed via `Fp2.pow` with exponents derived
  at COMPTIME from the verified `p_bytes` (`fp.pExponentBytes`, which
  hard-fails the build if a division is inexact). Tests pin them two
  ways: definitionally (`frobenius(a) == a^p` computed with plain
  mul/square) and byte-exactly against independently-recomputed values
  (NOTICE). Coefficients are recomputed per `frobenius` call — cheap
  for Part 1; caching is a Part-2 optimization (Backlog).
- **`Fp2` non-residue convention: `u² = -1`.** Confirmed by
  independently checking the `G2` generator on-curve under exactly this
  convention (`NOTICE`'s "Verification performed"). A different
  non-residue choice would produce a self-consistent but DIFFERENT
  curve/pairing — every constant and formula throughout the tower
  (`Fp6`'s `v³ = u+1`, `Fp12`'s `w² = v`, `G2`'s generator, Part 2's
  eventual Frobenius coefficients) must stay consistent with this one
  choice.
- **The `Fp2` wire-order pitfall**: an `Fp2` element `c0 + c1*u`
  serializes `c1` (high) before `c0` (low) — the OPPOSITE of the
  natural reading order. `fp2.zig`'s `encoded_bytes` doc comment flags
  this explicitly; it is a well-known, easy-to-miss BLS12-381 interop
  bug across implementations (byte-swapping the two halves produces a
  different-but-parseable value that can pass shape checks and only
  fail — or, on unlucky input, not even fail — the on-curve check).
- **The G2 cofactor scaffold bug (found and fixed by the crypto-core
  pass).** The scaffold's `g2.zig` `cofactor_bytes` was derived with
  the QUADRATIC-twist trace formula `#E' = p² + 1 - t2` — which is
  actually the order of `E(Fp2)` (the original curve over `Fp2`), not
  of the SEXTIC twist `G2` lives on. Because `E(Fp2) ⊇ E(Fp) ⊇ G1`,
  that wrong order is ALSO divisible by `r`, so the scaffold's
  exact-division sanity check passed. Caught because `clearCofactor`
  built on it failed the behavioral test (`[r]([h2]P) != O` — a direct
  Lagrange violation for the claimed group order); corrected via the
  CM equation for sextic twists (`f² = (4p² - t2²)/3`, `#E' = p² + 1 -
  (t2 - 3f)/2`) and triple-verified — see `g2.zig`'s `cofactor_bytes`
  doc comment and `NOTICE`. Lesson recorded: an exact-division check
  alone cannot distinguish the twist's order from the curve's; a
  behavioral (Lagrange) test can.

## Part 2 design (the pairing — crypto-core pass DONE, 2026-07-14)

- **Construction: optimal ate pairing (Vercauteren 2010), BLS12 family.**
  The standard choice for BLS12 curves — the shortest known Miller loop
  for this embedding degree (`k=12`), keyed to the low-Hamming-weight
  seed `x = -0xd201000000010000` (same `bls_x_abs` this file's siblings
  already cite). `G2`'s sextic twist is D-TYPE (confirmed by `g2.zig`'s
  own `b' = 4(1+u)` curve-equation choice and the `u^2=-1` non-residue
  convention `SPEC.md`'s Part-1 section documents) — line evaluations
  use the D-type untwisting map (Costello-Lange-Naehrig 2010).
- **Miller loop (as implemented)**: per-pair `T` accumulators in AFFINE
  twist coordinates (chord/tangent slope explicit for the line
  function; one `Fp2.inv` per step — total, because the twist group's
  order `r*h2` is odd, so no 2-torsion, and `T == ±Q` is impossible
  for subgroup inputs; see the doc comments' non-degeneracy arguments).
  Lines are evaluated at the TWISTED image of `P` — off the untwisted
  line by a fixed `w^3` factor per line, which the final exponentiation
  provably kills (`(w^3)^((p^12-1)/r) = 1`, derivation in `lineEval`'s
  doc comment) — giving the sparse `c0.c0/c0.c1/c1.c1` ("014") shape,
  promoted to a dense `Fp12.mul` in the correctness-first baseline.
  Multi-pairing shares one seed-bit walk + one accumulator per batch of
  up to 8 pairs (fixed stack storage, allocation-free; the batch
  product is exact — squaring distributes — so batching only forgoes
  cross-batch shared squarings). The negative seed is handled by ONE
  final conjugation.
- **Final exponentiation hard part (as implemented)**: the
  Hayashida–Hayasaka–Teruya (ePrint 2020/875, BLS12 case) EXACT-`d`
  cyclotomic identity `d = (p^4-p^2+1)/r = k*(x+p)*(x^2+p^2-1) + 1`
  with `k = (x-1)^2/3` — chosen over the scaffold's FCKRH (ePrint
  2011/465) suggestion because FCKRH's chain computes `f^(3d)` (a fixed
  CUBE of the canonical pairing: bilinearity-equivalent, and
  `pairingCheck`-equivalent since `gcd(3,r)=1`, but NOT byte-equal to
  the IETF draft's `e(G1,G2)` test vector — verified numerically for
  this exact seed before choosing). `k` is comptime-derived from
  `bls_x_abs` (division-by-3 exactness enforced at comptime — no
  transcribed constant); the whole chain runs on
  `Fp12.cyclotomicSquare` (Granger–Scott, PKC 2010, via the `Fp4`
  decomposition) + `Fp12.frobeniusMap` (naive repeated `frobenius`) +
  conjugation-as-inversion (`p^6 ≡ -1 (mod p^4-p^2+1)`, verified
  numerically).
- **The negative-seed sign convention (a real cross-implementation
  difference, discovered by the KAT work)**: this module — like
  zkcrypto/blst and the IETF pairing-friendly-curves draft's official
  test vector — conjugates the Miller accumulator for the negative
  seed. `py_ecc` does NOT (its loop walks `|x|` with no adjustment), so
  py_ecc's `e(G1,G2)` is exactly the canonical value's CONJUGATE (= Gt
  inverse). Both are valid bilinear pairings; the IETF draft's is the
  canonical one this module pins byte-exactly, and the py_ecc
  relationship is itself pinned by a dedicated cross-check test.
- **Verification net**: the bilinearity property suite (needs no
  external constant: non-degeneracy, `e([a]P,[b]Q) == e(P,Q)^(ab)`,
  additivity both sides, `e([a]P,Q) == e(P,[a]Q)`, `pairingCheck`
  identities positive and negative, `e(G1,G2)^r == 1`) PLUS the
  byte-exact `e(G1,G2)` KAT from the IETF draft (primary) and py_ecc
  (conjugate cross-check) — the "property tests as the strong oracle,
  byte-exact KAT layered on top" pattern from `adaptor`/`spake2plus`,
  now with both layers in place.

## Part 3 design (hash-to-curve — COMPLETE, crypto-core pass 2026-07-14)

- **Suites**: `BLS12381G1_XMD:SHA-256_SSWU_RO_` (RFC 9380 §8.8.1) and
  `BLS12381G2_XMD:SHA-256_SSWU_RO_` (§8.8.2) — the two `hash_to_curve`
  random-oracle encodings. Construction, per §3: `hash_to_field(msg,
  2)` → `map_to_curve` each of the two field elements → add the two
  curve points → `clear_cofactor`. `encode_to_curve` (the cheaper,
  nonuniform one-element variant, suites
  `BLS12381G{1,2}_XMD:SHA-256_SSWU_NU_`) is also implemented
  (`encodeToCurveG1`/`encodeToCurveG2`) since it shares the same
  `map_to_curve`/`clear_cofactor` machinery at near-zero extra cost.
- **`map_to_curve`** (`mapToCurveG1`/`mapToCurveG2`): §6.6.3 Simplified
  SWU for `AB == 0` — `sswuG1`/`sswuG2` (§6.6.2's Simplified SWU proper,
  onto the isogenous curves `E1'`/`E2'`, using the new RFC-9380 `inv0`/
  `sgn0` helpers on `Fp`/`Fp2`) then `isogenyMap11`/`isogenyMap3` (the
  Appendix E.2/E.3 rational isogeny maps back onto `G1`/`G2`'s actual
  curves — Horner evaluation over the 53-/13-coefficient tables, monic
  leading denominator terms implicit, zero-denominator exceptional case
  → identity). The SSWU `tv1 == 0` exceptional branch's totality (its
  `g(B/(Z*A))` is always a QR, so the gx2 fallback never fails) was
  verified numerically — see `NOTICE`.
- **`clear_cofactor` = `h_eff` multiplication, NOT plain-cofactor
  multiplication — a scaffold-plan CORRECTION.** The scaffold planned to
  reuse Part 1's `clearCofactor` (multiply by the true cofactor `h`)
  and documented both as "valid per §7"; that is WRONG for these
  suites: §7 pins `clear_cofactor(P) := h_eff * P` (a fixed suite
  parameter, §8.8.1/§8.8.2) and explicitly says plain-`h` multiplication
  "does not generally give the same result ... and MUST NOT be used"
  when `h_eff` comes from a fast clearing method (both BLS12-381
  suites' do). Confirmed empirically: `[h]R != [h_eff]R` on the RFC's
  own vector points, both groups — same subgroup, different point, so
  the J.9.1/J.10.1 final-`P` KATs would fail with `h`. The compositions
  multiply by the suites' `h_eff` via `scalarMulBytes` (`NOTICE`
  entry 21); `g1`/`g2.Jacobian.clearCofactor` remains what it always
  was — a correct way to land arbitrary points in the subgroup — it
  just is not RFC 9380's `clear_cofactor`.
- **`expand_message_xmd` reuse decision (flagged, not unilateral)**:
  `modules/frost` and `modules/voprf` each already carry an in-module
  `expand_message_xmd`, but both are hardcoded to a fixed `ell`
  (frost: SHA-256/`ell=2`; voprf: SHA-512/`ell=1`) that is too narrow
  for this module's needs (`G1`'s `hash_to_field(msg,2)` needs `ell=4`;
  `G2`'s needs `ell=8`), and reusing either directly would add a
  cross-module dependency edge from `bls12_381` to a threshold-signature
  or OPRF module for one hash primitive — a layering smell. This pass
  REIMPLEMENTS `expand_message_xmd` from the RFC 9380 §5.3.1 pseudocode
  directly, generalized to any `ell <= 255` (frost's/voprf's versions
  are each a specialization of this general shape) — a deliberate,
  explicitly-flagged departure from a literal "reuse the existing
  implementation" instruction; the owner may instead choose to factor a
  shared `expand_message_xmd` helper across all three modules later,
  but that is a cross-module refactor this scaffolding pass does not
  make unilaterally.
- **The `Fp` wide-reduction gap**: `hash_to_field`'s `OS2IP(tv) mod p`
  step needs to reduce a 512-bit (`L=64`-byte) integer into `Fp`
  (384-bit container). `scalar.zig`'s `Fr.reduceWide` already solves the
  analogous problem for `Fr` (256-bit container, same 512-bit input)
  via `std.crypto.ff.Uint(512).fromBytes` + `modulus.reduce(wide)` (the
  latter generic in the input width). The identical construction works
  unchanged for `Fp` — confirmed by reading `std.crypto.ff`'s
  `Modulus.reduce` signature (`fn reduce(self: Self, x: anytype) Fe`) —
  so this is REAL, not stubbed; kept as a file-local
  `reduceWideToFp` in `hash_to_curve.zig` rather than promoted to a
  public `Fp.reduceWide` in `fp.zig` (unlike `Fr`'s), since
  `hash_to_field` is (so far) its only consumer — promote it if a
  second caller appears.
- **Constant provenance**: `g1_iso_z`/`g1_iso_a`/`g1_iso_b` and
  `g2_iso_z`/`g2_iso_a`/`g2_iso_b` (the SSWU curve/`Z` parameters for
  `E1'`/`E2'`) are single field-element constants cited verbatim from
  RFC 9380 §8.8.1/§8.8.2's own raw text (re-confirmed against that text
  during the crypto-core pass), the same citation discipline `fp.zig`'s
  `p_bytes`/`g1.zig`'s generator already use. The 11-isogeny (`G1`,
  Appendix E.2, 53 `Fp` constants) and 3-isogeny (`G2`, Appendix E.3,
  13 `Fp2` constants) coefficient tables were NEVER hand-transcribed:
  they were parsed programmatically from the RFC's raw text and emitted
  as Zig literals mechanically, then validated end-to-end by an
  independent big-integer implementation reproducing the RFC's own
  `Q0`/`Q1`/`P` vectors on exactly those parsed values — see `NOTICE`'s
  Part-3 crypto-core verification section. Every table entry is a
  comptime `Fp.fromBytes` (`fpc`/`fp2c`), so a wrong-length or
  non-canonical value fails the build outright. The suites' `h_eff`
  scalars (§8.8.1/§8.8.2) are likewise cited from the raw text
  (`NOTICE` entry 21).

## Part 4 design (BLS signatures — SCAFFOLDED 2026-07-14, crypto-core pass COMPLETE 2026-07-14)

- **Ciphersuite: minimal-pubkey-size, ProofOfPossession, ONLY.**
  `BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_`
  (draft-irtf-cfrg-bls-signature-05 §4.2.3) — public keys in `G1`
  (48-byte compressed), signatures in `G2` (96-byte compressed),
  messages hashed via `hashToCurveG2` (Part 3) under this DST; proofs
  of possession hashed under the SEPARATE `BLS_POP_...` DST. This is
  the ciphersuite Ethereum's consensus layer (and most production BLS
  deployments) actually use — the smaller (48-byte) element goes in
  the more-frequently-transmitted slot (public keys, stored/gossiped
  far more often than the signatures verifying them). **Explicitly OUT
  OF SCOPE**: the min-sig variant (pubkeys in `G2`, signatures in
  `G1`) — "Part 4b" if a consumer needs it later, not implemented
  here; the Basic and MessageAugmentation schemes (draft §3.1/§3.2 —
  different rogue-key mitigations than ProofOfPossession's).
- **The exact verify equation, in this module's own argument
  convention.** `pairing.pairing(p: G1.Affine, q: G2.Affine)` fixes
  the FIRST argument to `G1`, second to `G2`. The draft's abstract
  `CoreVerify` (§2.7) computes `C1 = pairing(Q, xP)` / `C2 =
  pairing(R, P)` where, for min-pk, `Q`/`R` (message hash / signature)
  are `G2` elements and `xP`/`P` (public key / `G1` generator) are
  `G1` elements — so in THIS module's fixed argument order:
  ```
  C1 = pairing.pairing(pk.point, H(message))       = e(PK, H(m))
  C2 = pairing.pairing(G1.Affine.generator, sig.point) = e(G1_gen, sig)
  accept iff C1 == C2
  ```
  computed as ONE `pairingCheck` call (shared multi-Miller-loop + one
  final exponentiation, not two separate `pairing` calls):
  ```
  pairing.pairingCheck(&.{
      .{ .p = pk.point, .q = H(message) },
      .{ .p = negatedG1Generator(), .q = sig.point },
  })
  ```
  `coreAggregateVerify`/`fastAggregateVerify`/`popVerify` all reduce to
  the SAME `pairingCheck` idiom — one `PairingPair` per (pubkey,
  message-hash) term, plus one trailing `(-G1_generator, signature)`
  term. `bls_sig.zig`'s doc comments spell out each one exactly.
  **Crypto-core-pass implementation note (`coreAggregateVerify`)**: the
  function takes no allocator and `n` is caller-controlled, so the
  `n+1` pairs are not materialized as one slice; the raw Miller values
  are accumulated over fixed-size STACK chunks of 8
  (`multiMillerLoop` per chunk, `Fp12.mul` between chunks — 8 matches
  `pairing.zig`'s own internal Miller batch width, so nothing is lost
  versus one giant slice) and final-exponentiated ONCE — exactly
  equivalent to a single `pairingCheck` call, because Miller values
  multiply (`Fp12.conjugate` is multiplicative) and
  `finalExponentiation` is a group homomorphism. A dedicated test
  drives an 8-signer (9-pair) aggregate across the chunk boundary.
  `fastAggregateVerify` is implemented draft-faithfully as
  `aggregatePublicKeys` + `verify` on the aggregate: the mandatory
  checks run on the AGGREGATE key and the signature; per-key validity
  is the PoP registration-time precondition (below), not re-checked
  per verify.
- **Mandatory subgroup/`KeyValidate` checks — WHY, not just what.**
  Every pairing-based entry point (`verify`, `coreAggregateVerify`,
  `fastAggregateVerify`, `popVerify`) MUST, before touching the
  pairing at all:
  1. `signature_subgroup_check` on the signature's `G2` point
     (`g2.Jacobian.subgroupCheck`) — an on-curve-but-wrong-subgroup
     signature is exactly the "small-subgroup" attack class Part 1's
     own threat model (below) already centers on: an attacker who
     controls a low-order `E'(Fp2)` point as a "signature" can force a
     degenerate, predictable pairing value, potentially satisfying the
     verify equation for a value they never actually signed with a
     real secret key.
  2. `KeyValidate` on every public key involved (`keyValidate` — a
     non-identity + `g1.Jacobian.subgroupCheck` check) — the ROGUE-KEY
     attack class: without this, an attacker registering a public key
     computed as `PK_attacker = [c]G1 - PK_victim` (for a self-chosen
     `c`) can produce a valid-looking AGGREGATE signature for a set
     including the victim's key without ever knowing the victim's
     secret key, because the aggregate pairing check only constrains
     the SUM of the underlying secret exponents, not each one
     individually. The ProofOfPossession scheme (this ciphersuite)
     defends against this at KEY-REGISTRATION time instead
     (`popProve`/`popVerify` — every public key must publish a
     self-signature over its own bytes before being accepted into any
     aggregate; forging a PoP requires the same secret key as the
     public key it is over, closing the rogue-key degree of freedom)
     rather than re-`KeyValidate`-ing on every verify — this is
     `fastAggregateVerify`'s documented, UNENFORCED-BY-CODE
     precondition (draft §3.3.4: "the caller MUST know a proof of
     possession for all PK_i, and PopVerify(...) MUST be VALID" —
     checked once, at registration, never inside `fastAggregateVerify`
     itself).
  3. Splitting-zero / identity-input defense: `KeyValidate` also
     rejects the `G1` identity point as a public key — an unchecked
     identity public key can make `pairing(identity, anything) == 1`
     trivially satisfy some malformed verify constructions (the
     "splitting zero" pitfall the draft's own security considerations
     warn about); this module's `keyValidate` rejects it explicitly,
     ahead of the subgroup check.
- **`KeyGen` (draft §2.3) — REAL, but deliberately NOT KAT-verified.**
  Mechanical HKDF-Extract/Expand construction over `std.crypto.kdf.
  hkdf.HkdfSha256` plus `Fr.reduceWide` (`OS2IP(OKM) mod r`) — both
  already-real primitives, so implemented directly rather than
  stubbed. NOT wired against a byte-exact external vector: the draft's
  own Appendix B is "TBA" for KeyGen vectors as of `-05`, and EIP-2333
  — despite an almost-identical HKDF shape and the same
  `"BLS-SIG-KEYGEN-SALT-"` string — is a DIFFERENT algorithm (its
  `derive_master_SK` pre-hashes the salt on the FIRST call, which the
  draft's own text documents as the OLDER, `-04`-compatible behavior,
  not `-05`'s). Conflating the two would silently pin the wrong
  algorithm under a vector that looks plausible — flagged in
  `bls_sig.zig`'s `keyGen` doc comment as a `TODO(fable)` rather than
  guessed. `keyGen` is exercised today only by the self-consistent
  round-trip test (`keyGen` -> `skToPk` -> ... -> `verify`).
- **The `Aggregate`/`aggregatePublicKeys` vs. verify split.** Plain
  point summation (`Aggregate`, draft §2.8, and its `G1`-side analogue)
  needs NO pairing and no security judgment. Everything that CHECKS an
  aggregate (`coreAggregateVerify`, `aggregateVerify`,
  `fastAggregateVerify`) is where all the actual security lives (the
  pairing-product equation, the subgroup/`KeyValidate` preconditions
  above) — all implemented, fail-closed (verify-family functions return
  `false` on any subgroup/identity failure, never panic; the only
  error returns are the mechanical `EmptySet`/`LengthMismatch`/
  decode preconditions).
- **Test-vector sourcing.** `bls_sig.zig`'s wired KATs (`sign`,
  `verify`, `aggregate`, `fastAggregateVerify`) come from
  `ethereum/bls12-381-tests` (tag `v0.1.2`, JSON encoding), downloaded
  and independently verified for this pass (see `NOTICE`) — the same
  ciphersuite and test-case-category names (`sign`/`verify`/
  `aggregate`/`fast_aggregate_verify`) as the Ethereum
  `consensus-spec-tests` repo's `general/phase0/bls/` suite the
  original task pointed at; that larger repo's actual vector files
  ship only inside a ~200MB release tarball this pass did not download,
  so the smaller, directly-fetched-and-verified sibling repo was used
  instead — flagged here rather than silently assumed byte-identical.
  All wired KATs — `aggregate`, `sign`, `verify` (accept + reject),
  `fastAggregateVerify` — PASS byte-exactly since the crypto-core pass,
  alongside the self-consistent `popProve`/`popVerify` and end-to-end
  round trips.
- **Part 4 constant-time choices (crypto-core pass).** The SECRET-key
  paths — `keyGen`, `skToPk`, `sign`, `popProve` — touch `sk` only via
  `Fr`'s ff-backed constant-time arithmetic and Part 1's constant-time
  double-and-add-always `scalarMul` (`skToPk` on `G1`, `sign`/`popProve`
  on `G2`); they contain no secret-dependent branches or memory
  accesses (`keyGen`'s retry loop branches only on the derived scalar
  being exactly zero — probability ~2^-255, and the draft's own
  construction). The VERIFY family (`verify`, `coreAggregateVerify`/
  `aggregateVerify`, `fastAggregateVerify`, `popVerify`) and
  `aggregate`/`aggregatePublicKeys`/`keyValidate` operate exclusively
  on PUBLIC data (public keys, signatures, messages) and are
  variable-time, like the pairing and hash-to-curve they compose
  (see "Constant-time choices" under Threat model).

## Part 5 design (KZG polynomial commitments — COMPLETE 2026-07-14)

- **Model-after: `consensus-specs` `specs/deneb/polynomial-commitments.md`**
  (the deneb/EIP-4844 KZG spec — fetched from the `master` branch,
  2026-07-14; see `NOTICE` for the exact citation). Every constant
  (`FIELD_ELEMENTS_PER_BLOB`, `BYTES_PER_*`, `BLS_MODULUS`,
  `PRIMITIVE_ROOT_OF_UNITY`, the two Fiat-Shamir domain strings,
  `G1_POINT_AT_INFINITY`) is copied verbatim from the fetched spec text,
  not from memory — the same "cite, don't recall" discipline Parts 1-4
  apply to every curve constant (`NOTICE`).
- **The trusted setup: embedded, not fetched at runtime.** The official
  Ethereum KZG ceremony ("Summoning Ceremony") `trusted_setup.txt`
  (`ethereum/c-kzg-4844`'s copy — the de facto canonical distribution
  format: 4096 `G1` Lagrange points, 65 `G2` monomial points, 4096 `G1`
  monomial points, one 96-or-192-hex-char line each) is `@embedFile`d
  into the binary (`data/trusted_setup.txt`, ~788 KiB) rather than
  fetched over the network at load time — matching this repo's zero-I/O,
  zero-dep posture (`CONVENTIONS.md` §2) and how every other embedded
  data asset in this module (none, prior to this) would be handled if
  one existed. `loadTrustedSetup` parses AND validates every point
  (on-curve + subgroup check, REAL — Parts 1/2's own primitives) before
  returning it; nothing downstream trusts an unchecked ceremony point.
- **Basis-order finding (verified against `c-kzg-4844`'s own C loader,
  not assumed): the file's `G1` Lagrange points are NOT pre-permuted.**
  `c-kzg-4844`'s `src/setup/setup.c` reads the file into a plain array
  and only THEN calls `bit_reversal_permutation(...)` on it in place
  (the `_brp`-suffixed field name is the tell). This module's
  `TrustedSetup.g1_lagrange` therefore stores the FILE's natural order;
  `blobToKzgCommitment` applies `bitReversalPermutation` (REAL, this
  file) itself, matching the spec's own
  `bit_reversal_permutation(KZG_SETUP_G1_LAGRANGE)` composition in
  `blob_to_kzg_commitment` — a scaffolding-time finding worth recording
  explicitly (an easy point to get backwards, à la the Part-1 `g2`
  cofactor and Part-3 `h_eff` corrections this module has already hit
  once each).
- **Crypto-core pass (2026-07-14) — implementation decisions.** Every
  formerly-stubbed function is now real, each implementing exactly the
  construction its scaffold doc comment specified (the spec text was
  re-fetched and re-read during this pass, not recalled — `NOTICE`).
  The settled decisions:
  - **Trusted-setup loading is memoized process-wide** (`loadTrustedSetup`
    / `validated_setup_cache`): the input is a `@embedFile`d compile-time
    constant, so the first full validation's verdict holds for every
    call — subsequent loads deep-copy the already-validated points into
    the caller's allocator. NOTHING is skipped; the 8257-point on-curve +
    subgroup validation simply isn't repeated per call (the scaffold's
    per-call re-validation cost minutes per test binary for zero added
    assurance). The cache is a write-once atomic pointer (lock-free; a
    first-load race means redundant validation, never a wrong result).
  - **First-load validation is parallelized and uses the variable-time
    subgroup check for `G1`** (`parseAndValidateEmbeddedSetup` +
    `subgroupCheckVartime`): setup points are PUBLIC (threat model
    below), so the constant-time double-and-add-ALWAYS engine's ~3x
    overhead buys nothing; the per-point checks are independent and fan
    out across CPU cores (`std.Thread.spawn`, serial fallback if
    spawning fails). The 65 `G2` points ride on worker 0 with the
    existing constant-time check (not worth a `G2` vartime twin).
  - **Variable-time `G1` helpers** (`jacAddVartime`/`jacMixedAddVartime`/
    `jacScalarMulVartime`): the same EFD `add-2007-bl`/`madd-2007-bl`/
    `dbl-2009-l` formulas as `g1.zig`, with ordinary branches instead of
    branchless `ctSelect` resolution — pinned against the constant-time
    versions across random AND every degenerate input class (identity
    operands, `P == Q`, `P == -Q`) by dedicated tests. Used only on
    public data (MSM, setup validation); `verifyKzgProofImpl`'s two
    scalar multiplications stay on the constant-time engine (not worth
    a vartime path).
  - **`g1Msm` is Pippenger's bucket method** over the vartime helpers:
    `c`-bit windows (`c` by input size, capped at 8), per-window bucket
    accumulation via mixed addition, running-sum aggregation,
    most-significant-window-first folding with `c` doublings between
    windows — `O(n·256/c)` mixed additions versus naive double-and-add's
    `O(n·256)` full add+double pairs. Zero digits skip bucket work, so
    zero scalars/identity points drop out naturally; `n == 0` returns
    the identity (spec's explicit case). It takes an allocator (scalar
    bytes + bucket array) — a deliberate signature change from the
    scaffold.
  - **The primitive `2^32` root of unity is DERIVED, not embedded**
    (`primitiveRootOfUnity2Pow32`): `7^((r-1)/2^32) mod r` per the
    spec's own `compute_roots_of_unity` formula, with the exponent
    comptime-derived from the verified `scalar.r_bytes`
    (`root32_exponent_bytes` — `r`'s low 32 bits are `0x00000001`, so
    `(r-1)/2^32` is a borrow-free decrement + 4-byte shift). Its order
    is proven exactly `2^32` by tests (`w^(2^32) == 1`, `w^(2^31) ==
    -1`), and the whole domain convention is pinned against the REAL
    ceremony by the two monomial-vs-Lagrange cross-check tests (below).
  - **`evaluatePolynomialInEvaluationForm` takes an allocator now**
    (barycentric denominators are batch-inverted via the Montgomery
    trick, `batchInvInPlace` — one `Fr.inv` + `3n` muls instead of 4096
    Fermat exponentiations per evaluation); the in-domain branch
    (direct indexing) is exercised by KAT-adjacent tests.
  - **`computeKzgProofImpl`** batch-inverts the `x_i - z` denominators
    with the in-domain index (if any) masked behind a placeholder, and
    reuses those inverses for the spec's
    `compute_quotient_eval_within_domain` special case
    (`1/(z - x_i) == -1/(x_i - z)`).
- **`fft`/`ifft` are real but unused by this file's own public API.**
  EIP-4844's specific operations (`blob_to_kzg_commitment`,
  `compute_kzg_proof`) stay in evaluation form throughout (barycentric
  evaluation + direct Lagrange-basis MSM) — no coefficient-form
  conversion is needed for this exact API surface. They are reusable
  `Fr`-generic radix-2 Cooley-Tukey NTT primitives (a future consumer —
  coefficient-form conversion, EIP-7594 cells, a faster batched-proof
  scheme — will want them), verified against a naive DFT, the
  `ifft ∘ fft == id` round trip, AND the real ceremony: the strongest
  single test in the file recovers a KAT blob's coefficient form via
  `ifft` and reproduces the c-kzg-4844 commitment through the MONOMIAL
  setup basis — jointly pinning the root of unity, the
  blob-element-i-is-`p(roots_brp[i])` domain convention, the
  bit-reversal permutation, the `ifft`, and the MSM over both bases.
- **Canonical-field-element enforcement is REAL and load-bearing.**
  `bytesToBlsField`/`blobToPolynomial` REJECT (never reduce) any 32-byte
  input `>= BLS_MODULUS` — matching the spec's `bytes_to_bls_field`
  precisely (a REJECTING parse, unlike `Fr.reduceWide`'s wide-input
  REDUCING parse used elsewhere in this module for hash outputs). A
  blob, `z`, or `y` with a non-canonical element is a malformed-input
  error, not a silently-reduced one.

## Part 6 design (threshold BLS — COMPLETE, crypto-core pass 2026-07-14)

- **Trusted-dealer Shamir + Feldman VSS, min-pk ciphersuite, reusing
  Part 4 wholesale.** `threshold.zig` does not introduce a new
  ciphersuite or wire convention: `SecretKeyShare`/`PublicKeyShare`/
  `PartialSignature` are `bls_sig.SecretKey`/`PublicKey`/`Signature`
  plus a `u32` participant index, and `partialSign`/
  `verifyPartialSignature` are thin wrappers over `bls_sig.sign`/
  `bls_sig.verify` — see `threshold.zig`'s own module doc comment for
  why BLS's linear signing equation makes this possible with NO
  interactive round (contrast the sibling `frost` module's two-round
  Schnorr threshold protocol, which needs `SigningNonces`/binding
  factors/a sorted `commitment_list` precisely because Schnorr's
  challenge is NOT linear in the secret key the way BLS's `R = SK * Q`
  is).
- **Mirrors `frost`'s trusted-dealer shape, not its code.** `frost.
  trustedDealerKeygen` (RFC 9591 Appendix C.1/C.2, already REAL in that
  module) is the direct model for `splitSecretKey`: same two-part
  construction (Shamir `secret_share_shard` + Feldman `vss_commit`),
  same "caller-supplied coefficients, not internally sampled" shape,
  same `vss_commitment[0] == group_public_key` identity
  (`groupPublicKey`). `frost.secretShareCombine`'s Lagrange-coefficient
  formula is likewise the direct model for `combineSignatures`'s —
  restated over this module's own `Fr`/`u32` indices in
  `combineSignatures`'s own doc comment. Only the FIELD/GROUP the
  arithmetic runs over differs (`Fr`/`G1`/`G2` here vs. `Secp256k1.
  scalar`/`Secp256k1` there); the crypto-core pass (2026-07-14) ported
  `frost.zig`'s already-KAT-validated Horner-evaluation and Lagrange-
  interpolation loops nearly verbatim, swapping the field/group types.
  The Lagrange coefficient formula is `frost.deriveInterpolatingValue`'s
  exact numerator/denominator-product form:
  `lambda_i = Π_{j != i} x_j * (x_j - x_i)^-1` over `Fr`
  (== `Π (0 - x_j)/(x_i - x_j)`), with the division done via `Fr.inv`.
- **Part 6 const-time choices (secret paths).**
  - `evalPolynomialAt` (Shamir share computation — the secret `sk`, the
    secret coefficients, AND the output share): pure `Fr.add`/`Fr.mul`
    Horner loop, constant-time via `std.crypto.ff`'s Montgomery
    arithmetic; loop bound and evaluation point are public.
  - `feldmanCommitCoefficient` (`[coeff]G1` for a secret coefficient):
    `g1.Jacobian.scalarMul`, Part 1's constant-time
    double-and-add-always ladder — exactly `bls_sig.skToPk`'s path.
  - `partialSign` (secret share scalar): reuses Part 4's `bls_sig.sign`
    wholesale, i.e. `g2.zig`'s constant-time `scalarMul`.
  - `combineSignatures`' Lagrange coefficients are computed from PUBLIC
    participant indices only (same reasoning as `frost.
    deriveInterpolatingValue`), but `Fr.inv` is constant-time in its
    base anyway (Fermat via `powWithEncodedExponent`, `scalar.zig`) —
    conservative even if a deployment treats the signer set as
    sensitive. The `(0 - x_j)`-product's `denominator.inv()` can never
    fail: indices are distinct nonzero `u32`s (< `r`), so every
    `x_j - x_i` factor is a nonzero `Fr` element.
  - `derivePublicKeyShare`/`groupPublicKey`/`verifyPartialSignature`
    operate on public inputs only — no const-time requirement (though
    `derivePublicKeyShare` currently reuses the constant-time
    `scalarMul` anyway; a variable-time ladder remains a possible
    follow-up optimization, per its doc comment).
- **Trusted-dealer ONLY — full DKG explicitly OUT OF SCOPE.** A
  Pedersen-style (or Gennaro–Jarecki–Krawczyk–Rabin) distributed key
  generation, where no single party ever learns `sk`, is a materially
  different, INTERACTIVE protocol (every participant deals shares of
  their OWN random polynomial to every other participant, plus a
  complaint/justification sub-round to handle a cheating dealer) — not
  a variant of `splitSecretKey`'s signature, but a distinct module/part
  with its own network-message shapes. Flagged here as an explicit
  non-goal for THIS pass, for the owner to schedule separately if a
  no-trusted-dealer deployment is ever needed.
- **The keystone test property**: `combineSignatures` of any `>= t`
  distinctly-indexed partials from a `splitSecretKey` dealing MUST equal
  `bls_sig.sign(sk, msg)` byte-for-byte, and MUST verify under
  `groupPublicKey(vvec)` via the ORDINARY `bls_sig.verify` — i.e. a
  threshold-BLS signature is not a new, parallel signature format that
  needs its own verifier; it inherits Part 4's entire verification
  surface (aggregation, KAT-pinned equation, mandatory subgroup checks)
  for free. `threshold.zig`'s wired-but-stubbed tests assert exactly
  this chain, plus Feldman VSS-consistency (`derivePublicKeyShare`
  agreeing with a dealt share's own `bls_sig.skToPk`) and subset-
  independence (any two distinct `t`-sized subsets of a `(t,n)` dealing
  combine to the identical signature).

## Threat model / limits

- **The BLS subgroup-check pitfall — this module's central threat.**
  `E(Fp)` (`G1`'s curve) and `E'(Fp2)` (`G2`'s twist) both have order
  `r * h` for a cofactor `h` much larger than 1 (`h1` ≈ 126 bits, `h2` ≈
  636 bits — `g1.zig`/`g2.zig`'s `cofactor_bytes`). A point that merely
  satisfies the curve equation (`isOnCurve`) is NOT necessarily in the
  order-`r` subgroup `G1`/`G2` a pairing or BLS signature scheme
  requires. Accepting an unchecked on-curve-but-wrong-subgroup point
  from an untrusted source (e.g. a deserialized public key or
  signature) is a REAL, historically-exploited class of BLS
  implementation bug — an attacker can submit a small-subgroup point
  and force a degenerate or predictable pairing result, potentially
  forging signatures or breaking aggregation soundness depending on the
  scheme built on top. Every wire-decode entry point in this module
  (`fromBytesUncompressed`/`fromBytesCompressed` for both `G1` and
  `G2`) checks `isOnCurve` but explicitly does NOT check
  `subgroupCheck` — that is a SEPARATE call callers crossing a trust
  boundary MUST make (documented in each decode function's doc
  comment). Whether a later part of this module's arc (e.g. Part 4's
  BLS verify) folds the subgroup check into its own point-parsing
  entry point, or requires callers to call it explicitly, is that
  part's own design decision — Part 1 only guarantees the primitive
  (`subgroupCheck`, currently a stub) exists and is documented.
- **Constant-time choices (as implemented).**
  - `Fp`/`Fr` `add`/`sub`/`neg`/`mul`/`square` delegate to
    `std.crypto.ff` — constant-time by construction.
  - `Fr.pow` uses ff's fully-constant-time `powWithEncodedExponent`
    (constant-time in base AND exponent); `Fr.inv` likewise (its
    exponent `r-2` is fixed/public, but the conservative variant costs
    nothing and the base is often secret — threshold shares). `Fp.inv`/
    `Fp.sqrt` use the PUBLIC-EXPONENT variant — the exponents (`p-2`,
    `(p+1)/4`) are fixed public constants, and ff remains constant-time
    with respect to the BASE either way.
  - `Fp2.pow` (and the test-local `Fp6`/`Fp12` pows) are square-and-
    multiply, VARIABLE-TIME in the exponent bits — used exclusively
    with fixed public exponents (sqrt exponents, Frobenius-coefficient
    exponents); documented on the function.
  - `Fp2.sqrt`/`recoverY`/`toAffine`/deserialization are public-input
    paths and may branch (they do: on QR-ness, the sort bit, and
    identity).
  - `G1`/`G2` `scalarMul`/`scalarMulBytes` are constant-time
    double-and-add-ALWAYS: one `double` + one complete `add` +
    `ctSelect` accumulator update per bit, no secret-dependent branch
    or memory access (only the scalar's LENGTH, static per call site,
    is visible). Point `add` is COMPLETE via branchless resolution:
    the general `add-2007-bl` result and `double(a)` are both fully
    computed, then the degenerate cases (identity operands, `P == Q`,
    `P == -Q`) are folded in with `ctSelect` masks — no branch on which
    case fired (the accumulator's state is secret-dependent inside
    scalarMul).
  - `ctSelect` (`Fp`, lifted componentwise to `Fp2` and points) is a
    byte-mask merge over canonical serializations — branch-free and
    index-free.
  - `Fp.random`/`Fr.random` rejection-sample (never reduce a same-width
    sample — the classic bias footgun for these field shapes).
  Verification-side operations on public data (subgroup checks of
  public points, decompression) do not need — and some do not get —
  constant time; a faster public-scalar `scalarMul` variant is a
  possible later addition if a hot public-input call site appears.
- **The pairing is VARIABLE-TIME — deliberately.** Pairings operate on
  PUBLIC inputs (public keys, signatures, commitments — every
  BLS/KZG-style consumer verifies public data with them); the Miller
  loop's affine steps branch and invert on input-derived values, the
  final exponentiation's exponents are fixed public curve constants,
  and no constant-time contortion is attempted or needed. Do NOT feed
  the pairing secret points without revisiting this (no known scheme
  does). The pairing also assumes SUBGROUP inputs for its internal
  non-degeneracy arguments (the affine steps' `catch unreachable`
  inversions): an on-curve but small-order non-subgroup point could
  panic mid-loop — a loud failure, not a silent wrong value, and
  exactly the input class the module-wide subgroup-check obligation
  already excludes at trust boundaries.
- **Hash-to-curve is VARIABLE-TIME — deliberately, same reasoning as the
  pairing.** RFC 9380 hash-to-curve's input `msg` is, in every consumer
  this module anticipates (BLS message hashing, Part 4), PUBLIC — it is
  the message being signed/verified, never a secret. §6.6.2's Simplified
  SWU map branches on `is_square`/`sgn0` (data-dependent), and §6.6.3's
  isogeny map branches on its exceptional zero-denominator case; RFC
  9380 §10.3 itself only mandates constant-time behavior for
  password-hashing use cases (§10.2), which this module does not target.
  Do NOT feed `hash_to_curve`/`encode_to_curve` a secret message without
  revisiting this (no BLS/KZG/threshold-BLS consumer does).
- **Out of scope for Parts 1-3**: BLS signatures, KZG, threshold BLS
  (Parts 4-6). Also out of scope for Part 3 specifically: any suite
  other than the two `_XMD:SHA-256_SSWU_RO_`/`_NU_` pairs BLS12-381
  actually uses (e.g. `expand_message_xof`/SHAKE variants RFC 9380
  defines for other curves) — this module implements only the SHA-256
  `expand_message_xmd` variant.
- **Out of scope for Part 4** (see "Part 4 design" above for the
  detailed reasoning): the **min-sig** ciphersuite variant (public keys
  in `G2`, signatures in `G1` — the mirror image of this file's
  min-pk); the **Basic** and **MessageAugmentation** BLS schemes
  (draft §3.1/§3.2 — alternative rogue-key mitigations to
  ProofOfPossession's, requiring either a distinct-messages precondition
  or per-message public-key augmentation this module does not
  implement); KZG and threshold BLS remain Parts 5-6, unaffected by
  Part 4's scope.
- **KZG runs on PUBLIC data — variable-time is fine (Part 5).** Every
  KZG operation's inputs (blobs, commitments, proofs, the evaluation
  point `z`) are public by construction in every consumer this module
  anticipates (EIP-4844 blob transactions are broadcast in full; a
  trusted-setup ceremony's own points are published) — there is no
  secret-scalar path through `kzg.zig` analogous to Part 4's `sk`/
  `sign`/`popProve`. AS IMPLEMENTED: `g1Msm` and the trusted-setup
  subgroup validation run on dedicated variable-time `G1` arithmetic
  (`jacAddVartime`/`jacMixedAddVartime`/`jacScalarMulVartime`,
  `kzg.zig` — branchy twins of `g1.zig`'s constant-time formulas,
  pinned equal by tests); the FFT, barycentric evaluation, batch
  inversion, and pairing checks branch freely on input-derived values;
  no constant-time contortion is attempted anywhere in `kzg.zig`,
  consistent with `pairing.zig`'s and `hash_to_curve.zig`'s own
  "public-input, variable-time" threat-model notes above. Do NOT feed
  `kzg.zig` a secret value (or reuse its vartime helpers on secret
  scalars/points) without revisiting this.
- **Trusted-setup toxic waste is out of scope.** This module embeds and
  validates a PUBLISHED ceremony's output; it has no role in, and makes
  no claim about, the ceremony's own security (the "toxic waste" secret
  `s` must have been destroyed by at least one honest participant — a
  property of the ceremony process, not verifiable from the published
  points alone). `loadTrustedSetup`'s validation (on-curve + subgroup)
  only rules out MALFORMED points, not a compromised ceremony.
- **Out of scope for Part 5**: the EIP-7594 (PeerDAS) cell/multi-proof
  scheme (`compute_cells`, `compute_cells_and_kzg_proofs`,
  `verify_cell_kzg_proof_batch`, `recover_cells_and_kzg_proofs` —
  `c-kzg-4844`'s `tests/` directory carries vectors for these too, but
  they are a materially larger, separate spec
  (`specs/fulu/polynomial-commitments-sampling.md`-family) built on TOP
  of this file's primitives, not required by EIP-4844 itself); any KZG
  ceremony/setup GENERATION code (this module only ever CONSUMES a
  published setup); threshold BLS (Part 6, unaffected by Part 5's scope).
- **Out of scope for Part 6** (see "Part 6 design" above for the
  detailed reasoning): a full Pedersen/GJKR-style **distributed key
  generation (DKG)** — this file is trusted-dealer ONLY; the **min-sig**
  ciphersuite variant, for the same reason Part 4 does not implement it
  ("Part 4 design" above); any THRESHOLD-signing-time interactivity
  (there is none needed — the entire point of BLS's linear signing
  equation, per `threshold.zig`'s module doc comment) beyond a signer
  calling `partialSign` once and a combiner calling `combineSignatures`
  once shares reach it; and identifiable-abort tooling beyond the
  single `verifyPartialSignature` check already provided (no
  complaint/blame sub-protocol, since there is no interactive round to
  misbehave in). `SecretKeyShare.scalar` carries the SAME sensitivity as
  a `bls_sig.SecretKey.scalar` (see Part 4's constant-time notes above)
  — the stubbed `evalPolynomialAt`/`feldmanCommitCoefficient` MUST,
  once filled in, preserve that discipline (`Fr`'s constant-time ops,
  `g1.zig`'s constant-time `scalarMul`); `combineSignatures`'s Lagrange
  coefficients are computed from PUBLIC indices only, so no equivalent
  constraint applies there (same reasoning as `frost.
  deriveInterpolatingValue`'s own threat-model note).

## Verification

- **KAT/oracle vectors wired** (all pass):
  - `fp.zig`: `p` is odd and exactly 381 bits (`modulus.bits()`);
    `half_p_bytes` independently recomputed as `(p-1)>>1` via raw byte
    arithmetic; zero/one round-trips; canonical-boundary rejection;
    field identities (`a·a⁻¹ = 1`, `(a+b)² = a²+2ab+b²`,
    `sqrt(a²) ∈ {±a}`, `sqrt(-1) = null` since `p ≡ 3 mod 4`);
    `pExponentBytes` reproduces `half_p_bytes`; `random` round-trips.
  - `scalar.zig`: the mirror `Fr` set, plus `reduceWide` KATs (`r → 0`,
    `r+1 → 1`, canonical passthrough, and a byte-exact
    `(2^512 - 1) mod r` vector computed independently).
  - `fp2.zig`: Karatsuba mul pinned by `u·u = -1`,
    `(1+u)(1-u) = 2`; norm-trick inv; `mulByNonresidue ==` general mul
    by `ξ`; `frobenius` is an involution AND `== pow(p)`
    (definitional); `sqrt(a²) ∈ {±a}`, `sqrt(ξ) = null` (`ξ` is a
    non-residue — verified independently), both `sqrt` branches
    exercised.
  - `fp6.zig`/`fp12.zig`: ring identities; `v³ = ξ` and `w² = v`
    (the defining tower relations, checked through general mul);
    norm-trick inversions; `frobenius == pow(p)` (definitional — this
    single test would catch ANY Frobenius-coefficient error) and
    order-6/order-12 (+`frobenius⁶ == conjugate`) checks; the derived
    `γ` coefficients byte-match independently-recomputed constants.
  - `g1.zig`/`g2.zig`: generator serialization KATs (`97f1d3a7...`
    reproduced for `G1`); full group-law suite (identity/inverse/
    commutativity/associativity/`add-vs-double` consistency);
    scalar-mul edge cases and distributivity; **independent
    cross-check vectors** for `[2]G`, `[k]G`, `[2]G2`, `[k]G2`
    (computed with a from-scratch affine-coordinates implementation —
    different algorithm family, see `NOTICE`); `subgroupCheck` accepts
    the generators and REJECTS independently-verified non-subgroup
    curve points, which `clearCofactor` then repairs (this behavioral
    pair is what caught the scaffold's wrong `h2`); decompression
    round-trips with both sort-bit values plus a `NotOnCurve` reject
    vector; uncompressed round-trips through real Jacobian arithmetic.
  - `root.zig`: every `encoded_bytes` constant matches its documented
    wire width; the dark-tests aggregator lists every submodule (Parts 1+2).
  - `fp12.zig` (Part-2 additions): `cyclotomicSquare == square` on a
    REAL cyclotomic-subgroup element (built pairing-free via the easy-
    part construction, iterated 3 steps) and on `one`;
    `frobeniusMap(k)` consistency for `k ∈ {0,1,2,3,6,12,13}`
    (including `frobeniusMap(6) == conjugate` and the mod-12
    reduction).
  - `pairing.zig` (Part 2 — ALL PASS): `Gt`/`PairingPair`/`bls_x_abs`
    sanity; `millerLoop` is definitionally `multiMillerLoop`'s `N=1`
    case; non-degeneracy (`e(G1,G2) != 1`); bilinearity
    (`e([3]P,[5]Q) == e(P,Q)^15`, additivity in both `G1` and `G2`,
    `e([7]P,Q) == e(P,[7]Q) == e(P,Q)^7`); the `pairingCheck` identity
    `e(P,Q)*e(-P,Q) == 1`, a 3-pair product-to-one check, and the
    negative (`pairingCheck` on a single non-trivial pair rejects);
    infinity-input handling (`e(O,Q) == e(P,O) == 1`, including inside
    a multi-pairing); final-exp subgroup-membership sanity
    (`e(G1,G2)^r == 1`); and the byte-exact `e(G1,G2)` KAT — the IETF
    draft-irtf-cfrg-pairing-friendly-curves-11 Appendix B optimal-ate
    vector (primary) plus the py_ecc-is-the-conjugate cross-check (see
    "Part 2 design" above).
- **Green in both modes (Parts 1-2)**: `zig build test-bls12_381`
  (Debug) and `zig build test-bls12_381 -Doptimize=ReleaseFast`
  (ReleaseFast — exercised deliberately, this repo has been bitten by
  Debug-only-green UB before) both report **all tests pass, no
  panics**. `zig fmt --check modules/bls12_381/` is clean.
- **Part 3 vectors (`hash_to_curve.zig` + the new `fp.zig`/`fp2.zig`
  helpers — ALL PASS)**: `expandMessageXmd` against RFC 9380
  **Appendix K.1** (`expand_message_xmd(SHA-256)`, DST
  `"QUUX-V01-CS02-with-expander-SHA256-128"`) for both published
  `len_in_bytes` values, `0x20` (`ell=1`) and `0x80` (`ell=4`), all 5
  messages each; `hashToFieldFp`/`hashToFieldFp2` against **Appendix
  J.9.1**/**J.10.1**'s `u[0]`/`u[1]` intermediates;
  `mapToCurveG1`/`mapToCurveG2` against J.9.1/J.10.1's **`Q0`/`Q1`**
  intermediates (localizing any bug to the SSWU+isogeny map, plus
  on-curve assertions); `hashToCurveG1`/`hashToCurveG2` against
  J.9.1/J.10.1's **final `P`** — all 5 messages each, byte-exact, so
  the entire chain (including the add + `h_eff` clear_cofactor tail)
  is pinned at every stage the RFC publishes. Property tests:
  `hashToCurve*`/`encodeToCurve*` outputs are on-curve AND pass
  `subgroupCheck` for arbitrary non-vector messages; `sswuG1`/`sswuG2`
  at `u = 0` exercise the `tv1 == 0` exceptional branch and land on
  `E1'`/`E2'` with `sgn0(y) == 0`; `Fp`/`Fp2` `inv0`/`sgn0` unit tests
  (including the `sgn0`-vs-`isLexicographicallyLargest` convention
  distinction). **Net (Parts 1-3): `zig build test-bls12_381` reports
  all tests pass, no panics, in both Debug and ReleaseFast** (`zig fmt
  --check` clean).
- **Part 4 (`bls_sig.zig`) — COMPLETE**: the scaffold-era tests (wire codecs, `keyGen`
  IKM/determinism, `skToPk`+`keyValidate`, `keyValidate` rejecting the
  identity and a non-subgroup point, `aggregate`/`aggregatePublicKeys`
  empty-set rejection, single-signature and commutativity/associativity
  properties, `coreAggregateVerify`/`aggregateVerify`/
  `fastAggregateVerify` precondition rejection, and the byte-exact
  `aggregate` KAT against `ethereum/bls12-381-tests` v0.1.2's
  `aggregate_0xabab...ab.json`); the pairing-core tests, now passing:
  `sign` against `sign_case_11b8c7cad5238946.json` (byte-exact 96-byte
  signature), `verify` accept/reject against
  `verify_valid_case_195246ee3bd3b6ec.json` (the reject case
  substitutes a genuine mismatched signature rather than the upstream
  byte-tampered vector, which fails to DESERIALIZE rather than
  exercising `verify`'s pairing check — see that test's own comment),
  `fastAggregateVerify` against
  `fast_aggregate_verify_valid_3d7576f3c0e3570a.json`, a
  self-consistent `popProve`/`popVerify` round trip (accept + a
  wrong-key proof rejected), and a self-consistent end-to-end
  `keyGen`->`skToPk`->`sign`->`verify`->`aggregate`->`aggregateVerify`
  round trip with wrong-message/wrong-key/swapped-message tampering;
  PLUS new security-check tests proving the mandatory checks FIRE:
  `verify`/`popVerify`/`aggregateVerify` reject the IDENTITY public
  key (KeyValidate fires, fail-closed, no panic); `verify`/`popVerify`/
  `aggregateVerify`/`fastAggregateVerify` reject an on-curve
  NON-SUBGROUP `G2` signature (crafted via `mapToCurveG2` WITHOUT
  cofactor clearing, its non-membership asserted as a test
  precondition); and an 8-signer `aggregateVerify` round trip (9
  pairing terms) crossing `coreAggregateVerify`'s Miller-chunk
  boundary, with a cross-boundary message-swap tamper rejected.
  **`zig build test-bls12_381` reports all tests pass, no panics, in both
  Debug and ReleaseFast** (`zig fmt --check` clean).
- **Part 5 (`kzg.zig`) — COMPLETE**: constants cross-checked
  against the fetched spec text (including an independent decimal->hex
  conversion confirming `BLS_MODULUS == scalar.r_bytes`);
  `G1_POINT_AT_INFINITY` cross-checked against `g1.zig`'s own compressed-
  identity encoding; `isPowerOfTwo`/`reverseBits`/`bitReversalPermutation`
  (the involution property, plus the spec's own order-8 worked example)
  and `computePowers` (small worked cases, `n=0`). `loadTrustedSetup` is
  exercised structurally (every slice's length) AND semantically
  (`g1_monomial[0]`/`g2_monomial[0]` equal this module's own `G1`/`G2`
  generators — the `[s^0]G = G` identity). `bytesToBlsField`/
  `validateKzgG1`/`blobToPolynomial` are exercised for both their accept
  AND reject paths (non-canonical field element, non-subgroup point).
  **Byte-exact `ethereum/c-kzg-4844` KAT coverage across every EIP-4844
  public function**, over TWO embedded blobs — the constant-2 blob
  (permutation-invariant) and the "random" blob #4 (added by the
  crypto-core pass precisely because it is permutation-SENSITIVE: it
  would catch any bit-reversal/root-of-unity error the constant blob
  cannot); `NOTICE` has every fetched file's path + sha256:
  `blob_to_kzg_commitment_case_valid_blob_{1,4}`,
  `compute_kzg_proof_case_valid_blob_1_0` and `_4_{2,5}`,
  `verify_kzg_proof_case_correct_proof_1_0`/`_4_2` (accept) +
  `verify_kzg_proof_case_incorrect_proof_0_0`/`_4_2` (reject),
  `compute_blob_kzg_proof_case_valid_blob_{1,4}` (which pin
  `computeChallenge`'s Fiat-Shamir transcript byte-exactly — the output
  proof is an opening at the challenge point),
  `verify_blob_kzg_proof_case_correct_proof_{1,4}` (accept) +
  `verify_blob_kzg_proof_case_incorrect_proof_1` (reject), and a
  two-blob batch of the pinned tuples (accept) with swapped proofs
  (reject). PROPERTY layer: the vartime `G1` helpers against the
  constant-time engine on random + every degenerate case; `g1Msm`
  against naive scalar-mul (plus empty input, zero scalars, identity
  points, a full-width scalar); the derived root of unity's order is
  exactly `2^32` and the 4096-domain is primitive/complete; `fft`
  against a naive DFT + `ifft ∘ fft == id`; TWO monomial-vs-Lagrange
  ceremony cross-checks (the `ifft`-coefficient commitment of blob #4,
  and a linear polynomial's `[c0]G + [c1][s]G1` prediction); barycentric
  evaluation of a known linear polynomial (out-of-domain formula AND
  in-domain indexing); `computeKzgProof` at an in-domain `z` (y indexes
  the polynomial, proof verifies); a full pseudorandom-blob
  commit->prove->verify round trip with wrong-`y`/wrong-proof/
  tampered-blob rejections; `batchInvInPlace`; and fail-closed
  non-canonical-field-element rejection at every bytes entry point. Plus
  the scaffold's cross-check that the constant blob's KAT commitment
  independently equals `g1.zig`'s own pre-existing `[2]G` KAT.
  **`zig build test-bls12_381` (Debug) and `zig test -OReleaseFast`
  report all tests pass, no panics** (`zig fmt --check` clean).

## Status

**Part 1 COMPLETE (crypto-core pass, 2026-07-14).** All field-tower
and group arithmetic implemented and verified — see "Design &
invariants" for the settled design decisions (canonical storage,
derived Frobenius coefficients, branchless point arithmetic, the fixed
G2 cofactor) and "Verification" for the test inventory.

**Part 2 COMPLETE (crypto-core pass, 2026-07-14).** `multiMillerLoop`
(optimal ate, affine D-type-twist line evaluation, allocation-free
batched multi-pairing), `finalExpHardPart` (Hayashida–Hayasaka–Teruya
exact-`d` cyclotomic chain), `Fp12.cyclotomicSquare` (Granger–Scott)
and `Fp12.frobeniusMap` (naive repeated Frobenius) are all real and
verified — bilinearity property suite plus the byte-exact IETF-draft
`e(G1,G2)` KAT with py_ecc conjugate cross-check. See "Part 2 design"
above for the algorithm choices and the FCKRH-vs-exact-`d` and
negative-seed-convention findings.

**Part 3 COMPLETE (crypto-core pass, 2026-07-14).** The full RFC 9380
chain — `expandMessageXmd` (§5.3.1, general `ell`), `hashToFieldFp`/
`hashToFieldFp2` (§5.2), `sswuG1`/`sswuG2` (§6.6.2 Simplified SWU, on
the new `Fp`/`Fp2` `inv0`/`sgn0` helpers), `isogenyMap11`/`isogenyMap3`
(§6.6.3 iso_map over the programmatically-sourced Appendix E.2/E.3
tables), and the `hashToCurve*`/`encodeToCurve*` compositions with
`h_eff`-based `clear_cofactor` — is implemented and byte-exact against
RFC 9380's own J.9.1/J.10.1 vectors at every published stage. See
"Part 3 design" above for the `expand_message_xmd`
reuse-vs-reimplement decision, the `Fp` wide-reduction finding, the
constant-provenance discipline, and the `h_eff`-vs-plain-cofactor
scaffold correction (the one place the scaffold's stated plan was
wrong).

**Part 4 COMPLETE (crypto-core pass, 2026-07-14).** Minimal-pubkey-size,
ProofOfPossession BLS signatures (draft-irtf-cfrg-bls-signature-05):
everything is real — wire codecs, `keyGen`, `skToPk`, `keyValidate`,
`aggregate`/`aggregatePublicKeys`, and the pairing-based `sign`,
`verify`, `coreAggregateVerify`/`aggregateVerify`,
`fastAggregateVerify`, `popProve`/`popVerify` cores, each implementing
exactly the construction its scaffold doc comment specified (see
"Part 4 design" above, including the crypto-core-pass notes on the
chunked Miller accumulator and the draft-faithful
`fastAggregateVerify`). Byte-exact against the `ethereum/bls12-381-tests`
v0.1.2 `sign`/`verify`/`aggregate`/`fast_aggregate_verify` vectors;
mandatory subgroup/`KeyValidate` checks proven to fire by dedicated
fail-closed tests. `zig build test-bls12_381`: pass, no panics,
Debug AND ReleaseFast — see "Verification" above.

**Part 5 COMPLETE (crypto-core pass, 2026-07-14).** EIP-4844 (deneb)
KZG polynomial commitments over this module's own
`G1`/`G2`/`Fr`/pairing primitives — everything is real: the
trusted-setup loader (embeds + validates the official Ethereum KZG
ceremony's `trusted_setup.txt`, on-curve + subgroup-checking all 8257
points ONCE per process, memoized — see "Part 5 design"),
blob<->polynomial (de)serialization with canonical-field-element
enforcement, `G1`/commitment/proof validation, the bit-reversal-
permutation/`computePowers` helpers, `g1Msm` (Pippenger over
variable-time public-data arithmetic), the `Fr` radix-2 NTT
(`fft`/`ifft`) with the runtime-DERIVED primitive `2^32` root of unity,
`evaluatePolynomialInEvaluationForm` (barycentric, batch-inverted
denominators, in-domain special case), `computeKzgProofImpl`
(evaluation-form quotient incl. the in-domain removable singularity),
`verifyKzgProofImpl`/`verifyKzgProofBatchImpl` (the pairing-product
checks: `e(proof, [s]₂ - [z]₂) == e(commitment - [y]₁, [1]₂)` as
`pairingCheck({P - [y]G, -G2}, {proof, [s]₂ - [z]₂})`, and its
`r`-powers random-linear-combination batch generalization), and
`computeChallenge` (Fiat-Shamir). Byte-exact against
`ethereum/c-kzg-4844` KATs across every public function, over both a
constant and a permutation-sensitive random blob, plus two
monomial-vs-Lagrange ceremony cross-checks and a full property suite —
see "Verification" above. `zig build test-bls12_381`: pass, no
panics, Debug AND ReleaseFast.

**Part 6 COMPLETE (crypto-core pass 2026-07-14).** Trusted-dealer
threshold BLS over Part 4's min-pk ciphersuite: the four crypto cores —
`evalPolynomialAt` (Shamir evaluation), `feldmanCommitCoefficient`
(Feldman commit), `derivePublicKeyShare` (Feldman
evaluate-in-the-exponent), and `combineSignatures`'s interpolation step
(Lagrange-in-the-exponent, `frost.deriveInterpolatingValue`'s exact
coefficient formula `lambda_i = Π_{j != i} x_j * (x_j - x_i)^-1` via
`Fr.inv`) — implemented per the constructions quoted in each doc
comment; see "Part 6 design" above for the const-time breakdown. The
verification chain that pins Part 6 transitively to REAL vectors: the
keystone tests assert `splitSecretKey` -> `partialSign` (`t` distinct
shares) -> `combineSignatures` equals `bls_sig.sign(sk, msg)`
BYTE-FOR-BYTE and verifies via `bls_sig.verify(groupPublicKey(vvec),
...)` — and Part 4's `sign`/`verify` are themselves pinned to
`ethereum/bls12-381-tests` v0.1.2. Also covered: subset-independence
(two different 3-of-5 subsets combine to the identical signature),
Feldman VSS-consistency (`derivePublicKeyShare(vvec, i)` ==
`bls_sig.skToPk(share_i)` for every dealt share), partial-signature
verification over a real dealing (accepts every honest partial against
its VSS-derived key share; rejects wrong-share and wrong-index
partials), the `(t=2,n=3)` and `(t=n=4)` end-to-end cases, an
over-determined combine (4 partials of a `t=3` dealing, same
signature), a below-threshold sanity check (2 partials of a `t=3`
dealing interpolate a WRONG signature that fails `bls_sig.verify` —
`t` really is the threshold), and precondition rejection
(too-few/duplicate-index/zero-index partials, invalid `(t, n,
coefficients)` splits). `zig build test-bls12_381`: pass, no
panics, Debug AND ReleaseFast.

### Backlog (deferred)

1. **The Miller-loop sparse-multiplication optimization** — a freshly
   computed line value is structurally sparse (3 nonzero `Fp2`
   coefficients out of 6 — the "014" shape `lineEval` documents); the
   implemented baseline promotes it to a dense `Fp12` and uses the
   general `mul`, correctness-first; a dedicated sparse-multiply is a
   real, well-known follow-up optimization (`multiMillerLoop`'s own
   `TODO` note), not a correctness prerequisite. Projective (inversion-
   free) Miller-loop point steps belong to the same future performance
   pass (the affine steps cost one `Fp2.inv` each — fine for
   verification workloads, the only current consumers).
2. **The fast subgroup-check / cofactor-clearing paths** (Bowe's
   untwist-Frobenius-twist technique, cited in `NOTICE`) — the simple,
   always-correct `scalarMul`-by-order/cofactor forms are implemented;
   the fast paths are marked `TODO` at their call sites.
3. **Performance**: persistent Montgomery storage for
   `Fp`/`Fr` (currently canonical-at-rest — see the convention note in
   `fp.zig`), caching the `Fp6`/`Fp12` Frobenius coefficients (currently
   recomputed per call — directly relevant to `Fp12.frobeniusMap`'s own
   deferred caching choice), and a fixed-window scalar
   multiplication (currently double-and-add-always).
4. **Optional API additions if consumers appear**: a variable-time
   public-scalar `scalarMul` for verification-side hot paths; a
   secret-exponent (ladder) `Fp2.pow`; skipping the full final
   exponentiation for `pairingCheck` specifically when a cheaper
   not-equal-to-one test suffices (`pairingCheck`'s own `TODO(fable)`
   note).
5. **Part 3 performance follow-ups** (correctness is DONE — see
   Status): the endomorphism-based FAST `clear_cofactor` evaluation
   (Scott [WB19] for `G1`, Budroni-Pintore [BP17]/Appendix G.3 for
   `G2`) — the current implementation multiplies by the same suite
   `h_eff` those methods are equivalent to, via the generic
   constant-time `scalarMulBytes` (`G2`'s 636-bit `h_eff` makes this
   the dominant hash-to-curve cost); §6.6.3's add-on-`E1'`-before-
   `iso_map` homomorphism trick (saves one isogeny evaluation per
   `hash_to_curve`); and a variable-time public-scalar multiplication
   for the `h_eff` step (hash-to-curve input is public — see the
   threat model).
6. **Part 4 follow-ups (the crypto-core pass itself is DONE — see
   Status)**: a byte-exact `keyGen` KAT if a `-05`-compatible one
   surfaces (draft Appendix B is "TBA"; see "Part 4 design"'s EIP-2333
   non-equivalence finding — until then `keyGen` is covered by the
   deterministic/round-trip tests only); the min-sig ciphersuite
   variant ("Part 4b", out of scope for now).
7. **Part 5 follow-ups (the crypto-core pass itself is DONE — see
   Status)**: MSM performance beyond baseline Pippenger (signed/NAF
   digits, affine batch-inverted bucket additions, per-window
   parallelism — the implemented bucket method is already `O(n·256/c)`
   but each addition still pays full Jacobian arithmetic); a
   precomputed/cached bit-reversal-permuted Lagrange basis on
   `TrustedSetup` (currently re-permuted per commitment/proof — a cheap
   copy, but avoidable); EIP-7594/PeerDAS cell proofs (explicitly out
   of scope for Part 5 itself, `SPEC.md`'s threat model — `fft`/`ifft`
   are already in place for it).
8. **Part 6 crypto-core pass (NOT started — see Status)**: fill in
   `evalPolynomialAt` (Shamir), `feldmanCommitCoefficient` (Feldman
   commit), `derivePublicKeyShare` (Feldman evaluate-in-the-exponent),
   and `combineSignatures`'s Lagrange-in-the-exponent step — each
   stub's doc comment in `threshold.zig` already quotes the exact
   construction, and `frost.zig`'s already-implemented Horner-
   evaluation/Lagrange-interpolation loops are a direct porting
   reference (same shape, different field/group — see "Part 6 design").
   Once filled in, a full Pedersen/GJKR-style DKG (explicitly out of
   scope for this trusted-dealer pass, "Part 6 design"/"Out of scope
   for Part 6" above) would be a genuinely separate follow-up
   module/part, not an extension of `threshold.zig` itself.

## Anchoring

**Anchor grade:** class B · oracle MIXED

- **Class B** — published cryptographic or algorithmic construction with published vectors.
- **Oracle MIXED** — anchored for some paths, self for others — the evidence below names which.

**What the tests actually contain.** IETF draft/py_ecc byte-exact KATs; KeyGen deliberately NOT KAT-verified, self round-trip only

**How it got there.** No external oracle exists for what remains. VERIFIED: we do BLS-draft KeyGen, EIP-2333 is a DIFFERENT algorithm; no KAT exists (draft-07 App.B = TBA)

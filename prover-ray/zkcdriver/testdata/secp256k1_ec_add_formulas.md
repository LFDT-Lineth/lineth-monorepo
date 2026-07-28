# secp256k1 EC addition formulas and edge cases

This note is for implementing and checking elliptic-curve addition over the secp256k1 base field in ZKC/prover-ray.

References:

- secp256k1 parameters: <https://std.neuromancer.sk/secg/secp256k1/>
- Short-Weierstrass group law: <https://www.hyperelliptic.org/EFD/g1p/auto-shortw.html>
- Homogeneous projective complete addition (a=0): <https://hyperelliptic.org/EFD/g1p/auto-shortw-projective.html#addition-add-2015-rcb>
- Homogeneous projective doubling (used here, `dbl-1998-cmo`): <https://hyperelliptic.org/EFD/g1p/auto-shortw-projective.html#doubling-dbl-1998-cmo>
- Cohen–Miyaji–Ono, *Efficient elliptic curve exponentiation using mixed coordinates* (ASIACRYPT 1998, LNCS 1514, pp. 51–65) — source of the doubling formula: <https://doi.org/10.1007/3-540-49649-1_6>
- Jacobian formulas for `a = 0` short-Weierstrass curves: <https://www.hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html>
- Complete addition formulas for prime order elliptic curves (Renes–Costello–Batina 2015): <https://eprint.iacr.org/2015/1060>
- gnark-crypto secp256k1 API notes: <https://pkg.go.dev/github.com/consensys/gnark-crypto/ecc/secp256k1>

## Curve parameters

secp256k1 is the short-Weierstrass curve:

```text
E: y^2 = x^3 + 7 over Fp
```

where:

```text
p = 0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f
a = 0
b = 7
n = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141
h = 1
```

All additions, subtractions, multiplications, divisions, and equalities below are modulo `p`, unless explicitly stated otherwise.

## Point representation

Mathematically, the group identity is the point at infinity:

```text
O
```

In projective coordinates this is commonly represented as:

```text
O = (0 : 1 : 0)
```

For a circuit, prefer an explicit flag:

```text
Point = (is_inf, x, y)
```

with this validity rule:

```text
is_inf = 1  => point is O, x/y are ignored or conventionally set to 0
is_inf = 0  => 0 <= x < p, 0 <= y < p, and y^2 = x^3 + 7 mod p
```

Some libraries, including gnark-crypto affine points, encode infinity as `(0, 0)`. That works for secp256k1 because `(0, 0)` is not on `y^2 = x^3 + 7`, but an explicit flag is safer in a circuit because it avoids mixing encoding conventions with field validity.

## Negation and subtraction

For a finite point:

```text
P  = (x, y)
-P = (x, -y) = (x, p - y) mod p
```

Special cases:

```text
-O = O
if y = 0, then -P = P
```

For valid secp256k1 subgroup points, a finite `y = 0` point should not occur because it would be a non-trivial point of order 2, while secp256k1 has cofactor `h = 1` and odd group order `n`. Still, robust addition logic should handle the mathematical case.

Subtraction is addition with negation:

```text
P - Q = P + (-Q)
```

## Complete affine addition case table

Let:

```text
P = (p_inf, x1, y1)
Q = (q_inf, x2, y2)
R = P + Q = (r_inf, x3, y3)
```

The group-law cases are:

| Case | Condition | Result |
|---|---|---|
| invalid input | finite `P` or `Q` is non-canonical or not on curve | reject/fail |
| both infinity | `p_inf = 1` and `q_inf = 1` | `R = O` |
| left infinity | `p_inf = 1` and `q_inf = 0` | `R = Q` |
| right infinity | `p_inf = 0` and `q_inf = 1` | `R = P` |
| inverse / vertical line | finite points and `x1 = x2`, `y1 + y2 = 0 mod p` | `R = O` |
| doubling | finite points and `x1 = x2`, `y1 = y2`, `y1 != 0` | use doubling formula |
| distinct affine addition | finite points and `x1 != x2` | use addition formula |

For finite, valid curve points over an odd prime field, if `x1 = x2`, then necessarily `y2 = y1` or `y2 = -y1`. So after on-curve validation, the `x1 = x2` cases split into only doubling or inverse.

## Distinct finite-point addition

Applicable when:

```text
P != O
Q != O
x1 != x2
```

Slope:

```text
lambda = (y2 - y1) / (x2 - x1)
       = (y2 - y1) * inv(x2 - x1)
```

Output:

```text
x3 = lambda^2 - x1 - x2
y3 = lambda * (x1 - x3) - y1
```

Equivalent EFD-style form:

```text
x3 = (y2 - y1)^2 / (x2 - x1)^2 - x1 - x2
y3 = lambda * (x1 - x3) - y1
```

Circuit-friendly check when `lambda` is a witness:

```text
dx = x2 - x1 mod p
dy = y2 - y1 mod p

lambda * dx = dy mod p
x3 = lambda^2 - x1 - x2 mod p
y3 = lambda * (x1 - x3) - y1 mod p
```

This avoids doing a modular inverse inside the circuit. It must be gated by the condition `x1 != x2`, otherwise `dx = 0` makes the slope check meaningless.

## Doubling

Applicable when:

```text
P != O
Q != O
x1 = x2
y1 = y2
y1 != 0
```

For a short-Weierstrass curve:

```text
lambda = (3*x1^2 + a) / (2*y1)
```

For secp256k1, `a = 0`, so:

```text
lambda = 3*x1^2 / (2*y1)
```

Output:

```text
x3 = lambda^2 - 2*x1
y3 = lambda * (x1 - x3) - y1
```

## Doubling with `y = 0`

Applicable when:

```text
P = Q
y1 = 0
```

The tangent is vertical, so:

```text
P + P = O
```

As noted above, this should not happen for valid finite secp256k1 subgroup points, but it is part of the complete affine group law and should be accounted for if the circuit handles arbitrary curve-like inputs.

## Inverse / vertical-line case

Applicable when:

```text
P != O
Q != O
x1 = x2
y1 + y2 = 0 mod p
```

Then:

```text
Q = -P
P + Q = O
```

This includes the `P = Q` and `y1 = 0` doubling-to-infinity case.

## Infinity cases

Infinity is the identity element:

```text
O + O = O
O + Q = Q
P + O = P
```

For circuit outputs with an explicit infinity flag:

```text
O + O:
  r_inf = 1

O + Q:
  r_inf = q_inf
  x3 = x2
  y3 = y2

P + O:
  r_inf = p_inf
  x3 = x1
  y3 = y1
```

If using `(0,0)` as an infinity encoding, constrain:

```text
r_inf = 1 => x3 = 0 and y3 = 0
```

Do not use the affine slope formulas when either input is infinity.

## Projective coordinates

### Why projective for ZKC

A field inversion via Fermat's little theorem costs ~512 `mulmod_256` calls (256 squarings + up to 256 multiplications). In affine addition, each of the two `lambda = dy/dx` divisions forces an inversion. In a scalar multiplication with `k` doublings and additions, that is O(k) inversions, i.e. O(512k) `mulmod_256` calls just for the denominators.

Homogeneous projective coordinates defer all inversions to a single final conversion. Every intermediate step uses only `mulmod_256`, `addmod_256`, and `submod_256`, with one `invmod_256` at the very end.

### Representation

A projective point `(X : Y : Z)` with `Z ≠ 0` represents the affine point `(X/Z, Y/Z)`.

The secp256k1 curve equation `y² = x³ + 7` homogenizes to:

```text
Y² Z = X³ + 7 Z³
```

The point at infinity is represented as `O = (0 : 1 : 0)`, which satisfies the equation (`0 = 0 + 0`). No sentinel or flag is needed.

To convert back to affine at the end:

```text
if Z = 0:  result is O
else:       x = X · Z⁻¹ mod p
            y = Y · Z⁻¹ mod p
```

Two `mulmod_256` calls plus one `invmod_256`.

Note: projective representations are not unique — `(X:Y:Z)` and `(λX:λY:λZ)` are the same point. Never compare points component-wise; equality is:

```text
(X1:Y1:Z1) = (X2:Y2:Z2)  ⟺  X1·Z2 = X2·Z1  and  Y1·Z2 = Y2·Z1
```

---

### Complete unified addition (Renes–Costello–Batina 2015, Algorithm 7 for a = 0)

This formula is **correct for all input pairs** without any case analysis: distinct points, doubling, `P + O`, `O + O`, and `P + (−P)` all produce the right output. The curve must have prime order and `a = 0`, both of which secp256k1 satisfies.

Reference: Renes–Costello–Batina, *Complete addition formulas for prime order elliptic curves*, Algorithm 7 (33 steps, 12M + 2m_{b3} + 19a).

Let `b3 = 3 · b = 3 · 7 = 21` for secp256k1.

**Inputs:** `P1 = (X1 : Y1 : Z1)`, `P2 = (X2 : Y2 : Z2)`.
**Output:** `P3 = (X3 : Y3 : Z3) = P1 + P2`.

```text
 1.  t0 ← X1 · X2                          M
 2.  t1 ← Y1 · Y2                          M
 3.  t2 ← Z1 · Z2                          M
 4.  t3 ← X1 + Y1                          add
 5.  t4 ← X2 + Y2                          add
 6.  t3 ← t3 · t4                          M      // (X1+Y1)(X2+Y2)
 7.  t4 ← t0 + t1                          add
 8.  t3 ← t3 − t4                          sub    // t3 = X1Y2 + X2Y1
 9.  t4 ← Y1 + Z1                          add
10.  X3 ← Y2 + Z2                          add
11.  t4 ← t4 · X3                          M      // (Y1+Z1)(Y2+Z2)
12.  X3 ← t1 + t2                          add
13.  t4 ← t4 − X3                          sub    // t4 = Y1Z2 + Y2Z1
14.  X3 ← X1 + Z1                          add
15.  Y3 ← X2 + Z2                          add
16.  X3 ← X3 · Y3                          M      // (X1+Z1)(X2+Z2)
17.  Y3 ← t0 + t2                          add
18.  Y3 ← X3 − Y3                          sub    // Y3 = X1Z2 + X2Z1
19.  X3 ← t0 + t0                          add
20.  t0 ← X3 + t0                          add    // t0 = 3·(X1·X2)
21.  t2 ← b3 · t2                          ×b3    // t2 = 21·(Z1·Z2)
22.  Z3 ← t1 + t2                          add
23.  t1 ← t1 − t2                          sub
24.  Y3 ← b3 · Y3                          ×b3    // Y3 = 21·(X1Z2+X2Z1)
25.  X3 ← t4 · Y3                          M
26.  t2 ← t3 · t1                          M
27.  X3 ← t2 − X3                          sub    // X3 done
28.  Y3 ← Y3 · t0                          M
29.  t1 ← t1 · Z3                          M
30.  Y3 ← t1 + Y3                          add    // Y3 done
31.  t0 ← t0 · t3                          M
32.  Z3 ← Z3 · t4                          M
33.  Z3 ← Z3 + t0                          add    // Z3 done
```

**Operation count (complete unified addition, secp256k1):**

| Type | Count | Symbol in paper |
|---|---|---|
| Field multiplications (var × var) | 12 | 12M |
| Multiplications by b3 = 21 | 2 | 2m_{b3} |
| Field additions / subtractions | 19 | 19a |

Implementation options for the two `×b3 = ×21` steps:
- Keep as `mulmod(21, v, p)`: total **14M + 19 add/sub**
- Replace each with 5 addmods (×21 = ×16 + ×4 + ×1): total **12M + 29 add/sub**

Since `addmod_256` is cheaper than `mulmod_256` in circuit terms, prefer the second option.

**Correctness on special cases (no branches needed):**

| Case | What happens in the formula |
|---|---|
| `P + O` where `O = (0:1:0)` | Produces `(X1:Y1:Z1)` scaled by `Y2`; same projective class as P |
| `P + (-P)` | X3 and Z3 both collapse to 0; output is `(0:Y3:0) = O` |
| `P + P` (doubling) | Same formula, naturally handles the tangent case |

#### ZKC register pseudocode (complete addition, no variable overloading)

Every register is written exactly once. Inputs `X1 Y1 Z1 X2 Y2 Z2` are read-only.
All ops are mod p. `b3 = 21`; `×21` is expanded as 5 addmods (`×2→×4→×8→×16→+×4→+×1`).

```text
// products of matching coordinates
xx       = mul(X1, X2)              // X1·X2
yy       = mul(Y1, Y2)              // Y1·Y2
zz       = mul(Z1, Z2)              // Z1·Z2

// X1Y2+X2Y1  (Karatsuba)
sx1y1    = add(X1, Y1)
sx2y2    = add(X2, Y2)
pxy      = mul(sx1y1, sx2y2)        // (X1+Y1)(X2+Y2)
diag_xy  = add(xx, yy)              // X1X2 + Y1Y2
cxy      = sub(pxy, diag_xy)        // X1Y2 + X2Y1

// Y1Z2+Y2Z1  (Karatsuba)
sy1z1    = add(Y1, Z1)
sy2z2    = add(Y2, Z2)
pyz      = mul(sy1z1, sy2z2)        // (Y1+Z1)(Y2+Z2)
diag_yz  = add(yy, zz)              // Y1Y2 + Z1Z2
cyz      = sub(pyz, diag_yz)        // Y1Z2 + Y2Z1

// X1Z2+X2Z1  (Karatsuba)
sx1z1    = add(X1, Z1)
sx2z2    = add(X2, Z2)
pxz      = mul(sx1z1, sx2z2)        // (X1+Z1)(X2+Z2)
diag_xz  = add(xx, zz)              // X1X2 + Z1Z2
cxz      = sub(pxz, diag_xz)        // X1Z2 + X2Z1

// 3·(X1X2)
xx2      = add(xx, xx)
xx3      = add(xx2, xx)             // 3·X1X2

// 21·(Z1Z2)
zz2      = add(zz, zz)
zz4      = add(zz2, zz2)
zz8      = add(zz4, zz4)
zz16     = add(zz8, zz8)
zz20     = add(zz16, zz4)
zz21     = add(zz20, zz)            // 21·Z1Z2

// 21·(X1Z2+X2Z1)
cxz2     = add(cxz, cxz)
cxz4     = add(cxz2, cxz2)
cxz8     = add(cxz4, cxz4)
cxz16    = add(cxz8, cxz8)
cxz20    = add(cxz16, cxz4)
cxz21    = add(cxz20, cxz)          // 21·(X1Z2+X2Z1)

// paper steps 22–23
pre_z3   = add(yy, zz21)            // Y1Y2 + 21·Z1Z2
t1       = sub(yy, zz21)            // Y1Y2 - 21·Z1Z2

// X3
m_a      = mul(cyz, cxz21)          // (Y1Z2+Y2Z1) · 21·(X1Z2+X2Z1)
m_b      = mul(cxy, t1)             // (X1Y2+X2Y1) · (Y1Y2-21·Z1Z2)
X3       = sub(m_b, m_a)

// Y3
m_c      = mul(cxz21, xx3)          // 21·(X1Z2+X2Z1) · 3·(X1X2)
m_d      = mul(t1, pre_z3)          // (Y1Y2-21·Z1Z2) · (Y1Y2+21·Z1Z2)
Y3       = add(m_d, m_c)

// Z3
m_e      = mul(xx3, cxy)            // 3·(X1X2) · (X1Y2+X2Y1)
m_f      = mul(pre_z3, cyz)         // (Y1Y2+21·Z1Z2) · (Y1Z2+Y2Z1)
Z3       = add(m_f, m_e)
```

Operation count: **12 mul + 29 add/sub** (12 `mul` lines; 19 semantic add/sub + 10 from the two ×21 chains).

---

### Dedicated doubling formula (for scalar multiplication)

When implementing double-and-add scalar multiplication, every bit of the scalar produces one doubling and at most one addition. Since doublings dominate (log₂ scalar ≈ 256 iterations), a dedicated doubling formula that is cheaper than the general addition is worth using.

This is the standard-projective doubling of **Cohen–Miyaji–Ono 1998**, catalogued
in the EFD as [`dbl-1998-cmo`](https://hyperelliptic.org/EFD/g1p/auto-shortw-projective.html#doubling-dbl-1998-cmo).
The general CMO formula is `w = a·Z1² + 3·X1²`; for secp256k1 (`a = 0`) the `a·Z1²`
term drops, giving `w = 3·X1²`. All other steps are identical to the EFD entry.

**Inputs:** `P = (X1 : Y1 : Z1)`, `P ≠ O`.
**Output:** `(X3 : Y3 : Z3) = 2P`.

```text
CMO 1998 (dbl-1998-cmo), specialized to a = 0. The affine tangent slope
λ = 3x² / (2y) lifts to W = 3·X1² (numerator), S = Y1·Z1 (half-denominator):

W  = 3·X1²                    // tangent numerator (a=0 drops the a·Z1² term)
S  = Y1·Z1
B  = X1·Y1·S  = X1·Y1²·Z1
H  = W² − 8·B
X3 = 2·H·S
Y3 = W·(4·B − H) − 8·Y1²·S²
Z3 = 8·S³
```

Step-by-step with temporaries:

```text
t0 = mulmod(X1, X1, p)         // X1²
t1 = mulmod(Y1, Y1, p)         // Y1²
S  = mulmod(Y1, Z1, p)         // Y1·Z1
W  = addmod(addmod(t0,t0,p), t0, p)  // 3·X1²  (addmods, no mulmod)
t2 = mulmod(X1, Y1, p)         // X1·Y1
B  = mulmod(t2, S, p)          // X1·Y1·S
t3 = mulmod(W, W, p)           // W²
H  = submod(t3, addmod(addmod(addmod(addmod(addmod(addmod(B,B,p),B,p),B,p),B,p),B,p),addmod(B,B,p),p), p)
                                // H = W² − 8B  (8B via 3 doublings = 3 addmods)
t4 = mulmod(H, S, p)           // H·S
X3 = addmod(t4, t4, p)         // 2·H·S
t5 = mulmod(S, S, p)           // S²
t6 = mulmod(t1, t5, p)         // Y1²·S²
// 4B − H:
t7 = submod(addmod(addmod(B,B,p),addmod(B,B,p),p), H, p)   // 4B − H
t8 = mulmod(W, t7, p)          // W·(4B − H)
// 8·Y1²·S²: three doublings of t6
t9 = addmod(addmod(addmod(t6,t6,p),addmod(t6,t6,p),p),addmod(addmod(t6,t6,p),addmod(t6,t6,p),p),p)
Y3 = submod(t8, t9, p)         // W·(4B−H) − 8·Y1²·S²
t10 = mulmod(t5, S, p)         // S³
Z3  = addmod(addmod(addmod(t10,t10,p),addmod(t10,t10,p),p),addmod(addmod(t10,t10,p),addmod(t10,t10,p),p),p)
                                // 8·S³ (three doublings)
```

**Operation count (dedicated doubling, secp256k1):**

| Operation | Count | Notes |
|---|---|---|
| `mulmod_256` (var × var) | 9 | t0,t1,S,t2,B,W²,H·S,S²,Y1²S²,W·t7,S³ = 11 raw; W=3·t0 and X3=2·t4 use addmods |
| `addmod_256` / `submod_256` | ~15 | constants ×3, ×4, ×8 all via add chains |

More precisely: 11M (t0, t1, S, t2, B, t3=W², t4=H·S, t5=S², t6=t1·t5, t8=W·t7, t10=t5·S) + ~15 add/sub.

Compared to complete addition (12M + 29 add/sub), doubling costs 11M + 15 add/sub — a modest but real saving per scalar bit.

**Edge case:** `Z3 = 8·S³ = 0` iff `S = Y1·Z1 = 0`. For a valid finite secp256k1 point, `Z1 ≠ 0` (else it is already O), and `Y1 = 0` cannot happen for a prime-order curve (such a point would have order 2). So for valid inputs, this formula never produces infinity, and no branch is needed.

#### ZKC register pseudocode (doubling, no variable overloading)

Every register is written exactly once. Inputs `X1 Y1 Z1` are read-only. All ops are mod p.

```text
// basic products
x1sq     = mul(X1, X1)              // X1²
y1sq     = mul(Y1, Y1)              // Y1²
S        = mul(Y1, Z1)              // Y1·Z1

// W = 3·X1²  (tangent numerator, a=0)
x1sq2    = add(x1sq, x1sq)          // 2·X1²
W        = add(x1sq2, x1sq)         // W = 3·X1²

// B = X1·Y1·S
x1y1     = mul(X1, Y1)              // X1·Y1
B        = mul(x1y1, S)             // B = X1·Y1·S

// H = W² - 8B
W2       = mul(W, W)                // W²
B2       = add(B, B)                // 2B
B4       = add(B2, B2)              // 4B
B8       = add(B4, B4)              // 8B
H        = sub(W2, B8)              // H = W² - 8B

// X3 = 2·H·S
HS       = mul(H, S)                // H·S
X3       = add(HS, HS)              // X3 = 2·H·S

// Y3 = W·(4B - H) - 8·Y1²·S²
S2       = mul(S, S)                // S²
y1sqS2   = mul(y1sq, S2)            // Y1²·S²
B4mH     = sub(B4, H)               // 4B - H  (reuses B4 register, read-only)
WB4mH    = mul(W, B4mH)             // W·(4B - H)
y1sqS2_2 = add(y1sqS2, y1sqS2)     // 2·Y1²·S²
y1sqS2_4 = add(y1sqS2_2, y1sqS2_2) // 4·Y1²·S²
y1sqS2_8 = add(y1sqS2_4, y1sqS2_4) // 8·Y1²·S²
Y3       = sub(WB4mH, y1sqS2_8)    // Y3 = W·(4B-H) - 8·Y1²·S²

// Z3 = 8·S³
S3       = mul(S2, S)               // S³
S3_2     = add(S3, S3)              // 2·S³
S3_4     = add(S3_2, S3_2)         // 4·S³
Z3       = add(S3_4, S3_4)          // Z3 = 8·S³
```

Operation count: **11 mul + 15 add/sub** (11 `mul` lines; 12 add + 3 sub).

---

### Summary: operation counts for ZKC implementation

| Formula | mulmod_256 | addmod/submod | Notes |
|---|---|---|---|
| Affine distinct-point add | 1 (inv) + 3 | ~6 | inv costs ~512 mulmod |
| Affine doubling | 1 (inv) + 4 | ~6 | inv costs ~512 mulmod |
| **Projective complete add (RCB a=0)** | **12** | **~29** | no branches, handles all cases |
| **Projective doubling (a=0)** | **11** | **~15** | only for `P = Q`, use in scalar mul |

For scalar multiplication over a 256-bit scalar: ~256 doublings + ~128 additions on average.
- Affine: (256 + 128) × ~515 mulmod ≈ **198,000 mulmod calls**
- Projective: 256 × 11 + 128 × 12 + 1 (final inv) ≈ **4,353 mulmod calls** + ~9,000 addmod

The projective approach is roughly **45× cheaper** in `mulmod_256` calls for scalar multiplication.

---

### Implementation notes for ZKC

- All multiplications by small constants (2, 3, 4, 8, 21) should use `addmod_256` chains rather than `mulmod_256`. Example: `3·x = addmod(addmod(x,x,p),x,p)`. This reduces the dominant `mulmod_256` count.
- For `b3 = 21 = 16 + 4 + 1`: compute `x2=addmod(x,x,p)`, `x4=addmod(x2,x2,p)`, `x8=addmod(x4,x4,p)`, `x16=addmod(x8,x8,p)`, then `addmod(addmod(x16,x4,p),x,p)` — 5 addmods, 0 mulmod.
- Verify intermediate values against a reference implementation (e.g. Python with the secp256k1 field) before running the full prover pipeline.
- Point equality in projective coordinates requires cross-multiplication, not componentwise comparison.
- After scalar multiplication, a single `invmod_256` converts the result back to affine.

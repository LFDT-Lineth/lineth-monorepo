# secp256k1 EC addition formulas and edge cases

This note is for implementing and checking elliptic-curve addition over the secp256k1 base field in ZKC/prover-ray.

References:

- secp256k1 parameters: <https://std.neuromancer.sk/secg/secp256k1/>
- Short-Weierstrass group law: <https://www.hyperelliptic.org/EFD/g1p/auto-shortw.html>
- Jacobian formulas for `a = 0` short-Weierstrass curves: <https://www.hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html>
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

Circuit-friendly check when `lambda` is a witness:

```text
den = 2*y1 mod p
num = 3*x1^2 mod p

lambda * den = num mod p
x3 = lambda^2 - 2*x1 mod p
y3 = lambda * (x1 - x3) - y1 mod p
```

This must be gated by `den != 0`. If `den = 0`, the doubling result is infinity.

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

## Output validity

For finite output:

```text
r_inf = 0 => 0 <= x3 < p, 0 <= y3 < p, and y3^2 = x3^3 + 7 mod p
```

If the addition formula constraints are correctly enforced and the inputs are valid, the output should land on the curve automatically. Still, explicitly checking the output point is useful during early circuit development because it catches selector/gating mistakes.

## Circuit selectors for a complete add gadget

A complete affine-add gadget can be organized with boolean selectors:

```text
s_oo       = p_inf * q_inf
s_o_q      = p_inf * (1 - q_inf)
s_p_o      = (1 - p_inf) * q_inf
s_inverse  = finite inputs, x1 = x2, y1 + y2 = 0 mod p
s_double   = finite inputs, x1 = x2, y1 = y2, y1 != 0
s_add      = finite inputs, x1 != x2
```

Enforce:

```text
each selector is boolean
exactly one selector is active
every formula constraint is multiplied/gated by its selector
```

Result flag:

```text
r_inf = s_oo + s_inverse
```

plus the appropriate identity-copy cases:

```text
s_o_q => R = Q
s_p_o => R = P
```

and formula cases:

```text
s_add    => distinct-add constraints
s_double => doubling constraints
```

## Modular arithmetic checks without a separate add/sub native query

`mulmod_256` is special because prover-ray lowers it to a `NonNative` modular multiplication query. Modular addition/subtraction can usually be checked with split-register arithmetic plus small carry witnesses.

For bounded field elements `a, b, c < p`:

Addition:

```text
c = a + b mod p
a + b = c + k*p
k in {0, 1}
```

Subtraction:

```text
c = a - b mod p
a = b + c - k*p
```

Equivalently, avoid signed arithmetic by choosing the side with the carry:

```text
a + k*p = b + c
```

where `k` is small and range-checked. Depending on which side is larger, this `k` is typically `0` or `1`.

For larger expressions, use a wider integer equation:

```text
lhs + k_l*p = rhs + k_r*p
```

with `lhs`, `rhs`, and carries bounded tightly enough that the equality proves the intended congruence over integers, not only over the native KoalaBear field.

## Important implementation traps

- Validate finite input coordinates are canonical `< p`; otherwise different `u256` encodings can represent the same field element modulo `p`.
- Validate finite inputs are on curve before using `x1 = x2` to distinguish inverse vs doubling.
- Never apply the distinct-add slope formula when `x1 = x2`.
- Never apply the doubling slope formula when `y1 = 0`.
- If `lambda` is a witness, always prove the denominator relation, e.g. `lambda * dx = dy mod p`.
- If using private witnesses for `dx`, `dy`, carries, or `lambda`, constrain each witness to the relation it claims to represent.
- Infinity needs a first-class flag or a very carefully documented sentinel encoding.
- For ECDSA later, remember that point coordinates live modulo `p`, while scalars/signature arithmetic live modulo the group order `n`; these are different fields.

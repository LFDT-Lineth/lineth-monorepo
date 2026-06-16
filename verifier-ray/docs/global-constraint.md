# Global Constraint: What the Verifier Checks

**Inputs (trusted after PCS):**
```
witness_claims[i]  = C_i(r)   // each unique column, evaluated at r
quotient_claims[k] = Q_k(r)   // each quotient share, evaluated at r
merge_coin,  r                 // Fiat-Shamir coins
n                              // module size
```

**Step 1 — evaluate each vanishing expression at r:**
```
V_j(r) = eval_expression(V_j, witness_claims)

// e.g. V = A·B     →  V(r) = witness_claims[A] * witness_claims[B]
// e.g. V = C·(C−1) →  V(r) = witness_claims[C] * (witness_claims[C] − 1)
```

**Step 2 — apply cancellation:**
```
P_j(r) = V_j(r) · ∏_{k ∈ cancelled_j} (r − ω^k)
```

**Step 3 — aggregate with merge coin:**
```
P_agg(r) = Σ_j  merge_coin^j · P_j(r)
```

**Step 4 — reconstruct Q(r) from shares:**
```
Q(r) = Q_0(r) + r^n·Q_1(r) + r^{2n}·Q_2(r) + …
```

**Step 5 — identity check:**
```
assert  P_agg(r)  ==  (r^n − 1) · Q(r)
```
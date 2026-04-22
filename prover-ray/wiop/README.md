# wiop — Wizard IOP

`wiop` is the core framework for describing and executing Interactive Oracle
Proof (IOP) protocols. It underpins the ZK-EVM prover by providing a
declarative language for expressing constraints and a runtime for evaluating
them.

## Design

### Specification vs execution

The package splits cleanly into two concerns:

- **Specification** (`System`, `Module`, `Round`, `Column`, `Query`, …): a
  static, immutable description of the protocol graph built once at setup time.
- **Execution** (`Runtime`): a mutable snapshot of one protocol run, binding
  concrete field-element assignments to the symbolic objects.

This separation means the same `System` can be reused across many proving
sessions without re-registering any queries or columns.

### Symbolic expressions

Constraints are written as `Expression` trees, not closures over concrete
slices. The tree is compiled to a bytecode program the first time it is
evaluated (`expression_compiler.go`) and then cached on the node. Subsequent
evaluations skip the compilation step entirely.

The scalar/vector distinction is encoded in the type system: `FieldPromise` for
scalars, `VectorPromise` for row vectors. Mixing them is caught at construction
time by arity/module invariant checks rather than at evaluation time.

### Protocol lifecycle

```
System definition   │  interactive rounds
                    │
PrecomputedRound    │  Round 0     Round 1     …  Round N
  (offline data)    │  prover      verifier
                    │  assigns     draws coins
                    │  columns     ↓
                    │              prover assigns next columns …
```

`Runtime.AdvanceRound` closes the current round:

1. All oracle/public column assignments are fed into the Fiat-Shamir state.
2. Public cell values are fed into the Fiat-Shamir state.
3. The runtime moves to the next round.
4. One `CoinField` value is derived per coin declared in the new round.

This makes the interactive protocol non-interactive via the Fiat-Shamir
transform, with the transcript hash maintained inside `Runtime`.

### Visibility

`Visibility` controls both query eligibility and Fiat-Shamir feeding:

| Level      | Usable in active queries | Fed to Fiat-Shamir | Verifier-visible |
|------------|--------------------------|---------------------|-----------------|
| Internal   | No                       | No                  | No              |
| Oracle     | Yes                      | Yes (column hash)   | No              |
| Public     | Yes                      | Yes (raw values)    | Yes             |

An expression's effective visibility is the minimum of all its leaf
visibilities. A query containing any Internal leaf cannot remain active; it
must be reduced (compiled away) before verification.

### Query compilation model

Queries carry an `IsReduced` / `MarkAsReduced` flag. A compiler pass that
rewrites a high-level query into simpler ones marks the original as reduced.
Downstream passes and the verifier skip reduced queries. This enables
incremental, composable compilation without requiring a mutable query list.

`GnarkCheckableQuery` is the subset of queries that can be verified inside a
gnark arithmetic circuit. Queries that cannot be expressed in-circuit (e.g.
`TableRelation`, `LogDerivativeSum`) must be compiled away before the gnark
layer runs.

### Object identity

Every registered object (`Column`, `Cell`, `CoinField`) carries a
`*ContextFrame`: a node in a slash-separated label tree rooted at the
`System`'s label. The path is human-readable and used in error messages, while
the compact `ObjectID` (a 64-bit `uint64` encoding kind + slot + position) is
used in the `Runtime`'s maps for O(1) lookup.

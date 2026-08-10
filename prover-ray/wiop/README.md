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

1. The size of every dynamic module is fed into the Fiat-Shamir state.
2. If the round carries a commitment, that commitment is fed into the
   Fiat-Shamir state.
3. Cell values are fed into the Fiat-Shamir state.
4. The runtime moves to the next round.
5. One `CoinField` value is derived per coin declared in the new round.

This makes the interactive protocol non-interactive via the Fiat-Shamir
transform, with the transcript hash maintained inside `Runtime`.

### How columns enter the transcript

Columns are never transported in the `Proof` and their raw values are never
absorbed into Fiat-Shamir. Binding them to the transcript is the commitment
scheme's job: the `pcs` pass marks each round that owns columns with
`Round.HasCommitment` and registers a prover action that FRI-commits the
round's columns, and `AdvanceRound` absorbs that single commitment in place of
the columns themselves.

A consequence worth keeping in mind: a protocol that has *not* been through the
`pcs` pass has no witness binding at all. Its coins do not depend on the
columns, and the verifier — which holds no column data — cannot detect a
tampered witness. Any soundness test must therefore run the `pcs` pass (or seed
the transcript through a `PreSamplingHook`); otherwise the challenges are
constants, and the very first coin drawn from an untouched transcript is zero.

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

// Package wiop implements the Wizard IOP framework: a domain-specific language
// for describing and executing interactive oracle proof protocols over the
// KoalaBear field.
//
// # Overview
//
// A protocol is described as a [System]. The system owns a [PrecomputedRound]
// for offline data, a sequence of interactive [Round]s, and a set of [Module]s.
// Modules group [Column]s that share the same domain size and padding semantics.
// [Cell]s are scalar commitments, and [CoinField]s are random challenges drawn
// by the verifier.
//
// Constraints and verifier predicates are expressed as [Query] values:
// [Vanishing], [LagrangeEval], [LocalOpening], [TableRelation], and
// [LogDerivativeSum]. Each query references symbolic [Expression] objects that
// form an arithmetic AST evaluated at runtime.
//
// Protocol execution is handled by [Runtime], which holds concrete column
// assignments and drives Fiat-Shamir via [Runtime.AdvanceRound]. The same
// [ProverAction] interface is implemented by both the prover and the verifier, so
// both sides run against the identical execution loop.
//
// # Typical usage
//
//  1. Create a [System] with [NewSystemf].
//  2. Create [Module]s, [Round]s, [Column]s, [Cell]s, and [CoinField]s.
//  3. Register [Query] objects expressing the protocol predicates.
//  4. Hand the [System] to a compiler pipeline that reduces all queries.
//  5. Instantiate a [Runtime] and execute [ProverAction]s round by round,
//     calling [Runtime.AdvanceRound] between rounds.
package wiop
